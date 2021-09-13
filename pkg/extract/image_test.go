package extract

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"net/url"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/assert"
)

func TestRemoteImage(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/bogus", newFileResponder("images/bogus"))
	httpmock.RegisterResponder("GET", "/404", httpmock.NewJsonResponderOrPanic(404, ""))
	httpmock.RegisterResponder("GET", "/error", httpmock.NewErrorResponder(errors.New("HTTP")))

	formats := []string{"jpeg", "png", "gif", "ico", "bmp"}
	for _, name := range formats {
		name = "/img." + name
		httpmock.RegisterResponder("GET", name, newFileResponder("images/"+name))
	}

	t.Run("RemoteImage", func(t *testing.T) {
		t.Run("errors", func(t *testing.T) {
			tests := []struct {
				name string
				path string
				err  string
			}{
				{"url", "", "No image URL"},
				{"404", "/404", "Invalid response status (404)"},
				{"http", "/error", `Get "/error": HTTP`},
				{"bogus", "/bogus", "image: unknown format"},
			}

			for _, x := range tests {
				t.Run(x.name, func(t *testing.T) {
					ri, err := NewRemoteImage(x.path, nil)
					assert.Nil(t, ri)
					if ri != nil {
						defer ri.Close()
					}
					assert.Equal(t, x.err, err.Error())
				})
			}
		})

		for _, format := range formats {
			t.Run(format, func(t *testing.T) {
				ri, err := NewRemoteImage("/img."+format, nil)
				fmt.Printf("%+v\n", ri)
				if err != nil {
					t.Fatal(err)
				}
				defer ri.Close()
				assert.Equal(t, format, ri.Format())
			})
		}

		t.Run("fit", func(t *testing.T) {
			ri, _ := NewRemoteImage("/img.png", nil)
			defer ri.Close()

			w := ri.Width()
			h := ri.Height()
			assert.Equal(t, []uint{240, 181}, []uint{w, h})

			ri.Fit(uint(24), uint(24))
			assert.Equal(t, uint(24), ri.Width())
			assert.Equal(t, uint(18), ri.Height())

			ri.Fit(240, 240)
			assert.Equal(t, uint(24), ri.Width())
			assert.Equal(t, uint(18), ri.Height())
		})

		t.Run("encode", func(t *testing.T) {
			tests := []struct {
				name     string
				path     string
				format   string
				expected string
			}{
				{"auto-png", "/img.png", "", "png"},
				{"jpeg-jpeg", "/img.jpeg", "jpeg", "jpeg"},
				{"gif-gif", "/img.gif", "gif", "gif"},
				{"png-png", "/img.png", "png", "png"},
				{"png-gif", "/img.png", "gif", "gif"},
			}

			for _, x := range tests {
				t.Run(x.format, func(t *testing.T) {
					ri, err := NewRemoteImage(x.path, nil)
					defer func() {
						if err := ri.Close(); err != nil {
							panic(err)
						}
					}()
					assert.Nil(t, err)

					ri.SetFormat(x.format)

					var buf bytes.Buffer
					err = ri.Encode(&buf)
					assert.Nil(t, err)

					_, format, _ := image.DecodeConfig(bytes.NewReader(buf.Bytes()))
					assert.Equal(t, format, ri.Format())
				})
			}
		})
	})
}

func TestPicture(t *testing.T) {
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/img", newFileResponder("images/img.png"))

	base, _ := url.Parse("http://x/index.html")

	t.Run("URL error", func(t *testing.T) {
		p, err := NewPicture("/\b0x7f", base)
		assert.Nil(t, p)
		assert.NotNil(t, err)
	})

	t.Run("HTTP error", func(t *testing.T) {
		p, _ := NewPicture("/nowhere", base)
		err := p.Load(nil, 100, "")
		assert.NotNil(t, err)
	})

	t.Run("Load error", func(t *testing.T) {
		p, _ := NewPicture("/img", base)
		err := p.Load(nil, 0, "")
		assert.NotNil(t, err)
	})

	t.Run("Load", func(t *testing.T) {
		p, _ := NewPicture("/img", base)

		assert.Equal(t, "", p.Encoded())

		err := p.Load(nil, 100, "")
		assert.Nil(t, err)

		assert.Equal(t, [2]int{100, 75}, p.Size)
		assert.Equal(t, "image/png", p.Type)

		header := []byte{137, 80, 78, 71, 13, 10, 26, 10}
		assert.Equal(t, header, p.Bytes()[0:8])
		assert.Equal(t, "iVBORw0K", p.Encoded()[0:8])
	})
}
