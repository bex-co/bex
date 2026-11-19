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

package sniproxy

import (
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bex-co/bex/lego/types/netutil"
)

// Limiter bounds concurrent proxied connections. A global cap protects the
// process (goroutines, file descriptors, backend dials), an optional
// per-source cap stops one client IP from holding the process, and each route
// admits its RouteCapacity, sized from its datastore's own limit. Every
// acquisition is non-blocking (try-acquire semantics): an overloaded proxy
// sheds the excess connection immediately rather than queueing unbounded work
// (finding 6). A non-positive cap disables that dimension.
type Limiter struct {
	mu        sync.Mutex
	global    int
	maxGlobal int
	perSource map[netip.Addr]int
	maxSource int
	// perRoute counts the connections admitted to each route, one public
	// hostname of one datastore. Its capacity comes with each admission, so it
	// follows the datastore's plan without a restart (w6/m457).
	perRoute map[string]int

	// Handshake slots are held only from the PROXY header until the SNI is
	// resolved, so a connection that never finishes its handshake cannot keep
	// one for the whole handshake deadline. They are counted per network (an
	// IPv4 /24, an IPv6 /64), not per address: a botnet inside one network is
	// one source for this purpose.
	handshakeNetwork    map[netip.Addr]int
	maxHandshakeNetwork int
}

// ScopeResource is the reason a connection is shed when its route already
// holds its capacity.
const ScopeResource = "resource"

// ScopeHandshake is the reason a connection is shed when its network already
// has its share of connections still in their handshake.
const ScopeHandshake = "handshake"

// defaultPreHeaderTimeout bounds the first bytes of a connection: the PROXY
// preamble and the first bytes of the client's handshake. A client that sends
// nothing is closed after this, where the full handshake gets the proxy's
// handshake timeout.
const defaultPreHeaderTimeout = 2 * time.Second

// preHeaderTimeout is atomic so a test can shorten it while connection handlers
// from an earlier test are still starting.
var preHeaderTimeout atomic.Int64

func init() { preHeaderTimeout.Store(int64(defaultPreHeaderTimeout)) }

// PreHeaderTimeout is how long a connection has to send its first bytes.
func PreHeaderTimeout() time.Duration { return time.Duration(preHeaderTimeout.Load()) }

// SetPreHeaderTimeout changes it and returns the value it replaced.
func SetPreHeaderTimeout(d time.Duration) (previous time.Duration) {
	return time.Duration(preHeaderTimeout.Swap(int64(d)))
}

// NewLimiter builds a Limiter with the given global and per-source concurrent
// connection caps. A cap <= 0 disables that dimension.
func NewLimiter(maxGlobal, maxSource int) *Limiter {
	return &Limiter{
		maxGlobal: maxGlobal,
		maxSource: maxSource,
		perSource: make(map[netip.Addr]int),
		perRoute:  make(map[string]int),

		handshakeNetwork: make(map[netip.Addr]int),
	}
}

// SetHandshakeLimit configures how many connections one network may have still
// in their handshake. It must be called before the limiter is shared with
// connection handlers. A non-positive cap disables it.
func (l *Limiter) SetHandshakeLimit(maxPerNetwork int) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.maxHandshakeNetwork = maxPerNetwork
	l.mu.Unlock()
}

// handshakeNetwork is the key a handshake is counted under: the IPv4 /24 or the
// IPv6 /64 of the real client address, so rotating through a network's
// addresses does not dodge the cap and one network cannot hold more than its
// share of the process's slots.
func handshakeNetwork(addr netip.Addr) netip.Addr {
	addr = addr.Unmap()
	if addr.Is4() {
		return netip.PrefixFrom(addr, 24).Masked().Addr()
	}
	return netutil.SourceKey(addr)
}

// AcquireHandshake reserves a handshake slot for addr, the real client address
// resolved from the PROXY header. The caller releases it when the SNI is
// resolved (or the connection ends), so a slot is held only while a client has
// yet to say what it wants. ok is false when addr's network already has its
// share. The release frees the slot exactly once.
func (l *Limiter) AcquireHandshake(addr netip.Addr) (release func(), ok bool) {
	if l == nil || l.maxHandshakeNetwork <= 0 {
		return func() {}, true
	}
	return acquireCounted(l, l.handshakeNetwork, l.maxHandshakeNetwork, handshakeNetwork(addr))
}

// AcquireGlobal reserves one of the global connection slots. ok is false when the
// process is already at capacity — the caller must close the connection without
// dispatching a handler goroutine. The returned release frees the slot exactly
// once; it is a no-op when the global cap is disabled.
func (l *Limiter) AcquireGlobal() (release func(), ok bool) {
	if l == nil || l.maxGlobal <= 0 {
		return func() {}, true
	}
	l.mu.Lock()
	if l.global >= l.maxGlobal {
		l.mu.Unlock()
		return func() {}, false
	}
	l.global++
	l.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			l.global--
			l.mu.Unlock()
		})
	}, true
}

// AcquireSource reserves one per-source slot for addr — the real client address,
// resolved after any PROXY header. ok is false when that source is already at its
// cap. The returned release frees the slot exactly once; it is a no-op when the
// per-source cap is disabled.
func (l *Limiter) AcquireSource(addr netip.Addr) (release func(), ok bool) {
	if l == nil || l.maxSource <= 0 {
		return func() {}, true
	}
	return acquireCounted(l, l.perSource, l.maxSource, netutil.SourceKey(addr))
}

// acquireCounted takes one slot of key in counts when it holds fewer than max,
// and returns the release that frees it exactly once.
func acquireCounted[K comparable](l *Limiter, counts map[K]int, max int, key K) (release func(), ok bool) {
	l.mu.Lock()
	if counts[key] >= max {
		l.mu.Unlock()
		return func() {}, false
	}
	counts[key]++
	l.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			decrementKey(counts, key)
			l.mu.Unlock()
		})
	}, true
}

// RouteCapacity is how many connections at once a route admits for a
// datastore that accepts limit clients: twice the limit. The proxy cannot
// answer inside TLS, so every client it sheds sees a dropped connection; up to
// twice the limit, the client past the limit reaches the server and reads the
// server's own refusal instead (PostgreSQL's "too many clients", Valkey's
// "max number of clients reached"), even when as many clients arrive at once
// as the plan holds. A refused connection leaves as soon as the server has
// answered it, so what a route holds for long is bounded by the server
// (w6/m457, docs/ADR009-postgresql-management.md).
func RouteCapacity(limit int32) int { return 2 * int(limit) }

// AcquireRoute reserves one of route's capacity slots (RouteCapacity of its
// datastore's limit). The route key must include its namespace so identically
// named tenant resources never share a bucket. ok is false when the route is
// full; the release frees the slot exactly once. A capacity <= 0 admits.
func (l *Limiter) AcquireRoute(route string, capacity int) (release func(), ok bool) {
	if l == nil || capacity <= 0 {
		return func() {}, true
	}
	return acquireCounted(l, l.perRoute, capacity, route)
}

func decrementKey[K comparable](counts map[K]int, key K) {
	if counts[key] <= 1 {
		delete(counts, key)
		return
	}
	counts[key]--
}
