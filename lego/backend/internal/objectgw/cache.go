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
	"container/list"
	"net/http"
	"sync"
	"time"
)

const (
	// CacheMaxBytes bounds the object bodies one replica holds in memory (ADR112
	// D4). Hits spare Wasabi the egress that its fair-use rule counts.
	CacheMaxBytes = 64 << 20
	// CacheMaxObjectBytes is the largest object kept; a bigger one is streamed.
	CacheMaxObjectBytes = 1 << 20
	// CacheTTL is how long a body is served without asking Wasabi again, so an
	// overwritten object reaches visitors within this window.
	CacheTTL = time.Minute
	// cacheMaxEntries bounds the number of objects, so a crowd of tiny ones
	// cannot cost more than their bytes: each entry also holds a header map.
	cacheMaxEntries = 8192
	// MaxBufferedFetches bounds the cache fills in flight at once, each of which
	// holds up to CacheMaxObjectBytes while it reads. Past it an object is
	// streamed uncached.
	MaxBufferedFetches = 32
)

// cachedObject is a whole object and what it was stored with.
type cachedObject struct {
	key     string
	body    []byte
	header  http.Header // the entity headers Wasabi answered with
	modTime time.Time
	expires time.Time
}

// objectCache is a size-bounded LRU of whole small objects, keyed by bucket id
// and object key. A hit costs no Wasabi request: the gateway evaluates Range and
// the conditional headers itself (http.ServeContent).
type objectCache struct {
	maxBytes   int64
	maxObject  int64
	maxEntries int
	ttl        time.Duration
	now        func() time.Time

	mu    sync.Mutex
	bytes int64
	lru   *list.List // front is the most recently used
	items map[string]*list.Element
}

func newObjectCache() *objectCache {
	return &objectCache{
		maxBytes: CacheMaxBytes, maxObject: CacheMaxObjectBytes, maxEntries: cacheMaxEntries, ttl: CacheTTL,
		now: time.Now, lru: list.New(), items: map[string]*list.Element{},
	}
}

func (c *objectCache) get(key string) (*cachedObject, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	el, ok := c.items[key]
	if !ok {
		return nil, false
	}
	obj := el.Value.(*cachedObject)
	if !c.now().Before(obj.expires) {
		c.remove(el)
		return nil, false
	}
	c.lru.MoveToFront(el)
	return obj, true
}

// put stores obj, evicting the least recently used until it fits. An object over
// the per-object bound is not kept.
func (c *objectCache) put(obj *cachedObject) {
	size := int64(len(obj.body))
	if size > c.maxObject || size > c.maxBytes {
		return
	}
	obj.expires = c.now().Add(c.ttl)
	c.mu.Lock()
	defer c.mu.Unlock()
	if el, ok := c.items[obj.key]; ok {
		c.remove(el)
	}
	for c.bytes+size > c.maxBytes || len(c.items) >= c.maxEntries {
		oldest := c.lru.Back()
		if oldest == nil {
			break
		}
		c.remove(oldest)
	}
	c.items[obj.key] = c.lru.PushFront(obj)
	c.bytes += size
}

func (c *objectCache) remove(el *list.Element) {
	obj := c.lru.Remove(el).(*cachedObject)
	delete(c.items, obj.key)
	c.bytes -= int64(len(obj.body))
}
