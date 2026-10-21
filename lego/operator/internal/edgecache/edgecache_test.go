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

package edgecache

import (
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type origin struct {
	hits   atomic.Int64
	header http.Header
	status int
	delay  time.Duration
}

func (o *origin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.hits.Add(1)
	time.Sleep(o.delay)
	maps.Copy(w.Header(), o.header)
	w.WriteHeader(o.status)
	_, _ = w.Write([]byte("body:" + r.URL.Path))
}

func newCache(t *testing.T, profile string, o *origin, now func() time.Time) *Handler {
	t.Helper()
	if o.status == 0 {
		o.status = http.StatusOK
	}
	srv := httptest.NewServer(o)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return New(u, Config{Profile: profile, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16, Now: now})
}

func get(h http.Handler, method, target string, header map[string]string) *httptest.ResponseRecorder {
	r := httptest.NewRequest(method, target, nil)
	for k, v := range header {
		r.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	return w
}

func TestStaticProfileCachesRendersCommonFileTypesOnly(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileStatic, o, nil)
	if s := get(h, "GET", "http://app.test/app.css", nil).Header().Get(StatusHeader); s != "MISS" {
		t.Fatalf("first .css = %s, want MISS", s)
	}
	w := get(h, "GET", "http://app.test/app.css", nil)
	if w.Header().Get(StatusHeader) != "HIT" || w.Body.String() != "body:/app.css" {
		t.Fatalf("second .css = %s %q, want a HIT with the body", w.Header().Get(StatusHeader), w.Body.String())
	}
	if s := get(h, "GET", "http://app.test/page", nil).Header().Get(StatusHeader); s != "DYNAMIC" {
		t.Fatalf("an HTML page on the static profile = %s, want DYNAMIC", s)
	}
	get(h, "GET", "http://app.test/page", nil)
	if o.hits.Load() != 3 {
		t.Fatalf("origin hits = %d, want 3 (one css, two pages)", o.hits.Load())
	}
}

func TestAllProfileCachesPagesAndHeadIsServedFromCache(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileAll, o, nil)
	get(h, "GET", "http://app.test/page", nil)
	w := get(h, "HEAD", "http://app.test/page", nil)
	if w.Header().Get(StatusHeader) != "HIT" || w.Body.Len() != 0 {
		t.Fatalf("HEAD = %s body %q, want a HIT with no body", w.Header().Get(StatusHeader), w.Body.String())
	}
	if o.hits.Load() != 1 {
		t.Fatalf("origin hits = %d, want 1", o.hits.Load())
	}
}

func TestResponsesThatOptOutAreNeverStored(t *testing.T) {
	for name, header := range map[string]http.Header{
		"Set-Cookie":               {"Set-Cookie": {"s=1"}},
		"no-store":                 {"Cache-Control": {"no-store"}},
		"private":                  {"Cache-Control": {"private, max-age=600"}},
		"max-age=0":                {"Cache-Control": {"max-age=0"}},
		"Vary on a cookie":         {"Vary": {"Cookie"}},
		"CDN no-store wins":        {"CDN-Cache-Control": {"no-store"}, "Cache-Control": {"max-age=600"}},
		"s-maxage=0 beats max-age": {"Cache-Control": {"max-age=600, s-maxage=0"}},
	} {
		t.Run(name, func(t *testing.T) {
			o := &origin{header: header}
			h := newCache(t, ProfileAll, o, nil)
			get(h, "GET", "http://app.test/x", nil)
			if s := get(h, "GET", "http://app.test/x", nil).Header().Get(StatusHeader); s == "HIT" {
				t.Fatal("a response that opts out must not be served from cache")
			}
			if o.hits.Load() != 2 {
				t.Fatalf("origin hits = %d, want 2", o.hits.Load())
			}
		})
	}
}

func TestRequestsThatCannotBeSharedBypassTheCache(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileAll, o, nil)
	for _, tc := range []struct {
		method string
		header map[string]string
	}{
		{"POST", nil},
		{"GET", map[string]string{"Authorization": "Bearer x"}},
		{"GET", map[string]string{"Range": "bytes=0-1"}},
	} {
		if s := get(h, tc.method, "http://app.test/x", tc.header).Header().Get(StatusHeader); s != "DYNAMIC" {
			t.Errorf("%s %v = %s, want DYNAMIC", tc.method, tc.header, s)
		}
	}
}

func TestTTLFollowsDirectivesAndStatusDefaults(t *testing.T) {
	clock := time.Unix(0, 0)
	now := func() time.Time { return clock }
	o := &origin{header: http.Header{"Cache-Control": {"max-age=60"}}}
	h := newCache(t, ProfileAll, o, now)
	get(h, "GET", "http://app.test/x", nil)
	clock = clock.Add(59 * time.Second)
	if s := get(h, "GET", "http://app.test/x", nil).Header().Get(StatusHeader); s != "HIT" {
		t.Fatalf("within max-age = %s, want HIT", s)
	}
	clock = clock.Add(2 * time.Second)
	if s := get(h, "GET", "http://app.test/x", nil).Header().Get(StatusHeader); s != "EXPIRED" {
		t.Fatalf("past max-age = %s, want EXPIRED", s)
	}

	notFound := &origin{status: http.StatusNotFound}
	h = newCache(t, ProfileAll, notFound, now)
	get(h, "GET", "http://app.test/gone", nil)
	clock = clock.Add(3*time.Minute + time.Second)
	if s := get(h, "GET", "http://app.test/gone", nil).Header().Get(StatusHeader); s != "EXPIRED" {
		t.Fatalf("a 404 past Render's 3-minute default = %s, want EXPIRED", s)
	}
	if ttl, _ := responseTTL(&http.Response{StatusCode: 500, Header: http.Header{"Cache-Control": {"max-age=600"}}}); ttl != 0 {
		t.Fatalf("a 500 is never cached, whatever its directives say; got %s", ttl)
	}
}

