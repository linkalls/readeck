// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package sessions_test

import (
	"crypto/rand"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/sessions"
	"codeberg.org/readeck/readeck/pkg/http/securecookie"
)

func TestSession(t *testing.T) {
	k := make([]byte, 32)
	rand.Read(k)
	h := securecookie.NewHandler(securecookie.Key(k))

	t.Run("new session", func(t *testing.T) {
		assert := require.New(t)

		r := httptest.NewRequest("GET", "/", nil)
		session, err := sessions.New(h, r)
		assert.ErrorIs(err, http.ErrNoCookie)
		assert.True(session.IsNew)
	})

	t.Run("load session", func(t *testing.T) {
		assert := require.New(t)

		r := httptest.NewRequest("GET", "/test", nil)
		w := httptest.NewRecorder()
		session, err := sessions.New(h, r)
		assert.ErrorIs(err, http.ErrNoCookie)
		assert.True(session.IsNew)

		session.Payload.User = 2
		session.AddFlash("info", "woot")
		assert.NoError(session.Save(w, r))
		assert.Len(w.Result().Cookies(), 1)

		cookie := w.Result().Cookies()[0]

		// Load the session
		r = httptest.NewRequest("GET", "/test", nil)
		r.AddCookie(cookie)
		session, err = sessions.New(h, r)
		assert.NoError(err)
		assert.False(session.IsNew)
		assert.Equal(2, session.Payload.User)
		assert.Len(session.Payload.Flashes, 1)
	})

	t.Run("flash", func(t *testing.T) {
		assert := require.New(t)

		r := httptest.NewRequest("GET", "/", nil)
		session, err := sessions.New(h, r)
		assert.ErrorIs(err, http.ErrNoCookie)
		session.AddFlash("info", "woot")

		assert.Len(session.Payload.Flashes, 1)
		flashes := session.Flashes()
		assert.Empty(session.Payload.Flashes)
		assert.Len(flashes, 1)

		assert.Equal([]sessions.FlashMessage{
			{"info", "woot"},
		}, flashes)
	})
}
