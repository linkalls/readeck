// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package assets

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/readeck/readeck/configs"
	"github.com/readeck/readeck/internal/server"
	"github.com/readeck/readeck/pkg/csp"
)

type ctxNameKey struct{}

const svgGradient = `<?xml version="1.0" encoding="UTF-8"?>` +
	`<svg xmlns="http://www.w3.org/2000/svg" version="1.1" viewBox="0 0 1024 576" width="1024" height="576">` +
	`<defs>` +
	`<linearGradient id="gradient" x1="0%%" y1="0%%" x2="0%%" y2="100%%">` +
	`<stop stop-color="hsl(%d, %d%%, 70%%)" offset="0"/>` +
	`<stop stop-color="hsl(%d, %d%%, 70%%)" offset="1"/>` +
	`</linearGradient>` +
	`</defs>` +
	`<rect width="100%%" height="100%%" fill="url(#gradient)"/>` +
	`%s` +
	`</svg>`

type random struct {
	*rand.Rand
}

func newRandom(data uint64) *random {
	return &random{rand.New(rand.NewSource(int64(data)))}
}

func (r *random) GetSumStrings() []string {
	return []string{configs.BuildTime().String(), fmt.Sprintf("%d", r.Int())}
}

func (r *random) GetLastModified() []time.Time {
	return []time.Time{configs.BuildTime()}
}

// randomSvg sends an SVG image with a gradient. The gradient's color
// is based on the name.
func randomSvg(s *server.Server) http.Handler {
	r := chi.NewRouter()

	withHashCode := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			name := chi.URLParam(r, "name")
			data := uint64(0)
			for _, b := range []byte(name) {
				data = (data << 8) | uint64(b)
			}

			rd := newRandom(data)
			ctx := context.WithValue(r.Context(), ctxNameKey{}, rd)

			s.WriteEtag(w, rd)
			s.WriteLastModified(w, rd)
			csp.Policy{
				"base-uri":    {csp.None},
				"default-src": {csp.None},
				"style-src":   {csp.UnsafeInline},
			}.Write(w.Header())
			s.WithCaching(next).ServeHTTP(w, r.WithContext(ctx))
		})
	}

	r.With(withHashCode).Get("/", func(w http.ResponseWriter, r *http.Request) {
		rd := r.Context().Value(ctxNameKey{}).(*random)

		w.Header().Set("Content-Type", "image/svg+xml")
		fmt.Fprintf(w, svgGradient,
			rd.Perm(30)[0]+10,  // top color
			rd.Perm(55)[0]+20,  // top saturation
			rd.Perm(30)[0]+190, // bottom color
			rd.Perm(70)[1]+20,  // bottom saturation
			randomCircles(rd),
		)
	})
	return r
}

func randomCircles(r *random) string {
	numCircles := r.Perm(5)[0] + 5
	distribution := make([][4]int, numCircles)

	for i := range distribution {
		// Position
		distribution[i][0] = r.Perm(1024)[0]
		distribution[i][1] = r.Perm(576)[0]
		// Size
		distribution[i][2] = r.Perm(100)[0] + 30
		// Opacity
		distribution[i][3] = r.Perm(15)[0] + 10
	}

	res := new(strings.Builder)
	res.WriteString("<g>\n")
	for _, x := range distribution {
		fmt.Fprintf(res,
			`  <circle cx="%d" cy="%d" r="%d" style="fill:#ffffff;fill-opacity:%.2f" />`,
			x[0], x[1], x[2], float64(x[3])/100,
		)
		res.WriteString("\n")
	}
	res.WriteString("</g>")
	return res.String()
}
