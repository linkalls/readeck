// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"fmt"
	"io/fs"
	"math/rand"
	"strings"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// M07migrateBookmarkIDs performs a migration inside bookmark zip files
// It changes the id and href attributes to conform to what the
// extractor does from now on.
func M07migrateBookmarkIDs(_ *goqu.TxDatabase, _ fs.FS) error {
	return updateBookmarkZipFiles(updateArchiveHTML(func(top *html.Node) (changed int, err error) {
		// Set a random prefix for the whole document
		chars := []byte("ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz")
		rand.Shuffle(len(chars), func(i, j int) {
			chars[i], chars[j] = chars[j], chars[i]
		})
		prefix := fmt.Sprintf("%s.%s", chars[0:2], chars[3:7])

		total := 0

		// Update all nodes with an id attribute
		for _, node := range dom.QuerySelectorAll(top, "[id]") {
			if value := dom.GetAttribute(node, "id"); value != "" {
				dom.SetAttribute(node, "id", fmt.Sprintf("%s.%s", prefix, value))
				total++
			}
		}

		// Update all a[name], because we'll update the href="#..." later
		for _, node := range dom.QuerySelectorAll(top, "a[name]") {
			if value := dom.GetAttribute(node, "name"); value != "" {
				dom.SetAttribute(node, "name", fmt.Sprintf("%s.%s", prefix, value))
				total++
			}
		}

		// Update all nodes with an href attribute starting with "#"
		for _, node := range dom.QuerySelectorAll(top, "[href^='#']") {
			if value := strings.TrimPrefix(dom.GetAttribute(node, "href"), "#"); value != "" {
				dom.SetAttribute(node, "href", fmt.Sprintf("#%s.%s", prefix, value))
				total++
			}
		}

		return total, nil
	}))
}
