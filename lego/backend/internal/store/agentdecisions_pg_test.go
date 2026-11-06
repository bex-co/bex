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
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"
)

// runningSession is a session in phase running on its first turn.
func runningSession(t *testing.T, s *PGStore, workspaceID string) AgentSession {
	t.Helper()
	ctx := context.Background()
	row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: workspaceID, InitialPrompt: "deploy", ApprovalMode: "risky"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET phase='running', sandbox_id='sbx' WHERE id=$1`, row.ID); err != nil {
		t.Fatal(err)
	}
	row, err = s.GetAgentSession(ctx, row.ID)
	if err != nil || row.ApprovalMode != "risky" {
		t.Fatalf("session = %+v, %v", row, err)
	}
	return row
}

func openApproval(t *testing.T, s *PGStore, row AgentSession, requestID string, ttl time.Duration) AgentDecision {
	t.Helper()
	d, created, err := s.OpenAgentDecision(context.Background(), AgentDecisionInput{SessionID: row.ID, WorkspaceID: row.WorkspaceID, Turn: 1,
		RequestID: requestID, Kind: AgentDecisionApproval, Title: "Run: git push --force", Detail: json.RawMessage(`{"command":"git push --force"}`), TTL: ttl})
	if err != nil || !created {
		t.Fatalf("open %s = %+v created=%v %v", requestID, d, created, err)
	}
	return d
}

// ADR100 D2: a decision mirrors onto the session (needs_input), answers once,
// and every lifecycle exit — answer, expiry, the turn ending — clears it.
func TestPGAgentDecisionLifecycle(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "decisions")
	ctx := context.Background()
	row := runningSession(t, s, tenant.ID)

	d := openApproval(t, s, row, "perm-1", time.Hour)
	// A retried open returns the same decision.
	again, created, err := s.OpenAgentDecision(ctx, AgentDecisionInput{SessionID: row.ID, WorkspaceID: tenant.ID, Turn: 1, RequestID: "perm-1",
		Kind: AgentDecisionApproval, Title: "other", TTL: time.Hour})
	if err != nil || created || again.ID != d.ID || again.Title != d.Title {
		t.Fatalf("retried open = %+v created=%v %v", again, created, err)
	}
	read, _ := s.GetAgentSession(ctx, row.ID)
	if read.PendingDecisionID != d.ID || read.PendingDecisionKind != AgentDecisionApproval {
		t.Fatalf("mirror = %q/%q", read.PendingDecisionID, read.PendingDecisionKind)
	}
	// The derived state in SQL matches the Go mapping.
	listed, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{States: []string{"needs_input"}, Limit: 10})
	if err != nil || len(listed) != 1 || listed[0].ID != row.ID {
		t.Fatalf("needs_input list = %+v, %v", listed, err)
	}
	if working, _ := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{States: []string{"working"}, Limit: 10}); len(working) != 0 {
		t.Fatalf("a paused session lists as working: %+v", working)
	}
	// One needs_input row per decision in the webhook feed.
	feed, err := s.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), nil, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	decisionRows := 0
	for _, r := range feed {
		if r.Source == EventSourceAgent && r.Phase == AgentSessionNeedsInputPhase && r.ServiceID == row.ID && r.Turn == 1 {
			decisionRows++
		}
	}
	if decisionRows != 1 {
		t.Fatalf("webhook feed has %d needs_input rows, want 1: %+v", decisionRows, feed)
	}
	pushes, err := s.ListTerminalAgentSessionsForPush(ctx, time.Now().Add(-time.Hour))
	if err != nil || !containsSession(pushes, row.ID) {
		t.Fatalf("push scan = %+v, %v", pushes, err)
	}

	// A second request while one is pending (a parallel tool call) leaves the
	// first open; the mirror keeps the oldest and counts both.
	second := openApproval(t, s, row, "perm-2", time.Hour)
	if first, _ := s.GetAgentDecision(ctx, d.ID); first.State != AgentDecisionPending {
		t.Fatalf("the first decision became %s when a second opened", first.State)
	}
	if id, count := decisionMirror(t, s, row.ID); id != d.ID || count != 2 {
		t.Fatalf("mirror with two open = %q x%d", id, count)
	}

	// Of two concurrent answers exactly one wins.
	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range 2 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, results[i] = s.ResolveAgentDecision(ctx, row.ID, second.ID, AgentDecisionAllowed, json.RawMessage(`{"allow":true,"scope":"once"}`), "alice")
		}()
	}
	wg.Wait()
	won := 0
	for _, err := range results {
		switch {
		case err == nil:
			won++
		case !errors.Is(err, ErrAgentDecisionAnswered):
			t.Fatalf("concurrent answer = %v", err)
		}
	}
	if won != 1 {
		t.Fatalf("%d answers won", won)
	}
	if id, count := decisionMirror(t, s, row.ID); id != d.ID || count != 1 {
		t.Fatalf("mirror after the newer answer = %q x%d", id, count)
	}
	if _, err := s.ResolveAgentDecision(ctx, row.ID, d.ID, AgentDecisionDenied, json.RawMessage(`{"allow":false}`), "alice"); err != nil {
		t.Fatal(err)
	}
	if id, count := decisionMirror(t, s, row.ID); id != "" || count != 0 {
		t.Fatalf("an answered decision is still mirrored: %q x%d", id, count)
	}
	if _, err := s.ResolveAgentDecision(ctx, "ags-other", second.ID, AgentDecisionDenied, json.RawMessage(`{"allow":false}`), "bob"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("answer through another session = %v", err)
	}

	// Expiry: past due, the answer is refused and the sweep resolves it.
	late := openApproval(t, s, row, "perm-3", time.Millisecond)
	time.Sleep(20 * time.Millisecond)
	if _, err := s.ResolveAgentDecision(ctx, row.ID, late.ID, AgentDecisionAllowed, json.RawMessage(`{"allow":true}`), "alice"); !errors.Is(err, ErrAgentDecisionExpired) {
		t.Fatalf("late answer = %v", err)
	}
	expired, err := s.ExpireAgentDecisions(ctx, 10)
	if err != nil || len(expired) != 1 || expired[0].ID != late.ID || string(expired[0].Answer) != `{"allow": false}` {
		t.Fatalf("sweep = %+v, %v", expired, err)
	}
	if read, _ = s.GetAgentSession(ctx, row.ID); read.PendingDecisionID != "" {
		t.Fatalf("an expired decision is still mirrored")
	}

	// The turn ending cancels an open decision and clears the mirror (trigger).
	last := openApproval(t, s, row, "perm-4", time.Hour)
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET phase='canceled' WHERE id=$1`, row.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.GetAgentDecision(ctx, last.ID); got.State != AgentDecisionCanceled {
		t.Fatalf("decision after cancel = %s", got.State)
	}
	if read, _ = s.GetAgentSession(ctx, row.ID); read.PendingDecisionID != "" || read.PendingDecisionKind != "" {
		t.Fatalf("mirror after cancel = %q/%q", read.PendingDecisionID, read.PendingDecisionKind)
	}
	if _, count := decisionMirror(t, s, row.ID); count != 0 {
		t.Fatalf("open count after cancel = %d", count)
	}
	// A session no longer running cannot pause.
	if _, _, err := s.OpenAgentDecision(ctx, AgentDecisionInput{SessionID: row.ID, WorkspaceID: tenant.ID, Turn: 1, RequestID: "perm-5",
		Kind: AgentDecisionQuestion, Title: "q", TTL: time.Hour}); !errors.Is(err, ErrAgentDecisionNotOpen) {
		t.Fatalf("open on a canceled session = %v", err)
	}
	// Nor can another workspace's caller name the session.
	if _, _, err := s.OpenAgentDecision(ctx, AgentDecisionInput{SessionID: row.ID, WorkspaceID: "tea-other", Turn: 1, RequestID: "perm-6",
		Kind: AgentDecisionQuestion, Title: "q", TTL: time.Hour}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("open from another workspace = %v", err)
	}
}

