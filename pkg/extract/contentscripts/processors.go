// SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contentscripts

import (
	"context"
	"fmt"
	"net/url"
	"strings"

	"codeberg.org/readeck/readeck/pkg/extract"
	"github.com/antchfx/htmlquery"
	"github.com/araddon/dateparse"
	"github.com/go-shiori/dom"
)

var (
	runtimeCtxKey  = &contextKey{"runtime"}
	configCtxKey   = &contextKey{"config"}
	nextPageCtxKey = &contextKey{"next_page"}
)

func getRuntime(ctx context.Context) *Runtime {
	return ctx.Value(runtimeCtxKey).(*Runtime)
}

func getConfig(ctx context.Context) *SiteConfig {
	if cfg, ok := ctx.Value(configCtxKey).(*SiteConfig); ok {
		return cfg
	}
	return nil
}

// LoadScripts starts the content script runtime and adds it
// to the extractor context.
func LoadScripts(programs ...*Program) extract.Processor {
	return func(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
		if m.Step() != extract.StepStart || m.Position() > 0 {
			return next
		}

		vm, err := New(append(preloadedScripts, programs...)...)
		if err != nil {
			m.Log.WithError(err).Error("loading scripts")
			return next
		}
		vm.SetLogger(m.Log)
		vm.SetProcessMessage(m)

		m.Extractor.Context = context.WithValue(m.Extractor.Context, runtimeCtxKey, vm)
		m.Log.WithField("step", m.Step()).Debug("content script runtime ready")
		return next
	}
}

// LoadSiteConfig will try to find a matching site config
// for the first Drop (the extraction starting point).
//
// If a configuration is found, it will be added to the context.
//
// If the configuration indicates custom HTTP headers, they'll be added to
// the client.
func LoadSiteConfig(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepStart || m.Position() > 0 {
		return next
	}

	cfg, err := NewConfigForURL(SiteConfigFiles, m.Extractor.Drop().URL)
	if err != nil {
		m.Log.WithError(err).Warn("site configuration")
		return next
	}

	if cfg != nil {
		m.Log.WithField("files", cfg.files).Debug("site configuration loaded")
	} else {
		m.Log.Debug("no site configuration found")
		cfg = &SiteConfig{}
	}

	// Apply scripts "setConfig" function
	if err := getRuntime(m.Extractor.Context).SetConfig(cfg); err != nil {
		m.Log.WithError(err).Warn("setConfig")
	}

	// Add config to context
	m.Extractor.Context = context.WithValue(m.Extractor.Context, configCtxKey, cfg)

	// Set custom headers from configuration file
	prepareHeaders(m, cfg)

	return next
}

// ProcessMeta runs the content scripts processMeta exported functions.
func ProcessMeta(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom || m.Position() > 0 {
		return next
	}

	if err := getRuntime(m.Extractor.Context).ProcessMeta(); err != nil {
		m.Log.WithError(err).Warn("processMeta")
	}
	return next
}

func prepareHeaders(m *extract.ProcessMessage, cfg *SiteConfig) {
	if len(cfg.HTTPHeaders) == 0 {
		return
	}

	tr, ok := m.Extractor.Client().Transport.(*extract.Transport)
	if !ok {
		return
	}

	for k, v := range cfg.HTTPHeaders {
		m.Log.WithField("header", []string{k, v}).Debug("site config custom headers")
		tr.SetHeader(k, v)
	}
}

// ReplaceStrings applies all the replace_string directive in site config
// file on the received body.
func ReplaceStrings(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepBody {
		return next
	}

	cfg := getConfig(m.Extractor.Context)
	if cfg == nil {
		return next
	}

	d := m.Extractor.Drop()
	for _, r := range cfg.ReplaceStrings {
		d.Body = []byte(strings.ReplaceAll(string(d.Body), r[0], r[1]))
		m.Log.WithField("replace", r[:]).Debug("site config replace_string")
	}

	return next
}

// ExtractBody tries to find a body as defined by the "body" directives
// in the configuration file.
func ExtractBody(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	cfg := getConfig(m.Extractor.Context)
	if cfg == nil {
		return next
	}

	bodyNodes := dom.GetElementsByTagName(m.Dom, "body")
	if len(bodyNodes) == 0 {
		return next
	}
	body := bodyNodes[0]

	for _, selector := range cfg.BodySelectors {
		node, _ := htmlquery.Query(m.Dom, selector)
		if node == nil {
			continue
		}
		if len(dom.Children(node)) == 0 {
			continue
		}

		// First match, replace the root node and stop
		m.Log.WithField("nodes", len(dom.Children(node))).Debug("site config body found")

		newBody := dom.CreateElement("body")
		section := dom.CreateElement("section")
		dom.SetAttribute(section, "class", "article")
		dom.SetAttribute(section, "id", "article")
		dom.AppendChild(newBody, section)

		dom.AppendChild(section, node)
		dom.ReplaceChild(body.Parent, newBody, body)

		break
	}

	return next
}

// ExtractAuthor applies the "author" directives to find an author.
func ExtractAuthor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Position() > 0 || m.Step() != extract.StepDom {
		return next
	}

	cfg := getConfig(m.Extractor.Context)
	if cfg == nil {
		return next
	}

	for _, selector := range cfg.AuthorSelectors {
		// nodes, _ := m.Dom.Root().Search(selector)
		nodes, _ := htmlquery.QueryAll(m.Dom, selector)
		for _, n := range nodes {
			value := dom.TextContent(n)
			if value == "" {
				continue
			}
			m.Log.WithField("author", value).Debug("site config author")
			m.Extractor.Drop().AddAuthors(value)
		}
	}

	return next
}

