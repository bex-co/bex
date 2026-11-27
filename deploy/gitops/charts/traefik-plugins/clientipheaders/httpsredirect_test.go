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
	"strconv"
	"testing"
	"time"
)

// serveRedirect runs one request through the httpsRedirect instance and
// reports whether it reached the route behind it.
func serveRedirect(t *testing.T, r *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	return serveRedirectWith(t, &Config{HTTPSRedirect: true}, r)
}

// serveRedirectWith is serveRedirect with the instance's config, behind what
// the entrypoint's sanitizer sets on every answer.
func serveRedirectWith(t *testing.T, config *Config, r *http.Request) (*httptest.ResponseRecorder, bool) {
	t.Helper()
	passed := false
	next := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		passed = true
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, "app")
	})
	h, err := New(context.Background(), next, config, "https")
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	// Traefik's entrypoint pre-sets a nil Content-Type to stop net/http's
	// sniffing (contenttype.DisableAutoDetection).
	rec.Header()["Content-Type"] = nil
	rec.Header().Set("Cf-Ray", "0123456789abcdef-FRA")
	rec.Header().Set("Rndr-Id", "01234567-89ab-cdef")
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
			if !passed || rec.Code != http.StatusNotFound || rec.Header().Get("Location") != "" {
				t.Errorf("%s: passed %v, %d Location %q; want it served", tc.name, passed, rec.Code, rec.Header().Get("Location"))
			}
			continue
		}
		if passed || rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != tc.want {
			t.Errorf("%s: passed %v, %d Location %q; want 301 to %q", tc.name, passed, rec.Code, rec.Header().Get("Location"), tc.want)
		}
	}
}

// w4/m703: Render's edge answers a redirect itself, with the request's cf-ray
// and no rndr-id. A web service's and the unknown host's read DYNAMIC for
// every method, with no Age or Vary, and so does a static site's 307.
func TestHTTPSRedirectAnswersRendersEdgeHeaders(t *testing.T) {
	methods := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	for _, tc := range []struct {
		name    string
		config  *Config
		methods []string
	}{
		{"a web service", &Config{HTTPSRedirect: true}, methods},
		{"the unknown host", &Config{HTTPSRedirect: true, PlatformFallback: true}, methods},
		{"a static site's 307", &Config{HTTPSRedirect: true, StaticSite: true}, methods[2:]},
	} {
		for _, method := range tc.methods {
			r := httptest.NewRequest(method, "/x?q=1", nil)
			r.Host = "edge-" + method + ".onbex.co"
			rec, passed := serveRedirectWith(t, tc.config, r)
			h := rec.Header()
			if passed || rec.Code/100 != 3 || h.Get("Cf-Cache-Status") != "DYNAMIC" || h.Get("Cf-Ray") != "0123456789abcdef-FRA" ||
				len(h.Values("Rndr-Id")) != 0 || len(h.Values("Age")) != 0 || len(h.Values("Vary")) != 0 {
				t.Errorf("%s %s: passed %v, %d %v; want a redirect reading DYNAMIC with the ray and no Rndr-Id, Age or Vary", tc.name, method, passed, rec.Code, h)
			}
		}
	}
}

// w4/m703: a static site's GET and HEAD redirect read what Render's CDN
// reported for its stored copy: MISS the first time, then HIT with an Age, the
// two methods sharing the copy, and BYPASS with a query, which stores none.
// Each answer varies by encoding and carries no rndr-id.
func TestStaticSiteRedirectReadsTheStoredCopy(t *testing.T) {
	config := &Config{HTTPSRedirect: true, StaticSite: true}
	for i, tc := range []struct {
		method, target, host, status, age string
	}{
		{http.MethodGet, "/w4-m703/a%20b?q=1", "redirect-memo.onbex.co", "BYPASS", ""},
		{http.MethodGet, "/w4-m703/a%20b", "redirect-memo.onbex.co", "MISS", ""},
		{http.MethodHead, "/w4-m703/a%20b", "Redirect-Memo.onbex.co:80", "HIT", "0"},
		{http.MethodGet, "/w4-m703/a%20b?q=1", "redirect-memo.onbex.co", "BYPASS", ""},
		{http.MethodGet, "/w4-m703/a%20b/", "redirect-memo.onbex.co", "MISS", ""},
		{http.MethodGet, "/w4-m703/a%20b", "other-memo.onbex.co", "MISS", ""},
	} {
		r := httptest.NewRequest(tc.method, tc.target, nil)
		r.Host = tc.host
		rec, passed := serveRedirectWith(t, config, r)
		h := rec.Header()
		if passed || rec.Code != http.StatusMovedPermanently || h.Get("Cf-Cache-Status") != tc.status || h.Get("Age") != tc.age ||
			h.Get("Vary") != "accept-encoding" || h.Get("Cf-Ray") != "0123456789abcdef-FRA" || len(h.Values("Rndr-Id")) != 0 {
			t.Errorf("#%d %s %s%s: passed %v, %d %v; want 301 %s Age %q, Vary accept-encoding, the ray and no Rndr-Id",
				i, tc.method, tc.host, tc.target, passed, rec.Code, h, tc.status, tc.age)
		}
	}
}

