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

	. "github.com/onsi/gomega"
)

// w21/067: a visitor's first cold page after a deploy asks for its HTML,
// stylesheets, scripts and images at once. With the per-client cap at 3 the
// extras were shed at once with 503, which a browser does not retry for a
// subresource, and Render's CDN never answers that. An over-cap miss now waits
// briefly for a slot (admission.go) and still never holds one while it waits.

// latencyOrigin answers every key after a fixed delay and records how many Gets
// ran at once.
type latencyOrigin struct {
	delay time.Duration
	cur   atomic.Int64
	peak  atomic.Int64
}

func (o *latencyOrigin) Get(_ context.Context, _ string) (Object, error) {
	n := o.cur.Add(1)
	defer o.cur.Add(-1)
	for {
		p := o.peak.Load()
		if n <= p || o.peak.CompareAndSwap(p, n) {
			break
		}
	}
	time.Sleep(o.delay)
	return Object{Body: []byte("ok"), ContentType: "text/css"}, nil
}

func serve(h *Handler, host, path, client string) *httptest.ResponseRecorder {
	return serveCtx(context.Background(), h, host, path, client)
}

func serveCtx(ctx context.Context, h *Handler, host, path, client string) *httptest.ResponseRecorder {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil).WithContext(ctx)
	req.Host = host
	req.Header.Set("True-Client-IP", client)
	h.ServeHTTP(rec, req)
	return rec
}

// coldPage has one client fetch n distinct cold assets of a site at once and
// returns how many answered 200 and how many 503.
func coldPage(h *Handler, site, client string, n int) (ok, shed int) {
	var wg sync.WaitGroup
	var okCount, shedCount atomic.Int64
	for i := range n {
		wg.Go(func() {
			switch serve(h, site+".onbex.co", fmt.Sprintf("/asset%d.css", i), client).Code {
			case http.StatusOK:
				okCount.Add(1)
			case http.StatusServiceUnavailable:
				shedCount.Add(1)
			}
		})
	}
	wg.Wait()
	return int(okCount.Load()), int(shedCount.Load())
}

func TestAColdPageOfTwelveAssetsLoadsWithoutA503(t *testing.T) {
	g := NewWithT(t)

	// Without the wait the extras are shed: this is what a visitor saw.
	origin := &latencyOrigin{delay: 20 * time.Millisecond}
	h := newMultiSiteHandler(origin)
	h.admission.wait = 0
	_, shed := coldPage(h, "site", "198.51.100.1", 12)
	g.Expect(shed).To(BeNumerically(">", 0), "the control: a cold page of 12 against a cap of 3 sheds without a wait")

	origin = &latencyOrigin{delay: 20 * time.Millisecond}
	h = newMultiSiteHandler(origin)
	ok, shed := coldPage(h, "site", "198.51.100.1", 12)
	g.Expect(ok).To(Equal(12))
	g.Expect(shed).To(BeZero())
	g.Expect(origin.peak.Load()).To(BeNumerically("<=", int64(defaultMaxClientFetches)), "the client cap still bounds what the origin sees")
	g.Expect(h.admission.waiting).To(BeZero())
	g.Expect(h.admission.byClient).To(BeEmpty())
}

func TestAWaitIsShortAndEndsInTheUsualShed(t *testing.T) {
	g := NewWithT(t)
	origin := &holdOrigin{release: make(chan struct{})}
	h := newMultiSiteHandler(origin)
	h.admission.wait = 80 * time.Millisecond

	var held sync.WaitGroup
	for i := range defaultMaxClientFetches {
		held.Go(func() { serve(h, "site.onbex.co", fmt.Sprintf("/hold%d.html", i), "198.51.100.1") })
	}
	g.Eventually(func() int64 { return origin.entered.Load() }, 5*time.Second, time.Millisecond).Should(Equal(int64(defaultMaxClientFetches)))

	start := time.Now()
	rec := serve(h, "site.onbex.co", "/extra.html", "198.51.100.1")
	elapsed := time.Since(start)
	g.Expect(rec.Code).To(Equal(http.StatusServiceUnavailable))
	g.Expect(rec.Header().Get("Retry-After")).To(Equal("1"))
	g.Expect(elapsed).To(BeNumerically(">=", 70*time.Millisecond), "it waited")
	g.Expect(elapsed).To(BeNumerically("<", time.Second), "and not longer")
	g.Expect(h.admission.waiting).To(BeZero())

	close(origin.release)
	held.Wait()
}

