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
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
	"testing"
	"time"

	"k8s.io/client-go/tools/remotecommand"

	"github.com/bex-co/bex/lego/backend/internal/apps"
)

// gatedWriter blocks every Write until open is closed, like an SSH client
// that has stopped reading.
type gatedWriter struct {
	open chan struct{}
	mu   sync.Mutex
	buf  bytes.Buffer
	err  error
}

func (g *gatedWriter) Write(p []byte) (int, error) {
	<-g.open
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err != nil {
		return 0, g.err
	}
	return g.buf.Write(p)
}

// burstExecutor writes total bytes of a pattern as fast as its stdout allows
// and records when it finished, as the apiserver stream would deliver them.
type burstExecutor struct {
	total    int
	finished chan struct{}
}

func (b burstExecutor) Execute(_ context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, stdout, _ io.Writer) (int, error) {
	chunk := make([]byte, 32<<10)
	for written := 0; written < b.total; written += len(chunk) {
		for i := range chunk {
			chunk[i] = byte(written + i)
		}
		if _, err := stdout.Write(chunk[:min(len(chunk), b.total-written)]); err != nil {
			return 1, err
		}
	}
	close(b.finished)
	return 3, nil
}

func pattern(total int) []byte {
	out := make([]byte, total)
	for i := range out {
		out[i] = byte(i)
	}
	return out
}

// w6/099: with the client stalled, the executor still drains the apiserver
// stream up to the per-channel cap, and Execute returns only after every byte
// reached the client, in order, with the exit code.
func TestDrainingExecutorReadsAheadOfAStalledClient(t *testing.T) {
	const total = 4 << 20
	finished := make(chan struct{})
	client := &gatedWriter{open: make(chan struct{})}
	e := DrainingExecutor{Executor: burstExecutor{total: total, finished: finished}, Budget: NewDrainBudget(8<<20, 16<<20)}
	result := make(chan int, 1)
	go func() {
		code, err := e.Execute(t.Context(), apps.SSHInstanceTarget{}, nil, false, nil, nil, client, io.Discard)
		if err != nil {
			t.Error(err)
		}
		result <- code
	}()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the executor was held back by a stalled client below the cap")
	}
	select {
	case <-result:
		t.Fatal("Execute returned before the buffered output reached the client")
	case <-time.After(50 * time.Millisecond):
	}
	close(client.open)
	if code := <-result; code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	if !bytes.Equal(client.buf.Bytes(), pattern(total)) {
		t.Fatalf("client got %d bytes, want the exact %d in order", client.buf.Len(), total)
	}
}

// Past the per-channel cap the writer applies backpressure instead of
// growing: the executor cannot finish while the client is stalled.
func TestDrainingExecutorBlocksAtThePerChannelCap(t *testing.T) {
	finished := make(chan struct{})
	client := &gatedWriter{open: make(chan struct{})}
	e := DrainingExecutor{Executor: burstExecutor{total: 4 << 20, finished: finished}, Budget: NewDrainBudget(1<<20, 16<<20)}
	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = e.Execute(t.Context(), apps.SSHInstanceTarget{}, nil, false, nil, nil, client, io.Discard)
	}()
	select {
	case <-finished:
		t.Fatal("the executor wrote 4 MiB past a 1 MiB cap with the client stalled")
	case <-time.After(200 * time.Millisecond):
	}
	close(client.open)
	<-done
	if client.buf.Len() != 4<<20 {
		t.Fatalf("client got %d bytes, want %d", client.buf.Len(), 4<<20)
	}
}

// A spent process-wide budget never blocks another channel: it writes
// straight through instead, so one tenant's stalled client cannot freeze
// everyone's output.
func TestSpentDrainBudgetFallsBackToDirectWrites(t *testing.T) {
	budget := NewDrainBudget(1<<20, 1<<20)
	stalled := &gatedWriter{open: make(chan struct{})}
	first := budget.Channel(t.Context())
	if _, err := first.Writer(stalled).Write(make([]byte, 1<<20)); err != nil {
		t.Fatal(err)
	}
	second := budget.Channel(t.Context())
	var other bytes.Buffer
	wrote := make(chan struct{})
	go func() {
		_, _ = second.Writer(&other).Write([]byte("x"))
		close(wrote)
	}()
	select {
	case <-wrote:
	case <-time.After(2 * time.Second):
		t.Fatal("a second channel waited on the first channel's stalled client")
	}
	if err := second.Close(); err != nil || other.String() != "x" {
		t.Fatalf("second channel: %q, %v", other.String(), err)
	}
	close(stalled.open)
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
}

// Ending the channel's context releases a session whose client stopped
// reading: Execute returns instead of waiting on the buffered output.
func TestDrainingExecutorReturnsOnCancelWithAStalledClient(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	finished := make(chan struct{})
	client := &gatedWriter{open: make(chan struct{})}
	defer close(client.open)
	e := DrainingExecutor{Executor: burstExecutor{total: 1 << 20, finished: finished}, Budget: NewDrainBudget(8<<20, 16<<20)}
	result := make(chan error, 1)
	go func() {
		_, err := e.Execute(ctx, apps.SSHInstanceTarget{}, nil, false, nil, nil, client, io.Discard)
		result <- err
	}()
	<-finished
	cancel()
	select {
	case err := <-result:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("Execute kept waiting on a stalled client after cancellation")
	}
}

// A panicking executor still ends its writers and returns their budget.
func TestDrainingExecutorReturnsBudgetOnPanic(t *testing.T) {
	budget := NewDrainBudget(1<<20, 1<<20)
	e := DrainingExecutor{Executor: panicExecutor{}, Budget: budget}
	func() {
		defer func() { _ = recover() }()
		_, _ = e.Execute(t.Context(), apps.SSHInstanceTarget{}, nil, false, nil, nil, io.Discard, io.Discard)
	}()
	if !budget.total.TryAcquire(1 << 20) {
		t.Fatal("the panicked channel kept its share of the budget")
	}
}

type panicExecutor struct{}

func (panicExecutor) Execute(_ context.Context, _ apps.SSHInstanceTarget, _ []string, _ bool, _ remotecommand.TerminalSizeQueue, _ io.Reader, stdout, _ io.Writer) (int, error) {
	_, _ = stdout.Write([]byte("partial"))
	panic("boom")
}

// A failed forward surfaces to the executor's next write and to Close.
func TestDrainWriterReportsTheForwardingError(t *testing.T) {
	broken := &gatedWriter{open: make(chan struct{}), err: errors.New("channel closed")}
	close(broken.open)
	drain := NewDrainBudget(1<<20, 1<<20).Channel(t.Context())
	w := drain.Writer(broken)
	_, _ = w.Write([]byte("a"))
	deadline := time.Now().Add(2 * time.Second)
	var err error
	for err == nil && time.Now().Before(deadline) {
		_, err = w.Write([]byte("b"))
		time.Sleep(time.Millisecond)
	}
	if err == nil {
		t.Fatal("writes kept succeeding after the forward failed")
	}
	if cerr := drain.Close(); cerr == nil {
		t.Fatal("Close did not report the forwarding error")
	}
}

// Zero sizes disable buffering: writers are passed through untouched.
func TestDisabledDrainBudgetPassesWritersThrough(t *testing.T) {
	var buf bytes.Buffer
	if w := NewDrainBudget(0, 0).Channel(t.Context()).Writer(&buf); w != io.Writer(&buf) {
		t.Fatalf("writer = %T, want the original", w)
	}
}
