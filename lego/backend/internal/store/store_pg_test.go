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
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"reflect"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// TestPGStore exercises migrations + the real Store against a live Postgres.
// Hermetic-by-default: skipped unless BEX_TEST_DB_URI points at a throwaway
// database (e.g. `docker run --rm -e POSTGRES_PASSWORD=pw -p 5433:5432 postgres:17`
// → postgres://postgres:pw@localhost:5433/postgres?sslmode=disable).
func TestPGStore(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()

	// Idempotent: running the embedded migrations twice must be a no-op.
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("re-migrate: %v", err)
	}

	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	// Isolate from previous runs (order respects FKs via cascade). audit_events
	// has no FK to tenants (w4/m10 — a purged tenant's audit trail must outlive
	// the row's own cascade delete), so it needs its own truncate.
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `TRUNCATE audit_events`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	// Plan is the workspace plan (hobby/pro/scale/enterprise) — the CHECK
	// constraint rejects anything else. The app's compute tier ("starter") is a
	// separate ladder on apps.tier, unconstrained here.
	ten, err := s.CreateTenant(ctx, "acme", PlanPro)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	app, err := s.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 2, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create app: %v", err)
	}

	// Ids are Render-style typed opaque strings: "<prefix>-<20-char xid>".
	if len(ten.ID) != 24 || ten.ID[:4] != "tea-" || len(app.ID) != 24 || app.ID[:4] != "srv-" {
		t.Errorf("ids not Render-style: tenant=%q app=%q", ten.ID, app.ID)
	}

	assertErrorTaxonomy(ctx, t, s, ten)
	assertSlugMinting(ctx, t, s, app)
	assertProjectionJoin(ctx, t, s, app)
	assertDeployLifecycle(ctx, t, s, app)
	assertConcurrentDeployTriggers(ctx, t, s, ten.ID)
	assertDomainUniqueness(ctx, t, s, ten.ID)
	assertMembershipRoleOutbox(ctx, t, s, ten.ID)
	assertAuditEvents(ctx, t, s, ten)
	assertServiceEvents(ctx, t, s, ten, app)
	assertWorkspaceLifecycle(ctx, t, s, pool)
	assertRegistryCredentials(ctx, t, s, ten)
	assertAgentSessions(ctx, t, s, ten)
	assertProjectsAndEnvironments(ctx, t, s, pool, ten, app)
	assertWebhooks(ctx, t, s, pool, ten, app)
	assertDeleteCascades(ctx, t, s, pool, app)
}

func assertMembershipRoleOutbox(ctx context.Context, t *testing.T, s *PGStore, tenantID string) {
	t.Helper()
	const subject = "role-outbox-member"
	if err := s.AddMember(ctx, subject, tenantID, "developer"); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = s.RemoveMember(ctx, tenantID, subject) }()
	if err := s.UpdateMemberRole(ctx, tenantID, subject, "viewer"); err != nil {
		t.Fatal(err)
	}
	claimed, err := s.ClaimRoleReconciliations(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, row := range claimed {
		if row.TenantID == tenantID && row.Subject == subject && row.Role == "viewer" {
			found = true
		}
	}
	if !found {
		t.Fatalf("updated membership did not atomically enqueue exact-role repair: %+v", claimed)
	}
	// A stale worker acknowledging the prior desired role must not delete the
	// current row; only the role it actually applied may complete it.
	if err := s.CompleteRoleReconciliation(ctx, tenantID, subject, "developer"); err != nil {
		t.Fatal(err)
	}
	var queued int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM membership_role_reconciliations WHERE tenant_id=$1 AND subject=$2`, tenantID, subject).Scan(&queued); err != nil || queued != 1 {
		t.Fatalf("stale role acknowledgement deleted queue: count=%d err=%v", queued, err)
	}
	if err := s.CompleteRoleReconciliation(ctx, tenantID, subject, "viewer"); err != nil {
		t.Fatal(err)
	}
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM membership_role_reconciliations WHERE tenant_id=$1 AND subject=$2`, tenantID, subject).Scan(&queued); err != nil || queued != 0 {
		t.Fatalf("matching role acknowledgement count=%d err=%v", queued, err)
	}
}

func assertAgentSessions(ctx context.Context, t *testing.T, s *PGStore, tenant Tenant) {
	t.Helper()
	record, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/example", Branch: "main",
		AgentConfig: []byte(`{"agent":"codex","task":"test"}`), InitialPrompt: "test",
	})
	if err != nil {
		t.Fatalf("create agent session: %v", err)
	}
	if kind, ok := ids.KindOf(record.ID); !ok || kind != ids.AgentSession || record.Phase != "creating" {
		t.Fatalf("agent session id/phase = %q/%q", record.ID, record.Phase)
	}
	turns, err := s.AgentSessionTurns(ctx, record.ID)
	if err != nil || len(turns) != 1 || turns[0].Turn != 1 || turns[0].Prompt != "test" {
		t.Fatalf("atomic initial turn = %+v err=%v", turns, err)
	}
	record, err = s.SetAgentSessionLifecycle(ctx, record.ID, "sandbox-1", "running", "running", false)
	if err != nil || record.SandboxID != "sandbox-1" || record.Phase != "running" {
		t.Fatalf("bind agent sandbox = %+v err=%v", record, err)
	}
	got, err := s.GetAgentSession(ctx, record.ID)
	if err != nil || got.WorkspaceID != tenant.ID || got.Repo != "bex-co/example" {
		t.Fatalf("get agent session = %+v err=%v", got, err)
	}
	listed, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Limit: 50})
	if err != nil || len(listed) != 1 || listed[0].ID != record.ID {
		t.Fatalf("list agent sessions = %+v err=%v", listed, err)
	}

	// w3/m41 delivery surface: a dispatch bumps the turn counter and delivery
	// mode; ListAgentSessionsByPhases finds the running turn; Finalize records the
	// completed result + evidence and is queryable back.
	record, err = s.RecordAgentSessionDispatch(ctx, record.ID, "sandbox-2", "running", "running", "redispatch")
	if err != nil || record.SandboxID != "sandbox-2" || record.Turns != 1 || record.DeliveryMode != "redispatch" {
		t.Fatalf("dispatch agent session = %+v err=%v", record, err)
	}
	running, err := s.ListAgentSessionsByPhases(ctx, []string{"running", "creating"})
	if err != nil || len(running) != 1 || running[0].ID != record.ID {
		t.Fatalf("list-by-phases = %+v err=%v", running, err)
	}
	record, err = s.FinalizeAgentSession(ctx, record.ID, "completed", "abc123",
		"https://github.com/bex-co/example/pull/7", 7, []byte(`{"commandLog":["go test"]}`), "")
	if err != nil || record.Phase != "completed" || record.HeadSHA != "abc123" || record.PRNumber != 7 {
		t.Fatalf("finalize agent session = %+v err=%v", record, err)
	}
	got, err = s.GetAgentSession(ctx, record.ID)
	if err != nil || got.PRURL == "" || string(got.Evidence) == "{}" || got.Turns != 1 {
		t.Fatalf("finalized read-back = %+v err=%v", got, err)
	}
	if none, err := s.ListAgentSessionsByPhases(ctx, []string{"running"}); err != nil || len(none) != 0 {
		t.Fatalf("completed session still listed as running = %+v err=%v", none, err)
	}

	assertAgentSessionTranscripts(ctx, t, s, tenant, record.ID)

	record, err = s.SetAgentSessionLifecycle(ctx, record.ID, "", "canceled", "canceled", true)
	if err != nil || record.CanceledAt == nil {
		t.Fatalf("cancel agent session = %+v err=%v", record, err)
	}

	assertAgentSessionArchive(ctx, t, s, tenant, record.ID)
	assertAgentSessionPromptQuota(ctx, t, s, tenant)
	assertAgentSessionTurnAcceptanceCAS(ctx, t, s, tenant)
}

func assertAgentSessionPromptQuota(ctx context.Context, t *testing.T, s *PGStore, tenant Tenant) {
	t.Helper()
	record, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/quota", Branch: "main",
		AgentConfig: []byte(`{"agent":"codex","task":"seed"}`), InitialPrompt: "seed",
	})
	if err != nil {
		t.Fatalf("create prompt quota session: %v", err)
	}
	if _, err := s.SetAgentSessionLifecycle(ctx, record.ID, "", "completed", "completed", false); err != nil {
		t.Fatalf("complete prompt quota session: %v", err)
	}
	// Fill the aggregate just below 8 MiB using schema-valid 100 KiB turns.
	if _, err := s.Pool.Exec(ctx, `
		INSERT INTO agent_session_turns (session_id, turn, prompt, delivery_mode)
		SELECT $1, n, repeat('x', 100000), 'redispatch'
		FROM generate_series(2, 84) AS n`, record.ID); err != nil {
		t.Fatalf("seed prompt quota: %v", err)
	}
	if _, err := s.Pool.Exec(ctx, `UPDATE agent_sessions SET turns=84 WHERE id=$1`, record.ID); err != nil {
		t.Fatalf("advance prompt quota session: %v", err)
	}
	if _, err := s.BeginAgentSessionTurn(ctx, record.ID, strings.Repeat("y", 100000), "redispatch", "redispatching", "redispatching"); !errors.Is(err, ErrAgentSessionPromptQuota) {
		t.Fatalf("prompt quota error = %v, want ErrAgentSessionPromptQuota", err)
	}
	got, err := s.GetAgentSession(ctx, record.ID)
	if err != nil || got.Turns != 84 || got.Phase != "completed" {
		t.Fatalf("prompt quota rejection mutated session = %+v err=%v", got, err)
	}
}

func assertAgentSessionTurnAcceptanceCAS(ctx context.Context, t *testing.T, s *PGStore, tenant Tenant) {
	t.Helper()
	record, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/concurrent-turn", Branch: "main",
		AgentConfig: []byte(`{"agent":"codex","task":"initial"}`), InitialPrompt: "initial",
	})
	if err != nil {
		t.Fatalf("create concurrent-turn session: %v", err)
	}
	if _, err := s.SetAgentSessionLifecycle(ctx, record.ID, "", "completed", "completed", false); err != nil {
		t.Fatalf("complete concurrent-turn session: %v", err)
	}
	start := make(chan struct{})
	errs := make(chan error, 2)
	for _, prompt := range []string{"first contender", "second contender"} {
		prompt := prompt
		go func() {
			<-start
			_, err := s.BeginAgentSessionTurn(ctx, record.ID, prompt, "redispatch", "redispatching", "redispatching")
			errs <- err
		}()
	}
	close(start)
	var successes, conflicts int
	for range 2 {
		err := <-errs
		switch {
		case err == nil:
			successes++
		case errors.Is(err, ErrAgentSessionTurnState):
			conflicts++
		default:
			t.Fatalf("concurrent turn error = %v", err)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("concurrent turn results success=%d conflict=%d, want 1/1", successes, conflicts)
	}
	got, err := s.GetAgentSession(ctx, record.ID)
	if err != nil || got.Turns != 2 || got.Phase != "redispatching" {
		t.Fatalf("concurrent accepted turn row = %+v err=%v", got, err)
	}
	turns, err := s.AgentSessionTurns(ctx, record.ID)
	if err != nil || len(turns) != 2 {
		t.Fatalf("concurrent accepted turns = %+v err=%v, want initial + one winner", turns, err)
	}
}

// assertAgentSessionArchive proves the ADR065 store contract: the idempotent
// archive flag, the default-excludes-archived filtered list, keyset pagination,
// and retention expiry stamping archived_at.
func assertAgentSessionArchive(ctx context.Context, t *testing.T, s *PGStore, tenant Tenant, canceledID string) {
	t.Helper()

	// Two more sessions so the list has three rows to filter and page over.
	second, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/other", Branch: "main",
		AgentConfig: []byte(`{"agent":"codex","task":"second"}`),
	})
	if err != nil {
		t.Fatalf("create second session: %v", err)
	}
	third, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/example", Branch: "main",
		AgentConfig: []byte(`{"agent":"codex","task":"third"}`),
	})
	if err != nil {
		t.Fatalf("create third session: %v", err)
	}

	// Archive is idempotent and keeps the FIRST archive time; unarchive clears it.
	archived, err := s.SetAgentSessionArchived(ctx, second.ID, true)
	if err != nil || archived.ArchivedAt == nil {
		t.Fatalf("archive = %+v err=%v", archived, err)
	}
	firstArchivedAt := *archived.ArchivedAt
	rearchived, err := s.SetAgentSessionArchived(ctx, second.ID, true)
	if err != nil || rearchived.ArchivedAt == nil || !rearchived.ArchivedAt.Equal(firstArchivedAt) {
		t.Fatalf("re-archive must keep the original archived_at: %+v err=%v", rearchived, err)
	}
	if rearchived.Phase != "creating" {
		t.Fatalf("archive must not touch phase, got %q", rearchived.Phase)
	}

	// Default list = the unarchived working set; ArchivedOnly and ArchivedInclude
	// widen it. Order is newest first.
	working, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Limit: 50})
	if err != nil || len(working) != 2 || working[0].ID != third.ID || working[1].ID != canceledID {
		t.Fatalf("working-set list = %+v err=%v", working, err)
	}
	only, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Archived: ArchivedOnly, Limit: 50})
	if err != nil || len(only) != 1 || only[0].ID != second.ID {
		t.Fatalf("archived-only list = %+v err=%v", only, err)
	}
	all, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Archived: ArchivedInclude, Limit: 50})
	if err != nil || len(all) != 3 {
		t.Fatalf("all list = %d rows err=%v", len(all), err)
	}

	// Filters: phase and repo narrow; a filter matching nothing is empty.
	if rows, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Archived: ArchivedInclude, Phases: []string{"canceled"}, Limit: 50}); err != nil || len(rows) != 1 || rows[0].ID != canceledID {
		t.Fatalf("phase filter = %+v err=%v", rows, err)
	}
	if rows, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Archived: ArchivedInclude, Repo: "bex-co/other", Limit: 50}); err != nil || len(rows) != 1 || rows[0].ID != second.ID {
		t.Fatalf("repo filter = %+v err=%v", rows, err)
	}
	if rows, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Archived: ArchivedInclude, CreatedBefore: all[2].CreatedAt, Limit: 50}); err != nil || len(rows) != 0 {
		t.Fatalf("createdBefore oldest = %+v err=%v", rows, err)
	}

	// Keyset pagination: page of 2, cursor = last item's id, then the tail; an
	// unknown/foreign cursor is an empty page, never an error.
	page1, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Archived: ArchivedInclude, Limit: 2})
	if err != nil || len(page1) != 2 || page1[0].ID != all[0].ID || page1[1].ID != all[1].ID {
		t.Fatalf("page 1 = %+v err=%v", page1, err)
	}
	page2, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Archived: ArchivedInclude, Limit: 2, Cursor: page1[1].ID})
	if err != nil || len(page2) != 1 || page2[0].ID != all[2].ID {
		t.Fatalf("page 2 = %+v err=%v", page2, err)
	}
	if rows, err := s.ListAgentSessions(ctx, tenant.ID, AgentSessionListQuery{Archived: ArchivedInclude, Limit: 2, Cursor: "ags-doesnotexist000000000"}); err != nil || len(rows) != 0 {
		t.Fatalf("unknown cursor = %+v err=%v", rows, err)
	}

	// Unarchive returns the row to the working set.
	unarchived, err := s.SetAgentSessionArchived(ctx, second.ID, false)
	if err != nil || unarchived.ArchivedAt != nil {
		t.Fatalf("unarchive = %+v err=%v", unarchived, err)
	}

	// Retention expiry auto-archives (ADR065 D5): hibernate the third session,
	// expire it, and the row is canceled + archived with the snapshot cleared —
	// but its history (the row) survives.
	if _, err := s.SetAgentSessionLifecycle(ctx, third.ID, "sandbox-arch", "completed", "completed", false); err != nil {
		t.Fatalf("complete third: %v", err)
	}
	if _, err := s.ClaimAgentSessionForHibernation(ctx, third.ID); err != nil {
		t.Fatalf("claim hibernation: %v", err)
	}
	if _, err := s.HibernateAgentSession(ctx, third.ID, "snapshots/x", 42, "sha", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("hibernate: %v", err)
	}
	expired, err := s.ExpireHibernatedAgentSession(ctx, third.ID, "snapshots/x")
	if err != nil || expired.Phase != "canceled" || expired.ArchivedAt == nil || expired.SnapshotRef != "" {
		t.Fatalf("expire must cancel+archive+clear snapshot, got %+v err=%v", expired, err)
	}
	if expired.FailureReason != "hibernation retention window elapsed" {
		t.Fatalf("expire reason = %q", expired.FailureReason)
	}

	// Clean up the extra rows so the surrounding flow's counts stay stable.
	for _, id := range []string{second.ID, third.ID} {
		if err := s.DeleteAgentSession(ctx, id); err != nil {
			t.Fatalf("delete extra session %s: %v", id, err)
		}
	}
}

