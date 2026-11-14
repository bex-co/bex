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

package agentsessions

import (
	"context"
	"slices"
	"testing"
	"time"
)

// ADR047 D12: a reorder never trades places with the message the
// dispatcher is starting.

// A leased head is the message being started. "Send now" (a move to the front)
// would show the sender's message first while the head starts first, so the
// move is refused and nothing changes; a move that leaves the head alone is not.
func TestAMoveNeverTradesPlacesWithTheMessageBeingStarted(t *testing.T) {
	e := newQueueEnv(t)
	for _, prompt := range []string{"head", "two", "three"} {
		e.send(t, "alice", prompt)
	}
	e.settle(t, e.id)
	list, err := e.svc.QueuedMessages(caller("alice"), e.id)
	if err != nil || len(list.QueuedMessages) != 3 {
		t.Fatalf("queue = %+v, %v", list, err)
	}
	head, three := list.QueuedMessages[0], list.QueuedMessages[2]
	e.claimHead(t, time.Now()) // the dispatcher holds the head's lease now

	if _, err := e.svc.MoveQueuedMessage(caller("alice"), e.id, three.ID, 1); !isCode(err, codeQueuedMessageGone) {
		t.Fatalf("a move ahead of the head being started = %v, want %s", err, codeQueuedMessageGone)
	}
	if _, err := e.svc.MoveQueuedMessage(caller("alice"), e.id, head.ID, 3); !isCode(err, codeQueuedMessageGone) {
		t.Fatalf("a move of the head being started = %v, want %s", err, codeQueuedMessageGone)
	}
	if got := e.pending(t); !slices.Equal(got, []string{"head", "two", "three"}) {
		t.Fatalf("a refused move changed the queue: %v", got)
	}

	// Behind the head, the order is still the sender's to change.
	moved, err := e.svc.MoveQueuedMessage(caller("alice"), e.id, three.ID, 2)
	if err != nil || len(moved.QueuedMessages) != 3 || moved.QueuedMessages[0].ID != head.ID || moved.QueuedMessages[1].ID != three.ID {
		t.Fatalf("a move behind the head = %+v, %v", moved, err)
	}

	// A lease that lapsed holds nobody: the head may be passed again.
	if _, err := e.pg.Pool.Exec(context.Background(), `UPDATE agent_session_queued_messages SET claimed_until = now() - interval '1 second' WHERE id=$1`, head.ID); err != nil {
		t.Fatal(err)
	}
	if moved, err := e.svc.MoveQueuedMessage(caller("alice"), e.id, three.ID, 1); err != nil || moved.QueuedMessages[0].ID != three.ID {
		t.Fatalf("a move after the lease lapsed = %+v, %v", moved, err)
	}
}
