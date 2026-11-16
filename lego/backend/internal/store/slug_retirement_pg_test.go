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
	"os"
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// forgetSlugAtCleanup removes app's ledger row when the test ends. The ledger
// keeps a slug after its service and workspace are gone, so a test expecting a
// bare slug would otherwise fail when run again on the same database.
func forgetSlugAtCleanup(t *testing.T, st *PGStore, app App) {
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `DELETE FROM service_slugs WHERE service_id = $1`, app.ID)
	})
}

// A deleted service's subdomain is never minted again, as on Render
// (docs/render-artifacts/duplicate-service-names.md, Rule 4): a re-created name
// keeps the name and gets a suffixed subdomain, whether the old service went on
// its own or with its workspace, and whichever workspace asks. A create that
// never completed held no subdomain, so its retry gets the bare name.
func TestPGADeletedServicesSlugIsNeverMintedAgain(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tenant := func(name string) Tenant {
		t.Helper()
		ten, err := st.CreateTenant(ctx, name, PlanHobby)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = st.DeleteTenant(ctx, ten.ID) })
		return ten
	}
	create := func(ten Tenant, name string) App {
		t.Helper()
		app, err := st.CreateApp(ctx, App{TenantID: ten.ID, Name: name, Image: "nginx:1", Port: 80, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatalf("CreateApp(%q): %v", name, err)
		}
		if app.Name != name {
			t.Fatalf("CreateApp(%q) named it %q", name, app.Name)
		}
		forgetSlugAtCleanup(t, st, app)
		return app
	}
	suffixed := func(app App, name string) bool {
		suffix, ok := strings.CutPrefix(app.Slug, name+"-")
		return ok && ValidBlueprintResourceSuffix(suffix)
	}

	owner, other := tenant("slug-retire-owner"), tenant("slug-retire-other")
	const name = "retired-web"
	first := create(owner, name)
	if first.Slug != name {
		t.Fatalf("first claimant of %q got slug %q, want the bare name", name, first.Slug)
	}
	if err := st.DeleteApp(ctx, first.ID); err != nil {
		t.Fatal(err)
	}
	again := create(owner, name)
	if !suffixed(again, name) {
		t.Errorf("re-created %q in the same workspace got slug %q, want %s plus a four-character suffix: the deleted service's subdomain is never reused", name, again.Slug, name)
	}
	elsewhere := create(other, name)
	if !suffixed(elsewhere, name) || elsewhere.Slug == again.Slug {
		t.Errorf("another workspace's %q got slug %q (the re-created one has %q), want its own suffixed slug", name, elsewhere.Slug, again.Slug)
	}

	// A workspace's deletion cascades through its services; their subdomains stay retired.
	doomed := tenant("slug-retire-doomed")
	const wsName = "retired-ws"
	if held := create(doomed, wsName); held.Slug != wsName {
		t.Fatalf("first claimant of %q got slug %q, want the bare name", wsName, held.Slug)
	}
	if err := st.DeleteTenant(ctx, doomed.ID); err != nil {
		t.Fatal(err)
	}
	if after := create(other, wsName); !suffixed(after, wsName) {
		t.Errorf("%q created after its holder's workspace was deleted got slug %q, want a suffixed slug", wsName, after.Slug)
	}

	// A create rolled back before it completed never held its subdomain.
	const rolledName = "rolled-back-web"
	rolled := create(owner, rolledName)
	if err := st.RollbackAppCreation(ctx, rolled.ID); err != nil {
		t.Fatal(err)
	}
	if retry := create(owner, rolledName); retry.Slug != rolledName {
		t.Errorf("retry of a rolled-back create got slug %q, want the bare name %q", retry.Slug, rolledName)
	}
}

// Migration 0278 keeps every live service's slug, so no host changes, and
// retires the names bex still records for services deleted before it: the
// creation name from usage_retired_resources and the last shown name from
// resource_display_names. A live service's display name, a build source
// carrier's and a name that is no DNS label are not retired slugs. Every later
// insert into apps is recorded, whichever release wrote it, and the down
// migration removes all of it.
func TestServiceSlugsMigrationBackfill(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	up, err := migrationsFS.ReadFile("migrations/0278_service_slugs.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0278_service_slugs.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0278_service_slugs;
		SET LOCAL search_path TO migration_0278_service_slugs;
		CREATE TABLE apps (id text PRIMARY KEY, name text NOT NULL, slug text NOT NULL UNIQUE, created_at timestamptz NOT NULL DEFAULT now());
		CREATE TABLE usage_retired_resources (resource_kind text NOT NULL, resource_id text NOT NULL, name text NOT NULL, retired_at timestamptz NOT NULL DEFAULT now());
		CREATE TABLE resource_display_names (resource_kind text NOT NULL, resource_id text NOT NULL, display_name text NOT NULL, updated_at timestamptz NOT NULL DEFAULT now());
		INSERT INTO apps (id, name, slug) VALUES ('srv-live', 'web', 'web'), ('srv-suffixed', 'api', 'api-ab12');
		-- srv-live is shown as "front": a live service's shown name is no slug.
		INSERT INTO resource_display_names (resource_kind, resource_id, display_name) VALUES
			('service', 'srv-live', 'front'),
			('service', 'srv-gone', 'static-site'),
			('service', 'srv-renamed', 'My Site'),
			('service', 'bds-gone', 'my-source'),
			('database', 'dpg-gone', 'db');
		INSERT INTO usage_retired_resources (resource_kind, resource_id, name) VALUES
			('service', 'srv-recent', 'worker'),
			('service', 'srv-gone', 'static-site'),
			('database', 'dpg-gone', 'cache');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply 0278: %v", err)
	}
	// A replica still on the previous release inserts apps without knowing the
	// ledger; the trigger records its slug all the same.
	if _, err := tx.Exec(ctx, `INSERT INTO apps (id, name, slug) VALUES ('srv-old-writer', 'cron', 'cron')`); err != nil {
		t.Fatalf("pre-0278 writer after migration: %v", err)
	}
	rows, err := tx.Query(ctx, `SELECT slug FROM service_slugs ORDER BY slug`)
	if err != nil {
		t.Fatal(err)
	}
	got, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"api-ab12", "cron", "static-site", "web", "worker"}; !slices.Equal(got, want) {
		t.Errorf("service_slugs after 0278 = %v, want %v", got, want)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("roll back 0278: %v", err)
	}
	var left bool
	if err := tx.QueryRow(ctx, `SELECT to_regclass('service_slugs') IS NOT NULL OR to_regprocedure('bex_record_service_slug()') IS NOT NULL
		OR EXISTS (SELECT 1 FROM pg_trigger WHERE tgrelid = 'apps'::regclass AND tgname = 'apps_record_service_slug')`).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left {
		t.Error("the down migration left the ledger, its function or its trigger behind")
	}
}