// ExtractDate applies the "date" directives to find a date. If a date is found
// we try to parse it.
func ExtractDate(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Position() > 0 || m.Step() != extract.StepDom {
		return next
	}

	if !m.Extractor.Drop().Date.IsZero() {
		return next
	}

	cfg := getConfig(m.Extractor.Context)
	if cfg == nil {
		return next
	}

	for _, selector := range cfg.DateSelectors {
		nodes, _ := htmlquery.QueryAll(m.Dom, selector)
		for _, n := range nodes {
			date, err := dateparse.ParseLocal(dom.TextContent(n))
			if err == nil && !date.IsZero() {
				m.Log.WithField("date", date).Debug("site config date")
				m.Extractor.Drop().Date = date
				return next
			}
		}
	}

	return next
}

// StripTags removes the tags from the DOM root node, according to
// "strip_tags" configuration directives.
func StripTags(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	cfg := getConfig(m.Extractor.Context)
	if cfg == nil {
		return next
	}

	var value string

	for _, value = range cfg.StripSelectors {
		nodes, _ := htmlquery.QueryAll(m.Dom, value)
		dom.RemoveNodes(nodes, nil)
		m.Log.WithField("value", value).
			WithField("nodes", len(nodes)).
			Debug("site config strip_tags")
	}

	for _, value = range cfg.StripIDOrClass {
		selector := fmt.Sprintf(
			"//*[@id='%s' or contains(concat(' ',normalize-space(@class),' '),' %s ')]",
			value, value,
		)

		nodes, _ := htmlquery.QueryAll(m.Dom, selector)
		dom.RemoveNodes(nodes, nil)
		m.Log.WithField("value", value).
			WithField("nodes", len(nodes)).
			Debug("site config strip_id_or_class")
	}

	for _, value = range cfg.StripImageSrc {
		selector := fmt.Sprintf("//img[contains(@src, '%s')]", value)

		nodes, _ := htmlquery.QueryAll(m.Dom, selector)
		dom.RemoveNodes(nodes, nil)
		m.Log.WithField("value", value).
			WithField("nodes", len(nodes)).
			Debug("site config strip_image_src")
	}

	return next
}

// FindContentPage searches for SinglePageLinkSelectors in the page and,
// if it finds one, it reset the process to its beginning with the newly
// found URL.
func FindContentPage(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	// Don't look for any single page link for something that was recognized
	// as a media type.
	if m.Extractor.Drop().IsMedia() {
		return next
	}

	cfg := getConfig(m.Extractor.Context)
	if cfg == nil {
		return next
	}

	for _, selector := range cfg.SinglePageLinkSelectors {
		node, _ := htmlquery.Query(m.Dom, selector)
		if node == nil {
			continue
		}

		href := dom.GetAttribute(node, "href")
		if href == "" {
			href = dom.TextContent(node)
		}
		if href == "" {
			continue
		}
		u, err := m.Extractor.Drop().URL.Parse(href)
		if err != nil {
			continue
		}
		u.Fragment = ""

		if m.Extractor.Visited.IsPresent(u) {
			m.Log.WithField("url", u.String()).Debug("single page already visited")
			continue
		}

		m.Log.WithField("url", u.String()).Info("site config found single page link")
		if err = m.Extractor.ReplaceDrop(u); err != nil {
			m.Log.WithError(err).Error("cannot replace page")
			return nil
		}

		m.ResetPosition()

		return nil
	}

	return next
}

// FindNextPage looks for NextPageLinkSelectors and if it finds a URL, it's added to
// the message and can be processed later with GoToNextPage.
func FindNextPage(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepDom {
		return next
	}

	cfg := getConfig(m.Extractor.Context)
	if cfg == nil {
		return next
	}

	for _, selector := range cfg.NextPageLinkSelectors {
		node, _ := htmlquery.Query(m.Dom, selector)
		if node == nil {
			continue
		}

		href := dom.GetAttribute(node, "href")
		if href == "" {
			href = dom.TextContent(node)
		}
		if href == "" {
			continue
		}
		u, err := m.Extractor.Drop().URL.Parse(href)
		if err != nil {
			continue
		}
		u.Fragment = ""

		m.Log.WithField("url", u.String()).Debug("site config found next page")
		m.Extractor.Context = context.WithValue(m.Extractor.Context, nextPageCtxKey, u)
	}

	return next
}

// GoToNextPage checks if there is a "next_page" value in the process message. It then
// creates a new drop with the URL.
func GoToNextPage(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepFinish {
		return next
	}

	u, ok := m.Extractor.Context.Value(nextPageCtxKey).(*url.URL)
	if !ok || u == nil {
		return next
	}

	// Avoid crazy loops
	if m.Extractor.Visited.IsPresent(u) {
		m.Log.WithField("url", u.String()).Debug("next page already visited")
		return next
	}

	m.Log.WithField("url", u.String()).Info("go to next page")
	m.Extractor.AddDrop(u)
	m.Extractor.Context = context.WithValue(m.Extractor.Context, nextPageCtxKey, nil)

	return next
}
