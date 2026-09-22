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
	"reflect"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// Reusable names must never reassign audit evidence. Every generation has a
// nonempty six-event feed, including the deploy and sleep/wake arms that were
// already keyed by app id, so dropping the audit arm or the entire feed fails.
func TestPGServiceEventsKeepAuditOwnershipAcrossNameReuse(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	for _, tc := range []struct {
		name        string
		generations int
		target      func(Tenant, App) string
	}{
		{"tenant_id_name", 3, func(tenant Tenant, app App) string { return core.ServiceTarget(core.CRName(tenant.ID, app.Name)) }},
		{"tenant_name", 3, func(tenant Tenant, app App) string { return core.ServiceTarget(core.CRName(tenant.Name, app.Name)) }},
		{"bare_name", 3, func(_ Tenant, app App) string { return core.ServiceTarget(app.Name) }},
		{"never_used_name", 1, func(_ Tenant, app App) string { return core.ServiceTarget(app.Name) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tenant, err := st.CreateWorkspace(ctx, "event-owner-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
				_ = st.DeleteTenant(context.Background(), tenant.ID)
			})
			base := time.Now().UTC().Truncate(time.Microsecond).Add(-24 * time.Hour)
			var previous App
			var auditIDs []string
			for generation := 0; generation < tc.generations; generation++ {
				createdAt := base.Add(time.Duration(generation) * time.Hour)
				app, err := st.CreateApp(ctx, App{
					TenantID: tenant.ID, Name: "reusable", Image: "nginx:1", Type: "web_service",
					Branch: "main", Port: 80, Replicas: 1, Tier: "starter",
				})
				if err != nil {
					t.Fatal(err)
				}
				if app.ID == previous.ID {
					t.Fatal("replacement retained the deleted app's id")
				}
				// Fixed historical timestamps make the incarnation boundary explicit
				// without sleeps or races against the database clock.
				if _, err := pool.Exec(ctx, `UPDATE apps SET created_at = $2 WHERE id = $1`, app.ID, createdAt); err != nil {
					t.Fatal(err)
				}
				startedAt, finishedAt := createdAt.Add(time.Minute), createdAt.Add(2*time.Minute)
				if _, err := pool.Exec(ctx, `
					UPDATE deploys SET created_at = $2, started_at = $2, finished_at = $3,
					    status = 'live', commit = 'abc123', commit_message = 'fixture rollout', triggered_by = 'fixture-owner'
					WHERE id = $1`, app.FirstDeployID, startedAt, finishedAt); err != nil {
					t.Fatal(err)
				}
				var factRows []ServiceEventRow
				for i, factType := range []ServiceEventFactType{EventFactServiceHibernated, EventFactServiceWoken} {
					fact := ServiceEventFact{
						SourceKey: "observed:" + app.ID + ":" + string(factType), AppID: app.ID,
						Type: factType, At: createdAt.Add(time.Duration(3+i) * time.Minute),
					}
					if inserted, err := st.InsertServiceEventFact(ctx, fact); err != nil || !inserted {
						t.Fatalf("insert %s = (%v, %v)", factType, inserted, err)
					}
					factRows = append(factRows, ServiceEventRow{Key: "fact:" + fact.SourceKey, At: fact.At, Source: EventSourceFact, FactType: string(factType)})
				}
				recordAudit := func(target, verb string, at time.Time) string {
					t.Helper()
					auditID := ids.New(ids.Audit)
					if _, err := pool.Exec(ctx, `
						INSERT INTO audit_events (id, workspace_id, resource, target, verb, caller, outcome, at)
						VALUES ($1, $2, $3, $4, $5, 'fixture-owner', 'allowed', $6)`,
						auditID, tenant.ID, core.WorkspaceObject(tenant.ID), target, verb, at); err != nil {
						t.Fatal(err)
					}
					auditIDs = append(auditIDs, auditID)
					return auditID
				}
				legacyID := recordAudit(tc.target(tenant, app), "apps.Restart", createdAt.Add(5*time.Minute))
				typedID := recordAudit(core.ServiceTarget(app.ID), "apps.Scale", createdAt.Add(6*time.Minute))
				if previous.ID != "" {
					// Delayed legacy evidence still belongs to the previous generation.
					recordAudit(tc.target(tenant, app), "apps.Restart", createdAt.Add(-time.Minute))
					// A late writer still holding the deleted immutable id cannot
					// attach to the replacement even with a fresh event timestamp.
					recordAudit(core.ServiceTarget(previous.ID), "apps.Scale", createdAt.Add(7*time.Minute))
				}
				want := []ServiceEventRow{
					{Key: typedID + ":", At: createdAt.Add(6 * time.Minute), Source: EventSourceAudit, Verb: "apps.Scale", Caller: "fixture-owner"},
					{Key: legacyID + ":", At: createdAt.Add(5 * time.Minute), Source: EventSourceAudit, Verb: "apps.Restart", Caller: "fixture-owner"},
					factRows[1], factRows[0],
					{Key: app.FirstDeployID + ":ended", At: finishedAt, Source: EventSourceDeploy, Phase: EventPhaseEnded,
						DeployID: app.FirstDeployID, Status: DeployLive, Image: "nginx:1", CommitID: "abc123", CommitMessage: "fixture rollout",
						Caller: "fixture-owner", StartedAt: &startedAt, FinishedAt: &finishedAt},
					{Key: app.FirstDeployID + ":started", At: startedAt, Source: EventSourceDeploy, Phase: EventPhaseStarted,
						DeployID: app.FirstDeployID, Trigger: TriggerCreate, Image: "nginx:1", CommitID: "abc123", CommitMessage: "fixture rollout",
						Caller: "fixture-owner", StartedAt: &startedAt, FinishedAt: &finishedAt},
				}
				filter := ServiceEventFilter{
					Verbs: []string{"apps.Restart", "apps.Scale"}, Phases: []string{EventPhaseStarted, EventPhaseEnded},
					FactTypes: []string{string(EventFactServiceHibernated), string(EventFactServiceWoken)}, Limit: 100,
				}
				page, err := st.ListServiceEvents(ctx, app.ID, tenant.ID, filter)
				if err != nil {
					t.Fatal(err)
				}
				if len(page) != len(want) {
					t.Fatalf("generation %d has %d events, want six own events: %+v", generation, len(page), page)
				}
				for i, row := range page {
					if got := serviceEventIdentityUTC(row); !reflect.DeepEqual(got, want[i]) {
						t.Errorf("generation %d row %d changed ownership, data, or order:\ngot  %+v\nwant %+v", generation, i, got, want[i])
					}
					lookup, err := st.GetServiceEvent(ctx, tenant.ID, ids.Derive(ids.Event, row.Key))
					if err != nil {
						t.Fatalf("generation %d listed unresolvable event %s: %v", generation, row.Key, err)
					}
					if lookup.ServiceID != app.ID || !reflect.DeepEqual(lookup.Event, row) {
						t.Errorf("by-id changed owner or payload: %+v; want app %s and %+v", lookup, app.ID, row)
					}
				}
				hooks, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), filter.Verbs, []string{tenant.ID}, 100)
				if err != nil {
					t.Fatal(err)
				}
				if len(hooks) != len(want) {
					t.Fatalf("generation %d webhook feed has %d rows, want six own events: %+v", generation, len(hooks), hooks)
				}
				byKey := make(map[string]ServiceEventRow, len(want))
				for _, row := range want {
					byKey[row.Key] = row
				}
				for i, row := range hooks {
					if row.AppID != app.ID || row.TenantID != tenant.ID || row.ServiceName != app.Name || row.ServiceID != core.CRName(tenant.Name, app.Name) {
						t.Errorf("webhook attributed event to the wrong service: %+v", row)
					}
					expected, ok := byKey[row.Key]
					if !ok || !row.At.Equal(expected.At) || row.Source != expected.Source || row.Phase != expected.Phase ||
						row.DeployID != expected.DeployID || row.Status != expected.Status || row.Verb != expected.Verb || row.FactType != expected.FactType {
						t.Errorf("webhook source or payload differs from the own-service feed: %+v; want %+v", row, expected)
					}
					delete(byKey, row.Key)
					if i > 0 && (row.CursorAt.Before(hooks[i-1].CursorAt) || (row.CursorAt.Equal(hooks[i-1].CursorAt) && row.Key <= hooks[i-1].Key)) {
						t.Errorf("webhook keyset order is not strictly ascending: %+v then %+v", hooks[i-1], row)
					}
				}
				if len(byKey) != 0 {
					t.Errorf("webhook feed omitted own events: %+v", byKey)
				}
				// App deletion removes projections and deploy/fact rows, while the
				// raw audit history survives each reuse of the same name.
				if err := st.DeleteApp(ctx, app.ID); err != nil {
					t.Fatal(err)
				}
				var retained int
				if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE id = ANY($1)`, auditIDs).Scan(&retained); err != nil || retained != len(auditIDs) {
					t.Fatalf("generation %d retained %d/%d raw audit rows after deletion (err %v)", generation, retained, len(auditIDs), err)
				}
				previous = app
			}
		})
	}
}

func serviceEventIdentityUTC(row ServiceEventRow) ServiceEventRow {
	row.At = row.At.UTC()
	if row.StartedAt != nil {
		at := row.StartedAt.UTC()
		row.StartedAt = &at
	}
	if row.FinishedAt != nil {
		at := row.FinishedAt.UTC()
		row.FinishedAt = &at
	}
	return row
}

func TestPGServiceEventsDoNotReinterpretDeletedIDsAsNames(t *testing.T) {
	ctx := context.Background()
	st, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	tenant, err := st.CreateWorkspace(ctx, "event-id-name-"+ids.New(ids.Workspace), PlanHobby, "fixture-owner")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM audit_events WHERE workspace_id = $1`, tenant.ID)
		_ = st.DeleteTenant(context.Background(), tenant.ID)
	})
	old, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "original", Image: "nginx:1", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.DeleteApp(ctx, old.ID); err != nil {
		t.Fatal(err)
	}
	// Service ids are also legal public names. A stale typed target must not
	// fall through to the legacy bare-name compatibility branch.
	replacement, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: old.ID, Image: "nginx:1", Branch: "main", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	for _, audit := range []struct{ target, caller string }{{old.ID, "deleted-owner"}, {replacement.ID, "replacement-owner"}} {
		if err := st.Record(ctx, core.AuditEvent{
			Caller: audit.caller, Verb: "apps.Restart", Resource: core.WorkspaceObject(tenant.ID),
			Target: core.ServiceTarget(audit.target), Outcome: core.AuditAllowed, At: replacement.CreatedAt.Add(time.Second),
		}); err != nil {
			t.Fatal(err)
		}
	}
	page, err := st.ListServiceEvents(ctx, replacement.ID, tenant.ID, ServiceEventFilter{Verbs: []string{"apps.Restart"}, Limit: 100})
	if err != nil || len(page) != 1 || page[0].Caller != "replacement-owner" {
		t.Fatalf("typed-looking name inherited a deleted id's event or lost its own: %+v (err %v)", page, err)
	}
	hooks, err := st.ListWebhookEvents(ctx, time.Time{}, "", time.Now().Add(time.Minute), []string{"apps.Restart"}, []string{tenant.ID}, 100)
	if err != nil {
		t.Fatal(err)
	}
	var audits int
	for _, row := range hooks {
		if row.Source == EventSourceAudit {
			audits++
			if row.Key != page[0].Key || row.AppID != replacement.ID || row.TenantID != tenant.ID {
				t.Errorf("webhook reinterpreted a stale immutable id: %+v", row)
			}
		}
	}
	if audits != 1 {
		t.Fatalf("webhook feed has %d audit rows, want only the replacement's own row", audits)
	}
	var retained int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE workspace_id = $1`, tenant.ID).Scan(&retained); err != nil || retained != 2 {
		t.Fatalf("raw audit evidence was deleted: retained %d rows (err %v), want 2", retained, err)
	}
}

func TestPGServiceEventOwnershipMigrationPreservesAuditEvidence(t *testing.T) {
	ctx := context.Background()
	_, pool := webhookPGStore(t, ctx)
	t.Cleanup(pool.Close)
	up, err := migrationsFS.ReadFile("migrations/0132_service_event_ownership.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	// Keep historical corruption and migration DDL isolated from other tests.
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0132_ownership;
		SET LOCAL search_path TO migration_0132_ownership;
		CREATE TABLE tenants (id text PRIMARY KEY, name text NOT NULL);
		CREATE TABLE apps (id text PRIMARY KEY, tenant_id text NOT NULL, name text NOT NULL, created_at timestamptz NOT NULL);
		CREATE TABLE audit_events (id text PRIMARY KEY, workspace_id text NOT NULL, target text NOT NULL,
		    target_name text NOT NULL DEFAULT '', outcome text NOT NULL, at timestamptz NOT NULL);
		CREATE TABLE service_event_index (
		    workspace_id text NOT NULL, event_key text NOT NULL, source text NOT NULL, source_row_id text NOT NULL,
		    phase text NOT NULL, service_id text NOT NULL, service_name text NOT NULL, app_id text,
		    CONSTRAINT service_event_index_source_key UNIQUE (source, source_row_id, phase, workspace_id)
		);`); err != nil {
		t.Fatal(err)
	}
	tenantID, appID, datastoreID := ids.New(ids.Workspace), ids.New(ids.Service), ids.New(ids.Postgres)
	createdAt := time.Now().UTC().Truncate(time.Microsecond).Add(-time.Hour)
	if _, err := tx.Exec(ctx, `INSERT INTO tenants VALUES ($1, 'acme')`, tenantID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `INSERT INTO apps VALUES ($1, $2, 'web', $3)`, appID, tenantID, createdAt); err != nil {
		t.Fatal(err)
	}
	var rawIDs, retainedIDs []string
	for _, fixture := range []struct {
		target string
		at     time.Time
		appID  any
		keep   bool
	}{
		{core.ServiceTarget(core.CRName(tenantID, "web")), createdAt.Add(-time.Hour), appID, false},
		{core.ServiceTarget(core.CRName("acme", "web")), createdAt.Add(-time.Hour), appID, false},
		{core.ServiceTarget("web"), createdAt.Add(-time.Hour), appID, false},
		{core.ServiceTarget("web"), createdAt, appID, true},
		{core.ServiceTarget(appID), createdAt.Add(-time.Hour), appID, true},
		{core.DatabaseTarget(datastoreID), createdAt.Add(-time.Hour), nil, true},
	} {
		auditID := ids.New(ids.Audit)
		rawIDs = append(rawIDs, auditID)
		if fixture.keep {
			retainedIDs = append(retainedIDs, auditID)
		}
		if _, err := tx.Exec(ctx, `INSERT INTO audit_events (id, workspace_id, target, outcome, at) VALUES ($1, $2, $3, 'allowed', $4)`,
			auditID, tenantID, fixture.target, fixture.at); err != nil {
			t.Fatal(err)
		}
		serviceID := appID
		if fixture.appID == nil {
			serviceID = datastoreID
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO service_event_index (workspace_id, event_key, source, source_row_id, phase, service_id, service_name, app_id)
			VALUES ($1, $2 || ':', 'audit', $2, '', $3, 'web', $4)`, tenantID, auditID, serviceID, fixture.appID); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply ownership migration: %v", err)
	}
	var retained, raw int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM service_event_index`).Scan(&retained); err != nil || retained != len(retainedIDs) {
		t.Fatalf("migration retained %d index rows (err %v), want only valid service and datastore rows", retained, err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM service_event_index WHERE source_row_id = ANY($1)`, retainedIDs).Scan(&retained); err != nil || retained != len(retainedIDs) {
		t.Fatalf("migration removed legitimate indexed evidence: %d/%d retained (err %v)", retained, len(retainedIDs), err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM audit_events WHERE id = ANY($1)`, rawIDs).Scan(&raw); err != nil || raw != len(rawIDs) {
		t.Fatalf("migration deleted raw audit history: %d/%d rows remain (err %v)", raw, len(rawIDs), err)
	}
}
