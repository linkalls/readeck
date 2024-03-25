// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes

import (
	"net/http"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/forms"
)

func (h *viewsRouter) collectionList(w http.ResponseWriter, r *http.Request) {
	cl := r.Context().Value(ctxCollectionListKey{}).(collectionList)
	cl.Items = make([]collectionItem, len(cl.items))
	for i, item := range cl.items {
		cl.Items[i] = newCollectionItem(h.srv, r, item, ".")
	}

	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["Collections"] = cl.Items

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/collection_list", ctx)
}

func (h *viewsRouter) collectionCreate(w http.ResponseWriter, r *http.Request) {
	f := newCollectionForm(h.srv.Locale(r))

	switch r.Method {
	case http.MethodGet:
		// Add values from query string but don't perform validation
		f.BindQueryString(r.URL.Query())
	case http.MethodPost:
		forms.Bind(f, r)
		if f.IsValid() {
			c, err := f.createCollection(auth.GetRequestUser(r).ID)
			if err != nil {
				h.srv.Log(r).Error(err)
			} else {
				h.srv.Redirect(w, r, "./..", c.UID)
				return
			}
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)
	bl.Items = make([]bookmarkItem, len(bl.items))
	for i, item := range bl.items {
		bl.Items[i] = newBookmarkItem(h.srv, r, item, ".")
	}

	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["Pagination"] = bl.Pagination
	ctx["Bookmarks"] = bl.Items
	ctx["Form"] = f

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/collection_create", ctx)
}

func (h *viewsRouter) collectionInfo(w http.ResponseWriter, r *http.Request) {
	c := r.Context().Value(ctxCollectionKey{}).(*bookmarks.Collection)
	item := newCollectionItem(h.srv, r, c, "./..")

	f := newCollectionForm(h.srv.Locale(r))
	f.setCollection(c)

	if r.Method == http.MethodPost {
		forms.Bind(f, r)
		if f.IsValid() {
			if _, err := f.updateCollection(c); err != nil {
				h.srv.Log(r).Error(err)
			} else {
				tr := h.srv.Locale(r)
				h.srv.AddFlash(w, r, "success", tr.Gettext("Collection updated."))
				h.srv.Redirect(w, r, c.UID+"?edit=1")
				return
			}
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	bl := r.Context().Value(ctxBookmarkListKey{}).(bookmarkList)
	bl.Items = make([]bookmarkItem, len(bl.items))
	for i, item := range bl.items {
		bl.Items[i] = newBookmarkItem(h.srv, r, item, ".")
	}

	ctx := r.Context().Value(ctxBaseContextKey{}).(server.TC)
	ctx["Editing"] = r.URL.Query().Get("edit") == "1"
	ctx["Item"] = item
	ctx["Form"] = f
	ctx["Pagination"] = bl.Pagination
	ctx["Bookmarks"] = bl.Items

	h.srv.RenderTemplate(w, r, 200, "/bookmarks/collection", ctx)
}

func (h *viewsRouter) collectionDelete(w http.ResponseWriter, r *http.Request) {
	f := newCollectionDeleteForm(h.srv.Locale(r))
	f.Get("_to").Set("/bookmarks/collections")
	forms.Bind(f, r)

	c := r.Context().Value(ctxCollectionKey{}).(*bookmarks.Collection)

	// This update forces cache invalidation
	if err := c.Update(map[string]interface{}{}); err != nil {
		h.srv.Error(w, r, err)
		return
	}
	if err := f.trigger(c); err != nil {
		h.srv.Error(w, r, err)
		return
	}
	h.srv.Redirect(w, r, f.Get("_to").String())
}
