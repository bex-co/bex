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
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"
)

// w3/075. Traefik adds Accept-Encoding: gzip to a GET that came without one
// (Go's transport does so for every method but HEAD) and passes a HEAD as it
// came. So a client's bare GET stored the gzip key, its bare HEAD looked up the
// identity key, which no public GET fills, and a HEAD miss was proxied as a
// HEAD and never filled: every bare `curl -I` of a cached path read MISS and
// reached the app, where Cloudflare, Render's CDN, answers it from cache.

// edgeGzip is the Accept-Encoding the edge sends with a bare client's GET.
var edgeGzip = map[string]string{"Accept-Encoding": "gzip"}

func TestABareHeadReadsThePageTheEdgesGzipGetStored(t *testing.T) {
	clock := time.Unix(1_800_000_000, 0)
	now := func() time.Time { return clock }
	for name, o := range map[string]*origin{
		"without Content-Encoding":        {header: http.Header{"Cache-Control": {"max-age=600"}, "ETag": {`"v1"`}}},
		"with Content-Encoding: identity": page(`"v1"`),
	} {
		t.Run(name, func(t *testing.T) {
			h := newCache(t, ProfileStatic, o, now)
			if w := get(h, "GET", "http://app.test/x.css", edgeGzip); w.Header().Get(StatusHeader) != statusMiss {
				t.Fatalf("the edge's GET = %s, want the fill", w.Header().Get(StatusHeader))
			}
			clock = clock.Add(7 * time.Second)
			w := get(h, "HEAD", "http://app.test/x.css", nil)
			if w.Code != http.StatusOK || w.Body.Len() != 0 || w.Header().Get(StatusHeader) != statusHit {
				t.Fatalf("a bare HEAD = %d %s with %d body bytes, want the stored page's header as a HIT", w.Code, w.Header().Get(StatusHeader), w.Body.Len())
			}
			if got := w.Header().Values("Age"); len(got) != 1 || got[0] != "7" {
				t.Errorf("the HEAD's Age = %v, want [7]", got)
			}
			if got := w.Header().Get("Content-Length"); got != strconv.Itoa(len("body:/x.css")) {
				t.Errorf("the HEAD's Content-Length = %q, want the GET's", got)
			}
			if !unencoded(w.Header()) {
				t.Errorf("a bare HEAD reads Content-Encoding %q", w.Header().Values("Content-Encoding"))
			}
			w = get(h, "HEAD", "http://app.test/x.css", map[string]string{"If-None-Match": `"v1"`})
			assertNotModified(t, w, statusHit, `"v1"`)
			if got := w.Header().Values("Age"); len(got) != 1 || got[0] != "7" {
				t.Errorf("the conditional HEAD's Age = %v, want [7]", got)
			}
			if o.hits.Load() != 1 {
				t.Fatalf("origin hits = %d, want 1: both HEADs are the cache's to answer", o.hits.Load())
			}
		})
	}
}

// codingOrigin compresses for a client that takes gzip, as nginx's gzip and
// Express's compression do, and records the method and Accept-Encoding of
// every request that reaches it.
type codingOrigin struct {
	extra http.Header
	mu    sync.Mutex
	seen  []string
}

func (o *codingOrigin) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.mu.Lock()
	o.seen = append(o.seen, r.Method+" "+r.Header.Get("Accept-Encoding"))
	o.mu.Unlock()
	maps.Copy(w.Header(), o.extra)
	w.Header().Set("Cache-Control", "max-age=600")
	w.Header().Set("ETag", `"v1"`)
	w.Header().Set("Vary", "Accept-Encoding")
	body := []byte("body:" + r.URL.Path)
	if acceptsGzip(r.Header.Get("Accept-Encoding")) {
		var buf bytes.Buffer
		zw := gzip.NewWriter(&buf)
		_, _ = zw.Write(body)
		_ = zw.Close()
		body = buf.Bytes()
		w.Header().Set("Content-Encoding", "gzip")
	}
	w.Header().Set("Content-Length", strconv.Itoa(len(body)))
	if r.Method != http.MethodHead {
		_, _ = w.Write(body)
	}
}

func (o *codingOrigin) requests() []string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return slices.Clone(o.seen)
}

