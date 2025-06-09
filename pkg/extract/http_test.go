// SPDX-FileCopyrightText: © 2020 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package extract

import (
	"encoding/json"
	"net/http"
	"net/textproto"
	"testing"
	"time"

	"github.com/jarcoal/httpmock"
	"github.com/stretchr/testify/require"
)

func TestClient(t *testing.T) {
	t.Run("Client", func(t *testing.T) {
		assert := require.New(t)
		client := NewClient()
		assert.Equal(10*time.Second, client.Timeout)

		tr := client.Transport.(*Transport)
		assert.Equal("en-US,en;q=0.8", tr.header.Get("Accept-Language"))
		assert.Equal("text/html,application/xhtml+xml,application/xml;q=0.9,image/jpeg,image/png,*/*;q=0.8,application/signed-exchange;v=b3;q=0.7", tr.header.Get("Accept"))

		htr := tr.tr.(*http.Transport)
		assert.False(htr.DisableKeepAlives)
		assert.False(htr.DisableCompression)
		assert.Equal(50, htr.MaxIdleConns)
		assert.True(htr.ForceAttemptHTTP2)
	})

	t.Run("SetHeader", func(t *testing.T) {
		client := NewClient()
		SetHeader(client, "x-test", "abc")
		SetHeader(client, "Accept-Language", "")

		tr := client.Transport.(*Transport)
		require.Equal(t, "abc", tr.header.Get("x-test"))
		_, exists := tr.header[textproto.CanonicalMIMEHeaderKey("Accept-Language")]
		require.False(t, exists)
	})

	t.Run("RoundTrip", func(t *testing.T) {
		type echoResponse struct {
			URL    string
			Method string
			Header http.Header
		}

		httpmock.Activate()
		defer httpmock.DeactivateAndReset()

		httpmock.RegisterResponder("GET", `=~.*`,
			func(req *http.Request) (*http.Response, error) {
				return httpmock.NewJsonResponse(200, echoResponse{
					URL:    req.URL.String(),
					Header: req.Header,
				})
			})

		assert := require.New(t)
		client := NewClient()
		clientHeaders := client.Transport.(*Transport).header

		rsp, err := client.Get("https://example.net/")
		assert.NoError(err)
		defer rsp.Body.Close() //nolint:errcheck

		dec := json.NewDecoder(rsp.Body)
		var data echoResponse
		assert.NoError(dec.Decode(&data))

		assert.Equal("https://example.net/", data.URL)
		assert.Equal(clientHeaders, data.Header)
	})
}
