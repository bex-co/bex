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
	"sync"
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
	onMain := item("suite:3", FeedbackCIFailure)
	onMain.BaseFailingRef = "main"
	if _, err := s.QueueAgentSessionFeedback(ctx, onMain); err != nil {
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
		delivered[0].URL == "" || delivered[0].Body != "" || delivered[0].BaseFailingRef != "" || delivered[2].BaseFailingRef != "main" {
		t.Fatalf("delivered = %+v err=%v (origins carry no body, and the CI failure its base branch)", delivered, err)
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

// A takeover session is created on its own bex-agent/ branch with the pull
// request and its head ref recorded, and with the comment that started it as
// turn 1's delivered origin; a redelivery of that comment queues nothing, and
// one live session at most works on the pull request.
func TestPGAgentSessionTakeoverCreate(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-session-takeover")
	ctx := context.Background()
	head := "0123456789abcdef0123456789abcdef01234567"
	origin := AgentSessionFeedback{SourceKey: "comment:9001", Kind: FeedbackPRComment, Author: "alice-gh",
		URL: "https://github.com/bex-co/app/pull/8#issuecomment-9001", Body: "> fix it"}
	in := AgentSession{WorkspaceID: tenant.ID, Repo: "bex-co/app", InitialPrompt: "fix it", CreatedBy: "alice",
		TakeoverPR: 8, TakeoverHeadRef: "feature/fix-login", PRNumber: 8, PRURL: "https://github.com/bex-co/app/pull/8", HeadSHA: head,
		InitialOrigin: &origin, TakeoverScope: []string{tenant.ID}}
	in.InitialOrigin.Actor = "alice"
	session, err := s.CreateAgentSession(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	// The session's own branch is the default bex-agent/<id>; the pull
	// request's head is only what its PR verb fast-forwards.
	if session.Branch != DefaultAgentBranchPrefix+session.ID || !session.IsTakeover() || session.TakeoverHeadRef != "feature/fix-login" ||
		session.PRNumber != 8 || session.HeadSHA != head || session.PRURL == "" {
		t.Fatalf("takeover row = %+v", session)
	}
	if got, err := s.GetAgentSession(ctx, session.ID); err != nil || got.TakeoverPR != 8 || got.TakeoverHeadRef != "feature/fix-login" {
		t.Fatalf("read back = %+v %v", got, err)
	}
	if stored, err := s.HasAgentSessionFeedback(ctx, session.ID, "comment:9001"); err != nil || !stored {
		t.Fatalf("origin stored = %v %v", stored, err)
	}
	if stored, err := s.HasAgentSessionFeedback(ctx, session.ID, "review:1"); err != nil || stored {
		t.Fatalf("an unknown key stored = %v %v", stored, err)
	}
	if got, err := s.FindAgentSessionForPullRequest(ctx, []string{tenant.ID}, "Bex-Co/App", 8); err != nil || got.ID != session.ID {
		t.Fatalf("by PR = %v %v", got.ID, err)
	}
	delivered, err := s.ListDeliveredFeedback(ctx, session.ID)
	if err != nil || len(delivered) != 1 || *delivered[0].DeliveredTurn != 1 || delivered[0].Author != "alice-gh" || delivered[0].Kind != FeedbackPRComment {
		t.Fatalf("turn-1 origin = %+v %v", delivered, err)
	}
	if inserted, err := s.QueueAgentSessionFeedback(ctx, AgentSessionFeedback{SessionID: session.ID, SourceKey: "comment:9001",
		Kind: FeedbackPRComment, Actor: "alice", Body: "> fix it"}); err != nil || inserted {
		t.Fatalf("a redelivered starting comment was queued: %v %v", inserted, err)
	}
	if turns, err := s.FeedbackTurns(ctx, session.ID); err != nil || len(turns) != 1 || turns[0].Turn != 1 || turns[0].CIOnly {
		t.Fatalf("feedback turns = %+v %v", turns, err)
	}
	// While it works on the pull request, a second takeover of it is refused;
	// once it is canceled, the pull request can be taken over again.
	second := in
	second.InitialOrigin = nil
	if _, err := s.CreateAgentSession(ctx, second); !errors.Is(err, ErrAgentSessionPullRequestOwned) {
		t.Fatalf("a second takeover = %v, want ErrAgentSessionPullRequestOwned", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='canceled' WHERE id=$1`, session.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateAgentSession(ctx, second); err != nil {
		t.Fatalf("a takeover after cancel: %v", err)
	}

	// A session that took nothing over is 0, and keeps its default branch.
	plain, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, Repo: "bex-co/app", InitialPrompt: "x"})
	if err != nil || plain.TakeoverPR != 0 || plain.Branch != DefaultAgentBranchPrefix+plain.ID {
		t.Fatalf("plain row = %+v %v", plain, err)
	}
	// A takeover without its head ref or PR is refused, by the store and the schema.
	if _, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, Repo: "bex-co/app", TakeoverPR: 8, PRNumber: 8}); !errors.Is(err, ErrInvalid) {
		t.Fatalf("a takeover with no head ref = %v, want ErrInvalid", err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET takeover_pr_number=3 WHERE id=$1`, plain.ID); err == nil {
		t.Fatal("the schema accepted a takeover with no head ref")
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET takeover_head_ref='x' WHERE id=$1`, plain.ID); err == nil {
		t.Fatal("the schema accepted a head ref without a takeover")
	}
}

// An ended session never shadows the live one working on its pull request,
// and unarchiving a takeover while another live session works on its pull
// request is refused under the same lock as a create.
func TestPGAgentSessionTakeoverUnarchiveKeepsOneLiveSession(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "agent-session-takeover-unarchive")
	ctx := context.Background()
	in := AgentSession{WorkspaceID: tenant.ID, Repo: "bex-co/unarchive", InitialPrompt: "fix it", CreatedBy: "alice",
		TakeoverPR: 9, TakeoverHeadRef: "feature/x", PRNumber: 9, PRURL: "https://github.com/bex-co/unarchive/pull/9",
		HeadSHA: "0123456789abcdef0123456789abcdef01234567", TakeoverScope: []string{tenant.ID}}
	first, err := s.CreateAgentSession(ctx, in)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetAgentSessionArchived(ctx, first.ID, true); err != nil {
		t.Fatal(err)
	}
	second, err := s.CreateAgentSession(ctx, in) // the archived one no longer owns the PR
	if err != nil {
		t.Fatal(err)
	}
	// A newer, ended session on the same PR does not shadow the live one.
	ended, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, Repo: "bex-co/unarchive", InitialPrompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET pr_number=9, phase='canceled' WHERE id=$1`, ended.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := s.FindAgentSessionForPullRequest(ctx, []string{tenant.ID}, "bex-co/unarchive", 9); err != nil || got.ID != second.ID {
		t.Fatalf("by PR = %s %v, want the live %s", got.ID, err, second.ID)
	}
	if _, err := s.SetAgentSessionArchived(ctx, first.ID, false); !errors.Is(err, ErrAgentSessionPullRequestOwned) {
		t.Fatalf("unarchive while another works on the PR = %v, want ErrAgentSessionPullRequestOwned", err)
	}
	if got, _ := s.GetAgentSession(ctx, first.ID); got.ArchivedAt == nil {
		t.Fatal("the refused unarchive unarchived the session")
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='canceled' WHERE id=$1`, second.ID); err != nil {
		t.Fatal(err)
	}
	if got, err := s.SetAgentSessionArchived(ctx, first.ID, false); err != nil || got.ArchivedAt != nil {
		t.Fatalf("unarchive once the PR is free = %+v %v", got.ArchivedAt, err)
	}
	// A session that took nothing over unarchives as before.
	if _, err := s.SetAgentSessionArchived(ctx, ended.ID, true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetAgentSessionArchived(ctx, ended.ID, false); err != nil {
		t.Fatalf("unarchive a plain session: %v", err)
	}
	if _, err := s.SetAgentSessionArchived(ctx, "ags-missing", false); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unarchive a missing session = %v, want ErrNotFound", err)
	}
}

// Two `/bex` comments on one pull request, at once: the per-pull-request lock
// lets exactly one takeover create through, and the other is refused as owned.
func TestPGAgentSessionConcurrentTakeoversStartOne(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "agent-session-takeover-race")
	ctx := context.Background()
	in := AgentSession{WorkspaceID: tenant.ID, Repo: "bex-co/race", InitialPrompt: "fix it", CreatedBy: "alice",
		TakeoverPR: 12, TakeoverHeadRef: "feature/race", PRNumber: 12, PRURL: "https://github.com/bex-co/race/pull/12",
		HeadSHA: "0123456789abcdef0123456789abcdef01234567", TakeoverScope: []string{tenant.ID}}
	const racers = 6
	errs := make(chan error, racers)
	var start sync.WaitGroup
	start.Add(1)
	for range racers {
		go func() {
			start.Wait()
			_, err := s.CreateAgentSession(ctx, in)
			errs <- err
		}()
	}
	start.Done()
	created, owned := 0, 0
	for range racers {
		switch err := <-errs; {
		case err == nil:
			created++
		case errors.Is(err, ErrAgentSessionPullRequestOwned):
			owned++
		default:
			t.Fatalf("a racing takeover create: %v", err)
		}
	}
	if created != 1 || owned != racers-1 {
		t.Fatalf("created %d, refused as owned %d; want 1 and %d", created, owned, racers-1)
	}
}
