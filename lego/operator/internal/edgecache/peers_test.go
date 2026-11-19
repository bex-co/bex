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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// w3/m489. A service's cache tier runs two replicas that share one cache:
// each key has one owner, the other replica forwards to it, so the app sees
// one fill per key for the whole service and both replicas answer HIT.

// recorder is an app that counts its requests and keeps the headers of each.
type recorder struct {
	hits  atomic.Int64
	delay time.Duration
	mu    sync.Mutex
	seen  map[string][]http.Header
}

func (o *recorder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	o.hits.Add(1)
	o.mu.Lock()
	if o.seen == nil {
		o.seen = map[string][]http.Header{}
	}
	o.seen[r.URL.Path] = append(o.seen[r.URL.Path], r.Header.Clone())
	o.mu.Unlock()
	time.Sleep(o.delay)
	w.Header().Set("Cache-Control", "max-age=600")
	_, _ = w.Write([]byte("body:" + r.URL.Path))
}

func (o *recorder) headers(path string) []http.Header {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.seen[path]
}

// replica is one tier pod: a Handler behind its own server, which counts the
// requests reaching it and those that arrived as a hop.
type replica struct {
	h          *Handler
	srv        *httptest.Server
	addr       string
	served     atomic.Int64
	hopsServed atomic.Int64
}

func newTier(t *testing.T, app http.Handler) (a, b *replica) {
	t.Helper()
	upstream := httptest.NewServer(app)
	t.Cleanup(upstream.Close)
	u, _ := url.Parse(upstream.URL)
	tier := []*replica{{}, {}}
	for _, rep := range tier {
		rep.h = mustNew(t, u, Config{Profile: ProfileAll, MaxBytes: 1 << 20, MaxObjectBytes: 1 << 16})
		rep.srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rep.served.Add(1)
			if r.Header.Get(HopHeader) != "" {
				rep.hopsServed.Add(1)
			}
			rep.h.ServeHTTP(w, r)
		}))
		t.Cleanup(rep.srv.Close)
		rep.addr = rep.srv.Listener.Addr().String()
	}
	peers := []string{tier[0].addr, tier[1].addr}
	for _, rep := range tier {
		rep.h.SetPeers(rep.addr, peers)
	}
	return tier[0], tier[1]
}

// request is what the edge sends a replica for path: the service's host, and
// an explicit encoding preference so the client adds none.
func request(method, path string) *http.Request {
	r := httptest.NewRequest(method, "http://app.test"+path, nil)
	r.Header.Set("Accept-Encoding", "identity")
	return r
}

// ownedBy is a path of the given extension whose resource rep owns.
func ownedBy(t *testing.T, rep, other *replica, ext string, skip ...string) string {
	t.Helper()
	for i := range 1000 {
		path := fmt.Sprintf("/k%d%s", i, ext)
		key := resourceKey(request("GET", path))
		if other.h.owner(key) == rep.addr && rep.h.owner(key) == "" && !slices.Contains(skip, path) {
			return path
		}
	}
	t.Fatal("no key owned by the replica")
	return ""
}

// try sends the edge's GET for path to rep's server.
func (rep *replica) try(path string, header map[string]string) (*http.Response, string, error) {
	r, err := http.NewRequest(http.MethodGet, rep.srv.URL+path, nil)
	if err != nil {
		return nil, "", err
	}
	r.Host = "app.test"
	r.Header.Set("Accept-Encoding", "identity")
	for k, v := range header {
		r.Header.Set(k, v)
	}
	resp, err := rep.srv.Client().Do(r)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()
	body, err := io.ReadAll(resp.Body)
	return resp, string(body), err
}

func (rep *replica) do(t *testing.T, path string, header map[string]string) (*http.Response, string) {
	t.Helper()
	resp, body, err := rep.try(path, header)
	if err != nil {
		t.Fatal(err)
	}
	return resp, body
}

func storedCount(h *Handler) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.entries)
}

func TestATierFillsEachKeyOnceThroughEitherReplica(t *testing.T) {
	app := &recorder{delay: 50 * time.Millisecond}
	a, b := newTier(t, app)
	path := ownedBy(t, b, a, ".css")
	var wg sync.WaitGroup
	for i := range 12 {
		rep := []*replica{a, b}[i%2]
		wg.Go(func() {
			if resp, body, err := rep.try(path, nil); err != nil || resp.StatusCode != http.StatusOK || body != "body:"+path {
				t.Errorf("got %v %q: %v", resp, body, err)
			}
		})
	}
	wg.Wait()
	for _, rep := range []*replica{a, b, a, b} {
		resp, body := rep.do(t, path, nil)
		if resp.Header.Get(StatusHeader) != statusHit || body != "body:"+path || len(resp.Header.Values("Age")) != 1 {
			t.Fatalf("a later GET = %s %q Age %v, want the stored page as a HIT", resp.Header.Get(StatusHeader), body, resp.Header.Values("Age"))
		}
	}
	if app.hits.Load() != 1 {
		t.Fatalf("app hits = %d, want the key filled once for the whole tier", app.hits.Load())
	}
	if storedCount(a.h) != 0 || storedCount(b.h) != 1 {
		t.Fatalf("stored %d on the forwarder and %d on the owner, want the key on its owner only", storedCount(a.h), storedCount(b.h))
	}
}

