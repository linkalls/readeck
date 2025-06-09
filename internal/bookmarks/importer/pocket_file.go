// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package importer

import (
	"archive/zip"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"

	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/forms"
)

type pocketFileAdapter struct {
	idx   int
	Items []pocketBookmarkItem
}

type pocketBookmarkItem struct {
	Link       string        `json:"url"`
	Title      string        `json:"title"`
	Labels     types.Strings `json:"labels"`
	IsArchived bool          `json:"is_archived"`
	Created    time.Time     `json:"created"`
}

func (bi *pocketBookmarkItem) URL() string {
	return bi.Link
}

func (bi *pocketBookmarkItem) Meta() (*BookmarkMeta, error) {
	return &BookmarkMeta{
		Title:      bi.Title,
		Labels:     bi.Labels,
		IsArchived: bi.IsArchived,
		Created:    bi.Created,
	}, nil
}

func (adapter *pocketFileAdapter) Name(_ forms.Translator) string {
	return "Pocket"
}

func (adapter *pocketFileAdapter) Form() forms.Binder {
	return forms.Must(
		context.Background(),
		forms.NewFileField("data", forms.Required),
	)
}

func (adapter *pocketFileAdapter) Params(form forms.Binder) ([]byte, error) {
	if !form.IsValid() {
		return nil, nil
	}

	opener := form.Get("data").(*forms.FileField).V()

	reader, err := opener.Open()
	if err != nil {
		return nil, err
	}
	defer reader.Close() //nolint:errcheck

	zr, err := zip.NewReader(reader.(io.ReaderAt), opener.Size())
	if err != nil {
		form.AddErrors("data", errInvalidFile)
		return nil, nil
	}

	if err := adapter.loadBookmarks(zr); err != nil {
		form.AddErrors("data", err)
	}

	if len(adapter.Items) == 0 {
		form.AddErrors("data", errInvalidFile)
		return nil, nil
	}

	slices.SortFunc(adapter.Items, func(a, b pocketBookmarkItem) int {
		return int(a.Created.Unix()) - int(b.Created.Unix())
	})

	return json.Marshal(adapter)
}

func (adapter *pocketFileAdapter) LoadData(data []byte) error {
	return json.Unmarshal(data, adapter)
}

func (adapter *pocketFileAdapter) Next() (BookmarkImporter, error) {
	if adapter.idx+1 > len(adapter.Items) {
		return nil, io.EOF
	}

	adapter.idx++
	return &adapter.Items[adapter.idx-1], nil
}

func (adapter *pocketFileAdapter) loadBookmarks(zr *zip.Reader) error {
	// In the absence of any specification, we'll consider that any CSV file contains bookmarks.
	for _, file := range zr.File {
		if !strings.HasSuffix(file.Name, ".csv") {
			continue
		}
		if err := adapter.loadBookmarkRows(file); err != nil {
			return err
		}
	}

	return nil
}

func (adapter *pocketFileAdapter) loadBookmarkRows(f *zip.File) error {
	r, err := f.Open()
	if err != nil {
		return err
	}
	defer r.Close() //nolint:errcheck

	cr := csv.NewReader(r)
	header, err := cr.Read()
	if err != nil {
		return err
	}

	var idxTitle, idxURL, idxTime, idxTags, idxStatus int
	for i, x := range header {
		switch x {
		case "title":
			idxTitle = i
		case "url":
			idxURL = i
		case "time_added":
			idxTime = i
		case "tags":
			idxTags = i
		case "status":
			idxStatus = i
		}
	}

	for {
		record, err := cr.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			continue
		}
		if len(record) != len(header) {
			continue
		}

		uri, err := url.Parse(record[idxURL])
		if err != nil {
			continue
		}
		if !slices.Contains(allowedSchemes, uri.Scheme) {
			continue
		}
		uri.Fragment = ""
		if slices.ContainsFunc(adapter.Items, func(bi pocketBookmarkItem) bool {
			return bi.Link == uri.String()
		}) {
			continue
		}

		item := pocketBookmarkItem{
			Created:    time.Now(),
			Link:       uri.String(),
			IsArchived: record[idxStatus] == "archive",
			Labels:     types.Strings{},
		}

		title := strings.TrimSpace(record[idxTitle])
		if title != item.Link {
			item.Title = title
		}

		for _, label := range strings.Split(record[idxTags], "|") {
			if label = strings.TrimSpace(label); label != "" {
				item.Labels = append(item.Labels, label)
			}
		}

		ts, err := strconv.Atoi(record[idxTime])
		if err == nil {
			item.Created = time.Unix(int64(ts), 0).UTC()
		}

		adapter.Items = append(adapter.Items, item)
	}

	return nil
}
