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

import (
	"context"
	"sync"
	"time"
)

// Origin-fetch admission waits. A cache miss that finds its client, its site or
// the replica at capacity waits briefly for a slot instead of failing at once: a
// deploy empties every replica's cache, so a visitor's first cold page asks for
// its HTML, stylesheets, scripts and images together, and with the client cap at
// 3 the extras answered 503, which a browser does not retry for a subresource.
//
// A waiter holds no fetch slot and no buffer, only a timer on a request
// goroutine that exists anyway, so one client still cannot take other tenants'
// slots. The wait is short and the waiters are bounded per replica and per
// client, so an overload is still shed with 503 + Retry-After, a little later.
// 48 waiters per client is a cold page of about 50 assets (3 hold slots); 128 per
// replica keeps the wake-all below on the order of 100 µs per release.
const (
	defaultAdmissionWait    = time.Second
	defaultMaxWaiters       = 128
	defaultMaxClientWaiters = 48
)

// admission lets a request that cannot be admitted wait for capacity to free.
type admission struct {
	// wait is how long a request waits; zero or less means it never does, as
	// before (the tests use that to show what a cold page saw).
	wait         time.Duration
	maxWaiters   int
	maxPerClient int

	mu sync.Mutex
	// changed is closed, and replaced, when capacity is released while someone
	// waits, waking every waiter to try again.
	changed  chan struct{}
	waiting  int
	byClient map[string]int
}

func newAdmission() *admission {
	return &admission{
		wait:         defaultAdmissionWait,
		maxWaiters:   defaultMaxWaiters,
		maxPerClient: defaultMaxClientWaiters,
		changed:      make(chan struct{}),
		byClient:     map[string]int{},
	}
}

// notify wakes the waiters; it costs a lock and nothing else when none wait.
func (a *admission) notify() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.waiting == 0 {
		return
	}
	close(a.changed)
	a.changed = make(chan struct{})
}

// enter registers a waiter, or refuses at once when the replica or the client
// already has as many as it may.
func (a *admission) enter(client string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.wait <= 0 || a.waiting >= a.maxWaiters || (client != "" && a.byClient[client] >= a.maxPerClient) {
		return false
	}
	a.waiting++
	if client != "" {
		a.byClient[client]++
	}
	return true
}

func (a *admission) leave(client string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.waiting--
	if client == "" {
		return
	}
	if a.byClient[client] <= 1 {
		delete(a.byClient, client)
	} else {
		a.byClient[client]--
	}
}

// await calls try until it admits, the wait runs out, or ctx ends; try returns
// the release for what it reserved. The wake-up channel is read before each try,
// so a release between a failed try and the wait is not missed, and the first
// try repeats the caller's own because a release may have come before enter.
func (a *admission) await(ctx context.Context, client string, try func() (release func(), ok bool)) (func(), bool) {
	if !a.enter(client) {
		return nil, false
	}
	defer a.leave(client)
	timer := time.NewTimer(a.wait)
	defer timer.Stop()
	for {
		a.mu.Lock()
		changed := a.changed
		a.mu.Unlock()
		if release, ok := try(); ok {
			return release, true
		}
		select {
		case <-changed:
		case <-timer.C:
			return nil, false
		case <-ctx.Done():
			return nil, false
		}
	}
}