// serveCache runs a cache in front of app behind a real server, so a HEAD is
// answered with HEAD's wire semantics.
func serveCache(t *testing.T, app http.Handler) *httptest.Server {
	t.Helper()
	upstream := httptest.NewServer(app)
	t.Cleanup(upstream.Close)
	u, _ := url.Parse(upstream.URL)
	srv := httptest.NewServer(mustNew(t, u, Config{Profile: ProfileStatic, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16}))
	t.Cleanup(srv.Close)
	return srv
}

// edgeClient adds no Accept-Encoding: a request carries the edge's own.
var edgeClient = &http.Client{Transport: &http.Transport{DisableCompression: true}}

// send sends method for path to srv with header, as the edge would, and
// returns the response and its body.
func send(t *testing.T, srv *httptest.Server, method, path string, header map[string]string) (*http.Response, string) {
	t.Helper()
	r, err := http.NewRequest(method, srv.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	r.Host = "app.test"
	for k, v := range header {
		r.Header.Set(k, v)
	}
	resp, err := edgeClient.Do(r)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return resp, string(body)
}

// A gzip page answers no bare HEAD: it would state an encoding the client
// never accepted, and Traefik decompresses only the GETs it added gzip to. The
// HEAD fills its own key with a GET instead, and reads HIT from then on.
func TestABareHeadNeverReadsAGzipPage(t *testing.T) {
	o := &codingOrigin{}
	srv := serveCache(t, o)
	identity := strconv.Itoa(len("body:/x.css"))
	if resp, _ := send(t, srv, "GET", "/x.css", edgeGzip); resp.Header.Get("Content-Encoding") != "gzip" || resp.Header.Get(StatusHeader) != statusMiss {
		t.Fatalf("the edge's GET = %s %q, want the app's gzip page as the fill", resp.Header.Get(StatusHeader), resp.Header.Get("Content-Encoding"))
	}
	for _, want := range []string{statusMiss, statusHit} {
		resp, body := send(t, srv, "HEAD", "/x.css", nil)
		if resp.StatusCode != http.StatusOK || body != "" || resp.Header.Get(StatusHeader) != want {
			t.Fatalf("a bare HEAD = %d %s %q, want %s", resp.StatusCode, resp.Header.Get(StatusHeader), body, want)
		}
		if v := resp.Header.Values("Content-Encoding"); v != nil || resp.Header.Get("Content-Length") != identity {
			t.Fatalf("a bare HEAD (%s) reads Content-Encoding %q, Content-Length %q, want the identity page's (%s bytes)", want, v, resp.Header.Get("Content-Length"), identity)
		}
	}
	if resp, body := send(t, srv, "GET", "/x.css", map[string]string{"Accept-Encoding": "identity"}); body != "body:/x.css" || resp.Header.Get(StatusHeader) != statusHit {
		t.Fatalf("an identity GET after the HEAD's fill = %s %q, want the identity page as a HIT", resp.Header.Get(StatusHeader), body)
	}
	if resp, _ := send(t, srv, "HEAD", "/x.css", edgeGzip); resp.Header.Get("Content-Encoding") != "gzip" || resp.Header.Get(StatusHeader) != statusHit {
		t.Fatalf("a gzip HEAD = %s %q, want the gzip page's header as a HIT", resp.Header.Get(StatusHeader), resp.Header.Get("Content-Encoding"))
	}
	if got, want := o.requests(), []string{"GET gzip", "GET identity"}; !slices.Equal(got, want) {
		t.Fatalf("the app saw %q, want %q: the bare HEAD's miss filled once, with a GET", got, want)
	}
}

// Cloudflare fills a HEAD miss for a cacheable request with a GET, stores the
// page and returns its header alone
// (developers.cloudflare.com/cache/concepts/cache-behavior).
func TestAHeadMissIsFilledWithAGet(t *testing.T) {
	o := &codingOrigin{}
	srv := serveCache(t, o)
	resp, body := send(t, srv, "HEAD", "/a.css", nil)
	if resp.StatusCode != http.StatusOK || body != "" || resp.Header.Get(StatusHeader) != statusMiss || resp.ContentLength != int64(len("body:/a.css")) {
		t.Fatalf("a HEAD miss = %d %s %q Content-Length %d, want the GET's header as a MISS", resp.StatusCode, resp.Header.Get(StatusHeader), body, resp.ContentLength)
	}
	if resp, body := send(t, srv, "GET", "/a.css", map[string]string{"Accept-Encoding": "identity"}); body != "body:/a.css" || resp.Header.Get(StatusHeader) != statusHit {
		t.Fatalf("a GET after the HEAD's fill = %s %q, want the whole page as a HIT", resp.Header.Get(StatusHeader), body)
	}

	resp, body = send(t, srv, "HEAD", "/b.css", map[string]string{"If-None-Match": `"v1"`})
	if resp.StatusCode != http.StatusNotModified || body != "" || resp.Header.Get(StatusHeader) != statusMiss {
		t.Fatalf("a matching conditional HEAD miss = %d %s %q, want a 304 MISS", resp.StatusCode, resp.Header.Get(StatusHeader), body)
	}
	if resp, body := send(t, srv, "GET", "/b.css", map[string]string{"Accept-Encoding": "identity"}); body != "body:/b.css" || resp.Header.Get(StatusHeader) != statusHit {
		t.Fatalf("a GET after the conditional HEAD's fill = %s %q, want the whole page as a HIT", resp.Header.Get(StatusHeader), body)
	}
	if got, want := o.requests(), []string{"GET identity", "GET identity"}; !slices.Equal(got, want) {
		t.Fatalf("the app saw %q, want %q: one GET per path, no HEAD", got, want)
	}

	optOut := serveCache(t, &codingOrigin{extra: http.Header{"Set-Cookie": {"s=1"}}})
	for range 2 {
		resp, body := send(t, optOut, "HEAD", "/c.css", nil)
		if resp.StatusCode != http.StatusOK || body != "" || resp.Header.Get(StatusHeader) != statusBypass || resp.Header.Get("Set-Cookie") != "s=1" {
			t.Fatalf("a HEAD of a page that opts out = %d %s %q, want its header as a BYPASS", resp.StatusCode, resp.Header.Get(StatusHeader), body)
		}
	}
}

// A HEAD whose GET the cache does not keep reads none of its body: a HEAD of a
// large file must not download it from the app.
func TestAHeadReadsNoBodyTheCacheDoesNotKeep(t *testing.T) {
	const size = 64 << 20
	sent := make(chan int64, 1)
	app := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=600")
		w.Header().Set("Content-Length", strconv.Itoa(size))
		chunk := make([]byte, 32<<10)
		var n int64
		for n < size {
			m, err := w.Write(chunk)
			n += int64(m)
			if err != nil {
				break
			}
		}
		sent <- n
	})
	srv := serveCache(t, app)
	resp, body := send(t, srv, "HEAD", "/big.zip", nil)
	if resp.StatusCode != http.StatusOK || body != "" || resp.ContentLength != size {
		t.Fatalf("a HEAD of a large file = %d %q Content-Length %d, want the GET's header", resp.StatusCode, body, resp.ContentLength)
	}
	select {
	case n := <-sent:
		if n >= size {
			t.Fatalf("the app sent all %d bytes of a body the cache does not keep", n)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("the app is still sending a body nobody reads")
	}
}

