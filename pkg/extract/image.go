// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"io"

	"net/http"
	"net/url"

	"github.com/gabriel-vasile/mimetype"
	"github.com/readeck/readeck/pkg/img"
)

// NewRemoteImage loads an image and returns a new img.Image instance.
func NewRemoteImage(src string, client *http.Client) (img.Image, error) {
	if client == nil {
		client = http.DefaultClient
	}

	if src == "" {
		return nil, fmt.Errorf("No image URL")
	}

	// Send the request with a specific Accept header
	req, err := http.NewRequest("GET", src, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "image/webp,image/svg+xml,image/*,*/*;q=0.8")

	rsp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer rsp.Body.Close()

	if rsp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("Invalid response status (%d)", rsp.StatusCode)
	}

	buf := new(bytes.Buffer)
	mtype, err := mimetype.DetectReader(io.TeeReader(rsp.Body, buf))
	if err != nil {
		return nil, err
	}
	return img.New(mtype.String(), io.MultiReader(buf, rsp.Body))
}

// Picture is a remote picture
type Picture struct {
	Href   string
	Type   string
	Size   [2]int
	format string
	bytes  []byte
}

// NewPicture returns a new Picture instance from a given
// URL and its base.
func NewPicture(src string, base *url.URL) (*Picture, error) {
	href, err := base.Parse(src)
	if err != nil {
		return nil, err
	}

	return &Picture{
		Href: href.String(),
	}, nil
}

// Load loads the image remotely and fit it into the given
// boundaries size.
func (p *Picture) Load(client *http.Client, size uint, toFormat string) error {
	ri, err := NewRemoteImage(p.Href, client)
	if err != nil {
		return err
	}
	defer ri.Close()

	err = img.Pipeline(ri, pClean, pComp, pQual, pFit(size), pFormat(toFormat))
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	err = ri.Encode(&buf)
	if err != nil {
		return err
	}

	p.bytes = buf.Bytes()
	p.Size = [2]int{int(ri.Width()), int(ri.Height())}
	p.Type = ri.ContentType()
	p.format = ri.Format()
	return nil
}

// Copy returns a resized copy of the image, as a new Picture instance.
func (p *Picture) Copy(size uint, toFormat string) (*Picture, error) {
	ri, err := img.New(p.Type, bytes.NewReader(p.bytes))
	if err != nil {
		return nil, err
	}
	defer ri.Close()

	res := &Picture{Href: p.Href}
	err = img.Pipeline(ri, pClean, pComp, pQual, pFit(size), pFormat(toFormat))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	err = ri.Encode(&buf)
	if err != nil {
		return nil, err
	}

	res.bytes = buf.Bytes()
	res.Size = [2]int{int(ri.Width()), int(ri.Height())}
	res.Type = ri.ContentType()
	res.format = ri.Format()
	return res, nil
}

// Name returns the given name of the picture with the correct
// extension.
func (p *Picture) Name(name string) string {
	return fmt.Sprintf("%s.%s", name, p.format)
}

// Bytes returns the image data.
func (p *Picture) Bytes() []byte {
	return p.bytes
}

// Encoded returns a base64 encoded string of the image.
func (p *Picture) Encoded() string {
	if len(p.bytes) == 0 {
		return ""
	}

	return base64.StdEncoding.EncodeToString(p.bytes)
}

func pFormat(f string) img.ImageFilter {
	return func(im img.Image) error {
		return im.SetFormat(f)
	}
}

func pFit(s uint) img.ImageFilter {
	return func(im img.Image) error {
		return img.Fit(im, s, s)
	}
}
func pComp(im img.Image) error {
	return im.SetCompression(img.CompressionBest)
}
func pClean(im img.Image) error {
	return im.Clean()
}
func pQual(im img.Image) error {
	return im.SetQuality(75)
}
