package admin

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/internal/auth"
	"github.com/readeck/readeck/internal/auth/users"
	"github.com/readeck/readeck/internal/bookmarks"
	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/pkg/forms"
)

type (
	ctxUserListKey struct{}
	ctxUserKey     struct{}
)

var (
	errSameUser = errors.New("same user as authenticated")
)

type adminAPI struct {
	chi.Router
	srv *server.Server
}

func newAdminAPI(s *server.Server) *adminAPI {
	r := s.AuthenticatedRouter()
	api := &adminAPI{r, s}

	r.With(api.srv.WithPermission("api:admin:users", "read")).Group(func(r chi.Router) {
		r.With(api.withUserList).Get("/users", api.userList)
		r.With(api.withUser).Get("/users/{id:\\d+}", api.userInfo)
	})

	r.With(api.srv.WithPermission("api:admin:users", "write")).Group(func(r chi.Router) {
		r.With(api.withUserList).Post("/users", api.userCreate)
		r.With(api.withUser).Patch("/users/{id:\\d+}", api.userUpdate)
		r.With(api.withUser).Delete("/users/{id:\\d+}", api.userDelete)
	})

	return api
}

func (api *adminAPI) withUserList(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		res := userList{}

		pf := api.srv.GetPageParams(r, 50)
		if pf == nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}

		ds := users.Users.Query().
			Order(goqu.I("username").Asc()).
			Limit(uint(pf.Limit())).
			Offset(uint(pf.Offset()))

		var count int64
		var err error
		if count, err = ds.ClearOrder().ClearLimit().ClearOffset().Count(); err != nil {
			if errors.Is(err, users.ErrNotFound) {
				api.srv.TextMessage(w, r, http.StatusNotFound, "not found")
			} else {
				api.srv.Error(w, r, err)
			}
			return
		}

		res.items = []*users.User{}
		if err = ds.ScanStructs(&res.items); err != nil {
			api.srv.Error(w, r, err)
			return
		}

		res.Pagination = api.srv.NewPagination(r, int(count), pf.Limit(), pf.Offset())

		ctx := context.WithValue(r.Context(), ctxUserListKey{}, res)
		next.ServeHTTP(w, r.Clone(ctx))
	})
}

func (api *adminAPI) withUser(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		userid := chi.URLParam(r, "id")

		u, err := users.Users.GetOne(
			goqu.C("id").Eq(userid),
		)
		if err != nil {
			api.srv.Status(w, r, http.StatusNotFound)
			return
		}

		ctx := context.WithValue(r.Context(), ctxUserKey{}, u)
		next.ServeHTTP(w, r.Clone(ctx))
	})
}

func (api *adminAPI) deleteUser(r *http.Request, u *users.User) error {
	if u.ID == auth.GetRequestUser(r).ID {
		return errSameUser
	}

	// Remove user's bookmarks first
	if err := bookmarks.Bookmarks.DeleteUserBookmakrs(u); err != nil {
		return err
	}

	return u.Delete()
}

func (api *adminAPI) userList(w http.ResponseWriter, r *http.Request) {
	ul := r.Context().Value(ctxUserListKey{}).(userList)
	ul.Items = make([]userItem, len(ul.items))
	for i, item := range ul.items {
		ul.Items[i] = newUserItem(api.srv, r, item, ".")
	}

	api.srv.SendPaginationHeaders(w, r, ul.Pagination)
	api.srv.Render(w, r, http.StatusOK, ul.Items)
}

func (api *adminAPI) userInfo(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(*users.User)
	item := newUserItem(api.srv, r, u, "./..")
	item.Settings = u.Settings

	api.srv.Render(w, r, http.StatusOK, item)
}

func (api *adminAPI) userCreate(w http.ResponseWriter, r *http.Request) {
	f := users.NewUserForm()
	forms.Bind(f, r)
	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	u, err := f.CreateUser()
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}

	w.Header().Set("Location", api.srv.AbsoluteURL(r, ".", fmt.Sprint(u.ID)).String())
	api.srv.TextMessage(w, r, http.StatusCreated, "User created")
}

func (api *adminAPI) userUpdate(w http.ResponseWriter, r *http.Request) {
	f := users.NewUserForm()

	u := r.Context().Value(ctxUserKey{}).(*users.User)
	f.SetUser(u)

	forms.Bind(f, r)
	if !f.IsValid() {
		api.srv.Render(w, r, http.StatusUnprocessableEntity, f)
		return
	}

	updated, err := f.UpdateUser(u)
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}
	api.srv.Render(w, r, http.StatusOK, updated)
}

func (api *adminAPI) userDelete(w http.ResponseWriter, r *http.Request) {
	u := r.Context().Value(ctxUserKey{}).(*users.User)

	err := api.deleteUser(r, u)
	if err == nil {
		api.srv.Status(w, r, http.StatusNoContent)
		return
	}
	if errors.Is(err, errSameUser) {
		api.srv.TextMessage(w, r, http.StatusConflict, err.Error())
		return
	}

	api.srv.Error(w, r, err)
}

type userList struct {
	items      []*users.User
	Pagination server.Pagination
	Items      []userItem
}

type userItem struct {
	ID        int                 `json:"id"`
	Href      string              `json:"href"`
	Created   time.Time           `json:"created"`
	Updated   time.Time           `json:"updated"`
	Username  string              `json:"username"`
	Email     string              `json:"email"`
	Group     string              `json:"group"`
	Settings  *users.UserSettings `json:"settings,omitempty"`
	IsDeleted bool                `json:"is_deleted"`
}

func newUserItem(s *server.Server, r *http.Request, u *users.User, base string) userItem {
	return userItem{
		ID:        u.ID,
		Href:      s.AbsoluteURL(r, base, fmt.Sprint(u.ID)).String(),
		Created:   u.Created,
		Updated:   u.Updated,
		Username:  u.Username,
		Email:     u.Email,
		Group:     u.Group,
		IsDeleted: deleteUserTask.IsRunning(u.ID),
	}
}

func deleteUser(u *users.User) error {
	// Remove user's bookmarks first
	if err := bookmarks.Bookmarks.DeleteUserBookmakrs(u); err != nil {
		return err
	}

	return u.Delete()
}
