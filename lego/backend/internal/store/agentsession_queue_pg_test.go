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

package store

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"
)

// testBlockedBackoff is the dispatcher's wait before it retries a blocked head
// (agentsessions.queueBlockedBackoff), which the claim takes as an argument.
const testBlockedBackoff = 30 * time.Second

func newQueueSession(t *testing.T, s *PGStore, workspace, phase string) AgentSession {
	t.Helper()
	ctx := context.Background()
	session, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: workspace, Repo: "bex-co/app", InitialPrompt: "x", CreatedBy: "alice"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET phase=$2 WHERE id=$1`, session.ID, phase); err != nil {
		t.Fatal(err)
	}
	return session
}

func queuedPrompts(msgs []QueuedAgentMessage) []string {
	out := make([]string, 0, len(msgs))
	for _, m := range msgs {
		out = append(out, m.Prompt)
	}
	return out
}

func enqueue(t *testing.T, s *PGStore, sessionID, prompt, key string) QueuedAgentMessage {
	t.Helper()
	m, inserted, err := s.EnqueueAgentSessionMessage(context.Background(),
		QueuedAgentMessage{SessionID: sessionID, Author: "alice", Prompt: prompt, IdempotencyKey: key, RequestHash: "h:" + prompt}, 10)
	if err != nil || !inserted {
		t.Fatalf("enqueue %q = %v %v", prompt, inserted, err)
	}
	return m
}

func TestPGQueuedMessagesOrderCapAndIdempotency(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "running")

	first := enqueue(t, s, session.ID, "one", "k1")
	second := enqueue(t, s, session.ID, "two", "")
	if first.Index != 1 || second.Index != 2 || second.Position <= first.Position {
		t.Fatalf("queue order: %+v %+v", first, second)
	}
	// The same key with the same request returns the stored message and queues nothing.
	again, inserted, err := s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "one", IdempotencyKey: "k1", RequestHash: "h:one"}, 10)
	if err != nil || inserted || again.ID != first.ID {
		t.Fatalf("a retried key = %+v inserted=%v err=%v", again, inserted, err)
	}
	// The same key for a different request is refused.
	if _, _, err := s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "other", IdempotencyKey: "k1", RequestHash: "h:other"}, 10); !errors.Is(err, ErrQueuedMessageKeyReused) {
		t.Fatalf("a reused key = %v", err)
	}
	// The cap counts pending messages only.
	for i := 3; i <= 10; i++ {
		enqueue(t, s, session.ID, fmt.Sprintf("m%d", i), "")
	}
	if _, _, err := s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "eleven", RequestHash: "h:eleven"}, 10); !errors.Is(err, ErrAgentSessionQueueFull) {
		t.Fatalf("an eleventh message = %v", err)
	}
	if n, err := s.CountQueuedAgentMessages(ctx, session.ID); err != nil || n != 10 {
		t.Fatalf("count = %d %v", n, err)
	}
	if err := s.DeleteQueuedAgentMessage(ctx, session.ID, second.ID); err != nil {
		t.Fatal(err)
	}
	if _, inserted, err := s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "eleven", RequestHash: "h:eleven"}, 10); err != nil || !inserted {
		t.Fatalf("a slot freed by a delete = %v %v", inserted, err)
	}
	list, err := s.ListQueuedAgentMessages(ctx, session.ID)
	if err != nil || len(list) != 10 || list[0].Prompt != "one" || list[9].Prompt != "eleven" {
		t.Fatalf("list = %v %v", queuedPrompts(list), err)
	}
	for i, m := range list {
		if m.Index != i+1 {
			t.Fatalf("index of %q = %d, want %d", m.Prompt, m.Index, i+1)
		}
	}
	// Another session's queue is its own.
	other := newQueueSession(t, s, tenant.ID, "running")
	if list, err := s.ListQueuedAgentMessages(ctx, other.ID); err != nil || len(list) != 0 {
		t.Fatalf("another session's queue = %v %v", list, err)
	}
	if _, err := s.QueuedAgentMessageState(ctx, other.ID, first.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a message read through another session = %v", err)
	}
}

func TestPGQueuedMessagesEditMoveDelete(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-edit")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "running")
	a := enqueue(t, s, session.ID, "a", "")
	b := enqueue(t, s, session.ID, "b", "")
	c := enqueue(t, s, session.ID, "c", "")

	edited, err := s.EditQueuedAgentMessage(ctx, session.ID, b.ID, "b2", "bob")
	if err != nil || edited.Prompt != "b2" || edited.Author != "bob" || edited.Index != 2 {
		t.Fatalf("edit = %+v %v", edited, err)
	}
	moved, err := s.MoveQueuedAgentMessage(ctx, session.ID, c.ID, 1)
	if err != nil || !slices.Equal(queuedPrompts(moved), []string{"c", "a", "b2"}) {
		t.Fatalf("move to front = %v %v", queuedPrompts(moved), err)
	}
	moved, err = s.MoveQueuedAgentMessage(ctx, session.ID, c.ID, 99) // clamps to the end
	if err != nil || !slices.Equal(queuedPrompts(moved), []string{"a", "b2", "c"}) {
		t.Fatalf("move past the end = %v %v", queuedPrompts(moved), err)
	}
	list, _ := s.ListQueuedAgentMessages(ctx, session.ID)
	if !slices.Equal(queuedPrompts(list), []string{"a", "b2", "c"}) {
		t.Fatalf("the order did not persist: %v", queuedPrompts(list))
	}
	// A later message still queues after the moved ones.
	enqueue(t, s, session.ID, "d", "")
	list, _ = s.ListQueuedAgentMessages(ctx, session.ID)
	if !slices.Equal(queuedPrompts(list), []string{"a", "b2", "c", "d"}) {
		t.Fatalf("after a move a new message went %v", queuedPrompts(list))
	}
	if err := s.DeleteQueuedAgentMessage(ctx, session.ID, a.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueuedAgentMessageState(ctx, session.ID, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a deleted message = %v", err)
	}
	if err := s.DeleteQueuedAgentMessage(ctx, session.ID, a.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting it again = %v", err)
	}
	// A message that is delivered, dropped or leased can no longer change.
	if err := s.DropQueuedAgentMessage(ctx, b.ID, "gone"); err != nil {
		t.Fatal(err)
	}
	for name, run := range map[string]func() error{
		"edit":   func() error { _, err := s.EditQueuedAgentMessage(ctx, session.ID, b.ID, "x", "bob"); return err },
		"move":   func() error { _, err := s.MoveQueuedAgentMessage(ctx, session.ID, b.ID, 1); return err },
		"delete": func() error { return s.DeleteQueuedAgentMessage(ctx, session.ID, b.ID) },
	} {
		if err := run(); !errors.Is(err, ErrQueuedMessageNotPending) {
			t.Fatalf("%s of a dropped message = %v", name, err)
		}
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_session_queued_messages SET claimed_until=now() + interval '1 minute' WHERE id=$1`, c.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditQueuedAgentMessage(ctx, session.ID, c.ID, "x", "bob"); !errors.Is(err, ErrQueuedMessageNotPending) {
		t.Fatalf("edit of a message being dispatched = %v", err)
	}
	if err := s.DeleteQueuedAgentMessage(ctx, session.ID, c.ID); !errors.Is(err, ErrQueuedMessageNotPending) {
		t.Fatalf("delete of a message being dispatched = %v", err)
	}
	if _, err := s.EditQueuedAgentMessage(ctx, session.ID, "aqm-nope", "x", "bob"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("edit of an unknown message = %v", err)
	}
}

