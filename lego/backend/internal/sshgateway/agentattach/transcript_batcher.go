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

package agentattach

import (
	"context"
	"log"
	"sync"
	"time"

	"golang.org/x/sync/semaphore"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	defaultTranscriptBatchSize     = 32
	defaultTranscriptFlushInterval = 100 * time.Millisecond
	transcriptBatchBytes           = 1 << 20
	transcriptPendingBytes         = 4 << 20
	transcriptAppendTimeout        = 5 * time.Second
)

// One stream owns enqueue and close. A single writer preserves append order;
// both queue length and total queued/in-flight bytes are bounded. Slow storage
// backpressures a full queue, never the first browser frame. After a store error
// the live stream continues, leaving the entire missing suffix to ADR051 harvest
// rather than allocating later sequence numbers ahead of a failed batch.
type transcriptBatcher struct {
	ctx       context.Context
	queue     chan store.AgentSessionTranscriptPart
	budget    *semaphore.Weighted
	done      chan struct{}
	closeOnce sync.Once
}

func newTranscriptBatcher(ctx context.Context, st Store, sessionID string) *transcriptBatcher {
	b := &transcriptBatcher{
		ctx:    ctx,
		queue:  make(chan store.AgentSessionTranscriptPart, defaultTranscriptBatchSize),
		budget: semaphore.NewWeighted(transcriptPendingBytes),
		done:   make(chan struct{}),
	}
	go b.run(st, sessionID)
	return b
}

// enqueue queues one row, waiting on ctx for queue/byte budget. The stream
// passes its request context; the end-of-stream flush passes a bounded
// detached one so a disconnect still persists its in-flight items. A row
// larger than the whole byte budget is left to the Completer harvest.
func (b *transcriptBatcher) enqueue(ctx context.Context, part store.AgentSessionTranscriptPart) {
	size := int64(len(part.Part))
	if size > transcriptPendingBytes {
		log.Printf("agent attach: transcript item of %d bytes exceeds the tee budget; deferring it to harvest", size)
		return
	}
	if err := b.budget.Acquire(ctx, size); err != nil {
		return
	}
	select {
	case b.queue <- part:
	case <-ctx.Done():
		b.budget.Release(size)
	}
}

func (b *transcriptBatcher) close() {
	b.closeOnce.Do(func() { close(b.queue) })
	<-b.done
}

func (b *transcriptBatcher) run(st Store, sessionID string) {
	defer close(b.done)
	timer := time.NewTimer(defaultTranscriptFlushInterval)
	if !timer.Stop() {
		<-timer.C
	}
	defer timer.Stop()
	var tick <-chan time.Time
	var parts []store.AgentSessionTranscriptPart
	var bytes int64
	failed := false
	flush := func() {
		if len(parts) == 0 {
			return
		}
		timer.Stop()
		tick = nil
		// The request may be canceled by a disconnect. Accepted queued parts still
		// get one bounded attempt, and every terminal path waits for this writer.
		ctx, cancel := context.WithTimeout(context.WithoutCancel(b.ctx), transcriptAppendTimeout)
		err := st.AppendAgentSessionTranscript(ctx, sessionID, parts)
		cancel()
		if err != nil {
			failed = true
			log.Printf("agent attach: transcript batch failed; deferring suffix to harvest (session=%s parts=%d): %v", sessionID, len(parts), err)
		}
		b.budget.Release(bytes)
		parts, bytes = nil, 0
	}
	for {
		select {
		case part, ok := <-b.queue:
			if !ok {
				flush()
				return
			}
			size := int64(len(part.Part))
			if failed {
				b.budget.Release(size)
				continue
			}
			if len(parts) == 0 {
				timer.Reset(defaultTranscriptFlushInterval)
				tick = timer.C
			}
			parts = append(parts, part)
			bytes += size
			if len(parts) >= defaultTranscriptBatchSize || bytes >= transcriptBatchBytes {
				flush()
			}
		case <-tick:
			flush()
		}
	}
}
