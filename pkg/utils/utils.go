// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package utils provides simple various utilities
package utils

import (
	"fmt"
	"math"
	"net/url"
	"path"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// ShortText returns a string of maxChars maximum length. It attempts to cut between words
// when possible.
func ShortText(s string, maxChars int) string {
	runes := []rune(strings.TrimSpace(strings.Join(strings.Fields(s), " ")))
	if len(runes) <= maxChars {
		return string(runes)
	}

	res := &strings.Builder{}
	j := 0
	for i, word := range strings.FieldsFunc(s, unicode.IsSpace) {
		j += len(word)
		if j >= maxChars {
			if len(word) > maxChars {
				res.WriteString(word[0:maxChars])
			}
			break
		}
		if i > 0 {
			res.WriteString(" ")
		}
		res.WriteString(word)
	}
	res.WriteString("...")

	return res.String()
}

// ShortURL returns a string of maxChars maximum length where src is a URL. It attempts
// to nicely cut the path parts.
func ShortURL(s string, maxChars int) string {
	src, err := url.Parse(s)
	if err != nil {
		return ShortText(s, maxChars)
	}

	res := &strings.Builder{}
	maxChars = max(5, maxChars-len(src.Hostname()))
	res.WriteString(src.Hostname())
	res.WriteString("/")
	dir, file := path.Split(strings.Trim(src.Path, "/"))
	if len(dir+file) <= maxChars {
		res.WriteString(dir + file)
	} else {
		res.WriteString(".../" + ShortText(file, maxChars))
	}
	return res.String()
}

var (
	lat = []*unicode.RangeTable{unicode.Letter, unicode.Number}
	nop = []*unicode.RangeTable{unicode.Mark, unicode.Sk, unicode.Lm}
)

// Slug replaces each run of characters which are not unicode letters or
// numbers with a single hyphen, except for leading or trailing runes. Letters
// will be stripped of diacritical marks and lowercased. Letter or number
// codepoints that do not have combining marks or a lower-cased variant will
// be passed through unaltered.
func Slug(s string) string {
	buf := make([]rune, 0, len(s))
	dash := false
	for _, r := range norm.NFKD.String(s) {
		switch {
		// unicode 'letters' like mandarin characters pass through
		case unicode.IsOneOf(lat, r):
			buf = append(buf, unicode.ToLower(r))
			dash = true
		case unicode.IsOneOf(nop, r):
			// skip
		case dash:
			buf = append(buf, '-')
			dash = false
		}
	}
	if i := len(buf) - 1; i >= 0 && buf[i] == '-' {
		buf = buf[:i]
	}
	return string(buf)
}

// FormatBytes returns a human readable size in IEC format.
func FormatBytes(s uint64) string {
	if s == 0 {
		return "0 B"
	}

	sizes := []string{"B", "KiB", "MiB", "GiB", "TiB", "PiB", "EiB"}
	e := math.Floor(math.Log(float64(s)) / math.Log(1024))
	suffix := sizes[int(e)]

	f := "%.2f %s"
	if e < 1 {
		f = "%.0f %s"
	}

	return fmt.Sprintf(f, math.Floor(float64(s)/math.Pow(1024, e)*10+0.5)/10, suffix)
}
