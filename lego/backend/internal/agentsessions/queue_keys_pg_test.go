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
	"sync"
	"testing"
)

// ADR047 D12: an Idempotency-Key is honored for every keyed send,
// whether its first attempt started a turn, queued, or was already dispatched.
// A client that lost the first answer and sent again never runs the message
// twice and never queues a duplicate.

func (e queueEnv) sendKeyed(t *testing.T, sessionID, prompt, key string) (View, error) {
	t.Helper()
	return e.svc.Steer(caller("alice"), SteerRequest{SessionID: sessionID, Prompt: prompt, IdempotencyKey: key})
}

// The first attempt started the turn and its answer was lost: the retry gets
// the turn that started, not a queued duplicate that would run as a second turn.
func TestARetriedKeyedSendThatStartedATurnStartsNoSecond(t *testing.T) {
	e := newQueueEnv(t)
	e.settle(t, e.id)
	first, err := e.sendKeyed(t, e.id, "go", "k-start")
	if err != nil || first.Queued != nil || first.Turns != 2 {
		t.Fatalf("the first send = %+v, %v; want a started turn 2", first, err)
	}
	retry, err := e.sendKeyed(t, e.id, "go", "k-start")
	if err != nil || retry.Queued != nil || retry.ID != e.id || retry.Turns != 2 {
		t.Fatalf("the retry = %+v, %v; want the turn the first send started (200, not a queued message)", retry, err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || len(e.pending(t)) != 0 {
		t.Fatalf("the retry changed something: turns %v, queued %v", got, e.pending(t))
	}

	// Long after: the turn settled, the session is idle and its queue empty,
	// where the old code would start the same message again. The workspace is
	// now at its live-sandbox limit, which a new send would run into: a replay
	// starts nothing, so it is not refused for it.
	e.settle(t, e.id)
	e.create(t, "")
	e.svc.MaxLiveSandboxes = 1
	late, err := e.sendKeyed(t, e.id, "go", "k-start")
	if err != nil || late.Queued != nil || late.Turns != 2 {
		t.Fatalf("a late retry = %+v, %v", late, err)
	}
	if got := e.prompts(t, e.id); len(got) != 2 || e.phase(t, e.id) != PhaseCompleted {
		t.Fatalf("a late retry started a turn: turns %v, phase %s", got, e.phase(t, e.id))
	}

	// The same key with another message is the existing refusal.
	if _, err := e.sendKeyed(t, e.id, "something else", "k-start"); !isCode(err, codeIdempotencyKeyReused) {
		t.Fatalf("a key reused for another message = %v, want %s", err, codeIdempotencyKeyReused)
	}
	if got := e.prompts(t, e.id); len(got) != 2 {
		t.Fatalf("a refused reuse started a turn: %v", got)
	}
	// The key covers the whole request, not only the prompt.
	if _, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "go", IdempotencyKey: "k-start",
		SessionSecretsInput: SessionSecretsInput{SecretNames: []string{"OTHER"}}}); !isCode(err, codeIdempotencyKeyReused) {
		t.Fatalf("a key reused for the same prompt with other secrets = %v", err)
	}
}

