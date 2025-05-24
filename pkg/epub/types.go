// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package epub

import "encoding/xml"

const (
	nsDC       = "http://purl.org/dc/elements/1.1/"
	nsOPF      = "http://www.idpf.org/2007/opf"
	nsNCX      = "http://www.daisy.org/z3986/2005/ncx/"
	ncxDoctype = `<!DOCTYPE ncx PUBLIC "-//NISO//DTD ncx 2005-1//EN"
"http://www.daisy.org/z3986/2005/ncx-2005-1.dtd">
`
)

// Package is the root of content.opf.
type Package struct {
	XMLName  xml.Name `xml:"package"`
	Version  string   `xml:"version,attr"`
	XMLns    string   `xml:"xmlns,attr"`
	UniqueID string   `xml:"unique-identifier,attr"`
	Metadata Metadata
	Manifest Manifest
	Spine    Spine
}

// Metadata is the package>metadata tag.
type Metadata struct {
	XMLName    xml.Name   `xml:"metadata"`
	XMLnsDC    string     `xml:"xmlns:dc,attr"`
	XMLnsOPF   string     `xml:"xmlns:opf,attr"`
	Identifier Identifier `xml:"dc:identifier"`
	Language   string     `xml:"dc:language"`
	Title      string     `xml:"dc:title"`
	Creator    string     `xml:"dc:creator"`
}

// Identifier is the metadata>dc:identifier tag.
type Identifier struct {
	XMLName xml.Name `xml:"dc:identifier"`
	Value   string   `xml:",chardata"`
	ID      string   `xml:"id,attr"`
	Scheme  string   `xml:"opf:scheme,attr"`
}

// Manifest is the package>manifest tag.
type Manifest struct {
	XMLName xml.Name `xml:"manifest"`
	Items   []ManifestItem
}

// ManifestItem is a manifest>item tag.
type ManifestItem struct {
	XMLName   xml.Name `xml:"item"`
	ID        string   `xml:"id,attr"`
	Href      string   `xml:"href,attr"`
	MediaType string   `xml:"media-type,attr"`
}

// Spine is the package>spine tag.
type Spine struct {
	XMLName xml.Name `xml:"spine"`
	Toc     string   `xml:"toc,attr"`
	Items   []SpineItem
}

// SpineItem is a spine>itemref tag.
type SpineItem struct {
	XMLName xml.Name `xml:"itemref"`
	IDRef   string   `xml:"idref,attr"`
	Title   string   `xml:"-"`
	Src     string   `xml:"-"`
}

// TOC is the root of the toc.ncx file.
type TOC struct {
	XMLName xml.Name  `xml:"ncx"`
	XMLns   string    `xml:"xmlns,attr"`
	Version string    `xml:"version,attr"`
	Meta    []TOCMeta `xml:"head>meta"`
	Title   string    `xml:"docTitle>text"`
	Author  string    `xml:"docAuthor>text"`
	Nav     []TOCNav  `xml:"navMap>navPoint"`
}

// TOCMeta is a ncx>head>meta tag.
type TOCMeta struct {
	Name    string `xml:"name,attr"`
	Content string `xml:"content,attr"`
}

// TOCNav is a ncx>navMap>navPoint tag.
type TOCNav struct {
	ID    string `xml:"id,attr"`
	Title string `xml:"navLabel>text"`
	Src   TOCNavSrc
}

// TOCNavSrc is ncx>navMap>navPoint>content tag.
type TOCNavSrc struct {
	XMLName xml.Name `xml:"content"`
	Src     string   `xml:"src,attr"`
}

// NewPackage returns a new Package instance with default values.
func NewPackage() Package {
	return Package{
		Version:  "2.0",
		XMLns:    nsOPF,
		UniqueID: "BookID",
		Metadata: Metadata{
			XMLnsDC:  nsDC,
			XMLnsOPF: nsOPF,
			Identifier: Identifier{
				ID:     "BookID",
				Scheme: "UUID",
			},
			Language: "en",
			Title:    "untitled",
		},
		Manifest: Manifest{
			Items: []ManifestItem{},
		},
		Spine: Spine{
			Toc:   "ncx",
			Items: []SpineItem{},
		},
	}
}
