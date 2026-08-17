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
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/sync/semaphore"
)

// blockingOrigin fetches block until release is closed, so a test can hold origin
// fetches open and observe how many misses actually reach the origin. It is
// concurrency-safe (unlike fakeOrigin) for use under -race.
type blockingOrigin struct {
	release chan struct{}
	entered chan string // one send per Get that reaches the origin
	body    []byte
	total   atomic.Int64
}

func (b *blockingOrigin) Get(_ context.Context, key string) (Object, error) {
	b.total.Add(1)
	b.entered <- key
	<-b.release
	return Object{Body: b.body, ContentType: "text/html"}, nil
}

func newConcurrentHandler(origin Origin, gate *fetchGate) *Handler {
	h := New(staticResolver{host: testHost, site: Site{AppID: appID, Revision: rev}}, origin, 1<<20)
	if gate != nil {
		h.gate = gate
	}
	return h
}

func getStatus(h *Handler, path string) int {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://"+testHost+path, nil)
	req.Host = testHost
	h.ServeHTTP(rec, req)
	return rec.Code
}

// TestSingleflightCollapsesConcurrentSameKeyMisses proves finding 12's
// singleflight: a flood of concurrent misses for the same object fetches the
// origin exactly once. Pre-fix, every miss issued its own origin.Get.
func TestSingleflightCollapsesConcurrentSameKeyMisses(t *testing.T) {
	origin := &blockingOrigin{release: make(chan struct{}), entered: make(chan string, 128), body: []byte("home")}
	h := newConcurrentHandler(origin, nil)

	const n = 64
	var reached, done sync.WaitGroup
	reached.Add(n)
	done.Add(n)
	statuses := make([]int32, n)
	for i := range n {
		go func(i int) {
			defer done.Done()
			reached.Done()
			atomic.StoreInt32(&statuses[i], int32(getStatus(h, "/")))
		}(i)
	}

	reached.Wait()
	<-origin.entered // the leader is inside origin.Get, holding the flight open
	// A brief settle lets every straggler reach singleflight.Do and coalesce onto
	// the open flight before the leader is released.
	time.Sleep(100 * time.Millisecond)
	close(origin.release)
	done.Wait()

	if got := origin.total.Load(); got != 1 {
		t.Fatalf("origin fetches = %d, want 1 (singleflight collapse)", got)
	}
	for i := range n {
		if s := atomic.LoadInt32(&statuses[i]); s != http.StatusOK {
			t.Fatalf("request %d => %d, want 200", i, s)
		}
	}
}

// TestFetchGateBoundsConcurrentDistinctKeyFetches proves finding 12's admission
// gate: with a concurrency of 2, only two distinct-key misses reach the origin at
// once; the rest are shed as 503 without buffering the object.
func TestFetchGateBoundsConcurrentDistinctKeyFetches(t *testing.T) {
	origin := &blockingOrigin{release: make(chan struct{}), entered: make(chan string, 128), body: []byte("x")}
	h := newConcurrentHandler(origin, newFetchGate(2, 0)) // 2 concurrent fetches, byte budget off

	paths := []string{"/a.html", "/b.html", "/c.html", "/d.html", "/e.html"}
	const n = 5
	var done sync.WaitGroup
	done.Add(n)
	statuses := make([]int32, n)
	for i := range n {
		go func(i int) {
			defer done.Done()
			atomic.StoreInt32(&statuses[i], int32(getStatus(h, paths[i])))
		}(i)
	}

	// Two misses win the two slots and block in origin.Get.
	<-origin.entered
	<-origin.entered

	// The other three fail the gate and return 503 without touching the origin.
	count503 := func() int {
		c := 0
		for i := range n {
			if atomic.LoadInt32(&statuses[i]) == http.StatusServiceUnavailable {
				c++
			}
		}
		return c
	}
	deadline := time.Now().Add(2 * time.Second)
	for count503() < 3 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	if c := count503(); c != 3 {
		t.Fatalf("503 responses = %d, want 3 (gate sheds distinct-key overload)", c)
	}
	// While the two slots are held, no third fetch reaches the origin.
	select {
	case k := <-origin.entered:
		t.Fatalf("a third fetch reached the origin (%s) past the gate of 2", k)
	case <-time.After(100 * time.Millisecond):
	}
	if got := origin.total.Load(); got != 2 {
		t.Fatalf("origin fetches = %d, want 2 (gate bound)", got)
	}

	close(origin.release)
	done.Wait()
}

func TestFetchGateReservesCapacityForAnotherSite(t *testing.T) {
	g := newFetchGate(3, 0, 2)
	if !g.acquire("site-a") {
		t.Fatal("site A could not acquire its two fair-share slots")
	}
	if !g.acquire("site-a") {
		t.Fatal("site A could not acquire its two fair-share slots")
	}
	if g.acquire("site-a") {
		t.Fatal("site A acquired past its per-site cap")
	}
	if !g.acquire("site-b") {
		t.Fatal("site B could not use the reserved global slot")
	}
	g.release("site-b")
	g.release("site-a")
	g.release("site-a")
}