func TestPGQueuedMessagesClaimHeadOnceAndRecoverLease(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-claim")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "running")
	enqueue(t, s, session.ID, "one", "")
	enqueue(t, s, session.ID, "two", "")
	now := time.Now()
	phases := []string{"completed", "failed"}
	claimMine := func(at time.Time) []QueuedMessageClaim {
		t.Helper()
		all, err := s.ClaimQueuedAgentMessages(ctx, at, time.Minute, 200, phases, testBlockedBackoff)
		if err != nil {
			t.Fatal(err)
		}
		var mine []QueuedMessageClaim
		for _, c := range all {
			if c.Session.ID == session.ID {
				mine = append(mine, c)
			}
		}
		return mine
	}
	// A running session is not claimed; once it settles the head is, alone.
	if got := claimMine(now); len(got) != 0 {
		t.Fatalf("a running session's queue was claimed: %+v", got)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, session.ID); err != nil {
		t.Fatal(err)
	}
	got := claimMine(now)
	if len(got) != 1 || got[0].Message.Prompt != "one" {
		t.Fatalf("claim = %+v", got)
	}
	// Nobody else claims it, and the message behind the head is not offered.
	if again := claimMine(now); len(again) != 0 {
		t.Fatalf("a leased head was claimed twice: %+v", again)
	}
	// A crashed holder's lease runs out and the same head is claimed again.
	if after := claimMine(now.Add(2 * time.Minute)); len(after) != 1 || after[0].Message.Prompt != "one" {
		t.Fatalf("an expired lease was not reclaimed: %+v", after)
	}
	// A release that only lost a race returns the head at once.
	if err := s.ReleaseQueuedAgentMessage(ctx, got[0].Message.ID, ""); err != nil {
		t.Fatal(err)
	}
	if released := claimMine(now.Add(2*time.Minute + time.Second)); len(released) != 1 || released[0].Message.BlockedReason != "" {
		t.Fatalf("after a release with no reason = %+v", released)
	}
	// One that records why the attempt was blocked holds the head back (below).
	if err := s.ReleaseQueuedAgentMessage(ctx, got[0].Message.ID, "spend ceiling reached"); err != nil {
		t.Fatal(err)
	}
	released := claimMine(now.Add(3 * time.Minute))
	if len(released) != 1 || released[0].Message.BlockedReason != "spend ceiling reached" {
		t.Fatalf("after a blocked release, past the backoff = %+v", released)
	}
	// An archived or canceled session is never claimed.
	if _, err := s.SetAgentSessionArchived(ctx, session.ID, true); err != nil {
		t.Fatal(err)
	}
	if got := claimMine(now.Add(time.Hour)); len(got) != 0 {
		t.Fatalf("an archived session's queue was claimed: %+v", got)
	}
}

