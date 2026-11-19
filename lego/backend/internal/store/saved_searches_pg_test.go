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
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// renderDefaults are Render's three default views, in its order (w3/m505).
var renderDefaults = []struct{ name, kind string }{
	{"Application Logs", "APPLICATION_LOGS"},
	{"Request Logs", "REQUEST_LOGS"},
	{"Service Metrics", "METRICS"},
}

func assertDefaultViews(t *testing.T, projectID string, got []SavedSearch) {
	t.Helper()
	if len(got) != len(renderDefaults) {
		t.Fatalf("project %s has %d saved searches, want Render's %d: %+v", projectID, len(got), len(renderDefaults), got)
	}
	for i, want := range renderDefaults {
		v := got[i]
		if v.Name != want.name || v.Type != want.kind || v.ProjectID != projectID {
			t.Errorf("view %d = %s %s of %s, want %s %s", i, v.Name, v.Type, v.ProjectID, want.name, want.kind)
		}
		// The SQL derivation and the Go one name a default view the same.
		if wantID := ids.Derive(ids.SavedSearch, projectID, want.kind); v.ID != wantID {
			t.Errorf("%s id = %s, want id.Derive's %s", want.name, v.ID, wantID)
		}
		var doc map[string]any
		if err := json.Unmarshal(v.Filters, &doc); err != nil {
			t.Fatalf("%s filters %s: %v", want.name, v.Filters, err)
		}
		if filters, ok := doc["filters"].([]any); !ok || len(filters) != 0 || doc["version"] != float64(1) || len(doc) != 2 {
			t.Errorf("%s filters = %s, want {\"filters\": [], \"version\": 1}", want.name, v.Filters)
		}
	}
}

// A project gets Render's three default views when it is created, they read
// back by project, type and id, and they go with the project.
func TestPGProjectSavedSearchesFollowTheProject(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "views-"+ids.New(ids.Workspace), PlanPro, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	project, err := st.CreateProject(ctx, tenant.ID, "observed")
	if err != nil {
		t.Fatal(err)
	}
	views, err := st.ListSavedSearches(ctx, project.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	assertDefaultViews(t, project.ID, views)
	for _, v := range views {
		if v.TenantID != tenant.ID {
			t.Errorf("%s tenant = %q, want the project's %q", v.Name, v.TenantID, tenant.ID)
		}
	}

	requests, err := st.ListSavedSearches(ctx, project.ID, "REQUEST_LOGS")
	if err != nil || len(requests) != 1 || requests[0].Name != "Request Logs" {
		t.Fatalf("REQUEST_LOGS views = %+v, %v", requests, err)
	}
	got, err := st.GetSavedSearch(ctx, views[0].ID)
	if err != nil || got.ID != views[0].ID || got.Name != "Application Logs" || got.TenantID != tenant.ID {
		t.Fatalf("GetSavedSearch = %+v, %v", got, err)
	}
	if _, err := st.GetSavedSearch(ctx, ids.New(ids.SavedSearch)); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown saved search: %v, want ErrNotFound", err)
	}

	// Render's duplicate_saved_search_name: a name is unique in its project.
	if _, err := pool.Exec(ctx, `INSERT INTO saved_searches (id, project_id, name, type) VALUES ($1, $2, 'Application Logs', 'APPLICATION_LOGS')`,
		ids.New(ids.SavedSearch), project.ID); err == nil {
		t.Fatal("a second view named Application Logs was stored")
	}
	// Render Workflows is a non-goal: no WORKFLOW_RUNS view is stored.
	if _, err := pool.Exec(ctx, `INSERT INTO saved_searches (id, project_id, name, type) VALUES ($1, $2, 'Runs', 'WORKFLOW_RUNS')`,
		ids.New(ids.SavedSearch), project.ID); err == nil {
		t.Fatal("a WORKFLOW_RUNS view was stored")
	}

	if err := st.DeleteProject(ctx, project.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.GetSavedSearch(ctx, views[0].ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a deleted project's view reads %v, want ErrNotFound", err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM saved_searches WHERE project_id = $1`, project.ID).Scan(&left); err != nil || left != 0 {
		t.Fatalf("%d saved searches outlived their project (err %v)", left, err)
	}
}

// ListProjectServices answers what a project log view names: each service's
// shown name, type and public environment id, without build source carriers.
func TestPGListProjectServices(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "views-"+ids.New(ids.Workspace), PlanPro, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	project, err := st.CreateProject(ctx, tenant.ID, "observed")
	if err != nil {
		t.Fatal(err)
	}
	environment, err := st.CreateEnvironment(ctx, project.ID, tenant.ID, "production")
	if err != nil {
		t.Fatal(err)
	}
	create := func(name, kind string) App {
		t.Helper()
		app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: name, Image: "nginx:1", Type: kind, Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
		if err != nil {
			t.Fatal(err)
		}
		return app
	}
	web, cron, outside := create("web", "web_service"), create("nightly", "cron_job"), create("elsewhere", "web_service")
	if err := st.SetAppDisplayName(ctx, web.ID, "storefront"); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetProjectServices(ctx, project.ID, tenant.ID, []string{web.ID, cron.ID}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := st.SetEnvironmentServices(ctx, environment.ID, project.ID, tenant.ID, []string{web.ID}, nil); err != nil {
		t.Fatal(err)
	}
	got, err := st.ListProjectServices(ctx, project.ID)
	if err != nil {
		t.Fatal(err)
	}
	want := []ProjectService{
		{ID: cron.ID, Name: "nightly", Type: "cron_job"},
		{ID: web.ID, Name: "storefront", Type: "web_service", EnvironmentID: ids.EnvironmentPublicID(environment.ID)},
	}
	if len(got) != len(want) {
		t.Fatalf("ListProjectServices = %+v, want %+v (not %s)", got, want, outside.ID)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("service %d = %+v, want %+v", i, got[i], want[i])
		}
	}
}

// The migration backfills every existing project with the defaults, under the
// ids the Go side derives, and the trigger it installs gives the next
// project the same three.
func TestSavedSearchesMigrationBackfillsExistingProjects(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	up, err := migrationsFS.ReadFile("migrations/0308_saved_searches.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(label, sql string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}
	// The 0014 projects shape, without its workspace foreign key.
	exec("scratch schema", `
		CREATE SCHEMA migration_0308_saved_searches;
		SET LOCAL search_path TO migration_0308_saved_searches;
		CREATE TABLE projects (id text PRIMARY KEY, tenant_id text NOT NULL, name text NOT NULL,
			created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now());`)
	existing := []string{ids.New(ids.Project), ids.New(ids.Project)}
	for _, id := range existing {
		exec("existing project", `INSERT INTO projects (id, tenant_id, name) VALUES ($1, 'tea-a', $1)`, id)
	}
	exec("apply 0308", string(up))
	fresh := ids.New(ids.Project)
	exec("new project", `INSERT INTO projects (id, tenant_id, name) VALUES ($1, 'tea-a', 'fresh')`, fresh)

	for _, project := range append(existing, fresh) {
		rows, err := tx.Query(ctx, `SELECT s.id, s.project_id, s.name, s.type, s.filters FROM saved_searches s
			WHERE s.project_id = $1 `+savedSearchOrder, project)
		if err != nil {
			t.Fatal(err)
		}
		var views []SavedSearch
		for rows.Next() {
			var v SavedSearch
			if err := rows.Scan(&v.ID, &v.ProjectID, &v.Name, &v.Type, &v.Filters); err != nil {
				t.Fatal(err)
			}
			views = append(views, v)
		}
		rows.Close()
		assertDefaultViews(t, project, views)
	}
}
