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
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

// w21/m394 t009 (R25-05): the fetch gate bounds origin fetches per site (4)
// and per replica (16), so one client that floods unique missing paths on four
// sites held every slot and shed other tenants' cold objects as 503.

// multiSiteResolver serves a distinct static site for each "<name>.onbex.co".
type multiSiteResolver struct{}

func (multiSiteResolver) Resolve(host string) (Site, bool) {
	name, ok := strings.CutSuffix(host, ".onbex.co")
	if !ok {
		return Site{}, false
	}
	return Site{AppID: name, Revision: "rev-1", Incarnation: name + "-uid"}, true
}

// holdOrigin blocks every Get whose key path starts with "hold" until release
// is closed and answers every other key at once, so a test can pin an
// attacker's fetches while a victim's cold miss goes through (or is shed).
type holdOrigin struct {
	release chan struct{}
	entered atomic.Int64 // Gets blocked (or blockable) on a "hold" key
}

func (o *holdOrigin) Get(_ context.Context, key string) (Object, error) {
	if strings.Contains(key, "/hold") {
		o.entered.Add(1)
		<-o.release
	}
	return Object{Body: []byte("ok"), ContentType: "text/html"}, nil
}

func clientStatus(h *Handler, host, path, client string) int {
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "http://"+host+path, nil)
	req.Host = host
	req.Header.Set("True-Client-IP", client)
	h.ServeHTTP(rec, req)
	return rec.Code
}

// floodHeld has one client request four distinct held paths on each given
// site, and returns once every request has either reached the origin or been
// shed, together with how many were shed and a function that releases and
// waits for the rest.
func floodHeld(t *testing.T, h *Handler, origin *holdOrigin, client string, sites []string) (shed int, finish func()) {
	t.Helper()
	total := len(sites) * 4
	var done sync.WaitGroup
	var shedCount atomic.Int64
	for _, site := range sites {
		for i := range 4 {
			done.Go(func() {
				if clientStatus(h, site+".onbex.co", fmt.Sprintf("/hold%d.html", i), client) == http.StatusServiceUnavailable {
					shedCount.Add(1)
				}
			})
		}
	}
	NewWithT(t).Eventually(func() bool { return origin.entered.Load()+shedCount.Load() >= int64(total) }, 5*time.Second, 2*time.Millisecond).Should(BeTrue(), "flood settled")
	return int(shedCount.Load()), func() {
		close(origin.release)
		done.Wait()
	}
}

func newMultiSiteHandler(origin Origin) *Handler {
	return New(multiSiteResolver{}, origin, 1<<20)
}

// TestOneClientCannotStarveOtherTenantsSites: a client holding 4 slots on each
// of 4 sites took the replica's 16 slots, so a different client's cold miss on
// a fifth site answered 503.
func TestOneClientCannotStarveOtherTenantsSites(t *testing.T) {
	origin := &holdOrigin{release: make(chan struct{})}
	h := newMultiSiteHandler(origin)

	shed, finish := floodHeld(t, h, origin, "198.51.100.1", []string{"atk1", "atk2", "atk3", "atk4"})
	defer finish()

	if got := origin.entered.Load(); got > defaultMaxClientFetches {
		t.Fatalf("one client held %d origin fetches, cap is %d", got, defaultMaxClientFetches)
	}
	if shed == 0 {
		t.Fatal("the flood was never shed")
	}
	if status := clientStatus(h, "victim.onbex.co", "/index.html", "203.0.113.9"); status != http.StatusOK {
		t.Fatalf("another client's cold miss on a fifth site => %d, want 200", status)
	}
}

// TestOneClientCannotPinAWholeSite: the same client flooding one site held all
// of its 4 slots, so the site's own visitors' cold objects answered 503.
func TestOneClientCannotPinAWholeSite(t *testing.T) {
	origin := &holdOrigin{release: make(chan struct{})}
	h := newMultiSiteHandler(origin)

	_, finish := floodHeld(t, h, origin, "198.51.100.1", []string{"atk1"})
	defer finish()

	if status := clientStatus(h, "atk1.onbex.co", "/other.html", "203.0.113.9"); status != http.StatusOK {
		t.Fatalf("another client's cold miss on the flooded site => %d, want 200", status)
	}
}

