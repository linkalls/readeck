// SPDX-FileCopyrightText: © 2020 Radhi Fadlillah
//
// SPDX-License-Identifier: MIT

package archiver

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
	"golang.org/x/sync/errgroup"

	"github.com/antchfx/htmlquery"
	"github.com/go-shiori/dom"
)

var (
	rxLazyImageSrc    = regexp.MustCompile(`(?i)^\s*\S+\.(jpg|jpeg|png|webp)\S*\s*$`)
	rxLazyImageSrcset = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|webp)\s+\d`)
	rxImgExtensions   = regexp.MustCompile(`(?i)\.(jpg|jpeg|png|webp)`)
	rxSrcsetURL       = regexp.MustCompile(`(?i)(\S+)(\s+[\d.]+[xw])?(\s*(?:,|$))`)
	rxB64DataURL      = regexp.MustCompile(`(?i)^data:\s*([^\s;,]+)\s*;\s*base64\s*`)
)

func (arc *Archiver) processHTML(ctx context.Context, input io.Reader, baseURL *url.URL) (string, error) {
	// Parse input into HTML document
	doc, err := html.Parse(input)
	if err != nil {
		return "", err
	}

	arc.SendEvent(ctx, EventStartHTML(arc.Request.URL.String()))

	// Prepare documents by doing these steps :
	// - Set charset
	// - Set Content-Security-Policy to make sure no unwanted request happened
	// - Apply configuration to documents
	// - Replace all noscript to divs, to make it processed as well
	// - Remove all comments in documents
	// - Convert data-src and data-srcset attribute in lazy image to src and srcset
	// - Convert relative URL into absolute URL
	// - Remove subresources integrity attribute from links
	arc.setCharset(doc)
	arc.setContentSecurityPolicy(doc)
	arc.applyConfiguration(doc)
	arc.convertNoScriptToDiv(doc, true)
	arc.removeComments(doc)
	arc.convertLazyImageAttrs(doc)
	arc.convertRelativeURLs(doc, baseURL)
	arc.removeLinkIntegrityAttr(doc)

	// Find all nodes which might has subresource.
	// A node might has subresource if it fulfills one of these criteria :
	// - It has inline style;
	// - It's link for icon or stylesheets;
	// - It's tag name is either style, img, picture, figure, video, audio, source, iframe or object;
	resourceNodes := make(map[*html.Node]struct{})
	for _, node := range dom.GetElementsByTagName(doc, "*") {
		if style := dom.GetAttribute(node, "style"); strings.TrimSpace(style) != "" {
			resourceNodes[node] = struct{}{}
			continue
		}

		switch dom.TagName(node) {
		case "link":
			rel := dom.GetAttribute(node, "rel")
			if strings.Contains(rel, "icon") || strings.Contains(rel, "stylesheet") {
				resourceNodes[node] = struct{}{}
			}

		case "iframe", "embed", "object", "style", "script",
			"img", "picture", "figure", "video", "audio", "source":
			resourceNodes[node] = struct{}{}
		}
	}

	// Process each node concurrently
	g, ctx := errgroup.WithContext(ctx)
	for node := range resourceNodes {
		g.Go(func() error {
			// Update style attribute
			if dom.HasAttribute(node, "style") {
				err := arc.processStyleAttr(ctx, node, baseURL)
				if err != nil {
					return err
				}
			}

			// Update node depending on its tag name
			switch dom.TagName(node) {
			case "style":
				return arc.processStyleNode(ctx, node, baseURL)
			case "link":
				return arc.processLinkNode(ctx, node, baseURL)
			case "script":
				return arc.processScriptNode(ctx, node, baseURL)
			case "object", "embed", "iframe":
				return arc.processEmbedNode(ctx, node, baseURL)
			case "img", "picture", "figure", "video", "audio", "source":
				return arc.processMediaNode(ctx, node, baseURL)
			default:
				return nil
			}
		})
	}

	// Wait until all resources processed
	if err = g.Wait(); err != nil {
		return "", err
	}

	// Revert the converted noscripts
	arc.revertConvertedNoScript(doc)

	// Remove data attributes
	if arc.Flags&EnableDataAttributes == 0 {
		arc.removeDataAttributes(doc)
	}

	// Set all image to be lazy
	arc.setLazyImages(doc)

	// Convert document back to string
	return dom.OuterHTML(doc), nil
}

// setCharset adds a meta charset tag to the document.
func (arc *Archiver) setCharset(doc *html.Node) {
	nodes := dom.GetElementsByTagName(doc, "head")
	var head *html.Node
	if len(nodes) == 0 {
		head = dom.CreateElement("head")
		dom.AppendChild(doc, head)
	} else {
		head = nodes[0]
	}

	// Change existing charset if any
	for _, meta := range dom.GetElementsByTagName(head, "meta") {
		if dom.GetAttribute(meta, "charset") != "" {
			dom.SetAttribute(meta, "charset", "utf-8")
			return
		}
	}

	// None found, create one
	meta := dom.CreateElement("meta")
	dom.SetAttribute(meta, "charset", "utf-8")
	dom.AppendChild(head, meta)
}

// setContentSecurityPolicy prevent browsers from requesting any remote
// resources by setting Content-Security-Policy to only allow from
// inline element and data URL.
func (arc *Archiver) setContentSecurityPolicy(doc *html.Node) {
	// Remove existing CSP
	for _, meta := range dom.GetElementsByTagName(doc, "meta") {
		httpEquiv := dom.GetAttribute(meta, "http-equiv")
		if httpEquiv == "Content-Security-Policy" {
			meta.Parent.RemoveChild(meta)
		}
	}

	// Prepare list of CSP
	policies := []string{
		"default-src 'self' 'unsafe-inline' data:;",
		"connect-src 'none';",
	}

	if arc.Flags&EnableJS == 0 {
		policies = append(policies, "script-src 'none';")
	}

	if arc.Flags&EnableCSS == 0 {
		policies = append(policies, "style-src 'none';")
	}

	if arc.Flags&EnableEmbeds == 0 {
		policies = append(policies, "frame-src 'none'; child-src 'none';")
	}

	if arc.Flags&EnableImages == 0 {
		policies = append(policies, "image-src 'none';")
	}

	if arc.Flags&EnableMedia == 0 {
		policies = append(policies, "media-src 'none';")
	}

	// Find the head, create it if necessary
	heads := dom.GetElementsByTagName(doc, "head")
	if len(heads) == 0 {
		newHead := dom.CreateElement("head")
		dom.PrependChild(doc, newHead)
		heads = []*html.Node{newHead}
	}

	// Put the new CSP
	for i := len(policies) - 1; i >= 0; i-- {
		meta := dom.CreateElement("meta")
		dom.SetAttribute(meta, "http-equiv", "Content-Security-Policy")
		dom.SetAttribute(meta, "content", policies[i])
		dom.PrependChild(heads[0], meta)
	}
}

// applyConfiguration removes or replace elements following the configuration.
func (arc *Archiver) applyConfiguration(doc *html.Node) {
	if arc.Flags&EnableJS == 0 {
		// Remove script tags
		scripts := dom.GetAllNodesWithTag(doc, "script")
		dom.RemoveNodes(scripts, nil)

		// Remove links with javascript URL scheme
		for _, a := range dom.GetElementsByTagName(doc, "a") {
			href := dom.GetAttribute(a, "href")
			if strings.HasPrefix(href, "javascript:") {
				dom.SetAttribute(a, "href", "#")
			}
		}

		// Convert noscript to div
		arc.convertNoScriptToDiv(doc, false)
	}

	if arc.Flags&EnableCSS == 0 {
		// Remove style tags
		styles := dom.GetAllNodesWithTag(doc, "style")
		dom.RemoveNodes(styles, nil)

		// Remove inline style
		for _, node := range dom.GetElementsByTagName(doc, "*") {
			if dom.HasAttribute(node, "style") {
				dom.RemoveAttribute(node, "style")
			}
		}

		// Remove style links
		nodes := dom.GetAllNodesWithTag(doc, "link")
		dom.RemoveNodes(nodes, func(n *html.Node) bool {
			return dom.GetAttribute(n, "rel") == "stylesheet"
		})
	}

	if arc.Flags&EnableEmbeds == 0 {
		embeds := dom.GetAllNodesWithTag(doc, "object", "embed", "iframe")
		dom.RemoveNodes(embeds, nil)
	}

	if arc.Flags&EnableImages == 0 {
		images := dom.GetAllNodesWithTag(doc, "img", "picture")
		dom.RemoveNodes(images, nil)
	}

	if arc.Flags&EnableMedia == 0 {
		medias := dom.GetAllNodesWithTag(doc, "video", "audio", "source")
		dom.RemoveNodes(medias, nil)
	}
}

// convertNoScriptToDiv convert all noscript to div element.
func (arc *Archiver) convertNoScriptToDiv(doc *html.Node, markNewDiv bool) {
	noscripts := dom.GetElementsByTagName(doc, "noscript")
	dom.ForEachNode(noscripts, func(noscript *html.Node, _ int) {
		// Parse noscript content
		noscriptContent := dom.TextContent(noscript)
		tmpDoc, err := html.Parse(strings.NewReader(noscriptContent))
		if err != nil {
			return
		}
		tmpBody := dom.GetElementsByTagName(tmpDoc, "body")[0]

		// Create new div to contain noscript content
		div := dom.CreateElement("div")
		for _, child := range dom.ChildNodes(tmpBody) {
			dom.AppendChild(div, child)
		}

		// If needed, create attribute to mark it was noscript
		if markNewDiv {
			dom.SetAttribute(div, "data-obelisk-noscript", "true")
		}

		// Replace noscript with our new div
		dom.ReplaceChild(noscript.Parent, div, noscript)
	})
}

// removeComments find all comments in document then remove it.
func (arc *Archiver) removeComments(doc *html.Node) {
	// Find all comments
	var comments []*html.Node
	var finder func(*html.Node)

	finder = func(node *html.Node) {
		if node.Type == html.CommentNode {
			comments = append(comments, node)
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			finder(child)
		}
	}

	for child := doc.FirstChild; child != nil; child = child.NextSibling {
		finder(child)
	}

	// Remove it
	dom.RemoveNodes(comments, nil)
}

// convertLazyImageAttrs convert attributes data-src and data-srcset
// which often found in lazy-loaded images and pictures, into basic attribute
// src and srcset, so images that can be loaded without JS.
func (arc *Archiver) convertLazyImageAttrs(doc *html.Node) {
	imageNodes := dom.GetAllNodesWithTag(doc, "img", "picture", "figure")
	dom.ForEachNode(imageNodes, func(elem *html.Node, _ int) {
		src := dom.GetAttribute(elem, "src")
		srcset := dom.GetAttribute(elem, "srcset")
		nodeTag := dom.TagName(elem)
		nodeClass := dom.ClassName(elem)

		// In some sites (e.g. Kotaku), they put 1px square image as data uri in
		// the src attribute. So, here we check if the data uri is too short,
		// just might as well remove it.
		if src != "" && rxB64DataURL.MatchString(src) {
			// Make sure it's not SVG, because SVG can have a meaningful image
			// in under 133 bytes.
			parts := rxB64DataURL.FindStringSubmatch(src)
			if parts[1] == "image/svg+xml" {
				return
			}

			// Make sure this element has other attributes which contains
			// image. If it doesn't, then this src is important and
			// shouldn't be removed.
			srcCouldBeRemoved := false
			for _, attr := range elem.Attr {
				if attr.Key == "src" {
					continue
				}

				if rxImgExtensions.MatchString(attr.Val) && isValidURL(attr.Val) {
					srcCouldBeRemoved = true
					break
				}
			}

			// Here we assume if image is less than 100 bytes (or 133B
			// after encoded to base64) it will be too small, therefore
			// it might be placeholder image.
			if srcCouldBeRemoved {
				b64starts := strings.Index(src, "base64") + 7
				b64length := len(src) - b64starts
				if b64length < 133 {
					src = ""
					dom.RemoveAttribute(elem, "src")
				}
			}
		}

		if (src != "" || srcset != "") && !strings.Contains(strings.ToLower(nodeClass), "lazy") {
			return
		}

		for i := 0; i < len(elem.Attr); i++ {
			attr := elem.Attr[i]
			if attr.Key == "src" || attr.Key == "srcset" {
				continue
			}

			copyTo := ""
			if rxLazyImageSrcset.MatchString(attr.Val) {
				copyTo = "srcset"
			} else if rxLazyImageSrc.MatchString(attr.Val) {
				copyTo = "src"
			}

			if copyTo == "" || !isValidURL(attr.Val) {
				continue
			}

			if nodeTag == "img" || nodeTag == "picture" {
				// if this is an img or picture, set the attribute directly
				dom.SetAttribute(elem, copyTo, attr.Val)
			} else if nodeTag == "figure" && len(dom.GetAllNodesWithTag(elem, "img", "picture")) == 0 {
				// if the item is a <figure> that does not contain an image or picture,
				// create one and place it inside the figure see the nytimes-3
				// testcase for an example
				img := dom.CreateElement("img")
				dom.SetAttribute(img, copyTo, attr.Val)
				dom.AppendChild(elem, img)
			}

			// Since the attribute already copied, just remove it
			dom.RemoveAttribute(elem, attr.Key)
		}
	})
}

// convertRelativeURLs converts all relative URL in document into absolute URL.
// We do this for a, img, picture, figure, video, audio, source, link,
// embed, iframe and object.
func (arc *Archiver) convertRelativeURLs(doc *html.Node, baseURL *url.URL) {
	// Prepare nodes and methods
	as := dom.GetElementsByTagName(doc, "a")
	links := dom.GetElementsByTagName(doc, "link")
	embeds := dom.GetElementsByTagName(doc, "embed")
	scripts := dom.GetElementsByTagName(doc, "script")
	iframes := dom.GetElementsByTagName(doc, "iframe")
	objects := dom.GetElementsByTagName(doc, "object")
	medias := dom.GetAllNodesWithTag(doc, "img", "picture", "figure", "video", "audio", "source")

	convertNode := func(node *html.Node, attrName string) {
		if dom.HasAttribute(node, attrName) {
			val := dom.GetAttribute(node, attrName)
			newVal := createAbsoluteURL(val, baseURL)
			dom.SetAttribute(node, attrName, newVal)
		}
	}

	convertNodes := func(nodes []*html.Node, attrName string) {
		for _, node := range nodes {
			convertNode(node, attrName)
		}
	}

	// Convert all relative URLs
	convertNodes(as, "href")
	convertNodes(links, "href")
	convertNodes(embeds, "src")
	convertNodes(scripts, "src")
	convertNodes(iframes, "src")
	convertNodes(objects, "data")

	for _, media := range medias {
		convertNode(media, "src")
		convertNode(media, "poster")

		if srcset := dom.GetAttribute(media, "srcset"); srcset != "" {
			newSrcset := rxSrcsetURL.ReplaceAllStringFunc(srcset, func(s string) string {
				p := rxSrcsetURL.FindStringSubmatch(s)
				return createAbsoluteURL(p[1], baseURL) + p[2] + p[3]
			})
			dom.SetAttribute(media, "srcset", newSrcset)
		}
	}
}

// removeLinkIntegrityAttrs removes integrity attributes from link tags.
func (arc *Archiver) removeLinkIntegrityAttr(doc *html.Node) {
	for _, link := range dom.GetElementsByTagName(doc, "link") {
		dom.RemoveAttribute(link, "integrity")
	}
}

// removeDataAttributes removes all "data-" attributes from all tags.
func (arc *Archiver) removeDataAttributes(doc *html.Node) {
	nodes, err := htmlquery.QueryAll(doc, "//*[@*[starts-with(name(), 'data-')]]")
	if err != nil {
		return
	}

	dom.ForEachNode(nodes, func(node *html.Node, _ int) {
		keys := []string{}
		for _, attr := range node.Attr {
			if strings.HasPrefix(attr.Key, "data-") {
				keys = append(keys, attr.Key)
			}
		}
		for _, key := range keys {
			dom.RemoveAttribute(node, key)
		}
	})
}

// setLazyImages adds a loading="lazy" attribute to every image
// in the document.
func (arc *Archiver) setLazyImages(doc *html.Node) {
	for _, node := range dom.GetElementsByTagName(doc, "img") {
		dom.SetAttribute(node, "loading", "lazy")
	}
}

func (arc *Archiver) processURLNode(ctx context.Context, node *html.Node, attrName string, baseURL *url.URL) error {
	if !dom.HasAttribute(node, attrName) {
		return nil
	}

	uri := dom.GetAttribute(node, attrName)
	headers := http.Header{}
	switch node.Data {
	case "img", "picture", "source":
		headers.Set("Accept", "image/webp,image/svg+xml,image/*,*/*;q=0.8")
	}

	content, contentType, err := arc.processURL(ctx, uri, baseURL.String(), headers)
	if err != nil && err != errSkippedURL {
		arc.SendEvent(ctx, &EventError{Err: err, URI: uri})
		return err
	}

	newURL := uri
	if err == nil {
		newURL = arc.URLProcessor(uri, content, contentType)
	}

	dom.SetAttribute(node, attrName, newURL)
	return nil
}

func (arc *Archiver) processStyleAttr(ctx context.Context, node *html.Node, baseURL *url.URL) error {
	style := dom.GetAttribute(node, "style")
	newStyle, err := arc.processCSS(ctx, strings.NewReader(style), baseURL)
	if err == nil {
		dom.SetAttribute(node, "style", newStyle)
	}

	return err
}

func (arc *Archiver) processStyleNode(ctx context.Context, node *html.Node, baseURL *url.URL) error {
	style := dom.TextContent(node)
	newStyle, err := arc.processCSS(ctx, strings.NewReader(style), baseURL)
	if err == nil {
		dom.SetTextContent(node, newStyle)
	}

	return err
}

func (arc *Archiver) processLinkNode(ctx context.Context, node *html.Node, baseURL *url.URL) error {
	if !dom.HasAttribute(node, "href") {
		return nil
	}

	if rel := dom.GetAttribute(node, "rel"); strings.Contains(rel, "icon") {
		return arc.processURLNode(ctx, node, "href", baseURL)
	}

	uri := dom.GetAttribute(node, "href")
	content, _, err := arc.processURL(ctx, uri, baseURL.String(), nil)
	if err != nil {
		if err == errSkippedURL {
			return nil
		}
		return err
	}

	// Remove all attributes for this node
	for i := len(node.Attr) - 1; i >= 0; i-- {
		dom.RemoveAttribute(node, node.Attr[i].Key)
	}

	// Convert <link> into <style>
	node.Data = "style"
	dom.SetAttribute(node, "type", "text/css")
	dom.SetTextContent(node, string(content))
	return nil
}

func (arc *Archiver) processScriptNode(ctx context.Context, node *html.Node, baseURL *url.URL) error {
	if !dom.HasAttribute(node, "src") {
		return nil
	}

	uri := dom.GetAttribute(node, "src")
	content, _, err := arc.processURL(ctx, uri, baseURL.String(), nil)
	if err != nil {
		if err == errSkippedURL {
			return nil
		}
		return err
	}

	dom.RemoveAttribute(node, "src")
	dom.SetTextContent(node, string(content))
	return nil
}

func (arc *Archiver) processEmbedNode(ctx context.Context, node *html.Node, baseURL *url.URL) error {
	attrName := "src"
	if dom.TagName(node) == "object" {
		attrName = "data"
	}

	if !dom.HasAttribute(node, attrName) {
		return nil
	}

	uri := dom.GetAttribute(node, attrName)
	content, contentType, err := arc.processURL(ctx, uri, baseURL.String(), nil)
	if err != nil && err != errSkippedURL {
		return err
	}

	newURL := uri
	if err == nil {
		newURL = arc.URLProcessor(uri, content, contentType)
	}

	dom.SetAttribute(node, attrName, newURL)
	return nil
}

func (arc *Archiver) processMediaNode(ctx context.Context, node *html.Node, baseURL *url.URL) error {
	ctx = context.WithValue(ctx, ctxNodeKey{}, node)

	err := arc.processURLNode(ctx, node, "src", baseURL)
	if err != nil {
		dom.RemoveAttribute(node, "src")
	}

	err = arc.processURLNode(ctx, node, "poster", baseURL)
	if err != nil {
		dom.RemoveAttribute(node, "poster")
	}

	if !dom.HasAttribute(node, "srcset") {
		return nil
	}

	var newSets []string
	srcset := dom.GetAttribute(node, "srcset")
	for _, parts := range rxSrcsetURL.FindAllStringSubmatch(srcset, -1) {
		oldURL := parts[1]
		targetWidth := parts[2]

		content, contentType, err := arc.processURL(ctx, oldURL, baseURL.String(), nil)
		if err != nil && err != errSkippedURL {
			arc.SendEvent(ctx, &EventError{Err: err, URI: oldURL})
			continue
		}

		newSet := oldURL
		if err == nil {
			newSet = arc.URLProcessor(oldURL, content, contentType)
		}

		newSet += targetWidth
		newSets = append(newSets, newSet)
	}

	if len(newSets) > 0 {
		newSrcset := strings.Join(newSets, ",")
		dom.SetAttribute(node, "srcset", newSrcset)
	} else {
		dom.RemoveAttribute(node, "srcset")
	}

	return nil
}

func (arc *Archiver) revertConvertedNoScript(doc *html.Node) {
	divs := dom.GetElementsByTagName(doc, "div")
	dom.ForEachNode(divs, func(div *html.Node, _ int) {
		attr := dom.GetAttribute(div, "data-obelisk-noscript")
		if attr != "true" {
			return
		}

		noscript := dom.CreateElement("noscript")
		dom.SetTextContent(noscript, dom.InnerHTML(div))
		dom.ReplaceChild(div.Parent, noscript, div)
	})
}
