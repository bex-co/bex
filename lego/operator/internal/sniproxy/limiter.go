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
}

// scopeWorkspace is the rejectedScope returned when the per-workspace cap
// sheds a connection.
const scopeWorkspace = "workspace"

// NewLimiter builds a Limiter with the given global and per-source concurrent
// connection caps. A cap <= 0 disables that dimension.
func NewLimiter(maxGlobal, maxSource int) *Limiter {
	return &Limiter{
		maxGlobal:    maxGlobal,
		maxSource:    maxSource,
		perSource:    make(map[netip.Addr]int),
		perWorkspace: make(map[string]int),
		perResource:  make(map[string]int),
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
	key := addr.Unmap()
	l.mu.Lock()
	if l.perSource[key] >= l.maxSource {
		l.mu.Unlock()
		return func() {}, false
	}
	l.perSource[key]++
	l.mu.Unlock()
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			if l.perSource[key] <= 1 {
				delete(l.perSource, key)
			} else {
				l.perSource[key]--
			}
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

func decrementKey(counts map[string]int, key string) {
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
