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
	"sync"
	"time"
)

// Drain is how an SNI proxy stops for a release (w6/m515). Once Start closes,
// the listener closes, so the pod accepts nothing new, and every open
// connection keeps relaying until its client or the backend ends it.
// Connections still open Timeout later close as on a hard stop, so a zero
// Timeout closes them at once. A nil Start never drains.
type Drain struct {
	Start   <-chan struct{}
	Timeout time.Duration
}

// Serve is the shared accept loop for the SNI proxies. It admits each accepted
// connection through limiter's global cap BEFORE dispatching a handler goroutine,
// so an overload flood is shed at accept time (the connection is closed, no
// goroutine and no backend dial) instead of growing the process unboundedly
// (finding 6). handle owns the connection and must close it; the global slot is
// released when handle returns. onAcceptErr, if non-nil, is called for a
// non-shutdown Accept error.
//
// Serve owns ln. Cancelling ctx is the hard stop: ln and every open connection
// close at once. Once drain starts, ln closes and the open connections run on
// until they end or the drain's deadline closes them. Either way Serve returns
// once every handler has returned, so every limiter slot is free by then, and
// a drain whose last connection ends early returns early.
func Serve(
	ctx context.Context,
	ln net.Listener,
	limiter *Limiter,
	meter *ByteMeter,
	drain Drain,
	handle func(net.Conn),
	onAcceptErr func(error),
) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var mu sync.Mutex
	open := make(map[net.Conn]struct{})
	var handlers sync.WaitGroup
	stopped := make(chan struct{})
	context.AfterFunc(ctx, func() {
		defer close(stopped)
		_ = ln.Close()
		mu.Lock()
		defer mu.Unlock()
		for conn := range open {
			_ = conn.Close()
		}
	})
	defer func() { cancel(); <-stopped; handlers.Wait() }()
	go func() {
		select {
		case <-drain.Start:
		case <-ctx.Done():
			return
		}
		_ = ln.Close()
		deadline := time.AfterFunc(drain.Timeout, cancel)
		<-ctx.Done()
		deadline.Stop()
	}()
	for {
		conn, err := ln.Accept()
		if err != nil {
			if draining(drain.Start) {
				// The drain closed ln. Wait for the open connections, which
				// the deadline closes if they are still open then.
				handlers.Wait()
				return
			}
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
		mu.Lock()
		if ctx.Err() != nil {
			mu.Unlock()
			release()
			_ = conn.Close()
			return
		}
		open[conn] = struct{}{}
		mu.Unlock()
		handlers.Go(func() {
			defer func() {
				mu.Lock()
				delete(open, conn)
				mu.Unlock()
				release()
			}()
			handle(conn)
		})
	}
}

// draining reports whether drain has closed; a nil drain never does.
func draining(drain <-chan struct{}) bool {
	select {
	case <-drain:
		return true
	default:
		return false
	}
}
