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

package objectgw

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/time/rate"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage/publichost"
)

func testBucketHost() (Bucket, string) {
	b := Bucket{ID: ids.New(ids.ObjectBucket), WorkspaceID: ids.New(ids.Workspace), Region: objectstorage.Frankfurt}
	host, _ := publichost.Host(b.ID, testBaseDomain)
	return b, host
}

func TestHTTPLookupSpeaksToBexAPIWithTheBearer(t *testing.T) {
	bucket, host := testBucketHost()
	var gotHost, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotHost, gotAuth = r.URL.Query().Get("host"), r.Header.Get("Authorization")
		if r.URL.Path != LookupPath || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		if gotHost != host {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}
		_ = json.NewEncoder(w).Encode(LookupAnswer{
			BucketID: bucket.ID, WorkspaceID: bucket.WorkspaceID, Region: "frankfurt",
			CORS: []objectstorage.CORSRule{corsRule([]string{"*"}, []string{"GET"}, nil, nil, nil)},
		})
	}))
	defer srv.Close()
	l := &HTTPLookup{APIClient{URL: srv.URL, Token: "secret-bearer"}}

	got, err := l.Lookup(t.Context(), host)
	if err != nil || got.ID != bucket.ID || got.WorkspaceID != bucket.WorkspaceID || got.Region != objectstorage.Frankfurt || len(got.CORS) != 1 {
		t.Fatalf("Lookup = %+v, %v", got, err)
	}
	if gotAuth != "Bearer secret-bearer" || gotHost != host {
		t.Errorf("sent host %q with %q", gotHost, gotAuth)
	}
	if _, err := l.Lookup(t.Context(), "pub-"+strings.Repeat("0", 20)+"."+testBaseDomain); !errors.Is(err, ErrNotFound) {
		t.Errorf("a 404 = %v, want ErrNotFound", err)
	}
}

// TestHTTPLookupDistrustsWhatItCannotUse: an answer whose ids are not of their
// kinds, or whose region the gateway does not serve, is an error, never a bucket
// the gateway would build a Wasabi key from.
func TestHTTPLookupDistrustsWhatItCannotUse(t *testing.T) {
	good, host := testBucketHost()
	answers := map[string]LookupAnswer{
		"a bucket id that is not one":    {BucketID: ids.New(ids.Workspace), WorkspaceID: good.WorkspaceID, Region: "oregon"},
		"a workspace id that is not one": {BucketID: good.ID, WorkspaceID: ids.New(ids.Service), Region: "oregon"},
		"traversal in an id":             {BucketID: good.ID, WorkspaceID: "tea-../../x", Region: "oregon"},
		"an unserved region":             {BucketID: good.ID, WorkspaceID: good.WorkspaceID, Region: "ohio"},
		"no region":                      {BucketID: good.ID, WorkspaceID: good.WorkspaceID},
	}
	for name, answer := range answers {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _ = json.NewEncoder(w).Encode(answer) }))
		_, err := (&HTTPLookup{APIClient{URL: srv.URL, Token: "t"}}).Lookup(t.Context(), host)
		srv.Close()
		if err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v, want an error that is not a 404", name, err)
		}
	}
	for name, handler := range map[string]http.HandlerFunc{
		"a 500":    func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "boom", 500) },
		"a 401":    func(w http.ResponseWriter, _ *http.Request) { http.Error(w, "no", 401) },
		"not JSON": func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("<html>")) },
		"too large": func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte(strings.Repeat("a", maxLookupAnswerBytes+10)))
		},
	} {
		srv := httptest.NewServer(handler)
		_, err := (&HTTPLookup{APIClient{URL: srv.URL, Token: "t"}}).Lookup(t.Context(), host)
		srv.Close()
		if err == nil || errors.Is(err, ErrNotFound) {
			t.Errorf("%s: %v, want an error that is not a 404", name, err)
		}
	}
	// bex-api away is an error, not "not found": a browser must not be told the bucket is gone.
	if _, err := (&HTTPLookup{APIClient{URL: "http://127.0.0.1:1", Token: "t"}}).Lookup(t.Context(), host); err == nil || errors.Is(err, ErrNotFound) {
		t.Errorf("an unreachable bex-api = %v", err)
	}
}

