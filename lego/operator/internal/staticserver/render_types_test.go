/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package staticserver

import (
	"net/http"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/326: .js, .xml and .webmanifest are served Render's type whatever the
// upload stored, including the uploader's own guesses with the charset the
// origin adds.
func TestRenderExtensionTypes(t *testing.T) {
	for _, tc := range []struct{ file, want string }{
		{"/app.js", "application/javascript"},
		{"/_app/immutable/entry/start.B9_t3Ub4.js", "application/javascript"},
		{"/APP.JS", "application/javascript"},
		{"/worker.mjs", "application/javascript"},
		{"/config.cjs", "application/javascript"},
		{"/sitemap.xml", "application/xml"},
		{"/blog/rss.xml", "application/xml"},
		{"/site.webmanifest", "binary/octet-stream"},
	} {
		for _, origin := range []string{"", "binary/octet-stream", "text/javascript", "text/javascript; charset=utf-8", "text/xml; charset=utf-8", "application/manifest+json"} {
			if got := contentType(tc.file, origin); got != tc.want {
				t.Errorf("contentType(%s, %q) = %q, want Render's %q", tc.file, origin, got, tc.want)
			}
		}
	}
	// Every other type captured on Render already matched, and keeps the
	// origin's type.
	for file, origin := range map[string]string{
		"/style.css": "text/css; charset=utf-8", "/index.html": "text/html; charset=utf-8", "/version.json": "application/json",
		"/favicon.svg": "image/svg+xml", "/favicon.ico": "image/vnd.microsoft.icon", "/manifest.json": "application/json",
	} {
		if got := contentType(file, origin); got != origin {
			t.Errorf("contentType(%s, %q) = %q, want it kept", file, origin, got)
		}
	}
}

// Served: .js and .xml stay Brotli-encoded under their new types, as Render
// sends them; a manifest goes out identity, as Render's does; a Content-Type
// header rule still wins over all three (w4/272).
func TestRenderExtensionTypesServed(t *testing.T) {
	js, xml, manifest := repeated("console.log('render');\n"), repeated("<url><loc>/a</loc></url>\n"), repeated(`{"name":"a"}`+"\n")
	h, _ := newTestHandler(t, Site{
		Headers: []appv1alpha1.StaticHeader{{Path: "/custom/*", Name: "Content-Type", Value: "text/javascript; charset=utf-8"}},
	}, map[string]Object{
		key("app.js"):                  {Body: js, ContentType: "text/javascript; charset=utf-8"},
		key("sitemap.xml"):             {Body: xml, ContentType: "text/xml; charset=utf-8"},
		key("site.webmanifest"):        {Body: manifest, ContentType: "application/manifest+json"},
		key("custom/app.js"):           {Body: js, ContentType: "text/javascript; charset=utf-8"},
		key("custom/feed.xml"):         {Body: xml, ContentType: "text/xml; charset=utf-8"},
		key("custom/site.webmanifest"): {Body: manifest, ContentType: "application/manifest+json"},
	})
	for _, tc := range []struct {
		path, want string
		body       []byte
	}{
		{"/app.js", "application/javascript", js},
		{"/sitemap.xml", "application/xml", xml},
	} {
		rec := doWith(h, http.MethodGet, tc.path, brHeader("br, gzip"))
		if got := rec.Header().Get("Content-Type"); got != tc.want {
			t.Errorf("GET %s Content-Type = %q, want %q", tc.path, got, tc.want)
		}
		assertBrotli(t, tc.path, rec, tc.body)
	}
	rec := doWith(h, http.MethodGet, "/site.webmanifest", brHeader("br, gzip"))
	assertIdentity(t, "/site.webmanifest", rec, manifest)
	if got := rec.Header().Get("Content-Type"); got != "binary/octet-stream" {
		t.Errorf("GET /site.webmanifest Content-Type = %q, want binary/octet-stream", got)
	}
	for _, path := range []string{"/custom/app.js", "/custom/feed.xml", "/custom/site.webmanifest"} {
		if got := do(h, http.MethodGet, path).Header().Get("Content-Type"); got != "text/javascript; charset=utf-8" {
			t.Errorf("GET %s under a Content-Type rule = %q, want the rule's type", path, got)
		}
	}
}
