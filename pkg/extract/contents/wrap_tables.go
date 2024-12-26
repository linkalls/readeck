// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents

import (
	"log/slog"

	"codeberg.org/readeck/readeck/pkg/extract"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// WrapTables is a processor that wraps table elements inside a "figure" element.
// This helps rendering a table with an overflow on narrow screens.
func WrapTables(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	if m.Dom == nil {
		return next
	}

	nodes := dom.QuerySelectorAll(m.Dom, "table")
	if len(nodes) == 0 {
		return next
	}

	m.Log().Debug("wrap tables", slog.Int("tables", len(nodes)))

	dom.ForEachNode(nodes, func(n *html.Node, _ int) {
		p := n.Parent
		if dom.TagName(p) == "figure" && len(dom.Children(p)) == 1 {
			// If the table is the only direct child of a "figure" element, it's
			// already wrapped.
			return
		}

		e := dom.CreateElement("figure")
		p.InsertBefore(e, n)
		dom.AppendChild(e, n)
	})

	return next
}
