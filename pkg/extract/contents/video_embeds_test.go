// SPDX-FileCopyrightText: © 2025 Mislav Marohnić <hi@mislav.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package contents

import (
	"net/http"
	"strings"
	"testing"

	"codeberg.org/readeck/readeck/pkg/extract"
	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestExtractor_ConvertVideoEmbeds(t *testing.T) {
	assert := require.New(t)
	httpmock.Activate()
	defer httpmock.DeactivateAndReset()

	httpmock.RegisterResponder("GET", "/page1", httpmock.NewStringResponder(200, `
		<html>
		<body>
		<p>This is a video:</p>
		<iframe width="560" height="315" src="https://www.youtube.com/embed/haAimDKxo40?si=XZY" title="YouTube video player" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share" referrerpolicy="strict-origin-when-cross-origin" allowfullscreen=""></iframe>
		<p>Conclusion</p>
		</body>
		</html>
		`).HeaderSet(http.Header{"content-type": {"text/html"}}))

	ex, _ := extract.New("http://example.net/page1")
	ex.AddProcessors(ConvertVideoEmbeds)
	ex.Run()
	assert.Empty(ex.Errors())

	finalHTML := strings.TrimSpace(string(ex.HTML))
	assert.Equal(`<!-- page 1 -->

		<p>This is a video:</p>
		<figure><a href="https://www.youtube.com/watch?v=haAimDKxo40" data-readeck-video-iframe-src="https://www.youtube-nocookie.com/embed/haAimDKxo40?si=XZY"><img alt="YouTube video" src="https://i.ytimg.com/vi/haAimDKxo40/hqdefault.jpg"/></a><figcaption><a href="https://www.youtube.com/watch?v=haAimDKxo40">https://www.youtube.com/watch?v=haAimDKxo40</a></figcaption></figure>
		<p>Conclusion</p>`, finalHTML)
}
