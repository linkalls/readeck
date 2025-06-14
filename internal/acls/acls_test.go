// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package acls_test

import (
	"fmt"
	"strings"
	"testing"

	"codeberg.org/readeck/readeck/internal/acls"
	"github.com/stretchr/testify/require"
)

func TestCheckPermission(t *testing.T) {
	tests := []struct {
		Group    string
		Obj      string
		Act      string
		Expected bool
	}{
		{"admin", "api:profile", "read", true},
		{"staff", "api:profile", "read", true},
		{"user", "api:profile", "read", true},
		{"", "api:profile", "read", false},

		{"admin", "api:profile:tokens", "delete", true},
		{"staff", "api:profile:tokens", "delete", true},
		{"user", "api:profile:tokens", "delete", true},
		{"", "api:profile:tokens", "delete", false},

		{"admin", "system", "read", true},
		{"staff", "system", "read", true},
		{"user", "system", "read", false},
		{"", "system", "read", false},

		{"admin", "api:admin:users", "read", true},
		{"staff", "api:admin:users", "read", false},
		{"user", "api:admin:users", "read", false},
		{"", "api:admin:users", "read", false},

		{"admin", "admin:users", "read", true},
		{"staff", "admin:users", "read", false},
		{"user", "admin:users", "read", false},
		{"", "admin:users", "read", false},

		{"admin", "bookmarks", "read", true},
		{"staff", "bookmarks", "read", true},
		{"user", "bookmarks", "read", true},
		{"", "bookmarks", "read", false},

		{"admin", "email", "send", true},
		{"staff", "email", "send", true},
		{"user", "email", "send", true},
		{"", "email", "send", true},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s-%s-%s", test.Group, test.Obj, test.Act), func(t *testing.T) {
			res, err := acls.Check(test.Group, test.Obj, test.Act)
			require.NoError(t, err)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestGetPermissions(t *testing.T) {
	tests := []struct {
		Groups   []string
		Expected []string
	}{
		{
			[]string{"scoped_admin_r"},
			[]string{"api:admin:users:read", "api:profile:read", "api:profile:tokens:delete", "system:read"},
		},
		{
			[]string{"scoped_admin_w"},
			[]string{"api:admin:users:write", "api:profile:read", "api:profile:tokens:delete"},
		},
		{
			[]string{"scoped_admin_r", "scoped_admin_w"},
			[]string{"api:admin:users:read", "api:admin:users:write", "api:profile:read", "api:profile:tokens:delete", "system:read"},
		},
		{
			[]string{"scoped_bookmarks_r"},
			[]string{"api:bookmarks:collections:read", "api:bookmarks:export", "api:bookmarks:read", "api:opds:read", "api:profile:read", "api:profile:tokens:delete", "bookmarks:read"},
		},
		{
			[]string{"scoped_bookmarks_w"},
			[]string{"api:bookmarks:collections:write", "api:bookmarks:write", "api:profile:read", "api:profile:tokens:delete"},
		},
		{
			[]string{"unknown"},
			[]string{},
		},
	}

	for _, test := range tests {
		t.Run(strings.Join(test.Groups, ","), func(t *testing.T) {
			res, err := acls.GetPermissions(test.Groups...)
			require.NoError(t, err)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestInGroup(t *testing.T) {
	tests := []struct {
		Src      string
		Dest     string
		Expected bool
	}{
		{"user", "user", true},
		{"user", "admin", true},
		{"scoped_bookmarks_r", "user", true},
		{"scoped_admin_r", "user", false},
		{"scoped_admin_r", "admin", true},
	}

	for _, test := range tests {
		t.Run(fmt.Sprintf("%s in %s", test.Src, test.Dest), func(t *testing.T) {
			res := acls.InGroup(test.Src, test.Dest)
			require.Equal(t, test.Expected, res)
		})
	}
}
