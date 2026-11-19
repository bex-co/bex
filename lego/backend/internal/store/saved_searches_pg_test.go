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
	"fmt"
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

// A project's views are written in place (w3/m507): a new view gets a fresh
// svs- id and reads back with its workspace, a name stays unique in its
// project on create and on edit, a default view can be edited and deleted,
// and a project holds at most MaxSavedSearchesPerProject views.
func TestPGSavedSearchWrites(t *testing.T) {
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
	other, err := st.CreateProject(ctx, tenant.ID, "elsewhere")
	if err != nil {
		t.Fatal(err)
	}

	doc := json.RawMessage(`{"filters": [{"field": "LEVEL", "operator": "IS", "values": ["error"]}], "version": 1}`)
	created, err := st.CreateSavedSearch(ctx, SavedSearch{ProjectID: project.ID, TenantID: tenant.ID, Name: "Errors", Type: "APPLICATION_LOGS", Filters: doc})
	if err != nil {
		t.Fatal(err)
	}
	if kind, ok := ids.KindOf(created.ID); !ok || kind != ids.SavedSearch || created.Name != "Errors" ||
		created.ProjectID != project.ID || created.Type != "APPLICATION_LOGS" {
		t.Fatalf("created %+v", created)
	}
	var stored, want any
	_ = json.Unmarshal(created.Filters, &stored)
	_ = json.Unmarshal(doc, &want)
	if fmt.Sprint(stored) != fmt.Sprint(want) {
		t.Fatalf("created filters %s, want %s", created.Filters, doc)
	}
	views, err := st.ListSavedSearches(ctx, project.ID, "")
	if err != nil || len(views) != 4 || views[3].ID != created.ID || views[3].TenantID != tenant.ID {
		t.Fatalf("the project lists %+v, %v; want the defaults then the new view", views, err)
	}

	// Render's duplicate_saved_search_name: unique in the project, not across projects.
	if _, err := st.CreateSavedSearch(ctx, SavedSearch{ProjectID: project.ID, Name: "Errors", Type: "REQUEST_LOGS", Filters: doc}); !errors.Is(err, ErrConflict) {
		t.Fatalf("a taken name: %v, want ErrConflict", err)
	}
	if _, err := st.CreateSavedSearch(ctx, SavedSearch{ProjectID: other.ID, Name: "Errors", Type: "APPLICATION_LOGS", Filters: doc}); err != nil {
		t.Fatalf("the same name in another project: %v", err)
	}
	if _, err := st.CreateSavedSearch(ctx, SavedSearch{ProjectID: ids.New(ids.Project), Name: "Orphan", Type: "APPLICATION_LOGS", Filters: doc}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a view of no project: %v, want ErrNotFound", err)
	}

	// A default view is edited in place; its type and project stay.
	logs := views[0]
	edited, err := st.UpdateSavedSearch(ctx, SavedSearch{ID: logs.ID, TenantID: tenant.ID, Name: "All logs", Filters: json.RawMessage(`{"filters": [], "version": 1}`)})
	if err != nil || edited.ID != logs.ID || edited.Name != "All logs" || edited.Type != "APPLICATION_LOGS" || !edited.UpdatedAt.After(logs.UpdatedAt) {
		t.Fatalf("edited %+v, %v", edited, err)
	}
	if _, err := st.UpdateSavedSearch(ctx, SavedSearch{ID: logs.ID, Name: "Errors", Filters: logs.Filters}); !errors.Is(err, ErrConflict) {
		t.Fatalf("an edit onto a taken name: %v, want ErrConflict", err)
	}
	if _, err := st.UpdateSavedSearch(ctx, SavedSearch{ID: ids.New(ids.SavedSearch), Name: "Ghost", Filters: logs.Filters}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("editing an unknown view: %v, want ErrNotFound", err)
	}

	// A default view is deleted like any other.
	if err := st.DeleteSavedSearch(ctx, logs.ID); err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteSavedSearch(ctx, logs.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting a deleted view: %v, want ErrNotFound", err)
	}
	if _, err := st.GetSavedSearch(ctx, logs.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a deleted view reads %v, want ErrNotFound", err)
	}

	// The bound counts the project's own views only.
	views, _ = st.ListSavedSearches(ctx, project.ID, "")
	for i := len(views); i < MaxSavedSearchesPerProject; i++ {
		if _, err := st.CreateSavedSearch(ctx, SavedSearch{ProjectID: project.ID, Name: fmt.Sprintf("view %d", i), Type: "REQUEST_LOGS", Filters: doc}); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.CreateSavedSearch(ctx, SavedSearch{ProjectID: project.ID, Name: "one too many", Type: "REQUEST_LOGS", Filters: doc}); !errors.Is(err, ErrSavedSearchLimit) {
		t.Fatalf("a view past the bound: %v, want ErrSavedSearchLimit", err)
	}
	if _, err := st.CreateSavedSearch(ctx, SavedSearch{ProjectID: other.ID, Name: "still room", Type: "REQUEST_LOGS", Filters: doc}); err != nil {
		t.Fatalf("another project is not bounded by this one: %v", err)
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
