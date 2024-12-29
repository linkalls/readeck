// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"fmt"
	"net/http"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks/importer"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/forms"
)

func (h *viewsRouter) bookmarksImportMain(w http.ResponseWriter, r *http.Request) {
	tr := h.srv.Locale(r)
	trackID := chi.URLParam(r, "trackID")

	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)

	if trackID != "" {
		ctx.SetBreadcrumbs([][2]string{
			{"Bookmarks", h.srv.AbsoluteURL(r, "/bookmarks").String()},
			{tr.Gettext("Import"), h.srv.AbsoluteURL(r, "/bookmarks/import").String()},
			{tr.Gettext("Progress")},
		})
		ctx["TrackID"] = trackID
		ctx["Running"] = importer.ImportBookmarksTask.IsRunning(trackID)
		ctx["Progress"], _ = importer.NewImportProgress(trackID)
	} else {
		ctx.SetBreadcrumbs([][2]string{
			{"Bookmarks", h.srv.AbsoluteURL(r, "/bookmarks").String()},
			{tr.Gettext("Import")},
		})
	}

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/import/index", ctx)
}

func (h *viewsRouter) bookmarksImport(w http.ResponseWriter, r *http.Request) {
	tr := h.srv.Locale(r)
	source := chi.URLParam(r, "source")
	if source == "" {
		h.srv.Status(w, r, http.StatusNotFound)
		return
	}

	adapter := importer.LoadAdapter(source)
	if adapter == nil {
		h.srv.Status(w, r, http.StatusNotFound)
		return
	}

	f := importer.NewImportForm(adapter)
	f.SetLocale(tr)

	templateName := fmt.Sprintf("/bookmarks/import/form-%s", source)
	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["Form"] = f
	ctx.SetBreadcrumbs([][2]string{
		{"Bookmarks", h.srv.AbsoluteURL(r, "/bookmarks").String()},
		{tr.Gettext("Import"), h.srv.AbsoluteURL(r, "/bookmarks/import").String()},
		{adapter.Name(tr)},
	})

	if r.Method == http.MethodPost {
		forms.BindMultipart(f, r)

		var data []byte
		var err error
		if f.IsValid() {
			data, err = adapter.Params(f)
		}
		if err != nil {
			h.srv.Error(w, r, err)
			return
		}

		if !f.IsValid() {
			h.srv.RenderTemplate(w, r, http.StatusUnprocessableEntity, templateName, ctx)
			return
		}

		ignoreDuplicates := true
		if !f.Get("ignore_duplicates").IsNil() {
			ignoreDuplicates = f.Get("ignore_duplicates").Value().(bool)
		}

		// Create the import task
		trackID := importer.GetTrackID(h.srv.GetReqID(r))
		err = importer.ImportBookmarksTask.Run(trackID, importer.ImportParams{
			Source:          source,
			Data:            data,
			UserID:          auth.GetRequestUser(r).ID,
			RequestID:       h.srv.GetReqID(r),
			AllowDuplicates: !ignoreDuplicates,
			Label:           f.Get("label").String(),
		})
		if err != nil {
			h.srv.Error(w, r, err)
			return
		}

		h.srv.Redirect(w, r, "./..", trackID)
		return
	}

	h.srv.RenderTemplate(w, r, 200, templateName, ctx)
}