func TestConcurrentMissesCollapseIntoOneOriginRequest(t *testing.T) {
	o := &origin{delay: 50 * time.Millisecond}
	h := newCache(t, ProfileAll, o, nil)
	var wg sync.WaitGroup
	for range 10 {
		wg.Go(func() {
			if w := get(h, "GET", "http://app.test/hot", nil); w.Code != http.StatusOK {
				t.Errorf("status %d", w.Code)
			}
		})
	}
	wg.Wait()
	if o.hits.Load() != 1 {
		t.Fatalf("origin hits = %d, want the 10 concurrent misses collapsed into 1", o.hits.Load())
	}
}

func TestTheCacheStaysWithinItsByteBound(t *testing.T) {
	o := &origin{status: http.StatusOK}
	srv := httptest.NewServer(o)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	h := New(u, Config{Profile: ProfileAll, MaxBytes: 20, MaxObjectBytes: 1 << 10})
	get(h, "GET", "http://app.test/aaaa", nil) // "body:/aaaa" = 10 bytes
	get(h, "GET", "http://app.test/bbbb", nil)
	get(h, "GET", "http://app.test/cccc", nil) // evicts /aaaa
	if h.size > 20 {
		t.Fatalf("cache size %d over its 20-byte bound", h.size)
	}
	if s := get(h, "GET", "http://app.test/aaaa", nil).Header().Get(StatusHeader); s == "HIT" {
		t.Fatal("the least recently used entry must have been evicted")
	}
}

// The review cases (w5/m151): each would have served every visitor a broken
// response.
func TestAHeadMissNeverPoisonsGet(t *testing.T) {
	o := &origin{}
	h := newCache(t, ProfileAll, o, nil)
	get(h, "HEAD", "http://app.test/page", nil)
	w := get(h, "GET", "http://app.test/page", nil)
	if w.Body.String() != "body:/page" {
		t.Fatalf("GET after a HEAD miss = %q, want the full body", w.Body.String())
	}
}

func TestValidatorsNeverStoreA304(t *testing.T) {
	var sawValidator atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=600")
		if r.Header.Get("If-None-Match") != "" {
			sawValidator.Store(true)
			w.WriteHeader(http.StatusNotModified)
			return
		}
		_, _ = w.Write([]byte("page"))
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	h := New(u, Config{Profile: ProfileAll, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	get(h, "GET", "http://app.test/x", map[string]string{"If-None-Match": `"v1"`})
	w := get(h, "GET", "http://app.test/x", nil)
	if sawValidator.Load() || w.Code != http.StatusOK || w.Body.String() != "page" {
		t.Fatalf("validator reached origin=%v; next GET = %d %q, want 200 page", sawValidator.Load(), w.Code, w.Body.String())
	}
}

func TestTheOriginSeesTheKeysEncodingPreference(t *testing.T) {
	var got []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		got = append(got, r.Header.Get("Accept-Encoding"))
		mu.Unlock()
		_, _ = w.Write([]byte("x"))
	}))
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	h := New(u, Config{Profile: ProfileAll, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
	get(h, "GET", "http://app.test/a", map[string]string{"Accept-Encoding": "br, gzip"})
	get(h, "GET", "http://app.test/b", map[string]string{"Accept-Encoding": "gzip;q=0, br"})
	if len(got) != 2 || got[0] != "gzip" || got[1] != "identity" {
		t.Fatalf("origin saw Accept-Encoding %v, want [gzip identity]", got)
	}
}

func TestEveryCacheControlLineCounts(t *testing.T) {
	o := &origin{header: http.Header{"Cache-Control": {"max-age=60", "private"}}}
	h := newCache(t, ProfileAll, o, nil)
	get(h, "GET", "http://app.test/x", nil)
	if s := get(h, "GET", "http://app.test/x", nil).Header().Get(StatusHeader); s == "HIT" {
		t.Fatal("a private directive on a second Cache-Control line must keep the page out of the cache")
	}
}

func TestAnEventStreamIsNeverCachedAndReleasesWaiters(t *testing.T) {
	o := &origin{header: http.Header{"Content-Type": {"text/event-stream"}}, delay: 30 * time.Millisecond}
	h := newCache(t, ProfileAll, o, nil)
	var wg sync.WaitGroup
	for range 3 {
		wg.Go(func() { get(h, "GET", "http://app.test/events", nil) })
	}
	wg.Wait()
	if s := get(h, "GET", "http://app.test/events", nil).Header().Get(StatusHeader); s == "HIT" {
		t.Fatal("an event stream must never be served from cache")
	}
}
