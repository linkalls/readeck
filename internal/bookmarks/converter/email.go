// SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package converter

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/CloudyKit/jet/v6"
	"github.com/wneessen/go-mail"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	"codeberg.org/readeck/readeck/internal/email"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/utils"
)

// HTMLEmailExporter is a content exporter that send bookmarks by emails.
type HTMLEmailExporter struct {
	HTMLConverter
	to           string
	baseURL      *url.URL
	templateVars jet.VarMap
	cidPrefix    string
	options      []email.MessageOption
}

// NewHTMLEmailExporter returns a new [HTMLEmailExporter] instance.
func NewHTMLEmailExporter(to string, baseURL *url.URL, templateVars jet.VarMap, options ...email.MessageOption) HTMLEmailExporter {
	return HTMLEmailExporter{
		HTMLConverter: HTMLConverter{},
		to:            to,
		baseURL:       baseURL,
		templateVars:  templateVars,
		cidPrefix:     base58.NewUUID(),
		options:       options,
	}
}

// Export implements [Exporter].
// It create an email with a text/plan and text/html version and attaches images
// as inline resources.
func (e HTMLEmailExporter) Export(ctx context.Context, _ io.Writer, _ *http.Request, bookmarkList []*bookmarks.Bookmark) error {
	b := bookmarkList[0]

	tc, err := e.getTemplateContext(ctx, b)
	if err != nil {
		return err
	}

	// Prepare message
	msg, err := email.NewMsg(
		configs.Config.Email.FromNoReply.String(),
		e.to,
		"[Readeck] "+utils.ShortText(b.Title, 80),
		append(
			e.options,
			email.WithHTMLTemplate(
				"/emails/bookmark",
				e.templateVars,
				tc,
			),
		)...,
	)
	if err != nil {
		return err
	}

	// Attach resources
	var c *bookmarks.BookmarkContainer
	if c, err = b.OpenContainer(); err != nil {
		return err
	}
	defer c.Close()

	for _, x := range c.ListResources() {
		if err = func() error {
			fp, err := x.Open()
			if err != nil {
				return err
			}
			defer fp.Close() // nolint:errcheck
			name := path.Base(x.Name)
			return msg.EmbedReader(name, fp,
				mail.WithFileContentID(e.cidPrefix+"."+name),
			)
		}(); err != nil {
			return err
		}
	}

	if b.DocumentType == "photo" || b.DocumentType == "video" {
		if i, ok := b.Files["image"]; ok {
			fp, err := c.Open(i.Name)
			if err != nil {
				return err
			}
			defer fp.Close() // nolint:errcheck

			name := path.Base(i.Name)
			if err = msg.EmbedReader(name, fp,
				mail.WithFileContentID(e.cidPrefix+"."+name),
			); err != nil {
				return err
			}
		}
	}

	return email.Sender.SendEmail(msg)
}

func (e HTMLEmailExporter) getTemplateContext(ctx context.Context, b *bookmarks.Bookmark) (map[string]any, error) {
	ctx = WithURLReplacer(ctx, "./_resources/", "cid:"+e.cidPrefix+".")
	html, err := e.GetArticle(ctx, b)
	if err != nil {
		return nil, err
	}

	image := &bookmarks.BookmarkFile{}
	if i, ok := b.Files["image"]; ok {
		*image = *i
		image.Name = "cid:" + e.cidPrefix + "." + path.Base(image.Name)
	}

	return map[string]any{
		"HTML":    html,
		"Item":    b,
		"Image":   image,
		"SiteURL": e.baseURL.String(),
	}, nil
}

// EPUBEmailExporter is a content exporter that send converted bookmarks as EPUB attachment
// by emails.
type EPUBEmailExporter struct {
	to           string
	baseURL      *url.URL
	templateVars jet.VarMap
	options      []email.MessageOption
}

// NewEPUBEmailExporter returns an [NewEPUBEmailExporter] instance.
func NewEPUBEmailExporter(to string, baseURL *url.URL, templateVars jet.VarMap, options ...email.MessageOption) EPUBEmailExporter {
	return EPUBEmailExporter{
		to:           to,
		baseURL:      baseURL,
		templateVars: templateVars,
		options:      options,
	}
}

// Export implements [Exporter].
// It create an email with the bookmark's EPUB file attached to it.
func (e EPUBEmailExporter) Export(ctx context.Context, _ io.Writer, r *http.Request, bookmarkList []*bookmarks.Bookmark) error {
	b := bookmarkList[0]

	msg, err := email.NewMsg(
		configs.Config.Email.FromNoReply.String(),
		e.to,
		"[Readeck EPUB] "+utils.ShortText(b.Title, 80),
		append(
			e.options,
			email.WithMDTemplate(
				"/emails/bookmark_epub.jet.md",
				e.templateVars,
				map[string]any{
					"Item":    b,
					"SiteURL": e.baseURL.String(),
				},
			),
		)...,
	)
	if err != nil {
		return err
	}

	w := new(bytes.Buffer)
	ee := NewEPUBExporter(e.baseURL, e.templateVars)
	if err := ee.Export(ctx, w, r, []*bookmarks.Bookmark{b}); err != nil {
		return err
	}
	if err := msg.AttachReader(fmt.Sprintf(
		"%s-%s.epub",
		b.Created.Format(time.DateOnly),
		utils.Slug(strings.TrimSuffix(utils.ShortText(b.Title, 40), "...")),
	), w); err != nil {
		return err
	}

	return email.Sender.SendEmail(msg)
}
