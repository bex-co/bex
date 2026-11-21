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

package s3

import (
	"context"
	"errors"
	"fmt"
	"sync"
)

// reported is an error whose details were already printed, line by line: the
// group counts it and does not print it again.
type reported struct{ error }

func (e reported) Unwrap() error { return e.error }

// group runs a command's operations on at most n goroutines. A producer that
// calls do blocks while n are running, so a listing never races ahead of what
// the workers can finish. One operation failing is reported and the rest go on;
// an error every later operation would meet too (an expired key, a quota) stops
// the whole group.
type group struct {
	r      *run
	ctx    context.Context
	cancel context.CancelCauseFunc
	slots  chan struct{}
	wg     sync.WaitGroup

	mu       sync.Mutex
	failures int
	total    int
	stopped  error
}

func (r *run) newGroup(n int) *group {
	ctx, cancel := context.WithCancelCause(r.ctx)
	return &group{r: r, ctx: ctx, cancel: cancel, slots: make(chan struct{}, max(n, 1))}
}

// do runs fn as one operation, named label in a failure line. It returns false
// once the group has stopped, and the producer should stop producing.
func (g *group) do(label string, fn func(ctx context.Context) error) bool {
	select {
	case g.slots <- struct{}{}:
	case <-g.ctx.Done():
		return false
	}
	g.mu.Lock()
	g.total++
	g.mu.Unlock()
	g.wg.Add(1)
	go func() {
		defer func() { <-g.slots; g.wg.Done() }()
		if err := fn(g.ctx); err != nil {
			g.failed(label, err)
		}
	}()
	return true
}

func (g *group) failed(label string, err error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if fatal(err) {
		if g.stopped == nil {
			g.stopped = err
			g.cancel(err)
		}
		return
	}
	g.failures++
	var done reported
	if errors.As(err, &done) {
		return
	}
	g.r.prog.clear()
	_, _ = fmt.Fprintf(g.r.errOut, "failed: %s: %s\n", label, g.r.fail(err))
}

// alive is false once the group has stopped.
func (g *group) alive() bool { return g.ctx.Err() == nil }

// wait ends the group and returns what went wrong: the error that stopped it,
// or a count of the operations that failed.
func (g *group) wait() error {
	g.wg.Wait()
	g.cancel(nil)
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.stopped != nil {
		return g.r.fail(g.stopped)
	}
	if g.failures > 0 {
		return fmt.Errorf("%d of %d operations failed", g.failures, g.total)
	}
	return nil
}