func TestAReplicaServesTheRequestItselfWhenTheOwnerIsGone(t *testing.T) {
	app := &recorder{}
	a, b := newTier(t, app)
	path := ownedBy(t, b, a, ".css")
	b.srv.Close()
	for _, want := range []string{statusMiss, statusHit} {
		resp, body := a.do(t, path, nil)
		if resp.StatusCode != http.StatusOK || body != "body:"+path || resp.Header.Get(StatusHeader) != want {
			t.Fatalf("with the owner gone = %d %s %q, want the page as a %s", resp.StatusCode, resp.Header.Get(StatusHeader), body, want)
		}
	}
	if app.hits.Load() != 1 {
		t.Fatalf("app hits = %d, want 1", app.hits.Load())
	}
}

func TestOnlyAPeerCanMarkAHop(t *testing.T) {
	a, b := newTier(t, &recorder{})
	forged := request("GET", ownedBy(t, b, a, ".css"))
	forged.RemoteAddr = "203.0.113.9:5555"
	forged.Header.Set(HopHeader, "1")
	w := httptest.NewRecorder()
	a.h.ServeHTTP(w, forged)
	if w.Code != http.StatusOK || w.Header().Get(StatusHeader) != statusMiss {
		t.Fatalf("a forged hop = %d %s", w.Code, w.Header().Get(StatusHeader))
	}
	if b.served.Load() != 1 || storedCount(a.h) != 0 || storedCount(b.h) != 1 {
		t.Fatalf("a hop header from outside the tier: owner served %d, stored %d here and %d on the owner; want it ignored and the request forwarded",
			b.served.Load(), storedCount(a.h), storedCount(b.h))
	}

	path := ownedBy(t, b, a, ".css", forged.URL.Path)
	hop := request("GET", path)
	hop.RemoteAddr = "127.0.0.1:5555" // a peer's host
	hop.Header.Set(HopHeader, "1")
	w = httptest.NewRecorder()
	a.h.ServeHTTP(w, hop)
	if w.Code != http.StatusOK || w.Header().Get(StatusHeader) != statusMiss {
		t.Fatalf("a peer's hop = %d %s", w.Code, w.Header().Get(StatusHeader))
	}
	if b.served.Load() != 1 || storedCount(a.h) != 1 {
		t.Fatalf("a peer's hop: owner served %d, stored %d here; want it served here and never forwarded again", b.served.Load(), storedCount(a.h))
	}
}

// The app sees the same client address, host and scheme whichever replica
// the edge reached, and never the replicas' hop marker.
func TestTheAppSeesTheSameForwardingHeadersWhicheverReplicaTheEdgeReached(t *testing.T) {
	app := &recorder{}
	a, b := newTier(t, app)
	direct := ownedBy(t, a, b, ".css")
	hopped := ownedBy(t, a, b, ".css", direct)
	edge := map[string]string{"X-Forwarded-For": "198.51.100.77", "X-Forwarded-Proto": "https", "X-Forwarded-Host": "spoofed.test"}
	a.do(t, direct, edge)
	b.do(t, hopped, edge)
	if a.hopsServed.Load() != 1 {
		t.Fatalf("the owner served %d hops, want the second request to have taken one", a.hopsServed.Load())
	}
	forwarding := func(path string) []string {
		seen := app.headers(path)
		if len(seen) != 1 {
			t.Fatalf("the app saw %s %d times, want once", path, len(seen))
		}
		if v := seen[0].Values(HopHeader); v != nil {
			t.Errorf("the app saw %s: %v", HopHeader, v)
		}
		return []string{seen[0].Get("X-Forwarded-For"), seen[0].Get("X-Forwarded-Host"), seen[0].Get("X-Forwarded-Proto")}
	}
	want := []string{"198.51.100.77, 127.0.0.1", "app.test", "https"}
	if got := forwarding(direct); !slices.Equal(got, want) {
		t.Errorf("served by its owner, the app saw %q, want %q", got, want)
	}
	if got := forwarding(hopped); !slices.Equal(got, want) {
		t.Errorf("after a hop, the app saw %q, want %q", got, want)
	}
}

