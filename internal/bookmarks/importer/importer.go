// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package importer provides the necessary tooling to import bookmarks
// from various sources.
package importer

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"slices"
	"time"

	"github.com/doug-martin/goqu/v9"
	log "github.com/sirupsen/logrus"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/db/types"
	"codeberg.org/readeck/readeck/pkg/forms"
)

const (
	importText       = "text"
	importBrowser    = "browser"
	importGoodLinks  = "goodlinks"
	importOmnivore   = "omnivore"
	importPocketFile = "pocket-file"
	importWallabag   = "wallabag"
)

var (
	ErrIgnore    = errors.New("ignore")     // ErrIgnore is an error that can be ignored.
	ErrNoAdapter = errors.New("no adapter") // ErrNoAdapter is returned when an adapter does not exist.
)

var allowedSchemes = []string{"http", "https"}

// ImportLoader describes an import loader.
type ImportLoader interface {
	Name(forms.Translator) string
	Form() forms.Binder
	Params(forms.Binder) ([]byte, error)
}

// ImportWorker describes an import worker.
type ImportWorker interface {
	LoadData([]byte) error
	Next() (BookmarkImporter, error)
}

// BookmarkImporter describes a simple adapter item.
type BookmarkImporter interface {
	URL() string
}

// BookmarkEnhancer describes an item providing more adapter item information.
type BookmarkEnhancer interface {
	Meta() (*BookmarkMeta, error)
}

// BookmarkResourceProvider describes an item providing attached resources.
type BookmarkResourceProvider interface {
	Resources() []tasks.MultipartResource
}

// BookmarkReadabilityToggler describes an item than disable readability.
type BookmarkReadabilityToggler interface {
	EnableReadability() bool
}

// BookmarkMeta provides an import item extra information.
type BookmarkMeta struct {
	Title         string
	Published     time.Time
	Authors       types.Strings
	Lang          string
	TextDirection string
	DocumentType  string
	Description   string
	Embed         string
	Labels        types.Strings
	IsArchived    bool
	IsMarked      bool
	Created       time.Time
}

type importer struct {
	worker          ImportWorker
	log             *log.Entry
	user            *users.User
	requestID       string
	allowDuplicates bool
	label           string
}

type urlBookmarkItem string

func newURLBookmark(src string) (urlBookmarkItem, error) {
	uri, err := url.Parse(src)
	if err != nil {
		return urlBookmarkItem(""), nil
	}
	if !slices.Contains(allowedSchemes, uri.Scheme) {
		return urlBookmarkItem(""), fmt.Errorf("%w: invalid scheme %s (%s)", ErrIgnore, uri.Scheme, src)
	}
	uri.Fragment = ""

	return urlBookmarkItem(uri.String()), nil
}

func (b urlBookmarkItem) URL() string {
	return string(b)
}

// LoadAdapter returns an import loader based on a given name.
func LoadAdapter(name string) ImportLoader {
	switch name {
	case importText:
		return &textAdapter{}
	case importBrowser:
		return &browserAdapter{}
	case importGoodLinks:
		return &goodlinksAdapter{}
	case importOmnivore:
		return &omnivoreAPIAdapter{}
	case importPocketFile:
		return &pocketFileAdapter{}
	case importWallabag:
		return &wallabagAdapter{}
	default:
		return nil
	}
}

// NewImportForm returns a [forms.Form] combining common fields
// and fields defined by the import adapter.
func NewImportForm(adapter ImportLoader) *forms.Form {
	allFields := []forms.Field{}
	for _, f := range adapter.Form().Fields() {
		allFields = append(allFields, f.Field)
	}

	// f := &ImportForm{
	f := forms.Must(
		append([]forms.Field{
			forms.NewTextField("label", forms.Trim),
			forms.NewBooleanField("ignore_duplicates", forms.RequiredOrNil),
		}, allFields...)...,
	)

	f.Get("ignore_duplicates").Set(true)

	return f
}