func cachedWith(up Resolver, clock *fakeClock) *CachedResolver {
	c := NewCachedResolver(up)
	c.now = clock.now
	c.guesses, c.renewals = rate.NewLimiter(1000, 1000), rate.NewLimiter(1000, 1000)
	return c
}

type countingResolver struct {
	calls atomic.Int64
	fn    func(host string) (Bucket, error)
	gate  chan struct{}
}

func (c *countingResolver) Lookup(_ context.Context, host string) (Bucket, error) {
	c.calls.Add(1)
	if c.gate != nil {
		<-c.gate
	}
	return c.fn(host)
}

// TestLookupsAreCachedForThirtySecondsMissesToo: the ADR's window, and the reason
// disabling dev-url takes at most this long to reach a visitor.
func TestLookupsAreCachedForThirtySecondsMissesToo(t *testing.T) {
	bucket, host := testBucketHost()
	public := true
	up := &countingResolver{fn: func(string) (Bucket, error) {
		if public {
			return bucket, nil
		}
		return Bucket{}, ErrNotFound
	}}
	clock := newClock()
	c := cachedWith(up, clock)
	if LookupTTL != 30*time.Second {
		t.Fatalf("LookupTTL = %s, ADR112 D4 says 30 s", LookupTTL)
	}

	if _, err := c.Lookup(t.Context(), host); err != nil {
		t.Fatal(err)
	}
	for _, variant := range []string{host, strings.ToUpper(host), host + ":8443", host + "."} {
		if _, err := c.Lookup(t.Context(), variant); err != nil {
			t.Fatalf("%q: %v", variant, err)
		}
	}
	if up.calls.Load() != 1 {
		t.Errorf("%d upstream calls for one host in four spellings, want 1", up.calls.Load())
	}

	// Disabled upstream: still served inside the window, gone after it.
	public = false
	clock.advance(LookupTTL - time.Second)
	if _, err := c.Lookup(t.Context(), host); err != nil {
		t.Errorf("inside the window: %v, want the cached answer", err)
	}
	clock.advance(2 * time.Second)
	if _, err := c.Lookup(t.Context(), host); !errors.Is(err, ErrNotFound) {
		t.Errorf("after the window: %v, want ErrNotFound", err)
	}
	// And the miss is cached too: a guess costs one call per window.
	calls := up.calls.Load()
	for range 5 {
		if _, err := c.Lookup(t.Context(), host); !errors.Is(err, ErrNotFound) {
			t.Fatal(err)
		}
	}
	if up.calls.Load() != calls {
		t.Errorf("a cached miss asked upstream %d more times", up.calls.Load()-calls)
	}
	// Enabled again: visible after the miss expires.
	public = true
	clock.advance(LookupTTL + time.Second)
	if _, err := c.Lookup(t.Context(), host); err != nil {
		t.Errorf("after enabling: %v", err)
	}
}

func TestAFailedLookupIsNotCached(t *testing.T) {
	bucket, host := testBucketHost()
	fail := true
	up := &countingResolver{fn: func(string) (Bucket, error) {
		if fail {
			return Bucket{}, errors.New("bex-api is unreachable")
		}
		return bucket, nil
	}}
	c := cachedWith(up, newClock())
	if _, err := c.Lookup(t.Context(), host); err == nil {
		t.Fatal("want the error")
	}
	fail = false
	if _, err := c.Lookup(t.Context(), host); err != nil {
		t.Errorf("after bex-api came back: %v; an outage must not be remembered as an answer", err)
	}
}

