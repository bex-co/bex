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
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The races of ADR047 D12 the queue must win, against the real store: a message
// that reaches the queue as the turn settles, a replica that sleeps through its
// lease, the cap under concurrent sends, an edit against a dispatch, and a
// session that ends after its head was claimed.

var settledPhases = []string{PhaseCompleted, PhaseFailed}

// claimHead leases the head of the session's queue as one dispatcher replica
// does and returns the claim, as that replica would hold it.
func (e queueEnv) claimHead(t *testing.T, at time.Time) store.QueuedMessageClaim {
	t.Helper()
	claims, err := e.pg.ClaimQueuedAgentMessages(context.Background(), at, queueLease, queueBatch, settledPhases, queueBlockedBackoff)
	if err != nil {
		t.Fatal(err)
	}
	for _, claim := range claims {
		if claim.Session.ID == e.id {
			return claim
		}
	}
	t.Fatalf("no claim for %s among %d", e.id, len(claims))
	return store.QueuedMessageClaim{}
}

// A sender who read the session as running and queued behind that turn after
// it settled is not stranded: the dispatcher finds a settled session with a
// waiting head by polling, so no wake-up can be lost, and it runs once.
func TestAMessageQueuedAsTheTurnSettlesIsDispatchedOnce(t *testing.T) {
	e := newQueueEnv(t)
	ctx := context.Background()
	stale, err := e.pg.GetAgentSession(ctx, e.id) // what the sender read: running
	if err != nil {
		t.Fatal(err)
	}
	e.settle(t, e.id) // the turn settles before the message reaches the queue
	view, err := e.svc.enqueueSteer(caller("alice"), stale, SteerRequest{SessionID: e.id, Prompt: "lands as it settles"})
	if err != nil || view.Queued == nil {
		t.Fatalf("enqueue = %+v, %v", view, err)
	}

	d := e.dispatcher()
	if err := d.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "lands as it settles" {
		t.Fatalf("turns = %v, want the message started by the next pass", got)
	}
	// Once that turn settles too, later passes never run the message again.
	e.settle(t, e.id)
	for range 2 {
		if err := d.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}
	if got := e.prompts(t, e.id); len(got) != 2 {
		t.Fatalf("the message ran again: %v", got)
	}
	if len(e.pending(t)) != 0 {
		t.Fatalf("a delivered message is still queued: %v", e.pending(t))
	}
}

// A replica that claimed the head and stalled past its lease (a restart, a long
// pause) is replaced by another, which delivers; when the first wakes up and
// delivers its stale claim, the message does not run a second time.
func TestAReplicaThatSleptThroughItsLeaseDoesNotRunTheMessageAgain(t *testing.T) {
	e := newQueueEnv(t)
	ctx := context.Background()
	e.send(t, "alice", "orphaned")
	e.settle(t, e.id)
	now := time.Now()
	stale := e.claimHead(t, now)

	second := e.dispatcher()
	second.Now = func() time.Time { return now.Add(queueLease + time.Second) }
	if err := second.RunOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || got[1] != "orphaned" {
		t.Fatalf("the second replica did not deliver: %v", got)
	}
	e.settle(t, e.id) // the turn it started is over, so a second start would be possible
	e.dispatcher().deliver(ctx, stale)
	if got := e.prompts(t, e.id); len(got) != 2 {
		t.Fatalf("a stale claim ran the message again: %v", got)
	}
	if got := e.phase(t, e.id); got != PhaseCompleted {
		t.Fatalf("a stale claim moved the session to %s", got)
	}
}

// Sends racing past the cap each get an answer: exactly the cap is queued and
// the rest are refused as full, never more queued than the cap allows.
func TestConcurrentSendsNeverQueueMoreThanTheCap(t *testing.T) {
	e := newQueueEnv(t)
	const senders = 2 * maxQueuedMessages
	errs := make([]error, senders)
	var wg sync.WaitGroup
	for i := range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: fmt.Sprintf("send %d", i)})
		}()
	}
	wg.Wait()
	queued, full := 0, 0
	for i, err := range errs {
		switch {
		case err == nil:
			queued++
		case isCode(err, codeQueueFull):
			full++
		default:
			t.Fatalf("send %d: %v", i, err)
		}
	}
	if queued != maxQueuedMessages || full != senders-maxQueuedMessages {
		t.Fatalf("%d queued and %d refused as full, want %d and %d", queued, full, maxQueuedMessages, senders-maxQueuedMessages)
	}
	if got := len(e.pending(t)); got != maxQueuedMessages {
		t.Fatalf("pending = %d, want the cap %d", got, maxQueuedMessages)
	}
}