// TestClientKeepsEnoughSlotsForAColdPage: the cap is per client, so a visitor
// still has several concurrent origin fetches for a cold page.
func TestClientKeepsEnoughSlotsForAColdPage(t *testing.T) {
	origin := &holdOrigin{release: make(chan struct{})}
	h := newMultiSiteHandler(origin)
	var done sync.WaitGroup
	var ok atomic.Int64
	for i := range defaultMaxClientFetches {
		done.Go(func() {
			if clientStatus(h, "site1.onbex.co", fmt.Sprintf("/hold%d.html", i), "198.51.100.1") == http.StatusOK {
				ok.Add(1)
			}
		})
	}
	NewWithT(t).Eventually(func() bool { return origin.entered.Load() >= int64(defaultMaxClientFetches) }, 5*time.Second, 2*time.Millisecond).Should(BeTrue(), "every cold fetch reaches the origin")
	close(origin.release)
	done.Wait()
	if int(ok.Load()) != defaultMaxClientFetches {
		t.Fatalf("%d of %d concurrent cold fetches answered 200", ok.Load(), defaultMaxClientFetches)
	}
}

// TestRequestWithoutAClientAddressIsNotClientLimited: a request with neither
// True-Client-IP nor a parseable peer has no client to charge.
func TestRequestWithoutAClientAddressIsNotClientLimited(t *testing.T) {
	origin := &holdOrigin{release: make(chan struct{})}
	h := newMultiSiteHandler(origin)
	n := defaultMaxClientFetches + 2
	var done sync.WaitGroup
	for i := range n {
		done.Go(func() {
			rec := httptest.NewRecorder()
			host := fmt.Sprintf("anon%d.onbex.co", i)
			req := httptest.NewRequest(http.MethodGet, "http://"+host+"/hold.html", nil)
			req.Host = host
			req.RemoteAddr = "pipe"
			h.ServeHTTP(rec, req)
		})
	}
	NewWithT(t).Eventually(func() bool { return origin.entered.Load() >= int64(n) }, 5*time.Second, 2*time.Millisecond).Should(BeTrue(), "every request reaches the origin")
	close(origin.release)
	done.Wait()
}

// TestClientSlotsAreReturned: a finished fetch frees its client slot.
func TestClientSlotsAreReturned(t *testing.T) {
	h := newMultiSiteHandler(&holdOrigin{release: make(chan struct{})})
	for i := range 3 * defaultMaxClientFetches {
		if status := clientStatus(h, "site1.onbex.co", fmt.Sprintf("/page%d.html", i), "198.51.100.1"); status != http.StatusOK {
			t.Fatalf("sequential request %d => %d, want 200", i, status)
		}
	}
	if n := len(h.clients.inflight); n != 0 {
		t.Fatalf("%d client entries left after every fetch finished", n)
	}
}

