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

package controller

import (
	"context"
	"errors"
	"fmt"
	"math/rand/v2"
	"sync"
	"time"

	"github.com/bex-co/bex/lego/operator/internal/registry"
)

// The operator reads a tenant's image registry inside an App reconcile worker
// (resolveImageNetwork), and a registry that accepts a connection and never
// answers holds that worker for the whole read budget, on every attempt, for
// every App of the tenant. With two workers, about 14 such Apps kept both busy
// and every other tenant's reconcile queued behind them (w21/m394 t014, R25-04).
//
// registryReadGate bounds the tenant-controlled part, without moving the
// starvation onto other tenants' image deploys: one read at a time per registry
// host and per workspace, a breaker per host that backs off exponentially after
// a failed read, and a short busy answer, in place of waiting, whenever either
// bound or the breaker says no. A busy answer frees the worker at once. Reads of
// the platform's own registry are not tenant-controlled and do not pass through.
const (
	maxRegistryReadsPerHost      = 1
	maxRegistryReadsPerWorkspace = 1
	// registryBusyRetry is the short retry after a busy answer (a read of that
	// host or by that workspace is in flight).
	registryBusyRetry = 5 * time.Second
	// registryHostMemory is how long a failing host's history outlives its
	// breaker window, so a host that fails again soon backs off further.
	registryHostMemory = 5 * time.Minute
	// registryHostSweepAt is how many hosts the gate holds before it forgets
	// the failing ones past their memory.
	registryHostSweepAt = 512
)

type registryReadGate struct {
	mu          sync.Mutex
	now         func() time.Time
	hosts       map[string]*registryHost
	workspaces  map[string]int
	appFailures map[string]int
}

type registryHost struct {
	inflight  int
	failures  int
	openUntil time.Time
}

func newRegistryReadGate() *registryReadGate {
	return &registryReadGate{
		now:         time.Now,
		hosts:       map[string]*registryHost{},
		workspaces:  map[string]int{},
		appFailures: map[string]int{},
	}
}

// registryRetryDelay is the wait after the nth consecutive transient failure of
// one registry read, and the window a host's breaker stays closed after its nth
// consecutive failed read: 30 s, doubling to 5 minutes.
func registryRetryDelay(failures int) time.Duration {
	shift := min(max(failures-1, 0), 4)
	return min(imageRegistryRetry<<shift, imageRegistryRetryMax)
}

// acquire admits one read of host by workspace, or says how long to wait. The
// caller reports the outcome through release: failed is true only for a read
// that did not get an answer (a timeout or a transient status).
func (g *registryReadGate) acquire(host, workspace string) (release func(failed bool), retryAfter time.Duration, ok bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	h := g.hosts[host]
	if h != nil && now.Before(h.openUntil) {
		return nil, h.openUntil.Sub(now), false
	}
	if h != nil && h.inflight >= maxRegistryReadsPerHost {
		return nil, registryBusyRetry, false
	}
	if g.workspaces[workspace] >= maxRegistryReadsPerWorkspace {
		return nil, registryBusyRetry, false
	}
	if h == nil {
		g.sweep(now)
		h = &registryHost{}
		g.hosts[host] = h
	}
	h.inflight++
	g.workspaces[workspace]++
	var once sync.Once
	return func(failed bool) { once.Do(func() { g.finish(host, workspace, failed) }) }, 0, true
}

func (g *registryReadGate) finish(host, workspace string, failed bool) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.workspaces[workspace] <= 1 {
		delete(g.workspaces, workspace)
	} else {
		g.workspaces[workspace]--
	}
	h := g.hosts[host]
	if h == nil {
		return
	}
	h.inflight--
	if failed {
		h.failures++
		h.openUntil = g.now().Add(registryRetryDelay(h.failures))
		return
	}
	h.failures, h.openUntil = 0, time.Time{}
	if h.inflight == 0 {
		delete(g.hosts, host)
	}
}

// sweep forgets failing hosts past their window and memory once the gate holds
// many hosts, so tenant-chosen host names cannot grow it without bound.
func (g *registryReadGate) sweep(now time.Time) {
	if len(g.hosts) < registryHostSweepAt {
		return
	}
	for host, h := range g.hosts {
		if h.inflight == 0 && now.After(h.openUntil.Add(registryHostMemory)) {
			delete(g.hosts, host)
		}
	}
}

// appFailed counts one more consecutive transient failure of the App's registry
// read and returns the count.
func (g *registryReadGate) appFailed(app string) int {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.appFailures[app]++
	return g.appFailures[app]
}

// appSucceeded forgets the App's failures: its read answered, or the App is gone.
func (g *registryReadGate) appSucceeded(app string) {
	g.mu.Lock()
	defer g.mu.Unlock()
	delete(g.appFailures, app)
}

// registryBusyError is the answer to a read the gate did not admit. It is not a
// failure of the registry or of the App's read, so it is retried soon and does
// not count toward the App's backoff.
type registryBusyError struct {
	host       string
	retryAfter time.Duration
}

func (e *registryBusyError) Error() string {
	return fmt.Sprintf("image registry %s is being read already or is backing off; retrying", e.host)
}

// busyRequeue is the busy answer's requeue: its wait plus up to five seconds, so
// the Apps turned away together do not all come back at once.
func (e *registryBusyError) busyRequeue() time.Duration {
	jitter := min(e.retryAfter, registryBusyRetry)
	return e.retryAfter + time.Duration(rand.Int64N(int64(jitter)+1))
}

func (r *AppReconciler) registryReads() *registryReadGate {
	r.registryReadsOnce.Do(func() { r.registryReadGate = newRegistryReadGate() })
	return r.registryReadGate
}

// readExternalImageConfig reads a tenant-named registry's image config through
// the read gate.
func (r *AppReconciler) readExternalImageConfig(ctx context.Context, namespace, host, repo, reference, authorization string) (registry.ImageConfig, error) {
	release, wait, ok := r.registryReads().acquire(host, namespace)
	if !ok {
		return registry.ImageConfig{}, &registryBusyError{host: host, retryAfter: wait}
	}
	metadata, err := registry.ReadImageConfig(ctx, r.HTTPClient, host, repo, reference, authorization)
	var transient *registry.TransientError
	release(errors.As(err, &transient) && ctx.Err() == nil)
	return metadata, err
}
