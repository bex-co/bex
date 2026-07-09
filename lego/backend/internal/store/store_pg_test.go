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
	assertWorkspaceLifecycle(ctx, t, s, pool)
	assertDeleteCascades(ctx, t, s, pool, app)
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
	// Domains cascade with their app.
	var n int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM domains`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Errorf("domains after app delete = %d, want 0 (cascade)", n)
	}
}