func TestPGQueuedMessageStartsOneTurnWhoeverRaces(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-begin")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "completed")
	msg := enqueue(t, s, session.ID, "next", "")

	var wg sync.WaitGroup
	results := make([]error, 4)
	for i := range results {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = s.BeginQueuedAgentSessionTurn(ctx, msg.ID, "alice", session.ID, "next", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil)
		}()
	}
	wg.Wait()
	started := 0
	for _, err := range results {
		switch {
		case err == nil:
			started++
		case !errors.Is(err, ErrAgentSessionTurnState) && !errors.Is(err, ErrQueuedMessageNotPending):
			t.Fatalf("a racing begin = %v", err)
		}
	}
	if started != 1 {
		t.Fatalf("%d turns started for one message: %v", started, results)
	}
	row, err := s.GetAgentSession(ctx, session.ID)
	if err != nil || row.Turns != 2 || row.Phase != "redispatching" {
		t.Fatalf("session = %+v %v", row, err)
	}
	state, err := s.QueuedAgentMessageState(ctx, session.ID, msg.ID)
	if err != nil || state.DeliveredTurn == nil || *state.DeliveredTurn != 2 || state.DeliveredAt == nil {
		t.Fatalf("message = %+v %v", state, err)
	}
	if n, _ := s.CountQueuedAgentMessages(ctx, session.ID); n != 0 {
		t.Fatalf("a delivered message is still pending: %d", n)
	}
	// A dropped message starts no turn and leaves the session as it was.
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, session.ID); err != nil {
		t.Fatal(err)
	}
	gone := enqueue(t, s, session.ID, "gone", "")
	if err := s.DropQueuedAgentMessage(ctx, gone.ID, "member left"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginQueuedAgentSessionTurn(ctx, gone.ID, "alice", session.ID, "gone", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil); !errors.Is(err, ErrQueuedMessageNotPending) {
		t.Fatalf("a dropped message started a turn: %v", err)
	}
	if row, _ := s.GetAgentSession(ctx, session.ID); row.Turns != 2 || row.Phase != "completed" {
		t.Fatalf("a refused begin changed the session: %+v", row)
	}
}

// A session that ends between the dispatcher's read and its begin (a cancel, an
// archive) starts no turn: the begin rechecks under the row lock, and the
// message it was to deliver stays undelivered.
func TestPGQueuedMessageStartsNoTurnOnASessionThatEndedAfterTheClaim(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-ended")
	ctx := context.Background()
	for name, end := range map[string]string{
		"canceled": `UPDATE agent_sessions SET phase='canceled', canceled_at=now() WHERE id=$1`,
		"archived": `UPDATE agent_sessions SET archived_at=now() WHERE id=$1`,
	} {
		session := newQueueSession(t, s, tenant.ID, "completed")
		msg := enqueue(t, s, session.ID, "too late", "")
		if _, err := s.Pool.Exec(ctx, end, session.ID); err != nil {
			t.Fatal(err)
		}
		_, err := s.BeginQueuedAgentSessionTurn(ctx, msg.ID, "alice", session.ID, "too late", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil)
		if !errors.Is(err, ErrAgentSessionTurnState) {
			t.Fatalf("%s: begin = %v, want ErrAgentSessionTurnState", name, err)
		}
		row, err := s.GetAgentSession(ctx, session.ID)
		if err != nil || row.Turns != 1 {
			t.Fatalf("%s: a refused begin changed the session: %+v %v", name, row, err)
		}
		state, err := s.QueuedAgentMessageState(ctx, session.ID, msg.ID)
		if err != nil || state.DeliveredAt != nil {
			t.Fatalf("%s: the message was recorded delivered: %+v %v", name, state, err)
		}
	}
}