// assertAgentSessionTranscripts proves the w3/m43 transcript store: ordered
// append + replay, cross-replica/re-attach turn-local idempotency, the global
// cursor, and cascade with the session row.
func assertAgentSessionTranscripts(ctx context.Context, t *testing.T, s *PGStore, tenant Tenant, sessionID string) {
	t.Helper()

	// Empty transcript: no max seq, empty replay.
	if _, ok, err := s.AgentSessionTranscriptMaxSeq(ctx, sessionID); err != nil || ok {
		t.Fatalf("empty transcript max seq: ok=%v err=%v", ok, err)
	}
	if parts, err := s.AgentSessionTranscript(ctx, sessionID, -1, 1<<30, 0); err != nil || len(parts) != 0 {
		t.Fatalf("empty transcript replay = %+v err=%v", parts, err)
	}
	if total, err := s.AgentSessionTranscriptBytes(ctx, sessionID); err != nil || total != 0 {
		t.Fatalf("empty transcript bytes = %d err=%v", total, err)
	}

	first := []AgentSessionTranscriptPart{
		{PartIndex: 0, Turn: 1, Part: []byte(`{"type":"start"}`)},
		{PartIndex: 1, Turn: 1, Part: []byte(`{"type":"text-delta","delta":"hi"}`)},
		{PartIndex: 2, Turn: 1, Part: []byte(`{"type":"data-acp","data":{"kind":"plan"}}`)},
	}
	if err := s.AppendAgentSessionTranscript(ctx, sessionID, first); err != nil {
		t.Fatalf("append transcript: %v", err)
	}
	// Re-teeing the SAME parts (another replica / a re-attach re-reading the
	// driver's replayed history) must be a no-op, never a duplicate.
	if err := s.AppendAgentSessionTranscript(ctx, sessionID, first); err != nil {
		t.Fatalf("idempotent re-append: %v", err)
	}
	maxSeq, ok, err := s.AgentSessionTranscriptMaxSeq(ctx, sessionID)
	if err != nil || !ok || maxSeq != 2 {
		t.Fatalf("max seq after append = %d ok=%v err=%v", maxSeq, ok, err)
	}
	full, err := s.AgentSessionTranscript(ctx, sessionID, -1, 1<<30, 0)
	if err != nil || len(full) != 3 {
		t.Fatalf("full replay = %d parts err=%v (dedup failed if 6)", len(full), err)
	}
	// The cumulative-byte counter the write paths seed their quota from.
	var wantBytes int64
	for _, p := range first {
		wantBytes += int64(len(p.Part))
	}
	if total, err := s.AgentSessionTranscriptBytes(ctx, sessionID); err != nil || total != wantBytes {
		t.Fatalf("transcript bytes = %d err=%v, want %d", total, err, wantBytes)
	}
	// The bounded replay read (w1/m65 F10 fix): a budget covering only the first
	// two parts returns exactly that prefix, in order, never the whole
	// transcript — the cap is enforced by the store method, not the caller.
	prefixBudget := int64(len(first[0].Part) + len(first[1].Part))
	prefix, err := s.AgentSessionTranscript(ctx, sessionID, -1, prefixBudget, 0)
	if err != nil || len(prefix) != 2 || prefix[0].Seq != 0 || prefix[1].Seq != 1 {
		t.Fatalf("bounded replay = %+v err=%v, want the 2-part prefix", prefix, err)
	}
	// The SQL row limit (ADR065 D2's poll-shaped page) bounds independently of
	// the byte budget.
	if page, err := s.AgentSessionTranscript(ctx, sessionID, -1, 1<<30, 2); err != nil || len(page) != 2 || page[1].Seq != 1 {
		t.Fatalf("row-limited page = %+v err=%v, want 2 rows", page, err)
	}
	if full[0].Seq != 0 || full[2].Seq != 2 || string(full[1].Part) != `{"type":"text-delta","delta":"hi"}` {
		t.Fatalf("replay order/verbatim wrong: %+v", full)
	}

	if maxIndex, ok, err := s.AgentSessionTranscriptTurnMaxIndex(ctx, sessionID, 1); err != nil || !ok || maxIndex != 2 {
		t.Fatalf("turn 1 max local index = %d ok=%v err=%v, want 2", maxIndex, ok, err)
	}
	if _, ok, err := s.AgentSessionTranscriptTurnMaxIndex(ctx, sessionID, 2); err != nil || ok {
		t.Fatalf("turn 2 local index exists=%v err=%v before append", ok, err)
	}

	// The gateway resumes its live tee strictly after the stored max: append
	// seq 3+ and read only the tail via the cursor.
	if err := s.AppendAgentSessionTranscript(ctx, sessionID, []AgentSessionTranscriptPart{
		{PartIndex: 0, Turn: 2, Part: []byte(`{"type":"finish"}`)},
	}); err != nil {
		t.Fatalf("append tail: %v", err)
	}
	tail, err := s.AgentSessionTranscript(ctx, sessionID, maxSeq, 1<<30, 0)
	if err != nil || len(tail) != 1 || tail[0].Seq != 3 || tail[0].Turn != 2 {
		t.Fatalf("cursor tail = %+v err=%v", tail, err)
	}
	if maxIndex, ok, err := s.AgentSessionTranscriptTurnMaxIndex(ctx, sessionID, 2); err != nil || !ok || maxIndex != 0 {
		t.Fatalf("turn 2 max local index = %d ok=%v err=%v, want 0", maxIndex, ok, err)
	}

	// Cascade: a transcript on a throwaway session vanishes with the session row.
	// (Cascade is the ONLY transcript deletion — ADR065 D2 removed the unwired
	// age-based prune: a session's conversation lives as long as its row.)
	throwaway, err := s.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: tenant.ID, Repo: "bex-co/example", Branch: "main",
		AgentConfig: []byte(`{"agent":"codex","task":"cascade"}`),
	})
	if err != nil {
		t.Fatalf("create cascade session: %v", err)
	}
	if err := s.AppendAgentSessionTranscript(ctx, throwaway.ID, []AgentSessionTranscriptPart{
		{Seq: 0, Turn: 1, Part: []byte(`{"type":"start"}`)},
	}); err != nil {
		t.Fatalf("append cascade transcript: %v", err)
	}
	if err := s.DeleteAgentSession(ctx, throwaway.ID); err != nil {
		t.Fatalf("delete cascade session: %v", err)
	}
	if parts, err := s.AgentSessionTranscript(ctx, throwaway.ID, -1, 1<<30, 0); err != nil || len(parts) != 0 {
		t.Fatalf("transcript survived session delete = %+v err=%v", parts, err)
	}

	// A part referencing a nonexistent session fails closed (FK → not found).
	if err := s.AppendAgentSessionTranscript(ctx, "ags-doesnotexist000000000", []AgentSessionTranscriptPart{
		{Seq: 0, Turn: 1, Part: []byte(`{"type":"start"}`)},
	}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("append to missing session err = %v, want ErrNotFound", err)
	}
}

// assertConcurrentDeployTriggers proves the App-row lock and partial unique
// indexes turn overlapping API triggers into deterministic
// active-plus-latest-pending history: both callers succeed, the original row
// remains active, exactly one trigger stays queued, and the queued row has the
// highest generation even if the older request reaches Postgres last.
func assertConcurrentDeployTriggers(ctx context.Context, t *testing.T, s *PGStore, tenantID string) {
	t.Helper()
	app, err := s.CreateApp(ctx, App{
		TenantID: tenantID, Name: "deploy-race", Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create deploy-race app: %v", err)
	}
	defer func() {
		if err := s.DeleteApp(ctx, app.ID); err != nil {
			t.Errorf("delete deploy-race app: %v", err)
		}
	}()

	start := make(chan struct{})
	created := make([]Deploy, 2)
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i := range created {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			created[i], errs[i] = s.CreateDeploy(ctx, app.ID, TriggerAPI, app.Image, int64(i+2), CommitInfo{})
		}(i)
	}
	close(start)
	wg.Wait()
	for i, err := range errs {
		if err != nil {
			t.Fatalf("concurrent trigger %d: %v", i, err)
		}
	}

	history, err := s.ListDeploys(ctx, app.ID, DeployFilter{})
	if err != nil || len(history) != 3 {
		t.Fatalf("race history = %+v (err %v), want initial + two triggers", history, err)
	}
	statusCounts := map[string]int{}
	for _, d := range history {
		statusCounts[d.Status]++
	}
	if statusCounts[DeployCreated] != 1 || statusCounts[DeployQueued] != 1 || statusCounts[DeployCanceled] != 1 {
		t.Fatalf("race statuses = %v, want one active create, latest queued, and skipped queued row canceled", statusCounts)
	}
	var open Deploy
	for _, d := range history {
		if d.Generation == 3 && d.Status == DeployQueued {
			open = d
			break
		}
	}
	if open.ID == "" {
		t.Fatalf("race history = %+v, want highest App generation 3 queued", history)
	}

	// Cancel and convergence use the same row-locked transition writer. Let
	// them race and prove exactly one terminal fact wins without a rewrite.
	var results [2]bool
	errs = make([]error, 2)
	start = make(chan struct{})
	for i, status := range []string{DeployCanceled, DeployLive} {
		wg.Add(1)
		go func(i int, status string) {
			defer wg.Done()
			<-start
			results[i], errs[i] = s.CloseDeploy(ctx, open.ID, status, "")
		}(i, status)
	}
	close(start)
	wg.Wait()
	if errs[0] != nil || errs[1] != nil || results[0] == results[1] {
		t.Fatalf("cancel/live race = results %v errors %v, want exactly one successful transition", results, errs)
	}
	settled, err := s.GetDeploy(ctx, app.ID, open.ID)
	if err != nil || (settled.Status != DeployCanceled && settled.Status != DeployLive) || settled.FinishedAt == nil {
		t.Fatalf("cancel/live race settled = %+v (err %v), want immutable canceled or live terminal row", settled, err)
	}
}

// assertRegistryCredentials exercises w2/m14's registry_credentials store
// methods against real Postgres: Create/List/Get/GetByHost/Update/Delete, the
// cross-workspace scoping guard (a caller can never fetch/delete another
// workspace's row even by guessing its id), and newest-first host-lookup
// ordering — things that depend on Postgres's WHERE/ORDER BY, not just Go logic.
func assertRegistryCredentials(ctx context.Context, t *testing.T, s *PGStore, ten Tenant) {
	t.Helper()
	other, err := s.CreateTenant(ctx, "other-workspace", PlanPro)
	if err != nil {
		t.Fatalf("create other tenant: %v", err)
	}

	c, err := s.CreateRegistryCredential(ctx, ten.ID, "", "ghcr.io", "alice", "alice@example.com", nil)
	if err != nil {
		t.Fatalf("create registry credential: %v", err)
	}
	if len(c.ID) < 4 || c.ID[:4] != "rgc-" {
		t.Errorf("id not Render-style: %q", c.ID)
	}
	if c.CreatedAt.IsZero() || c.UpdatedAt.IsZero() {
		t.Errorf("timestamps not set: %+v", c)
	}
	if c.Name != "ghcr.io" {
		t.Errorf("empty name should default to host: %q", c.Name)
	}

	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	c2, err := s.CreateRegistryCredential(ctx, ten.ID, "Docker Hub prod", "docker.io", "bob", "bob@example.com", &expires)
	if err != nil {
		t.Fatalf("create second registry credential: %v", err)
	}
	if c2.Name != "Docker Hub prod" {
		t.Errorf("explicit name not stored: %q", c2.Name)
	}

	bound, err := s.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "registry-bound", Image: "docker.io/acme/private:1",
		RegistryCredentialID: &c2.ID, Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create registry-bound app: %v", err)
	}
	defer func() {
		if err := s.DeleteApp(ctx, bound.ID); err != nil {
			t.Errorf("delete registry-bound app: %v", err)
		}
	}()
	gotBound, err := s.GetApp(ctx, bound.ID)
	if err != nil || gotBound.RegistryCredentialID == nil || *gotBound.RegistryCredentialID != c2.ID {
		t.Fatalf("get registry-bound app = %+v (err %v)", gotBound, err)
	}
	empty := ""
	if err := s.SetAppSource(ctx, bound.ID, "", "docker.io/acme/private:2", "main", &empty); err != nil {
		t.Fatalf("clear registry binding: %v", err)
	}
	gotBound, err = s.GetApp(ctx, bound.ID)
	if err != nil || gotBound.RegistryCredentialID == nil || *gotBound.RegistryCredentialID != "" {
		t.Fatalf("explicit empty registry binding did not persist: %+v (err %v)", gotBound, err)
	}

	list, err := s.ListRegistryCredentials(ctx, ten.ID)
	if err != nil || len(list) != 2 || list[0].ID != c2.ID {
		t.Fatalf("list (want newest first) = %+v (err %v)", list, err)
	}

	got, err := s.GetRegistryCredential(ctx, ten.ID, c.ID)
	if err != nil || got.Host != "ghcr.io" || got.Username != "alice" {
		t.Fatalf("get = %+v (err %v)", got, err)
	}
	gotByID, err := s.GetRegistryCredentialByID(ctx, c.ID)
	if err != nil || gotByID.WorkspaceID != ten.ID {
		t.Fatalf("unscoped binding lookup = %+v (err %v)", gotByID, err)
	}
	if _, err := s.GetRegistryCredentialByID(ctx, "rgc-no-such"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unscoped unknown binding lookup: want ErrNotFound, got %v", err)
	}
	if _, err := s.GetRegistryCredential(ctx, other.ID, c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get scoped to the wrong workspace: want ErrNotFound, got %v", err)
	}

	byHost, err := s.GetRegistryCredentialByHost(ctx, ten.ID, "docker.io")
	if err != nil || byHost.ID != c2.ID {
		t.Fatalf("get by host = %+v (err %v)", byHost, err)
	}
	if _, err := s.GetRegistryCredentialByHost(ctx, ten.ID, "no-such-host.example"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get by unknown host: want ErrNotFound, got %v", err)
	}

	updated, err := s.UpdateRegistryCredential(ctx, ten.ID, c.ID, "GHCR alice", "alice2", &expires)
	if err != nil || updated.Name != "GHCR alice" || updated.Username != "alice2" || updated.ExpiresAt == nil || !updated.ExpiresAt.Equal(expires) {
		t.Fatalf("update = %+v (err %v)", updated, err)
	}

	if err := s.TouchRegistryCredential(ctx, ten.ID, c.ID); err != nil {
		t.Fatalf("touch: %v", err)
	}
	if err := s.TouchRegistryCredential(ctx, other.ID, c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("touch scoped to the wrong workspace: want ErrNotFound, got %v", err)
	}

	if err := s.DeleteRegistryCredential(ctx, other.ID, c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("delete scoped to the wrong workspace: want ErrNotFound, got %v", err)
	}
	if err := s.DeleteRegistryCredential(ctx, ten.ID, c.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetRegistryCredential(ctx, ten.ID, c.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get after delete: want ErrNotFound, got %v", err)
	}
}

