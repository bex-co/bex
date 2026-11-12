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
	"testing"
	"time"
)

func TestPGAgentSessionFeedbackQueueAndDelivery(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-session-feedback")
	ctx := context.Background()
	session, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, Repo: "bex-co/app", InitialPrompt: "x", CreatedBy: "carol"})
	if err != nil {
		t.Fatal(err)
	}
	set := func(sql string, args ...any) {
		t.Helper()
		if _, err := pool.Exec(ctx, sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	set(`UPDATE agent_sessions SET pr_number=7, head_sha='abc123', phase='running' WHERE id=$1`, session.ID)

	// A PR and a head find their session; GitHub's repo spelling may differ in case.
	if got, err := s.FindAgentSessionForPullRequest(ctx, []string{tenant.ID}, "Bex-Co/App", 7); err != nil || got.ID != session.ID {
		t.Fatalf("by PR = %v %v", got.ID, err)
	}
	if _, err := s.FindAgentSessionForPullRequest(ctx, []string{"tea-other"}, "bex-co/app", 7); !errors.Is(err, ErrNotFound) {
		t.Fatalf("another workspace's PR = %v", err)
	}
	if heads, err := s.FindAgentSessionsForHead(ctx, []string{tenant.ID}, "bex-co/app", "abc123"); err != nil || len(heads) != 1 {
		t.Fatalf("by head = %v %v", heads, err)
	}

	item := func(key, kind string) AgentSessionFeedback {
		return AgentSessionFeedback{SessionID: session.ID, SourceKey: key, Kind: kind, Actor: "alice", Author: "alice-gh", URL: "https://github.com/x", Body: "> " + key}
	}
	for _, it := range []AgentSessionFeedback{item("comment:1", FeedbackPRComment), item("suite:2", FeedbackCIFailure)} {
		if inserted, err := s.QueueAgentSessionFeedback(ctx, it); err != nil || !inserted {
			t.Fatalf("queue %s = %v %v", it.SourceKey, inserted, err)
		}
	}
	if inserted, err := s.QueueAgentSessionFeedback(ctx, item("comment:1", FeedbackPRComment)); err != nil || inserted {
		t.Fatalf("a redelivery was queued again: %v %v", inserted, err)
	}

	now := time.Now()
	claimMine := func(at time.Time) []FeedbackBatch {
		t.Helper()
		got, err := s.ClaimDeliverableFeedback(ctx, at, time.Minute, 200, []string{"completed", "failed"})
		if err != nil {
			t.Fatal(err)
		}
		var mine []FeedbackBatch
		for _, b := range got {
			if b.Session.ID == session.ID {
				mine = append(mine, b)
			}
		}
		return mine
	}
	// A running session takes no feedback; once it finishes, one caller leases it.
	if got := claimMine(now); len(got) != 0 {
		t.Fatalf("a running session's feedback was claimed: %+v", got)
	}
	set(`UPDATE agent_sessions SET phase='completed' WHERE id=$1`, session.ID)
	got := claimMine(now)
	if len(got) != 1 || len(got[0].Items) != 2 || got[0].Items[0].SourceKey != "comment:1" {
		t.Fatalf("claim = %+v", got)
	}
	if again := claimMine(now); len(again) != 0 {
		t.Fatalf("a leased batch was claimed twice: %+v", again)
	}
	if after := claimMine(now.Add(2 * time.Minute)); len(after) != 1 {
		t.Fatalf("an expired lease was not reclaimed: %+v", after)
	}

	// Release lets the next pass retry; delivery records the turn and the CI run.
	if err := s.ReleaseFeedback(ctx, session.ID, []string{"comment:1", "suite:2"}); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeedbackDelivered(ctx, session.ID, []string{"comment:1", "suite:2"}, 2); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueAgentSessionFeedback(ctx, item("suite:3", FeedbackCIFailure)); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkFeedbackDelivered(ctx, session.ID, []string{"suite:3"}, 3); err != nil {
		t.Fatal(err)
	}
	turns, err := s.FeedbackTurns(ctx, session.ID)
	if err != nil || len(turns) != 2 || turns[0] != (FeedbackTurn{Turn: 3, CIOnly: true}) || turns[1] != (FeedbackTurn{Turn: 2, CIOnly: false}) {
		t.Fatalf("feedback turns = %+v err=%v, want turn 3 CI-only then turn 2 mixed", turns, err)
	}
	delivered, err := s.ListDeliveredFeedback(ctx, session.ID)
	if err != nil || len(delivered) != 3 || *delivered[0].DeliveredTurn != 2 || *delivered[2].DeliveredTurn != 3 ||
		delivered[0].URL == "" || delivered[0].Body != "" {
		t.Fatalf("delivered = %+v err=%v (origins carry no body)", delivered, err)
	}
	if left := claimMine(now.Add(time.Hour)); len(left) != 0 {
		t.Fatalf("delivered feedback was claimed again: %+v", left)
	}

	// A dropped item is never claimed, and an archived session takes nothing.
	if _, err := s.QueueAgentSessionFeedback(ctx, item("comment:4", FeedbackPRComment)); err != nil {
		t.Fatal(err)
	}
	if err := s.DropFeedback(ctx, session.ID, []string{"comment:4"}, "the member lost access"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.QueueAgentSessionFeedback(ctx, item("comment:5", FeedbackPRComment)); err != nil {
		t.Fatal(err)
	}
	set(`UPDATE agent_sessions SET archived_at=now() WHERE id=$1`, session.ID)
	if left := claimMine(now.Add(time.Hour)); len(left) != 0 {
		t.Fatalf("an archived session's feedback was claimed: %+v", left)
	}
	// The ended-session sweep drops what an archived or canceled session holds.
	if err := s.DropEndedSessionsFeedback(ctx, []string{"canceled"}); err != nil {
		t.Fatal(err)
	}
	var reason string
	if err := pool.QueryRow(ctx, `SELECT coalesce(dropped_reason, '') FROM agent_session_feedback WHERE session_id=$1 AND source_key='comment:5'`, session.ID).Scan(&reason); err != nil || reason != "the session ended" {
		t.Fatalf("ended-session sweep left reason %q (%v)", reason, err)
	}
}
