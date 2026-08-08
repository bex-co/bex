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
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// blockingOrigin fetches block until release is closed, so a test can hold origin
// fetches open and observe how many misses actually reach the origin. It is
// concurrency-safe (unlike fakeOrigin) for use under -race.
type blockingOrigin struct {
	release chan struct{}
	entered chan string // one send per Get that reaches the origin
	body    []byte
	total   int64
}

func (b *blockingOrigin) Get(_ context.Context, key string) (Object, error) {
	atomic.AddInt64(&b.total, 1)
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

	if got := atomic.LoadInt64(&origin.total); got != 1 {
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
	if got := atomic.LoadInt64(&origin.total); got != 2 {
		t.Fatalf("origin fetches = %d, want 2 (gate bound)", got)
	}

	close(origin.release)
	done.Wait()
}
