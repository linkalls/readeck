// SPDX-FileCopyrightText: © 2024 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// Package forwarded provides tools to deal with proxy related HTTP Headers.
package forwarded

import (
	"iter"
	"net"
	"net/http"
	"strings"
)

const (
	xForwardedFor   = "X-Forwarded-For"
	xForwardedHost  = "X-Forwarded-Host"
	xForwardedProto = "X-Forwarded-Proto"
)

// IsForwarded returns true if a request contains any x-forwarded header.
func IsForwarded(header http.Header) bool {
	return header.Get(xForwardedFor) != "" || header.Get(xForwardedHost) != "" || header.Get(xForwardedProto) != ""
}

// ParseXForwardedFor returns an iterator of all valid IP addresses
// found in X-Forwarded-For header. It yields IP addresses in reverse
// order so we can easily find the first mach from the rightmost value.
func ParseXForwardedFor(header http.Header) iter.Seq2[int, net.IP] {
	values := header[xForwardedFor]
	return func(yield func(int, net.IP) bool) {
		idx := 0
		for i := len(values) - 1; i >= 0; i-- {
			value := strings.Split(values[i], ",")
			for j := len(value) - 1; j >= 0; j-- {
				if ip := net.ParseIP(strings.TrimSpace(value[j])); ip != nil {
					if !yield(idx, ip) {
						return
					}
					idx++
				}
			}
		}
	}
}

// ParseXForwardedProto returns the value of X-Forwarded-Proto header.
// Possible return values are "http", "https" or an empty string.
func ParseXForwardedProto(header http.Header) string {
	res := strings.ToLower(strings.TrimSpace(header.Get(xForwardedProto)))
	if res == "http" || res == "https" {
		return res
	}
	return ""
}

// ParseXForwardedHost returns the (trimmed) value of X-Forwarded-Host header.
func ParseXForwardedHost(header http.Header) string {
	return strings.TrimSpace(header.Get(xForwardedHost))
}
