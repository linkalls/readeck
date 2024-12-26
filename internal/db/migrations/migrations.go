// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

//revive:disable:package-comments
package migrations

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-shiori/dom"
	"golang.org/x/net/html"

	"codeberg.org/readeck/readeck/configs"
	"codeberg.org/readeck/readeck/pkg/zipfs"
)

// Files contains all the static files needed by the app
//
//go:embed */*
var Files embed.FS

type (
	fileUpdateFunc func(path string) (newpath string, err error)
	domUpdateFunc  func(top *html.Node) (changed int, err error)
)

// updateBookmarkZipFiles applies a [fileUpdateFunc] on every zip file in the
// data/bookmarks folders.
// It performs a kind of transaction, first collecting a list of files returned by
// the updater function and then renaming then if everything went well.
// In case of failure, temporary files are all removed.
func updateBookmarkZipFiles(fn fileUpdateFunc) (err error) {
	var hasErrors bool
	var files [][2]string

	// Find all files in bookmark folder
	p := filepath.Join(configs.Config.Main.DataDirectory, "bookmarks")

	// Walk files and apply the updater callback
	err = filepath.WalkDir(p, func(path string, d fs.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if d.IsDir() {
			return nil
		}

		if filepath.Ext(path) != ".zip" {
			return nil
		}

		newpath, err := fn(path)
		if err != nil {
			slog.Error("",
				slog.String("path", path),
				slog.Any("err", err),
			)
			hasErrors = true
		}
		if newpath != "" {
			files = append(files, [2]string{path, newpath})
		}

		return nil
	})
	if err != nil {
		return err
	}

	if hasErrors {
		// Remove temporary files
		for _, x := range files {
			if err := os.Remove(x[1]); err != nil {
				slog.Error("deleting file",
					slog.String("file", x[1]),
					slog.Any("err", err),
				)
			}
		}
		return errors.New("migration failed")
	}

	// Commit files
	for _, x := range files {
		l := slog.With(
			slog.String("src", x[1]),
			slog.String("dest", x[0]),
		)
		if err := os.Rename(x[1], x[0]); err != nil {
			l.Error("moving file", slog.Any("err", err))
			return err
		}

		l.Debug("file moved")
	}
	return nil
}

// updateArchiveHTML performs an update in a zip's index.html file.
// It receives a [domUpdateFunc] that returns its changes and an error.
// When that function returns one or more changes, a new zip file containing the
// modified HTML is created.
func updateArchiveHTML(fn domUpdateFunc) fileUpdateFunc {
	return func(path string) (newpath string, err error) {
		z := zipfs.NewZipRW(nil, nil, 0)
		if err = z.AddSourceFile(path); err != nil {
			return "", err
		}
		defer z.Close() //nolint:errcheck

		fd, err := z.Source().Open("index.html")
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// no index.html, stop here
				return "", nil
			}
			return "", err
		}
		defer fd.Close() //nolint:errcheck

		top, err := html.Parse(fd)
		if err != nil {
			return "", err
		}
		changes, err := fn(top)
		if err != nil {
			return "", err
		}
		if changes == 0 {
			// no need to change the zip file, stop here
			return "", nil
		}

		// Create the new zip file
		dest := fmt.Sprintf("%s~", path)
		if err = z.AddDestFile(dest); err != nil {
			return dest, err
		}
		for _, x := range z.SrcFiles() {
			if x.Name == "index.html" {
				if err = z.Add(&x.FileHeader, strings.NewReader(dom.OuterHTML(top))); err != nil {
					return dest, err
				}
				continue
			}

			if err = z.Copy(x.Name); err != nil {
				return dest, err
			}
		}

		if err = z.Close(); err != nil {
			return dest, err
		}

		slog.Info("file updated",
			slog.Int("changes", changes),
			slog.String("path", path),
			slog.String("dest", dest),
		)
		return dest, nil
	}
}