// assertProjectsAndEnvironments exercises the environments store layer
// (layered on top of w1/m31's projects table) against real Postgres: creating
// an environment under a project, service assignment (which also joins the
// service to the environment's project), reassignment, delete cascades, and
// the defensive project_id+environment_id filter ListEnvironmentServices
// applies against drift from the independent SetProjectServices verb.
func assertProjectsAndEnvironments(ctx context.Context, t *testing.T, s *PGStore, pool *pgxpool.Pool, ten Tenant, app App) {
	t.Helper()
	proj, err := s.CreateProject(ctx, ten.ID, "web-stack")
	if err != nil {
		t.Fatalf("create project: %v", err)
	}

	env, err := s.CreateEnvironment(ctx, proj.ID, ten.ID, "staging")
	if err != nil {
		t.Fatalf("create environment: %v", err)
	}
	if len(env.ID) != 24 || env.ID[:4] != "env-" {
		t.Errorf("environment id not Render-style: %q", env.ID)
	}
	// w4/m28 no-lockout invariant: empty means deny-all now, so a fresh
	// environment must start explicitly seeded allow-all (0.0.0.0/0 + ::/0)
	// — never an empty list a member would enforce as deny-everything.
	if len(env.IPAllowList) != 2 || env.IPAllowList[0].CIDRBlock != "0.0.0.0/0" || env.IPAllowList[1].CIDRBlock != "::/0" {
		t.Errorf("new environment ip_allow_list = %+v, want the seeded allow-all pair", env.IPAllowList)
	}
	prod, err := s.CreateEnvironment(ctx, proj.ID, ten.ID, "production")
	if err != nil {
		t.Fatalf("create second environment: %v", err)
	}
	if _, err := s.CreateEnvironment(ctx, proj.ID, ten.ID, "staging"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate environment name: want ErrConflict, got %v", err)
	}

	if list, err := s.ListEnvironments(ctx, proj.ID); err != nil || len(list) != 2 {
		t.Fatalf("ListEnvironments = %+v (err %v), want 2", list, err)
	}

	// Assigning by the public stable id also joins the service to its project.
	if err := s.SetEnvironmentServices(ctx, env.ID, proj.ID, ten.ID, []string{app.ID}); err != nil {
		t.Fatalf("set environment services: %v", err)
	}
	var gotProjectID *string
	if err := pool.QueryRow(ctx, `SELECT project_id FROM apps WHERE id = $1`, app.ID).Scan(&gotProjectID); err != nil {
		t.Fatal(err)
	}
	if gotProjectID == nil || *gotProjectID != proj.ID {
		t.Errorf("assigning to an environment should also set apps.project_id = %q, got %v", proj.ID, gotProjectID)
	}
	if ids, err := s.ListEnvironmentServices(ctx, env.ID, proj.ID); err != nil || len(ids) != 1 || ids[0] != app.ID {
		t.Fatalf("ListEnvironmentServices(staging) = %+v (err %v), want public id [%s]", ids, err, app.ID)
	}
	if ids, err := s.ListProjectServices(ctx, proj.ID); err != nil || len(ids) != 1 || ids[0] != app.ID {
		t.Fatalf("ListProjectServices = %+v (err %v), want public id [%s]", ids, err, app.ID)
	}

	// Legacy name input remains accepted during the stable-id transition.
	if err := s.SetEnvironmentServices(ctx, prod.ID, proj.ID, ten.ID, []string{app.Name}); err != nil {
		t.Fatalf("reassign: %v", err)
	}
	if ids, err := s.ListEnvironmentServices(ctx, env.ID, proj.ID); err != nil || len(ids) != 0 {
		t.Fatalf("ListEnvironmentServices(staging) after reassign = %+v (err %v), want empty", ids, err)
	}
	if ids, err := s.ListEnvironmentServices(ctx, prod.ID, proj.ID); err != nil || len(ids) != 1 || ids[0] != app.ID {
		t.Fatalf("ListEnvironmentServices(production) = %+v (err %v), want public id [%s]", ids, err, app.ID)
	}

	if err := s.RenameEnvironment(ctx, env.ID, "staging-v2"); err != nil {
		t.Fatalf("rename environment: %v", err)
	}
	if got, err := s.GetEnvironment(ctx, env.ID); err != nil || got.Name != "staging-v2" {
		t.Fatalf("get after rename = %+v (err %v)", got, err)
	}

	// The ACL triple round-trips with per-entry descriptions (w4/m24,
	// migration 0034: ip_allow_list is jsonb {cidrBlock, description} entries).
	acl := []core.IPAllowListEntry{{CIDRBlock: "10.0.0.0/8", Description: "office"}, {CIDRBlock: "192.0.2.0/24"}}
	if err := s.SetEnvironmentACL(ctx, env.ID, "protected", true, acl); err != nil {
		t.Fatalf("SetEnvironmentACL: %v", err)
	}
	if got, err := s.GetEnvironment(ctx, env.ID); err != nil ||
		got.ProtectedStatus != "protected" || !got.NetworkIsolationEnabled ||
		len(got.IPAllowList) != 2 || got.IPAllowList[0] != acl[0] || got.IPAllowList[1] != acl[1] {
		t.Fatalf("ACL round-trip = %+v (err %v), want %+v", got.IPAllowList, err, acl)
	}
	if got, err := s.GetEnvironmentProtectedStatus(ctx, env.ID); err != nil || got != "protected" {
		t.Fatalf("GetEnvironmentProtectedStatus(protected) = %q, %v", got, err)
	}
	if got, err := s.GetEnvironmentProtectedStatus(ctx, "env-deleted"); err != nil || got != "unprotected" {
		t.Fatalf("GetEnvironmentProtectedStatus(missing) = %q, %v, want unprotected, nil", got, err)
	}
	// Migration 0053 closes the storage seam too: a direct legacy string write
	// is rejected before the strict application decoder can encounter it.
	if _, err := pool.Exec(ctx, `UPDATE environments SET ip_allow_list = '["203.0.113.0/24"]'::jsonb WHERE id = $1`, env.ID); err == nil {
		t.Fatal("environment storage accepted a bare CIDR string")
	}
	if got, err := s.GetEnvironment(ctx, env.ID); err != nil || len(got.IPAllowList) != 2 || got.IPAllowList[0] != acl[0] || got.IPAllowList[1] != acl[1] {
		t.Fatalf("rejected legacy write changed canonical ACL = %+v (err %v)", got.IPAllowList, err)
	}

	// w4/m32/t001: SetProjectServices must NULL environment_id (not just
	// project_id) for a departing row, in the same transaction — leaving a
	// project must not strand a stale apps.environment_id (and the App CR's
	// frozen spec.environmentIPAllowList it implies) behind. app is currently
	// a member of prod (both project_id and environment_id set).
	departed, err := s.SetProjectServices(ctx, proj.ID, ten.ID, nil)
	if err != nil {
		t.Fatalf("SetProjectServices(remove all): %v", err)
	}
	if len(departed) != 1 || departed[0] != app.Name {
		t.Fatalf("SetProjectServices departedWithEnv = %v, want [%s] (app carried a non-null environment_id)", departed, app.Name)
	}
	var gotEnvID *string
	if err := pool.QueryRow(ctx, `SELECT project_id, environment_id FROM apps WHERE id = $1`, app.ID).Scan(&gotProjectID, &gotEnvID); err != nil {
		t.Fatal(err)
	}
	if gotProjectID != nil || gotEnvID != nil {
		t.Errorf("after departing the project, project_id=%v environment_id=%v, want both NULL", gotProjectID, gotEnvID)
	}
	// Rejoin so the DeleteEnvironment/DeleteProject assertions below (which
	// assume app is a prod member) still hold.
	if err := s.SetEnvironmentServices(ctx, prod.ID, proj.ID, ten.ID, []string{app.ID}); err != nil {
		t.Fatalf("rejoin prod: %v", err)
	}

	// Deleting an environment un-assigns its services but leaves their
	// project membership untouched (only setProjectServices/deleting the
	// PROJECT does that).
	if err := s.DeleteEnvironment(ctx, prod.ID); err != nil {
		t.Fatalf("delete environment: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT project_id FROM apps WHERE id = $1`, app.ID).Scan(&gotProjectID); err != nil {
		t.Fatal(err)
	}
	if gotProjectID == nil || *gotProjectID != proj.ID {
		t.Errorf("deleting the environment should NOT clear apps.project_id, got %v", gotProjectID)
	}
	if err := s.DeleteEnvironment(ctx, prod.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("re-delete environment: want ErrNotFound, got %v", err)
	}

	// Deleting the project cascades its remaining environment (staging) too.
	if err := s.DeleteProject(ctx, proj.ID); err != nil {
		t.Fatalf("delete project: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM environments WHERE project_id = $1`, proj.ID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Errorf("environments not cascaded on project delete: %d rows remain", remaining)
	}
}

// assertDeployLifecycle exercises w2/m5's deploy history against real
// Postgres: CreateApp already opened deploy #1 (trigger "create") in the same
// transaction as the app row; ListOpenDeploys/CloseDeploy are the
// reconciler's write-back seam; CreateDeploy is the trigger verb's seam. Ordering
// (newest-first) and the ErrNotFound cross-app scope guard are asserted here
// because they depend on Postgres's ORDER BY / WHERE, not just Go logic.
func assertDeployLifecycle(ctx context.Context, t *testing.T, s *PGStore, app App) {
	t.Helper()
	deploys, err := s.ListDeploys(ctx, app.ID, DeployFilter{})
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(deploys) != 1 || deploys[0].Trigger != "create" || deploys[0].Status != DeployCreated {
		t.Fatalf("deploy #1 (from CreateApp) = %+v", deploys)
	}
	first := deploys[0]

	open, ok, err := openDeployFor(ctx, s, app.ID)
	if err != nil || !ok || open.ID != first.ID {
		t.Fatalf("open deploy = %+v ok=%v (err %v)", open, ok, err)
	}

	if won, err := s.CloseDeploy(ctx, first.ID, DeployLive, "img:resolved"); err != nil || !won {
		t.Fatalf("close deploy: won=%v err=%v", won, err)
	}
	// Idempotent: a deploy that's already terminal doesn't get re-closed with a
	// different status, and CAS reports it lost the race.
	if won, err := s.CloseDeploy(ctx, first.ID, DeployUpdateFailed, ""); err != nil || won {
		t.Fatalf("re-close deploy: won=%v err=%v, want won=false", won, err)
	}
	closed, err := s.GetDeploy(ctx, app.ID, first.ID)
	if err != nil || closed.Status != DeployLive || closed.FinishedAt == nil || closed.ResolvedImage != "img:resolved" {
		t.Fatalf("closed deploy = %+v (err %v), want status live with finished_at + resolved_image set", closed, err)
	}
	if _, ok, err := openDeployFor(ctx, s, app.ID); err != nil || ok {
		t.Fatalf("open deploy after close: ok=%v (err %v), want none open", ok, err)
	}

	second, err := s.CreateDeploy(ctx, app.ID, "api", app.Image, 2, CommitInfo{Hash: "abc1234def", Message: "fix: header"})
	if err != nil || second.Status != DeployCreated {
		t.Fatalf("trigger deploy: %+v (err %v)", second, err)
	}
	// Commit metadata round-trips through the real columns (w9/001) — `commit`
	// is an unreserved SQL keyword, so this also proves the unquoted column
	// name survives real Postgres.
	if got, err := s.GetDeploy(ctx, app.ID, second.ID); err != nil || got.Commit != "abc1234def" || got.CommitMessage != "fix: header" {
		t.Fatalf("commit round-trip = %+v (err %v), want hash+message back", got, err)
	}
	deploys, err = s.ListDeploys(ctx, app.ID, DeployFilter{})
	if err != nil || len(deploys) != 2 || deploys[0].ID != second.ID {
		t.Fatalf("list after trigger (want newest first) = %+v (err %v)", deploys, err)
	}

	// DeployFilter against real Postgres (w2/m31) — the additive WHERE/LIMIT
	// building and the keyset subquery are SQL, so they're asserted here, not
	// just against the Go fakes. The deeper multi-page walk lives in
	// deploys_test.go's memStore tests; the two rows on hand (second = open,
	// newer; first = live, older) cover each clause's direction.
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{Statuses: []string{DeployLive}}); err != nil || len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("status filter = %+v (err %v), want [%s]", got, err, first.ID)
	}
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{Limit: 1}); err != nil || len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("limit 1 = %+v (err %v), want the newest [%s]", got, err, second.ID)
	}
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{Cursor: second.ID}); err != nil || len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("cursor after newest = %+v (err %v), want [%s]", got, err, first.ID)
	}
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{Cursor: first.ID}); err != nil || len(got) != 0 {
		t.Fatalf("cursor after oldest = %+v (err %v), want the empty end-of-history page", got, err)
	}
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{Cursor: "dep-doesnotexist00000"}); err != nil || len(got) != 0 {
		t.Fatalf("unknown cursor = %+v (err %v), want an empty page, never the unfiltered list", got, err)
	}
	// createdBefore/createdAfter are exclusive: second's own instant bounds out
	// second itself in both directions.
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{CreatedBefore: second.CreatedAt}); err != nil || len(got) != 1 || got[0].ID != first.ID {
		t.Fatalf("createdBefore = %+v (err %v), want [%s]", got, err, first.ID)
	}
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{CreatedAfter: first.CreatedAt}); err != nil || len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("createdAfter = %+v (err %v), want [%s]", got, err, second.ID)
	}

	if _, err := s.GetDeploy(ctx, "srv-doesnotexist00000", second.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get deploy scoped to the wrong app: want ErrNotFound, got %v", err)
	}

	// Reaching live atomically deactivates the prior live deploy and advances
	// both rows' transition timestamps. The prior finished_at remains the time
	// it originally reached live; deactivation is represented by updated_at.
	if won, err := s.CloseDeploy(ctx, second.ID, DeployLive, "img:second"); err != nil || !won {
		t.Fatalf("close second live: won=%v err=%v", won, err)
	}
	prior, err := s.GetDeploy(ctx, app.ID, first.ID)
	if err != nil || prior.Status != DeployDeactivated || prior.FinishedAt == nil || !prior.FinishedAt.Equal(*closed.FinishedAt) || !prior.UpdatedAt.After(closed.UpdatedAt) {
		t.Fatalf("prior deploy after second live = %+v (err %v), want deactivated with original finished_at and newer updated_at", prior, err)
	}
	current, err := s.GetDeploy(ctx, app.ID, second.ID)
	if err != nil || current.Status != DeployLive || current.StartedAt == nil || current.FinishedAt == nil || !current.UpdatedAt.After(second.UpdatedAt) {
		t.Fatalf("current deploy = %+v (err %v), want live with transition timestamps", current, err)
	}
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{UpdatedAfter: second.UpdatedAt}); err != nil || len(got) != 2 {
		t.Fatalf("updatedAfter = %+v (err %v), want both rows changed by the live/deactivate transaction", got, err)
	}
	if got, err := s.ListDeploys(ctx, app.ID, DeployFilter{FinishedAfter: second.CreatedAt}); err != nil || len(got) != 1 || got[0].ID != second.ID {
		t.Fatalf("finishedAfter = %+v (err %v), want only the second deploy", got, err)
	}

	// Image-pull failure is journaled in the SAME transaction as the terminal
	// deploy edge. Exercise it on an isolated App so the service-feed assertions
	// below retain their original two-deploy fixture.
	failureApp, err := s.CreateApp(ctx, App{
		TenantID: app.TenantID, Name: "image-pull-failure", Image: "registry.example/missing:v1",
		Branch: "main", Port: 8080, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create image-pull fixture: %v", err)
	}
	defer func() {
		if err := s.DeleteApp(ctx, failureApp.ID); err != nil {
			t.Errorf("delete image-pull fixture: %v", err)
		}
	}()
	failureDeploys, err := s.ListDeploys(ctx, failureApp.ID, DeployFilter{})
	if err != nil || len(failureDeploys) != 1 {
		t.Fatalf("image-pull deploy fixture = %+v (err %v)", failureDeploys, err)
	}
	failedDeploy := failureDeploys[0]
	if won, err := s.TransitionDeploy(ctx, failedDeploy.ID, DeployUpdateFailed, "", "bounded operator diagnosis", EventReasonImagePullBackoff); err != nil || !won {
		t.Fatalf("image-pull transition = (%v, %v)", won, err)
	}
	failureEvents, err := s.ListServiceEvents(ctx, failureApp.ID, core.ServiceTarget(failureApp.Name), failureApp.TenantID, ServiceEventFilter{
		FactTypes: []string{string(EventFactImagePullFailed)},
	})
	if err != nil || len(failureEvents) != 1 {
		t.Fatalf("image-pull facts = %+v (err %v), want exactly one", failureEvents, err)
	}
	failureFact := failureEvents[0]
	if failureFact.DeployID != failedDeploy.ID || failureFact.Image != failureApp.Image || failureFact.ReasonCode != EventReasonImagePullBackoff {
		t.Fatalf("image-pull fact = %+v, want deploy/image/bounded reason", failureFact)
	}
	if won, err := s.TransitionDeploy(ctx, failedDeploy.ID, DeployUpdateFailed, "", "retry", EventReasonImagePullBackoff); err != nil || won {
		t.Fatalf("image-pull retry = (%v, %v), want terminal no-op", won, err)
	}
}

