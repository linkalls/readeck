// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/*
Package extract is a content extractor for HTML pages.
It works by using processors that are triggers at different (or several)
steps of the extraction process.
*/
package extract

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/go-shiori/dom"

	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/pkg/glob"
)

type (
	// ProcessStep defines a type of process applied during extraction.
	ProcessStep int

	// Processor is the process function.
	Processor func(*ProcessMessage, Processor) Processor

	// ProcessList holds the processes that will be applied.
	ProcessList []Processor

	// ProcessMessage holds the process message that is passed (and changed)
	// by the subsequent processes.
	ProcessMessage struct {
		Extractor *Extractor
		Dom       *html.Node

		logger       *slog.Logger
		position     int
		resetCounter int
		maxReset     int
		maxDrops     int
		step         ProcessStep
		canceled     bool
	}

	// ProxyMatcher describes a mapping of host/url for proxy dispatch.
	ProxyMatcher interface {
		// Returns the matching host
		Host() string
		// Returns the proxy URL
		URL() *url.URL
	}
)

const (
	// StepStart happens before the connection is made.
	StepStart ProcessStep = iota + 1

	// StepBody happens after receiving the resource body.
	StepBody

	// StepDom happens after parsing the resource DOM tree.
	StepDom

	// StepFinish happens at the very end of the extraction.
	StepFinish

	// StepPostProcess happens after looping over each Drop.
	StepPostProcess

	// StepDone is always called at the very end of the extraction.
	StepDone
)

func (s ProcessStep) String() string {
	switch s {
	case 1:
		return "start"
	case 2:
		return "body"
	case 3:
		return "dom"
	case 4:
		return "finish"
	case 5:
		return "done"
	}

	return strconv.Itoa(int(s))
}

// Step returns the current process step.
func (m *ProcessMessage) Step() ProcessStep {
	return m.step
}

// Position returns the current process position.
func (m *ProcessMessage) Position() int {
	return m.position
}

// ResetPosition lets the process start over (normally with a new URL).
// It holds a counter and cancels everything after too many resets (defined by maxReset).
func (m *ProcessMessage) ResetPosition() {
	if m.resetCounter >= m.maxReset {
		m.Cancel("too many redirects")
	}
	m.resetCounter++
	m.position = -1
}

// ResetContent empty the message Dom and all the drops body.
func (m *ProcessMessage) ResetContent() {
	m.Dom = nil
	m.Extractor.Drops()[m.position].Body = []byte{}
}

// Cancel fully cancel the extract process.
func (m *ProcessMessage) Cancel(reason string, args ...interface{}) {
	m.Log().Error("operation canceled", slog.Any("err", fmt.Errorf(reason, args...)))
	m.canceled = true
}

// Log returns the message's [slog.Logger].
func (m *ProcessMessage) Log() *slog.Logger {
	return m.logger.With(slog.Group("step",
		slog.Int("id", int(m.step)),
		slog.String("name", m.step.String()),
	))
}

// SetDataAttribute adds a data attribute to a given node.
// The attribute follows a pattern that's "x-data-{random-string}-{name}"
// The random value is there to avoid attribute injection from a page.
func (m *ProcessMessage) SetDataAttribute(node *html.Node, name, value string) {
	dom.SetAttribute(node, "x-data-"+m.Extractor.uniqueID+"-"+name, value)
}

// transformDataAttributes converts all existing x-data-{random-string}-* to
// regular data attributes.
// The resulting attribute looks like "data-readeck-{name}".
func (m *ProcessMessage) transformDataAttributes() {
	if m.Dom == nil {
		return
	}

	dom.ForEachNode(dom.QuerySelectorAll(m.Dom, "*"), func(n *html.Node, _ int) {
		for i, a := range n.Attr {
			if strings.HasPrefix(a.Key, "x-data-"+m.Extractor.uniqueID+"-") {
				n.Attr[i].Key = "data-readeck-" + a.Key[len("x-data-"+m.Extractor.uniqueID+"-"):]
			}
		}
	})
}

// Error holds all the non-fatal errors that were
// caught during extraction.
type Error []error

func (e Error) Error() string {
	s := make([]string, len(e))
	for i, err := range e {
		s[i] = err.Error()
	}
	return strings.Join(s, ", ")
}

// URLList hold a list of URLs.
type URLList map[string]bool

// Add adds a new URL to the list.
func (l URLList) Add(v *url.URL) {
	c := *v
	c.Fragment = ""
	l[c.String()] = true
}

// IsPresent returns.
func (l URLList) IsPresent(v *url.URL) bool {
	c := *v
	c.Fragment = ""
	return l[c.String()]
}

