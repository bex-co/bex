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

// ADR047 D12: a turn runs the text last saved, and a blocked
// message is not retried on every poll.

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// An edit saved after a lapsed lease, before the turn begins, is what the turn
// runs: the begin refuses a message that changed since its claim, the
// dispatcher releases it, and the next pass starts it as it now reads.
type editBeforeBegin struct {
	QueueStore
	pg   *store.PGStore
	done atomic.Bool
	text string
	as   string
}

func (b *editBeforeBegin) BeginQueuedAgentSessionTurn(ctx context.Context, messageID, author, id, prompt, deliveryMode, phase, status string, attachments store.AgentAttachmentBinding, output store.AgentTurnOutput, secretNames []string, skills []store.TurnSkill) (store.AgentSession, error) {
	if b.done.CompareAndSwap(false, true) {
		// The lease lapsed while Steer was working, and a member edited the message.
		if _, err := b.pg.Pool.Exec(ctx, `UPDATE agent_session_queued_messages SET claimed_until = now() - interval '1 second' WHERE id=$1`, messageID); err != nil {
			return store.AgentSession{}, err
		}
		if _, err := b.pg.EditQueuedAgentMessage(ctx, id, messageID, b.text, b.as); err != nil {
			return store.AgentSession{}, err
		}
	}
	return b.QueueStore.BeginQueuedAgentSessionTurn(ctx, messageID, author, id, prompt, deliveryMode, phase, status, attachments, output, secretNames, skills)
}

func TestAnEditAfterTheClaimIsNotLost(t *testing.T) {
	e := newQueueEnv(t)
	msg := e.send(t, "alice", "original").Queued
	e.settle(t, e.id)
	e.svc.Queue = &editBeforeBegin{QueueStore: e.pg, pg: e.pg, text: "edited", as: "bob"}
	d := e.dispatcher()
	ctx := context.Background()

	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 1 {
		t.Fatalf("a message that changed after its claim started a turn: %v", got)
	}
	state, err := e.pg.QueuedAgentMessageState(ctx, e.id, msg.ID)
	if err != nil || state.DeliveredAt != nil || state.DroppedReason != "" || state.Prompt != "edited" || state.Author != "bob" || state.BlockedReason != "" {
		t.Fatalf("the message after the refused begin = %+v, %v; want it pending as edited, with no blocked reason", state, err)
	}
	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "edited" {
		t.Fatalf("turns = %v, want the edited text to run", got)
	}
	if state, err := e.pg.QueuedAgentMessageState(ctx, e.id, msg.ID); err != nil || state.DeliveredTurn == nil || *state.DeliveredTurn != 2 {
		t.Fatalf("the message after the next pass = %+v, %v", state, err)
	}
}

// countingSteer counts the steers the dispatcher runs.
type countingSteer struct {
	QueueSessions
	n *atomic.Int32
}

func (c countingSteer) Steer(ctx context.Context, req SteerRequest) (View, error) {
	c.n.Add(1)
	return c.QueueSessions.Steer(ctx, req)
}

// A head blocked by a limit is retried about twice a minute, not on every 5 s
// pass, so a workspace at its cap does not run a full steer per poll.
func TestABlockedHeadIsRetriedAtMostEveryHalfMinute(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "waits for capacity")
	e.settle(t, e.id)
	blocker := e.create(t, "") // holds the workspace's one live sandbox slot
	e.svc.MaxLiveSandboxes = 1
	var steers atomic.Int32
	now := time.Now()
	d := &MessageQueue{Sessions: countingSteer{e.svc, &steers}, Store: e.pg, Now: func() time.Time { return now }}
	ctx := context.Background()
	pass := func(after time.Duration) {
		t.Helper()
		now = now.Add(after)
		if err := d.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}

	pass(0)
	if steers.Load() != 1 {
		t.Fatalf("the first pass ran %d steers, want 1", steers.Load())
	}
	// Polls at 5 s, 10 s and 29 s after the first attempt.
	for _, wait := range []time.Duration{queuePollInterval, queuePollInterval, queueBlockedBackoff - 2*queuePollInterval - time.Second} {
		pass(wait)
	}
	if steers.Load() != 1 {
		t.Fatalf("a blocked head ran %d steers within %s of its last attempt, want 1", steers.Load(), queueBlockedBackoff)
	}
	pass(2 * time.Second) // past the backoff: tried again, still blocked
	if steers.Load() != 2 {
		t.Fatalf("a blocked head was not retried after the backoff: %d steers", steers.Load())
	}
	// Capacity frees up; the head runs at the next retry, not before.
	e.settle(t, blocker.ID)
	pass(queuePollInterval)
	if got := e.prompts(t, e.id); len(got) != 1 || steers.Load() != 2 {
		t.Fatalf("a retry inside the backoff: turns %v, %d steers", got, steers.Load())
	}
	pass(queueBlockedBackoff)
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "waits for capacity" || steers.Load() != 3 {
		t.Fatalf("after the backoff: turns %v, %d steers", got, steers.Load())
	}
}
