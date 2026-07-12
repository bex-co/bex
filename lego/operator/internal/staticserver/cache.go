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
	mu      sync.Mutex
	budget  int64
	used    int64
	entries map[string]Object
}

func newCache(budget int64) *cache {
	return &cache{budget: budget, entries: map[string]Object{}}
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

func (c *cache) put(key string, obj Object) {
	if c == nil || c.budget <= 0 {
		return
	}
	size := int64(len(obj.Body))
	if size > c.budget {
		return // too big to cache; served uncached
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.entries[key]; exists {
		return
	}
	for c.used+size > c.budget {
		for k, v := range c.entries { // evict an arbitrary entry
			delete(c.entries, k)
			c.used -= int64(len(v.Body))
			break
		}
	}
	c.entries[key] = obj
	c.used += size
}