// An edit that races a dispatch never loses: either the edit is saved and the
// turn runs the saved text, or the dispatch won and the edit is refused as
// dispatched, with the turn running what was queued. A pass that finds the
// session locked by the edit skips it, so one more pass follows the race.
func TestAnEditRacingADispatchRunsWhatTheUserLastSaved(t *testing.T) {
	base := newQueueEnv(t)
	editWon, dispatchWon := 0, 0
	for round := range 12 {
		e := queueEnv{lineageEnv: base.lineageEnv, roles: base.roles, id: base.create(t, "").ID}
		msg := e.send(t, "alice", "original").Queued
		e.settle(t, e.id)

		start := make(chan struct{})
		var wg sync.WaitGroup
		var editErr, runErr error
		wg.Add(2)
		go func() {
			defer wg.Done()
			<-start
			_, editErr = e.svc.EditQueuedMessage(caller("bob"), e.id, msg.ID, "edited")
		}()
		go func() {
			defer wg.Done()
			<-start
			runErr = e.dispatcher().RunOnce(context.Background())
		}()
		close(start)
		wg.Wait()
		if runErr == nil {
			runErr = e.dispatcher().RunOnce(context.Background())
		}
		if runErr != nil {
			t.Fatalf("round %d: dispatch: %v", round, runErr)
		}
		got := e.prompts(t, e.id)
		if len(got) != 2 {
			t.Fatalf("round %d: turns = %v, want the message dispatched once", round, got)
		}
		switch {
		case editErr == nil && got[1] == "edited":
			editWon++
		case isCode(editErr, codeQueuedMessageGone) && got[1] == "original":
			dispatchWon++
		default:
			t.Fatalf("round %d: the edit answered %v and the turn ran %q: a saved edit must be the text that runs", round, editErr, got[1])
		}
	}
	t.Logf("edit won %d rounds, dispatch won %d", editWon, dispatchWon)
}

// A head whose session was canceled or archived after the claim starts no
// turn: the cancel or archive cleared the queue, and the stale claim finds the
// session closed.
func TestAClaimedMessageDoesNotRunOnceItsSessionEnded(t *testing.T) {
	for name, end := range map[string]func(e queueEnv) error{
		"canceled": func(e queueEnv) error { _, err := e.svc.Cancel(caller("alice"), e.id); return err },
		"archived": func(e queueEnv) error { _, err := e.svc.Archive(caller("alice"), e.id); return err },
	} {
		t.Run(name, func(t *testing.T) {
			e := newQueueEnv(t)
			msg := e.send(t, "alice", "claimed, then too late").Queued
			e.settle(t, e.id)
			claim := e.claimHead(t, time.Now())
			if err := end(e); err != nil {
				t.Fatal(err)
			}
			e.dispatcher().deliver(context.Background(), claim)
			if got := e.prompts(t, e.id); len(got) != 1 {
				t.Fatalf("a turn started on a %s session: %v", name, got)
			}
			state, err := e.pg.QueuedAgentMessageState(context.Background(), e.id, msg.ID)
			if err != nil || state.DeliveredAt != nil || state.DroppedReason == "" {
				t.Fatalf("message = %+v, %v; want it dropped by the %s, never delivered", state, err, name)
			}
		})
	}
}

// Messages dispatch strictly in their queue order, across moves and deletes.
func TestDispatchFollowsTheQueueOrderAfterMovesAndDeletes(t *testing.T) {
	e := newQueueEnv(t)
	ctx := context.Background()
	a := e.send(t, "alice", "a").Queued
	b := e.send(t, "alice", "b").Queued
	c := e.send(t, "alice", "c").Queued
	e.send(t, "alice", "d")
	if _, err := e.svc.MoveQueuedMessage(caller("alice"), e.id, c.ID, 1); err != nil { // c a b d
		t.Fatal(err)
	}
	if err := e.svc.DeleteQueuedMessage(caller("alice"), e.id, b.ID); err != nil { // c a d
		t.Fatal(err)
	}
	if _, err := e.svc.MoveQueuedMessage(caller("alice"), e.id, a.ID, 99); err != nil { // c d a
		t.Fatal(err)
	}
	e.settle(t, e.id)
	d := e.dispatcher()
	for range 3 {
		if err := d.RunOnce(ctx); err != nil {
			t.Fatal(err)
		}
		e.settle(t, e.id)
	}
	if got := e.prompts(t, e.id)[1:]; !slices.Equal(got, []string{"c", "d", "a"}) {
		t.Fatalf("dispatch order = %v, want c d a", got)
	}
}
