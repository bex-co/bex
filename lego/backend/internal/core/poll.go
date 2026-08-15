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
	"context"
	"log"
	"time"
)

// MinPollInterval is the floor Poll applies to a non-positive interval. A zero
// interval is a misconfiguration (BEX_CP_RESYNC=0s reaches one of these loops),
// and time.NewTicker panics on it — which in a worker goroutine takes the whole
// process down. Running slowly is the better failure.
const MinPollInterval = time.Second

// Poll runs step immediately and then once per interval until ctx is done,
// logging each failure as "name: err". It is the shape every bex-api background
// worker whose step REPORTS its failures shares, collected here so the parts
// that are easy to get subtly wrong are decided once:
//
//   - The step runs BEFORE the first tick, so a restart resumes durable work
//     immediately instead of idling for one interval.
//   - A step failure is never fatal. Every one of these workers reads its work
//     from a durable queue, so the next tick retries; the loop only ends when
//     the caller cancels.
//   - A failure is NOT logged once ctx is already done. During shutdown an
//     in-flight step fails simply because its context was canceled, and logging
//     that would report a routine stop as a worker error.
//
// A worker whose step logs internally and returns nothing keeps its own loop:
// its errors never reach name, so routing it through here would only borrow the
// ticker and advertise an error contract it does not have.
func Poll(ctx context.Context, name string, interval time.Duration, step func(context.Context) error) {
	PollWake(ctx, name, interval, nil, step)
}

// PollWake is Poll plus a wake channel that runs the step ahead of its next
// tick, for a worker that is also nudged by an event (a store reconciler woken
// by a write). Sends on wake should be non-blocking at the sender, so a wake
// arriving mid-step coalesces into the next pass rather than queueing.
func PollWake(ctx context.Context, name string, interval time.Duration, wake <-chan struct{}, step func(context.Context) error) {
	if interval <= 0 {
		interval = MinPollInterval
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	pollLoop(ctx, name, ticker.C, wake, step)
}

// PollTicks is Poll driven by a caller-supplied channel, so a test can step a
// worker deterministically instead of waiting on wall-clock ticks.
func PollTicks(ctx context.Context, name string, ticks <-chan time.Time, step func(context.Context) error) {
	pollLoop(ctx, name, ticks, nil, step)
}

// pollLoop is the one loop body. A nil wake blocks forever in the select, which
// is exactly the behavior a caller with no wake channel wants.
func pollLoop(ctx context.Context, name string, ticks <-chan time.Time, wake <-chan struct{}, step func(context.Context) error) {
	for {
		// Checked before the step, not just in the select below: when a tick and
		// a cancellation are both ready select picks at random, so without this a
		// canceled worker could run one more full pass against a dead context.
		if ctx.Err() != nil {
			return
		}
		if err := step(ctx); err != nil && ctx.Err() == nil {
			log.Printf("%s: %v", name, err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticks:
		case <-wake:
		}
	}
}
