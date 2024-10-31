// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package preferences provides a struct with methods to get some user preferences
// values. They can come from the user's model or from the session.
package preferences

import (
	"fmt"
	"slices"
	"strconv"

	"codeberg.org/readeck/readeck/internal/auth/users"
	"codeberg.org/readeck/readeck/internal/sessions"
)

var (
	readerFontList = [][2]string{
		{"lora", "bookmark-display--font-lora"},
		{"public-sans", "bookmark-display--font-public-sans"},
		{"merriweather", "bookmark-display--font-merriweather"},
		{"inter", "bookmark-display--font-inter"},
		{"plex-serif", "bookmark-display--font-plex-serif"},
		{"literata", "bookmark-display--font-literata"},
		{"luciole", "bookmark-display--font-luciole"},
		{"atkinson-hyperlegible", "bookmark-display--font-atkinson-hyperlegible"},
		{"jetbrains-mono", "bookmark-display--font-jetbrains-mono"},
	}
	readerFontSizes = []string{
		"bookmark-display--size-1",
		"bookmark-display--size-2",
		"bookmark-display--size-3",
		"bookmark-display--size-4",
		"bookmark-display--size-5",
		"bookmark-display--size-6",
	}
	readerLineHeights = []string{
		"bookmark-display--leading-1",
		"bookmark-display--leading-2",
		"bookmark-display--leading-3",
		"bookmark-display--leading-4",
		"bookmark-display--leading-5",
		"bookmark-display--leading-6",
	}
	bookmarkListLayouts = []string{
		"grid",
		"compact",
	}
)

// Pair holds a value and class name pair.
type Pair struct {
	Value string
	Class string
}

// Preferences contains the user preferences after initilization.
type Preferences struct {
	user                  *users.User
	session               *sessions.Session
	idxReaderFont         int
	idxReaderFontSize     int
	idxReaderLineHeight   int
	idxRookmarkListLayout int
}

// New returns a new Preferences instance.
// It computes the values for each preference at this moment.
func New(user *users.User, session *sessions.Session) *Preferences {
	p := &Preferences{
		user:                  user,
		session:               session,
		idxReaderFont:         0,
		idxReaderFontSize:     2,
		idxReaderLineHeight:   2,
		idxRookmarkListLayout: 0,
	}

	if user != nil && user.Settings != nil {
		if idx := slices.IndexFunc(readerFontList, func(e [2]string) bool {
			return e[0] == user.Settings.ReaderSettings.Font
		}); idx >= 0 {
			p.idxReaderFont = idx
		}

		if idx := user.Settings.ReaderSettings.FontSize - 1; idx >= 0 && idx < len(readerFontSizes) {
			p.idxReaderFontSize = idx
		}

		if idx := user.Settings.ReaderSettings.LineHeight - 1; idx >= 0 && idx < len(readerLineHeights) {
			p.idxReaderLineHeight = idx
		}
	}

	if session != nil {
		if idx := slices.Index(bookmarkListLayouts, session.Payload.Preferences.BookmarkListDisplay); idx >= 0 {
			p.idxRookmarkListLayout = idx
		}
	}

	return p
}

// FontList returns the available font faces.
func (p *Preferences) FontList() [][2]string {
	return readerFontList
}

// FontSizes returns the available sizes.
func (p *Preferences) FontSizes() []string {
	return readerFontSizes
}

// LineHeights returns the available line heights.
func (p *Preferences) LineHeights() []string {
	return readerLineHeights
}

// ReaderFont returns the user's reader font.
func (p *Preferences) ReaderFont() Pair {
	return Pair{
		readerFontList[p.idxReaderFont][0],
		readerFontList[p.idxReaderFont][1],
	}
}

// ReaderFontSize returns the user's reader font size.
func (p *Preferences) ReaderFontSize() Pair {
	return Pair{
		strconv.Itoa(p.idxReaderFontSize + 1),
		readerFontSizes[p.idxReaderFontSize],
	}
}

// ReaderLineHeight returns the user's reader line height.
func (p *Preferences) ReaderLineHeight() Pair {
	return Pair{
		strconv.Itoa(p.idxReaderLineHeight + 1),
		readerLineHeights[p.idxReaderLineHeight],
	}
}

// BookmarkListLayout returns the bookmark list layout.
func (p *Preferences) BookmarkListLayout() Pair {
	return Pair{
		bookmarkListLayouts[p.idxRookmarkListLayout],
		fmt.Sprintf("bookmark-list--%s", bookmarkListLayouts[p.idxRookmarkListLayout]),
	}
}