func TestARequestTheCacheCannotShareIsNeverForwarded(t *testing.T) {
	a, b := newTier(t, &recorder{})
	page := ownedBy(t, b, a, "")
	for _, tc := range []struct {
		method string
		header map[string]string
	}{
		{"POST", nil},
		{"GET", map[string]string{"Authorization": "Bearer x"}},
		{"GET", map[string]string{"Range": "bytes=0-1"}},
		{"GET", map[string]string{"Upgrade": "websocket"}},
	} {
		r := request(tc.method, page)
		for k, v := range tc.header {
			r.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		a.h.ServeHTTP(w, r)
		if w.Header().Get(StatusHeader) != statusDynamic {
			t.Errorf("%s %v = %s, want DYNAMIC", tc.method, tc.header, w.Header().Get(StatusHeader))
		}
	}
	for _, profile := range []string{ProfileStatic, ProfileNone} {
		a.h.SetProfile(profile)
		w := httptest.NewRecorder()
		a.h.ServeHTTP(w, request("GET", page))
		if w.Header().Get(StatusHeader) != statusDynamic {
			t.Errorf("an HTML page on %q = %s, want DYNAMIC", profile, w.Header().Get(StatusHeader))
		}
	}
	if b.served.Load() != 0 {
		t.Fatalf("the owner served %d requests the cache cannot share, want none forwarded", b.served.Load())
	}
	a.h.SetProfile(ProfileAll)
	a.h.ServeHTTP(httptest.NewRecorder(), request("GET", page))
	if b.served.Load() != 1 {
		t.Fatalf("the owner served %d after an eligible GET, want it forwarded", b.served.Load())
	}
}

// Rendezvous hashing: every replica that knows the same peers picks the same
// owner whatever their order, each peer owns a share, and losing a peer moves
// only the keys it owned.
func TestOwnershipIsSharedAndStable(t *testing.T) {
	addrs := []string{"10.0.0.1:8080", "10.0.0.2:8080", "10.0.0.3:8080"}
	view := func(self string, peers []string) *Handler {
		h := &Handler{}
		h.SetPeers(self, peers)
		return h
	}
	owner := func(h *Handler, self, key string) string {
		if o := h.owner(key); o != "" {
			return o
		}
		return self
	}
	backwards := slices.Clone(addrs)
	slices.Reverse(backwards)
	first, reversed := view(addrs[0], addrs), view(addrs[2], backwards)
	without := view(addrs[0], addrs[:2])
	shares := map[string]int{}
	const keys = 3000
	for i := range keys {
		key := fmt.Sprintf("app.test\x00/k%d\x00false", i)
		o := owner(first, addrs[0], key)
		if r := owner(reversed, addrs[2], key); r != o {
			t.Fatalf("key %d: owner %s from one replica, %s from another", i, o, r)
		}
		if w := owner(without, addrs[0], key); o != addrs[2] && w != o {
			t.Fatalf("key %d moved from %s to %s when another peer left", i, o, w)
		}
		shares[o]++
	}
	for _, addr := range addrs {
		if n := shares[addr]; n < keys/5 || n > keys/2 {
			t.Errorf("%s owns %d of %d keys, want about a third", addr, n, keys)
		}
	}
	if (&Handler{}).owner("k") != "" || view(addrs[0], addrs[:1]).owner("k") != "" {
		t.Fatal("a replica alone, or without peers, must serve every key itself")
	}
}

func TestRunPeersFollowsTheTiersAddresses(t *testing.T) {
	h := &Handler{}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	lookup := func(_ context.Context, name string) ([]string, error) {
		calls++
		if name != "web-cache-peers" {
			t.Errorf("resolved %q", name)
		}
		switch calls {
		case 1:
			return []string{"10.0.0.2", "fd00::1", "10.0.0.1", "10.0.0.2"}, nil
		case 2, 3:
			return nil, errors.New("no such host")
		}
		cancel()
		return nil, context.Canceled
	}
	var logged []string
	RunPeers(ctx, h, "web-cache-peers", "10.0.0.1", 8080, time.Millisecond, lookup, func(format string, args ...any) {
		logged = append(logged, fmt.Sprintf(format, args...))
	})
	set := h.peers.Load()
	if want := []string{"10.0.0.1:8080", "10.0.0.2:8080", "[fd00::1]:8080"}; set == nil || set.self != "10.0.0.1:8080" || !slices.Equal(set.addrs, want) {
		t.Fatalf("peers %+v, want self 10.0.0.1:8080 among %q kept through the failed lookups", set, want)
	}
	want := []string{
		"edge-cache: peers [10.0.0.1:8080 10.0.0.2:8080 [fd00::1]:8080] (self 10.0.0.1:8080)",
		"edge-cache: resolve peers web-cache-peers: no such host",
	}
	if !slices.Equal(logged, want) {
		t.Fatalf("logged\n  %q\nwant\n  %q", logged, want)
	}
}