// assertAuditEvents exercises w4/m10's audit_events store methods against real
// Postgres: Record (the core.AuditSink write side, *PGStore satisfies it
// directly), newest-first ordering, the since/until/cursor filters, and
// PurgeAuditEvents' retention delete — all things that depend on Postgres's
// ORDER BY/keyset comparison, not just Go logic.
func assertAuditEvents(ctx context.Context, t *testing.T, s *PGStore, ten Tenant) {
	t.Helper()
	base := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	var recorded []AuditRow
	for i, outcome := range []core.AuditOutcome{core.AuditAllowed, core.AuditDenied, core.AuditAllowed} {
		ev := core.AuditEvent{
			Caller: "user-x", CallerMethod: "session",
			Verb:     "apps.Suspend",
			Resource: "workspace:" + ten.ID,
			Outcome:  outcome,
			At:       base.Add(time.Duration(i) * time.Minute),
		}
		if i == 2 {
			disabled := false
			ev.Verb = "apps.SetMaintenanceMode"
			ev.MaintenanceModeTo = &disabled
		}
		if err := s.Record(ctx, ev); err != nil {
			t.Fatalf("record audit event %d: %v", i, err)
		}
	}
	// A second workspace's event must never leak into ten's list.
	if err := s.Record(ctx, core.AuditEvent{Caller: "other", Verb: "apps.Suspend", Resource: "workspace:tea-other0000000", Outcome: core.AuditAllowed, At: base}); err != nil {
		t.Fatalf("record audit event for other workspace: %v", err)
	}

	all, err := s.ListAuditEvents(ctx, ten.ID, AuditFilter{})
	if err != nil {
		t.Fatalf("list audit events: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("list audit events = %d rows, want 3 (scoped to %s)", len(all), ten.ID)
	}
	// Newest first.
	if all[0].At.Before(all[1].At) || all[1].At.Before(all[2].At) {
		t.Fatalf("list audit events not newest-first: %+v", all)
	}
	if all[0].Outcome != string(core.AuditAllowed) || all[0].Verb != "apps.SetMaintenanceMode" || all[0].Caller != "user-x" ||
		all[0].MaintenanceModeTo == nil || *all[0].MaintenanceModeTo {
		t.Errorf("newest row = %+v", all[0])
	}
	recorded = all

	// Cursor resumes strictly after the given row — page size 1 from the
	// newest should walk the same three rows in the same order.
	page, err := s.ListAuditEvents(ctx, ten.ID, AuditFilter{Limit: 1})
	if err != nil || len(page) != 1 || page[0].ID != recorded[0].ID {
		t.Fatalf("first page = %+v (err %v), want [%s]", page, err, recorded[0].ID)
	}
	page2, err := s.ListAuditEvents(ctx, ten.ID, AuditFilter{Limit: 1, Cursor: page[0].ID})
	if err != nil || len(page2) != 1 || page2[0].ID != recorded[1].ID {
		t.Fatalf("second page = %+v (err %v), want [%s]", page2, err, recorded[1].ID)
	}

	// since/until bound At inclusively — the middle event only.
	windowed, err := s.ListAuditEvents(ctx, ten.ID, AuditFilter{Since: base.Add(time.Minute), Until: base.Add(time.Minute)})
	if err != nil || len(windowed) != 1 || windowed[0].Outcome != string(core.AuditDenied) {
		t.Fatalf("windowed list = %+v (err %v), want the single denied event", windowed, err)
	}

	// OldestFirst (Render's direction=forward, w4/013) is the exact mirror:
	// ASC total order, cursor resumes strictly NEWER than the cursor row.
	forward, err := s.ListAuditEvents(ctx, ten.ID, AuditFilter{OldestFirst: true})
	if err != nil || len(forward) != 3 {
		t.Fatalf("oldest-first list = %+v (err %v), want 3 rows", forward, err)
	}
	for i := range forward {
		if forward[i].ID != recorded[len(recorded)-1-i].ID {
			t.Fatalf("oldest-first order = %+v, want the newest-first list reversed (%+v)", forward, recorded)
		}
	}
	fwdPage2, err := s.ListAuditEvents(ctx, ten.ID, AuditFilter{OldestFirst: true, Limit: 1, Cursor: forward[0].ID})
	if err != nil || len(fwdPage2) != 1 || fwdPage2[0].ID != forward[1].ID {
		t.Fatalf("oldest-first second page = %+v (err %v), want [%s]", fwdPage2, err, forward[1].ID)
	}

	// An unknown cursor yields an empty page, not an error or the unfiltered list.
	if junk, err := s.ListAuditEvents(ctx, ten.ID, AuditFilter{Cursor: "aud-doesnotexist00000"}); err != nil || len(junk) != 0 {
		t.Fatalf("unknown cursor = %+v (err %v), want an empty page", junk, err)
	}

	// Retention is a global sweep, not workspace-scoped: purging everything
	// before base+2m removes ten's two oldest events AND the other
	// workspace's single (older) event — 3 rows total — leaving only ten's
	// newest event standing.
	purged, err := s.PurgeAuditEvents(ctx, base.Add(2*time.Minute))
	if err != nil {
		t.Fatalf("purge audit events: %v", err)
	}
	if purged != 3 {
		t.Fatalf("purged = %d, want 3 (ten's 2 oldest + the other workspace's 1 event)", purged)
	}
	remaining, err := s.ListAuditEvents(ctx, ten.ID, AuditFilter{})
	if err != nil || len(remaining) != 1 || remaining[0].ID != recorded[0].ID {
		t.Fatalf("remaining after purge = %+v (err %v), want [%s] (the newest event, at+2m, not < the purge boundary)", remaining, err, recorded[0].ID)
	}
}

// assertServiceEvents exercises w3/m7's composed feed against real Postgres —
// the parts that are Postgres's job, not Go's: the UNION ALL of two key spaces,
// the (at DESC, key DESC) total order across them, and the keyset cursor's
// stability when a row is inserted between two pages.
//
// It runs after assertDeployLifecycle, so `app` already has the two deploys that
// function left behind: #1 deactivated and #2 live. Both have a started and an
// ended event — four deploy events total.
func assertServiceEvents(ctx context.Context, t *testing.T, s *PGStore, ten Tenant, app App) {
	t.Helper()
	target := core.ServiceTarget(app.Name)
	verbs := []string{"apps.Suspend", "apps.Scale"}
	phases := []string{EventPhaseStarted, EventPhaseEnded}

	deploys, err := s.ListDeploys(ctx, app.ID, DeployFilter{})
	if err != nil || len(deploys) != 2 {
		t.Fatalf("precondition: deploys = %+v (err %v), want 2", deploys, err)
	}
	// Anchor the audit rows AFTER the deploy rows (whose timestamps are Postgres's
	// now()), so the expected newest-first order is known exactly.
	base := deploys[0].CreatedAt.Add(time.Second)

	record := func(at time.Time, verb, workspace, tgt string, outcome core.AuditOutcome) {
		t.Helper()
		if err := s.Record(ctx, core.AuditEvent{
			Caller: "user-x", CallerMethod: "session",
			Verb: verb, Resource: core.WorkspaceObject(workspace), Target: tgt, Outcome: outcome, At: at,
		}); err != nil {
			t.Fatalf("record %s: %v", verb, err)
		}
	}
	record(base, "apps.Suspend", ten.ID, target, core.AuditAllowed)
	record(base.Add(time.Second), "apps.Scale", ten.ID, target, core.AuditAllowed)
	// The three rows that must NOT reach the feed:
	//   denied      — an attempt is audit-log material, not something that happened.
	//   cross-tenant— a stranger's authorize passed against THEIR workspace before
	//                 GetApp rejected them; the row exists, but not in this feed.
	//   unmapped    — a verb the events vocabulary does not name (internal/events).
	record(base.Add(2*time.Second), "apps.Suspend", ten.ID, target, core.AuditDenied)
	record(base.Add(3*time.Second), "apps.Suspend", "tea-stranger00000000", target, core.AuditAllowed)
	record(base.Add(4*time.Second), "apps.Create", ten.ID, target, core.AuditAllowed)
	legacyTarget := core.ServiceTarget("shared-public-name")
	record(base.Add(5*time.Second), "apps.Suspend", core.DefaultTenant, legacyTarget, core.AuditAllowed)
	record(base.Add(6*time.Second), "apps.Suspend", ten.ID, legacyTarget, core.AuditAllowed)
	fact := ServiceEventFact{
		SourceKey: "git:delivery-1:" + app.ID + ":ignored", AppID: app.ID,
		Type: EventFactCommitIgnored, At: base.Add(500 * time.Millisecond),
		ReasonCode: EventReasonBuildFilter, CommitID: "abc123",
	}
	if inserted, err := s.InsertServiceEventFact(ctx, fact); err != nil || !inserted {
		t.Fatalf("insert event fact: %v", err)
	}
	if inserted, err := s.InsertServiceEventFact(ctx, fact); err != nil || inserted {
		t.Fatalf("retry event fact = (%v, %v), want idempotent no-op", inserted, err)
	}
	// A lifecycle-step fact carries its outcome in the checked status column
	// (w7/m66) — insert one and read it back to prove the column round-trips.
	// Filtered reads below all scope to commit_ignored/verbs/phases, so this
	// extra fact never perturbs their counts.
	buildFact := ServiceEventFact{
		SourceKey: "deploy:dep-1:build_ended", AppID: app.ID,
		Type: EventFactBuildEnded, At: base.Add(700 * time.Millisecond),
		DeployID: "dep-1", Status: EventStatusFailed,
	}
	if inserted, err := s.InsertServiceEventFact(ctx, buildFact); err != nil || !inserted {
		t.Fatalf("insert build_ended fact: %v", err)
	}
	cronFacts := []ServiceEventFact{
		{SourceKey: "cron:" + app.ID + ":run-1:started", AppID: app.ID, Type: EventFactCronRunStarted, At: base.Add(800 * time.Millisecond)},
		{SourceKey: "cron:" + app.ID + ":run-1:ended", AppID: app.ID, Type: EventFactCronRunEnded, At: base.Add(900 * time.Millisecond), Status: EventStatusSucceeded},
	}
	if err := s.InsertServiceEventFacts(ctx, cronFacts); err != nil {
		t.Fatalf("batch insert cron facts: %v", err)
	}
	if err := s.InsertServiceEventFacts(ctx, cronFacts); err != nil {
		t.Fatalf("batch replay cron facts: %v", err)
	}
	var cronFactCount int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM service_event_facts WHERE source_key = ANY($1)`,
		[]string{cronFacts[0].SourceKey, cronFacts[1].SourceKey}).Scan(&cronFactCount); err != nil || cronFactCount != 2 {
		t.Fatalf("batched cron fact count = %d (err %v), want exactly 2 after replay", cronFactCount, err)
	}
	buildOnly, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID,
		ServiceEventFilter{FactTypes: []string{string(EventFactBuildEnded)}})
	if err != nil || len(buildOnly) != 1 || buildOnly[0].FactStatus != EventStatusFailed {
		t.Fatalf("build_ended round-trip = %+v (err %v), want status=failed", buildOnly, err)
	}
	factTypes := []string{string(EventFactCommitIgnored)}

	all, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{Verbs: verbs, Phases: phases, FactTypes: factTypes})
	if err != nil {
		t.Fatalf("list service events: %v", err)
	}
	if len(all) != 7 {
		t.Fatalf("feed = %d events, want 7 (4 deploy + 2 audit + 1 fact; denied/cross-tenant/unmapped excluded)\n%+v", len(all), all)
	}
	// Newest first, and the order is TOTAL: every adjacent pair is strictly
	// ordered by (at, key), never merely equal — which is what makes the cursor
	// below resumable.
	for i := 1; i < len(all); i++ {
		prev, cur := all[i-1], all[i]
		if prev.At.Before(cur.At) || (prev.At.Equal(cur.At) && prev.Key <= cur.Key) {
			t.Fatalf("feed not in strict (at DESC, key DESC) order at %d: %+v then %+v", i, prev, cur)
		}
	}
	if all[0].Verb != "apps.Scale" || all[1].FactType != string(EventFactCommitIgnored) || all[2].Verb != "apps.Suspend" {
		t.Errorf("three newest = %+v, %+v, %+v; want scale, commit fact, suspend", all[0], all[1], all[2])
	}
	if all[0].Source != EventSourceAudit || all[0].Caller != "user-x" {
		t.Errorf("audit event = %+v, want source=audit caller=user-x", all[0])
	}
	// Both terminal deploys project their start and end transitions, including
	// the prior deploy's later deactivation.
	seenPhases := map[string]int{}
	for _, e := range all {
		if e.Source == EventSourceDeploy {
			seenPhases[e.Phase]++
		}
	}
	if seenPhases[EventPhaseStarted] != 2 || seenPhases[EventPhaseEnded] != 2 {
		t.Errorf("deploy phases = %v, want 2 started + 2 ended (deactivated and live)", seenPhases)
	}
	legacyOnly, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{
		Since: base.Add(5 * time.Second), Verbs: []string{"apps.Suspend"}, LegacyTarget: legacyTarget,
	})
	if err != nil || len(legacyOnly) != 1 || legacyOnly[0].At != base.Add(6*time.Second) {
		t.Fatalf("legacy target scope = %+v (err %v), want only the owner-workspace row", legacyOnly, err)
	}

	// Keyset paging: walk the feed 2 at a time and reassemble it exactly. Between
	// page 1 and page 2 a NEWER event lands — the page-2 cursor must not notice
	// (an OFFSET would have shifted and re-served a row here).
	page1, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{Verbs: verbs, Phases: phases, FactTypes: factTypes, Limit: 2})
	if err != nil || len(page1) != 2 {
		t.Fatalf("page 1 = %+v (err %v), want 2", page1, err)
	}
	record(base.Add(time.Minute), "apps.Suspend", ten.ID, target, core.AuditAllowed) // concurrent insert, newest
	after := page1[len(page1)-1]

	var walked []ServiceEventRow
	walked = append(walked, page1...)
	for {
		page, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{
			Verbs: verbs, Phases: phases, FactTypes: factTypes, Limit: 2, AfterAt: after.At, AfterKey: after.Key,
		})
		if err != nil {
			t.Fatalf("page after %s: %v", after.Key, err)
		}
		if len(page) == 0 {
			break
		}
		walked = append(walked, page...)
		after = page[len(page)-1]
	}
	if len(walked) != len(all) {
		t.Fatalf("paged walk = %d events, want the original %d (a row inserted mid-walk must not duplicate or drop one)", len(walked), len(all))
	}
	seen := map[string]bool{}
	for i, e := range walked {
		if seen[e.Key] {
			t.Fatalf("paged walk repeated %s", e.Key)
		}
		seen[e.Key] = true
		if e.Key != all[i].Key {
			t.Fatalf("paged walk diverged at %d: %s, want %s", i, e.Key, all[i].Key)
		}
	}

	// The window bounds `at` inclusively — two audit events plus the interleaved fact.
	windowed, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{
		Verbs: verbs, Phases: phases, FactTypes: factTypes, Since: base, Until: base.Add(time.Second),
	})
	if err != nil || len(windowed) != 3 {
		t.Fatalf("windowed feed = %+v (err %v), want 2 audit events + 1 fact", windowed, err)
	}

	// The type filter is a PUSH-DOWN: narrowing to one kind of event must bound the
	// SQL page, not the Go result. With limit=2 and only the deploy phases asked
	// for, both rows must come back deploy rows — a Go-side filter after the LIMIT
	// would have spent the page on the two newest (audit) rows and returned an empty
	// one, which a cursor client reads as the end of the feed.
	deploysOnly, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{
		Verbs: nil, Phases: []string{EventPhaseStarted}, Limit: 2,
	})
	if err != nil || len(deploysOnly) != 2 {
		t.Fatalf("type-filtered page = %+v (err %v), want 2 FULL rows", deploysOnly, err)
	}
	for _, e := range deploysOnly {
		if e.Source != EventSourceDeploy || e.Phase != EventPhaseStarted {
			t.Errorf("type-filtered page leaked %+v — the filter must run in SQL, before the LIMIT", e)
		}
	}
	// The converse: no phases ⇒ no deploy rows at all.
	auditOnly, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{Verbs: []string{"apps.Scale"}})
	if err != nil || len(auditOnly) != 1 || auditOnly[0].Verb != "apps.Scale" {
		t.Fatalf("verb-filtered feed = %+v (err %v), want just the scale", auditOnly, err)
	}
	factsOnly, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{FactTypes: factTypes, Limit: 1})
	if err != nil || len(factsOnly) != 1 || factsOnly[0].FactType != string(EventFactCommitIgnored) {
		t.Fatalf("fact-filtered feed = %+v (err %v), want just commit_ignored", factsOnly, err)
	}

	// A hand-applied app (no control-plane row) and a service nobody targeted:
	// an empty feed, never another service's rows.
	empty, err := s.ListServiceEvents(ctx, "srv-doesnotexist0000", core.ServiceTarget("nope"), ten.ID, ServiceEventFilter{Verbs: verbs, Phases: phases, FactTypes: factTypes})
	if err != nil || len(empty) != 0 {
		t.Fatalf("feed of an unknown service = %+v (err %v), want empty", empty, err)
	}
}

// assertWorkspaceLifecycle exercises the w6/m1 workspace store methods against
// real Postgres: atomic create (tenant + owner membership), get/rename, the
// per-subject and per-tenant counts, and delete cascading tenant_members.
func assertWorkspaceLifecycle(ctx context.Context, t *testing.T, s *PGStore, pool *pgxpool.Pool) {
	t.Helper()
	ws, err := s.CreateWorkspace(ctx, "workspace-a", PlanHobby, "user-x")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	// The owner membership landed in the same transaction.
	members, err := s.ListTenantMembers(ctx, ws.ID)
	if err != nil || len(members) != 1 || members[0].Subject != "user-x" || members[0].Role != "admin" {
		t.Fatalf("owner membership = %+v (err %v)", members, err)
	}
	if n, _ := s.CountTenantMembers(ctx, ws.ID); n != 1 {
		t.Errorf("member count = %d, want 1", n)
	}

	got, err := s.GetTenant(ctx, ws.ID)
	if err != nil || got.Name != "workspace-a" || got.Plan != PlanHobby {
		t.Fatalf("get tenant = %+v (err %v)", got, err)
	}
	renamed, err := s.RenameTenant(ctx, ws.ID, "workspace-a2")
	if err != nil || renamed.Name != "workspace-a2" {
		t.Fatalf("rename = %+v (err %v)", renamed, err)
	}

	// Per-subject plan count backs the 5-Hobby cap; the subject sees only their
	// workspaces.
	if n, _ := s.CountWorkspacesForSubjectPlan(ctx, "user-x", PlanHobby); n != 1 {
		t.Errorf("hobby count for user-x = %d, want 1", n)
	}
	if n, _ := s.CountWorkspacesForSubjectPlan(ctx, "user-x", PlanPro); n != 0 {
		t.Errorf("pro count for user-x = %d, want 0", n)
	}
	if list, _ := s.ListTenantsForSubject(ctx, "user-x"); len(list) != 1 || list[0].ID != ws.ID {
		t.Errorf("ListTenantsForSubject = %+v", list)
	}
	if list, _ := s.ListTenantsForSubject(ctx, "nobody"); len(list) != 0 {
		t.Errorf("ListTenantsForSubject(nobody) = %+v, want empty", list)
	}

	// Delete cascades the membership row (FK ON DELETE CASCADE).
	if err := s.DeleteTenant(ctx, ws.ID); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenant_members WHERE tenant_id = $1`, ws.ID).Scan(&remaining); err != nil {
		t.Fatal(err)
	}
	if remaining != 0 {
		t.Errorf("tenant_members not cascaded on delete: %d rows remain", remaining)
	}
	if err := s.DeleteTenant(ctx, ws.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("re-delete: want ErrNotFound, got %v", err)
	}
}

// TestTenantMintIdempotentAndRaceSafe exercises w1/m9's first-login mint
// against a real database: a second call for the same identity mints nothing,
// and N concurrent first logins for one identity — the actual race the unique
// partial index on owner_identity_id (not a check-then-insert) is meant to
// close — still yield exactly one tenant + one membership row.
func TestTenantMintIdempotentAndRaceSafe(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	first, err := s.CreateTenantWithMember(ctx, "identity-once", PlanHobby)
	if err != nil {
		t.Fatalf("first mint: %v", err)
	}
	second, err := s.CreateTenantWithMember(ctx, "identity-once", PlanHobby)
	if err != nil {
		t.Fatalf("second mint: %v", err)
	}
	if second.ID != first.ID {
		t.Errorf("repeat mint must return the same tenant: first=%s second=%s", first.ID, second.ID)
	}
	assertOneTenantOneMember(ctx, t, pool, "identity-once")

	// The actual race: N goroutines calling CreateTenantWithMember for the SAME
	// identity concurrently. The unique partial index on owner_identity_id (not
	// a Go-level check-then-insert) is what makes this converge to one row.
	const n = 20
	ids := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			ten, err := s.CreateTenantWithMember(ctx, "identity-racer", PlanHobby)
			if err != nil {
				t.Errorf("concurrent mint %d: %v", i, err)
				return
			}
			ids[i] = ten.ID
		}(i)
	}
	wg.Wait()
	for i, id := range ids {
		if id != ids[0] {
			t.Fatalf("concurrent mints diverged: goroutine 0 got %s, goroutine %d got %s", ids[0], i, id)
		}
	}
	assertOneTenantOneMember(ctx, t, pool, "identity-racer")
}

func assertOneTenantOneMember(ctx context.Context, t *testing.T, pool *pgxpool.Pool, identityID string) {
	t.Helper()
	var tenantCount, memberCount int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenants WHERE owner_identity_id = $1`, identityID).Scan(&tenantCount); err != nil {
		t.Fatal(err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenant_members WHERE subject = $1`, identityID).Scan(&memberCount); err != nil {
		t.Fatal(err)
	}
	if tenantCount != 1 || memberCount != 1 {
		t.Errorf("identity %s: tenants=%d members=%d, want 1,1", identityID, tenantCount, memberCount)
	}
}