func TestPGQueuedMessagesClearedWithTheSession(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-clear")
	ctx := context.Background()
	for name, end := range map[string]func(AgentSession) error{
		"canceled": func(a AgentSession) error { _, _, err := s.SetAgentSessionCanceled(ctx, a.ID); return err },
		"canceling": func(a AgentSession) error {
			_, err := s.SetAgentSessionLifecycle(ctx, a.ID, "", "canceling", "canceling", true)
			return err
		},
		"archived": func(a AgentSession) error { _, err := s.SetAgentSessionArchived(ctx, a.ID, true); return err },
		"deleted":  func(a AgentSession) error { return s.DeleteAgentSession(ctx, a.ID) },
	} {
		t.Run(name, func(t *testing.T) {
			session := newQueueSession(t, s, tenant.ID, "running")
			msg := enqueue(t, s, session.ID, "pending", "")
			if err := end(session); err != nil {
				t.Fatal(err)
			}
			if n, err := s.CountQueuedAgentMessages(ctx, session.ID); err != nil || n != 0 {
				t.Fatalf("pending after %s = %d %v", name, n, err)
			}
			state, err := s.QueuedAgentMessageState(ctx, session.ID, msg.ID)
			if name == "deleted" {
				if !errors.Is(err, ErrNotFound) {
					t.Fatalf("a deleted session kept its message: %+v %v", state, err)
				}
				return
			}
			if err != nil || state.DroppedReason == "" {
				t.Fatalf("message after %s = %+v %v", name, state, err)
			}
			// Nothing more queues on a session that ended its queue (a deleted
			// session has no row to queue on).
			if _, _, err := s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "late", RequestHash: "h"}, 10); !errors.Is(err, ErrAgentSessionNotSteerable) {
				t.Fatalf("a message queued after %s = %v", name, err)
			}
		})
	}
}

func TestPGFeedbackYieldsToQueuedMessagesAndTurnsRecordInterruption(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-feedback")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "completed")
	if _, err := s.QueueAgentSessionFeedback(ctx, AgentSessionFeedback{SessionID: session.ID, SourceKey: "comment:1", Kind: FeedbackPRComment, Actor: "alice", Body: "> hi"}); err != nil {
		t.Fatal(err)
	}
	claim := func() int {
		t.Helper()
		batches, err := s.ClaimDeliverableFeedback(ctx, time.Now(), time.Minute, 200, []string{"completed", "failed"})
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, b := range batches {
			if b.Session.ID == session.ID {
				n++
				if err := s.ReleaseFeedback(ctx, session.ID, []string{"comment:1"}); err != nil {
					t.Fatal(err)
				}
			}
		}
		return n
	}
	msg := enqueue(t, s, session.ID, "first", "")
	if n := claim(); n != 0 {
		t.Fatalf("feedback was claimed ahead of a queued message")
	}
	if err := s.DeleteQueuedAgentMessage(ctx, session.ID, msg.ID); err != nil {
		t.Fatal(err)
	}
	if n := claim(); n != 1 {
		t.Fatalf("feedback was not claimed once the queue drained")
	}

	// A turn records that it was interrupted.
	if _, err := s.BeginAgentSessionTurn(ctx, session.ID, "go", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkAgentSessionTurnInterrupted(ctx, session.ID, 2); err != nil {
		t.Fatal(err)
	}
	turns, err := s.AgentSessionTurns(ctx, session.ID)
	if err != nil || len(turns) != 2 || turns[0].Interrupted || !turns[1].Interrupted {
		t.Fatalf("turns = %+v %v", turns, err)
	}
}

// A session whose head keeps being refused does not hold every claim slot
// against the others: the one tried longest ago goes first.
func TestPGQueuedMessagesClaimTheSessionTriedLongestAgoFirst(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-fair")
	ctx := context.Background()
	stuck := newQueueSession(t, s, tenant.ID, "completed")
	fresh := newQueueSession(t, s, tenant.ID, "completed")
	stuckHead := enqueue(t, s, stuck.ID, "stuck", "")
	enqueue(t, s, fresh.ID, "fresh", "")
	// stuck is the older session by updated_at and was tried a moment ago.
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET updated_at = now() - interval '1 hour' WHERE id=$1`, stuck.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_session_queued_messages SET last_attempt_at=$2 WHERE id=$1`, stuckHead.ID, now.Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	// One slot: the session never tried goes first, though stuck is older.
	got, err := s.ClaimQueuedAgentMessages(ctx, now, time.Minute, 1, []string{"completed"}, testBlockedBackoff)
	if err != nil || len(got) != 1 {
		t.Fatalf("claim = %v, %v", got, err)
	}
	if got[0].Session.ID == stuck.ID {
		t.Fatalf("the session tried a second ago was claimed before one never tried")
	}
}

// Only the sender can edit a message that carries their attachments: the
// uploads are theirs, and the turn attaches them as its author.
func TestPGQueuedMessageWithAttachmentsIsEditedBySenderOnly(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-attach")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "running")
	withFile, _, err := s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "see the file",
		Request: []byte(`{"attachmentIds":["att-1"]}`), RequestHash: "h"}, 10)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.EditQueuedAgentMessage(ctx, session.ID, withFile.ID, "reworded", "bob"); !errors.Is(err, ErrQueuedMessageAttachmentsOwned) {
		t.Fatalf("another member's edit = %v", err)
	}
	if edited, err := s.EditQueuedAgentMessage(ctx, session.ID, withFile.ID, "reworded", "alice"); err != nil || edited.Prompt != "reworded" {
		t.Fatalf("the sender's edit = %+v, %v", edited, err)
	}
	plain := enqueue(t, s, session.ID, "plain", "")
	if edited, err := s.EditQueuedAgentMessage(ctx, session.ID, plain.ID, "by bob", "bob"); err != nil || edited.Author != "bob" {
		t.Fatalf("an edit of a message with no attachments = %+v, %v", edited, err)
	}
}