func TestCacheEnforcesPerSiteBudget(t *testing.T) {
	c := newCache(128 << 20)
	body := make([]byte, 20<<20)
	c.put("site-a", "a/one", Object{Body: body})
	c.put("site-a", "a/two", Object{Body: body})
	c.put("site-b", "b/one", Object{Body: body})

	if _, ok := c.get("b/one"); !ok {
		t.Fatal("site A's cache pressure evicted site B within the global budget")
	}
	if c.siteUsed["site-a"] > 32<<20 {
		t.Fatalf("site A cache use = %d, exceeds 32 MiB share", c.siteUsed["site-a"])
	}
}

// TestCacheChargesEntryMetadata proves the entry-cost model (codex-security
// round 12, finding 2): a zero-byte body still consumes budget — key,
// content type, and the fixed per-entry overhead — so a tenant cannot park
// unbounded zero-cost entries by publishing and warming empty files. The
// per-site budget is the eviction trigger here; the same charge guards the
// global budget.
func TestCacheChargesEntryMetadata(t *testing.T) {
	c := newCache(1 << 20) // per-site share min(1 MiB, 32 MiB) = 1 MiB
	empty := Object{Body: nil, ContentType: "text/plain"}
	// Each entry costs len(key)+len(type)+entryOverhead ≈ 330 bytes, so ~3300
	// zero-byte entries exhaust the 1 MiB per-site budget.
	for i := range 8000 {
		c.put("site-a", fmt.Sprintf("a/rev-1/f%d", i), empty)
	}
	if n := len(c.entries); int64(n)*entryOverhead > c.perSiteBudget+entryOverhead {
		t.Fatalf("cache held %d zero-byte entries past its per-site byte budget", n)
	}
	if c.siteUsed["site-a"] > c.perSiteBudget {
		t.Fatalf("site A cache use = %d exceeds per-site budget %d", c.siteUsed["site-a"], c.perSiteBudget)
	}
}

// blockingWriter is an http.ResponseWriter whose Write blocks until release is
// closed, simulating a slow client that keeps the fetched body live after the
// fetch gate has already released its reservation.
type blockingWriter struct {
	header  http.Header
	code    int
	release chan struct{}
	writing chan struct{} // one send when Write is entered
}

func (w *blockingWriter) Header() http.Header  { return w.header }
func (w *blockingWriter) WriteHeader(code int) { w.code = code }
func (w *blockingWriter) Write(p []byte) (int, error) {
	w.writing <- struct{}{}
	<-w.release
	return len(p), nil
}

// TestLiveBodyLeaseBoundsSlowClientWrites proves the live-body lease: a fetched
// body stays accounted from fetch until the response write completes, so once
// the budget is held by a blocked writer a new distinct fetch is shed as 503
// even though the fetch gate itself has free capacity, and is served again once
// the slow client drains.
func TestLiveBodyLeaseBoundsSlowClientWrites(t *testing.T) {
	origin := newFakeOrigin(map[string]Object{
		key("a.html"): {Body: []byte("12345678"), ContentType: "text/html"},
		key("b.html"): {Body: []byte("12345678"), ContentType: "text/html"},
	})
	h := newConcurrentHandler(origin, newFetchGate(32, 0)) // fetch gate wide open
	h.liveBodies = semaphore.NewWeighted(10)               // one 8-byte body fits, two do not

	blocked := &blockingWriter{header: http.Header{}, release: make(chan struct{}), writing: make(chan struct{}, 1)}
	var done sync.WaitGroup
	done.Go(func() {
		req := httptest.NewRequest(http.MethodGet, "http://"+testHost+"/a.html", nil)
		req.Host = testHost
		h.ServeHTTP(blocked, req)
	})
	<-blocked.writing // the first response holds 8 of the 10 live-body bytes in Write

	// A distinct miss still passes the fetch gate and reaches the origin, but
	// its body cannot be leased: the response is shed as 503 before headers.
	if status := getStatus(h, "/b.html"); status != http.StatusServiceUnavailable {
		t.Fatalf("GET /b.html => %d, want 503 (live-body budget held by a slow client)", status)
	}

	close(blocked.release)
	done.Wait()
	if blocked.code != http.StatusOK {
		t.Fatalf("blocked write finished with %d, want 200", blocked.code)
	}
	// Once the slow client drains, the lease is returned and serving resumes.
	if status := getStatus(h, "/b.html"); status != http.StatusOK {
		t.Fatalf("GET /b.html after release => %d, want 200", status)
	}
}