// Extractor is a page extractor.
type Extractor struct {
	URL     *url.URL
	HTML    []byte
	Text    string
	Visited URLList
	Logs    []string
	Context context.Context

	client          *http.Client
	logger          *slog.Logger
	processors      ProcessList
	errors          Error
	drops           []*Drop
	uniqueID        string
	cachedResources map[string]*cachedResource
}

// New returns an Extractor instance for a given URL,
// with a default HTTP client.
func New(src string, options ...func(e *Extractor)) (*Extractor, error) {
	URL, err := url.Parse(src)
	if err != nil {
		return nil, err
	}
	URL.Fragment = ""

	id := make([]byte, 4)
	rand.Read(id)

	res := &Extractor{
		URL:             URL,
		Visited:         URLList{},
		Context:         context.TODO(),
		client:          NewClient(),
		cachedResources: make(map[string]*cachedResource),
		processors:      ProcessList{},
		drops:           []*Drop{NewDrop(URL)},
		uniqueID:        hex.EncodeToString(id),
	}

	t := res.client.Transport.(*Transport)
	t.SetRoundTripper(res.getFromCache)

	for _, fn := range options {
		if fn != nil {
			fn(res)
		}
	}

	if res.logger == nil {
		res.logger = slog.New(newLogRecorder(slog.Default().Handler(), res))
	}

	return res, nil
}

// SetLogger sets the extractor logger.
// This logger will copy everything to the extractor internal log and error list.
// Arguments are [slog.With] arguments and are shared between the parent logger
// and the log recorder.
func SetLogger(logger *slog.Logger, args ...any) func(e *Extractor) {
	return func(e *Extractor) {
		e.logger = slog.New(newLogRecorder(logger.Handler(), e)).With(args...)
	}
}

// SetDeniedIPs sets a list of ip or cird that cannot be reached
// by the extraction client.
func SetDeniedIPs(netList []*net.IPNet) func(e *Extractor) {
	return func(e *Extractor) {
		if t, ok := e.client.Transport.(*Transport); ok {
			t.deniedIPs = netList
		}
	}
}

// SetProxyList adds a new proxy dispatcher function to the HTTP transport.
func SetProxyList(list []ProxyMatcher) func(e *Extractor) {
	return func(e *Extractor) {
		t := e.client.Transport.(*Transport)
		htr := t.tr.(*http.Transport)
		htr.Proxy = func(r *http.Request) (*url.URL, error) {
			for _, p := range list {
				if glob.Glob(p.Host(), r.URL.Host) {
					e.Log().Debug("using proxy", slog.String("proxy", p.URL().String()))
					return p.URL(), nil
				}
			}
			return nil, nil
		}
	}
}

// Client returns the extractor's HTTP client.
func (e *Extractor) Client() *http.Client {
	return e.client
}

// Log returns the extractor's logger.
func (e *Extractor) Log() *slog.Logger {
	return e.logger
}

// AddToCache adds a resource to the extractor's resource cache.
// The cache will be used by the HTTP client during its round trip.
func (e *Extractor) AddToCache(url string, headers map[string]string, body []byte) {
	e.cachedResources[url] = &cachedResource{headers: headers, data: newCacheEntry(body)}
}

// IsInCache returns true if a given URL is present in the
// resource cache mapping.
func (e *Extractor) IsInCache(url string) bool {
	_, ok := e.cachedResources[url]
	return ok
}

// Errors returns the extractor's error list.
func (e *Extractor) Errors() Error {
	return e.errors
}

// AddError add a new error to the extractor's error list.
func (e *Extractor) AddError(err error) {
	e.errors = append(e.errors, err)
}

// Drops returns the extractor's drop list.
func (e *Extractor) Drops() []*Drop {
	return e.drops
}

// Drop return the extractor's first drop, when there is one.
func (e *Extractor) Drop() *Drop {
	if len(e.drops) == 0 {
		return nil
	}
	return e.drops[0]
}

// AddDrop adds a new Drop to the drop list.
func (e *Extractor) AddDrop(src *url.URL) {
	e.drops = append(e.drops, NewDrop(src))
}

// ReplaceDrop replaces the main Drop with a new one.
func (e *Extractor) ReplaceDrop(src *url.URL) error {
	if len(e.drops) != 1 {
		return errors.New("cannot replace a drop when there are more that one")
	}

	e.drops[0] = NewDrop(src)
	return nil
}

// AddProcessors adds extract processor(s) to the list.
func (e *Extractor) AddProcessors(p ...Processor) {
	e.processors = append(e.processors, p...)
}

// NewProcessMessage returns a new ProcessMessage for a given step.
func (e *Extractor) NewProcessMessage(step ProcessStep) *ProcessMessage {
	return &ProcessMessage{
		Extractor:    e,
		logger:       e.logger,
		step:         step,
		resetCounter: 0,
		maxReset:     10,
		maxDrops:     100,
	}
}