// The first attempt queued, was dispatched and the queue drained: the retry
// returns the delivered message and starts nothing.
func TestARetriedKeyedSendThatQueuedReturnsItsMessageInEveryState(t *testing.T) {
	e := newQueueEnv(t)
	e.send(t, "alice", "ahead of it")
	queued, err := e.sendKeyed(t, e.id, "later", "k-queued")
	if err != nil || queued.Queued == nil || queued.Queued.Position != 2 {
		t.Fatalf("the first send = %+v, %v", queued.Queued, err)
	}
	// Pending: the same message, with its place, and nothing queued twice.
	again, err := e.sendKeyed(t, e.id, "later", "k-queued")
	if err != nil || again.Queued == nil || again.Queued.ID != queued.Queued.ID || again.Queued.State != QueuedPending || again.Queued.Position != 2 {
		t.Fatalf("a retry while queued = %+v, %v; want the same message at place 2", again.Queued, err)
	}
	if got := e.pending(t); len(got) != 2 {
		t.Fatalf("the retry queued a duplicate: %v", got)
	}

	// Dispatched: both messages run, one turn each, and the queue drains.
	d := e.dispatcher()
	for range 2 {
		e.settle(t, e.id)
		if err := d.RunOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	e.settle(t, e.id)
	if got := e.prompts(t, e.id); len(got) != 3 || len(e.pending(t)) != 0 {
		t.Fatalf("after the queue drained: turns %v, queued %v", got, e.pending(t))
	}
	delivered, err := e.sendKeyed(t, e.id, "later", "k-queued")
	if err != nil || delivered.Queued == nil || delivered.Queued.ID != queued.Queued.ID ||
		delivered.Queued.State != QueuedDispatched || delivered.Queued.Turn != 3 {
		t.Fatalf("a retry after delivery = %+v, %v; want the dispatched message of turn 3", delivered.Queued, err)
	}
	if got := e.prompts(t, e.id); len(got) != 3 || e.phase(t, e.id) != PhaseCompleted {
		t.Fatalf("a retry after delivery started a turn: %v phase %s", got, e.phase(t, e.id))
	}
	if _, err := e.sendKeyed(t, e.id, "other", "k-queued"); !isCode(err, codeIdempotencyKeyReused) {
		t.Fatalf("a delivered message's key reused for another message = %v", err)
	}
}

// Keys are per session: the same key starts one turn on each of two sessions,
// and each retry finds its own.
func TestAnIdempotencyKeyBelongsToItsSession(t *testing.T) {
	e := newQueueEnv(t)
	other := e.create(t, "").ID
	e.settle(t, e.id)
	e.settle(t, other)
	for _, id := range []string{e.id, other} {
		view, err := e.sendKeyed(t, id, "same words", "shared-key")
		if err != nil || view.Queued != nil || view.Turns != 2 {
			t.Fatalf("session %s: first send = %+v, %v; want a started turn 2", id, view, err)
		}
		if retry, err := e.sendKeyed(t, id, "same words", "shared-key"); err != nil || retry.Queued != nil || retry.Turns != 2 {
			t.Fatalf("session %s: retry = %+v, %v", id, retry, err)
		}
		if got := e.prompts(t, id); len(got) != 2 {
			t.Fatalf("session %s: turns = %v", id, got)
		}
	}
}

// A send with no key is never a replay: the same message sent twice is two
// messages, as before.
func TestAKeylessSendIsNeverReplayed(t *testing.T) {
	e := newQueueEnv(t)
	e.settle(t, e.id)
	first, err := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "again"})
	second, err2 := e.svc.Steer(caller("alice"), SteerRequest{SessionID: e.id, Prompt: "again"})
	if err != nil || err2 != nil || first.Queued != nil || second.Queued == nil || second.Queued.Position != 1 {
		t.Fatalf("keyless sends = %+v / %+v, %v %v; want a start, then a queued message", first.Queued, second.Queued, err, err2)
	}
}

// Retries racing the first attempt (a client that timed out and sent again
// while the first was still running) start one turn between them and queue
// nothing: every attempt is answered, with the turn or its replay.
func TestRacingRetriesOfAKeyedSendStartOneTurn(t *testing.T) {
	e := newQueueEnv(t)
	e.settle(t, e.id)
	const senders = 8
	views := make([]View, senders)
	errs := make([]error, senders)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range senders {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			views[i], errs[i] = e.sendKeyed(t, e.id, "race", "k-race")
		}()
	}
	close(start)
	wg.Wait()
	for i := range senders {
		if errs[i] != nil || views[i].Queued != nil || views[i].Turns != 2 {
			t.Fatalf("attempt %d = %+v, %v; want the started turn 2 and no queued message", i, views[i], errs[i])
		}
	}
	if got := e.prompts(t, e.id); len(got) != 2 || len(e.pending(t)) != 0 {
		t.Fatalf("racing retries left turns %v and queued %v", got, e.pending(t))
	}
}
