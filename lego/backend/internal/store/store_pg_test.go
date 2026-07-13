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
	"os"
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
	assertProjectionJoin(ctx, t, s, app)
	assertDeployLifecycle(ctx, t, s, app)
	assertDomainUniqueness(ctx, t, s, ten.ID)
	assertAuditEvents(ctx, t, s, ten)
	assertServiceEvents(ctx, t, s, ten, app)
	assertWorkspaceLifecycle(ctx, t, s, pool)
	assertDeleteCascades(ctx, t, s, pool, app)
}

// assertDeployLifecycle exercises w2/m5's deploy history against real
// Postgres: CreateApp already opened deploy #1 (trigger "create") in the same
// transaction as the app row; ListOpenDeploys/CloseDeploy are the
// reconciler's write-back seam; CreateDeploy is the trigger verb's seam. Ordering
// (newest-first) and the ErrNotFound cross-app scope guard are asserted here
// because they depend on Postgres's ORDER BY / WHERE, not just Go logic.
func assertDeployLifecycle(ctx context.Context, t *testing.T, s *PGStore, app App) {
	t.Helper()
	deploys, err := s.ListDeploys(ctx, app.ID)
	if err != nil {
		t.Fatalf("list deploys: %v", err)
	}
	if len(deploys) != 1 || deploys[0].Trigger != "create" || deploys[0].Status != DeployUpdateInProgress {
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

	second, err := s.CreateDeploy(ctx, app.ID, "api", app.Image, 2)
	if err != nil || second.Status != DeployUpdateInProgress {
		t.Fatalf("trigger deploy: %+v (err %v)", second, err)
	}
	deploys, err = s.ListDeploys(ctx, app.ID)
	if err != nil || len(deploys) != 2 || deploys[0].ID != second.ID {
		t.Fatalf("list after trigger (want newest first) = %+v (err %v)", deploys, err)
	}

	if _, err := s.GetDeploy(ctx, "srv-doesnotexist00000", second.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("get deploy scoped to the wrong app: want ErrNotFound, got %v", err)
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
	if all[0].Outcome != string(core.AuditAllowed) || all[0].Verb != "apps.Suspend" || all[0].Caller != "user-x" {
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
// function left behind: #1 closed live (started + ended = two events) and #2 open
// (started only) — three deploy events.
func assertServiceEvents(ctx context.Context, t *testing.T, s *PGStore, ten Tenant, app App) {
	t.Helper()
	target := core.ServiceTarget(app.Name)
	verbs := []string{"apps.Suspend", "apps.Scale"}
	phases := []string{EventPhaseStarted, EventPhaseEnded}

	deploys, err := s.ListDeploys(ctx, app.ID)
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

	all, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{Verbs: verbs, Phases: phases})
	if err != nil {
		t.Fatalf("list service events: %v", err)
	}
	if len(all) != 5 {
		t.Fatalf("feed = %d events, want 5 (3 deploy + 2 audit; denied/cross-tenant/unmapped excluded)\n%+v", len(all), all)
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
	if all[0].Verb != "apps.Scale" || all[1].Verb != "apps.Suspend" {
		t.Errorf("two newest = %q, %q; want the audit rows apps.Scale then apps.Suspend", all[0].Verb, all[1].Verb)
	}
	if all[0].Source != EventSourceAudit || all[0].Caller != "user-x" {
		t.Errorf("audit event = %+v, want source=audit caller=user-x", all[0])
	}
	// The closed deploy projects BOTH of its transitions; the open one only its start.
	seenPhases := map[string]int{}
	for _, e := range all {
		if e.Source == EventSourceDeploy {
			seenPhases[e.Phase]++
		}
	}
	if seenPhases[EventPhaseStarted] != 2 || seenPhases[EventPhaseEnded] != 1 {
		t.Errorf("deploy phases = %v, want 2 started (both deploys) + 1 ended (only the closed one)", seenPhases)
	}

	// Keyset paging: walk the feed 2 at a time and reassemble it exactly. Between
	// page 1 and page 2 a NEWER event lands — the page-2 cursor must not notice
	// (an OFFSET would have shifted and re-served a row here).
	page1, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{Verbs: verbs, Phases: phases, Limit: 2})
	if err != nil || len(page1) != 2 {
		t.Fatalf("page 1 = %+v (err %v), want 2", page1, err)
	}
	record(base.Add(time.Minute), "apps.Suspend", ten.ID, target, core.AuditAllowed) // concurrent insert, newest
	after := page1[len(page1)-1]

	var walked []ServiceEventRow
	walked = append(walked, page1...)
	for {
		page, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{
			Verbs: verbs, Phases: phases, Limit: 2, AfterAt: after.At, AfterKey: after.Key,
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

	// The window bounds `at` inclusively — the two audit events only.
	windowed, err := s.ListServiceEvents(ctx, app.ID, target, ten.ID, ServiceEventFilter{
		Verbs: verbs, Phases: phases, Since: base, Until: base.Add(time.Second),
	})
	if err != nil || len(windowed) != 2 {
		t.Fatalf("windowed feed = %+v (err %v), want the 2 audit events", windowed, err)
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

	// A hand-applied app (no control-plane row) and a service nobody targeted:
	// an empty feed, never another service's rows.
	empty, err := s.ListServiceEvents(ctx, "srv-doesnotexist0000", core.ServiceTarget("nope"), ten.ID, ServiceEventFilter{Verbs: verbs, Phases: phases})
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
	// Primary domain must come first — it becomes spec.host.
	if len(d.Hosts) != 2 || d.Hosts[0] != "web.example.com" || d.Hosts[1] != "extra.example.com" {
		t.Errorf("hosts = %v", d.Hosts)
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

	if err := s.AddDomain(ctx, a1.ID, "shared.example.com"); err != nil {
		t.Fatalf("first AddDomain: %v", err)
	}
	// Same app, same host → idempotent no-op (the host UNIQUE conflict is this
	// app's own row, so it is swallowed).
	if err := s.AddDomain(ctx, a1.ID, "shared.example.com"); err != nil {
		t.Errorf("same-app re-add must be idempotent, got %v", err)
	}
	// A different app claiming the same host → real cross-app collision, surfaced
	// (Render's "already exists on another site"), not swallowed.
	if err := s.AddDomain(ctx, a2.ID, "shared.example.com"); !errors.Is(err, ErrConflict) {
		t.Errorf("cross-app AddDomain => ErrConflict, got %v", err)
	}
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
		if !r.DeploySucceeded || !r.DeployFailed {
			t.Errorf("recipient %s defaults = (%v,%v), want (true,true)", r.Subject, r.DeploySucceeded, r.DeployFailed)
		}
	}

	// bob opts out of success emails only.
	got, err := s.UpsertNotificationSettings(ctx, ten.ID, "bob", false, true)
	if err != nil {
		t.Fatalf("upsert: %v", err)
	}
	if got.DeploySucceeded || !got.DeployFailed {
		t.Errorf("upserted = (%v,%v), want (false,true)", got.DeploySucceeded, got.DeployFailed)
	}
	if got, err := s.GetNotificationSettings(ctx, ten.ID, "bob"); err != nil || got.DeploySucceeded || !got.DeployFailed {
		t.Errorf("get after upsert = %+v (%v), want (false,true)", got, err)
	}
	// admin-1 is untouched — still the default via the join.
	recipients, err = s.ListNotifyRecipients(ctx, ten.ID)
	if err != nil || len(recipients) != 2 {
		t.Fatalf("recipients after upsert = %d (%v), want 2", len(recipients), err)
	}
	for _, r := range recipients {
		switch r.Subject {
		case "bob":
			if r.DeploySucceeded || !r.DeployFailed {
				t.Errorf("bob recipient = (%v,%v), want (false,true)", r.DeploySucceeded, r.DeployFailed)
			}
		case "admin-1":
			if !r.DeploySucceeded || !r.DeployFailed {
				t.Errorf("admin-1 recipient = (%v,%v), want (true,true) (default, unmodified)", r.DeploySucceeded, r.DeployFailed)
			}
		}
	}

	// A second upsert updates the same row (unique index), not a duplicate.
	if _, err := s.UpsertNotificationSettings(ctx, ten.ID, "bob", true, false); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM notification_settings WHERE tenant_id = $1 AND subject = 'bob'`, ten.ID).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("rows for bob = %d, want 1 (upsert, not insert)", n)
	}
	if got, err := s.GetNotificationSettings(ctx, ten.ID, "bob"); err != nil || !got.DeploySucceeded || got.DeployFailed {
		t.Errorf("get after re-upsert = %+v (%v), want (true,false)", got, err)
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
