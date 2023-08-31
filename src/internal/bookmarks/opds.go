// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"fmt"
	"net/http"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"
	"github.com/lithammer/shortuuid/v4"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/opds/catalog"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/opds"
)

type opdsRouter struct {
	chi.Router
	*apiRouter
}

// NewOPDSRouteHandler returns a chi Router handler with the OPDS
// routes for the bookmark domain.
func NewOPDSRouteHandler(s *server.Server) func(r chi.Router) {
	return func(r chi.Router) {
		h := &opdsRouter{r, newAPIRouter(s)}

		r.With(h.srv.WithPermission("api:bookmarks", "read")).Group(func(r chi.Router) {
			r.With(h.withCollectionFilters, h.withBookmarkList).Get("/all", h.bookmarkList)
			r.With(h.withBookmarkFilters, h.withBookmarkList).
				Get("/{filter:(unread|archives|favorites)}", h.bookmarkList)
			r.With(h.withColletionList).Get("/collections", h.collectionList)
			r.With(h.withCollection).Get("/collections/{uid}", h.collectionInfo)
		})
	}
}

func (h *opdsRouter) bookmarkList(w http.ResponseWriter, r *http.Request) {
	lastUpdate, err := Bookmarks.GetLastUpdate(
		goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
	)
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}

	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)

	c := catalog.New(h.srv, r,
		catalog.WithFeedType(opds.OPDSTypeAcquisistion),
		catalog.WithTitle("Readeck Bookmarks"),
		catalog.WithURL(h.srv.AbsoluteURL(r).String()),
		catalog.WithUpdated(lastUpdate),
		func(feed *opds.Feed) {
			links := h.srv.GetPaginationLinks(r, bl.Pagination)
			for _, x := range links {
				catalog.WithLink(opds.OPDSTypeAcquisistion, x.Rel, x.URL)(feed)
			}

			for _, b := range bl.items {
				id, _ := shortuuid.DefaultEncoder.Decode(b.UID)
				issued := b.Created
				if b.Published != nil && !b.Published.IsZero() {
					issued = *b.Published
				}

				catalog.WithBookEntry(
					id, b.Title,
					h.srv.AbsoluteURL(r, "/api/bookmarks", b.UID, "article.epub").String(),
					issued, b.Created, b.Updated,
					b.SiteName, b.Lang, b.Description,
				)(feed)
			}
		},
	)

	if err := c.Render(w, r); err != nil {
		h.srv.Error(w, r, err)
	}
}

func (h *opdsRouter) collectionList(w http.ResponseWriter, r *http.Request) {
	lastUpdate, err := Bookmarks.GetLastUpdate(
		goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
	)
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}

	cl := r.Context().Value(ctxCollectionListKey{}).(collectionList)

	c := catalog.New(h.srv, r,
		catalog.WithFeedType(opds.OPDSTypeNavigation),
		catalog.WithTitle("Readeck Bookmark Collections"),
		catalog.WithURL(h.srv.AbsoluteURL(r).String()),
		catalog.WithUpdated(lastUpdate),
		func(feed *opds.Feed) {
			links := h.srv.GetPaginationLinks(r, cl.Pagination)
			for _, x := range links {
				catalog.WithLink(opds.OPDSTypeAcquisistion, x.Rel, x.URL)(feed)
			}

			for _, item := range cl.items {
				catalog.WithNavEntry(
					item.Name, lastUpdate,
					h.srv.AbsoluteURL(r, ".", item.UID).String(),
				)(feed)
			}
		},
	)

	if err := c.Render(w, r); err != nil {
		h.srv.Error(w, r, err)
	}
}

func (h *opdsRouter) collectionInfo(w http.ResponseWriter, r *http.Request) {
	lastUpdate, err := Bookmarks.GetLastUpdate(
		goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
	)
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}

	item := r.Context().Value(ctxCollectionKey{}).(*Collection)
	c := catalog.New(h.srv, r,
		catalog.WithFeedType(opds.OPDSTypeAcquisistion),
		catalog.WithTitle(fmt.Sprintf("Readeck Collection: %s", item.Name)),
		catalog.WithURL(h.srv.AbsoluteURL(r).String()),
		catalog.WithUpdated(lastUpdate),
		func(feed *opds.Feed) {
			id, _ := shortuuid.DefaultEncoder.Decode(item.UID)
			catalog.WithBookEntry(
				id, fmt.Sprintf("Collection ebook - %s", item.Name),
				h.srv.AbsoluteURL(r, "/api/bookmarks", fmt.Sprintf("export.epub?collection=%s", item.UID)).String(),
				item.Created, item.Created, item.Updated,
				"Readeck", "", "",
			)(feed)

			catalog.WithNavEntry(
				fmt.Sprintf("Browse collection: %s", item.Name), lastUpdate,
				h.srv.AbsoluteURL(r, "../../bookmarks", fmt.Sprintf("all?collection=%s", item.UID)).String(),
			)(feed)
		},
	)

	if err := c.Render(w, r); err != nil {
		h.srv.Error(w, r, err)
	}
}
