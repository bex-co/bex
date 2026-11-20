// Copyright 2026.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bex_client_ip_headers

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// serveRedirect runs one request through the httpsRedirect instance and
// reports whether it reached the route behind it.
func serveRedirect(t *testing.T, r *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	passed := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		passed = true
		_, _ = io.WriteString(w, "app")
	})
	h, err := New(context.Background(), next, &Config{HTTPSRedirect: true}, "https")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	// Traefik's entrypoint pre-sets a nil Content-Type to stop net/http's
	// sniffing (contenttype.DisableAutoDetection).
	rec.Header()["Content-Type"] = nil
	h.ServeHTTP(rec, r)
	return rec, passed
}

// w8/357: Render answers plain HTTP with 301 for GET and HEAD and 307 for
// every other method, keeping the path and raw query, and with Go's
// http.Redirect body: HTML for GET, nothing otherwise.
func TestHTTPSRedirectSendsRendersCodePerMethod(t *testing.T) {
	const want = "https://probe.onbex.co/x/a%20b?q=1&r=%2F"
	for _, tc := range []struct {
		method, contentType, body string
		code                      int
	}{
		{http.MethodGet, "text/html; charset=utf-8", "<a href=\"https://probe.onbex.co/x/a%20b?q=1&amp;r=%2F\">Moved Permanently</a>.\n\n", 301},
		{http.MethodHead, "text/html; charset=utf-8", "", 301},
		{http.MethodPost, "", "", 307},
		{http.MethodPut, "", "", 307},
		{http.MethodPatch, "", "", 307},
		{http.MethodDelete, "", "", 307},
		{http.MethodOptions, "", "", 307},
	} {
		r := httptest.NewRequest(tc.method, "/x/a%20b?q=1&r=%2F", nil)
		r.Host = "probe.onbex.co"
		rec, passed := serveRedirect(t, r)
		if passed || rec.Code != tc.code || rec.Header().Get("Location") != want ||
			rec.Header().Get("Content-Type") != tc.contentType || rec.Body.String() != tc.body {
			t.Errorf("%s: passed %v, %d Location %q Content-Type %q body %q; want %d %q %q %q",
				tc.method, passed, rec.Code, rec.Header().Get("Location"), rec.Header().Get("Content-Type"), rec.Body.String(),
				tc.code, want, tc.contentType, tc.body)
		}
	}
}

func TestHTTPSRedirectReadsTheSchemeAndHostAsTraefikDoes(t *testing.T) {
	for _, tc := range []struct {
		name, target, host, proto string
		want                      string // Location; "" passes through
	}{
		{"https passes through", "https://probe.onbex.co/x?q=1", "", "", ""},
		{"https on 443 passes through", "https://probe.onbex.co/x", "probe.onbex.co:443", "", ""},
		{"a WebSocket over TLS passes through", "https://probe.onbex.co/ws", "", "wss", ""},
		{"a WebSocket over plain HTTP redirects", "/ws", "probe.onbex.co", "ws", "https://probe.onbex.co/ws"},
		{"the edge's http overrides the TLS state", "https://probe.onbex.co/x", "", "http", "https://probe.onbex.co/x"},
		{"http drops port 80", "/x?q=1", "probe.onbex.co:80", "", "https://probe.onbex.co/x?q=1"},
		{"https on another port goes to 443", "https://probe.onbex.co:8443/x", "", "", "https://probe.onbex.co/x"},
		{"an absolute-form target keeps its path", "http://probe.onbex.co/a/b?c=%2F", "", "", "https://probe.onbex.co/a/b?c=%2F"},
		{"an empty query mark is kept", "/x?", "probe.onbex.co", "", "https://probe.onbex.co/x?"},
		{"an IPv6 host keeps its brackets", "/x", "[2001:db8::1]:80", "", "https://[2001:db8::1]/x"},
	} {
		r := httptest.NewRequest(http.MethodGet, tc.target, nil)
		if tc.host != "" {
			r.Host = tc.host
		}
		if tc.proto != "" {
			r.Header.Set("X-Forwarded-Proto", tc.proto)
		}
		rec, passed := serveRedirect(t, r)
		if tc.want == "" {
			if !passed || rec.Code != http.StatusOK || rec.Header().Get("Location") != "" {
				t.Errorf("%s: passed %v, %d Location %q; want it served", tc.name, passed, rec.Code, rec.Header().Get("Location"))
			}
			continue
		}
		if passed || rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != tc.want {
			t.Errorf("%s: passed %v, %d Location %q; want 301 to %q", tc.name, passed, rec.Code, rec.Header().Get("Location"), tc.want)
		}
	}
}
