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

package core

import (
	"bytes"
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"testing"
	"time"
)

// captureLog redirects the standard logger for one test and returns a func that
// reads what was written. The Poll loop logs through log.Printf, which is the
// behavior under test — a worker's failures must reach the operator's log.
func captureLog(t *testing.T) func() string {
	t.Helper()
	sink := &syncBuffer{}
	previous, flags := log.Writer(), log.Flags()
	log.SetOutput(sink)
	log.SetFlags(0)
	t.Cleanup(func() {
		log.SetOutput(previous)
		log.SetFlags(flags)
	})
	return sink.String
}

// syncBuffer is written by the poll goroutine and read by the test, so it needs
// its own lock — log's internal one does not cover the read side.
type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.buf.String()
}

// awaitCall reports the next step invocation, failing the test rather than
// hanging forever if the loop stopped.
func awaitCall(t *testing.T, calls <-chan struct{}) {
	t.Helper()
	select {
	case <-calls:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the poll loop to run its step")
	}
}

// The step runs before the first tick: a restarted worker must resume durable
// work immediately, not idle for a whole interval first.
func TestPollTicksRunsTheStepBeforeTheFirstTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := make(chan struct{}, 8)
	done := make(chan struct{})
	// A channel that never fires: only the pre-tick run can call the step.
	go func() {
		defer close(done)
		PollTicks(ctx, "test worker", make(chan time.Time), func(context.Context) error {
			calls <- struct{}{}
			return nil
		})
	}()

	awaitCall(t, calls)
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("PollTicks did not return after its context was canceled")
	}
	select {
	case <-calls:
		t.Error("the step ran again after cancellation")
	default:
	}
}

func TestPollTicksRunsOncePerTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time)
	calls := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		PollTicks(ctx, "test worker", ticks, func(context.Context) error {
			calls <- struct{}{}
			return nil
		})
	}()

	awaitCall(t, calls) // the pre-tick run
	for range 3 {
		ticks <- time.Now()
		awaitCall(t, calls)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("PollTicks did not return after its context was canceled")
	}
}

// A failing step is logged under the worker's name and the loop keeps going:
// every one of these workers reads durable work, so the next tick retries.
func TestPollTicksLogsFailuresAndKeepsRunning(t *testing.T) {
	logged := captureLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time)
	calls := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		PollTicks(ctx, "test worker", ticks, func(context.Context) error {
			calls <- struct{}{}
			return errors.New("boom")
		})
	}()

	awaitCall(t, calls)
	ticks <- time.Now()
	awaitCall(t, calls) // still running after a failure
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("PollTicks did not return after its context was canceled")
	}
	if got := logged(); !strings.Contains(got, "test worker: boom") {
		t.Errorf("log = %q, want it to name the worker and the error", got)
	}
}

// During shutdown an in-flight step fails only because its context was
// canceled. Logging that would report a routine stop as a worker error, so
// every copy of this loop guarded it — and now one place does.
func TestPollTicksDoesNotLogAStepThatFailedBecauseOfShutdown(t *testing.T) {
	logged := captureLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		defer close(done)
		PollTicks(ctx, "test worker", make(chan time.Time), func(ctx context.Context) error {
			cancel() // the shutdown that the step is about to fail from
			return context.Canceled
		})
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("PollTicks did not return after its context was canceled")
	}
	if got := logged(); got != "" {
		t.Errorf("log = %q, want nothing: a step that failed during shutdown is not a worker error", got)
	}
}

// A wake channel runs the step ahead of its next tick, which is how a store
// reconciler reacts to a write instead of waiting out its resync.
func TestPollWakeRunsTheStepAheadOfTheTick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	wake := make(chan struct{}, 1)
	calls := make(chan struct{}, 8)
	done := make(chan struct{})
	go func() {
		defer close(done)
		// An hour between ticks: only a wake can drive the step again.
		PollWake(ctx, "test worker", time.Hour, wake, func(context.Context) error {
			calls <- struct{}{}
			return nil
		})
	}()

	awaitCall(t, calls) // the pre-tick run
	for range 3 {
		wake <- struct{}{}
		awaitCall(t, calls)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("PollWake did not return after its context was canceled")
	}
}

// A non-positive interval is a misconfiguration (BEX_CP_RESYNC=0s reaches one of
// these loops). time.NewTicker panics on it, which in a worker goroutine takes
// the process down, so the floor is applied here rather than in four callers.
func TestPollFloorsANonPositiveInterval(t *testing.T) {
	for _, interval := range []time.Duration{0, -time.Second} {
		ctx, cancel := context.WithCancel(context.Background())
		calls := make(chan struct{}, 1)
		done := make(chan struct{})
		go func() {
			defer close(done)
			Poll(ctx, "test worker", interval, func(context.Context) error {
				select {
				case calls <- struct{}{}:
				default:
				}
				return nil
			})
		}()
		awaitCall(t, calls) // reached the step at all => no panic
		cancel()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Fatalf("Poll(%v) did not return after its context was canceled", interval)
		}
	}
}

// An already-canceled context runs no step at all. Without the check at the top
// of the loop a ready tick and a ready cancellation race in the select, so a
// stopping worker could still make one full pass against a dead context.
func TestPollRunsNoStepForAnAlreadyCanceledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	ticks := make(chan time.Time, 1)
	ticks <- time.Now() // both cases in the select are ready
	var calls int
	PollTicks(ctx, "test worker", ticks, func(context.Context) error {
		calls++
		return nil
	})
	if calls != 0 {
		t.Errorf("step ran %d times against an already-canceled context, want 0", calls)
	}
}

// Poll owns its ticker: the step keeps running without any caller-supplied
// channel, and the loop still returns on cancellation. It asserts repetition,
// not a precise cadence — timing that tight would only buy flakes.
func TestPollKeepsRunningOnItsOwnTicker(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := make(chan struct{}, 16)
	done := make(chan struct{})
	go func() {
		defer close(done)
		Poll(ctx, "test worker", time.Millisecond, func(context.Context) error {
			select {
			case calls <- struct{}{}:
			default: // never block the loop once the test has seen enough
			}
			return nil
		})
	}()

	for range 3 {
		awaitCall(t, calls)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Poll did not return after its context was canceled")
	}
}

// A panicking step is one failed pass, not the end of the process (w6/m95).
// These workers each sweep EVERY row in a table, so an unrecovered nil
// dereference on one malformed row takes bex-api down and stops the store
// reconciler's deploy gate timeouts — the backstop that keeps a deploy row from
// sitting in a non-terminal status forever — for every app, not just that one.
func TestPollTicksSurvivesAPanickingStep(t *testing.T) {
	logged := captureLog(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ticks := make(chan time.Time)
	calls := make(chan struct{}, 8)
	done := make(chan struct{})
	panics := 0
	go func() {
		defer close(done)
		PollTicks(ctx, "test worker", ticks, func(context.Context) error {
			calls <- struct{}{}
			panics++
			if panics == 1 {
				panic("nil deploy row")
			}
			return nil
		})
	}()

	awaitCall(t, calls)
	ticks <- time.Now()
	awaitCall(t, calls) // the next tick still ran: one bad row is not fatal
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("PollTicks did not return after its context was canceled")
	}
	got := logged()
	if !strings.Contains(got, "nil deploy row") {
		t.Errorf("log = %q, want the panic value", got)
	}
	if !strings.Contains(got, "poll.go") {
		t.Errorf("log = %q, want a stack trace — the panic value alone will not identify the row", got)
	}
	if !strings.Contains(got, "test worker") {
		t.Errorf("log = %q, want the worker named", got)
	}
}
