// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package opds provides the routes for the OPDS catalogs.
package opds

import (
	"net/http"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	bookmark_routes "codeberg.org/readeck/readeck/internal/bookmarks/routes"
	"codeberg.org/readeck/readeck/internal/opds/catalog"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/opds"
)

type opdsRouter struct {
	chi.Router
	srv *server.Server
}

// SetupRoutes adds the OPDS catalog HTTP routes.
func SetupRoutes(s *server.Server) {
	h := &opdsRouter{s.AuthenticatedRouter(), s}

	h.Use(middleware.GetHead)
	h.With(s.WithPermission("api:opds", "read")).Group(func(r chi.Router) {
		r.Get("/", h.mainCatalog)
		r.Route("/bookmarks", bookmark_routes.NewOPDSRouteHandler(s))
	})

	s.AddRoute("/opds", h)
}

func (h *opdsRouter) mainCatalog(w http.ResponseWriter, r *http.Request) {
	lastUpdate, err := bookmarks.Bookmarks.GetLastUpdate(
		goqu.C("user_id").Eq(auth.GetRequestUser(r).ID),
	)
	if err != nil {
		h.srv.Error(w, r, err)
		return
	}

	tr := h.srv.Locale(r)

	c := catalog.New(h.srv, r,
		catalog.WithFeedType(opds.OPDSTypeNavigation),
		catalog.WithTitle("Readeck"),
		catalog.WithUpdated(lastUpdate),
		catalog.WithURL(h.srv.AbsoluteURL(r).String()),
		catalog.WithNavEntry(
			tr.Gettext("Unread Bookmarks"), lastUpdate,
			h.srv.AbsoluteURL(r, ".", "bookmarks/unread").String(),
		),
		catalog.WithNavEntry(
			tr.Gettext("Archived Bookmarks"), lastUpdate,
			h.srv.AbsoluteURL(r, ".", "bookmarks/archives").String(),
		),
		catalog.WithNavEntry(
			tr.Gettext("Favorite Bookmarks"), lastUpdate,
			h.srv.AbsoluteURL(r, ".", "bookmarks/favorites").String(),
		),
		catalog.WithNavEntry(
			tr.Gettext("All Bookmarks"), lastUpdate,
			h.srv.AbsoluteURL(r, ".", "bookmarks/all").String(),
		),
		catalog.WithNavEntry(
			tr.Gettext("Bookmark Collections"), lastUpdate,
			h.srv.AbsoluteURL(r, ".", "bookmarks/collections").String(),
			func(e *opds.Entry) {
				e.Links[0].TypeLink = opds.OPDSTypeNavigation
			},
		),
	)

	if err := c.Render(w, r); err != nil {
		h.srv.Error(w, r, err)
	}
}