func TestWaitersAreBoundedPerClientAndPerReplica(t *testing.T) {
	g := NewWithT(t)
	origin := &holdOrigin{release: make(chan struct{})}
	h := newMultiSiteHandler(origin)
	h.admission.wait = 400 * time.Millisecond
	h.admission.maxPerClient = 2
	h.admission.maxWaiters = 3

	var all sync.WaitGroup
	for i := range defaultMaxClientFetches {
		all.Go(func() { serve(h, "site.onbex.co", fmt.Sprintf("/hold%d.html", i), "198.51.100.1") })
	}
	g.Eventually(func() int64 { return origin.entered.Load() }, 5*time.Second, time.Millisecond).Should(Equal(int64(defaultMaxClientFetches)))

	// Two requests of the same client wait; a third is shed without waiting.
	for i := range 2 {
		all.Go(func() { serve(h, "site.onbex.co", fmt.Sprintf("/wait%d.html", i), "198.51.100.1") })
	}
	g.Eventually(func() int { h.admission.mu.Lock(); defer h.admission.mu.Unlock(); return h.admission.waiting }, 5*time.Second, time.Millisecond).Should(Equal(2))
	start := time.Now()
	g.Expect(serve(h, "site.onbex.co", "/third.html", "198.51.100.1").Code).To(Equal(http.StatusServiceUnavailable))
	g.Expect(time.Since(start)).To(BeNumerically("<", 100*time.Millisecond), "over the client's waiter cap it is shed at once")

	// The replica's waiter cap (3) holds across clients: another client's
	// waiter takes the last place, the next one is shed at once.
	for i := range defaultMaxClientFetches {
		all.Go(func() { serve(h, "other.onbex.co", fmt.Sprintf("/hold%d.html", i), "198.51.100.2") })
	}
	g.Eventually(func() int64 { return origin.entered.Load() }, 5*time.Second, time.Millisecond).Should(Equal(int64(2 * defaultMaxClientFetches)))
	all.Go(func() { serve(h, "other.onbex.co", "/wait.html", "198.51.100.2") })
	g.Eventually(func() int { h.admission.mu.Lock(); defer h.admission.mu.Unlock(); return h.admission.waiting }, 5*time.Second, time.Millisecond).Should(Equal(3))
	start = time.Now()
	g.Expect(serve(h, "other.onbex.co", "/shed.html", "198.51.100.2").Code).To(Equal(http.StatusServiceUnavailable))
	g.Expect(time.Since(start)).To(BeNumerically("<", 100*time.Millisecond))

	close(origin.release)
	all.Wait()
	g.Expect(h.admission.waiting).To(BeZero())
	g.Expect(h.admission.byClient).To(BeEmpty())
}

// A waiting client holds no slot: another client's cold miss goes straight
// through while the first is parked at its cap (R25-05 still holds).
func TestAWaitingClientDoesNotKeepOthersWaiting(t *testing.T) {
	g := NewWithT(t)
	origin := &holdOrigin{release: make(chan struct{})}
	h := newMultiSiteHandler(origin)
	h.admission.wait = 2 * time.Second

	var all sync.WaitGroup
	for i := range 3 * defaultMaxClientFetches {
		all.Go(func() { serve(h, "site.onbex.co", fmt.Sprintf("/hold%d.html", i), "198.51.100.1") })
	}
	g.Eventually(func() int { h.admission.mu.Lock(); defer h.admission.mu.Unlock(); return h.admission.waiting }, 5*time.Second, time.Millisecond).Should(Equal(2 * defaultMaxClientFetches))

	start := time.Now()
	g.Expect(serve(h, "victim.onbex.co", "/index.html", "203.0.113.9").Code).To(Equal(http.StatusOK))
	g.Expect(time.Since(start)).To(BeNumerically("<", 500*time.Millisecond))
	g.Expect(origin.entered.Load()).To(Equal(int64(defaultMaxClientFetches)), "the waiters hold no origin fetch")

	close(origin.release)
	all.Wait()
}

func TestAWaiterLeavesWhenItsRequestIsCancelled(t *testing.T) {
	g := NewWithT(t)
	origin := &holdOrigin{release: make(chan struct{})}
	h := newMultiSiteHandler(origin)
	h.admission.wait = 5 * time.Second

	var held sync.WaitGroup
	for i := range defaultMaxClientFetches {
		held.Go(func() { serve(h, "site.onbex.co", fmt.Sprintf("/hold%d.html", i), "198.51.100.1") })
	}
	g.Eventually(func() int64 { return origin.entered.Load() }, 5*time.Second, time.Millisecond).Should(Equal(int64(defaultMaxClientFetches)))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan *httptest.ResponseRecorder, 1)
	go func() { done <- serveCtx(ctx, h, "site.onbex.co", "/extra.html", "198.51.100.1") }()
	g.Eventually(func() int { h.admission.mu.Lock(); defer h.admission.mu.Unlock(); return h.admission.waiting }, 5*time.Second, time.Millisecond).Should(Equal(1))

	start := time.Now()
	cancel()
	g.Eventually(done, time.Second).Should(Receive())
	g.Expect(time.Since(start)).To(BeNumerically("<", time.Second), "it did not sit out its wait")
	g.Expect(h.admission.waiting).To(BeZero())

	close(origin.release)
	held.Wait()
}

// A fetch the gate refuses has taken its client's slot for a moment and gives it
// back; a waiter of the same client may have been refused by that very slot, so
// the release must wake the waiters like any other.
func TestARefusedFetchWakesTheWaitersItBrieflyBlocked(t *testing.T) {
	g := NewWithT(t)
	h := newMultiSiteHandler(&latencyOrigin{})
	// The site's gate slots are all taken, by other clients.
	for range defaultMaxSiteFetches {
		g.Expect(h.gate.acquire("site")).To(BeTrue())
	}
	// Someone is waiting, so a release must close the current wake-up channel.
	h.admission.mu.Lock()
	h.admission.waiting = 1
	changed := h.admission.changed
	h.admission.mu.Unlock()

	release, ok := h.tryAdmit("site", "198.51.100.1")
	g.Expect(ok).To(BeFalse())
	g.Expect(release).To(BeNil())
	g.Expect(changed).To(BeClosed(), "the waiters were woken")
	g.Expect(h.clients.inflight).To(BeEmpty(), "and the client's slot is back")
}
