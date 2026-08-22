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

	"github.com/bex-co/bex/lego/backend/internal/store"
)

const (
	defaultTranscriptBatchSize     = 32
	defaultTranscriptFlushInterval = 100 * time.Millisecond
)

// transcriptBatcher queues accepted UI-message parts for bounded, batched
// persistence while the attach loop forwards each part to the browser first.
type transcriptBatcher struct {
	store     Store
	sessionID string
	maxParts  int
	flushWait time.Duration

	mu         sync.Mutex
	parts      []store.AgentSessionTranscriptPart
	flushTimer *time.Timer
	wg         sync.WaitGroup
	closed     bool
	ctx        context.Context
}

func newTranscriptBatcher(ctx context.Context, store Store, sessionID string) *transcriptBatcher {
	return &transcriptBatcher{
		store:     store,
		sessionID: sessionID,
		maxParts:  defaultTranscriptBatchSize,
		flushWait: defaultTranscriptFlushInterval,
		ctx:       ctx,
	}
}

func (b *transcriptBatcher) enqueue(part store.AgentSessionTranscriptPart) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.closed {
		return
	}
	b.parts = append(b.parts, part)
	if len(b.parts) >= b.maxParts {
		b.flushLocked(false)
		return
	}
	if b.flushTimer == nil {
		b.flushTimer = time.AfterFunc(b.flushWait, func() {
			b.mu.Lock()
			defer b.mu.Unlock()
			b.flushLocked(false)
		})
	}
}

func (b *transcriptBatcher) flush() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.flushLocked(true)
}

func (b *transcriptBatcher) close() {
	b.mu.Lock()
	b.closed = true
	b.flushLocked(true)
	b.mu.Unlock()
	b.wg.Wait()
}

func (b *transcriptBatcher) flushLocked(wait bool) {
	if b.flushTimer != nil {
		b.flushTimer.Stop()
		b.flushTimer = nil
	}
	if len(b.parts) == 0 {
		return
	}
	batch := b.parts
	b.parts = nil
	if wait {
		if err := b.store.AppendAgentSessionTranscript(b.ctx, b.sessionID, batch); err != nil {
			log.Printf("agent attach: transcript batch failed (session=%s parts=%d): %v", b.sessionID, len(batch), err)
		}
		return
	}
	b.wg.Add(1)
	go func() {
		defer b.wg.Done()
		if err := b.store.AppendAgentSessionTranscript(b.ctx, b.sessionID, batch); err != nil {
			log.Printf("agent attach: transcript batch failed (session=%s parts=%d): %v", b.sessionID, len(batch), err)
		}
	}()
}
