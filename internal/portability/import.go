// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package portability

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"

	"github.com/doug-martin/goqu/v9"

	"codeberg.org/readeck/readeck/internal/auth/tokens"
	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/db"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

// Importer is a content importer.
type Importer struct {
	usernames []string
	users     map[int]int
	clearData bool
	zr        *zip.Reader
	output    io.Writer
}

// NewImporter creates a new [Importer].
func NewImporter(zr *zip.Reader, usernames []string, clearData bool) (*Importer, error) {
	return &Importer{
		zr:        zr,
		usernames: usernames,
		users:     map[int]int{},
		clearData: clearData,
		output:    io.Discard,
	}, nil
}

// Output returns the message output writer.
func (imp *Importer) Output() io.Writer {
	return imp.output
}

// SetOutput sets the message output writer.
func (imp *Importer) SetOutput(w io.Writer) {
	imp.output = w
}

// Load loads all the data into Readeck's database and content folder.
func (imp *Importer) Load() error {
	fd, err := imp.zr.Open("data.json")
	if err != nil {
		return err
	}

	fnList := []func(*goqu.TxDatabase, *portableData) error{
		imp.loadUsers,
		imp.loadTokens,
		imp.loadCollections,
		imp.loadBookmarks,
	}

	var data portableData
	dec := json.NewDecoder(fd)
	if err = dec.Decode(&data); err != nil {
		return err
	}
	tx, err := db.Q().Begin()
	if err != nil {
		return err
	}
	return tx.Wrap(func() error {
		if imp.clearData {
			if err = imp.clearDB(tx); err != nil {
				return err
			}
		}

		for _, fn := range fnList {
			if err = fn(tx, &data); err != nil {
				return err
			}
		}
		return nil
	})
}

func (imp *Importer) clearDB(tx *goqu.TxDatabase) error {
	if _, err := tx.Delete(bookmarks.TableName).Executor().Exec(); err != nil {
		return err
	}

	if _, err := tx.Delete(users.TableName).Executor().Exec(); err != nil {
		return err
	}
	return nil
}

func (imp *Importer) loadUsers(tx *goqu.TxDatabase, data *portableData) (err error) {
	allUsers := len(imp.usernames) == 0

	for _, item := range data.Users {
		if allUsers {
			imp.usernames = append(imp.usernames, item.Username)
		}

		if !slices.Contains(imp.usernames, item.Username) {
			continue
		}

		var count int64
		count, err = tx.Select().From(users.TableName).Where(
			goqu.Or(
				goqu.C("username").Eq(item.Username),
				goqu.C("email").Eq(item.Email),
			),
		).Prepared(true).Count()
		if err != nil {
			return err
		}
		if count > 0 {
			fmt.Fprintf( // nolint:errcheck
				imp.output,
				"\tERR: user \"%s\" or \"%s\" already exists\n", item.Username, item.Email,
			)
			continue
		}

		originalID := item.ID
		if item.ID, err = insertInto(tx, users.TableName, item, func(x *users.User) {
			x.ID = 0
			x.SetSeed()
			if !imp.clearData || x.UID == "" {
				x.UID = base58.NewUUID()
			}
		}); err != nil {
			return
		}
		imp.users[originalID] = item.ID
	}

	fmt.Fprintf(imp.output, "\t- %d user(s) imported\n", len(imp.users)) // nolint:errcheck
	return
}

func (imp *Importer) loadTokens(tx *goqu.TxDatabase, data *portableData) (err error) {
	ids := slices.Collect(maps.Keys(imp.users))

	i := 0
	for _, item := range data.Tokens {
		if item.UserID == nil {
			continue
		}
		if !slices.Contains(ids, *item.UserID) {
			continue
		}

		if item.ID, err = insertInto(tx, tokens.TableName, item, func(x *tokens.Token) {
			x.ID = 0
			x.UserID = ptrTo(imp.users[*x.UserID])
			if !imp.clearData || x.UID == "" {
				x.UID = base58.NewUUID()
			}
		}); err != nil {
			return
		}
		i++
	}

	fmt.Fprintf(imp.output, "\t- %d token(s) imported\n", i) // nolint:errcheck
	return
}

func (imp *Importer) loadCollections(tx *goqu.TxDatabase, data *portableData) (err error) {
	ids := slices.Collect(maps.Keys(imp.users))

	i := 0
	for _, item := range data.BookmarkCollections {
		if item.UserID == nil {
			continue
		}
		if !slices.Contains(ids, *item.UserID) {
			continue
		}

		if item.ID, err = insertInto(tx, bookmarks.CollectionTable, item, func(x *bookmarks.Collection) {
			x.ID = 0
			x.UserID = ptrTo(imp.users[*x.UserID])
			if !imp.clearData || x.UID == "" {
				x.UID = base58.NewUUID()
			}
		}); err != nil {
			return
		}
		i++
	}

	fmt.Fprintf(imp.output, "\t- %d collection(s) imported\n", i) // nolint:errcheck
	return
}

func (imp *Importer) loadBookmarks(tx *goqu.TxDatabase, data *portableData) (err error) {
	ids := slices.Collect(maps.Keys(imp.users))

	i := 0
	for _, item := range data.Bookmarks {
		if !slices.Contains(ids, item.UserID) {
			continue
		}

		if err = imp.loadBookmark(tx, &item); err != nil {
			return
		}
		i++
	}

	fmt.Fprintf(imp.output, "\t- %d bookmark(s) imported\n", i) // nolint:errcheck
	return
}

func (imp *Importer) loadBookmark(tx *goqu.TxDatabase, item *bookmarkItem) (err error) {
	p := path.Join("bookmarks", item.UID, "info.json")
	fd, err := imp.zr.Open(p)
	if err != nil {
		return err
	}

	var b bookmarks.Bookmark
	dec := json.NewDecoder(fd)
	if err = dec.Decode(&b); err != nil {
		return
	}

	if b.ID, err = insertInto(tx, bookmarks.TableName, &b, func(x *bookmarks.Bookmark) {
		x.ID = 0
		x.FilePath, _ = x.GetBaseFileURL()
		x.UserID = ptrTo(imp.users[*x.UserID])
		if !imp.clearData || x.UID == "" {
			x.UID = base58.NewUUID()
		}
	}); err != nil {
		return
	}

	// Copy files to zipfile
	dest := filepath.Join(bookmarks.StoragePath(), b.FilePath+".zip")
	if err = os.MkdirAll(path.Dir(dest), 0o750); err != nil {
		return err
	}
	w, err := os.Create(dest)
	if err != nil {
		return err
	}

	zw := zipfs.NewZipRW(w, nil, 0)
	defer zw.Close() // nolint:errcheck

	prefix := "bookmarks/" + item.UID + "/container/"
	for _, f := range imp.zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		if !strings.HasPrefix(f.Name, prefix) {
			continue
		}

		h := f.FileHeader
		h.Name = strings.TrimPrefix(h.Name, prefix)

		rr, err := f.OpenRaw()
		if err != nil {
			return err
		}

		rw, err := zw.GetRawWriter(&h)
		if err != nil {
			return err
		}

		if _, err = io.Copy(rw, rr); err != nil {
			return err
		}
	}

	return
}
