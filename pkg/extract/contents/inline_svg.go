// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents

import (
	"bytes"
	"fmt"

	"codeberg.org/readeck/readeck/pkg/extract"
	"github.com/go-shiori/dom"
	"golang.org/x/net/html"
)

// ExtractInlineSVGs is a processor that converts inline SVG to cached resources.
// Each SVG node is saved in the resource cache with a known URL, then the node is replaced
// by an img tag linking to this resource.
func ExtractInlineSVGs(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	m.Log.Debug("extract inline SVG images")

	dom.ForEachNode(dom.QuerySelectorAll(m.Dom, "svg"), func(n *html.Node, i int) {
		// Extract the node content to a buffer, as a standalone SVG file.
		buf := new(bytes.Buffer)
		buf.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
		buf.WriteString("\n")
		err := html.Render(buf, n)
		if err != nil {
			m.Log.Error(err)
			return
		}

		// Push image to extractor's cache.
		src := fmt.Sprintf("http://__resources.cache/%d.svg", i)

		m.Extractor.AddToCache(src, map[string]string{
			"Content-Type": "image/svg+xml",
		}, buf.Bytes())

		// Replace the SVG node by an image.
		imgNode := dom.CreateElement("img")
		dom.SetAttribute(imgNode, "src", src)

		dom.ReplaceChild(n.Parent, imgNode, n)
	})

	return next
}
