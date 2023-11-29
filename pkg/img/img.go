// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

/*
Package img provides a unified image loader and manipulation pipeline.
It can load images using the native go implementations for the major
web image types, perform some manipulation like resizing and write
the result to an io.Writer.
*/
package img

import (
	"fmt"
	"image/color"
	"io"
)

// Gray16Palette is a 16 level b&w palette.
var Gray16Palette = []color.Color{
	color.RGBA{0x00, 0x00, 0x00, 0xff},
	color.RGBA{0x11, 0x11, 0x11, 0xff},
	color.RGBA{0x22, 0x22, 0x22, 0xff},
	color.RGBA{0x33, 0x33, 0x33, 0xff},
	color.RGBA{0x44, 0x44, 0x44, 0xff},
	color.RGBA{0x55, 0x55, 0x55, 0xff},
	color.RGBA{0x66, 0x66, 0x66, 0xff},
	color.RGBA{0x77, 0x77, 0x77, 0xff},
	color.RGBA{0x88, 0x88, 0x88, 0xff},
	color.RGBA{0x99, 0x99, 0x99, 0xff},
	color.RGBA{0xaa, 0xaa, 0xaa, 0xff},
	color.RGBA{0xbb, 0xbb, 0xbb, 0xff},
	color.RGBA{0xcc, 0xcc, 0xcc, 0xff},
	color.RGBA{0xdd, 0xdd, 0xdd, 0xff},
	color.RGBA{0xee, 0xee, 0xee, 0xff},
	color.RGBA{0xff, 0xff, 0xff, 0xff},
}

// ImageCompression is the compression level used for PNG images.
type ImageCompression uint8

const (
	// CompressionFast is a fast method.
	CompressionFast ImageCompression = iota

	// CompressionBest is the best space saving method.
	CompressionBest
)

// Image describes the interface of an image manipulation type.
type Image interface {
	Close() error
	Format() string
	ContentType() string
	Width() uint
	Height() uint
	SetFormat(string) error
	Resize(uint, uint) error
	Encode(io.Writer) error
	SetCompression(ImageCompression) error
	SetQuality(uint8) error
	Grayscale() error
	Gray16() error
	Clean() error
}

// ImageFilter is a filter application function used by the
// Pipeline method of an Image instance.
type ImageFilter func(Image) error

// Handler is a function that returns an Image type instance from an io.Reader.
type Handler func(io.Reader) (Image, error)

type handlersList map[string]Handler

// handlers holds the available image handlers.
var handlers = make(handlersList)

// AddImageHandler adds (or replaces) a handler for the given types.
func AddImageHandler(f Handler, types ...string) {
	for _, t := range types {
		handlers[t] = f
	}
}

// New creates a new Image instance, using the available handlers.
func New(contentType string, r io.Reader) (Image, error) {
	handler, ok := handlers[contentType]
	if !ok {
		return nil, fmt.Errorf("no img handler for %s", contentType)
	}

	return handler(r)
}

// Pipeline apply all the given ImageFilter functions to the image.
func Pipeline(im Image, filters ...ImageFilter) error {
	for _, fn := range filters {
		err := fn(im)
		if err != nil {
			return err
		}
	}
	return nil
}

// Fit resizes the image keeping the aspect ratio and staying within
// the given width and height.
func Fit(im Image, w, h uint) error {
	ow := im.Width()
	oh := im.Height()

	if w > ow && h > oh {
		return nil
	}

	srcAspectRatio := float64(ow) / float64(oh)
	maxAspectRatio := float64(w) / float64(h)

	var nw, nh uint
	if srcAspectRatio > maxAspectRatio {
		nw = w
		nh = uint(float64(nw) / srcAspectRatio)
	} else {
		nh = h
		nw = uint(float64(nh) * srcAspectRatio)
	}

	return im.Resize(nw, nh)
}
