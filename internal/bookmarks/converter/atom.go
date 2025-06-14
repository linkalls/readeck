// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package converter

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"sort"
	"time"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/atom"
	"codeberg.org/readeck/readeck/pkg/base58"
)

// AtomExporter is an [Exporter] that produces an Atom feed.
type AtomExporter struct {
	HTMLConverter
	srv *server.Server
}

// NewAtomExporter return a new [AtomExporter] instance.
func NewAtomExporter(srv *server.Server) AtomExporter {
	return AtomExporter{
		HTMLConverter: HTMLConverter{},
		srv:           srv,
	}
}

// Export implements [Exporter].
func (e AtomExporter) Export(ctx context.Context, w io.Writer, r *http.Request, bookmarkList []*bookmarks.Bookmark) error {
	if w, ok := w.(http.ResponseWriter); ok {
		w.Header().Set("Content-Type", atom.MimeType)
	}

	selfURL := e.srv.AbsoluteURL(r).String()
	htmlURL := e.srv.AbsoluteURL(r, "/bookmarks").String()
	if len(r.URL.Query()) > 0 {
		htmlURL += "?" + r.URL.Query().Encode()
	}

	mtimes := []time.Time{configs.BuildTime()}
	for _, b := range bookmarkList {
		mtimes = append(mtimes, b.Updated)
	}
	sort.Slice(mtimes, func(i, j int) bool {
		return mtimes[i].After(mtimes[j])
	})

	feed := &atom.Feed{
		Xmlns:    atom.NS,
		ID:       "urn:uuid:",
		Title:    "Readeck",
		Subtitle: "Readeck's Bookmarks",
		Updated:  atom.Time(mtimes[0]),
		Link: []*atom.Link{
			{
				Href: selfURL,
				Rel:  "self",
				Type: atom.MimeType,
			},
			{
				Href: htmlURL,
				Rel:  "alternate",
				Type: "text/html",
			},
		},
		Icon: e.srv.AbsoluteURL(r, "/", e.srv.AssetURL(r, "img/fi/favicon.ico")).String(),
	}

	feed.Entries = make([]*atom.Entry, len(bookmarkList))

	for i, b := range bookmarkList {
		id, _ := base58.DecodeUUID(b.UID)

		feed.Entries[i] = &atom.Entry{
			ID:        "urn:uuid:" + id.String(),
			Title:     b.Title,
			Updated:   atom.Time(b.Updated),
			Published: atom.Time(b.Created),
			Links: []*atom.Link{
				{
					Href: e.srv.AbsoluteURL(r, "/bookmarks", b.UID).String(),
					Rel:  "alternate",
					Type: "text/html",
				},
			},
		}

		if b.Description != "" {
			feed.Entries[i].Summary = &atom.Summary{
				Type:    "text",
				Content: b.Description,
			}
		}

		resources := bookmarks.BookmarkFiles{}
		for k, v := range b.Files {
			if k == "image" && (b.DocumentType == "photo" || b.DocumentType == "video") {
				resources[k] = &bookmarks.BookmarkFile{
					Name: e.srv.AbsoluteURL(r, "/bm", b.FilePath, v.Name).String(),
					Type: v.Type,
					Size: v.Size,
				}
			}
		}

		buf := new(bytes.Buffer)
		html, err := e.GetArticle(ctx, b)
		if err != nil {
			return err
		}
		tpl, err := server.GetTemplate("bookmarks/bookmark_atom.jet.html")
		if err != nil {
			return err
		}
		tc := map[string]any{
			"HTML":      html,
			"Item":      b,
			"Resources": resources,
		}
		if err := tpl.Execute(buf, e.srv.TemplateVars(r), tc); err != nil {
			return err
		}
		feed.Entries[i].Content = &atom.Content{
			Type:    "html",
			Content: buf.String(),
		}
	}

	return feed.WriteXML(w)
}
