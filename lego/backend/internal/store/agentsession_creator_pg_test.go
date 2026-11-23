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
	"slices"
	"testing"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func creatorSession(t *testing.T, s *PGStore, workspaceID, createdBy, automationID string) AgentSession {
	t.Helper()
	row, err := s.CreateAgentSession(context.Background(), AgentSession{
		WorkspaceID: workspaceID, AgentConfig: []byte(`{"agent":"claude","task":"x"}`), InitialPrompt: "x",
		CreatedBy: createdBy, AutomationID: automationID,
	})
	if err != nil {
		t.Fatalf("create session for %q: %v", createdBy, err)
	}
	return row
}

func sessionIDs(rows []AgentSession) []string {
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.ID)
	}
	return out
}

// w7/m296: the creator filter keeps only the listed subjects' sessions, in the
// keyset order the cursor pages, and a value Postgres cannot bind matches
// nothing instead of failing the query.
func TestPGAgentSessionListByCreator(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "creator-filter")
	ctx := context.Background()
	a1 := creatorSession(t, s, tenant.ID, "alice", "")
	c1 := creatorSession(t, s, tenant.ID, "carol", "")
	a2 := creatorSession(t, s, tenant.ID, "alice", "")

	page, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{CreatedBy: []string{"alice"}, Limit: 1})
	if err != nil || len(page) != 1 || page[0].ID != a2.ID {
		t.Fatalf("first page = %v err=%v, want [%s]", sessionIDs(page), err, a2.ID)
	}
	next, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{CreatedBy: []string{"alice"}, Cursor: page[0].ID, Limit: 1})
	if err != nil || len(next) != 1 || next[0].ID != a1.ID {
		t.Fatalf("second page = %v err=%v, want [%s]", sessionIDs(next), err, a1.ID)
	}
	both, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{CreatedBy: []string{"alice", "carol"}, Limit: 10})
	if err != nil || !slices.Equal(sessionIDs(both), []string{a2.ID, c1.ID, a1.ID}) {
		t.Fatalf("alice+carol = %v err=%v", sessionIDs(both), err)
	}
	none, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{CreatedBy: []string{"bad\x00subject"}, Limit: 10})
	if err != nil || len(none) != 0 {
		t.Fatalf("unbindable creator = %v err=%v, want an empty page", sessionIDs(none), err)
	}
}