// Run start the extraction process.
func (e *Extractor) Run() {
	i := 0
	m := e.NewProcessMessage(0)

	defer func() {
		m.step = StepDone
		e.runProcessors(m)
		if e.client != nil {
			e.client.CloseIdleConnections()
		}
	}()

	for i < len(e.drops) {
		d := e.drops[i]

		// Don't visit the same URL twice
		if e.Visited.IsPresent(d.URL) {
			i++
			continue
		}
		e.Visited.Add(d.URL)

		// Don't let any page fool us into processing an
		// unlimited number of pages.
		if len(e.drops) >= m.maxDrops {
			m.Cancel("too many pages")
		}

		m.position = i

		// Start extraction
		m.Log().Info("start",
			slog.Int("idx", i),
			slog.String("url", d.URL.String()),
		)
		m.step = StepStart
		e.runProcessors(m)
		if m.canceled {
			return
		}

		err := d.Load(e.client)
		if err != nil {
			m.Log().Error("cannot load resource", slog.Any("err", err))
			return
		}

		// First process pass
		m.Log().Debug("step body")
		m.step = StepBody
		e.runProcessors(m)
		if m.canceled {
			return
		}

		// Load the dom
		if d.IsHTML() {
			func() {
				doc, err := html.Parse(bytes.NewReader(d.Body))
				defer func() {
					m.Dom = nil
				}()

				if err != nil {
					m.Log().Error("cannot parse resource", slog.Any("err", err))
					return
				}

				m.Log().Debug("step DOM")
				m.Dom = doc
				m.step = StepDom

				d.fixRelativeURIs(m) // Fix relative URIs before any processor
				e.runProcessors(m)
				if m.canceled {
					return
				}

				m.transformDataAttributes()

				// Render the final document body
				if m.Dom != nil {
					buf := bytes.NewBuffer(nil)
					html.Render(buf, convertBodyNodes(m.Dom))
					d.Body = buf.Bytes()
				}
			}()
		}

		// Final processes
		m.Log().Debug("step finish")
		m.step = StepFinish
		e.runProcessors(m)
		if m.canceled {
			return
		}

		// A processor can change the position in the loop
		i = m.position + 1
	}

	// Postprocess
	m.Log().Debug("postprocess")
	m.step = StepPostProcess
	e.setFinalHTML()
	e.runProcessors(m)
}

func (e *Extractor) getFromCache(req *http.Request) (*http.Response, error) {
	u := req.URL.String()
	entry, ok := e.cachedResources[u]
	if !ok {
		return nil, nil
	}

	e.Log().Debug("cache hit", slog.String("url", u))
	headers := make(http.Header)
	for k, v := range entry.headers {
		headers.Set(k, v)
	}

	return &http.Response{
		Status:        "OK",
		StatusCode:    http.StatusOK,
		Header:        headers,
		Body:          entry.data,
		Request:       req,
		ContentLength: -1,
	}, nil
}

func (e *Extractor) runProcessors(m *ProcessMessage) {
	if len(e.processors) == 0 {
		return
	}

	p := e.processors[0]
	i := 0
	for {
		var next Processor
		i++
		if i < len(e.processors) {
			next = e.processors[i]
		}
		p = p(m, next)
		if p == nil {
			return
		}
	}
}

// convertBodyNodes extracts all the element from a
// document body and then returns a new HTML Document
// containing only the body's children.
func convertBodyNodes(top *html.Node) *html.Node {
	doc := &html.Node{
		Type: html.DocumentNode,
	}
	for _, node := range dom.GetElementsByTagName(top, "body") {
		for _, c := range dom.ChildNodes(node) {
			doc.AppendChild(dom.Clone(c, true))
		}
	}

	return doc
}

func (e *Extractor) setFinalHTML() {
	buf := &bytes.Buffer{}
	for i, d := range e.drops {
		if len(d.Body) == 0 {
			continue
		}
		fmt.Fprintf(buf, "<!-- page %d -->\n", i+1)
		buf.Write(d.Body)
		buf.WriteString("\n")
	}
	e.HTML = buf.Bytes()
}

type cachedResource struct {
	headers map[string]string
	data    io.ReadCloser
}

type cacheEntry struct {
	*bytes.Reader
}

func newCacheEntry(data []byte) *cacheEntry {
	return &cacheEntry{
		bytes.NewReader(data),
	}
}

// Close implement io.ReadCloser on a cache entry.
// It rewinds the reader's position.
func (cr *cacheEntry) Close() (err error) {
	_, err = cr.Seek(0, 0)
	return
}
