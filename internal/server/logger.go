// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package server

import (
	"context"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5/middleware"
)

// Logger is a middleware that logs requests.
func Logger() func(next http.Handler) http.Handler {
	return middleware.RequestLogger(&httpLogger{})
}

type httpLogger struct{}

func (sl *httpLogger) NewLogEntry(r *http.Request) middleware.LogEntry {
	attrs := httpAttrs{
		slog.String("@id", middleware.GetReqID(r.Context())),
		slog.Group("request",
			slog.String("method", r.Method),
			slog.String("path", r.RequestURI),
			slog.String("proto", r.Proto),
			slog.String("remote_addr", r.RemoteAddr),
		),
	}
	slog.LogAttrs(context.TODO(), slog.LevelDebug,
		"http "+r.Method,
		attrs...,
	)

	return attrs
}

type httpAttrs []slog.Attr

func (attrs httpAttrs) Write(status, bytes int, _ http.Header, elapsed time.Duration, _ interface{}) {
	slog.LogAttrs(context.TODO(), slog.LevelInfo,
		// fmt.Sprintf("http %d %s", status, http.StatusText(status)),
		"http "+strconv.Itoa(status)+" "+http.StatusText(status),
		append(attrs,
			slog.Group("response",
				slog.Int("status", status),
				slog.Int("length", bytes),
				slog.Float64("elapsed_ms", float64(elapsed.Nanoseconds())/1000000.0),
			),
		)...,
	)
}

func (attrs httpAttrs) Panic(_ interface{}, _ []byte) {
}