// TestTenantForIdentityAndClient exercises the resolver's read path
// (tenant_members.subject, shared by human identities and API-key client ids)
// plus AddMember/BindClient/UnbindClient against a real database.
func TestOwnerIDForSubject(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE owner_ids`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	// First sight mints an own- id; a second call returns the SAME id (stable).
	a1, err := s.OwnerIDForSubject(ctx, "identity-a")
	if err != nil {
		t.Fatalf("OwnerIDForSubject: %v", err)
	}
	if _, ok := ids.KindOf(a1); !ok || a1[:4] != "own-" {
		t.Fatalf("own id = %q, want a well-formed own- id", a1)
	}
	a2, err := s.OwnerIDForSubject(ctx, "identity-a")
	if err != nil || a2 != a1 {
		t.Fatalf("not stable: %q then %q (err %v)", a1, a2, err)
	}
	// A different subject gets a distinct id.
	b1, err := s.OwnerIDForSubject(ctx, "identity-b")
	if err != nil || b1 == a1 {
		t.Fatalf("distinct subjects share an id: a=%q b=%q (err %v)", a1, b1, err)
	}
}

func TestTenantForIdentityAndClient(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	if _, err := s.TenantForIdentity(ctx, "nobody"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown identity: want ErrNotFound, got %v", err)
	}

	ten, err := s.CreateTenant(ctx, "platform-tenant", PlanHobby)
	if err != nil {
		t.Fatalf("create tenant: %v", err)
	}
	// AddMember is the platform tenant-create path's write (store/api.go) — the
	// membership row a resolver needs to map an admin identity to its workspace.
	if err := s.AddMember(ctx, "identity-admin", ten.ID, "admin"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := s.AddMember(ctx, "identity-admin", ten.ID, "admin"); err != nil {
		t.Fatalf("add member (idempotent repeat): %v", err)
	}
	got, err := s.TenantForIdentity(ctx, "identity-admin")
	if err != nil || got.ID != ten.ID {
		t.Fatalf("TenantForIdentity: %v %+v", err, got)
	}

	if _, err := s.TenantForIdentity(ctx, "client-unbound"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unbound client: want ErrNotFound, got %v", err)
	}
	if err := s.BindClient(ctx, "client-1", ten.ID); err != nil {
		t.Fatalf("bind client: %v", err)
	}
	got, err = s.TenantForIdentity(ctx, "client-1")
	if err != nil || got.ID != ten.ID {
		t.Fatalf("TenantForIdentity(bound client) after bind: %v %+v", err, got)
	}
	// Re-binding to a second tenant moves the binding (a key can be re-bound,
	// not just bound once) — BindClient deletes any prior row for this client
	// before inserting, since the PK is (tenant_id, subject) not subject alone.
	ten2, err := s.CreateTenant(ctx, "platform-tenant-2", PlanHobby)
	if err != nil {
		t.Fatalf("create second tenant: %v", err)
	}
	if err := s.BindClient(ctx, "client-1", ten2.ID); err != nil {
		t.Fatalf("re-bind client: %v", err)
	}
	got, err = s.TenantForIdentity(ctx, "client-1")
	if err != nil || got.ID != ten2.ID {
		t.Fatalf("TenantForIdentity(bound client) after re-bind: %v %+v", err, got)
	}

	if err := s.UnbindClient(ctx, "client-1"); err != nil {
		t.Fatalf("unbind client: %v", err)
	}
	if _, err := s.TenantForIdentity(ctx, "client-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unbound after UnbindClient: want ErrNotFound, got %v", err)
	}
	// Unbinding an already-unbound (or never-bound) client is a no-op, not an
	// error — the api-keys revoke path relies on this.
	if err := s.UnbindClient(ctx, "client-1"); err != nil {
		t.Errorf("re-unbind (idempotent): %v", err)
	}
	if err := s.UnbindClient(ctx, "client-never-bound"); err != nil {
		t.Errorf("unbind never-bound client (idempotent): %v", err)
	}
}

func assertErrorTaxonomy(ctx context.Context, t *testing.T, s *PGStore, ten Tenant) {
	t.Helper()
	if _, err := s.CreateTenant(ctx, "acme", PlanHobby); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate tenant: want ErrConflict, got %v", err)
	}
	if _, err := s.CreateTenant(ctx, "badplan", "platinum"); !errors.Is(err, ErrInvalid) {
		t.Errorf("invalid plan: want ErrInvalid (CHECK violation), got %v", err)
	}
	if _, err := s.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "x", Branch: "main", Port: 1, Replicas: 1, Tier: "free"}); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate app: want ErrConflict, got %v", err)
	}
	if _, err := s.CreateApp(ctx, App{TenantID: "tea-doesnotexist0000", Name: "x", Image: "x", Branch: "main", Port: 1, Replicas: 1, Tier: "free"}); !errors.Is(err, ErrNotFound) {
		t.Errorf("app with unknown tenant: want ErrNotFound, got %v", err)
	}
	if _, err := s.GetApp(ctx, "garbage-id"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get with unknown id: want ErrNotFound, got %v", err)
	}
}

// dnsLabelRE is a loose DNS-1123 label check (lowercase alphanumerics and
// interior hyphens) — good enough to catch a slug that broke the hostname
// contract without re-implementing RFC 1123 in the test.
var dnsLabelRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

// assertSlugMinting exercises w4/m19 t002: CreateApp mints apps.slug, the
// globally-unique public subdomain, alongside the workspace-scoped name. app
// (created at the top of TestPGStore, name "web") is the first-ever claimant
// of that name, so it holds the bare slug; this proves a second tenant
// claiming the SAME name gets a random "-xxxx" suffix instead of a conflict,
// and that a max-length name still yields a slug that fits a DNS label.
func assertSlugMinting(ctx context.Context, t *testing.T, s *PGStore, app App) {
	t.Helper()
	if app.Slug != app.Name {
		t.Errorf("first claimant of a free name: slug = %q, want bare name %q", app.Slug, app.Name)
	}

	other, err := s.CreateTenant(ctx, "slug-collider", PlanPro)
	if err != nil {
		t.Fatalf("create second tenant: %v", err)
	}
	collided, err := s.CreateApp(ctx, App{
		TenantID: other.ID, Name: app.Name, Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("cross-tenant same-name create: %v", err)
	}
	wantPrefix := app.Name + "-"
	if !strings.HasPrefix(collided.Slug, wantPrefix) || len(collided.Slug) != len(wantPrefix)+4 {
		t.Errorf("collided slug = %q, want %q + 4 random chars", collided.Slug, wantPrefix)
	}

	// Max-length name (30 chars, the ValidAppName cap): the suffixed slug (35
	// chars) must still be a valid DNS label — comfortably under the 63-char
	// limit a hostname combined with BEX_BASE_DOMAIN must respect.
	longName := strings.Repeat("a", 30)
	longFirst, err := s.CreateApp(ctx, App{
		TenantID: app.TenantID, Name: longName, Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create app with max-length name: %v", err)
	}
	longCollided, err := s.CreateApp(ctx, App{
		TenantID: other.ID, Name: longName, Image: "traefik/whoami",
		Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
	})
	if err != nil {
		t.Fatalf("create second app with max-length name: %v", err)
	}
	if len(longCollided.Slug) > 63 || !dnsLabelRE.MatchString(longCollided.Slug) {
		t.Errorf("suffixed max-length slug not a valid DNS label: %q", longCollided.Slug)
	}

	// Clean up the extra apps this helper minted — each opened a deploy row
	// (CreateApp's own invariant), and assertDeleteCascades later asserts the
	// deploys table is empty after it deletes the ONE app it knows about.
	for _, extra := range []App{collided, longFirst, longCollided} {
		if err := s.DeleteApp(ctx, extra.ID); err != nil {
			t.Fatalf("cleanup delete %s: %v", extra.ID, err)
		}
	}
}

func assertProjectionJoin(ctx context.Context, t *testing.T, s *PGStore, app App) {
	t.Helper()
	if _, err := s.CreateDomain(ctx, app.ID, "extra.example.com", false); err != nil {
		t.Fatalf("create domain: %v", err)
	}
	if _, err := s.CreateDomain(ctx, app.ID, "web.example.com", true); err != nil {
		t.Fatalf("create primary domain: %v", err)
	}
	desired, err := s.ListDesiredApps(ctx)
	if err != nil {
		t.Fatalf("list desired: %v", err)
	}
	if len(desired) != 1 {
		t.Fatalf("desired = %d rows, want 1", len(desired))
	}
	d := desired[0]
	if d.TenantName != "acme" || d.Image != "traefik/whoami" || d.Replicas != 2 {
		t.Errorf("desired row = %+v", d)
	}
	if d.PrimaryHost != "web.example.com" || len(d.Hosts) != 1 || d.Hosts[0] != "extra.example.com" {
		t.Errorf("primaryHost=%q hosts=%v", d.PrimaryHost, d.Hosts)
	}
}

// assertDomainUniqueness exercises w7/m6's cross-app collision guard against
// real Postgres. domains.host is globally UNIQUE, so AddDomain is idempotent only
// for the SAME app; a different app claiming a registered host must surface
// ErrConflict rather than being silently swallowed (the domainOwner check that
// distinguishes the two conflict cases). Creates its own apps and deletes them so
// the domains table is clean again for assertDeleteCascades' count==0.
func assertDomainUniqueness(ctx context.Context, t *testing.T, s *PGStore, tenantID string) {
	t.Helper()
	a1, err := s.CreateApp(ctx, App{TenantID: tenantID, Name: "dom-a", Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("create dom-a: %v", err)
	}
	a2, err := s.CreateApp(ctx, App{TenantID: tenantID, Name: "dom-b", Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatalf("create dom-b: %v", err)
	}
	defer func() { _ = s.DeleteApp(ctx, a1.ID); _ = s.DeleteApp(ctx, a2.ID) }()

	// Blueprint re-sync uses ReplaceDomains. Two replicas racing to claim one
	// host must have exactly one database winner, and the loser's DELETE+INSERT
	// transaction must roll back to its former domain set.
	if err := s.ReplaceDomains(ctx, a1.ID, "old-a.example.com", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceDomains(ctx, a2.ID, "old-b.example.com", nil); err != nil {
		t.Fatal(err)
	}
	start := make(chan struct{})
	errs := make([]error, 2)
	var wg sync.WaitGroup
	for i, appID := range []string{a1.ID, a2.ID} {
		wg.Add(1)
		go func(i int, appID string) {
			defer wg.Done()
			<-start
			errs[i] = s.ReplaceDomains(ctx, appID, "race.example.com", nil)
		}(i, appID)
	}
	close(start)
	wg.Wait()
	winner, loser := -1, -1
	for i, err := range errs {
		switch {
		case err == nil:
			winner = i
		case errors.Is(err, ErrConflict):
			loser = i
		default:
			t.Fatalf("concurrent ReplaceDomains %d: %v", i, err)
		}
	}
	if winner < 0 || loser < 0 {
		t.Fatalf("concurrent ReplaceDomains errors=%v, want one winner and one conflict", errs)
	}
	loserID := []string{a1.ID, a2.ID}[loser]
	loserOld := []string{"old-a.example.com", "old-b.example.com"}[loser]
	var preserved int
	if err := s.Pool.QueryRow(ctx, `SELECT count(*) FROM domains WHERE app_id = $1 AND host = $2`, loserID, loserOld).Scan(&preserved); err != nil || preserved != 1 {
		t.Fatalf("loser domain rollback count=%d err=%v", preserved, err)
	}
	if err := s.ReplaceDomains(ctx, a1.ID, "", nil); err != nil {
		t.Fatal(err)
	}
	if err := s.ReplaceDomains(ctx, a2.ID, "", nil); err != nil {
		t.Fatal(err)
	}

	if err := s.AddDomain(ctx, a1.ID, "shared.example.com", ""); err != nil {
		t.Fatalf("first AddDomain: %v", err)
	}
	// Same app, same host updates redirect metadata without adding a row.
	if err := s.AddDomain(ctx, a1.ID, "shared.example.com", "canonical.example.com"); err != nil {
		t.Errorf("same-app re-add must update redirect metadata, got %v", err)
	}
	var redirectForName string
	if err := s.Pool.QueryRow(ctx,
		`SELECT COALESCE(redirect_for_name, '') FROM domains WHERE app_id = $1 AND host = $2`,
		a1.ID, "shared.example.com").Scan(&redirectForName); err != nil {
		t.Fatalf("read updated redirect_for_name: %v", err)
	}
	if redirectForName != "canonical.example.com" {
		t.Fatalf("redirect_for_name = %q, want canonical.example.com", redirectForName)
	}
	// A different app claiming the same host → real cross-app collision, surfaced
	// (Render's "already exists on another site"), not swallowed.
	if err := s.AddDomain(ctx, a2.ID, "shared.example.com", ""); !errors.Is(err, ErrConflict) {
		t.Errorf("cross-app AddDomain => ErrConflict, got %v", err)
	}
}

// assertWebhooks exercises w3/m11's outbound-webhook store methods against
// real Postgres: endpoint CRUD (secret write-only past creation, enforced by
// the read column lists), cross-workspace scoping, the watermark's
// seed-once/advance semantics, the composed workspace-wide event feed
// (webhookEventsQuery — the ascending twin of the service-events view, with
// its tenant/app join and truthfulness predicates), and the delivery queue's
// due/record lifecycle including the enabled-join park.
func assertWebhooks(ctx context.Context, t *testing.T, s *PGStore, pool *pgxpool.Pool, ten Tenant, app App) {
	t.Helper()
	// The watermark is a singleton with no tenant FK, so the test's TRUNCATE
	// tenants CASCADE leaves it behind — clear it for a deterministic re-run.
	if _, err := pool.Exec(ctx, `TRUNCATE webhook_watermark`); err != nil {
		t.Fatal(err)
	}

	// --- endpoint CRUD + scoping ---
	ep, err := s.CreateWebhookEndpoint(ctx, ten.ID, "primary", "https://hooks.example.com/a", "whsec_secret-a", []string{"deploy_started", "deploy_ended"}, true, "user-x")
	if err != nil {
		t.Fatalf("create webhook endpoint: %v", err)
	}
	if ep.ID[:4] != "whk-" || ep.Secret != "whsec_secret-a" || ep.Name != "primary" || !ep.Enabled {
		t.Errorf("created endpoint = %+v", ep)
	}
	if _, err := s.CreateWebhookEndpoint(ctx, ten.ID, " PRIMARY ", "https://hooks.example.com/duplicate", "s", []string{}, false, ""); !errors.Is(err, ErrConflict) {
		t.Errorf("case-insensitive duplicate webhook name = %v, want ErrConflict", err)
	}
	ep2, err := s.CreateWebhookEndpoint(ctx, ten.ID, "secondary", "https://hooks.example.com/b", "whsec_secret-b", []string{}, false, "user-x")
	if err != nil {
		t.Fatalf("create second webhook endpoint: %v", err)
	}
	got, err := s.GetWebhookEndpoint(ctx, ten.ID, ep.ID)
	if err != nil {
		t.Fatalf("get webhook endpoint: %v", err)
	}
	if got.Secret != "" {
		t.Errorf("Get returned the secret %q — reads must never select it", got.Secret)
	}
	if len(got.EventTypes) != 2 || got.EventTypes[0] != "deploy_started" {
		t.Errorf("event types did not round-trip: %+v", got.EventTypes)
	}
	list, err := s.ListWebhookEndpoints(ctx, []string{ten.ID}, time.Time{}, "", 20)
	if err != nil || len(list) != 2 || list[0].Secret != "" || list[1].Secret != "" {
		t.Errorf("list = %+v (err %v), want 2 endpoints with no secret", list, err)
	}
	page1, err := s.ListWebhookEndpoints(ctx, []string{ten.ID}, time.Time{}, "", 1)
	if err != nil || len(page1) != 1 {
		t.Fatalf("endpoint page 1 = %+v (err %v)", page1, err)
	}
	page2, err := s.ListWebhookEndpoints(ctx, []string{ten.ID}, page1[0].CreatedAt, page1[0].ID, 1)
	if err != nil || len(page2) != 1 || page2[0].ID == page1[0].ID {
		t.Fatalf("endpoint page 2 = %+v (err %v), page 1 = %+v", page2, err, page1)
	}
	if _, err := s.GetWebhookEndpoint(ctx, "tea-stranger00000000", ep.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-workspace get = %v, want ErrNotFound", err)
	}
	if err := s.DeleteWebhookEndpoint(ctx, "tea-stranger00000000", ep.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-workspace delete = %v, want ErrNotFound", err)
	}
	if _, err := s.CreateWebhookEndpoint(ctx, "tea-doesnotexist0000", "x", "https://x", "s", []string{"deploy_started"}, true, ""); !errors.Is(err, ErrNotFound) {
		t.Errorf("create under a missing tenant = %v, want ErrNotFound (FK)", err)
	}

	disabled, err := s.SetWebhookEndpointEnabled(ctx, ten.ID, ep.ID, false, "manual")
	if err != nil || disabled.Enabled || disabled.DisabledReason != "manual" {
		t.Errorf("disable = %+v (err %v)", disabled, err)
	}
	enabled, err := s.SetWebhookEndpointEnabled(ctx, ten.ID, ep.ID, true, "ignored")
	if err != nil || !enabled.Enabled || enabled.DisabledReason != "" {
		t.Errorf("re-enable = %+v (err %v), want enabled with reason cleared", enabled, err)
	}

	// --- watermark: seeded once, later Ensure calls don't move it ---
	seed := time.Date(2026, 7, 14, 10, 0, 0, 0, time.UTC)
	wmAt, wmKey, err := s.EnsureWebhookWatermark(ctx, seed)
	if err != nil || !wmAt.Equal(seed) || wmKey != "" {
		t.Fatalf("watermark seed = (%v, %q, %v), want (%v, \"\")", wmAt, wmKey, err, seed)
	}
	wmAt2, _, err := s.EnsureWebhookWatermark(ctx, seed.Add(time.Hour))
	if err != nil || !wmAt2.Equal(seed) {
		t.Errorf("second Ensure moved the watermark to %v; it must seed only once", wmAt2)
	}

	// --- the composed workspace-wide feed ---
	// The app has deploys + audit rows from earlier assertions; add one row of
	// each exclusion class to prove the truthfulness predicates carry over.
	// An audit row's target carries whatever name the caller passed
	// (core.AuthorizeApp), which has two legitimate spellings (w4/m19): the
	// full service id "<tenantName>-<appName>" (the common one) and the bare
	// app name (the LabelServiceName fallback) — the feed must match both.
	fullTarget := core.ServiceTarget(core.CRName(ten.Name, app.Name))
	bareTarget := core.ServiceTarget(app.Name)
	// PostgreSQL stores timestamptz at microsecond precision. Keep occurrence
	// fixtures on that boundary so equality assertions test persisted values,
	// not Go-only nanoseconds discarded by the database.
	at := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Minute)
	recordAudit := func(atRow time.Time, verb, workspace, target, targetName string, outcome core.AuditOutcome) {
		t.Helper()
		if err := s.Record(ctx, core.AuditEvent{
			Caller: "user-x", Verb: verb, Resource: core.WorkspaceObject(workspace),
			Target: target, TargetName: targetName, Outcome: outcome, At: atRow,
		}); err != nil {
			t.Fatalf("record %s: %v", verb, err)
		}
	}
	recordAudit(at, "apps.Restart", ten.ID, fullTarget, "", core.AuditAllowed)
	recordAudit(at.Add(time.Second), "apps.Restart", ten.ID, bareTarget, "", core.AuditAllowed)
	recordAudit(at.Add(2*time.Second), "apps.Restart", ten.ID, fullTarget, "", core.AuditDenied)                  // denied: excluded
	recordAudit(at.Add(3*time.Second), "apps.Restart", "tea-stranger00000000", fullTarget, "", core.AuditAllowed) // cross-tenant: excluded
	recordAudit(at.Add(4*time.Second), "apps.SetRoutes", ten.ID, fullTarget, "", core.AuditAllowed)               // verb not pushed down: excluded
	recordAudit(at.Add(5*time.Second), core.AuditVerbPostgresCreated, ten.ID, core.DatabaseTarget("dpg-orders"), "orders", core.AuditAllowed)
	lateOccurrence := at.Add(-time.Hour)
	if inserted, err := s.InsertServiceEventFact(ctx, ServiceEventFact{
		SourceKey: "observed:late-recovery", AppID: app.ID,
		Type: EventFactServerAvailable, At: lateOccurrence,
	}); err != nil || !inserted {
		t.Fatalf("insert late-observed fact = (%v, %v)", inserted, err)
	}

	rows, err := s.ListWebhookEvents(ctx, at.Add(-time.Second), "", time.Now().UTC().Add(time.Hour), []string{"apps.Restart", core.AuditVerbPostgresCreated}, []string{ten.ID}, 100)
	if err != nil {
		t.Fatalf("list webhook events: %v", err)
	}
	var restarts int
	var postgresCreates int
	var recoveries int
	for _, r := range rows {
		if r.Source == EventSourceFact {
			if r.FactType == string(EventFactServerAvailable) {
				if !r.At.Equal(lateOccurrence) || !r.CursorAt.After(r.At) {
					t.Errorf("late-observed fact = %+v", r)
				}
				// The app-joined arms carry the internal app id, so the push
				// dispatcher resolves nothing per row.
				if r.AppID != app.ID {
					t.Errorf("fact row carried app id %q, want %q", r.AppID, app.ID)
				}
				recoveries++
			}
			continue
		}
		if r.Source != EventSourceAudit {
			continue
		}
		switch r.Verb {
		case "apps.Restart":
			if r.TenantID != ten.ID || r.ServiceID != core.CRName(ten.Name, app.Name) || r.ServiceName != app.Name {
				t.Errorf("unexpected audit row in feed: %+v", r)
			}
			if r.AppID != app.ID {
				t.Errorf("audit row carried app id %q, want %q", r.AppID, app.ID)
			}
			restarts++
		case core.AuditVerbPostgresCreated:
			if r.TenantID != ten.ID || r.ServiceID != "dpg-orders" || r.ServiceName != "orders" {
				t.Errorf("unexpected datastore audit row in feed: %+v", r)
			}
			// The datastore arm has no apps join, so it carries no app id.
			if r.AppID != "" {
				t.Errorf("datastore audit row carried app id %q, want empty", r.AppID)
			}
			postgresCreates++
		default:
			t.Errorf("unexpected audit verb in feed: %+v", r)
		}
	}
	if restarts != 2 {
		t.Errorf("feed carried %d apps.Restart events, want exactly 2 (both target spellings; denied/cross-tenant/unmapped excluded)\n%+v", restarts, rows)
	}
	if postgresCreates != 1 {
		t.Errorf("feed carried %d postgres.CreatePostgres events, want exactly 1\n%+v", postgresCreates, rows)
	}
	if recoveries != 1 {
		t.Errorf("feed carried %d late recovery facts, want 1\n%+v", recoveries, rows)
	}
	// Ascending keyset: rows must come back oldest-first and resume exactly.
	if len(rows) >= 2 {
		for i := 1; i < len(rows); i++ {
			if rows[i-1].CursorAt.After(rows[i].CursorAt) {
				t.Errorf("feed not ascending by dispatch cursor: %v then %v", rows[i-1].CursorAt, rows[i].CursorAt)
			}
		}
		resumed, err := s.ListWebhookEvents(ctx, rows[0].CursorAt, rows[0].Key, time.Now().UTC().Add(time.Hour), []string{"apps.Restart", core.AuditVerbPostgresCreated}, []string{ten.ID}, 100)
		if err != nil || len(resumed) != len(rows)-1 {
			t.Errorf("keyset resume = %d rows (err %v), want %d", len(resumed), err, len(rows)-1)
		}
	}

	// --- delivery queue lifecycle ---
	now := time.Now().UTC().Truncate(time.Microsecond) // timestamptz keeps microseconds
	d := WebhookDelivery{
		ID: "whd-testdelivery00000", EndpointID: ep.ID, EventID: "evt-testevent00000000",
		EventType: "deploy_started", ServiceID: app.Name, Payload: `{"type":"deploy_started"}`,
		NextAttemptAt: now,
	}
	if err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{d}, now, "advance-key"); err != nil {
		t.Fatalf("enqueue: %v", err)
	}
	wmAt3, wmKey3, err := s.EnsureWebhookWatermark(ctx, time.Now())
	if err != nil || !wmAt3.Equal(now) || wmKey3 != "advance-key" {
		t.Errorf("watermark after enqueue = (%v, %q, %v), want (%v, advance-key)", wmAt3, wmKey3, err, now)
	}
	due, err := s.ClaimDueWebhookDeliveries(ctx, now.Add(time.Second), now.Add(time.Second+time.Minute), 10)
	if err != nil || len(due) != 1 {
		t.Fatalf("due = %+v (err %v), want the one enqueued delivery", due, err)
	}
	if due[0].Secret != "whsec_secret-a" || due[0].URL != "https://hooks.example.com/a" || due[0].CreatedBy != "user-x" {
		t.Errorf("due join = %+v, want the endpoint's secret/url/creator", due[0])
	}
	// A failed attempt reschedules; the row stays open but future-dated.
	if err := s.RecordWebhookAttempt(ctx, d.ID, 502, "bad gateway", "upstream down", now, now.Add(time.Hour), false, false); err != nil {
		t.Fatalf("record attempt: %v", err)
	}
	if due, _ := s.ClaimDueWebhookDeliveries(ctx, now.Add(time.Second), now.Add(time.Second+time.Minute), 10); len(due) != 0 {
		t.Errorf("rescheduled delivery must not be due, got %+v", due)
	}
	// Disabling the endpoint parks the queue even when the row is due.
	if err := s.DisableWebhookEndpoint(ctx, ep.ID, "auto"); err != nil {
		t.Fatalf("disable: %v", err)
	}
	if due, _ := s.ClaimDueWebhookDeliveries(ctx, now.Add(2*time.Hour), now.Add(2*time.Hour+time.Minute), 10); len(due) != 0 {
		t.Errorf("a disabled endpoint's deliveries must not be due, got %+v", due)
	}
	if _, err := s.SetWebhookEndpointEnabled(ctx, ten.ID, ep.ID, true, ""); err != nil {
		t.Fatalf("re-enable: %v", err)
	}
	// A delivered attempt closes the row.
	if err := s.RecordWebhookAttempt(ctx, d.ID, 200, "", "ok", now.Add(2*time.Hour), now.Add(2*time.Hour), true, false); err != nil {
		t.Fatalf("record delivered: %v", err)
	}
	history, err := s.ListWebhookDeliveries(ctx, WebhookDeliveryFilter{EndpointID: ep.ID, Limit: 10})
	if err != nil || len(history) != 1 {
		t.Fatalf("history = %+v (err %v)", history, err)
	}
	h := history[0]
	if h.AttemptCount != 2 || h.DeliveredAt == nil || h.LastStatus != 200 {
		t.Errorf("history row = %+v, want 2 attempts, delivered, 200", h)
	}
	failedAt := now.Add(3 * time.Hour)
	failedDelivery := WebhookDelivery{
		ID: "whd-failedhistory000", EndpointID: ep.ID, EventID: "evt-failedhistory0000",
		EventType: "deploy_ended", ServiceID: app.Name, Payload: `{}`, NextAttemptAt: failedAt,
	}
	if err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{failedDelivery}, failedAt, "failed-key"); err != nil {
		t.Fatalf("enqueue failed history fixture: %v", err)
	}
	if err := s.RecordWebhookAttempt(ctx, failedDelivery.ID, 503, "unavailable", "try later", failedAt, failedAt, false, true); err != nil {
		t.Fatalf("record terminal failure: %v", err)
	}
	deliveredHistory, err := s.ListWebhookDeliveries(ctx, WebhookDeliveryFilter{EndpointID: ep.ID, Status: "delivered", Limit: 10})
	if err != nil || len(deliveredHistory) != 1 || deliveredHistory[0].ID != d.ID {
		t.Fatalf("delivered history = %+v (err %v)", deliveredHistory, err)
	}
	failedHistory, err := s.ListWebhookDeliveries(ctx, WebhookDeliveryFilter{
		EndpointID: ep.ID, SentAfter: now.Add(2 * time.Hour), SentBefore: now.Add(4 * time.Hour), Status: "failed", Limit: 10,
	})
	if err != nil || len(failedHistory) != 1 || failedHistory[0].ID != failedDelivery.ID || failedHistory[0].ResponseBody != "try later" {
		t.Fatalf("time/status-filtered failed history = %+v (err %v)", failedHistory, err)
	}
	if ep2.Enabled {
		t.Fatal("secondary pagination fixture must remain disabled so it cannot receive fan-out")
	}

	// --- w1/m58 multi-replica correctness ---
	cnow := time.Now().UTC().Truncate(time.Microsecond) // timestamptz microsecond boundary
	// Dispatch dedup: re-dispatching the same (endpoint, event) — what a second
	// replica or a crash-replay does — inserts no duplicate delivery, even with a
	// fresh delivery id, thanks to the (endpoint_id, event_id) unique index +
	// ON CONFLICT DO NOTHING.
	dupA := WebhookDelivery{ID: ids.New(ids.WebhookDelivery), EndpointID: ep.ID, EventID: "evt-dedup-0000000000", EventType: "deploy_started", ServiceID: app.Name, Payload: `{}`, NextAttemptAt: cnow}
	dupB := dupA
	dupB.ID = ids.New(ids.WebhookDelivery) // different delivery id, SAME (endpoint, event)
	if err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{dupA}, cnow, "k1"); err != nil {
		t.Fatalf("enqueue dupA: %v", err)
	}
	if err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{dupB}, cnow, "k2"); err != nil {
		t.Fatalf("enqueue dupB: %v", err)
	}
	var nDup int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE endpoint_id=$1 AND event_id=$2`, ep.ID, "evt-dedup-0000000000").Scan(&nDup); err != nil || nDup != 1 {
		t.Fatalf("dedup: (endpoint, event) has %d rows (err %v), want exactly 1 (ON CONFLICT DO NOTHING)", nDup, err)
	}

	// SKIP LOCKED disjoint claim: enqueue a batch of due deliveries, then claim
	// concurrently from several goroutines (two replicas' send passes). Every row
	// must be claimed by EXACTLY ONE claimer — never two — proving no double-send.
	const nRows = 24
	batch := make([]WebhookDelivery, 0, nRows)
	for i := 0; i < nRows; i++ {
		batch = append(batch, WebhookDelivery{
			ID: ids.New(ids.WebhookDelivery), EndpointID: ep.ID,
			EventID:   ids.New(ids.WebhookDelivery), // arbitrary unique per-row event token
			EventType: "deploy_started", ServiceID: app.Name, Payload: `{}`, NextAttemptAt: cnow,
		})
	}
	if err := s.EnqueueWebhookDeliveries(ctx, batch, cnow, "k3"); err != nil {
		t.Fatalf("enqueue claim batch: %v", err)
	}
	var mu sync.Mutex
	claimedBy := map[string]int{} // delivery id -> claimer index
	var wg sync.WaitGroup
	for g := 0; g < 4; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			got, err := s.ClaimDueWebhookDeliveries(ctx, cnow.Add(time.Second), cnow.Add(time.Minute), nRows)
			if err != nil {
				t.Errorf("claim g%d: %v", g, err)
				return
			}
			mu.Lock()
			defer mu.Unlock()
			for _, d := range got {
				if prev, dup := claimedBy[d.ID]; dup {
					t.Errorf("delivery %s double-claimed by g%d and g%d (SKIP LOCKED broken)", d.ID, prev, g)
				}
				claimedBy[d.ID] = g
			}
		}(g)
	}
	wg.Wait()
	claimed := 0
	for _, d := range batch {
		if _, ok := claimedBy[d.ID]; ok {
			claimed++
		}
	}
	if claimed != nRows {
		t.Errorf("SKIP LOCKED claim covered %d/%d rows; every due row must be claimed exactly once", claimed, nRows)
	}

	// Lease re-visibility: a claimed-but-unacked row (a crashed send) is invisible
	// while leased and becomes due again after the lease, so it retries at-least-once
	// rather than being lost.
	lease := WebhookDelivery{ID: ids.New(ids.WebhookDelivery), EndpointID: ep.ID, EventID: "evt-lease-0000000000", EventType: "deploy_started", ServiceID: app.Name, Payload: `{}`, NextAttemptAt: cnow}
	if err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{lease}, cnow, "k4"); err != nil {
		t.Fatalf("enqueue lease: %v", err)
	}
	leaseUntil := cnow.Add(30 * time.Second)
	if got, err := s.ClaimDueWebhookDeliveries(ctx, cnow.Add(time.Second), leaseUntil, 100); err != nil || !containsDelivery(got, lease.ID) {
		t.Fatalf("lease claim did not return the row (err %v)", err)
	}
	if got, _ := s.ClaimDueWebhookDeliveries(ctx, leaseUntil.Add(-time.Second), leaseUntil.Add(time.Minute), 100); containsDelivery(got, lease.ID) {
		t.Error("leased row must not be re-claimable before the lease expires")
	}
	if got, err := s.ClaimDueWebhookDeliveries(ctx, leaseUntil.Add(time.Second), leaseUntil.Add(time.Minute), 100); err != nil || !containsDelivery(got, lease.ID) {
		t.Fatalf("expired lease must be re-claimable (err %v)", err)
	}

	// notified_at CAS: exactly one failure notice per endpoint per window across
	// replicas/restarts, and re-enabling clears it so the next cycle emails once more.
	win := time.Hour
	t0 := time.Now().UTC()
	if ok, err := s.ClaimWebhookFailureNotice(ctx, ep.ID, t0, t0.Add(-win)); err != nil || !ok {
		t.Fatalf("first failure-notice claim must succeed (err %v)", err)
	}
	if ok, _ := s.ClaimWebhookFailureNotice(ctx, ep.ID, t0.Add(time.Minute), t0.Add(time.Minute-win)); ok {
		t.Error("second claim within the window must be suppressed (no re-email on restart/second replica)")
	}
	if ok, _ := s.ClaimWebhookFailureNotice(ctx, ep.ID, t0.Add(2*win), t0.Add(win)); !ok {
		t.Error("a claim past the suppression window must succeed again")
	}
	if _, err := s.SetWebhookEndpointEnabled(ctx, ten.ID, ep.ID, false, "manual"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetWebhookEndpointEnabled(ctx, ten.ID, ep.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimWebhookFailureNotice(ctx, ep.ID, t0.Add(3*win), t0.Add(2*win)); err != nil || !ok {
		t.Fatalf("re-enable must clear notified_at so a notice can send again (err %v)", err)
	}

	// Sweep age-out of an abandoned PENDING row (scan finding #3). A disabled
	// endpoint's open deliveries are never claimed (ClaimDue requires e.enabled)
	// nor terminalized, so without an age-out they park forever and grow the
	// shared table without bound. The sweep must reclaim a pending row older than
	// the retention floor, while a RECENT pending row survives (park-and-resume).
	parked := WebhookDelivery{
		ID: ids.New(ids.WebhookDelivery), EndpointID: ep.ID, EventID: "evt-parked-000000000",
		EventType: "deploy_started", ServiceID: app.Name, Payload: `{}`, NextAttemptAt: cnow,
	}
	if err := s.EnqueueWebhookDeliveries(ctx, []WebhookDelivery{parked}, cnow, "k-parked"); err != nil {
		t.Fatalf("enqueue parked: %v", err)
	}
	if n, err := s.SweepWebhookDeliveries(ctx, cnow.Add(-time.Hour), 1000, 100); err != nil || n != 0 {
		t.Fatalf("recent pending row swept (n=%d, err %v); park-and-resume must survive the retention floor", n, err)
	}
	if !deliveryExists(ctx, pool, parked.ID) {
		t.Fatal("recent pending delivery was reclaimed before the retention floor")
	}
	if n, err := s.SweepWebhookDeliveries(ctx, cnow.Add(time.Hour), 1000, 100); err != nil || n < 1 {
		t.Fatalf("aged pending row not swept (n=%d, err %v), want >=1", n, err)
	}
	if deliveryExists(ctx, pool, parked.ID) {
		t.Fatal("abandoned pending delivery survived the age-out sweep")
	}

	// Deleting the endpoint cascades its deliveries.
	if err := s.DeleteWebhookEndpoint(ctx, ten.ID, ep.ID); err != nil {
		t.Fatalf("delete endpoint: %v", err)
	}
	var nDeliveries int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE endpoint_id = $1`, ep.ID).Scan(&nDeliveries); err != nil || nDeliveries != 0 {
		t.Errorf("deliveries after endpoint delete = %d (err %v), want 0 (cascade)", nDeliveries, err)
	}
}

func containsDelivery(due []DueWebhookDelivery, id string) bool {
	for _, d := range due {
		if d.ID == id {
			return true
		}
	}
	return false
}

func deliveryExists(ctx context.Context, pool *pgxpool.Pool, id string) bool {
	var n int
	_ = pool.QueryRow(ctx, `SELECT count(*) FROM webhook_deliveries WHERE id=$1`, id).Scan(&n)
	return n > 0
}

func assertDeleteCascades(ctx context.Context, t *testing.T, s *PGStore, pool *pgxpool.Pool, app App) {
	t.Helper()
	if err := s.DeleteApp(ctx, app.ID); err != nil {
		t.Fatalf("delete app: %v", err)
	}
	if err := s.DeleteApp(ctx, app.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("re-delete: want ErrNotFound, got %v", err)
	}
	// Domains and deploys cascade with their app.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM domains`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("domains after app delete = %d, want 0 (cascade)", n)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM deploys`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("deploys after app delete = %d, want 0 (cascade)", n)
	}
}

// TestMembersAndInvites exercises w4/m12's write side against a real database:
// role changes, removal, the last-admin counter, and the invite lifecycle
// (create → list pending → redeem, with the cascade and the accepted/expired
// exclusions).
func TestMembersAndInvites(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	ten, err := s.CreateWorkspace(ctx, "acme", PlanPro, "admin-1")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Role change + last-admin counter.
	if n, err := s.CountTenantAdmins(ctx, ten.ID); err != nil || n != 1 {
		t.Fatalf("admins = %d (%v), want 1", n, err)
	}
	if err := s.AddMember(ctx, "bob", ten.ID, "viewer"); err != nil {
		t.Fatalf("add member: %v", err)
	}
	if err := s.UpdateMemberRole(ctx, ten.ID, "bob", "developer"); err != nil {
		t.Fatalf("update role: %v", err)
	}
	if m, err := s.GetTenantMember(ctx, ten.ID, "bob"); err != nil || m.Role != "developer" {
		t.Fatalf("member role = %q (%v), want developer", m.Role, err)
	}
	if err := s.UpdateMemberRole(ctx, ten.ID, "ghost", "developer"); !errors.Is(err, ErrNotFound) {
		t.Errorf("update absent member: want ErrNotFound, got %v", err)
	}
	if err := s.RemoveMember(ctx, ten.ID, "bob"); err != nil {
		t.Fatalf("remove member: %v", err)
	}
	if err := s.RemoveMember(ctx, ten.ID, "bob"); !errors.Is(err, ErrNotFound) {
		t.Errorf("re-remove: want ErrNotFound, got %v", err)
	}

	// Invite lifecycle. The role must be one the workspace's plan actually offers
	// (Pro: admin/developer) — AcceptInvitesForEmail enforces the plan at accept
	// time (w6/m13), so a contributor invite on a Pro workspace, a state the
	// members service's invite-time guard would never mint anyway, is left pending.
	exp := time.Now().Add(24 * time.Hour)
	inv, err := s.CreateInvite(ctx, ten.ID, "carol@example.com", "developer", "tok", "admin-1", exp)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if _, err := s.CreateInvite(ctx, ten.ID, "carol@example.com", "viewer", "tok2", "admin-1", exp); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate outstanding invite: want ErrConflict, got %v", err)
	}
	pending, err := s.ListInvites(ctx, ten.ID)
	if err != nil || len(pending) != 1 {
		t.Fatalf("pending invites = %d (%v), want 1", len(pending), err)
	}

	// Redeem: carol's login turns the invite into a membership at its role and
	// marks the invite accepted (so it drops off the pending list).
	accepted, err := s.AcceptInvitesForEmail(ctx, "carol@example.com", "identity-carol")
	if err != nil || len(accepted) != 1 || accepted[0].ID != inv.ID {
		t.Fatalf("accept: %v %+v", err, accepted)
	}
	if m, err := s.GetTenantMember(ctx, ten.ID, "identity-carol"); err != nil || m.Role != "developer" {
		t.Fatalf("redeemed member role = %q (%v), want developer", m.Role, err)
	}
	if pending, _ := s.ListInvites(ctx, ten.ID); len(pending) != 0 {
		t.Errorf("pending after accept = %d, want 0", len(pending))
	}
	// A second login redeems nothing (idempotent).
	if again, err := s.AcceptInvitesForEmail(ctx, "carol@example.com", "identity-carol"); err != nil || len(again) != 0 {
		t.Errorf("second accept: %v %+v, want none", err, again)
	}

	// Deleting the workspace cascades its invites.
	if err := s.DeleteTenant(ctx, ten.ID); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenant_invites`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("invites after tenant delete = %d, want 0 (cascade)", n)
	}
}

