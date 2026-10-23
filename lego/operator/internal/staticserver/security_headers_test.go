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
	"strings"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's static sites send HSTS and nosniff on every file (w8/194); an error
// response gets nosniff only, and a saved header rule replaces either default.
func TestStaticResponsesCarryRendersSecurityHeaders(t *testing.T) {
	h, _ := newTestHandler(t, Site{Headers: []appv1alpha1.StaticHeader{
		{Path: "/custom/*", Name: "Strict-Transport-Security", Value: "max-age=60"},
		{Path: "/custom/*", Name: "X-Content-Type-Options", Value: "custom"},
	}}, map[string]Object{
		key("index.html"):      {Body: []byte("<p>hi</p>"), ContentType: "text/html"},
		key("app.js"):          {Body: []byte("x"), ContentType: "text/javascript"},
		key("custom/app.js"):   {Body: []byte("x"), ContentType: "text/javascript"},
		key("img/logo.png"):    {Body: []byte("png"), ContentType: "image/png"},
		key("404.html"):        {Body: []byte("<p>nope</p>"), ContentType: "text/html"},
		key("docs/index.html"): {Body: []byte("<p>docs</p>"), ContentType: "text/html"},
	})
	const hsts = "max-age=315360000; includeSubdomains; preload"
	check := func(label string, rec interface{ Header() http.Header }, wantHSTS, wantSniff string) {
		t.Helper()
		if got := rec.Header().Get("Strict-Transport-Security"); got != wantHSTS {
			t.Errorf("%s Strict-Transport-Security = %q, want %q", label, got, wantHSTS)
		}
		if got := rec.Header().Get("X-Content-Type-Options"); got != wantSniff {
			t.Errorf("%s X-Content-Type-Options = %q, want %q", label, got, wantSniff)
		}
	}
	for _, p := range []string{"/", "/app.js", "/img/logo.png", "/docs/"} {
		check("GET "+p, doWith(h, http.MethodGet, p, nil), hsts, "nosniff")
	}
	check("HEAD", doWith(h, http.MethodHead, "/app.js", nil), hsts, "nosniff")
	check("206", doWith(h, http.MethodGet, "/app.js", http.Header{"Range": {"bytes=0-0"}}), hsts, "nosniff")
	first := doWith(h, http.MethodGet, "/app.js", nil)
	rec304 := doWith(h, http.MethodGet, "/app.js", http.Header{"If-None-Match": {first.Header().Get("Etag")}})
	if rec304.Code != http.StatusNotModified {
		t.Fatalf("conditional GET => %d, want 304", rec304.Code)
	}
	check("304", rec304, hsts, "nosniff")
	check("custom rule", doWith(h, http.MethodGet, "/custom/app.js", nil), "max-age=60", "custom")

	miss := doWith(h, http.MethodGet, "/missing.js", nil)
	if miss.Code != http.StatusNotFound {
		t.Fatalf("miss => %d", miss.Code)
	}
	check("404", miss, "", "nosniff")
	if strings.Contains(strings.Join(miss.Header().Values("Strict-Transport-Security"), ","), "max-age") {
		t.Error("an error response carries HSTS")
	}
}
