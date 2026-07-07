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

	ten, err := s.CreateTenant(ctx, "acme", "starter")
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
	assertDeleteCascades(ctx, t, s, pool, app)
}

func assertErrorTaxonomy(ctx context.Context, t *testing.T, s *PGStore, ten Tenant) {
	t.Helper()
	if _, err := s.CreateTenant(ctx, "acme", "free"); !errors.Is(err, ErrConflict) {
		t.Errorf("duplicate tenant: want ErrConflict, got %v", err)
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
