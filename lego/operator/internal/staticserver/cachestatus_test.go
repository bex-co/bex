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
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m703: every answer reads the CF-Cache-Status Render's static sites
// report (captured 2026-11-27): a file MISS when this request fetched it and
// HIT from the cache, HEAD, 304 and 206 as the file, and BYPASS for a missing
// file, the 404.html page, a route redirect and an error; DYNAMIC for any
// other method. A header rule naming the header changes nothing.
func TestEveryAnswerReadsRendersCacheStatus(t *testing.T) {
	rule := appv1alpha1.StaticHeader{Path: "/*", Name: "cf-cache-status", Value: "FORGED"}
	h, origin := newTestHandler(t, Site{
		NoSPAFallback: true,
		Headers:       []appv1alpha1.StaticHeader{rule},
		Routes: []appv1alpha1.StaticRoute{
			{Type: "redirect", Source: "/old", Destination: "/new"},
			{Type: "rewrite", Source: "/app/*", Destination: "/index.html"},
		},
	}, map[string]Object{
		key("index.html"):    {Body: []byte("<h1>home</h1>"), ContentType: "text/html"},
		key("404.html"):      {Body: []byte("<h1>gone</h1>"), ContentType: "text/html"},
		key("favicon.ico"):   {Body: append([]byte{0, 0, 1, 0}, make([]byte, 64)...), ContentType: "image/vnd.microsoft.icon"},
		key("broken/a.json"): {Body: []byte("{}"), ContentType: "application/json"},
	})
	origin.errs[key("broken/a.json")] = errFakeOrigin
	for i, tc := range []struct {
		method, path string
		header       http.Header
		code         int
		status       string
	}{
		{http.MethodGet, "/", nil, 200, "MISS"},
		{http.MethodGet, "/", nil, 200, "HIT"},
		{http.MethodHead, "/index.html", nil, 200, "HIT"},
		{http.MethodHead, "/favicon.ico", nil, 200, "MISS"},
		{http.MethodGet, "/favicon.ico", http.Header{"Range": {"bytes=0-3"}}, 206, "HIT"},
		{http.MethodGet, "/app/deep/link", nil, 200, "HIT"},
		{http.MethodGet, "/missing.png", nil, 404, "BYPASS"},
		{http.MethodHead, "/missing.png", nil, 404, "BYPASS"},
		{http.MethodGet, "/old", nil, 301, "BYPASS"},
		{http.MethodGet, "/broken/a.json", nil, 502, "BYPASS"},
		{http.MethodGet, "/.git/config", nil, 404, "BYPASS"},
		{http.MethodPost, "/", nil, 200, "DYNAMIC"},
		{http.MethodDelete, "/old", nil, 301, "DYNAMIC"},
		{http.MethodOptions, "/missing.png", nil, 200, "DYNAMIC"},
	} {
		rec := doWith(h, tc.method, tc.path, tc.header)
		if rec.Code != tc.code || rec.Header().Values("Cf-Cache-Status")[0] != tc.status || len(rec.Header().Values("Cf-Cache-Status")) != 1 {
			t.Errorf("#%d %s %s: %d CF-Cache-Status %q, want %d %s", i, tc.method, tc.path, rec.Code, rec.Header().Values("Cf-Cache-Status"), tc.code, tc.status)
		}
	}
	etag := do(h, http.MethodGet, "/").Header().Get("Etag")
	if rec := doWith(h, http.MethodGet, "/", http.Header{"If-None-Match": {etag}}); rec.Code != 304 || rec.Header().Get("Cf-Cache-Status") != "HIT" {
		t.Errorf("a matching If-None-Match: %d %q, want 304 HIT", rec.Code, rec.Header().Get("Cf-Cache-Status"))
	}
}

// errFakeOrigin is a transient origin failure, never cached.
var errFakeOrigin = &net.OpError{Op: "dial", Err: net.UnknownNetworkError("down")}

// w4/m703: a site with no 404.html and the implicit SPA fallback answers its
// bare 404 with BYPASS, and its fallback page as the file it is.
func TestBareNotFoundAndSPAFallbackReadTheirStatus(t *testing.T) {
	h, _ := newTestHandler(t, Site{}, map[string]Object{
		key("index.html"): {Body: []byte("<h1>home</h1>"), ContentType: "text/html"},
	})
	for i, tc := range []struct{ path, status string }{
		{"/missing.png", "BYPASS"}, {"/deep/link", "MISS"}, {"/other/link", "HIT"},
	} {
		if got := do(h, http.MethodGet, tc.path).Header().Get("Cf-Cache-Status"); got != tc.status {
			t.Errorf("#%d %s: %q, want %s", i, tc.path, got, tc.status)
		}
	}
}