// Tags are replaced whole, and an archived row comes back unchanged so the
// service can refuse (ADR065 D1) without a second read.
func TestPGReplaceAgentSessionTags(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "replace-tags")
	ctx := context.Background()
	row := creatorSession(t, s, tenant.ID, "alice", "")

	got, err := s.ReplaceAgentSessionTags(ctx, row.ID, []string{"bug", "ui"})
	if err != nil || !slices.Equal(got.Tags, []string{"bug", "ui"}) || !got.UpdatedAt.After(row.UpdatedAt) {
		t.Fatalf("replace = %+v err=%v", got.Tags, err)
	}
	tagged, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Tag: "ui", Limit: 10})
	if err != nil || !slices.Equal(sessionIDs(tagged), []string{row.ID}) {
		t.Fatalf("tag filter after replace = %v err=%v", sessionIDs(tagged), err)
	}
	if got, err = s.ReplaceAgentSessionTags(ctx, row.ID, nil); err != nil || len(got.Tags) != 0 {
		t.Fatalf("clear = %+v err=%v", got.Tags, err)
	}
	if _, err := s.ReplaceAgentSessionTags(ctx, row.ID, []string{"keep"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetAgentSessionArchived(ctx, row.ID, true); err != nil {
		t.Fatal(err)
	}
	got, err = s.ReplaceAgentSessionTags(ctx, row.ID, []string{"changed"})
	if err != nil || got.ArchivedAt == nil || !slices.Equal(got.Tags, []string{"keep"}) {
		t.Fatalf("archived replace = %+v archived=%v err=%v, want the unchanged row", got.Tags, got.ArchivedAt, err)
	}
	if _, err := s.ReplaceAgentSessionTags(ctx, ids.New(ids.AgentSession), []string{"x"}); err == nil {
		t.Fatal("replacing tags on a missing session succeeded")
	}
}

// w10/m599: a session's own `gh` binding is set to true or false or removed
// (inherit), the rest of agent_config is kept, and an archived row comes back
// unchanged so the service can refuse it.
func TestPGSetAgentSessionGitHubCLI(t *testing.T) {
	s, _, tenant := agentSessionPGStore(t, "set-github-cli")
	ctx := context.Background()
	row := creatorSession(t, s, tenant.ID, "alice", "")
	config := func(got AgentSession) map[string]any {
		t.Helper()
		var out map[string]any
		if err := json.Unmarshal(got.AgentConfig, &out); err != nil {
			t.Fatal(err)
		}
		return out
	}
	expect := func(when string, got AgentSession, want any) {
		t.Helper()
		cfg := config(got)
		binding, present := cfg["githubCli"]
		if want == nil && present || want != nil && binding != want {
			t.Fatalf("%s: agentConfig = %v, want githubCli %v", when, cfg, want)
		}
		if cfg["agent"] != "claude" || cfg["task"] != "x" {
			t.Fatalf("%s: the rest of agentConfig changed: %v", when, cfg)
		}
	}
	off, on := false, true
	got, err := s.SetAgentSessionGitHubCLI(ctx, row.ID, &off)
	if err != nil || !got.UpdatedAt.After(row.UpdatedAt) {
		t.Fatalf("set off = %+v err=%v", got, err)
	}
	expect("off", got, false)
	if got, err = s.SetAgentSessionGitHubCLI(ctx, row.ID, &on); err != nil {
		t.Fatal(err)
	}
	expect("on", got, true)
	if got, err = s.GetAgentSession(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	expect("read back", got, true)
	if got, err = s.SetAgentSessionGitHubCLI(ctx, row.ID, nil); err != nil {
		t.Fatal(err)
	}
	expect("inherit", got, nil)
	if _, err := s.SetAgentSessionGitHubCLI(ctx, row.ID, &off); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetAgentSessionArchived(ctx, row.ID, true); err != nil {
		t.Fatal(err)
	}
	got, err = s.SetAgentSessionGitHubCLI(ctx, row.ID, &on)
	if err != nil || got.ArchivedAt == nil {
		t.Fatalf("archived set = %+v err=%v, want the unchanged row", got, err)
	}
	expect("archived", got, false)
	if _, err := s.SetAgentSessionGitHubCLI(ctx, ids.New(ids.AgentSession), &on); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a missing session = %v, want ErrNotFound", err)
	}
}

// 0218 backfills the creator only where it is exact: a session an automation
// started, whose automation was never edited (version 1). A session of an
// edited automation, or with no automation, stays empty.
func TestPGAgentSessionCreatorBackfill(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "creator-backfill")
	ctx := context.Background()
	automation := func(createdBy string, version int) string {
		t.Helper()
		id := ids.New(ids.AgentAutomation)
		if _, err := pool.Exec(ctx, `
			INSERT INTO agent_automations (id, workspace_id, name, created_by, triggers, template, version)
			VALUES ($1, $2, $3, $4, '[]', '{}', $5)`, id, tenant.ID, "a"+id[len(id)-8:], createdBy, version); err != nil {
			t.Fatal(err)
		}
		return id
	}
	untouched := creatorSession(t, s, tenant.ID, "", automation("alice", 1))
	edited := creatorSession(t, s, tenant.ID, "", automation("carol", 2))
	manual := creatorSession(t, s, tenant.ID, "", "")
	recorded := creatorSession(t, s, tenant.ID, "dave", automation("erin", 1))

	up, err := migrationsFS.ReadFile("migrations/0218_agent_session_creator.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DROP INDEX agent_sessions_creator_idx`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("re-run 0218: %v", err)
	}
	creator := func(id string) string {
		t.Helper()
		var got string
		if err := tx.QueryRow(ctx, `SELECT created_by FROM agent_sessions WHERE id = $1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		return got
	}
	for id, want := range map[string]string{untouched.ID: "alice", edited.ID: "", manual.ID: "", recorded.ID: "dave"} {
		if got := creator(id); got != want {
			t.Errorf("session %s created_by = %q, want %q", id, got, want)
		}
	}
}
