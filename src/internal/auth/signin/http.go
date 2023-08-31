// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package signin

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"codeberg.org/readeck/readeck/internal/auth"
	"codeberg.org/readeck/readeck/internal/email"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/forms"
)

// SetupRoutes mounts the routes for the auth domain.
func SetupRoutes(s *server.Server) {
	newAuthHandler(s)

	api := newAuthAPI(s)
	s.AddRoute("/api/auth", api)
}

type authHandler struct {
	chi.Router
	srv *server.Server
}

func newAuthHandler(s *server.Server) *authHandler {
	// Non authenticated routes
	r := chi.NewRouter()
	r.Use(
		s.WithSession(),
		s.Csrf,
	)

	h := &authHandler{r, s}
	s.AddRoute("/login", r)
	r.Get("/", h.login)
	r.Post("/", h.login)

	if email.CanSendEmail() {
		r.Get("/recover", h.recover)
		r.Post("/recover", h.recover)
		r.Get("/recover/{code}", h.recover)
		r.Post("/recover/{code}", h.recover)
	}

	// Authenticated routes
	ar := chi.NewRouter()
	ar.Use(
		s.WithSession(),
		s.WithRedirectLogin,
		auth.Required,
	)
	s.AddRoute("/logout", ar)

	ar.Post("/", h.logout)

	return h
}

func (h *authHandler) login(w http.ResponseWriter, r *http.Request) {
	f := newLoginForm()

	if r.Method == http.MethodGet {
		// Set the redirect value from the query string
		f.Get("redirect").Set(r.URL.Query().Get("r"))
	}

	if r.Method == http.MethodPost {
		forms.Bind(f, r)

		if f.IsValid() {
			user := f.checkUser()
			if user != nil {
				// User is authenticated, let's carry on
				sess := h.srv.GetSession(r)
				sess.Payload.User = user.ID
				sess.Payload.Seed = user.Seed
				sess.Save(r, w)

				// Get redirection from a form "redirect" parameter
				// Since it goes to Redirect(), it will be sanitized there
				// and can only stay within the app.
				redir := f.Get("redirect").String()
				if redir == "" || strings.HasPrefix(redir, "/login") {
					redir = "/"
				}

				h.srv.Redirect(w, r, redir)
				return
			}
			// we must set the content type to avoid the
			// error middleware interception.
			w.Header().Set("content-type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
		}
		w.WriteHeader(http.StatusUnprocessableEntity)
	}

	h.srv.RenderTemplate(w, r, http.StatusOK, "/auth/login", server.TC{
		"Form": f,
	})
}

func (h *authHandler) logout(w http.ResponseWriter, r *http.Request) {
	sess := h.srv.GetSession(r)
	sess.MaxAge = -1
	if err := sess.Save(r, w); err != nil {
		h.srv.Error(w, r, err)
		return
	}

	h.srv.Redirect(w, r, "/login")
}
