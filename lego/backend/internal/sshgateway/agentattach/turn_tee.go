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
	"encoding/json"
	"log"
	"sync"

	"github.com/bex-co/bex/lego/backend/internal/agenttranscript"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// maxTeePendingBytes bounds the in-flight (not yet completed) items one live
// tee holds in memory. It matches the driver's whole-session log cap, so only a
// pathological turn reaches it; the tee then stops and the Completer harvest
// persists the turn from the driver log.
const maxTeePendingBytes = 16 << 20

// replayedTurn records which rows of the current turn this client already
// received from the durable replay, so the live splice forwards each item once.
type replayedTurn struct {
	indexes  map[int64]bool
	maxIndex int64
	// legacy: a replayed row was a pre-item raw chunk (its index is a chunk
	// ordinal), so the old ordinal-prefix rule decides what to suppress.
	legacy bool
}

func newReplayedTurn() *replayedTurn {
	return &replayedTurn{indexes: map[int64]bool{}, maxIndex: -1}
}

func (r *replayedTurn) add(part store.AgentSessionTranscriptPart) {
	r.indexes[part.PartIndex] = true
	r.maxIndex = max(r.maxIndex, part.PartIndex)
	r.legacy = r.legacy || (!part.Item && !standaloneChunk(part.Part))
}

// standaloneChunk reports a legacy row that is also a valid item (one chunk
// that coalescing stores as-is), so a turn that holds only such rows is not
// treated as legacy.
func standaloneChunk(part json.RawMessage) bool {
	switch agenttranscript.ChunkType(part) {
	case "start", "finish", "error":
		return true
	}
	return false
}

// suppress reports whether the chunk at ordinal, belonging to the item at
// owner (-1: never stored), was already delivered by the durable replay.
func (r *replayedTurn) suppress(ordinal, owner int64) bool {
	if r.legacy || owner < 0 {
		return ordinal <= r.maxIndex
	}
	return r.indexes[owner]
}

// turnTee folds one live turn's chunks into items (agenttranscript, the same
// coalescer as the Completer harvest) and persists each item once it is final,
// in index order, through the bounded batcher. Stream end — normal, quota stop
// or disconnect — flushes in-flight items as partial rows; the harvest replaces
// them with their complete copies.
type turnTee struct {
	ctx       context.Context
	batcher   *transcriptBatcher
	coalescer *agenttranscript.Coalescer
	sessionID string
	turn      int
	total     int64 // stored transcript bytes of the session, cumulative
	quota     int64
	replayed  *replayedTurn
	overflow  bool // the in-flight bound stopped the tee
	closeOnce sync.Once
}

func newTurnTee(ctx context.Context, st Store, sessionID string, turn int, storedBytes, quota int64, replayed *replayedTurn) *turnTee {
	return &turnTee{
		ctx:       ctx,
		batcher:   newTranscriptBatcher(ctx, st, sessionID),
		coalescer: agenttranscript.NewCoalescer(),
		sessionID: sessionID,
		turn:      turn,
		total:     storedBytes,
		quota:     quota,
		replayed:  replayed,
	}
}

// push folds one chunk in. forward reports whether the client still needs it;
// ok is false once the session quota (or the in-flight bound) is reached, and
// the caller stops the stream without forwarding the chunk.
func (t *turnTee) push(ordinal int64, payload string) (forward, ok bool) {
	owner, released := t.coalescer.Push(ordinal, json.RawMessage(payload))
	if !t.persist(t.ctx, released) {
		return false, false
	}
	if t.coalescer.PendingBytes() > maxTeePendingBytes {
		log.Printf("agent attach: in-flight transcript items exceed %d bytes, leaving the turn to the harvest (session=%s)", maxTeePendingBytes, t.sessionID)
		t.overflow = true
		return false, false
	}
	return !t.replayed.suppress(ordinal, owner), true
}

func (t *turnTee) persist(ctx context.Context, items []agenttranscript.Item) bool {
	for _, it := range items {
		if t.replayed.indexes[it.Index] {
			continue // durable already (and charged by the replay)
		}
		size := int64(len(it.Part))
		if t.total+size > t.quota {
			return false
		}
		t.total += size
		t.batcher.enqueue(ctx, store.AgentSessionTranscriptPart{
			PartIndex: it.Index, Turn: t.turn, Part: it.Part, Item: true, Partial: it.Partial,
		})
	}
	return true
}

// close flushes in-flight items as partial rows (unless the in-flight bound
// stopped the tee), then waits for the batcher. The flush outlives a canceled
// request context with the batcher's bounded append timeout.
func (t *turnTee) close() {
	t.closeOnce.Do(func() {
		pending := t.coalescer.Flush()
		if !t.overflow {
			ctx, cancel := context.WithTimeout(context.WithoutCancel(t.ctx), transcriptAppendTimeout)
			t.persist(ctx, pending)
			cancel()
		}
		t.batcher.close()
	})
}
