// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package utils_test

import (
	"math"
	"strconv"
	"testing"

	"codeberg.org/readeck/readeck/pkg/utils"
	"github.com/stretchr/testify/require"
)

func TestShortText(t *testing.T) {
	tests := []struct {
		Text     string
		Expected string
	}{
		{"abcd", "abcd"},
		{"abcdefghij", "abcdefghij"},
		{"abcd abcd abcde", "abcd abcd..."},
		{"abcde abcde abcde", "abcde..."},
		{"abcdeabcdeabcde", "abcdeabcde..."},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			res := utils.ShortText(test.Text, 10)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestShortURL(t *testing.T) {
	tests := []struct {
		Src      string
		Expected string
	}{
		{"https://example.net/abcd/abcd", "example.net/abcd/abcd"},
		{"https://example.net/abcd/abcd/efgh/ijkl/mnop/qrst/uvw/xyz", "example.net/.../xyz"},
		{"https://example.net/abcd/abcd/verylongpathpart/abcd", "example.net/.../abcd"},
		{"/test", "/test"},
		{"\b/test", "\b/test"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			res := utils.ShortURL(test.Src, 40)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestSlug(t *testing.T) {
	tests := []struct {
		Text     string
		Expected string
	}{
		{"abcd", "abcd"},
		{"abcd efgh _  xyz", "abcd-efgh-xyz"},
		{"c'est intéressant comme ça ?", "c-est-interessant-comme-ca"},
		{"Ogólnie znana teza głosi", "ogolnie-znana-teza-głosi"},
		{"Είναι πλέον κοινά παραδεκτό", "ειναι-πλεον-κοινα-παραδεκτο"},
		{"هناك حقيقة مثبتة منذ", "هناك-حقيقة-مثبتة-منذ"},
		{"🙂 happy 🐈", "happy"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			res := utils.Slug(test.Text)
			require.Equal(t, test.Expected, res)
		})
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		s        uint64
		expected string
	}{
		{0, "0 B"},
		{100, "100 B"},
		{1000, "1000 B"},
		{1024, "1.00 KiB"},
		{1024 * 1024 * 5, "5.00 MiB"},
		{uint64(math.Pow(1024, 3) * 2.1), "2.10 GiB"}, // nolint:staticcheck
		{858492928, "818.70 MiB"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			res := utils.FormatBytes(test.s)
			require.Equal(t, test.expected, res)
		})
	}
}

func TestNormalize(t *testing.T) {
	tests := []struct {
		text     string
		expected string
	}{
		{" abc ", "abc"},
		{"   abc  \n ", "abc"},
		{"ab\t c\t\n🙂 勤", "ab c 🙂 勤"},
		{"ab\t c\t\n🙂 勤", "ab c 🙂 勤"},
		{"\n국교는 인정되지 \n\t 아니하며.   대법원장과  대법관이\n\n", "국교는 인정되지 아니하며. 대법원장과 대법관이"},
	}

	for i, test := range tests {
		t.Run(strconv.Itoa(i+1), func(t *testing.T) {
			res := utils.NormalizeSpaces(test.text)
			require.Equal(t, test.expected, res)
		})
	}
}
