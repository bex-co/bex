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
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestPGWorkspaceStateCascadeMigrationRoundTrip(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tx, err := st.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := tx.Exec(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	migrate := func(direction string) {
		t.Helper()
		migration, err := migrationsFS.ReadFile("migrations/0133_workspace_state_cascades." + direction + ".sql")
		if err != nil {
			t.Fatal(err)
		}
		exec(string(migration))
	}
	audit := func(workspaceID string) {
		t.Helper()
		exec(`INSERT INTO audit_events (id, workspace_id, verb, resource, target, outcome, at)
			VALUES ($1, $2, 'Update', $3, $4, 'allowed', now())`,
			ids.New(ids.Audit), workspaceID, core.WorkspaceObject(workspaceID), "database:"+ids.New(ids.Postgres))
	}
	assertRows := func(workspaceID string, stateRows, auditRows, dispatchRows int) {
		t.Helper()
		for _, table := range []struct {
			name, column string
			want         int
		}{
			{"jobs", "tenant_id", stateRows},
			{"datastore_event_facts", "workspace_id", stateRows},
			{"datastore_observed_checkpoints", "workspace_id", stateRows},
			{"service_event_index", "workspace_id", stateRows * 2},
			{"audit_events", "workspace_id", auditRows},
			{"agent_session_dispatches", "workspace_id", dispatchRows},
		} {
			var count int
			if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{table.name}.Sanitize()+
				" WHERE "+pgx.Identifier{table.column}.Sanitize()+" = $1", workspaceID).Scan(&count); err != nil {
				t.Fatal(err)
			}
			if count != table.want {
				t.Errorf("%s rows for %s = %d, want %d", table.name, workspaceID, count, table.want)
			}
		}
	}

	// Execute both directions inside a transaction: fixtures and schema changes
	// roll back together, including if an assertion fails halfway through.
	migrate("down")
	departing, remaining, orphan := ids.New(ids.Workspace), ids.New(ids.Workspace), ids.New(ids.Workspace)
	for _, workspaceID := range []string{departing, remaining} {
		exec(`INSERT INTO tenants (id, name, plan) VALUES ($1, $1, 'hobby')`, workspaceID)
	}
	for _, workspaceID := range []string{departing, remaining, orphan} {
		datastoreID := ids.New(ids.Postgres)
		exec(`INSERT INTO jobs (id, service_name, tenant_id, start_command) VALUES ($1, 'worker', $2, 'true')`, ids.New(ids.Job), workspaceID)
		exec(`INSERT INTO datastore_observed_checkpoints (datastore_id, workspace_id, kind) VALUES ($1, $2, 'postgres')`, datastoreID, workspaceID)
		exec(`INSERT INTO datastore_event_facts (source_key, workspace_id, datastore_id, kind, fact_type, at)
			VALUES ($1, $2, $3, 'postgres', 'postgres_unavailable', now())`, datastoreID+":unavailable", workspaceID, datastoreID)
		audit(workspaceID)
		exec(`INSERT INTO agent_session_dispatches (session_id, turn, workspace_id, abandoned) VALUES ($1, 1, $2, true)`, ids.New(ids.AgentSession), workspaceID)
		assertRows(workspaceID, 1, 1, 1)
	}

	migrate("up")
	assertRows(orphan, 0, 1, 1)
	assertRows(departing, 1, 1, 1)
	assertRows(remaining, 1, 1, 1)
	exec(`DELETE FROM tenants WHERE id = $1`, departing)
	assertRows(departing, 0, 1, 1)
	assertRows(remaining, 1, 1, 1)
	// Audit rows deliberately outlive the tenant, including writes arriving
	// after deletion. Their derived event projection must not resurrect it.
	audit(departing)
	assertRows(departing, 0, 2, 1)

	migrate("down")
	exec(`DELETE FROM tenants WHERE id = $1`, remaining)
	assertRows(remaining, 1, 1, 1)
	migrate("up")
	assertRows(remaining, 0, 1, 1)
	t.Log("0133 down/up/down/up: legacy orphans removed, four workspace tables cascade, remaining workspace survives, audit/dispatch retention preserved")
}

func TestPGAuditSurvivesConcurrentWorkspaceDeletion(t *testing.T) {
	st := newReplayTestStore(t)
	for _, targetKind := range []string{"database", "keyvalue", "service_id", "service_name"} {
		t.Run(targetKind, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			tenant, err := st.CreateTenant(ctx, "audit-delete-"+uniqueMachineRun(), PlanHobby)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_ = st.DeleteTenant(context.Background(), tenant.ID)
				_, _ = st.Pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
			})
			target := targetKind + ":" + ids.New(ids.Postgres)
			if targetKind == "keyvalue" {
				target = "keyvalue:" + ids.New(ids.KeyValue)
			}
			if targetKind == "service_id" || targetKind == "service_name" {
				app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "audit-race-" + uniqueMachineRun(), Type: "web_service", Image: "test/image", Branch: "main", Port: 8080, Replicas: 1, Tier: "free"})
				if err != nil {
					t.Fatal(err)
				}
				target = core.ServiceTarget(app.ID)
				if targetKind == "service_name" {
					target = core.ServiceTarget(app.Name)
				}
			}

			// One dedicated writer connection identifies its blocking dependency
			// exactly; no scheduler delay stands in for observing the PG lock.
			writerConfig := st.Pool.Config()
			writerConfig.MaxConns, writerConfig.MinConns = 1, 0
			writerPool, err := pgxpool.NewWithConfig(ctx, writerConfig)
			if err != nil {
				t.Fatal(err)
			}
			defer writerPool.Close()
			var writerPID, deleterPID int32
			if err := writerPool.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&writerPID); err != nil {
				t.Fatal(err)
			}
			deleteTx, err := st.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = deleteTx.Rollback(context.Background()) }()
			if err := deleteTx.QueryRow(ctx, `SELECT pg_backend_pid()`).Scan(&deleterPID); err != nil {
				t.Fatal(err)
			}
			if _, err := deleteTx.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID); err != nil {
				t.Fatal(err)
			}
			recorded := make(chan error, 1)
			go func() {
				recorded <- NewPGStore(writerPool).Record(ctx, core.AuditEvent{
					Verb: "Update", Resource: core.WorkspaceObject(tenant.ID), Target: target,
					Outcome: core.AuditAllowed, At: time.Now().UTC(),
				})
			}()
			for {
				var blocked bool
				if err := st.Pool.QueryRow(ctx, `SELECT $2 = ANY(pg_blocking_pids($1))`, writerPID, deleterPID).Scan(&blocked); err != nil {
					t.Fatal(err)
				}
				if blocked {
					break
				}
				select {
				case err := <-recorded:
					t.Fatalf("audit completed before concurrent deletion released its lock: %v", err)
				case <-ctx.Done():
					t.Fatal(ctx.Err())
				case <-time.After(10 * time.Millisecond):
				}
			}
			if err := deleteTx.Commit(ctx); err != nil {
				t.Fatal(err)
			}
			if err := <-recorded; err != nil {
				t.Fatalf("audit lost during workspace deletion: %v", err)
			}
			var audits, projections int
			if err := st.Pool.QueryRow(ctx, `SELECT
				(SELECT count(*) FROM audit_events WHERE workspace_id = $1),
				(SELECT count(*) FROM service_event_index WHERE workspace_id = $1)`, tenant.ID).Scan(&audits, &projections); err != nil {
				t.Fatal(err)
			}
			if audits != 1 || projections != 0 {
				t.Fatalf("deleted workspace: audit=%d projection=%d, want 1 retained audit and no projection", audits, projections)
			}
		})
	}
}
