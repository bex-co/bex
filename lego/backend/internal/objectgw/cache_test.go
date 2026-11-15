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
	"net/http"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

func TestASecondReadOfASmallObjectCostsNoWasabiRequest(t *testing.T) {
	e := newGW(t)
	e.put("o", randomBytes(t, 5000), objectstorage.ObjectHeaders{ContentType: "image/gif"})
	first := e.do(http.MethodGet, "/o")
	if got := e.objectRequests(http.MethodGet); got != 1 {
		t.Fatalf("the first read made %d Wasabi requests, want 1", got)
	}
	for range 5 {
		again := e.do(http.MethodGet, "/o")
		if again.Code != http.StatusOK || again.Body.String() != first.Body.String() || again.Header().Get("Content-Type") != "image/gif" {
			t.Fatalf("a cached read differs: %d %v", again.Code, again.Header())
		}
	}
	if got := e.objectRequests(http.MethodGet); got != 1 {
		t.Errorf("6 reads made %d Wasabi requests, want 1: the cache did not hold", got)
	}
}

// TestTheCacheExpiresSoAnOverwriteReachesVisitors: a body is served for CacheTTL
// at most, then read again from Wasabi.
func TestTheCacheExpiresSoAnOverwriteReachesVisitors(t *testing.T) {
	e := newGW(t)
	e.put("o", []byte("version one"), objectstorage.ObjectHeaders{})
	if got := e.do(http.MethodGet, "/o").Body.String(); got != "version one" {
		t.Fatal(got)
	}
	e.put("o", []byte("version two"), objectstorage.ObjectHeaders{})
	e.clock.advance(CacheTTL - time.Second)
	if got := e.do(http.MethodGet, "/o").Body.String(); got != "version one" {
		t.Errorf("inside the window = %q, want the cached version one", got)
	}
	e.clock.advance(2 * time.Second)
	if got := e.do(http.MethodGet, "/o").Body.String(); got != "version two" {
		t.Errorf("after the window = %q, want version two", got)
	}
}

func TestAnObjectOverTheCacheBoundIsStreamedEveryTime(t *testing.T) {
	e := newGW(t)
	e.put("big", randomBytes(t, CacheMaxObjectBytes+1), objectstorage.ObjectHeaders{})
	e.do(http.MethodGet, "/big")
	e.do(http.MethodGet, "/big")
	if got := e.objectRequests(http.MethodGet); got != 2 {
		t.Errorf("two reads of an object over the bound made %d Wasabi requests, want 2", got)
	}
}

// TestARangedOrConditionalFirstReadIsNotCached: only a whole object is kept, so
// a partial answer is never served as a whole one.
func TestARangedOrConditionalFirstReadIsNotCached(t *testing.T) {
	e := newGW(t)
	body := randomBytes(t, 4000)
	e.put("o", body, objectstorage.ObjectHeaders{})
	e.do(http.MethodGet, "/o", "Range", "bytes=0-9")
	e.do(http.MethodGet, "/o", "If-None-Match", `"nope"`)
	full := e.do(http.MethodGet, "/o")
	if full.Code != http.StatusOK || full.Body.Len() != len(body) {
		t.Fatalf("a full read after a ranged one = %d with %d bytes, want 200 with %d", full.Code, full.Body.Len(), len(body))
	}
}

func TestTheCacheIsBoundedAndEvictsTheLeastRecentlyUsed(t *testing.T) {
	c := newObjectCache()
	c.maxBytes, c.maxObject, c.maxEntries = 100, 40, 3
	clock := newClock()
	c.now = clock.now
	obj := func(key string, n int) *cachedObject { return &cachedObject{key: key, body: make([]byte, n)} }

	c.put(obj("a", 40))
	c.put(obj("b", 40))
	if _, ok := c.get("a"); !ok { // a is now the most recently used
		t.Fatal("a missing")
	}
	c.put(obj("c", 40)) // 120 > 100: the least recently used (b) goes
	if _, ok := c.get("b"); ok {
		t.Error("b survived; it was the least recently used")
	}
	if _, ok := c.get("a"); !ok {
		t.Error("a was evicted though it was just used")
	}
	if c.bytes > c.maxBytes {
		t.Errorf("holding %d bytes, over the bound %d", c.bytes, c.maxBytes)
	}
	c.put(obj("huge", 41)) // over the per-object bound
	if _, ok := c.get("huge"); ok {
		t.Error("an object over the per-object bound was kept")
	}
	// The entry count is bounded too: a crowd of empty objects cannot cost more than its headers.
	for i := range 10 {
		c.put(obj("e"+strconv.Itoa(i), 0))
	}
	if len(c.items) > c.maxEntries {
		t.Errorf("%d entries, bound %d", len(c.items), c.maxEntries)
	}
	// Replacing a key does not double count its bytes.
	c = newObjectCache()
	c.now = clock.now
	c.put(obj("k", 10))
	c.put(obj("k", 25))
	if c.bytes != 25 || len(c.items) != 1 {
		t.Errorf("after a replace: %d bytes, %d entries, want 25 and 1", c.bytes, len(c.items))
	}
	// An expired entry is gone and its bytes are given back.
	clock.advance(CacheTTL + time.Second)
	if _, ok := c.get("k"); ok || c.bytes != 0 {
		t.Errorf("an expired entry was served or kept: %d bytes", c.bytes)
	}
}

// TestConcurrentReadsOfTheSameObjectAreSafe is for the race detector: many
// readers fill and hit the cache at once and all get the object.
func TestConcurrentReadsOfTheSameObjectAreSafe(t *testing.T) {
	e := newGW(t)
	body := randomBytes(t, 20_000)
	e.put("o", body, objectstorage.ObjectHeaders{})
	var wg sync.WaitGroup
	errs := make(chan string, 64)
	for range 32 {
		wg.Go(func() {
			rec := e.do(http.MethodGet, "/o")
			if rec.Code != http.StatusOK || rec.Body.Len() != len(body) {
				errs <- strconv.Itoa(rec.Code)
			}
		})
	}
	wg.Wait()
	close(errs)
	for code := range errs {
		t.Errorf("a concurrent read answered %s", code)
	}
}
