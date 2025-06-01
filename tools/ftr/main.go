// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

// tools/ftr provides a command line interface to convert site config text files
// to JSON files.
package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

func main() {
	level := slog.LevelDebug
	if ci, _ := strconv.ParseBool(os.Getenv("CI")); ci {
		level = slog.LevelInfo
	}

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: level,
		ReplaceAttr: func(_ []string, a slog.Attr) slog.Attr {
			if a.Key == "time" {
				return slog.Attr{}
			}
			return a
		},
	})))

	flag.Parse()

	if len(flag.Args()) < 2 {
		slog.Error("Usage: fftr_convert <src> <dest>")
		os.Exit(1)
	}
	srcDir, _ := filepath.Abs(flag.Arg(0))
	destDir, _ := filepath.Abs(flag.Arg(1))

	info, err := os.Stat(srcDir)
	if err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
	if !info.IsDir() {
		slog.Error("not a directory", slog.String("src", srcDir))
		os.Exit(1)
	}

	info, err = os.Stat(destDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			if e := os.MkdirAll(destDir, 0o750); e != nil {
				slog.Error(e.Error())
			}
		} else {
			slog.Error(err.Error())
		}
	}
	if info != nil && !info.IsDir() {
		slog.Error("not a directory", slog.String("dir", destDir))
	}

	slog.Info("Reading FiveFilters files",
		slog.String("src", srcDir),
		slog.String("dest", destDir),
	)

	// Parse fftr files
	err = filepath.Walk(srcDir, func(name string, info os.FileInfo, _ error) error {
		if path.Base(name) == "LICENSE.txt" {
			return nil
		}
		if info.IsDir() {
			return nil
		}
		if path.Ext(name) != ".txt" {
			return nil
		}

		if err := converTextConfig(name, destDir); err != nil {
			slog.Error(err.Error(), slog.String("name", name))
			return nil
		}

		return nil
	})
	if err != nil {
		slog.Error(err.Error())
	}
}

// Config holds the fivefilters configuration.
type Config struct {
	Files []string `json:"-"`

	TitleSelectors          []string          `json:"title_selectors"`
	BodySelectors           []string          `json:"body_selectors"`
	DateSelectors           []string          `json:"date_selectors"`
	AuthorSelectors         []string          `json:"author_selectors"`
	StripSelectors          []string          `json:"strip_selectors"`
	StripIDOrClass          []string          `json:"strip_id_or_class"`
	StripImageSrc           []string          `json:"strip_image_src"`
	NativeAdSelectors       []string          `json:"native_ad_selectors"`
	Tidy                    bool              `json:"tidy"`
	Prune                   bool              `json:"prune"`
	AutoDetectOnFailure     bool              `json:"autodetect_on_failure"`
	SinglePageLinkSelectors []string          `json:"single_page_link_selectors"`
	NextPageLinkSelectors   []string          `json:"next_page_link_selectors"`
	ReplaceStrings          [][2]string       `json:"replace_strings"`
	HTTPHeaders             map[string]string `json:"http_headers"`
	Tests                   []FilterTest      `json:"tests"`
}

// FilterTest holds the values for a filter's test.
type FilterTest struct {
	URL      string   `json:"url"`
	Contains []string `json:"contains"`
}

func converTextConfig(filename string, dest string) error {
	fp, err := os.Open(filename)
	log := slog.With(slog.String("src", path.Base(filename)))

	if err != nil {
		log.Error(err.Error())
		return err
	}
	defer fp.Close() //nolint:errcheck

	cfg, err := newConfig(fp, log)
	if err != nil {
		return err
	}

	buf := bytes.NewBuffer([]byte{})
	encoder := json.NewEncoder(buf)
	encoder.SetIndent("", "  ")

	if err := encoder.Encode(cfg); err != nil {
		return err
	}

	destFile := path.Join(dest, path.Base(filename))
	destFile = destFile[0:len(destFile)-len(path.Ext(destFile))] + ".json"
	fd, err := os.Create(destFile)
	if err != nil {
		return err
	}
	defer fd.Close() //nolint:errcheck
	if _, err = fd.Write(buf.Bytes()); err != nil {
		return err
	}
	log.Debug("ok", slog.String("dest", path.Base(destFile)))
	return nil
}

