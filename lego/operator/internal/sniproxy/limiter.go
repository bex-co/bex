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
	"context"
	"net"
	"net/netip"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bex-co/bex/lego/types/netutil"
)

// Limiter bounds concurrent proxied connections. A global cap protects the
// process (goroutines, file descriptors, backend dials) and a per-source cap
// stops one client IP from starving the others. Both acquisitions are
// non-blocking (try-acquire semantics): an overloaded proxy sheds the excess
// connection immediately rather than queueing unbounded work (finding 6). A
// non-positive cap disables that dimension.
type Limiter struct {
	mu           sync.Mutex
	global       int
	maxGlobal    int
	perSource    map[netip.Addr]int
	maxSource    int
	perWorkspace map[string]int
	maxWorkspace int
	perResource  map[string]int
	maxResource  int

	// Handshake slots are held only from the PROXY header until the SNI is
	// resolved, so a connection that never finishes its handshake cannot keep
	// one for the whole handshake deadline. They are counted per network (an
	// IPv4 /24, an IPv6 /64), not per address: a botnet inside one network is
	// one source for this purpose.
	handshakeNetwork    map[netip.Addr]int
	maxHandshakeNetwork int
}

// scopeWorkspace is the rejectedScope returned when the per-workspace cap
// sheds a connection.
const scopeWorkspace = "workspace"

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
		maxGlobal:    maxGlobal,
		maxSource:    maxSource,
		perSource:    make(map[netip.Addr]int),
		perWorkspace: make(map[string]int),
		perResource:  make(map[string]int),

		handshakeNetwork: make(map[netip.Addr]int),
	}
}

// SetRouteLimits configures post-SNI fairness caps. It must be called before
// the limiter is shared with connection handlers. A non-positive cap disables
// that dimension.
func (l *Limiter) SetRouteLimits(maxWorkspace, maxResource int) {
	if l == nil {
		return
	}
	l.mu.Lock()
	l.maxWorkspace = maxWorkspace
	l.maxResource = maxResource
	l.mu.Unlock()
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
	return l.acquireCounted(l.handshakeNetwork, l.maxHandshakeNetwork, handshakeNetwork(addr))
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
	return l.acquireCounted(l.perSource, l.maxSource, netutil.SourceKey(addr))
}

// acquireCounted takes one slot of key in counts when it holds fewer than max,
// and returns the release that frees it exactly once.
func (l *Limiter) acquireCounted(counts map[netip.Addr]int, max int, key netip.Addr) (release func(), ok bool) {
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

// AcquireRoute atomically reserves the post-SNI workspace and resource slots.
// The resource key must include its workspace/namespace so identically named
// tenant resources never share a bucket. Admission is all-or-nothing: a failed
// resource check cannot leak a workspace reservation (finding 19).
func (l *Limiter) AcquireRoute(workspace, resource string) (release func(), rejectedScope string, ok bool) {
	if l == nil {
		return func() {}, "", true
	}
	l.mu.Lock()
	if l.maxWorkspace > 0 && l.perWorkspace[workspace] >= l.maxWorkspace {
		l.mu.Unlock()
		return func() {}, scopeWorkspace, false
	}
	if l.maxResource > 0 && l.perResource[resource] >= l.maxResource {
		l.mu.Unlock()
		return func() {}, "resource", false
	}
	if l.maxWorkspace > 0 {
		l.perWorkspace[workspace]++
	}
	if l.maxResource > 0 {
		l.perResource[resource]++
	}
	l.mu.Unlock()

	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			if l.maxWorkspace > 0 {
				decrementKey(l.perWorkspace, workspace)
			}
			if l.maxResource > 0 {
				decrementKey(l.perResource, resource)
			}
			l.mu.Unlock()
		})
	}, "", true
}

func decrementKey[K comparable](counts map[K]int, key K) {
	if counts[key] <= 1 {
		delete(counts, key)
		return
	}
	counts[key]--
}

// Serve is the shared accept loop for the SNI proxies. It admits each accepted
// connection through limiter's global cap BEFORE dispatching a handler goroutine,
// so an overload flood is shed at accept time (the connection is closed, no
// goroutine and no backend dial) instead of growing the process unboundedly
// (finding 6). handle owns the connection and must close it; the global slot is
// released when handle returns. onAcceptErr, if non-nil, is called for a
// non-shutdown Accept error. Serve returns when ctx is cancelled (and the
// listener is closed by the caller).
func Serve(
	ctx context.Context,
	ln net.Listener,
	limiter *Limiter,
	meter *ByteMeter,
	handle func(net.Conn),
	onAcceptErr func(error),
) {
	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			if onAcceptErr != nil {
				onAcceptErr(err)
			}
			continue
		}
		release, ok := limiter.AcquireGlobal()
		if !ok {
			if meter != nil {
				meter.Reject("global")
			}
			_ = conn.Close()
			continue
		}
		go func() {
			defer release()
			handle(conn)
		}()
	}
}