// beginKeyed starts a turn of the session for a direct send under key.
func beginKeyed(s *PGStore, sessionID, prompt, key, hash string) error {
	_, err := s.BeginKeyedAgentSessionTurn(context.Background(), key, hash, sessionID, prompt, "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil)
	return err
}

func settleSession(t *testing.T, s *PGStore, sessionID string) {
	t.Helper()
	if _, err := s.Pool.Exec(context.Background(), `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, sessionID); err != nil {
		t.Fatal(err)
	}
}

// ADR047 D12: a turn a direct send starts records the send's key and
// request digest in the transaction that inserts it, so the key is found
// whether its first attempt started a turn or queued, and the second use of a
// key starts nothing.
func TestPGKeyedTurnRecordsItsKeyAndAKeyIsUsedOnce(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-keys")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "completed")

	if err := beginKeyed(s, session.ID, "go", "k1", "h1"); err != nil {
		t.Fatal(err)
	}
	found, err := s.FindKeyedAgentSessionSend(ctx, session.ID, "k1")
	if err != nil || found.Turn != 2 || found.RequestHash != "h1" || found.Message != nil {
		t.Fatalf("find after a keyed start = %+v, %v; want turn 2, hash h1, no message", found, err)
	}
	if _, err := s.FindKeyedAgentSessionSend(ctx, session.ID, "nope"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unused key = %v", err)
	}

	// The session is idle again: the same key starts no second turn, whatever
	// the request's digest (the caller compares it), and queues nothing.
	settleSession(t, s, session.ID)
	for _, hash := range []string{"h1", "other"} {
		if err := beginKeyed(s, session.ID, "go", "k1", hash); !errors.Is(err, ErrAgentSessionKeyUsed) {
			t.Fatalf("a second begin under the key (hash %s) = %v, want ErrAgentSessionKeyUsed", hash, err)
		}
	}
	if _, _, err := s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "go", IdempotencyKey: "k1", RequestHash: "h1"}, 10); !errors.Is(err, ErrAgentSessionKeyUsed) {
		t.Fatalf("an enqueue under a key that started a turn = %v, want ErrAgentSessionKeyUsed", err)
	}
	if _, _, err := s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "other", IdempotencyKey: "k1", RequestHash: "other"}, 10); !errors.Is(err, ErrQueuedMessageKeyReused) {
		t.Fatalf("an enqueue under a key that started a turn, for another request = %v, want ErrQueuedMessageKeyReused", err)
	}
	if n, _ := s.CountQueuedAgentMessages(ctx, session.ID); n != 0 {
		t.Fatalf("a refused enqueue queued %d messages", n)
	}
	if row, _ := s.GetAgentSession(ctx, session.ID); row.Turns != 2 {
		t.Fatalf("a refused begin started a turn: %d turns", row.Turns)
	}

	// A refused begin records no key: nothing was started under it.
	running := newQueueSession(t, s, tenant.ID, "running")
	if err := beginKeyed(s, running.ID, "go", "k-refused", "h"); !errors.Is(err, ErrAgentSessionTurnState) {
		t.Fatalf("a keyed begin on a running session = %v", err)
	}
	if _, err := s.FindKeyedAgentSessionSend(ctx, running.ID, "k-refused"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a refused begin recorded its key: %v", err)
	}

	// Keys are per session, and a turn without a key records none.
	other := newQueueSession(t, s, tenant.ID, "completed")
	if err := beginKeyed(s, other.ID, "go", "k1", "h1"); err != nil {
		t.Fatalf("the same key on another session = %v", err)
	}
	settleSession(t, s, other.ID)
	for range 2 {
		if _, err := s.BeginAgentSessionTurn(ctx, other.ID, "plain", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil); err != nil {
			t.Fatal(err)
		}
		settleSession(t, s, other.ID)
	}
	var keyed int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_session_turns WHERE session_id=$1 AND (idempotency_key IS NOT NULL OR request_hash IS NOT NULL)`, other.ID).Scan(&keyed); err != nil || keyed != 1 {
		t.Fatalf("%d turns carry a key, want only the keyed one (%v)", keyed, err)
	}
}

// A key used by a queued message is found in every state of the message, and a
// direct send under it starts nothing; the turn that delivered the message
// carries no key of its own.
func TestPGKeyedQueuedMessageIsFoundPendingDeliveredAndDropped(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-keys-msg")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "running")
	enqueue(t, s, session.ID, "ahead", "")
	keyed := enqueue(t, s, session.ID, "keyed", "k-msg")
	dropped := enqueue(t, s, session.ID, "dropped", "k-drop")

	found, err := s.FindKeyedAgentSessionSend(ctx, session.ID, "k-msg")
	if err != nil || found.Turn != 0 || found.Message == nil || found.Message.ID != keyed.ID || found.Message.Index != 2 || found.RequestHash != "h:keyed" {
		t.Fatalf("find while pending = %+v, %v; want the message at place 2", found, err)
	}
	if err := s.DropQueuedAgentMessage(ctx, dropped.ID, "gone"); err != nil {
		t.Fatal(err)
	}
	if found, err := s.FindKeyedAgentSessionSend(ctx, session.ID, "k-drop"); err != nil || found.Message == nil || found.Message.DroppedReason != "gone" {
		t.Fatalf("find after a drop = %+v, %v", found, err)
	}

	settleSession(t, s, session.ID)
	if err := beginKeyed(s, session.ID, "keyed", "k-msg", "h:keyed"); !errors.Is(err, ErrAgentSessionKeyUsed) {
		t.Fatalf("a direct send under a queued message's key = %v, want ErrAgentSessionKeyUsed", err)
	}
	for range 2 { // the head, then the keyed message, each as its own turn
		claims, err := s.ClaimQueuedAgentMessages(ctx, time.Now().Add(time.Hour), time.Minute, 200, []string{"completed"}, testBlockedBackoff)
		if err != nil {
			t.Fatal(err)
		}
		var head QueuedMessageClaim
		for _, c := range claims {
			if c.Session.ID == session.ID {
				head = c
			}
		}
		if head.Message.ID == "" {
			t.Fatal("no claim for the session")
		}
		if _, err := s.BeginQueuedAgentSessionTurn(ctx, head.Message.ID, head.Message.Author, session.ID, head.Message.Prompt, "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil); err != nil {
			t.Fatal(err)
		}
		settleSession(t, s, session.ID)
	}
	found, err = s.FindKeyedAgentSessionSend(ctx, session.ID, "k-msg")
	if err != nil || found.Message == nil || found.Message.DeliveredTurn == nil || *found.Message.DeliveredTurn != 3 || found.Message.DeliveredAt == nil {
		t.Fatalf("find after delivery = %+v, %v; want the message delivered by turn 3", found, err)
	}
	var carriers int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_session_turns WHERE session_id=$1 AND idempotency_key IS NOT NULL`, session.ID).Scan(&carriers); err != nil || carriers != 0 {
		t.Fatalf("a turn a message started carries a key: %d %v", carriers, err)
	}
}

// Sends racing under one key start one turn between them; every other attempt
// is refused as a use of the key, never a second turn and never a queued copy.
func TestPGRacingKeyedBeginsStartOneTurn(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-keys-race")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "completed")
	const senders = 8
	errs := make([]error, senders)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			if i%2 == 0 {
				errs[i] = beginKeyed(s, session.ID, "race", "k-race", "h")
				return
			}
			_, _, errs[i] = s.EnqueueAgentSessionMessage(ctx, QueuedAgentMessage{SessionID: session.ID, Author: "alice", Prompt: "race", IdempotencyKey: "k-race", RequestHash: "h"}, 10)
		}()
	}
	close(start)
	wg.Wait()
	// Whichever send got the session lock first owns the key: a begin starts
	// the turn and every enqueue then sees it; an enqueue queues the message
	// and every begin then sees that.
	var turnsStarted, queued int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_session_turns WHERE session_id=$1 AND idempotency_key='k-race'`, session.ID).Scan(&turnsStarted); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM agent_session_queued_messages WHERE session_id=$1 AND idempotency_key='k-race'`, session.ID).Scan(&queued); err != nil {
		t.Fatal(err)
	}
	if turnsStarted+queued != 1 {
		t.Fatalf("%d turns and %d messages carry the key, want exactly one of them (errors %v)", turnsStarted, queued, errs)
	}
	for i, err := range errs {
		if err != nil && !errors.Is(err, ErrAgentSessionKeyUsed) && !errors.Is(err, ErrAgentSessionTurnState) {
			t.Fatalf("attempt %d = %v", i, err)
		}
	}
}

// A request to stop a turn is recorded on that turn, keeps its first time, and
// never reaches another turn; a turn that does not exist cannot be asked.
func TestPGInterruptRequestBelongsToItsTurn(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-interrupt-request")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "completed")
	if _, err := s.BeginAgentSessionTurn(ctx, session.ID, "two", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil); err != nil {
		t.Fatal(err)
	}
	requested := func() []*time.Time {
		t.Helper()
		turns, err := s.AgentSessionTurns(ctx, session.ID)
		if err != nil || len(turns) != 2 {
			t.Fatalf("turns = %+v, %v", turns, err)
		}
		return []*time.Time{turns[0].InterruptRequestedAt, turns[1].InterruptRequestedAt}
	}
	if got := requested(); got[0] != nil || got[1] != nil {
		t.Fatalf("a turn nobody asked to stop carries a request: %v", got)
	}
	if err := s.RequestAgentSessionTurnInterrupt(ctx, session.ID, 2); err != nil {
		t.Fatal(err)
	}
	first := requested()
	if first[0] != nil || first[1] == nil {
		t.Fatalf("a request for turn 2 = %v; want it on turn 2 only", first)
	}
	if err := s.RequestAgentSessionTurnInterrupt(ctx, session.ID, 2); err != nil {
		t.Fatal(err)
	}
	if again := requested(); !again[1].Equal(*first[1]) {
		t.Fatalf("asking again moved the request: %v then %v", first[1], again[1])
	}
	if err := s.RequestAgentSessionTurnInterrupt(ctx, session.ID, 3); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a request for a turn that does not exist = %v", err)
	}
	if err := s.RequestAgentSessionTurnInterrupt(ctx, "ags-nope", 1); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a request for a session that does not exist = %v", err)
	}
}

// ADR047 D12: the head the dispatcher holds a lease on is being
// started, so a move that would give it another place is refused whole.
func TestPGMoveNeverTradesPlacesWithALeasedHead(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-move-leased")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "completed")
	head := enqueue(t, s, session.ID, "head", "")
	two := enqueue(t, s, session.ID, "two", "")
	three := enqueue(t, s, session.ID, "three", "")
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_session_queued_messages SET claimed_until = now() + interval '1 minute' WHERE id=$1`, head.ID); err != nil {
		t.Fatal(err)
	}
	positions := func() []int64 {
		t.Helper()
		list, err := s.ListQueuedAgentMessages(ctx, session.ID)
		if err != nil {
			t.Fatal(err)
		}
		out := make([]int64, len(list))
		for i, m := range list {
			out[i] = m.Position
		}
		return out
	}
	before := positions()

	// Ahead of the head, or the head itself: refused, with no position changed.
	for name, run := range map[string]func() error{
		"to the front":        func() error { _, err := s.MoveQueuedAgentMessage(ctx, session.ID, three.ID, 1); return err },
		"second to the front": func() error { _, err := s.MoveQueuedAgentMessage(ctx, session.ID, two.ID, 1); return err },
		"the head itself":     func() error { _, err := s.MoveQueuedAgentMessage(ctx, session.ID, head.ID, 3); return err },
	} {
		if err := run(); !errors.Is(err, ErrQueuedMessageNotPending) {
			t.Fatalf("a move %s = %v, want ErrQueuedMessageNotPending", name, err)
		}
		if got := positions(); !slices.Equal(got, before) {
			t.Fatalf("a refused move %s changed positions: %v then %v", name, before, got)
		}
	}
	// Behind it the others still trade places, and the head's own place stays.
	moved, err := s.MoveQueuedAgentMessage(ctx, session.ID, three.ID, 2)
	if err != nil || !slices.Equal(queuedPrompts(moved), []string{"head", "three", "two"}) {
		t.Fatalf("a move behind the head = %v, %v", queuedPrompts(moved), err)
	}
	if got := positions(); got[0] != before[0] {
		t.Fatalf("the head's position moved: %v then %v", before, got)
	}
	// Once its lease lapses it takes part again.
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_session_queued_messages SET claimed_until = now() - interval '1 second' WHERE id=$1`, head.ID); err != nil {
		t.Fatal(err)
	}
	if moved, err := s.MoveQueuedAgentMessage(ctx, session.ID, three.ID, 1); err != nil || !slices.Equal(queuedPrompts(moved), []string{"three", "head", "two"}) {
		t.Fatalf("a move after the lease lapsed = %v, %v", queuedPrompts(moved), err)
	}
}

// The begin that delivers a message runs what the dispatcher claimed only if
// that is still what the message says: an edit after a lapsed lease changes
// the text and the member it runs as, and is not lost.
func TestPGQueuedBeginRefusesAMessageEditedSinceItsClaim(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-begin-changed")
	ctx := context.Background()
	session := newQueueSession(t, s, tenant.ID, "completed")
	msg := enqueue(t, s, session.ID, "original", "")
	begin := func(author, prompt string) error {
		_, err := s.BeginQueuedAgentSessionTurn(ctx, msg.ID, author, session.ID, prompt, "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil)
		return err
	}
	check := func(what string) {
		t.Helper()
		row, err := s.GetAgentSession(ctx, session.ID)
		state, serr := s.QueuedAgentMessageState(ctx, session.ID, msg.ID)
		if err != nil || serr != nil || row.Turns != 1 || row.Phase != "completed" || state.DeliveredAt != nil {
			t.Fatalf("%s: a refused begin changed something: session %+v (%v), message %+v (%v)", what, row, err, state, serr)
		}
	}

	// The prompt changed.
	if _, err := s.EditQueuedAgentMessage(ctx, session.ID, msg.ID, "edited", "alice"); err != nil {
		t.Fatal(err)
	}
	if err := begin("alice", "original"); !errors.Is(err, ErrQueuedMessageChanged) {
		t.Fatalf("a begin with the claim's old text = %v, want ErrQueuedMessageChanged", err)
	}
	check("old text")
	// Only the author changed: the claim's authorization was for another member.
	if _, err := s.EditQueuedAgentMessage(ctx, session.ID, msg.ID, "edited", "bob"); err != nil {
		t.Fatal(err)
	}
	if err := begin("alice", "edited"); !errors.Is(err, ErrQueuedMessageChanged) {
		t.Fatalf("a begin as the claim's old author = %v, want ErrQueuedMessageChanged", err)
	}
	check("old author")
	// A begin that reads the message as it now stands starts it.
	if err := begin("bob", "edited"); err != nil {
		t.Fatalf("a begin with the current text and author = %v", err)
	}
	if row, _ := s.GetAgentSession(ctx, session.ID); row.Turns != 2 {
		t.Fatalf("turns = %d, want 2", row.Turns)
	}
	if err := begin("bob", "edited"); !errors.Is(err, ErrAgentSessionTurnState) && !errors.Is(err, ErrQueuedMessageNotPending) {
		t.Fatalf("a second begin = %v", err)
	}
}

// ADR047 D12: a head the last attempt left blocked is claimed again
// only once that attempt is the backoff old; every other head is claimed at
// once, and an edit, which clears the reason, makes a blocked head claimable.
func TestPGBlockedHeadIsClaimedAgainOnlyAfterTheBackoff(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-queue-backoff")
	ctx := context.Background()
	blocked := newQueueSession(t, s, tenant.ID, "completed")
	free := newQueueSession(t, s, tenant.ID, "completed")
	blockedHead := enqueue(t, s, blocked.ID, "blocked", "")
	freeHead := enqueue(t, s, free.ID, "free", "")
	phases := []string{"completed"}
	claimed := func(at time.Time) map[string]bool {
		t.Helper()
		all, err := s.ClaimQueuedAgentMessages(ctx, at, time.Minute, 200, phases, testBlockedBackoff)
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]bool{}
		for _, c := range all {
			out[c.Message.ID] = true
		}
		return out
	}
	release := func(id, reason string) {
		t.Helper()
		if err := s.ReleaseQueuedAgentMessage(ctx, id, reason); err != nil {
			t.Fatal(err)
		}
	}

	t0 := time.Now()
	if got := claimed(t0); !got[blockedHead.ID] || !got[freeHead.ID] {
		t.Fatalf("a first claim = %v; a head never tried is claimed", got)
	}
	release(blockedHead.ID, "live agent-session sandboxes limit reached")
	release(freeHead.ID, "") // it only lost a race

	// The free head is claimed on the next pass; the blocked one is not.
	if got := claimed(t0.Add(5 * time.Second)); got[blockedHead.ID] || !got[freeHead.ID] {
		t.Fatalf("5 s on, claimed %v; want the free head only", got)
	}
	release(freeHead.ID, "")
	if got := claimed(t0.Add(testBlockedBackoff - time.Millisecond)); got[blockedHead.ID] {
		t.Fatalf("a blocked head was claimed before its backoff ended")
	}
	release(freeHead.ID, "")
	if got := claimed(t0.Add(testBlockedBackoff + time.Millisecond)); !got[blockedHead.ID] {
		t.Fatalf("a blocked head was not claimed after its backoff: %v", got)
	}

	// Claimed again and blocked again: the backoff runs from that attempt.
	t1 := t0.Add(testBlockedBackoff + time.Millisecond)
	release(blockedHead.ID, "still blocked")
	if got := claimed(t1.Add(testBlockedBackoff - time.Second)); got[blockedHead.ID] {
		t.Fatalf("the backoff did not restart from the latest attempt: %v", got)
	}
	// An edit clears the reason, so the person's change is tried at once.
	if _, err := s.EditQueuedAgentMessage(ctx, blocked.ID, blockedHead.ID, "blocked, reworded", "alice"); err != nil {
		t.Fatal(err)
	}
	if got := claimed(t1.Add(testBlockedBackoff - time.Second)); !got[blockedHead.ID] {
		t.Fatalf("an edited head was held back: %v", got)
	}
}
