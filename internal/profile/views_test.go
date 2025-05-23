// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package profile_test

import (
	"encoding/json"
	"fmt"
	"net/url"
	"path"
	"testing"

	"github.com/doug-martin/goqu/v9"
	"github.com/stretchr/testify/require"

	"codeberg.org/readeck/readeck/internal/auth/tokens"
	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestViews(t *testing.T) {
	app := NewTestApp(t)
	defer func() {
		app.Close(t)
	}()

	client := NewClient(t, app)

	t.Run("profile", func(t *testing.T) {
		RunRequestSequence(t, client, "user",
			RequestTest{Target: "/profile", ExpectStatus: 200},
			RequestTest{
				Method: "POST",
				Target: "/profile",
				Form: url.Values{
					"username": {"user@localhost"},
					"email":    {"user"},
				},
				ExpectStatus: 422,
				Assert: func(t *testing.T, r *Response) {
					require.Contains(t, string(r.Body), "must contain English letters")
					require.Contains(t, string(r.Body), "not a valid email address")
				},
			},
			RequestTest{Target: "/profile", ExpectStatus: 200},
			RequestTest{
				Method: "POST",
				Target: "/profile",
				Form: url.Values{
					"username": {"user"},
				},
				ExpectStatus:   303,
				ExpectRedirect: "/profile",
			},
			RequestTest{Target: "/profile", ExpectStatus: 200},
			RequestTest{
				Method: "POST",
				Target: "/profile",
				Form: url.Values{
					"username": {"   "},
				},
				ExpectStatus: 422,
			},
			RequestTest{Target: "/profile"},
			RequestTest{
				Method: "POST",
				Target: "/profile",
				Form: url.Values{
					"username": {"user"},
					"email":    {"invalid"},
				},
				ExpectStatus: 422,
			},
			RequestTest{Target: "/profile"},
			RequestTest{
				Method: "POST",
				Target: "/profile/session",
				Form: url.Values{
					"bookmark_list_display": {"grid"},
				},
				ExpectStatus: 200,
				ExpectJSON: `{
					"bookmark_list_display":"grid"
				}`,
			},
			RequestTest{Target: "/profile"},
			RequestTest{
				Method: "POST",
				Target: "/profile/session",
				Form: url.Values{
					"bookmark_list_display": {"something"},
				},
				ExpectStatus: 422,
			},
		)
	})

	t.Run("password", func(t *testing.T) {
		defer func() {
			if err := app.Users["user"].User.SetPassword("user"); err != nil {
				t.Logf("error updating password: %s", err)
			}
		}()

		RunRequestSequence(t, client, "user",
			RequestTest{Target: "/profile/password", ExpectStatus: 200},
			RequestTest{
				Method: "POST",
				Target: "/profile/password",
				Form: url.Values{
					"current":  {"user"},
					"password": {"user1234"},
				},
				ExpectStatus:   303,
				ExpectRedirect: "/profile/password",
			},
			// The session has been updated, we can still use the website
			RequestTest{Target: "/profile", ExpectStatus: 200},
		)
	})

	t.Run("tokens", func(t *testing.T) {
		RunRequestSequence(t, client, "staff",
			RequestTest{Target: "/profile/tokens", ExpectStatus: 200},
			RequestTest{
				Method:         "POST",
				Target:         "/profile/tokens",
				ExpectStatus:   303,
				ExpectRedirect: "/profile/tokens/.+",
			},
			RequestTest{
				Target:         "{{ (index .History 0).Redirect }}",
				ExpectStatus:   200,
				ExpectContains: "New token created",
			},
			RequestTest{
				Method:         "POST",
				Target:         "{{ (index .History 0).Path }}",
				Form:           url.Values{"application": {"test"}},
				ExpectStatus:   303,
				ExpectRedirect: "/profile/tokens/.+",
			},

			// Delete token
			RequestTest{Target: "{{ (index .History 0).Redirect }}"},
			RequestTest{
				Method:         "POST",
				Target:         "{{ (index .History 0).Path }}/delete",
				ExpectStatus:   303,
				ExpectRedirect: "/profile/tokens",
			},
			RequestTest{ //nolint:dupl
				Target:         "{{ (index .History 1).Path }}",
				ExpectStatus:   200,
				ExpectContains: "Token will be removed in a few seconds",
				Assert: func(t *testing.T, r *Response) {
					assert := require.New(t)

					_, tokenID := path.Split(r.URL.Path)
					token, err := tokens.Tokens.GetOne(goqu.C("uid").Eq(tokenID))
					if err != nil {
						t.Error(err)
					}

					// An event was sent
					assert.Len(Events().Records("task"), 1)
					evt := map[string]interface{}{}
					assert.NoError(json.Unmarshal(Events().Records("task")[0], &evt))
					assert.Equal("token.delete", evt["name"])
					assert.InDelta(float64(token.ID), evt["id"], 0)

					// There's a task in the store
					task := fmt.Sprintf("tasks:token.delete:%d", token.ID)
					m := Store().Get(task)
					assert.NotEmpty(m)

					payload := map[string]interface{}{}
					assert.NoError(json.Unmarshal([]byte(m), &payload))
					assert.InDelta(float64(20), payload["delay"], 0)
				},
			},

			// Cancel deletion
			RequestTest{
				Target: "{{ (index .History 0).Path }}",
			},
			RequestTest{
				Method:         "POST",
				Target:         "{{ (index .History 0).Path }}/delete",
				Form:           url.Values{"cancel": {"1"}},
				ExpectStatus:   303,
				ExpectRedirect: "/profile/tokens",
			},
			RequestTest{
				Target: "{{ (index .History 1).Path }}",
				Assert: func(t *testing.T, r *Response) {
					_, tokenID := path.Split(r.URL.Path)
					token, err := tokens.Tokens.GetOne(goqu.C("uid").Eq(tokenID))
					if err != nil {
						t.Error(err)
					}

					// The task is not in the store anymore
					task := fmt.Sprintf("tasks:token.delete:%d", token.ID)
					m := Store().Get(task)
					require.Empty(t, m)
				},
			},
		)
	})
}
