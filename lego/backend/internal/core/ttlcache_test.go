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

package core

// TTLCache is the shared positive-result cache behind both the auth gate and
// the OpenFGA checker, so its three load-bearing properties get direct tests
// here: an expired entry is never served (and is dropped on lookup, not left
// to linger), eviction is exact (the three Delete variants touch only what they
// match), and the map stays bounded at CacheMax while never confusing one
// key's value for another's.

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

func ttlLen[V any](c *TTLCache[V]) int {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.m)
}

func ttlHasKey[V any](c *TTLCache[V], key string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.m[key]
	return ok
}

func TestTTLCacheServesOnlyLiveEntries(t *testing.T) {
	c := NewTTLCache[int]()
	future := time.Now().Add(time.Hour)
	c.Put("live", 1, future)
	if v, ok := c.Get("live"); !ok || v != 1 {
		t.Fatalf("live entry = %v/%v, want 1/true", v, ok)
	}

	c.Put("dead", 2, time.Now().Add(-time.Nanosecond))
	if v, ok := c.Get("dead"); ok {
		t.Fatalf("expired entry served as %v", v)
	}
	if ttlHasKey(c, "dead") {
		t.Error("an expired entry must be dropped on Get, not left to linger until CacheMax")
	}
	if _, ok := c.Get("missing"); ok {
		t.Error("absent key reported present")
	}

	// A later Put revives the same key with a fresh expiry.
	c.Put("dead", 3, future)
	if v, ok := c.Get("dead"); !ok || v != 3 {
		t.Fatalf("revived entry = %v/%v, want 3/true", v, ok)
	}
}

func TestTTLCacheExpiresOverTime(t *testing.T) {
	c := NewTTLCache[int]()
	c.Put("k", 7, time.Now().Add(20*time.Millisecond))
	if _, ok := c.Get("k"); !ok {
		t.Fatal("entry must be live inside its TTL")
	}
	time.Sleep(120 * time.Millisecond)
	if _, ok := c.Get("k"); ok {
		t.Fatal("entry must expire once its TTL passes")
	}
}

func TestTTLCacheDeleteIsExact(t *testing.T) {
	c := NewTTLCache[int]()
	future := time.Now().Add(time.Hour)
	c.Put("a", 1, future)
	c.Put("ab", 2, future)
	c.Delete("a")
	if _, ok := c.Get("a"); ok {
		t.Error("Delete must evict its key")
	}
	if v, ok := c.Get("ab"); !ok || v != 2 {
		t.Error("Delete must not touch a key it merely prefixes")
	}
	c.Delete("absent") // no-op, must not panic
}

func TestTTLCacheDeleteIfMatchesValues(t *testing.T) {
	c := NewTTLCache[int]()
	future := time.Now().Add(time.Hour)
	c.Put("k1", 1, future)
	c.Put("k2", 2, future)
	c.Put("k3", 1, future)
	c.DeleteIf(func(v int) bool { return v == 1 })
	for _, key := range []string{"k1", "k3"} {
		if _, ok := c.Get(key); ok {
			t.Errorf("DeleteIf left %q", key)
		}
	}
	if v, ok := c.Get("k2"); !ok || v != 2 {
		t.Error("DeleteIf must not touch non-matching values")
	}
}

func TestTTLCacheDeleteKeyIfMatchesKeys(t *testing.T) {
	c := NewTTLCache[int]()
	future := time.Now().Add(time.Hour)
	c.Put("subject\x00a", 1, future)
	c.Put("subject\x00b", 2, future)
	c.Put("subjectx\x00c", 3, future) // shares the "subject" string prefix
	c.DeleteKeyIf(func(key string) bool { return strings.HasPrefix(key, "subject\x00") })
	for _, key := range []string{"subject\x00a", "subject\x00b"} {
		if _, ok := c.Get(key); ok {
			t.Errorf("DeleteKeyIf left %q", key)
		}
	}
	if v, ok := c.Get("subjectx\x00c"); !ok || v != 3 {
		t.Error("DeleteKeyIf must match the separator, not just the string prefix")
	}
}

func TestTTLCacheStaysBoundedAtCacheMax(t *testing.T) {
	future := time.Now().Add(time.Hour)

	t.Run("expired entries are swept before anything live is dropped", func(t *testing.T) {
		c := NewTTLCache[int]()
		past := time.Now().Add(-time.Hour)
		for i := range CacheMax {
			c.Put("dead-"+strconv.Itoa(i), i, past)
		}
		if got := ttlLen(c); got != CacheMax {
			t.Fatalf("cache holds %d expired entries, want %d", got, CacheMax)
		}
		c.Put("live", 42, future)
		if v, ok := c.Get("live"); !ok || v != 42 {
			t.Fatalf("live entry = %v/%v after the sweep", v, ok)
		}
		if got := ttlLen(c); got != 1 {
			t.Fatalf("cache holds %d entries, want 1: the sweep must drop the expired, not the new entry", got)
		}
	})

	t.Run("all-live cache resets instead of growing", func(t *testing.T) {
		c := NewTTLCache[int]()
		for i := range CacheMax {
			c.Put("k-"+strconv.Itoa(i), i, future)
		}
		c.Put("overflow", -1, future)
		if got := ttlLen(c); got > CacheMax {
			t.Fatalf("cache holds %d entries, want <= %d", got, CacheMax)
		}
		if v, ok := c.Get("overflow"); !ok || v != -1 {
			t.Fatalf("overflow entry = %v/%v; the Put that filled past the cap must still be stored", v, ok)
		}
		// Whatever survives still maps a key to its own value — a reset must
		// never let one entry answer for another.
		c.mu.Lock()
		for key, entry := range c.m {
			if entry.v >= 0 && key != "k-"+strconv.Itoa(entry.v) {
				t.Errorf("key %q carries %d", key, entry.v)
			}
		}
		c.mu.Unlock()
	})
}

func TestTTLCacheConcurrentAccess(t *testing.T) {
	c := NewTTLCache[int]()
	future := time.Now().Add(time.Hour)
	var wg sync.WaitGroup
	for g := range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range 500 {
				key := fmt.Sprintf("%d-%d", g, j)
				c.Put(key, j, future)
				if v, ok := c.Get(key); ok && v != j {
					t.Errorf("key %q = %d, want %d", key, v, j)
				}
				if j%7 == 0 {
					c.Delete(key)
				}
				if j%23 == 0 {
					c.DeleteIf(func(v int) bool { return v == j+1 })
					c.DeleteKeyIf(func(k string) bool { return strings.HasSuffix(k, "-0") && g == 0 })
				}
			}
		}()
	}
	wg.Wait()

	c.DeleteKeyIf(func(string) bool { return true })
	if got := ttlLen(c); got != 0 {
		t.Fatalf("cache holds %d entries after DeleteKeyIf(all), want 0", got)
	}
	if _, ok := c.Get("0-0"); ok {
		t.Error("DeleteKeyIf(all) left an entry readable")
	}
}
