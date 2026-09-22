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
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// w2/m96 t001: the retained-name record is what lets a charge line name a
// resource that no longer exists. Its whole value rests on four behaviours —
// written on create, moved forward on rename, surviving the resource's
// deletion, and purged with the workspace — so each is asserted against real
// Postgres rather than a fake.
func TestResourceDisplayNamesPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE resource_display_names, apps, tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)

	tenant, err := st.CreateWorkspace(ctx, "names", PlanPro, "identity-a")
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateWorkspace(ctx, "other", PlanPro, "identity-b")
	if err != nil {
		t.Fatal(err)
	}

	nameOf := func(t *testing.T, tenantID, kind, id string) string {
		t.Helper()
		got, err := st.ResourceDisplayNames(ctx, tenantID, []ResourceDisplayName{{Kind: kind, ID: id}})
		if err != nil {
			t.Fatalf("ResourceDisplayNames: %v", err)
		}
		return got[ResourceDisplayNameKey(kind, id)]
	}

	// Create writes the row, in the app's own transaction.
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "checkout", Type: "web_service", Image: "nginx:1", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateApp: %v", err)
	}
	if got := nameOf(t, tenant.ID, ResourceKindService, app.ID); got != "checkout" {
		t.Fatalf("after create = %q, want checkout", got)
	}

	// Rename moves it forward, so the resource bills under its current name.
	if err := st.SetAppDisplayName(ctx, app.ID, "checkout-api"); err != nil {
		t.Fatalf("SetAppDisplayName: %v", err)
	}
	if got := nameOf(t, tenant.ID, ResourceKindService, app.ID); got != "checkout-api" {
		t.Fatalf("after rename = %q, want checkout-api", got)
	}
	// Clearing the display name falls back to apps.name, the same rule readers use.
	if err := st.SetAppDisplayName(ctx, app.ID, ""); err != nil {
		t.Fatalf("SetAppDisplayName(clear): %v", err)
	}
	if got := nameOf(t, tenant.ID, ResourceKindService, app.ID); got != "checkout" {
		t.Fatalf("after clearing the display name = %q, want the fallback checkout", got)
	}

	// The point of the record: deleting the resource must NOT delete the name.
	if _, err := pool.Exec(ctx, `DELETE FROM apps WHERE id = $1`, app.ID); err != nil {
		t.Fatalf("delete app: %v", err)
	}
	if got := nameOf(t, tenant.ID, ResourceKindService, app.ID); got != "checkout" {
		t.Fatalf("after the resource was deleted = %q, want the retained checkout", got)
	}

	// Same display name, new id: two rows, so the two lifetimes bill separately.
	reborn, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "checkout", Type: "web_service", Image: "nginx:1", Replicas: 1})
	if err != nil {
		t.Fatalf("CreateApp(reborn): %v", err)
	}
	if reborn.ID == app.ID {
		t.Fatal("the re-created app reused the deleted id")
	}
	var rows int
	if err := pool.QueryRow(ctx,
		`SELECT count(*) FROM resource_display_names WHERE tenant_id=$1 AND resource_kind=$2 AND display_name=$3`,
		tenant.ID, ResourceKindService, "checkout").Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if rows != 2 {
		t.Fatalf("rows named checkout = %d, want 2 (one per id)", rows)
	}

	// Batch upsert: overwrite, insert, and ignore blanks.
	if err := st.RecordResourceDisplayNames(ctx, tenant.ID, []ResourceDisplayName{
		{Kind: ResourceKindService, ID: app.ID, Name: "renamed-after-death"},
		{Kind: ResourceKindSandbox, ID: "sbx-1", Name: "bex-co/bex (main)"},
		{Kind: ResourceKindPostgres, ID: "", Name: "no id"},
		{Kind: ResourceKindPostgres, ID: "dpg-1", Name: ""},
		// A duplicate inside one batch must not blow up ON CONFLICT.
		{Kind: ResourceKindSandbox, ID: "sbx-1", Name: "ignored duplicate"},
	}); err != nil {
		t.Fatalf("RecordResourceDisplayNames: %v", err)
	}
	if got := nameOf(t, tenant.ID, ResourceKindService, app.ID); got != "renamed-after-death" {
		t.Fatalf("batch overwrite = %q", got)
	}
	if got := nameOf(t, tenant.ID, ResourceKindSandbox, "sbx-1"); got != "bex-co/bex (main)" {
		t.Fatalf("batch insert = %q", got)
	}
	if got := nameOf(t, tenant.ID, ResourceKindPostgres, "dpg-1"); got != "" {
		t.Fatalf("a blank name was stored as %q", got)
	}

	// Tenant scoping: another workspace sees none of it.
	if got := nameOf(t, other.ID, ResourceKindService, app.ID); got != "" {
		t.Fatalf("cross-tenant read = %q, want empty", got)
	}

	// Workspace deletion purges the record through the tenants cascade.
	if err := st.DeleteTenant(ctx, tenant.ID); err != nil {
		t.Fatalf("DeleteTenant: %v", err)
	}
	var left int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM resource_display_names WHERE tenant_id = $1`, tenant.ID).Scan(&left); err != nil {
		t.Fatal(err)
	}
	if left != 0 {
		t.Fatalf("rows left after workspace deletion = %d, want 0", left)
	}
}

// SandboxLabels is what names a metered sandbox UUID: the agent session that
// owns it, or — once it has been rehydrated onto a new sandbox — the dispatch
// that records the one it came from.
func TestSandboxLabelsPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE agent_session_dispatches, agent_sessions, tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)

	tenant, err := st.CreateWorkspace(ctx, "labels", PlanPro, "identity-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO agent_sessions (id, workspace_id, repo, branch, sandbox_id, phase)
		 VALUES ('ags-1', $1, 'bex-co/bex', 'main', 'sbx-current', 'completed'),
		        ('ags-2', $1, 'acme/site', '', 'sbx-nobranch', 'completed')`, tenant.ID); err != nil {
		t.Fatalf("seed sessions: %v", err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO agent_session_dispatches (session_id, turn, workspace_id, previous_sandbox_id)
		 VALUES ('ags-1', 1, $1, 'sbx-previous')`, tenant.ID); err != nil {
		t.Fatalf("seed dispatch: %v", err)
	}

	got, err := st.SandboxLabels(ctx, tenant.ID, []string{"sbx-current", "sbx-previous", "sbx-nobranch", "sbx-unknown"})
	if err != nil {
		t.Fatalf("SandboxLabels: %v", err)
	}
	for id, want := range map[string]string{
		"sbx-current":  "bex-co/bex (main)",
		"sbx-previous": "bex-co/bex (main)",
		"sbx-nobranch": "acme/site",
	} {
		if got[id] != want {
			t.Errorf("%s = %q, want %q", id, got[id], want)
		}
	}
	if label, ok := got["sbx-unknown"]; ok {
		t.Errorf("a sandbox with no session resolved to %q, want absent", label)
	}
}

// LiveSandboxes answers the question SandboxLabels deliberately cannot: whether
// the sandbox behind a charge line still exists. Three outcomes, not two —
// running, terminated, and never metered (w4/129).
func TestLiveSandboxesPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, `TRUNCATE sandbox_meter_states, tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)

	tenant, err := st.CreateWorkspace(ctx, "phases", PlanPro, "identity-a")
	if err != nil {
		t.Fatal(err)
	}
	other, err := st.CreateWorkspace(ctx, "phases-other", PlanPro, "identity-b")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx,
		`INSERT INTO sandbox_meter_states (workspace_id, sandbox_id, phase, tier, weight_milli, observed_at)
		 VALUES ($1, 'sbx-running', 'running', 'standard', 1000, now()),
		        ($1, 'sbx-terminated', 'terminated', 'standard', 1000, now()),
		        ($2, 'sbx-other-workspace', 'running', 'standard', 1000, now())`,
		tenant.ID, other.ID); err != nil {
		t.Fatalf("seed meter states: %v", err)
	}

	got, err := st.LiveSandboxes(ctx, tenant.ID,
		[]string{"sbx-running", "sbx-terminated", "sbx-never-metered", "sbx-other-workspace"})
	if err != nil {
		t.Fatalf("LiveSandboxes: %v", err)
	}
	if live, ok := got["sbx-running"]; !ok || !live {
		t.Errorf("sbx-running = %v (present %v), want true", live, ok)
	}
	if live, ok := got["sbx-terminated"]; !ok || live {
		t.Errorf("sbx-terminated = %v (present %v), want false", live, ok)
	}
	// Absent, not false: nothing is known, so nothing may be claimed.
	if live, ok := got["sbx-never-metered"]; ok {
		t.Errorf("a sandbox with no meter state resolved to %v, want absent", live)
	}
	// Another workspace's sandbox is not this workspace's business.
	if live, ok := got["sbx-other-workspace"]; ok {
		t.Errorf("another workspace's sandbox resolved to %v, want absent", live)
	}
}
