// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package migrations

import (
	"io/fs"

	"github.com/doug-martin/goqu/v9"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// M15wrapTables wraps every table element into a "figure" element.
func M15wrapTables(_ *goqu.TxDatabase, _ fs.FS) error {
	return updateBookmarkZipFiles(updateArchiveHTML(func(top *html.Node) (int, error) {
		nodes := dom.QuerySelectorAll(top, "table")
		if len(nodes) == 0 {
			return 0, nil
		}

		total := 0
		dom.ForEachNode(nodes, func(n *html.Node, _ int) {
			p := n.Parent
			if dom.TagName(p) == "figure" && len(dom.Children(p)) == 1 {
				// If the table is the only direct child of a "figure" element, it's
				// already wrapped.
				return
			}

			total++
			e := dom.CreateElement("figure")
			p.InsertBefore(e, n)
			dom.AppendChild(e, n)
		})

		return total, nil
	}))
}
