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

package sshgateway

import (
	"context"
	"io"
	"sync"

	"golang.org/x/sync/semaphore"
	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
)

// DrainBudget bounds the exec output the gateway holds in memory, per channel
// and across the process (w6/099). kube-apiserver's upgrade proxy closes an
// exec stream as soon as its client→backend half fails, which a slow reader
// triggers once the process exits, and the output still in flight is lost.
// Reading the apiserver stream at full speed into this buffer, and forwarding
// it at the SSH client's pace, keeps that tail.
//
// Only a channel's own cap ever blocks its writer. When the process-wide
// budget is spent, a channel falls back to writing straight through (the
// unbuffered backpressure), so stalled clients cannot freeze other sessions.
type DrainBudget struct {
	total      *semaphore.Weighted
	perChannel int64
}

// NewDrainBudget allows perChannel bytes per channel and total across all;
// non-positive values disable buffering (writes pass straight through).
func NewDrainBudget(perChannel, total int) *DrainBudget {
	if perChannel <= 0 || total <= 0 {
		return nil
	}
	return &DrainBudget{total: semaphore.NewWeighted(int64(total)), perChannel: int64(min(perChannel, total))}
}

// ChannelDrain is one channel's share of the budget: its stdout and stderr
// writers together hold at most the per-channel cap. stdout and stderr keep
// their own order; their relative order may drift by up to the cap, which
// remotecommand never guaranteed either.
type ChannelDrain struct {
	ctx     context.Context
	budget  *DrainBudget
	own     *semaphore.Weighted
	writers []*DrainWriter
}

// Channel opens a channel's share; nil when buffering is disabled. ctx ends
// the channel: blocked writes and Close return once it is done.
func (b *DrainBudget) Channel(ctx context.Context) *ChannelDrain {
	if b == nil {
		return nil
	}
	return &ChannelDrain{ctx: ctx, budget: b, own: semaphore.NewWeighted(b.perChannel)}
}

// Writer wraps w; a nil ChannelDrain returns w itself.
func (c *ChannelDrain) Writer(w io.Writer) io.Writer {
	if c == nil {
		return w
	}
	d := &DrainWriter{channel: c, dst: w, done: make(chan struct{})}
	d.cond = sync.NewCond(&d.mu)
	c.writers = append(c.writers, d)
	go d.run()
	return d
}

// Close forwards everything buffered and reports the first write error. Once
// the channel's context is done it discards what is left instead of waiting
// on a client that stopped reading.
func (c *ChannelDrain) Close() error {
	if c == nil {
		return nil
	}
	var first error
	for _, w := range c.writers {
		if err := w.close(c.ctx); err != nil && first == nil {
			first = err
		}
	}
	return first
}

// maxDrainChunk bounds one buffered write, so a large Write never needs the
// whole per-channel cap at once.
const maxDrainChunk = 256 << 10

// DrainWriter queues writes and forwards them, in order, from one goroutine.
type DrainWriter struct {
	channel *ChannelDrain
	dst     io.Writer
	mu      sync.Mutex
	cond    *sync.Cond
	queue   [][]byte
	sending bool
	closed  bool
	err     error
	done    chan struct{}
}

// Write buffers p (the caller may reuse p), or writes it straight through
// when the process-wide budget is spent, and returns the first forwarding
// error.
func (d *DrainWriter) Write(p []byte) (int, error) {
	c := d.channel
	written := 0
	for len(p) > 0 {
		n := min(len(p), maxDrainChunk, int(c.budget.perChannel))
		if err := d.failed(); err != nil {
			return written, err
		}
		if err := c.own.Acquire(c.ctx, int64(n)); err != nil {
			return written, err
		}
		if !c.budget.total.TryAcquire(int64(n)) {
			c.own.Release(int64(n))
			if err := d.direct(p[:n]); err != nil {
				return written, err
			}
		} else {
			chunk := append([]byte(nil), p[:n]...)
			d.mu.Lock()
			d.queue = append(d.queue, chunk)
			d.mu.Unlock()
			d.cond.Broadcast()
		}
		p, written = p[n:], written+n
	}
	return written, nil
}

// direct waits for this writer's queue to drain, keeping order, then writes
// chunk itself.
func (d *DrainWriter) direct(chunk []byte) error {
	d.mu.Lock()
	for (len(d.queue) > 0 || d.sending) && d.err == nil {
		d.cond.Wait()
	}
	err := d.err
	d.mu.Unlock()
	if err != nil {
		return err
	}
	if _, err := d.dst.Write(chunk); err != nil {
		d.fail(err)
		return err
	}
	return nil
}

func (d *DrainWriter) failed() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.err
}

func (d *DrainWriter) fail(err error) {
	d.mu.Lock()
	if d.err == nil {
		d.err = err
	}
	d.mu.Unlock()
	d.cond.Broadcast()
}

func (d *DrainWriter) run() {
	defer close(d.done)
	for {
		d.mu.Lock()
		for len(d.queue) == 0 && !d.closed {
			d.cond.Wait()
		}
		if len(d.queue) == 0 {
			d.mu.Unlock()
			return
		}
		chunk := d.queue[0]
		d.queue = d.queue[1:]
		d.sending = true
		failed := d.err != nil
		d.mu.Unlock()
		if !failed {
			if _, err := d.dst.Write(chunk); err != nil {
				d.fail(err)
			}
		}
		d.channel.budget.total.Release(int64(len(chunk)))
		d.channel.own.Release(int64(len(chunk)))
		d.mu.Lock()
		d.sending = false
		d.mu.Unlock()
		d.cond.Broadcast()
	}
}

// close ends the writer and waits for its queue to drain, or until ctx is
// done; then the rest is discarded and returned to the budget as run reaches
// it (a forward stuck on a dead client ends when the caller closes dst).
func (d *DrainWriter) close(ctx context.Context) error {
	d.mu.Lock()
	d.closed = true
	d.mu.Unlock()
	d.cond.Broadcast()
	select {
	case <-d.done:
	case <-ctx.Done():
		d.fail(ctx.Err())
	}
	return d.failed()
}

// DrainingExecutor buffers an Executor's output through a DrainBudget and
// returns only once everything buffered has reached the SSH channel, so the
// exit status still follows the output.
type DrainingExecutor struct {
	Executor
	Budget *DrainBudget
}

// Execute implements Executor.
func (e DrainingExecutor) Execute(ctx context.Context, target apps.SSHInstanceTarget, command []string, tty bool, sizes remotecommand.TerminalSizeQueue, stdin io.Reader, stdout, stderr io.Writer) (code int, err error) {
	drain := e.Budget.Channel(ctx)
	if stdout != nil {
		stdout = drain.Writer(stdout)
	}
	if stderr != nil {
		stderr = drain.Writer(stderr)
	}
	// Deferred so a panicking executor still ends the writers and returns
	// their budget.
	defer func() {
		if drainErr := drain.Close(); err == nil && drainErr != nil {
			err = drainErr
		}
	}()
	return e.Executor.Execute(ctx, target, command, tty, sizes, stdin, stdout, stderr)
}
