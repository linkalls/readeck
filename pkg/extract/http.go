// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"crypto/tls"
	"fmt"
	"maps"
	"math/rand/v2"
	"net"
	"net/http"
	"net/http/cookiejar"
	"net/textproto"
	"time"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"
)

const uaString = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/130.0.0.0 Safari/537.3"

// defaultDialer is our own default net.Dialer with shorter timeout and keepalive.
var defaultDialer = net.Dialer{
	Timeout:   15 * time.Second,
	KeepAlive: 30 * time.Second,
}

var cipherSuites = []uint16{
	// Chrome like cipher suite
	tls.TLS_AES_128_GCM_SHA256,
	tls.TLS_AES_256_GCM_SHA384,
	tls.TLS_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305_SHA256,
	tls.TLS_ECDHE_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_ECDHE_RSA_WITH_AES_256_CBC_SHA,
	tls.TLS_RSA_WITH_AES_128_GCM_SHA256,
	tls.TLS_RSA_WITH_AES_256_GCM_SHA384,
	tls.TLS_RSA_WITH_AES_128_CBC_SHA,
	tls.TLS_RSA_WITH_AES_256_CBC_SHA,
}

var greaseCiphers = []uint16{
	0x0a0a, 0x1a1a, 0x2a2a, 0x3a3a, 0x4a4a, 0x5a5a, 0x6a6a, 0x7a7a,
	0x8a8a, 0x9a9a, 0xaaaa, 0xbaba, 0xcaca, 0xdada, 0xeaea, 0xfafa,
}

// defaultTransport is our http.RoundTripper with some custom settings.
var defaultTransport = &http.Transport{
	Proxy:       http.ProxyFromEnvironment,
	DialContext: defaultDialer.DialContext,
	TLSClientConfig: &tls.Config{
		// Note: although some ciphers and TLS version are disabled by default for good reasons,
		// we need to enable them for some websites :/
		CipherSuites: cipherSuites,
		MinVersion:   tls.VersionTLS12,
	},
	ForceAttemptHTTP2:     true,
	DisableCompression:    false,
	DisableKeepAlives:     false,
	MaxIdleConns:          50,
	MaxIdleConnsPerHost:   2,
	IdleConnTimeout:       30 * time.Second,
	TLSHandshakeTimeout:   10 * time.Second,
	ExpectContinueTimeout: 1 * time.Second,
}

// defaultHeaders are the HTTP headers that are sent with every new request.
// They're attached to the transport and can be overridden and/or modified
// while using the associated client.
var defaultHeaders = http.Header{
	"User-Agent":                []string{uaString},
	"Accept":                    []string{"text/html,application/xhtml+xml,application/xml;q=0.9,image/jpeg,image/png,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7"},
	"Accept-Language":           []string{"en-US,en;q=0.8"},
	"Cache-Control":             []string{"max-age=0"},
	"Upgrade-Insecure-Requests": []string{"1"},
	"Sec-CH-UA":                 []string{`"Google Chrome";v="137", "Chromium";v="137"`},
	"Sec-CH-UA-mobile":          []string{"?0"},
	"Sec-CH-UA-platform":        []string{`"Windows"`},
	"Sec-Fetch-Site":            []string{"none"},
}

// Transport is a wrapper around http.RoundTripper that
// lets you set default headers sent with every request.
type Transport struct {
	tr        http.RoundTripper
	header    http.Header
	deniedIPs []*net.IPNet
	roundTrip transportCache
}

type transportCache func(*http.Request) (*http.Response, error)

// RoundTrip is the transport interceptor.
func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if t.roundTrip != nil {
		rsp, err := t.roundTrip(req)
		if err != nil || rsp != nil {
			return rsp, err
		}
	}

	if err := t.checkDestIP(req); err != nil {
		return nil, err
	}

	// Add the client's default headers that don't exist in the
	// current request.
	for k, values := range t.header {
		if _, ok := req.Header[textproto.CanonicalMIMEHeaderKey(k)]; !ok {
			req.Header[k] = values
		}
	}

	t.setTLSGrease()

	return t.tr.RoundTrip(req)
}

// setTLSGrease adds a random GREASE cipher to the cipher suite.
// see https://www.rfc-editor.org/rfc/rfc8701
func (t *Transport) setTLSGrease() {
	if tr, ok := t.tr.(*http.Transport); ok {
		tr.TLSClientConfig.CipherSuites = append([]uint16{
			greaseCiphers[rand.IntN(len(greaseCiphers))], //nolint:gosec
		}, cipherSuites...)
	}
}

// setHeader lets you set a default header for any subsequent requests.
func (t *Transport) setHeader(name, value string) {
	if value == "" {
		t.header.Del(name)
		return
	}
	t.header.Set(name, value)
}

// SetRoundTripper sets an extra transport's round trip function.
func (t *Transport) SetRoundTripper(f transportCache) {
	t.roundTrip = f
}

func (t *Transport) checkDestIP(r *http.Request) error {
	if len(t.deniedIPs) == 0 {
		// An empty list disables the IP check altogether
		return nil
	}

	hostname := r.URL.Hostname()
	host, err := idna.ToASCII(hostname)
	if err != nil {
		return fmt.Errorf("invalid hostname %s", hostname)
	}

	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("cannot resolve %s", host)
	}

	for _, cidr := range t.deniedIPs {
		for _, ip := range ips {
			if cidr.Contains(ip) {
				return fmt.Errorf("ip %s is blocked by rule %s", ip, cidr)
			}
		}
	}

	return nil
}

// NewClient returns a new http.Client with our custom transport.
func NewClient() *http.Client {
	// We first try to use http.DefaultTransport and, only when it's
	// an http.Transport instance, we swap it for our own transport.
	// This way, httpmock keeps working with tests.
	tr := http.DefaultTransport
	if _, ok := http.DefaultTransport.(*http.Transport); ok {
		tr = defaultTransport
	}

	cookies, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	return &http.Client{
		Timeout: 10 * time.Second,
		Jar:     cookies,
		Transport: &Transport{
			tr:        tr,
			header:    maps.Clone(defaultHeaders),
			deniedIPs: []*net.IPNet{},
		},
	}
}

// SetHeader sets a header on a given client.
func SetHeader(client *http.Client, name, value string) {
	if t, ok := client.Transport.(*Transport); ok {
		t.setHeader(name, value)
	}
}