// w4/m703: the stored copy lives staticRedirectTTL, Cloudflare's default for
// a 301, then reads EXPIRED once and is stored afresh. The Age counts whole
// seconds since it was stored.
func TestRedirectMemoExpiresAsCloudflareDoes(t *testing.T) {
	m := &redirectMemo{seen: map[string]time.Time{}}
	start := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	for i, tc := range []struct {
		after  time.Duration
		status string
		age    int64
	}{
		{0, "MISS", -1},
		{1500 * time.Millisecond, "HIT", 1},
		{staticRedirectTTL - time.Second, "HIT", int64((staticRedirectTTL - time.Second) / time.Second)},
		{staticRedirectTTL, "EXPIRED", -1},
		{staticRedirectTTL + 7*time.Second, "HIT", 7},
	} {
		status, age := m.status("site.onbex.co/x", false, start.Add(tc.after))
		if status != tc.status || age != tc.age {
			t.Errorf("#%d at +%v: %s Age %d, want %s Age %d", i, tc.after, status, age, tc.status, tc.age)
		}
	}
}

// w4/m703: the memo never holds more than maxStaticRedirects. A full memo
// drops its expired copies first, then arbitrary ones, and the URL it forgot
// reads MISS again.
func TestRedirectMemoIsBounded(t *testing.T) {
	m := &redirectMemo{seen: map[string]time.Time{}}
	start := time.Date(2026, 11, 27, 12, 0, 0, 0, time.UTC)
	for i := 0; i < maxStaticRedirects; i++ {
		at := start
		if i%2 == 0 {
			at = start.Add(-staticRedirectTTL) // stored two hours earlier
		}
		m.seen["old.onbex.co/"+strconv.Itoa(i)] = at
	}
	if status, _ := m.status("new.onbex.co/0", false, start); status != "MISS" || len(m.seen) != maxStaticRedirects/2+1 {
		t.Fatalf("%s with %d held; want MISS with only the %d fresh copies and the new one", status, len(m.seen), maxStaticRedirects/2)
	}
	for i := 1; i <= maxStaticRedirects; i++ {
		m.status("new.onbex.co/"+strconv.Itoa(i), false, start)
		if len(m.seen) > maxStaticRedirects {
			t.Fatalf("%d held after %d new URLs, past the bound %d", len(m.seen), i, maxStaticRedirects)
		}
	}
	forgotten := 0
	for i := 1; i < maxStaticRedirects; i += 2 {
		if _, ok := m.seen["old.onbex.co/"+strconv.Itoa(i)]; !ok {
			forgotten++
		}
	}
	if forgotten == 0 {
		t.Fatal("no fresh copy was evicted, though the memo filled twice")
	}
}

// w4/m703: the unknown host's 404 reads DYNAMIC and carries no rndr-id, as
// Render's no-such-service answer does, while a tenant route's passed-through
// answer keeps its rndr-id and gets no status from the redirect.
func TestFallbackMarksItsPassedThroughAnswer(t *testing.T) {
	for _, tc := range []struct {
		name          string
		config        *Config
		status, rndID string
	}{
		{"the unknown host", &Config{HTTPSRedirect: true, PlatformFallback: true}, "DYNAMIC", ""},
		{"a web service", &Config{HTTPSRedirect: true}, "", "01234567-89ab-cdef"},
		{"a static site", &Config{HTTPSRedirect: true, StaticSite: true}, "", "01234567-89ab-cdef"},
	} {
		for _, method := range []string{http.MethodGet, http.MethodHead, http.MethodPost} {
			rec, passed := serveRedirectWith(t, tc.config, httptest.NewRequest(method, "https://nobody.onbex.co/x", nil))
			h := rec.Header()
			if !passed || rec.Code != http.StatusNotFound || h.Get("Cf-Cache-Status") != tc.status || h.Get("Rndr-Id") != tc.rndID ||
				h.Get("Cf-Ray") != "0123456789abcdef-FRA" {
				t.Errorf("%s %s: passed %v, %d %v; want the route's 404 with status %q and Rndr-Id %q", tc.name, method, passed, rec.Code, h, tc.status, tc.rndID)
			}
		}
	}
}