// In a tier, the edge's gzip GET and a bare HEAD of the same path have
// different keys and so, keyed alone, could have different owners: the HEAD's
// would never see the GET's page. Both go to the resource's owner.
func TestABareHeadReadsTheEdgesGzipGetThroughEitherReplica(t *testing.T) {
	app := &recorder{}
	a, b := newTier(t, app)
	const paths = 16
	for i := range paths {
		path := fmt.Sprintf("/h%d.css", i)
		if resp, _ := send(t, a.srv, "GET", path, edgeGzip); resp.Header.Get(StatusHeader) != statusMiss {
			t.Fatalf("the edge's GET of %s = %s, want the fill", path, resp.Header.Get(StatusHeader))
		}
		for _, rep := range []*replica{a, b} {
			resp, body := send(t, rep.srv, "HEAD", path, nil)
			if resp.StatusCode != http.StatusOK || body != "" || resp.Header.Get(StatusHeader) != statusHit || len(resp.Header.Values("Age")) != 1 {
				t.Fatalf("a bare HEAD of %s = %d %s Age %v, want the stored page's header as a HIT", path, resp.StatusCode, resp.Header.Get(StatusHeader), resp.Header.Values("Age"))
			}
		}
	}
	if app.hits.Load() != paths {
		t.Fatalf("app hits = %d, want one fill per path for the whole tier", app.hits.Load())
	}
}
