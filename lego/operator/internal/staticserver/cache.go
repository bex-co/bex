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

import "sync"

// cache is a byte-budgeted in-memory object cache. Keys are
// "<appID>/<revision>/<path>" — a revision prefix is immutable, so an entry is
// never stale and needs no TTL (a new deploy is a new revision => a new key).
// When an insert would exceed the budget, entries are evicted (in map order,
// which Go randomizes) until it fits, then the object is stored if it fits at
// all. A single object larger than the whole budget is served but not cached.
type cache struct {
	mu            sync.Mutex
	budget        int64
	perSiteBudget int64
	used          int64
	siteUsed      map[string]int64
	entrySites    map[string]string
	entries       map[string]Object
}

// entryOverhead is the fixed per-entry allocation charged on top of the body:
// the key and content-type strings, the two map slots, and the Object struct.
// Without it a zero-byte body would cost nothing to cache, and a tenant could
// publish millions of empty files and warm them into entries that never
// trigger eviction — the byte budget would read as unexhausted while the
// metadata alone grows the shared server toward its memory limit
// (codex-security round 12, finding 2).
const entryOverhead = 256

// entryCharge is the budget cost of caching obj under key: retained body
// capacity plus the entry's own metadata. It is the ONE cost model — put and
// evictOne share it — so an entry is always released for exactly what it was
// charged. Capacity (not length) is charged because the body comes from
// io.ReadAll, whose final slice can carry up to ~2x its length in spare
// capacity that stays allocated while the entry lives.
func entryCharge(key string, obj Object) int64 {
	return int64(cap(obj.Body)) + int64(len(key)) + int64(len(obj.ContentType)) + entryOverhead
}

func newCache(budget int64) *cache {
	perSiteBudget := min(budget, 32<<20)
	return &cache{
		budget:        budget,
		perSiteBudget: perSiteBudget,
		siteUsed:      map[string]int64{},
		entrySites:    map[string]string{},
		entries:       map[string]Object{},
	}
}

func (c *cache) get(key string) (Object, bool) {
	if c == nil || c.budget <= 0 {
		return Object{}, false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	obj, ok := c.entries[key]
	return obj, ok
}

func (c *cache) put(site, key string, obj Object) {
	if c == nil || c.budget <= 0 {
		return
	}
	size := entryCharge(key, obj)
	if size > c.budget || size > c.perSiteBudget {
		return // too big to cache; served uncached
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; exists {
		return
	}
	for c.siteUsed[site]+size > c.perSiteBudget {
		if !c.evictOne(site) {
			return
		}
	}
	for c.used+size > c.budget {
		if !c.evictOne("") {
			return
		}
	}
	c.entries[key] = obj
	c.used += size
	c.siteUsed[site] += size
	c.entrySites[key] = site
}

// evictOne removes one entry, restricted to site when non-empty.
func (c *cache) evictOne(site string) bool {
	for key, obj := range c.entries {
		entrySite := c.entrySites[key]
		if site != "" && entrySite != site {
			continue
		}
		size := entryCharge(key, obj)
		delete(c.entries, key)
		delete(c.entrySites, key)
		c.used -= size
		if c.siteUsed[entrySite] <= size {
			delete(c.siteUsed, entrySite)
		} else {
			c.siteUsed[entrySite] -= size
		}
		return true
	}
	return false
}
