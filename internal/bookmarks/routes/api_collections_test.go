// SPDX-FileCopyrightText: © 2021 Olivier Meunier <olivier@neokraft.net>
//
// SPDX-License-Identifier: AGPL-3.0-only

package routes_test

import (
	"testing"

	. "codeberg.org/readeck/readeck/internal/testing" //revive:disable:dot-imports
)

func TestCollectionAPI(t *testing.T) {
	app := NewTestApp(t)
	defer func() {
		app.Close(t)
	}()

	client := NewClient(t, app)

	RunRequestSequence(t, client, "user",
		RequestTest{
			JSON:         true,
			Target:       "/api/bookmarks/collections",
			ExpectStatus: 200,
			ExpectJSON:   `[]`,
		},
		RequestTest{
			Method:       "POST",
			Target:       "/api/bookmarks/collections",
			JSON:         map[string]interface{}{},
			ExpectStatus: 422,
			ExpectJSON: `{
				"is_valid": false,
				"errors": null,
				"fields": {
					"author": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": null
					},
					"bf": {
						"is_null": true,
						"is_bound": false,
						"value": false,
						"errors": null
					},
					"has_errors": {
						"is_null": true,
						"is_bound": false,
						"value": false,
						"errors": null
					},
					"has_labels": {
						"is_null": true,
						"is_bound": false,
						"value": false,
						"errors": null
					},
					"id": {
						"is_null": true,
						"is_bound": false,
						"value": null,
						"errors": null
					},
					"is_archived": {
						"is_null": true,
						"is_bound": false,
						"value": false,
						"errors": null
					},
					"is_loaded": {
						"is_null": true,
						"is_bound": false,
						"value": false,
						"errors": null
					},
					"is_marked": {
						"is_null": true,
						"is_bound": false,
						"value": false,
						"errors": null
					},
					"is_pinned": {
						"is_null": true,
						"is_bound": false,
						"value": false,
						"errors": null
					},
					"labels": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": null
					},
					"name": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": [
							"field is required"
						]
					},
					"search": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": null
					},
					"site": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": null
					},
					"range_end": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": null
					},
					"range_start": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": null
					},
					"read_status": {
						"is_null": true,
						"is_bound": false,
						"value": null,
						"errors": null
					},
					"title": {
						"is_null": true,
						"is_bound": false,
						"value": "",
						"errors": null
					},
					"type": {
						"is_null": true,
						"is_bound": false,
						"value": null,
						"errors": null
					}
				}
			}`,
		},
		RequestTest{
			Method: "POST",
			Target: "/api/bookmarks/collections",
			JSON: map[string]interface{}{
				"name":      "test-collection",
				"is_marked": true,
				"type":      []string{"article"},
				"labels":    "test 🥳",
			},
			ExpectStatus:   201,
			ExpectRedirect: "/api/bookmarks/collections/.+",
			ExpectJSON:     `{"status":201,"message":"Collection created"}`,
		},
		RequestTest{
			JSON:         true,
			Target:       "{{ (index .History 0).Redirect }}",
			ExpectStatus: 200,
			ExpectJSON: `{
				"id": "<<PRESENCE>>",
				"href": "<<PRESENCE>>",
				"created": "<<PRESENCE>>",
				"updated": "<<PRESENCE>>",
				"name": "test-collection",
				"is_pinned": false,
				"is_deleted": false,
				"search":"",
				"title":"",
				"author":"",
				"site":"",
				"type": ["article"],
				"labels":"test 🥳",
				"read_status": null,
				"is_marked": true,
				"is_archived": null,
				"is_loaded": null,
				"has_errors": null,
				"has_labels": null,
				"range_start": "",
				"range_end": ""
			}`,
		},
		RequestTest{
			Method: "PATCH",
			Target: "{{ (index .History 0).Path }}",
			JSON: map[string]interface{}{
				"name":      "new name",
				"is_pinned": true,
			},
			ExpectStatus: 200,
			ExpectJSON: `{
				"id": "<<PRESENCE>>",
				"is_pinned": true,
				"name": "new name",
				"updated": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			JSON:         true,
			Target:       "/api/bookmarks/collections",
			ExpectStatus: 200,
			ExpectJSON: `[
				{
					"id": "<<PRESENCE>>",
					"href": "<<PRESENCE>>",
					"created": "<<PRESENCE>>",
					"updated": "<<PRESENCE>>",
					"name": "new name",
					"is_pinned": true,
					"is_deleted": false,
					"search":"",
					"title":"",
					"author":"",
					"site":"",
					"type": ["article"],
					"labels":"test 🥳",
					"read_status": null,
					"is_marked": true,
					"is_archived": null,
					"is_loaded": null,
					"has_errors": null,
					"has_labels": null,
					"range_start": "",
					"range_end": ""
				}
			]`,
		},
		RequestTest{
			Method: "PATCH",
			Target: "{{ (index .History 1).Path }}",
			JSON: map[string]interface{}{
				"name":        "new name",
				"is_archived": nil,
				"is_marked":   false,
			},
			ExpectStatus: 200,
			ExpectJSON: `{
				"id": "<<PRESENCE>>",
				"is_archived": null,
				"is_marked": false,
				"name": "new name",
				"updated": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			JSON:         true,
			Target:       "{{ (index .History 0).Path }}",
			ExpectStatus: 200,
			ExpectJSON: `{
				"id": "<<PRESENCE>>",
				"href": "<<PRESENCE>>",
				"created": "<<PRESENCE>>",
				"updated": "<<PRESENCE>>",
				"name": "new name",
				"is_pinned": true,
				"is_deleted": false,
				"search":"",
				"title":"",
				"author":"",
				"site":"",
				"type": ["article"],
				"labels":"test 🥳",
				"read_status": null,
				"is_marked": false,
				"is_archived": null,
				"is_loaded": null,
				"has_errors": null,
				"has_labels": null,
				"range_start": "",
				"range_end": ""
			}`,
		},
		RequestTest{
			Method: "PATCH",
			Target: "{{ (index .History 0).Path }}",
			JSON: map[string]interface{}{
				"name":        "new name",
				"is_archived": nil,
				"is_marked":   nil,
			},
			ExpectStatus: 200,
			ExpectJSON: `{
				"id": "<<PRESENCE>>",
				"is_archived": null,
				"is_marked": null,
				"name": "new name",
				"updated": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			JSON:         true,
			Target:       "{{ (index .History 0).Path }}",
			ExpectStatus: 200,
			ExpectJSON: `{
				"id": "<<PRESENCE>>",
				"href": "<<PRESENCE>>",
				"created": "<<PRESENCE>>",
				"updated": "<<PRESENCE>>",
				"name": "new name",
				"is_pinned": true,
				"is_deleted": false,
				"search":"",
				"title":"",
				"author":"",
				"site":"",
				"type": ["article"],
				"labels":"test 🥳",
				"read_status": null,
				"is_marked": null,
				"is_archived": null,
				"is_loaded": null,
				"has_errors": null,
				"has_labels": null,
				"range_start": "",
				"range_end": ""
			}`,
		},
		RequestTest{
			Method: "PATCH",
			Target: "{{ (index .History 0).Path }}",
			JSON: map[string]interface{}{
				"name":        "new name",
				"search":      "some search title:tt label:label1 label:label2 site:example.com",
				"type":        []string{"article", "video"},
				"read_status": []string{"unread", "reading"},
			},
			ExpectStatus: 200,
			ExpectJSON: `{
				"id": "<<PRESENCE>>",
				"labels":"label1 label2 test 🥳",
				"name": "new name",
				"read_status": ["unread", "reading"],
				"search":"some search",
				"site":"example.com",
				"title":"tt",
				"type": ["article", "video"],
				"updated": "<<PRESENCE>>"
			}`,
		},
		RequestTest{
			JSON:         true,
			Target:       "{{ (index .History 0).Path }}",
			ExpectStatus: 200,
			ExpectJSON: `{
				"id": "<<PRESENCE>>",
				"href": "<<PRESENCE>>",
				"created": "<<PRESENCE>>",
				"updated": "<<PRESENCE>>",
				"name": "new name",
				"is_pinned": true,
				"is_deleted": false,
				"search":"some search",
				"title":"tt",
				"author":"",
				"site":"example.com",
				"type": ["article", "video"],
				"labels":"label1 label2 test 🥳",
				"read_status": ["unread", "reading"],
				"is_marked": null,
				"is_archived": null,
				"is_loaded": null,
				"has_errors": null,
				"has_labels": null,
				"range_start": "",
				"range_end": ""
			}`,
		},
		RequestTest{
			JSON:         true,
			Method:       "DELETE",
			Target:       "{{ (index .History 0).Path }}",
			ExpectStatus: 204,
		},
		RequestTest{
			JSON:         true,
			Target:       "{{ (index .History 0).Path }}",
			ExpectStatus: 404,
		},
	)
}
