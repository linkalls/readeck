// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package cookbook

import (
	"bytes"
	"context"
	"log/slog"
	"time"

	"codeberg.org/readeck/readeck/pkg/archiver"
	"codeberg.org/readeck/readeck/pkg/extract"
)

type ctxLogger struct{}

func archiveProcessor(m *extract.ProcessMessage, next extract.Processor) extract.Processor {
	if m.Step() != extract.StepPostProcess {
		return next
	}

	if len(m.Extractor.HTML) == 0 {
		return next
	}
	if !m.Extractor.Drop().IsHTML() {
		return next
	}

	m.Log().Debug("create archive")

	req := &archiver.Request{
		Client: m.Extractor.Client(),
		Input:  bytes.NewReader(m.Extractor.HTML),
		URL:    m.Extractor.Drop().URL,
	}
	arc, err := archiver.New(req)
	if err != nil {
		m.Log().Error("archive error", slog.Any("err", err))
		return next
	}

	arc.MaxConcurrentDownload = 5
	arc.Flags = archiver.EnableImages | archiver.EnableDataAttributes
	arc.RequestTimeout = 45 * time.Second
	arc.EventHandler = eventHandler

	ctx := context.WithValue(context.Background(), ctxLogger{}, m.Log())

	if err := arc.Archive(ctx); err != nil {
		m.Log().Error("archive error", slog.Any("err", err))
		return next
	}

	m.Extractor.HTML = arc.Result

	return next
}

func eventHandler(ctx context.Context, _ *archiver.Archiver, evt archiver.Event) {
	logger := ctx.Value(ctxLogger{}).(*slog.Logger)

	attrs := []slog.Attr{}
	for k, v := range evt.Fields() {
		attrs = append(attrs, slog.Any(k, v))
	}
	msg := "archiver"
	level := slog.LevelDebug

	switch evt.(type) {
	case *archiver.EventError:
		msg = "archive error"
		level = slog.LevelError
	case archiver.EventStartHTML:
		msg = "start archive"
		level = slog.LevelInfo
	case *archiver.EventFetchURL:
		msg = "load archive resource"
	}

	logger.LogAttrs(context.Background(), level, msg, attrs...)
}
