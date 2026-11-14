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
	"fmt"
	"sync"
	"time"
)

// originFetchTimeout bounds one origin fetch. The fetch is shared by every
// request waiting on its key and runs apart from whichever request started it,
// so a client that goes away cannot fail the others; the deadline is what keeps
// an origin that never answers from holding a fetch-gate slot for good. A var
// so a test can shorten it.
var originFetchTimeout = 30 * time.Second

// flight is one origin fetch shared by the requests waiting on its key.
type flight struct {
	ctx     context.Context
	cancel  context.CancelFunc
	done    chan struct{} // closed once obj and err are set
	obj     Object
	err     error
	waiters int // guarded by flights.mu
}

// flights collapses concurrent misses for one object key into a single origin
// fetch (finding 12): a viral asset is fetched once, not once per request.
//
// It is singleflight with two differences. The fetch does not belong to the
// request that started it: it runs under a context that keeps that request's
// values but not its cancellation, so the leader's client going away does not
// fail every request that coalesced onto its fetch. And the fetch is cancelled
// when the last request waiting on it leaves, so work nobody wants (and an
// admission wait nobody needs) stops at once instead of running to its deadline.
// That relies on the origin honoring its context, as the S3 origin does.
type flights struct {
	mu sync.Mutex
	m  map[string]*flight
}

func newFlights() *flights { return &flights{m: map[string]*flight{}} }

// do runs fetch for key, or waits on the one already running, and returns its
// result; it returns early with ctx's error when the caller's own context ends.
func (f *flights) do(ctx context.Context, key string, fetch func(context.Context) (Object, error)) (Object, error) {
	fl := f.join(ctx, key, fetch)
	defer f.leave(fl)
	select {
	case <-ctx.Done():
		return Object{}, ctx.Err()
	case <-fl.done:
		return fl.obj, fl.err
	}
}

// join returns the flight for key, starting fetch under its own deadline when
// none is running, and counts the caller as waiting on it. Every join is paired
// with one leave (do pairs them). A flight whose context has ended (everyone left, or its
// deadline passed) is on its way out and is not joined: the caller starts a
// fresh one.
func (f *flights) join(ctx context.Context, key string, fetch func(context.Context) (Object, error)) *flight {
	f.mu.Lock()
	defer f.mu.Unlock()
	fl := f.m[key]
	if fl == nil || fl.ctx.Err() != nil {
		fctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), originFetchTimeout)
		fl = &flight{ctx: fctx, cancel: cancel, done: make(chan struct{})}
		f.m[key] = fl
		go f.run(key, fl, fetch)
	}
	fl.waiters++
	return fl
}

func (f *flights) run(key string, fl *flight, fetch func(context.Context) (Object, error)) {
	completed := false
	defer func() {
		// A panic in an origin must not take the shared server down from a
		// goroutine no request's recovery covers, and a fetch that ended without
		// returning (a panic, runtime.Goexit) must not read as an empty object.
		if r := recover(); r != nil || !completed {
			fl.err = fmt.Errorf("staticserver: origin fetch did not complete: %v", r)
		}
		f.mu.Lock()
		if f.m[key] == fl {
			delete(f.m, key)
		}
		f.mu.Unlock()
		fl.cancel()
		close(fl.done)
	}()
	fl.obj, fl.err = fetch(fl.ctx)
	completed = true
}

// leave ends one caller's wait. The last caller out cancels the fetch and
// waits for it to unwind, so nothing a request started is still running when
// the request returns. The cancel happens under the lock that join takes: a
// request that joins just after the last one left must see the flight as ended
// and start its own, not inherit the cancellation.
func (f *flights) leave(fl *flight) {
	f.mu.Lock()
	fl.waiters--
	last := fl.waiters == 0
	if last {
		fl.cancel()
	}
	f.mu.Unlock()
	if last {
		<-fl.done
	}
}
