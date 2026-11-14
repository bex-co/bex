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
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// w21/m394 t015 (R25-06, R25-07). The cache charged only len(body) against its
// byte budget, so the key, the cloned headers and the entry's own structure were
// free: 100K unique cacheable 404s accounted 1.8 MiB and used 72 MiB, and the real
// binary reached 250 MiB against a 160 MiB limit. It also passed Traefik a
// forwarding view of one client address and "http".

func newCacheOver(t *testing.T, h http.Handler, cfg Config) *Handler {
	t.Helper()
	srv := httptest.NewServer(h)
	t.Cleanup(srv.Close)
	u, _ := url.Parse(srv.URL)
	return New(u, cfg)
}

// perEntryOverhead is the least the cache may charge an entry beyond its body,
// key and headers: the structure's measured cost was about 756 bytes.
const perEntryOverhead = 756

func TestUniqueNotFoundsStayWithinTheMemoryBound(t *testing.T) {
	notFound := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("no"))
	})
	const maxBytes = 1 << 20
	h := newCacheOver(t, notFound, Config{Profile: ProfileStatic, MaxBytes: maxBytes, MaxObjectBytes: 1 << 10})
	for i := range 5000 {
		get(h, "GET", fmt.Sprintf("http://app.test/missing-%d.css", i), nil)
	}
	if h.size > maxBytes {
		t.Fatalf("accounted size %d is over the %d bound", h.size, maxBytes)
	}
	// Each entry is charged at least its structure, so the byte bound alone
	// bounds how many entries (and so how much real memory) the cache holds.
	if limit := maxBytes / perEntryOverhead; len(h.entries) > limit {
		t.Fatalf("%d entries held, want at most %d: unique 404s must not be free", len(h.entries), limit)
	}
}

func TestAnEntryIsChargedItsKeyAndHeaders(t *testing.T) {
	redirect := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Location", "/elsewhere")
		w.Header().Set("X-Padding", strings.Repeat("h", 500))
		w.WriteHeader(http.StatusMovedPermanently)
	})
	h := newCacheOver(t, redirect, Config{Profile: ProfileAll, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 10})
	path := "/" + strings.Repeat("k", 7000)
	get(h, "GET", "http://app.test"+path, nil)
	if len(h.entries) != 1 {
		t.Fatalf("%d entries, want the redirect cached", len(h.entries))
	}
	if h.size < 7000+500 {
		t.Fatalf("an empty-bodied redirect with a 7 KB key and a 500-byte header is charged %d, want at least 7500", h.size)
	}
}

func TestExpiredEntriesAreReapedWhenAnotherIsStored(t *testing.T) {
	now := time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	notFound := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	})
	h := newCacheOver(t, notFound, Config{Profile: ProfileStatic, MaxBytes: 64 << 20, MaxObjectBytes: 1 << 10, Now: clock})
	for i := range 1000 {
		get(h, "GET", fmt.Sprintf("http://app.test/old-%d.css", i), nil)
	}
	if len(h.entries) != 1000 {
		t.Fatalf("%d entries, want 1000 cached 404s", len(h.entries))
	}
	now = now.Add(4 * time.Minute) // past a 404's 3-minute default
	get(h, "GET", "http://app.test/new.css", nil)
	if len(h.entries) > 1 {
		t.Fatalf("%d entries after the TTL, want the expired 1000 dropped by the next store", len(h.entries))
	}
	var want int64
	for el := h.lru.Front(); el != nil; el = el.Next() {
		want += entryCost(el.Value.(*entry))
	}
	if h.size != want {
		t.Fatalf("accounted size %d, want the live entries' %d", h.size, want)
	}
}

// Behind Traefik the app sees the client address and scheme the edge saw, as
// ADR004 says and as the activator's proxy already passes them.
func TestForwardedHeadersSurviveTheCache(t *testing.T) {
	var seen http.Header
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
		_, _ = w.Write([]byte("ok"))
	})
	h := newCacheOver(t, capture, Config{Profile: ProfileStatic, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 10})

	r := httptest.NewRequest(http.MethodGet, "http://app.test/dynamic", nil) // not a static file: proxied
	r.RemoteAddr = "10.10.0.5:41234"
	r.Header.Set("X-Forwarded-For", "198.51.100.77")
	r.Header.Set("X-Forwarded-Proto", "https")
	r.Header.Set("True-Client-IP", "198.51.100.77")
	h.ServeHTTP(httptest.NewRecorder(), r)

	if got := seen.Get("X-Forwarded-For"); got != "198.51.100.77, 10.10.0.5" {
		t.Errorf("X-Forwarded-For = %q, want the client then the Traefik peer", got)
	}
	if got := seen.Get("X-Forwarded-Proto"); got != "https" {
		t.Errorf("X-Forwarded-Proto = %q, want https", got)
	}
	if got := seen.Get("True-Client-IP"); got != "198.51.100.77" {
		t.Errorf("True-Client-IP = %q, want it kept", got)
	}
}

// A caller outside the trusted edge cannot make the cache lie about the scheme:
// only http and https are passed on.
func TestAnOddForwardedProtoIsNotPassedOn(t *testing.T) {
	var seen http.Header
	capture := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = r.Header.Clone()
	})
	h := newCacheOver(t, capture, Config{Profile: ProfileStatic, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 10})
	r := httptest.NewRequest(http.MethodGet, "http://app.test/dynamic", nil)
	r.RemoteAddr = "10.10.0.5:41234"
	r.Header.Set("X-Forwarded-Proto", "javascript")
	h.ServeHTTP(httptest.NewRecorder(), r)
	if got := seen.Get("X-Forwarded-Proto"); got != "http" {
		t.Fatalf("X-Forwarded-Proto = %q, want the plain-http default for an unknown value", got)
	}
}