// TestInviteResendAndTokenAcceptance exercises the w1/m33 additions against a
// real database: RefreshInvite (expiry pushed, token ROTATED since w1/041 —
// the old link dies, the fresh one redeems; revives a lapsed invite, 404 on
// accepted) and AcceptInviteByToken (cross-email join, named
// already-accepted/expired refusals, plan-seat refusal on a full Hobby
// workspace) — plus the w1/041 at-rest contract: only sha256(token) is stored,
// and reads never surface it.
func TestInviteResendAndTokenAcceptance(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	ten, err := s.CreateWorkspace(ctx, "acme", PlanPro, "admin-1")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// w1/041 at-rest contract: the row holds sha256("tok-late"), never the
	// plaintext (the plaintext column is gone), and reads don't surface it.
	lapsed := time.Now().Add(-time.Hour)
	inv, err := s.CreateInvite(ctx, ten.ID, "late@example.com", "developer", "tok-late", "admin-1", lapsed)
	if err != nil {
		t.Fatalf("create invite: %v", err)
	}
	if inv.Token != "tok-late" {
		t.Errorf("create must hand back the plaintext for the mail, got %q", inv.Token)
	}
	var atRest string
	wantHash := hashInviteToken("tok-late")
	if err := pool.QueryRow(ctx, `SELECT token_hash FROM tenant_invites WHERE id = $1`, inv.ID).Scan(&atRest); err != nil {
		t.Fatalf("read token_hash: %v", err)
	}
	if atRest != wantHash {
		t.Errorf("token_hash at rest = %q, want sha256 hex %q", atRest, wantHash)
	}
	if got, err := s.GetInvite(ctx, ten.ID, inv.ID); err != nil || got.Token != "" {
		t.Errorf("GetInvite token = %q (%v), want empty — reads never surface the capability", got.Token, err)
	}

	// RefreshInvite: expiry moves and the token ROTATES (w1/041 — the hash at
	// rest can't reproduce the old plaintext for the resent mail); the id is
	// stable and a LAPSED (expired, unaccepted) invite is revived.
	if pending, _ := s.ListInvites(ctx, ten.ID); len(pending) != 0 {
		t.Fatalf("expired invite still pending: %d", len(pending))
	}
	fresh := time.Now().Add(48 * time.Hour)
	resent, err := s.RefreshInvite(ctx, ten.ID, inv.ID, "tok-rotated", fresh)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if resent.ID != inv.ID || resent.Token != "tok-rotated" {
		t.Errorf("refresh churned identity or dropped the fresh token: %+v", resent)
	}
	if pending, _ := s.ListInvites(ctx, ten.ID); len(pending) != 1 {
		t.Errorf("revived invite not pending: %d", len(pending))
	}
	if _, err := s.RefreshInvite(ctx, "tea-other", inv.ID, "tok-x", fresh); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-workspace refresh: want ErrNotFound, got %v", err)
	}
	// The superseded link is dead: only the freshly minted token redeems.
	if _, err := s.AcceptInviteByToken(ctx, "tok-late", "identity-newcomer"); !errors.Is(err, ErrNotFound) {
		t.Errorf("superseded token: want ErrNotFound, got %v", err)
	}

	// AcceptInviteByToken: the recipient signed up under a different email —
	// the token is the capability; the membership lands at the invited role.
	acc, err := s.AcceptInviteByToken(ctx, "tok-rotated", "identity-newcomer")
	if err != nil {
		t.Fatalf("accept by token: %v", err)
	}
	if acc.ID != inv.ID || acc.TenantID != ten.ID {
		t.Errorf("accepted = %+v", acc)
	}
	if m, err := s.GetTenantMember(ctx, ten.ID, "identity-newcomer"); err != nil || m.Role != "developer" {
		t.Fatalf("member after token accept = %+v (%v)", m, err)
	}
	// Named refusals: second redemption, refresh of an accepted invite, unknown
	// and expired tokens.
	if _, err := s.AcceptInviteByToken(ctx, "tok-rotated", "identity-other"); !errors.Is(err, ErrConflict) {
		t.Errorf("second redemption: want ErrConflict, got %v", err)
	}
	if _, err := s.RefreshInvite(ctx, ten.ID, inv.ID, "tok-y", fresh); !errors.Is(err, ErrNotFound) {
		t.Errorf("refresh accepted invite: want ErrNotFound, got %v", err)
	}
	if _, err := s.AcceptInviteByToken(ctx, "tok-ghost", "identity-x"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown token: want ErrNotFound, got %v", err)
	}
	if _, err := s.CreateInvite(ctx, ten.ID, "expired@example.com", "developer", "tok-exp", "admin-1", time.Now().Add(-time.Minute)); err != nil {
		t.Fatalf("create expired invite: %v", err)
	}
	if _, err := s.AcceptInviteByToken(ctx, "tok-exp", "identity-x"); !errors.Is(err, ErrConflict) {
		t.Errorf("expired token: want ErrConflict, got %v", err)
	}

	// Plan-seat refusal: a full Hobby workspace refuses the token redemption
	// (named), unlike the login path's silent skip.
	hobby, err := s.CreateWorkspace(ctx, "solo", PlanHobby, "owner-1")
	if err != nil {
		t.Fatalf("create hobby workspace: %v", err)
	}
	if _, err := s.CreateInvite(ctx, hobby.ID, "second@example.com", "admin", "tok-full", "owner-1", time.Now().Add(time.Hour)); err != nil {
		t.Fatalf("create hobby invite: %v", err)
	}
	if _, err := s.AcceptInviteByToken(ctx, "tok-full", "identity-second"); !errors.Is(err, ErrConflict) {
		t.Errorf("full hobby workspace: want ErrConflict, got %v", err)
	}
}

