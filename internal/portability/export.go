// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package portability

import (
	"archive/zip"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

// Exporter is a content exporter.
// It exports content into a zip file writer.
type Exporter struct {
	userIDs  []int
	zfs      *zipfs.ZipRW
	output   io.Writer
	manifest exportManifest
}

// NewExporter creates a new [Exporter] for the given users. The provided [io.Writer] is used
// to output contents as a zip file.
func NewExporter(w io.Writer, usernames []string) (*Exporter, error) {
	var userIDs []int
	ds := users.Users.Query().Select(goqu.C("id"))

	if len(usernames) > 0 {
		ds = ds.Where(goqu.C("username").In(usernames))
	}
	if err := ds.ScanVals(&userIDs); err != nil {
		return nil, err
	}

	return &Exporter{
		userIDs: userIDs,
		zfs:     zipfs.NewZipRW(w, nil, 0),
		manifest: exportManifest{
			Date:  time.Now(),
			Files: make(map[string]string),
		},
		output: io.Discard,
	}, nil
}

// Close closes the output zip file descriptor.
func (ex *Exporter) Close() error {
	return ex.zfs.Close()
}

// Output returns the message output writer.
func (ex *Exporter) Output() io.Writer {
	return ex.output
}

// SetOutput sets the message output writer.
func (ex *Exporter) SetOutput(w io.Writer) {
	ex.output = w
}

// ExportAll exports all the user content.
func (ex *Exporter) ExportAll() error {
	var err error
	data := portableData{
		Info: exportInfo{
			Date:           time.Now(),
			Version:        "1",
			ReadeckVersion: configs.Version(),
		},
	}

	if data.Users, err = marshalItems[*users.User](
		users.Users.Query().
			Where(goqu.C("id").In(ex.userIDs)).
			Order(goqu.C("username").Asc()),
	); err != nil {
		return err
	}
	fmt.Fprintf(ex.output, "\t- %d user(s) exported\n", len(data.Users)) // nolint:errcheck

	if data.Tokens, err = marshalItems[*tokens.Token](
		tokens.Tokens.Query().
			Where(goqu.C("user_id").In(ex.userIDs)).
			Order(goqu.C("created").Asc()),
	); err != nil {
		return err
	}
	fmt.Fprintf(ex.output, "\t- %d token(s) exported\n", len(data.Tokens)) // nolint:errcheck

	if data.BookmarkCollections, err = marshalItems[*bookmarks.Collection](
		bookmarks.Collections.Query().
			Where(goqu.C("user_id").In(ex.userIDs)).
			Order(goqu.C("created").Asc()),
	); err != nil {
		return err
	}
	fmt.Fprintf(ex.output, "\t- %d collection(s) exported\n", len(data.BookmarkCollections)) // nolint:errcheck

	if data.Bookmarks, err = marshalItems[bookmarkItem](
		bookmarks.Bookmarks.Query().
			Select("uid", "user_id").
			Where(goqu.C("user_id").In(ex.userIDs)).
			Order(goqu.C("created").Asc()),
	); err != nil {
		return err
	}

	// Save each bookmark now
	for _, item := range data.Bookmarks {
		if err = ex.saveBookmark(item); err != nil {
			return err
		}
	}
	fmt.Fprintf(ex.output, "\t- %d bookmark(s) exported\n", len(data.Bookmarks)) // nolint:errcheck

	// Save data.json
	w, err := ex.zfs.GetWriter(&zip.FileHeader{Name: "data.json", Method: zip.Deflate})
	if err != nil {
		return err
	}
	enc := json.NewEncoder(w)
	return enc.Encode(data)
}

func (ex *Exporter) saveBookmark(item bookmarkItem) error {
	b, err := bookmarks.Bookmarks.GetOne(goqu.C("uid").Eq(item.UID))
	if err != nil {
		return err
	}

	dest := path.Join("bookmarks", b.UID, "info.json")
	w, err := ex.zfs.GetWriter(&zip.FileHeader{
		Name:   dest,
		Method: zip.Deflate,
	})
	if err != nil {
		return err
	}

	enc := json.NewEncoder(w)
	if err = enc.Encode(b); err != nil {
		return err
	}

	c, err := b.OpenContainer()
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}

	for _, x := range c.File {
		if x.FileInfo().IsDir() {
			continue
		}

		h := x.FileHeader
		h.Name = path.Join("bookmarks", b.UID, "container", h.Name)

		r, err := x.OpenRaw()
		if err != nil {
			return err
		}

		w, err := ex.zfs.GetRawWriter(&h)
		if err != nil {
			return err
		}

		if _, err = io.Copy(w, r); err != nil {
			return err
		}
	}

	return nil
}