func TestConcurrentLookupsOfOneHostShareACall(t *testing.T) {
	bucket, host := testBucketHost()
	up := &countingResolver{fn: func(string) (Bucket, error) { return bucket, nil }, gate: make(chan struct{})}
	c := cachedWith(up, newClock())
	var wg sync.WaitGroup
	for range 20 {
		wg.Go(func() {
			if _, err := c.Lookup(t.Context(), host); err != nil {
				t.Error(err)
			}
		})
	}
	// Let every caller reach the shared call, then release it.
	for up.calls.Load() == 0 {
		time.Sleep(time.Millisecond)
	}
	time.Sleep(20 * time.Millisecond)
	close(up.gate)
	wg.Wait()
	if up.calls.Load() != 1 {
		t.Errorf("%d upstream calls for 20 concurrent lookups of one host, want 1", up.calls.Load())
	}
}

// TestGuessingHostsCostsAtMostTheLookupRate: an attacker has unlimited guesses at
// a bucket host and each costs one call to bex-api, so uncached lookups are
// rate-limited, and the hosts already cached keep working while it is.
func TestGuessingHostsCostsAtMostTheLookupRate(t *testing.T) {
	bucket, host := testBucketHost()
	up := &countingResolver{fn: func(h string) (Bucket, error) {
		if publichost.NormalizeHost(h) == host {
			return bucket, nil
		}
		return Bucket{}, ErrNotFound
	}}
	clock := newClock()
	c := cachedWith(up, clock)
	c.guesses = rate.NewLimiter(1, 5)
	if _, err := c.Lookup(t.Context(), host); err != nil { // one of the five
		t.Fatal(err)
	}
	busy := 0
	for i := range 50 {
		guess := fmt.Sprintf("pub-%020d.%s", i, testBaseDomain)
		if _, err := c.Lookup(t.Context(), guess); errors.Is(err, ErrBusy) {
			busy++
		}
	}
	if busy < 40 || up.calls.Load() > 12 {
		t.Errorf("%d of 50 guesses refused, %d upstream calls; the rate is 1 a second with a burst of 5", busy, up.calls.Load())
	}
	// The cached bucket is unaffected by the flood, and so is its renewal when its
	// entry expires in the middle of one: that refresh is not a guess.
	if _, err := c.Lookup(t.Context(), host); err != nil {
		t.Errorf("a cached host was refused during a flood: %v", err)
	}
	clock.advance(LookupTTL + time.Second)
	c.guesses = rate.NewLimiter(0, 0) // every guess is refused from here on
	if _, err := c.Lookup(t.Context(), host); err != nil {
		t.Errorf("a bucket that was served could not be renewed during a flood: %v", err)
	}
	if _, err := c.Lookup(t.Context(), fmt.Sprintf("pub-%020d.%s", 99, testBaseDomain)); !errors.Is(err, ErrBusy) {
		t.Errorf("a new guess during the flood = %v, want ErrBusy", err)
	}
}

func TestTheLookupCacheIsBounded(t *testing.T) {
	up := &countingResolver{fn: func(string) (Bucket, error) { return Bucket{}, ErrNotFound }}
	c := cachedWith(up, newClock())
	c.MaxEntries = 10
	for i := range 100 {
		_, _ = c.Lookup(t.Context(), fmt.Sprintf("pub-%020d.%s", i, testBaseDomain))
	}
	if len(c.entries) > c.MaxEntries {
		t.Errorf("%d answers held, bound %d", len(c.entries), c.MaxEntries)
	}
}

func TestOneCallersCancellationDoesNotFailTheOthers(t *testing.T) {
	bucket, host := testBucketHost()
	up := &countingResolver{fn: func(string) (Bucket, error) { return bucket, nil }}
	c := cachedWith(up, newClock())
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	// The shared call is detached from its first caller's context.
	if _, err := c.Lookup(ctx, host); err != nil {
		t.Errorf("a lookup under a cancelled context failed: %v", err)
	}
}