// Import performs the iteration on its adapter and import every item.
func (imp importer) Import(f func([]int)) {
	ids := []int{}

	for {
		b, err := imp.createBookmark(imp.worker.Next)
		logger := imp.log
		if b != nil {
			logger = logger.WithField("url", b.URL)
			if b.UID != "" {
				logger = logger.WithField("id", b.UID)
			}
		}

		if err == io.EOF {
			break
		}
		if errors.Is(err, ErrIgnore) {
			logger.WithError(err).Debug("import item")
			continue
		}
		if err != nil {
			logger.WithError(err).Error("import item")
			continue
		}

		logger.Info("bookmark created")
		ids = append(ids, b.ID)
		f(ids)
	}

	if len(ids) == 0 {
		if err := clearStoreProgressList(GetTrackID(imp.requestID)); err != nil {
			imp.log.WithError(err).Error("clearing progress")
		}
		imp.log.Info("import finished")
	}
}

func (imp importer) createBookmark(next func() (BookmarkImporter, error)) (*bookmarks.Bookmark, error) {
	item, err := next()
	if err != nil {
		return nil, err
	}

	uri, err := url.Parse(item.URL())
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrIgnore, err)
	}
	if !slices.Contains(allowedSchemes, uri.Scheme) {
		return nil, fmt.Errorf("%w: invalid scheme %s (%s)", ErrIgnore, uri.Scheme, uri)
	}
	uri.Fragment = ""

	b := &bookmarks.Bookmark{
		UserID:   &imp.user.ID,
		State:    bookmarks.StateLoading,
		URL:      uri.String(),
		Site:     uri.Hostname(),
		SiteName: uri.Hostname(),
	}

	if !imp.allowDuplicates {
		count, err := bookmarks.Bookmarks.Query().Where(
			goqu.C("user_id").Eq(imp.user.ID),
			goqu.Or(
				goqu.C("url").Eq(uri.String()),
				goqu.C("initial_url").Eq(uri.String()),
			),
		).Prepared(true).Count()
		if err != nil {
			return nil, err
		}
		if count > 0 {
			return nil, fmt.Errorf("already exists, %w", ErrIgnore)
		}
	}

	var created time.Time
	if t, ok := item.(BookmarkEnhancer); ok {
		bm, err := t.Meta()
		if err != nil {
			return nil, err
		}
		if !bm.Published.IsZero() {
			b.Published = &bm.Published
		}
		if bm.Title != "" {
			b.Title = bm.Title
		}

		b.Authors = bm.Authors
		b.Lang = bm.Lang
		b.TextDirection = bm.TextDirection
		b.DocumentType = bm.DocumentType
		b.Description = bm.Description
		b.Embed = bm.Embed
		b.Labels = bm.Labels
		b.IsArchived = bm.IsArchived
		b.IsMarked = bm.IsMarked
		created = bm.Created
	}

	if imp.label != "" {
		b.Labels = append(b.Labels, imp.label)
	}

	if err = bookmarks.Bookmarks.Create(b); err != nil {
		return nil, err
	}

	if !created.IsZero() {
		// Force update of the creation date
		_ = b.Update(map[string]interface{}{
			"created": created,
		})
	}

	var resources []tasks.MultipartResource
	if t, ok := item.(BookmarkResourceProvider); ok {
		resources = t.Resources()
	}

	readabilityEnabled := true
	if t, ok := item.(BookmarkReadabilityToggler); ok {
		readabilityEnabled = t.EnableReadability()
	}

	if err = ImportExtractTask.Run(b.ID, tasks.ExtractParams{
		BookmarkID: b.ID,
		RequestID:  imp.requestID,
		Resources:  resources,
		FindMain:   readabilityEnabled,
	}); err != nil {
		b.State = bookmarks.StateError
		_ = b.Save()
	}

	return b, nil
}
