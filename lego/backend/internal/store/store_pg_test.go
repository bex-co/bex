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

	"github.com/jackc/pgx/v5/pgxpool"
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
	// Isolate from previous runs (order respects FKs via cascade).
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
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

	if err := s.CloseDeploy(ctx, first.ID, DeployLive); err != nil {
		t.Fatalf("close deploy: %v", err)
	}
	// Idempotent: a deploy that's already terminal doesn't get re-closed with a
	// different status.
	if err := s.CloseDeploy(ctx, first.ID, DeployUpdateFailed); err != nil {
		t.Fatalf("re-close deploy: %v", err)
	}
	closed, err := s.GetDeploy(ctx, app.ID, first.ID)
	if err != nil || closed.Status != DeployLive || closed.FinishedAt == nil {
		t.Fatalf("closed deploy = %+v (err %v), want status live with finished_at set", closed, err)
	}
	if _, ok, err := openDeployFor(ctx, s, app.ID); err != nil || ok {
		t.Fatalf("open deploy after close: ok=%v (err %v), want none open", ok, err)
	}

	second, err := s.CreateDeploy(ctx, app.ID, "api", app.Image)
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
