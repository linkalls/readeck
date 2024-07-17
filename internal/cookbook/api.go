// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package cookbook

import (
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"path"
	"runtime"
	"time"

	"github.com/go-chi/chi/v5"
	log "github.com/sirupsen/logrus"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/internal/bookmarks"
	bookmark_tasks "codeberg.org/readeck/readeck/internal/bookmarks/tasks"
	"codeberg.org/readeck/readeck/internal/server"
	"codeberg.org/readeck/readeck/pkg/accept"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/extract/contents"
	"codeberg.org/readeck/readeck/pkg/extract/contentscripts"
	"codeberg.org/readeck/readeck/pkg/extract/meta"
)

// cookbookAPI is the base cookbook api router.
type cookbookAPI struct {
	chi.Router
	srv *server.Server
}

// newCookbookAPI returns a CookbokAPI with all the routes
// set up.
func newCookbookAPI(s *server.Server) *cookbookAPI {
	r := s.AuthenticatedRouter()

	api := &cookbookAPI{Router: r, srv: s}
	r.With(api.srv.WithPermission("api:cookbook", "read")).Group(func(r chi.Router) {
		r.Get("/urls", api.urlList)
		r.Get("/extract", api.extract)
		r.Post("/extract", api.extract)
	})

	return api
}

func (api *cookbookAPI) extract(w http.ResponseWriter, r *http.Request) {
	src := r.URL.Query().Get("url")
	if src == "" {
		http.Error(w, http.StatusText(400), 400)
		return
	}

	proxyList := make([]extract.ProxyMatcher, len(configs.Config.Extractor.ProxyMatch))
	for i, x := range configs.Config.Extractor.ProxyMatch {
		proxyList[i] = x
	}

	ex, err := extract.New(
		src,
		extract.SetLogFields(&log.Fields{"@id": api.srv.GetReqID(r)}),
		extract.SetDeniedIPs(configs.ExtractorDeniedIPs()),
		extract.SetProxyList(proxyList),
	)
	if err != nil {
		panic(err)
	}

	if r.Method == http.MethodPost {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			panic(err)
		}
		defer r.Body.Close()
		ex.AddToCache(src, map[string]string{
			"Content-Type": "text/html",
		}, body)
	}

	ex.AddProcessors(
		contentscripts.LoadScripts(
			bookmarks.GetContentScripts(api.srv.Log(r).Logger)...,
		),
		meta.ExtractMeta,
		meta.ExtractOembed,
		contentscripts.ProcessMeta,
		meta.SetDropProperties,
		meta.ExtractFavicon,
		meta.ExtractPicture,
		contentscripts.LoadSiteConfig,
		contentscripts.ReplaceStrings,
		contentscripts.FindContentPage,
		contentscripts.ExtractAuthor,
		contentscripts.ExtractDate,
		contentscripts.FindNextPage,
		contentscripts.ExtractBody,
		contentscripts.StripTags,
		contentscripts.GoToNextPage,
		contents.ExtractInlineSVGs,
		contents.Readability(),
		bookmark_tasks.CleanDomProcessor,
		contents.Text,
		archiveProcessor,
	)
	ex.Run()
	runtime.GC()

	// Very rough but good enough for our tests
	accepted := accept.NegotiateContentType(r.Header, []string{"application/json", "text/html"}, "application/json")
	if accepted == "text/html" {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(ex.HTML)
		return
	}

	drop := ex.Drop()

	res := &extractResult{
		URL:           drop.UnescapedURL(),
		Logs:          ex.Logs,
		Errors:        []string{},
		Meta:          drop.Meta,
		Properties:    drop.Properties,
		Domain:        drop.Domain,
		Title:         drop.Title,
		Authors:       drop.Authors,
		Site:          drop.URL.Hostname(),
		SiteName:      drop.Site,
		Lang:          drop.Lang,
		TextDirection: drop.TextDirection,
		Date:          &drop.Date,
		DocumentType:  drop.DocumentType,
		Description:   drop.Description,
		HTML:          string(ex.HTML),
		Text:          ex.Text,
		Images:        map[string]*extractImg{},
		Links:         []any{},
	}

	if drop.IsMedia() {
		res.Embed = drop.Meta.LookupGet("oembed.html")
	}

	for _, x := range ex.Errors() {
		res.Errors = append(res.Errors, x.Error())
	}
	if res.Date.IsZero() {
		res.Date = nil
	}

	for k, p := range drop.Pictures {
		res.Images[k] = &extractImg{
			Encoded: fmt.Sprintf("data:%s;base64,%s", p.Type, p.Encoded()),
			Size:    p.Size,
		}
	}

	for _, link := range bookmark_tasks.GetExtractedLinks(ex.Context) {
		res.Links = append(res.Links, link)
	}

	api.srv.Render(w, r, 200, res)
}

func (api *cookbookAPI) urlList(w http.ResponseWriter, r *http.Request) {
	urls := map[string][]string{}
	i := 0

	err := fs.WalkDir(contentscripts.SiteConfigFiles, ".", func(p string, d fs.DirEntry, err error) error {
		defer func() {
			i++
		}()

		if err != nil {
			return err
		}
		if d.IsDir() || path.Ext(d.Name()) != ".json" {
			return nil
		}

		f, err := contentscripts.SiteConfigFiles.Open(p)
		if err != nil {
			return nil
		}
		defer f.Close() //nolint:errcheck

		cfg, err := contentscripts.NewSiteConfig(f)
		if err != nil {
			log.WithField("cf", d.Name()).WithError(err).Error("error parsing file")
			return nil
		}

		if cfg != nil && len(cfg.Tests) > 0 {
			name := fmt.Sprintf("%d - %s", i, path.Base(d.Name()))
			urls[name] = make([]string, len(cfg.Tests))
			for i := range cfg.Tests {
				urls[name][i] = cfg.Tests[i].URL
			}
		}

		return nil
	})
	if err != nil {
		api.srv.Error(w, r, err)
		return
	}
	api.srv.Render(w, r, http.StatusOK, urls)
}

type extractImg struct {
	Size    [2]int `json:"size"`
	Encoded string `json:"encoded"`
}

type extractResult struct {
	URL           string                 `json:"url"`
	Logs          []string               `json:"logs"`
	Errors        []string               `json:"errors"`
	Meta          extract.DropMeta       `json:"meta"`
	Properties    extract.DropProperties `json:"properties"`
	Domain        string                 `json:"domain"`
	Title         string                 `json:"title"`
	Authors       []string               `json:"authors"`
	Site          string                 `json:"site"`
	SiteName      string                 `json:"site_name"`
	Lang          string                 `json:"lang"`
	TextDirection string                 `json:"text_direction"`
	Date          *time.Time             `json:"date"`
	DocumentType  string                 `json:"document_type"`
	Description   string                 `json:"description"`
	HTML          string                 `json:"html"`
	Text          string                 `json:"text"`
	Embed         string                 `json:"embed"`
	Images        map[string]*extractImg `json:"images"`
	Links         []any                  `json:"links"`
}