// TestClaimShellNonce verifies the cross-replica web-shell single-use claim
// (w1/042 L7) against a real database: first claim wins, a second claim of the
// same nonce loses (any replica — it's one table), and expired rows are pruned
// by the next claim rather than accreting.
func TestClaimShellNonce(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE shell_ticket_nonces`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	exp := time.Now().Add(90 * time.Second)
	if ok, err := s.ClaimShellNonce(ctx, "nonce-1", exp); err != nil || !ok {
		t.Fatalf("first claim = (%v, %v), want (true, nil)", ok, err)
	}
	if ok, err := s.ClaimShellNonce(ctx, "nonce-1", exp); err != nil || ok {
		t.Fatalf("second claim = (%v, %v), want (false, nil)", ok, err)
	}

	// An expired nonce is pruned by the next claim, freeing its row (the ticket
	// itself is already unredeemable — shellticket.Verify enforces expiry).
	if _, err := pool.Exec(ctx,
		`INSERT INTO shell_ticket_nonces (nonce, expires_at) VALUES ('nonce-old', now() - interval '1 minute')`); err != nil {
		t.Fatal(err)
	}
	if ok, err := s.ClaimShellNonce(ctx, "nonce-2", exp); err != nil || !ok {
		t.Fatalf("claim after prune = (%v, %v), want (true, nil)", ok, err)
	}
	var stale int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM shell_ticket_nonces WHERE nonce = 'nonce-old'`).Scan(&stale); err != nil {
		t.Fatal(err)
	}
	if stale != 0 {
		t.Errorf("expired nonce not pruned")
	}
}

// TestCLIRefreshStore proves the hash-keyed response store round-trips exact
// bytes while live and becomes an immediate logical miss after expiry.
func TestCLIRefreshStore(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	hash := sha256.Sum256([]byte(t.Name() + time.Now().String()))
	defer func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM cli_refresh_idempotency WHERE token_hash = $1`, hash[:])
	}()

	want := cliRefreshResponse{
		Body:   []byte(`{"access_token":"access-a","refresh_token":"refresh-b"}`),
		Status: http.StatusOK,
	}
	body, status, err := st.IdempotentCLIRefresh(ctx, hash, time.Minute, func(context.Context) ([]byte, int, error) {
		return want.Body, want.Status, nil
	})
	if err != nil || status != want.Status || string(body) != string(want.Body) {
		t.Fatalf("mint = status %d body %q err %v, want status %d body %q", status, body, err, want.Status, want.Body)
	}
	got, ok, err := getCLIRefresh(ctx, pool, hash)
	if err != nil || !ok {
		t.Fatalf("get = (%+v, %v, %v), want hit", got, ok, err)
	}
	if got.Status != want.Status || string(got.Body) != string(want.Body) {
		t.Fatalf("get = status %d body %q, want status %d body %q", got.Status, got.Body, want.Status, want.Body)
	}

	if _, err := pool.Exec(ctx,
		`UPDATE cli_refresh_idempotency SET expires_at = now() - interval '1 second' WHERE token_hash = $1`, hash[:],
	); err != nil {
		t.Fatalf("expire: %v", err)
	}
	if got, ok, err := getCLIRefresh(ctx, pool, hash); err != nil || ok {
		t.Fatalf("get expired = (%+v, %v, %v), want miss", got, ok, err)
	}

	// A fresh mint performs the bounded physical sweep. The logical expiry
	// above is immediate; this assertion proves expired rows do not accumulate.
	sweepHash := sha256.Sum256([]byte(t.Name() + "-sweep-" + time.Now().String()))
	defer func() {
		_, _ = pool.Exec(context.Background(),
			`DELETE FROM cli_refresh_idempotency WHERE token_hash = $1`, sweepHash[:])
	}()
	if _, _, err := st.IdempotentCLIRefresh(ctx, sweepHash, time.Minute, func(context.Context) ([]byte, int, error) {
		return []byte(`{"access_token":"sweep","refresh_token":"sweep"}`), http.StatusOK, nil
	}); err != nil {
		t.Fatalf("mint triggering sweep: %v", err)
	}
	var stale int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM cli_refresh_idempotency WHERE token_hash = $1`, hash[:],
	).Scan(&stale); err != nil {
		t.Fatal(err)
	}
	if stale != 0 {
		t.Fatal("expired CLI refresh row survived the bounded sweep")
	}
}

// TestCheckOwnership verifies that CheckOwnership returns nil when all
// public-schema tables are owned by the connecting role (the normal post-
// migration state), and returns an error when a table has drifted to a
// different owner (the tenant_invites incident, w1/m26 t006).
func TestCheckOwnership(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	// Normal post-migration state: all tables owned by the current user.
	if err := CheckOwnership(ctx, pool); err != nil {
		t.Fatalf("clean ownership check failed: %v", err)
	}

	// Simulate drift: create a throwaway role, transfer a table's owner, verify
	// CheckOwnership catches it. Skipped if the test user lacks SUPERUSER/CREATEROLE.
	const driftRole = "bex_ownership_test_role"
	if _, err := pool.Exec(ctx, `CREATE ROLE `+driftRole); err != nil {
		t.Skipf("cannot create role (no privilege): %v", err)
	}
	defer func() {
		_, _ = pool.Exec(ctx, `ALTER TABLE tenants OWNER TO CURRENT_USER`)
		_, _ = pool.Exec(ctx, `DROP ROLE IF EXISTS `+driftRole)
	}()
	if _, err := pool.Exec(ctx, `ALTER TABLE tenants OWNER TO `+driftRole); err != nil {
		t.Skipf("cannot change table owner: %v", err)
	}
	if err := CheckOwnership(ctx, pool); err == nil {
		t.Error("CheckOwnership returned nil with misowned table, want error")
	}
}

// TestAcceptInviteRespectsPlanLimits pins the fix for the plan-limit bypass this
// milestone (w6/m13) proved on real prod: a workspace on Pro invites a second
// member (and a `developer`, a role Hobby doesn't offer), then downgrades to
// Hobby. ChangePlan's guards count ACCEPTED members, so the downgrade succeeds
// with the invites still pending; when the invitee first logs in, the accept path
// used to redeem them unconditionally — leaving a Hobby workspace (cap: 1 member,
// admin-only) with 2 members and a forbidden role. Accept now enforces the
// workspace's current plan and leaves a violating invite pending, so it can
// self-heal if the workspace upgrades again.
func TestAcceptInviteRespectsPlanLimits(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	exp := time.Now().Add(24 * time.Hour)

	// A Pro workspace invites a 2nd member (admin) and a developer — both legal on Pro.
	ten, err := s.CreateWorkspace(ctx, "acme", PlanPro, "admin-1")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if _, err := s.CreateInvite(ctx, ten.ID, "seat@example.com", "admin", "tok-seat", "admin-1", exp); err != nil {
		t.Fatalf("invite seat: %v", err)
	}
	if _, err := s.CreateInvite(ctx, ten.ID, "dev@example.com", "developer", "tok-dev", "admin-1", exp); err != nil {
		t.Fatalf("invite dev: %v", err)
	}

	// ...then downgrades to Hobby (cap: 1 member, admin-only) while both are pending.
	if _, err := s.UpdateTenantPlan(ctx, ten.ID, PlanHobby); err != nil {
		t.Fatalf("downgrade: %v", err)
	}

	// The 2nd seat must NOT be redeemed: it would put the Hobby workspace at 2 members.
	accepted, err := s.AcceptInvitesForEmail(ctx, "seat@example.com", "identity-seat")
	if err != nil {
		t.Fatalf("accept seat: %v", err)
	}
	if len(accepted) != 0 {
		t.Errorf("hobby workspace redeemed a 2nd member: accepted %d, want 0 (member cap 1)", len(accepted))
	}
	if _, err := s.GetTenantMember(ctx, ten.ID, "identity-seat"); !errors.Is(err, ErrNotFound) {
		t.Errorf("seat became a member of a full Hobby workspace: %v", err)
	}

	// The developer invite must NOT be redeemed either: Hobby offers no such role.
	if accepted, err := s.AcceptInvitesForEmail(ctx, "dev@example.com", "identity-dev"); err != nil || len(accepted) != 0 {
		t.Errorf("hobby workspace redeemed a developer: accepted %d (%v), want 0 (role not on plan)", len(accepted), err)
	}

	// Both invites stay pending (not consumed) so they can self-heal on upgrade.
	if pending, err := s.ListInvites(ctx, ten.ID); err != nil || len(pending) != 2 {
		t.Fatalf("pending after refused accepts = %d (%v), want 2 (left redeemable)", len(pending), err)
	}

	// Upgrading back to Pro lets the very same invites redeem on the next login.
	if _, err := s.UpdateTenantPlan(ctx, ten.ID, PlanPro); err != nil {
		t.Fatalf("upgrade: %v", err)
	}
	if accepted, err := s.AcceptInvitesForEmail(ctx, "seat@example.com", "identity-seat"); err != nil || len(accepted) != 1 {
		t.Fatalf("accept after upgrade = %d (%v), want 1", len(accepted), err)
	}
	if m, err := s.GetTenantMember(ctx, ten.ID, "identity-seat"); err != nil || m.Role != "admin" {
		t.Errorf("redeemed role = %q (%v), want admin", m.Role, err)
	}
	if accepted, err := s.AcceptInvitesForEmail(ctx, "dev@example.com", "identity-dev"); err != nil || len(accepted) != 1 {
		t.Fatalf("accept developer after upgrade = %d (%v), want 1", len(accepted), err)
	}

	// An invite that only UPGRADES an existing member's role takes no new seat, so
	// it redeems even on a plan whose member cap is already met.
	solo, err := s.CreateWorkspace(ctx, "solo", PlanHobby, "identity-solo")
	if err != nil {
		t.Fatalf("create hobby workspace: %v", err)
	}
	if _, err := s.CreateInvite(ctx, solo.ID, "solo@example.com", "admin", "tok-solo", "identity-solo", exp); err != nil {
		t.Fatalf("invite solo: %v", err)
	}
	if accepted, err := s.AcceptInvitesForEmail(ctx, "solo@example.com", "identity-solo"); err != nil || len(accepted) != 1 {
		t.Errorf("self re-invite on a full Hobby workspace = %d (%v), want 1 (role change, no new seat)", len(accepted), err)
	}
}

