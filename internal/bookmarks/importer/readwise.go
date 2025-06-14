// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package importer

import (
	"bufio"
	"context"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"time"

	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type readwiseAdapter struct {
	idx   int
	Items []readwiseBookmarkItem `json:"items"`
}

type readwiseBookmarkItem struct {
	Link       string        `json:"url"`
	Title      string        `json:"title"`
	Created    time.Time     `json:"created"`
	Labels     types.Strings `json:"labels"`
	IsArchived bool          `json:"is_archived"`
	IsFavorite bool          `json:"is_favorite"`
}

const (
	// Readwise Reader exported CSV headers are:
	// Title, URL, ID, Document tags, Saved date, Reading progress, Location, Seen.
	readwiseHeaderTitle    = 0
	readwiseHeaderURL      = 1
	readwiseHeaderTags     = 3
	readwiseHeaderCreated  = 4
	readwiseHeaderLocation = 6

	// Basically time.RFC3339, but with space character instead of "T".
	readwiseTimeFormat = "2006-01-02 15:04:05-07:00"
)

var readwiseBookmarkSkipErr = errors.New("bookmark skip")

func newReadwiseBookmarkItem(record []string) (readwiseBookmarkItem, error) {
	res := readwiseBookmarkItem{}
	if len(record) < readwiseHeaderLocation {
		return res, errors.New("not enough columns in CSV")
	}
	// Skip items added to Reader via email forward rather than a URL
	if strings.HasPrefix(record[readwiseHeaderURL], "mailto:") {
		return res, readwiseBookmarkSkipErr
	}
	res.Link = record[readwiseHeaderURL]
	res.Title = strings.TrimSpace(record[readwiseHeaderTitle])

	if record[readwiseHeaderCreated] != "" {
		if createdTime, err := time.Parse(readwiseTimeFormat, record[readwiseHeaderCreated]); err == nil {
			res.Created = createdTime
		} else {
			return res, fmt.Errorf("error parsing created timestamp: %w", err)
		}
	}

	if record[readwiseHeaderTags] != "" {
		tags, err := parseReadwiseTags(record[readwiseHeaderTags])
		if err != nil {
			return res, fmt.Errorf("error parsing tags: %w", err)
		}
		if slices.Contains(tags, "favorite") {
			res.IsFavorite = true
			res.Labels = slices.DeleteFunc(tags, func(tag string) bool {
				return tag == "favorite"
			})
		} else {
			res.Labels = tags
		}
	}

	if strings.ToLower(record[readwiseHeaderLocation]) == "archive" {
		res.IsArchived = true
	}

	return res, nil
}

func (bi *readwiseBookmarkItem) URL() string {
	return bi.Link
}

func (bi *readwiseBookmarkItem) Meta() (*BookmarkMeta, error) {
	return &BookmarkMeta{
		Title:      bi.Title,
		Created:    bi.Created,
		Labels:     bi.Labels,
		IsArchived: bi.IsArchived,
		IsMarked:   bi.IsFavorite,
	}, nil
}

func (adapter *readwiseAdapter) Name(tr forms.Translator) string {
	return tr.Gettext("Readwise Reader CSV")
}

func (adapter *readwiseAdapter) Form() forms.Binder {
	return forms.Must(
		context.Background(),
		forms.NewFileField("data", forms.Required),
	)
}

func (adapter *readwiseAdapter) Params(form forms.Binder) ([]byte, error) {
	if !form.IsValid() {
		return nil, nil
	}

	reader, err := form.Get("data").(*forms.FileField).V().Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close() //nolint:errcheck

	r := csv.NewReader(reader)
	// discard the header row
	if _, err := r.Read(); err != nil {
		form.AddErrors("data", forms.Gettext("Empty or invalid import file"))
		return nil, nil
	}

	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			form.AddErrors("data", forms.Gettext("Empty or invalid import file"))
			return nil, nil
		}
		item, err := newReadwiseBookmarkItem(record)
		if errors.Is(err, readwiseBookmarkSkipErr) {
			continue
		} else if err != nil {
			form.AddErrors("data", forms.Gettext("Empty or invalid import file"))
			return nil, nil
		}

		adapter.Items = append(adapter.Items, item)
	}

	if len(adapter.Items) == 0 {
		form.AddErrors("data", forms.Gettext("Empty or invalid import file"))
		return nil, nil
	}

	slices.Reverse(adapter.Items)
	return json.Marshal(adapter)
}

func (adapter *readwiseAdapter) LoadData(data []byte) error {
	return json.Unmarshal(data, adapter)
}

func (adapter *readwiseAdapter) Next() (BookmarkImporter, error) {
	if adapter.idx+1 > len(adapter.Items) {
		return nil, io.EOF
	}

	adapter.idx++
	return &adapter.Items[adapter.idx-1], nil
}

// Readwise Reader CSV export encodes document tags as a JSON-like array, but it's not valid JSON
// due to single quotes used. Since Readwise does not allow double quotes nor backslashes in tag
// values, we can get away with a straightforward parser.
func parseReadwiseTags(field string) ([]string, error) {
	var tags []string

	r := bufio.NewReader(strings.NewReader(field))
	if delim, err := r.ReadByte(); err != nil {
		return tags, err
	} else if delim != '[' {
		return tags, errors.New("invalid label format")
	}

	for {
		char, err := r.ReadByte()
		if err != nil {
			return tags, err
		}

		if char == ']' {
			break
		}

		if char == '\'' || char == '"' {
			tagValue, err := r.ReadString(char)
			if err != nil {
				return tags, err
			}
			tags = append(tags, tagValue[:len(tagValue)-1])
		}
	}

	return tags, nil
}
