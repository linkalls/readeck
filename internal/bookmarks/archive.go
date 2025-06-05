// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package bookmarks

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"mime"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/go-shiori/dom"
	"github.com/google/uuid"
	"golang.org/x/sync/semaphore"

	"codeberg.org/readeck/readeck/pkg/archiver"
	"codeberg.org/readeck/readeck/pkg/base58"
	"codeberg.org/readeck/readeck/pkg/extract"
	"codeberg.org/readeck/readeck/pkg/img"
)

const (
	resourceDirName = "_resources"

	// Fit large images to a maximum width that still looks crisp in bookmark
	// view. Since the current default article width is 48rem, the base font
	// size in web browsers is typically 16px, and a common scaling factor for
	// high-density displays is 2x, this is calculated as: 48 × 16px × 2.
	maxImageWidth = 1536
)

var (
	// We can't process too many images at the same time if we don't want to overload
	// the system and freeze everything just because the image processing has way
	// too much work to do.
	imgSem = semaphore.NewWeighted(2)
	imgCtx = context.TODO()
)

// ResourceDirName returns the resource folder name in an archive.
func ResourceDirName() string {
	return resourceDirName
}

// NewArchive runs the archiver and returns a BookmarkArchive instance.
func NewArchive(_ context.Context, ex *extract.Extractor) (*archiver.Archiver, error) {
	req := &archiver.Request{
		Client: ex.Client(),
		Input:  bytes.NewReader(ex.HTML),
		URL:    ex.Drop().URL,
	}

	arc, err := archiver.New(req)
	if err != nil {
		return nil, err
	}

	arc.MaxConcurrentDownload = 4
	arc.Flags = archiver.EnableImages
	arc.RequestTimeout = 45 * time.Second

	arc.EventHandler = eventHandler(ex)

	arc.ImageProcessor = imageProcessor
	arc.URLProcessor = urlProcessor

	if err := arc.Archive(context.Background()); err != nil {
		return nil, err
	}

	return arc, nil
}

var mimeTypes = map[string]string{
	"application/javascript":        ".js",
	"application/json":              ".json",
	"application/ogg":               ".ogx",
	"application/pdf":               ".pdf",
	"application/rtf":               ".rtf",
	"application/vnd.ms-fontobject": ".eot",
	"application/xhtml+xml":         ".xhtml",
	"application/xml":               ".xml",
	"audio/aac":                     ".aac",
	"audio/midi":                    ".midi",
	"audio/x-midi":                  ".midi",
	"audio/mpeg":                    ".mp3",
	"audio/ogg":                     ".oga",
	"audio/opus":                    ".opus",
	"audio/wav":                     ".wav",
	"audio/webm":                    ".weba",
	"font/otf":                      ".otf",
	"font/ttf":                      ".ttf",
	"font/woff":                     ".woff",
	"font/woff2":                    ".woff2",
	"image/bmp":                     ".bmp",
	"image/gif":                     ".gif",
	"image/jpeg":                    ".jpg",
	"image/png":                     ".png",
	"image/svg+xml":                 ".svg",
	"image/tiff":                    ".tiff",
	"image/vnd.microsoft.icon":      ".ico",
	"image/webp":                    ".webp",
	"text/calendar":                 ".ics",
	"text/css":                      ".css",
	"text/csv":                      ".csv",
	"text/html":                     ".html",
	"text/javascript":               ".js",
	"text/plain":                    ".txt",
	"video/mp2t":                    ".ts",
	"video/mp4":                     ".mp4",
	"video/mpeg":                    ".mpeg",
	"video/ogg":                     ".ogv",
	"video/webm":                    ".webm",
	"video/x-msvideo":               ".avi",
}

func eventHandler(ex *extract.Extractor) func(ctx context.Context, arc *archiver.Archiver, evt archiver.Event) {
	return func(_ context.Context, _ *archiver.Archiver, evt archiver.Event) {
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

		ex.Log().LogAttrs(context.Background(), level, msg, attrs...)
	}
}

// GetURLfilename returns a filename from a URL and MIME type. The filename
// is a short UUID based on the URL and its extension is based on the
// MIME type.
func GetURLfilename(uri string, contentType string) string {
	ext, ok := mimeTypes[strings.Split(contentType, ";")[0]]
	if !ok {
		ext = ".bin"
	}

	return base58.EncodeUUID(
		uuid.NewSHA1(uuid.NameSpaceURL, []byte(uri)),
	) + ext
}

func urlProcessor(uri string, _ []byte, contentType string) string {
	return "./" + path.Join(resourceDirName, GetURLfilename(uri, contentType))
}

func imageProcessor(ctx context.Context, arc *archiver.Archiver, input io.Reader, contentType string, uri *url.URL) ([]byte, string, error) {
	err := imgSem.Acquire(imgCtx, 1)
	if err != nil {
		return nil, "", err
	}
	defer imgSem.Release(1)

	if mt, _, err := mime.ParseMediaType(contentType); err == nil {
		contentType = mt
	}

	if _, ok := imageTypes[contentType]; !ok {
		r, err := io.ReadAll(input)
		if err != nil {
			return []byte{}, "", err
		}
		return r, contentType, nil
	}

	im, err := img.New(contentType, input)
	// If for any reason, we can't read the image, just return nothing
	if err != nil {
		arc.SendEvent(ctx, &archiver.EventError{Err: err, URI: uri.String()})
		return []byte{}, "", nil
	}
	defer func() {
		if err := im.Close(); err != nil {
			arc.SendEvent(ctx, &archiver.EventError{Err: err, URI: uri.String()})
		}
	}()

	err = img.Pipeline(im,
		func(im img.Image) error { return im.Clean() },
		func(im img.Image) error { return im.SetQuality(75) },
		func(im img.Image) error { return im.SetCompression(img.CompressionBest) },
		func(im img.Image) error { return img.Fit(im, maxImageWidth, 0) },
	)
	if err != nil {
		arc.SendEvent(ctx, &archiver.EventError{Err: err, URI: uri.String()})
		return []byte{}, "", nil
	}

	var buf bytes.Buffer
	err = im.Encode(&buf)
	if err != nil {
		arc.SendEvent(ctx, &archiver.EventError{Err: err, URI: uri.String()})
		return []byte{}, "", nil
	}

	// Set width and height on the <img> element
	node, ok := archiver.GetContextNode(ctx)
	if ok && dom.TagName(node) == "img" {
		dom.SetAttribute(node, "width", strconv.FormatInt(int64(im.Width()), 10))
		dom.SetAttribute(node, "height", strconv.FormatInt(int64(im.Height()), 10))
	}

	arc.SendEvent(ctx, archiver.EventInfo{"uri": uri.String(), "format": im.Format()})
	return buf.Bytes(), im.ContentType(), nil
}

// Note: we skip gif files since they're usually optimized already
// and could be animated, which isn't supported by all backends.
var imageTypes = map[string]struct{}{
	"image/bmp":     {},
	"image/jpeg":    {},
	"image/png":     {},
	"image/svg+xml": {},
	"image/tiff":    {},
	"image/webp":    {},
}