func newConfig(file io.Reader, log *slog.Logger) (*Config, error) {
	res := &Config{
		AutoDetectOnFailure: true,
	}

	scanner := bufio.NewScanner(file)
	entries := make([][3]string, 0)
	for scanner.Scan() {
		t := strings.TrimSpace(scanner.Text())
		if t == "" || strings.HasPrefix(t, "#") || strings.HasPrefix(t, "//") {
			continue
		}
		entry, err := parseLine(t)
		if err != nil {
			log.Warn("can't parse config line",
				slog.Any("err", err),
				slog.String("line", t),
			)
			continue
		}
		entries = append(entries, entry)
	}

	parseFunctions := map[string]entryParser{
		"body":                  simpleStringValue(&res.BodySelectors),
		"title":                 simpleStringValue(&res.TitleSelectors),
		"date":                  simpleStringValue(&res.DateSelectors),
		"author":                simpleStringValue(&res.AuthorSelectors),
		"strip":                 simpleStringValue(&res.StripSelectors),
		"strip_id_or_class":     simpleStringValue(&res.StripIDOrClass),
		"strip_image_src":       simpleStringValue(&res.StripImageSrc),
		"native_ad_clue":        simpleStringValue(&res.NativeAdSelectors),
		"prune":                 simpleBoolValue(&res.Prune),
		"tidy":                  simpleBoolValue(&res.Tidy),
		"autodetect_on_failure": simpleBoolValue(&res.AutoDetectOnFailure),
		"single_page_link":      simpleStringValue(&res.SinglePageLinkSelectors),
		"next_page_link":        simpleStringValue(&res.NextPageLinkSelectors),
		"http_header":           setHeaderValue,
		"find_string":           setReplaceString,
		"replace_string":        setReplaceString,
		"test_url":              setFilterTest,
	}

	for i, line := range entries {
		fn, ok := parseFunctions[line[0]]
		if ok {
			err := fn(res, i, entries)
			if err != nil {
				return res, err
			}
		}
	}

	return res, nil
}

var lineRE = regexp.MustCompile(`^(.+?)(?:\((.+)\))?:\s*(.*)$`)

func parseLine(line string) ([3]string, error) {
	if !lineRE.MatchString(line) {
		return [3]string{}, errors.New("cannot parse line")
	}

	m := lineRE.FindAllStringSubmatch(line, -1)
	if strings.HasPrefix(m[0][3], "'") && strings.HasSuffix(m[0][3], "'") && len(m[0][3]) > 1 {
		m[0][3] = m[0][3][1 : len(m[0][3])-1]
	}

	return [3]string{m[0][1], m[0][2], m[0][3]}, nil
}

type entryParser func(*Config, int, [][3]string) error

func simpleStringValue(v *[]string) entryParser {
	return func(_ *Config, i int, entries [][3]string) error {
		*v = append(*v, entries[i][2])
		return nil
	}
}

func simpleBoolValue(v *bool) entryParser {
	return func(_ *Config, i int, entries [][3]string) error {
		*v = entries[i][2] == "yes"
		return nil
	}
}

func setHeaderValue(cfg *Config, i int, entries [][3]string) error {
	if entries[i][1] == "" {
		return fmt.Errorf("header value not set (%s)", entries[i][2])
	}

	if cfg.HTTPHeaders == nil {
		cfg.HTTPHeaders = map[string]string{}
	}
	cfg.HTTPHeaders[entries[i][1]] = entries[i][2]
	return nil
}

func setReplaceString(cfg *Config, i int, entries [][3]string) error {
	line := entries[i]
	switch line[0] {
	case "replace_string":
		if line[1] != "" {
			cfg.ReplaceStrings = append(cfg.ReplaceStrings, [2]string{line[1], line[2]})
			return nil
		}
		if i-1 < 0 {
			return fmt.Errorf("no preceding find_string entry before replace_string: %s", line[2])
		}
		prev := entries[i-1]
		if prev[0] != "find_string" {
			return fmt.Errorf("invalid preceding entry before replace_string: %s", line[2])
		}
	case "find_string":
		if i+1 >= len(entries) {
			return fmt.Errorf("no subsequent replace_string entry after find_string: %s", line[2])
		}
		next := entries[i+1]
		if next[0] != "replace_string" {
			return fmt.Errorf("invalid subsequent entry after find_string: %s", line[2])
		}
		if next[1] != "" {
			return fmt.Errorf("invalid subsequent entry after find_string: %s", line[2])
		}
		cfg.ReplaceStrings = append(cfg.ReplaceStrings, [2]string{line[2], next[2]})
	}
	return nil
}

func setFilterTest(cfg *Config, i int, entries [][3]string) error {
	line := entries[i]
	res := FilterTest{URL: line[2], Contains: make([]string, 0)}

	for {
		i++
		if i < len(entries) && entries[i][0] == "test_contains" {
			res.Contains = append(res.Contains, entries[i][2])
			continue
		}
		break
	}
	cfg.Tests = append(cfg.Tests, res)
	return nil
}
