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

// w9/098: every plan change keeps the instant a workspace's audit history
// starts in step. Hobby -> Pro stamps the change; Pro -> Scale keeps it; a
// downgrade keeps it and the next upgrade moves it forward; a workspace
// created on Pro has none.
func TestPGPlanChangesStampTheAuditHistoryStart(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	ws, err := st.CreateTenant(ctx, "ws-audit-visible-from", PlanHobby)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, ws.ID) })
	near := func(got, want time.Time) bool { return got.Sub(want).Abs() < time.Minute }

	if plan, got, err := st.WorkspaceAuditPlan(ctx, ws.ID); err != nil || plan != PlanHobby || !got.IsZero() {
		t.Fatalf("new workspace = %q, start %s, %v; want hobby with no start", plan, got, err)
	}

	upgradedAt := time.Now()
	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanPro); err != nil {
		t.Fatal(err)
	}
	plan, start, err := st.WorkspaceAuditPlan(ctx, ws.ID)
	if err != nil || plan != PlanPro || !near(start, upgradedAt) {
		t.Fatalf("after hobby->pro = %q, start %s, %v; want pro starting at the upgrade", plan, start, err)
	}

	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanScale); err != nil {
		t.Fatal(err)
	}
	if _, got, err := st.WorkspaceAuditPlan(ctx, ws.ID); err != nil || !got.Equal(start) {
		t.Fatalf("after pro->scale the start = %s, %v; want the kept %s", got, err, start)
	}
	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanHobby); err != nil {
		t.Fatal(err)
	}
	if _, got, err := st.WorkspaceAuditPlan(ctx, ws.ID); err != nil || !got.Equal(start) {
		t.Fatalf("after scale->hobby the start = %s, %v; want the kept %s", got, err, start)
	}
	if _, err := st.UpdateTenantPlan(ctx, ws.ID, PlanPro); err != nil {
		t.Fatal(err)
	}
	if _, got, err := st.WorkspaceAuditPlan(ctx, ws.ID); err != nil || !got.After(start) {
		t.Fatalf("after a second hobby->pro the start = %s, %v; want later than %s", got, err, start)
	}

	pro, err := st.CreateTenant(ctx, "ws-audit-visible-from-pro", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(ctx, pro.ID) })
	if _, err := st.UpdateTenantPlan(ctx, pro.ID, PlanScale); err != nil {
		t.Fatal(err)
	}
	if _, got, err := st.WorkspaceAuditPlan(ctx, pro.ID); err != nil || !got.IsZero() {
		t.Fatalf("a workspace created on Pro, now Scale, starts at %s, %v; want no start", got, err)
	}

	if _, _, err := st.WorkspaceAuditPlan(ctx, "tea-absent"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an unknown workspace's start = %v, want ErrNotFound", err)
	}
}

// Migration 0292 adds the nullable column without touching existing rows (a
// workspace from before it has no start, so its history stays visible), and
// its down path removes it.
func TestWorkspaceAuditVisibleFromMigrationAppliesAndRollsBack(t *testing.T) {
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

	up, err := migrationsFS.ReadFile("migrations/0292_workspace_audit_visible_from.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0292_workspace_audit_visible_from.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0292_audit_visible_from;
		SET LOCAL search_path TO migration_0292_audit_visible_from;
		CREATE TABLE tenants (id text PRIMARY KEY, plan text NOT NULL DEFAULT 'hobby');
		INSERT INTO tenants (id, plan) VALUES ('tea-pre', 'pro');
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply migration 0292: %v", err)
	}
	var visibleFrom *time.Time
	if err := tx.QueryRow(ctx, `SELECT audit_visible_from FROM tenants WHERE id = 'tea-pre'`).Scan(&visibleFrom); err != nil {
		t.Fatalf("read back the pre-existing workspace: %v", err)
	}
	if visibleFrom != nil {
		t.Errorf("a workspace from before 0292 starts at %s, want NULL", visibleFrom)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("roll back migration 0292: %v", err)
	}
	var stillThere bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='migration_0292_audit_visible_from'
			  AND table_name='tenants'
			  AND column_name='audit_visible_from'
		)`).Scan(&stillThere); err != nil {
		t.Fatal(err)
	}
	if stillThere {
		t.Error("audit_visible_from survived the down migration")
	}
}