// TestDefaultWorkspaceIsTheOldestMembership is w6/m14's t001 against a real
// database: with two memberships, the bare join returned an arbitrary row (the
// w6/m11 field bug — a caller's "current workspace" could differ call to call).
// The default workspace is now the OLDEST membership, stably.
func TestDefaultWorkspaceIsTheOldestMembership(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	// Two workspaces for one subject. CreateWorkspace writes the tenant + the
	// owner's membership in one transaction, so "older workspace" == "older
	// membership" here — the ordinary case (a second workspace created later).
	older, err := s.CreateWorkspace(ctx, "older", PlanHobby, "dana")
	if err != nil {
		t.Fatalf("create older: %v", err)
	}
	newer, err := s.CreateWorkspace(ctx, "newer", PlanHobby, "dana")
	if err != nil {
		t.Fatalf("create newer: %v", err)
	}

	// Repeated calls must all give the SAME answer — the point of the ORDER BY.
	// (Without it this passed or failed on Postgres' whim, which is exactly how
	// it shipped and then misbehaved in production.)
	for i := range 10 {
		got, err := s.TenantForIdentity(ctx, "dana")
		if err != nil {
			t.Fatalf("TenantForIdentity (call %d): %v", i, err)
		}
		if got.ID != older.ID {
			t.Fatalf("call %d: default workspace = %s (%s), want the OLDEST membership %s (%s)",
				i, got.Name, got.ID, older.Name, older.ID)
		}
	}

	// IsMember answers for BOTH workspaces — the gate that lets a caller name
	// the newer one explicitly (ownerId) even though it is not their default.
	for _, w := range []Tenant{older, newer} {
		member, err := s.IsMember(ctx, "dana", w.ID)
		if err != nil {
			t.Fatalf("IsMember(%s): %v", w.Name, err)
		}
		if !member {
			t.Errorf("IsMember(dana, %s) = false, want true", w.Name)
		}
	}
	// A workspace she does not belong to, and one that does not exist at all:
	// both false, no error — "you may not act there", not a leak of existence.
	stranger, err := s.CreateWorkspace(ctx, "stranger", PlanHobby, "eve")
	if err != nil {
		t.Fatalf("create stranger: %v", err)
	}
	for _, id := range []string{stranger.ID, "tea-does-not-exist"} {
		member, err := s.IsMember(ctx, "dana", id)
		if err != nil {
			t.Errorf("IsMember(dana, %s): %v", id, err)
		}
		if member {
			t.Errorf("IsMember(dana, %s) = true, want false", id)
		}
	}
}

// TestNotificationSettings (w3/m9) exercises the deploy-notification store
// path: a member with no row gets the default (both true) via
// ListNotifyRecipients' COALESCE, an explicit Upsert overrides it for that
// member only, and a second Upsert updates in place rather than duplicating
// (the (tenant_id, subject) unique index).
func TestNotificationSettings(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	ten, err := s.CreateWorkspace(ctx, "acme", PlanPro, "admin-1")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	if err := s.AddMember(ctx, "bob", ten.ID, "developer"); err != nil {
		t.Fatalf("add member: %v", err)
	}

	// No explicit row for either member yet: GetNotificationSettings is
	// ErrNotFound (the service layer applies the default), but
	// ListNotifyRecipients already resolves the default for both.
	if _, err := s.GetNotificationSettings(ctx, ten.ID, "admin-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("get with no row: want ErrNotFound, got %v", err)
	}
	recipients, err := s.ListNotifyRecipients(ctx, ten.ID)
	if err != nil || len(recipients) != 2 {
		t.Fatalf("recipients = %d (%v), want 2", len(recipients), err)
	}
	for _, r := range recipients {
		if r.DeployStarted || r.DeploySucceeded || !r.DeployFailed {
			t.Errorf("recipient %s defaults = (%v,%v,%v), want (false,false,true)", r.Subject, r.DeployStarted, r.DeploySucceeded, r.DeployFailed)
		}
	}

	// bob opts out of start and success emails only.
	got, err := s.UpsertNotificationSettings(ctx, ten.ID, "bob", false, false, true)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got.DeployStarted || got.DeploySucceeded || !got.DeployFailed {
		t.Errorf("upserted = (%v,%v,%v), want (false,false,true)", got.DeployStarted, got.DeploySucceeded, got.DeployFailed)
	}
	if got, err := s.GetNotificationSettings(ctx, ten.ID, "bob"); err != nil || got.DeployStarted || got.DeploySucceeded || !got.DeployFailed {
		t.Errorf("get after upsert = %+v (%v), want (false,false,true)", got, err)
	}
	// admin-1 is untouched — still the default via the join.
	recipients, err = s.ListNotifyRecipients(ctx, ten.ID)
	if err != nil || len(recipients) != 2 {
		t.Fatalf("recipients after upsert = %d (%v), want 2", len(recipients), err)
	}
	for _, r := range recipients {
		switch r.Subject {
		case "bob":
			if r.DeployStarted || r.DeploySucceeded || !r.DeployFailed {
				t.Errorf("bob recipient = (%v,%v,%v), want (false,false,true)", r.DeployStarted, r.DeploySucceeded, r.DeployFailed)
			}
		case "admin-1":
			if r.DeployStarted || r.DeploySucceeded || !r.DeployFailed {
				t.Errorf("admin-1 recipient = (%v,%v,%v), want (false,false,true) (default, unmodified)", r.DeployStarted, r.DeploySucceeded, r.DeployFailed)
			}
		}
	}

	// Native push policy extends this same row. Updating it preserves bob's
	// existing email booleans, and a later email update preserves the JSONB
	// document in turn.
	pushPolicy := json.RawMessage(`{"enabled":false,"events":[],"minimumUrgency":"important","timeZone":"UTC","workingHours":[],"quietHours":[],"maxDeferralSeconds":3600,"serviceOverrides":[]}`)
	pushRow, err := s.UpsertNotificationPushPolicy(ctx, ten.ID, "bob", pushPolicy)
	if err != nil {
		t.Fatalf("upsert push policy: %v", err)
	}
	if pushRow.DeployStarted || pushRow.DeploySucceeded || !pushRow.DeployFailed {
		t.Fatalf("push upsert changed email settings: %+v", pushRow)
	}
	var wantPush, gotPush any
	if err := json.Unmarshal(pushPolicy, &wantPush); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(pushRow.PushPolicy, &gotPush); err != nil {
		t.Fatalf("stored push policy: %v", err)
	}
	if !reflect.DeepEqual(gotPush, wantPush) {
		t.Fatalf("stored push policy = %s, want %s", pushRow.PushPolicy, pushPolicy)
	}

	// A second upsert updates the same row (unique index), not a duplicate.
	if _, err := s.UpsertNotificationSettings(ctx, ten.ID, "bob", true, true, false); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_settings WHERE tenant_id = $1 AND subject = 'bob'`, ten.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows for bob = %d, want 1 (upsert, not insert)", n)
	}
	if got, err := s.GetNotificationSettings(ctx, ten.ID, "bob"); err != nil || !got.DeployStarted || !got.DeploySucceeded || got.DeployFailed {
		t.Errorf("get after re-upsert = %+v (%v), want (true,true,false)", got, err)
	} else {
		var afterEmail any
		if err := json.Unmarshal(got.PushPolicy, &afterEmail); err != nil || !reflect.DeepEqual(afterEmail, wantPush) {
			t.Errorf("email update changed push policy = %s (%v)", got.PushPolicy, err)
		}
	}

	// Deleting the workspace cascades its notification_settings rows.
	if err := s.DeleteTenant(ctx, ten.ID); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_settings`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("notification_settings after tenant delete = %d, want 0 (cascade)", n)
	}
}

// TestSandboxKeyMintIdempotentAndResolves exercises the per-workspace OpenSandbox
// tenant key (m32 t006): one key per workspace, race-safe minting, and the
// reverse key→workspace lookup the tenant-provider endpoint serves.
func TestSandboxKeyMintIdempotentAndResolves(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	// The workspace_id FK (migration 0056) requires real tenant rows, so the key
	// dies with its workspace instead of outliving it (w1/m61).
	tenA, err := s.CreateWorkspace(ctx, "sbxkey-a", PlanPro, "sbxkey-owner-a")
	if err != nil {
		t.Fatalf("create workspace a: %v", err)
	}
	tenB, err := s.CreateWorkspace(ctx, "sbxkey-b", PlanPro, "sbxkey-owner-b")
	if err != nil {
		t.Fatalf("create workspace b: %v", err)
	}

	// First mint returns a key; repeat mint returns the SAME key (idempotent).
	k1, err := s.SandboxKeyForWorkspace(ctx, tenA.ID)
	if err != nil {
		t.Fatalf("first mint: %v", err)
	}
	if k1 == "" {
		t.Fatal("empty key")
	}
	k2, err := s.SandboxKeyForWorkspace(ctx, tenA.ID)
	if err != nil {
		t.Fatalf("second mint: %v", err)
	}
	if k1 != k2 {
		t.Errorf("repeat mint diverged: %q vs %q", k1, k2)
	}

	// A different workspace gets a distinct key.
	kb, err := s.SandboxKeyForWorkspace(ctx, tenB.ID)
	if err != nil {
		t.Fatal(err)
	}
	if kb == k1 {
		t.Error("distinct workspaces must get distinct keys")
	}

	// Reverse lookup resolves the key back to its workspace; unknown → ErrNotFound.
	ws, err := s.WorkspaceForSandboxKey(ctx, k1)
	if err != nil || ws != tenA.ID {
		t.Errorf("resolve k1 = %q err %v, want %q", ws, err, tenA.ID)
	}
	if _, err := s.WorkspaceForSandboxKey(ctx, "osk-nope"); !errors.Is(err, ErrNotFound) {
		t.Errorf("unknown key err = %v, want ErrNotFound", err)
	}

	// Lookup-only resolver (w1/m61): returns the minted key WITHOUT minting, and
	// reports found=false for a workspace that never created a sandbox — the seam
	// the workspace-delete purger uses to find the key to tear sandboxes down.
	gotKey, found, err := s.SandboxKeyLookup(ctx, tenA.ID)
	if err != nil || !found || gotKey != k1 {
		t.Errorf("SandboxKeyLookup(a) = %q found=%v err=%v, want %q true nil", gotKey, found, err, k1)
	}
	if gotKey, found, err := s.SandboxKeyLookup(ctx, "tea-never"); err != nil || found || gotKey != "" {
		t.Errorf("SandboxKeyLookup(unknown) = %q found=%v err=%v, want \"\" false nil", gotKey, found, err)
	}

	// Concurrent first-mints for a fresh workspace converge to one key (the
	// UNIQUE(workspace_id) constraint, not a check-then-insert).
	tenRace, err := s.CreateWorkspace(ctx, "sbxkey-race", PlanPro, "sbxkey-owner-race")
	if err != nil {
		t.Fatalf("create workspace race: %v", err)
	}
	const n = 20
	keys := make([]string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			k, err := s.SandboxKeyForWorkspace(ctx, tenRace.ID)
			if err != nil {
				t.Errorf("concurrent mint %d: %v", i, err)
				return
			}
			keys[i] = k
		}(i)
	}
	wg.Wait()
	for i, k := range keys {
		if k != keys[0] {
			t.Fatalf("concurrent mints diverged: 0=%q %d=%q", keys[0], i, k)
		}
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM sandbox_tenant_keys WHERE workspace_id = $1`, tenRace.ID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Errorf("race key rows = %d, want 1", count)
	}
}

// TestSandboxKeyCascadesOnTenantDelete proves migration 0056's ON DELETE CASCADE:
// deleting a workspace drops its sandbox tenant key so a stale key can no longer
// resolve to a dead tenant, closing the w1/m61 orphaned-credential gap.
func TestSandboxKeyCascadesOnTenantDelete(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)

	ten, err := s.CreateWorkspace(ctx, "sbxkey-cascade", PlanPro, "sbxkey-cascade-owner")
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}
	key, err := s.SandboxKeyForWorkspace(ctx, ten.ID)
	if err != nil {
		t.Fatalf("mint key: %v", err)
	}
	// The key resolves both ways while the workspace lives.
	if _, found, err := s.SandboxKeyLookup(ctx, ten.ID); err != nil || !found {
		t.Fatalf("pre-delete lookup found=%v err=%v, want true", found, err)
	}
	if ws, err := s.WorkspaceForSandboxKey(ctx, key); err != nil || ws != ten.ID {
		t.Fatalf("pre-delete resolve = %q err=%v, want %q", ws, err, ten.ID)
	}

	if err := s.DeleteTenant(ctx, ten.ID); err != nil {
		t.Fatalf("delete tenant: %v", err)
	}

	// The FK cascade dropped the key row: neither direction resolves anymore.
	if gotKey, found, err := s.SandboxKeyLookup(ctx, ten.ID); err != nil || found || gotKey != "" {
		t.Errorf("post-delete lookup = %q found=%v err=%v, want \"\" false nil", gotKey, found, err)
	}
	if _, err := s.WorkspaceForSandboxKey(ctx, key); !errors.Is(err, ErrNotFound) {
		t.Errorf("post-delete resolve err = %v, want ErrNotFound", err)
	}
}

// TestPGGroupingTxRollsBackPartialSets proves w8/m20 t001's whole point: a
// grouping apply that fails mid-loop persists NOTHING from that sync.
func TestPGGroupingTxRollsBackPartialSets(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := &PGStore{Pool: pool}
	ten, err := s.CreateTenant(ctx, "grouping-tx", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	tenant := ten.ID

	injected := fmt.Errorf("injected mid-loop failure")
	err = s.RunGroupingTx(ctx, func(g GroupingStore) error {
		p1, err := g.CreateProject(ctx, tenant, "tx-alpha")
		if err != nil {
			return err
		}
		if _, err := g.CreateEnvironment(ctx, p1.ID, tenant, "staging"); err != nil {
			return err
		}
		if _, err := g.CreateProject(ctx, tenant, "tx-beta"); err != nil {
			return err
		}
		return injected
	})
	if !errors.Is(err, injected) {
		t.Fatalf("RunGroupingTx error = %v, want the injected failure", err)
	}
	projects, environments, err := s.CountWorkspaceGroupings(ctx, tenant)
	if err != nil {
		t.Fatal(err)
	}
	if projects != 0 || environments != 0 {
		t.Fatalf("rolled-back sync persisted rows: %d projects, %d environments", projects, environments)
	}

	// A successful run commits everything it wrote.
	if err := s.RunGroupingTx(ctx, func(g GroupingStore) error {
		p, err := g.CreateProject(ctx, tenant, "tx-alpha")
		if err != nil {
			return err
		}
		_, err = g.CreateEnvironment(ctx, p.ID, tenant, "staging")
		return err
	}); err != nil {
		t.Fatal(err)
	}
	if projects, environments, _ = s.CountWorkspaceGroupings(ctx, tenant); projects != 1 || environments != 1 {
		t.Fatalf("committed sync counts = %d projects, %d environments", projects, environments)
	}
}

// TestPGReclaimEmptyBlueprintGroupings proves the disconnect sweep deletes
// only empty, unreferenced groupings (w8/m20 t004): a populated environment
// (apps member) and a CR-referenced one survive; the reclaimed project goes
// only once it is empty of environments and members.
func TestPGReclaimEmptyBlueprintGroupings(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	s := &PGStore{Pool: pool}
	ten, err := s.CreateTenant(ctx, "grouping-reclaim", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	tenant := ten.ID

	empty, err := s.CreateProject(ctx, tenant, "rc-empty")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateEnvironment(ctx, empty.ID, tenant, "staging"); err != nil {
		t.Fatal(err)
	}
	populated, err := s.CreateProject(ctx, tenant, "rc-populated")
	if err != nil {
		t.Fatal(err)
	}
	popEnv, err := s.CreateEnvironment(ctx, populated.ID, tenant, "production")
	if err != nil {
		t.Fatal(err)
	}
	app, err := s.CreateApp(ctx, App{TenantID: tenant, Name: "rc-web", Image: "traefik/whoami", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.SetEnvironmentServices(ctx, popEnv.ID, populated.ID, tenant, []string{app.ID}); err != nil {
		t.Fatal(err)
	}
	referenced, err := s.CreateProject(ctx, tenant, "rc-referenced")
	if err != nil {
		t.Fatal(err)
	}
	refEnv, err := s.CreateEnvironment(ctx, referenced.ID, tenant, "production")
	if err != nil {
		t.Fatal(err)
	}

	pairs := []GroupingPair{
		{Project: "rc-empty", Environment: "staging"},
		{Project: "rc-populated", Environment: "production"},
		{Project: "rc-referenced", Environment: "production"},
		{Project: "rc-missing", Environment: "gone"},
	}
	removedEnvs, removedProjects, err := s.ReclaimEmptyBlueprintGroupings(ctx, tenant, pairs,
		map[string]bool{refEnv.ID: true}, map[string]bool{referenced.ID: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(removedEnvs) != 1 || removedEnvs[0] != "rc-empty/staging" {
		t.Fatalf("removed environments = %v", removedEnvs)
	}
	if len(removedProjects) != 1 || removedProjects[0] != "rc-empty" {
		t.Fatalf("removed projects = %v", removedProjects)
	}
	// Survivors intact: the populated environment and the CR-referenced one.
	if _, err := s.GetEnvironment(ctx, popEnv.ID); err != nil {
		t.Fatalf("populated environment must survive: %v", err)
	}
	if _, err := s.GetEnvironment(ctx, refEnv.ID); err != nil {
		t.Fatalf("referenced environment must survive: %v", err)
	}
	if _, err := s.GetProject(ctx, populated.ID); err != nil {
		t.Fatalf("populated project must survive: %v", err)
	}
}