// w4/m703: a file whose final headers opt out of a CDN's cache reads BYPASS,
// as Render's edge reports such an answer, whether or not it was cached here.
func TestAFileThatOptsOutOfCachingReadsBypass(t *testing.T) {
	for _, tc := range []struct {
		name   string
		header appv1alpha1.StaticHeader
		status string
	}{
		{"no-store", appv1alpha1.StaticHeader{Path: "/*", Name: "Cache-Control", Value: "no-store"}, "BYPASS"},
		{"private", appv1alpha1.StaticHeader{Path: "/*", Name: "Cache-Control", Value: "private, max-age=600"}, "BYPASS"},
		{"no lifetime", appv1alpha1.StaticHeader{Path: "/*", Name: "Cache-Control", Value: "public, max-age=0"}, "BYPASS"},
		{"a Vary past Accept-Encoding", appv1alpha1.StaticHeader{Path: "/*", Name: "Vary", Value: "Origin"}, "BYPASS"},
		{"a year", appv1alpha1.StaticHeader{Path: "/*", Name: "Cache-Control", Value: "public, max-age=31536000, immutable"}, "HIT"},
	} {
		h, _ := newTestHandler(t, Site{Headers: []appv1alpha1.StaticHeader{tc.header}}, map[string]Object{
			key("app.js"): {Body: []byte("console.log(1)"), ContentType: "text/javascript"},
		})
		do(h, http.MethodGet, "/app.js")
		if got := do(h, http.MethodGet, "/app.js").Header().Get("Cf-Cache-Status"); got != tc.status {
			t.Errorf("%s: second read %q, want %s", tc.name, got, tc.status)
		}
	}
}

// w4/m703: an object evicted from the cache between two reads is fetched
// again, and reads MISS again.
func TestAnEvictedFileReadsMissAgain(t *testing.T) {
	h, origin := newTestHandler(t, Site{}, map[string]Object{
		key("index.html"): {Body: []byte("<h1>home</h1>"), ContentType: "text/html"},
	})
	read := func() string { return do(h, http.MethodGet, "/").Header().Get("Cf-Cache-Status") }
	first, second := read(), read()
	h.cache = newCache(1 << 20) // every entry evicted
	statuses := []string{first, second, read()}
	if strings.Join(statuses, ",") != "MISS,HIT,MISS" || origin.gets[key("index.html")] != 2 {
		t.Fatalf("statuses %v after %d origin reads, want MISS,HIT,MISS after 2", statuses, origin.gets[key("index.html")])
	}
}

// w4/m703: requests that wait on another's origin fetch read it as fetched,
// MISS, like the request that started it; the next reads HIT.
func TestACollapsedFetchReadsMissForEveryWaiter(t *testing.T) {
	origin := &blockingOrigin{release: make(chan struct{}), entered: make(chan string, 16), body: []byte("home")}
	h := newConcurrentHandler(origin, nil)
	const n = 8
	statuses := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Go(func() {
			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodGet, "http://"+testHost+"/", nil)
			h.ServeHTTP(rec, req)
			statuses[i] = rec.Header().Get("Cf-Cache-Status")
		})
	}
	<-origin.entered
	time.Sleep(100 * time.Millisecond) // the rest join the open fetch
	close(origin.release)
	wg.Wait()
	for i, got := range statuses {
		if got != "MISS" {
			t.Errorf("request %d: %q, want MISS", i, got)
		}
	}
	if origin.total.Load() != 1 {
		t.Fatalf("%d origin fetches, want the one collapsed fetch", origin.total.Load())
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "http://"+testHost+"/", nil))
	if got := rec.Header().Get("Cf-Cache-Status"); got != "HIT" {
		t.Fatalf("the next read: %q, want HIT", got)
	}
}

// w4/m703: a suspended site's answer reads BYPASS to a GET, as Render's
// suspended static site did, and DYNAMIC to a POST, whatever a rule says.
func TestASuspendedSiteReadsBypass(t *testing.T) {
	h, _ := newTestHandler(t, Site{Unavailable: true, Headers: []appv1alpha1.StaticHeader{{Path: "/*", Name: "CF-Cache-Status", Value: "HIT"}}}, nil)
	for _, tc := range []struct{ method, status string }{{http.MethodGet, "BYPASS"}, {http.MethodHead, "BYPASS"}, {http.MethodPost, "DYNAMIC"}} {
		rec := do(h, tc.method, "/")
		if rec.Code != http.StatusServiceUnavailable || rec.Header().Get("Cf-Cache-Status") != tc.status {
			t.Errorf("%s: %d %q, want 503 %s", tc.method, rec.Code, rec.Header().Get("Cf-Cache-Status"), tc.status)
		}
	}
}

// w4/m703: an external rewrite's answer reads DYNAMIC, whatever its
// destination or a rule sent.
func TestAnExternalRewriteReadsDynamic(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("CF-Cache-Status", "HIT")
		_, _ = w.Write([]byte("api"))
	}))
	defer upstream.Close()
	site := externalSite(upstream.URL + "/v1/*")
	site.Headers = []appv1alpha1.StaticHeader{{Path: "/*", Name: "CF-Cache-Status", Value: "MISS"}}
	h, _ := newTestHandler(t, site, nil)
	h.external = newExternalProxy(func(net.IP) bool { return false })
	for _, method := range []string{http.MethodGet, http.MethodPost} {
		rec := do(h, method, "/api/users")
		if rec.Code != http.StatusOK || rec.Header().Values("Cf-Cache-Status")[0] != "DYNAMIC" || len(rec.Header().Values("Cf-Cache-Status")) != 1 {
			t.Errorf("%s: %d %q, want 200 DYNAMIC", method, rec.Code, rec.Header().Values("Cf-Cache-Status"))
		}
	}
}