// decisionMirror reads the session's mirrored oldest open decision and count.
func decisionMirror(t *testing.T, s *PGStore, sessionID string) (string, int) {
	t.Helper()
	var id *string
	var count int
	if err := s.Pool.QueryRow(context.Background(), `SELECT pending_decision_id, pending_decision_count FROM agent_sessions WHERE id = $1`,
		sessionID).Scan(&id, &count); err != nil {
		t.Fatal(err)
	}
	if id == nil {
		return "", count
	}
	return *id, count
}

// An answer racing the turn ending (cancel, completion) must not deadlock.
// The phase trigger holds the session row and then updates the decision row;
// here a transaction does exactly that around an answer in flight. With the
// answer locking the session first it simply waits; had it locked the
// decision first, Postgres would abort one side with a deadlock (40P01).
func TestPGAgentDecisionAnswerRacingTheTurnEndDoesNotDeadlock(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "decision-race")
	ctx := context.Background()
	row := runningSession(t, s, tenant.ID)
	d := openApproval(t, s, row, "perm-1", time.Hour)

	turnEnd, err := s.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = turnEnd.Rollback(ctx) }()
	if _, err := turnEnd.Exec(ctx, `SELECT 1 FROM agent_sessions WHERE id = $1 FOR UPDATE`, row.ID); err != nil {
		t.Fatal(err)
	}
	answered := make(chan error, 1)
	go func() {
		_, err := s.ResolveAgentDecision(ctx, row.ID, d.ID, AgentDecisionAllowed, json.RawMessage(`{"allow":true,"scope":"once"}`), "alice")
		answered <- err
	}()
	time.Sleep(200 * time.Millisecond) // the answer is now blocked on the session row
	cancelCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if _, err := turnEnd.Exec(cancelCtx, `UPDATE agent_session_decisions SET state = 'canceled', answered_at = clock_timestamp()
		WHERE id = $1 AND state = 'pending'`, d.ID); err != nil {
		t.Fatalf("the turn end could not cancel the decision: %v", err)
	}
	if err := turnEnd.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if err := <-answered; err != nil && !errors.Is(err, ErrAgentDecisionAnswered) {
		t.Fatalf("the racing answer = %v", err)
	}
	if got, _ := s.GetAgentDecision(ctx, d.ID); got.State != AgentDecisionCanceled {
		t.Fatalf("decision state = %s, want canceled (the turn end won)", got.State)
	}
}