func TestClientKey(t *testing.T) {
	req := func(header, remote string) *http.Request {
		r := httptest.NewRequest(http.MethodGet, "http://site.onbex.co/", nil)
		r.RemoteAddr = remote
		if header != "" {
			r.Header.Set("True-Client-IP", header)
		}
		return r
	}
	cases := []struct {
		name         string
		header, peer string
		want         string
	}{
		{"edge header wins over the peer", "198.51.100.7", "10.0.0.9:4000", "198.51.100.7"},
		{"header is trimmed", " 198.51.100.7 ", "10.0.0.9:4000", "198.51.100.7"},
		{"no header uses the peer host", "", "10.0.0.9:4000", "10.0.0.9"},
		{"unparseable header falls back to the peer", "not-an-ip", "10.0.0.9:4000", "10.0.0.9"},
		{"IPv4-mapped IPv6 is the IPv4 address", "::ffff:198.51.100.7", "10.0.0.9:4000", "198.51.100.7"},
		{"IPv6 collapses to its /64", "2001:db8:1:2:aaaa:bbbb:cccc:dddd", "10.0.0.9:4000", "2001:db8:1:2::"},
		{"same /64, other interface id", "2001:db8:1:2::1", "10.0.0.9:4000", "2001:db8:1:2::"},
		{"zone is dropped", "fe80::1%eth0", "10.0.0.9:4000", "fe80::"},
		{"IPv6 peer without a header", "", "[2001:db8:9:9::5]:4000", "2001:db8:9:9::"},
		{"nothing usable", "", "pipe", ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clientKey(req(tc.header, tc.peer)); got != tc.want {
				t.Fatalf("clientKey = %q, want %q", got, tc.want)
			}
		})
	}
	if clientKey(req("2001:db8:1:2::1", "x:1")) == clientKey(req("2001:db8:1:3::1", "x:1")) {
		t.Fatal("two different /64s share a client key")
	}
}

// fillNegatives stores n unique confirmed misses for site, as n distinct 404s do.
func fillNegatives(c *cache, site string, n int) {
	for i := range n {
		c.put(site, fmt.Sprintf("%s/rev-1/missing-%d.html", site, i), Object{negative: true})
	}
}

// TestNegativeEntriesCannotEvictRealObjects: every unique missing path stores
// a negative entry, and eviction was random across a site's whole budget, so a
// flood of unique 404s pushed the site's real cached objects out.
func TestNegativeEntriesCannotEvictRealObjects(t *testing.T) {
	c := newCache(128 << 20)
	body := make([]byte, 1<<20)
	for i := range 8 {
		c.put("site-a", fmt.Sprintf("a/real%d", i), Object{Body: body, ContentType: "text/html"})
	}
	fillNegatives(c, "site-a", 150_000)
	for i := range 8 {
		if _, ok := c.get(fmt.Sprintf("a/real%d", i)); !ok {
			t.Fatalf("real object %d was evicted by unique 404s", i)
		}
	}
	if c.siteNegUsed["site-a"] > c.negativeBudget {
		t.Fatalf("site A negative use = %d, exceeds budget %d", c.siteNegUsed["site-a"], c.negativeBudget)
	}
	if c.siteNegUsed["site-a"] == 0 {
		t.Fatal("negative entries are not cached at all; the point is to keep them cheap, not drop them")
	}
}

// TestNegativeEntriesFromManySitesCannotEvictRealObjectsGlobally: under the
// shared byte budget a negative entry may only displace another negative one.
func TestNegativeEntriesFromManySitesCannotEvictRealObjectsGlobally(t *testing.T) {
	c := newCache(8 << 20)
	body := make([]byte, 1<<20)
	for i := range 4 {
		c.put("site-real", fmt.Sprintf("real/%d", i), Object{Body: body, ContentType: "text/html"})
	}
	for s := range 40 {
		fillNegatives(c, fmt.Sprintf("site-%d", s), 5000)
	}
	for i := range 4 {
		if _, ok := c.get(fmt.Sprintf("real/%d", i)); !ok {
			t.Fatalf("real object %d was evicted by other sites' unique 404s", i)
		}
	}
	if c.used > c.budget {
		t.Fatalf("cache use %d exceeds the budget %d", c.used, c.budget)
	}
}

// A real object may still displace negative entries to fit.
func TestRealObjectDisplacesNegativeEntries(t *testing.T) {
	c := newCache(4 << 20)
	fillNegatives(c, "site-a", 20_000)
	c.put("site-a", "a/big", Object{Body: make([]byte, 3<<20), ContentType: "text/html"})
	if _, ok := c.get("a/big"); !ok {
		t.Fatal("a real object could not displace negative entries")
	}
}
