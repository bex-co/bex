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
	"slices"
	"testing"
	"time"
)

// w9/m386: a new pull request is due at once; the claim takes due ones and
// pushes each to its retry; a record sets the state and the next read (nil
// leaves the due index), moving pr_state_at only on a change; a new pull
// request on the same session starts unread again; a pull_request delivery is
// confined to its workspaces; the list filters by state.
func TestPGAgentSessionPRStates(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "pr-states")
	ctx := context.Background()
	other, err := s.CreateTenant(ctx, "pr-states-other-"+time.Now().Format("150405.000000000"), PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(context.Background(), `DELETE FROM tenants WHERE id=$1`, other.ID) })
	newSession := func(ws string, pr int) string {
		t.Helper()
		row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: ws, Repo: "bex-co/app", InitialPrompt: "x"})
		if err != nil {
			t.Fatal(err)
		}
		if pr > 0 {
			if err := s.RecordAgentSessionPullRequest(ctx, row.ID, "https://github.com/bex-co/app/pull/x", pr, "abc"); err != nil {
				t.Fatal(err)
			}
		}
		return row.ID
	}
	nextCheck := func(id string) *time.Time {
		t.Helper()
		var at *time.Time
		if err := pool.QueryRow(ctx, `SELECT pr_next_check_at FROM agent_sessions WHERE id=$1`, id).Scan(&at); err != nil {
			t.Fatal(err)
		}
		return at
	}
	fresh := newSession(tenant.ID, 101)
	noPR := newSession(tenant.ID, 0)
	elsewhere := newSession(other.ID, 101)
	if nextCheck(fresh) == nil || nextCheck(noPR) != nil {
		t.Fatalf("a new pull request is not due at once, or a session without one is: %v %v", nextCheck(fresh), nextCheck(noPR))
	}
	// A takeover session is born with its pull request, and is due at once too.
	takeover, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, Repo: "bex-co/app", InitialPrompt: "x", PRNumber: 106, PRURL: "https://github.com/bex-co/app/pull/106", TakeoverPR: 106, TakeoverHeadRef: "feature"})
	if err != nil {
		t.Fatal(err)
	}
	if nextCheck(takeover.ID) == nil {
		t.Fatal("a takeover session's pull request is not due")
	}

	now := time.Now().UTC().Truncate(time.Second).Add(time.Minute)
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET pr_next_check_at=$2 WHERE id=$1`, elsewhere, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.ClaimAgentSessionPRChecks(ctx, now, time.Hour, 201)
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, c := range claimed {
		ids = append(ids, c.SessionID)
		if c.SessionID == fresh && (c.Repo != "bex-co/app" || c.PRNumber != 101 || c.PRState != "" || c.PRStateAt != nil || c.WorkspaceID != tenant.ID) {
			t.Fatalf("claimed row = %+v", c)
		}
	}
	if !slices.Contains(ids, fresh) || !slices.Contains(ids, takeover.ID) || slices.Contains(ids, noPR) || slices.Contains(ids, elsewhere) {
		t.Fatalf("claimed %v", ids)
	}
	if at := nextCheck(fresh); at == nil || !at.Equal(now.Add(time.Hour)) {
		t.Fatalf("the claim did not push the read to its retry: %v", at)
	}
	if again, _ := s.ClaimAgentSessionPRChecks(ctx, now, time.Hour, 201); slices.ContainsFunc(again, func(c AgentSessionPRCheck) bool { return c.SessionID == fresh }) {
		t.Fatal("a claimed pull request was claimed again before its retry")
	}

	// A record sets the state and the next read; pr_state_at moves on a change only.
	at := now.Add(time.Minute)
	next := at.Add(5 * time.Minute)
	if changed, err := s.RecordAgentSessionPRState(ctx, fresh, PRStateOpen, at, &next); err != nil || !changed {
		t.Fatalf("'' -> open = %v %v", changed, err)
	}
	if changed, err := s.RecordAgentSessionPRState(ctx, fresh, PRStateOpen, at.Add(time.Hour), &next); err != nil || changed {
		t.Fatalf("open -> open = %v %v", changed, err)
	}
	read, err := s.GetAgentSession(ctx, fresh)
	if err != nil || read.PRState != PRStateOpen || read.PRStateAt == nil || !read.PRStateAt.Equal(at) {
		t.Fatalf("after record = %q %v / %v", read.PRState, read.PRStateAt, err)
	}
	if changed, err := s.RecordAgentSessionPRState(ctx, fresh, PRStateMerged, at, nil); err != nil || !changed || nextCheck(fresh) != nil {
		t.Fatalf("merged = %v %v, next %v", changed, err, nextCheck(fresh))
	}
	if changed, err := s.RecordAgentSessionPRState(ctx, noPR, PRStateOpen, at, &next); err != nil || changed {
		t.Fatalf("a session without a pull request took a state: %v %v", changed, err)
	}
	if _, err := s.RecordAgentSessionPRState(ctx, fresh, "draft", at, nil); err == nil {
		t.Fatal("the column accepted a state Devin does not have")
	}

	// A new pull request on the same session starts unread and due.
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='running' WHERE id=$1`, fresh); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordAgentSessionPullRequest(ctx, fresh, "https://github.com/bex-co/app/pull/107", 107, ""); err != nil {
		t.Fatal(err)
	}
	if read, _ := s.GetAgentSession(ctx, fresh); read.PRState != "" || read.PRStateAt != nil || nextCheck(fresh) == nil {
		t.Fatalf("a new pull request kept the old state: %q %v next %v", read.PRState, read.PRStateAt, nextCheck(fresh))
	}

	// A pull_request delivery touches only its workspaces' sessions.
	n, err := s.RecordPullRequestState(ctx, []string{tenant.ID}, "bex-co/app", 106, PRStateClosed, at, &next)
	if err != nil || n != 1 {
		t.Fatalf("delivery changed %d / %v", n, err)
	}
	if read, _ := s.GetAgentSession(ctx, elsewhere); read.PRState != "" {
		t.Fatalf("another workspace's session took the state: %q", read.PRState)
	}

	// The list filters by state; sessions without a pull request never match.
	rows, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{PRStates: []string{PRStateClosed}, Limit: 50})
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].ID != takeover.ID {
		t.Fatalf("closed list = %v", rows)
	}
}