// Deleting a session with an open decision removes both (the mirror's foreign
// key is ON DELETE SET NULL, the decision's ON DELETE CASCADE).
func TestPGAgentSessionWithAnOpenDecisionDeletes(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "decision-delete")
	ctx := context.Background()
	row := runningSession(t, s, tenant.ID)
	d := openApproval(t, s, row, "perm-1", time.Hour)
	if _, err := s.Pool.Exec(ctx, `DELETE FROM agent_sessions WHERE id=$1`, row.ID); err != nil {
		t.Fatalf("delete a session with an open decision: %v", err)
	}
	if _, err := s.GetAgentDecision(ctx, d.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("decision after the session was deleted = %v", err)
	}
}

// A decision opening wakes wait_agent_sessions: the notify trigger fires on
// the mirror column, not only on phase.
func TestPGAgentDecisionNotifiesTheStateChannel(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "decision-notify")
	row := runningSession(t, s, tenant.ID)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	woke := make(chan string, 8)
	go func() { _ = s.ListenAgentSessionStates(ctx, func(id string) { woke <- id }) }()
	time.Sleep(200 * time.Millisecond) // the LISTEN is set up
	openApproval(t, s, row, "perm-1", time.Hour)
	select {
	case id := <-woke:
		if id != row.ID {
			t.Fatalf("notified %s, want %s", id, row.ID)
		}
	case <-ctx.Done():
		t.Fatal("opening a decision did not notify the state channel")
	}
}

func containsSession(rows []AgentSession, id string) bool {
	for _, r := range rows {
		if r.ID == id {
			return true
		}
	}
	return false
}
