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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w9/m389 t004: every plan change keeps the workspace's log floor in step. An
// upgrade stores the old plan's floor at that moment, so lines that had aged
// out stay hidden; a further upgrade keeps the earlier floor; a downgrade
// drops a floor its shorter window already covers; a new workspace has none.
func TestPGPlanChangesKeepTheLogFloor(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ws, err := st.CreateTenant(ctx, "ws-logs-visible-from", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, ws.ID) })
	const day = 24 * time.Hour
	near := func(got, want time.Time) bool { return got.Sub(want).Abs() < time.Minute }

	if plan, got, err := st.WorkspaceLogPlan(ctx, ws.ID); err != nil || plan != PlanHobby || !got.IsZero() {
		t.Fatalf("new workspace = %q, floor %s, %v; want hobby with no floor", plan, got, err)
	}

	upgradedAt := time.Now()
	if updated, err := st.UpdateTenantPlan(ctx, ws.ID, PlanPro); err != nil || updated.Plan != PlanPro {
		t.Fatalf("hobby->pro = %+v, %v", updated, err)
	}
	plan, hobbyFloor, err := st.WorkspaceLogPlan(ctx, ws.ID)
	if err != nil || plan != PlanPro || !near(hobbyFloor, upgradedAt.Add(-7*day)) {
		t.Fatalf("after hobby->pro = %q, floor %s, %v; want pro with the upgrade time - 7d", plan, hobbyFloor, err)
	}
	if plan, err := st.WorkspacePlan(ctx, ws.ID); err != nil || plan != PlanPro {
		t.Fatalf("WorkspacePlan = %q, %v; want pro", plan, err)
	}

	// Pro -> Scale at once: Pro's own floor (now-14d) is older, so Hobby's
	// kept floor still binds and is kept unchanged.
	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanScale); err != nil {
		t.Fatal(err)
	}
	if _, got, err := st.WorkspaceLogPlan(ctx, ws.ID); err != nil || !got.Equal(hobbyFloor) {
		t.Fatalf("after pro->scale the floor = %s, %v; want the kept %s", got, err, hobbyFloor)
	}

	// Scale -> Hobby narrows to Hobby's own seven days: the kept floor is no
	// later than that, so it is dropped.
	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanHobby); err != nil {
		t.Fatal(err)
	}
	if _, got, err := st.WorkspaceLogPlan(ctx, ws.ID); err != nil || !got.IsZero() {
		t.Fatalf("after scale->hobby the floor = %s, %v; want none", got, err)
	}

	// A floor set long ago that has lapsed is dropped at the next change.
	if _, err := st.Pool.Exec(ctx, `UPDATE tenants SET plan = 'pro', logs_visible_from = now() - interval '20 days' WHERE id = $1`, ws.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanScale); err != nil {
		t.Fatal(err)
	}
	if _, got, err := st.WorkspaceLogPlan(ctx, ws.ID); err != nil || !near(got, time.Now().Add(-14*day)) {
		t.Fatalf("pro->scale over a lapsed floor = %s, %v; want Pro's own floor at the change", got, err)
	}

	if _, err := st.UpdateTenantPlan(ctx, "tea-absent", PlanPro); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown workspace's plan change = %v, want ErrNotFound", err)
	}
	if _, _, err := st.WorkspaceLogPlan(ctx, "tea-absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown workspace's floor = %v, want ErrNotFound", err)
	}
}

// Migration 0262 adds the nullable column without touching existing rows (a
// workspace from before it reads as having no kept floor), and its down path
// removes it.
func TestWorkspaceLogsVisibleFromMigrationAppliesAndRollsBack(t *testing.T) {
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

	up, err := migrationsFS.ReadFile("migrations/0262_workspace_logs_visible_from.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0262_workspace_logs_visible_from.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0262_logs_visible_from;
		SET LOCAL search_path TO migration_0262_logs_visible_from;
		CREATE TABLE tenants (id text PRIMARY KEY, plan text NOT NULL DEFAULT 'hobby');
		INSERT INTO tenants (id, plan) VALUES ('tea-pre', 'pro');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply migration 0262: %v", err)
	}
	var visibleFrom *time.Time
	if err := tx.QueryRow(ctx, `SELECT logs_visible_from FROM tenants WHERE id = 'tea-pre'`).Scan(&visibleFrom); err != nil {
		t.Fatalf("read back the pre-existing workspace: %v", err)
	}
	if visibleFrom != nil {
		t.Errorf("a workspace from before 0262 keeps floor %s, want NULL", visibleFrom)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("roll back migration 0262: %v", err)
	}
	var stillThere bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='migration_0262_logs_visible_from'
			  AND table_name='tenants'
			  AND column_name='logs_visible_from'
		)`).Scan(&stillThere); err != nil {
		t.Fatal(err)
	}
	if stillThere {
		t.Error("logs_visible_from survived the down migration")
	}
}
