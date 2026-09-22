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
	"fmt"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

var tenantScopeRetention = map[string]string{
	"audit_events.workspace_id":                "retain:security audit history; audit.Service.Run invokes PurgeAuditEvents daily/startup with the configured retention (90 days by default)",
	"ssh_sessions.workspace_id":                "retain:security session history; audit.Service.Run invokes PurgeSSHSessions with the same retention",
	"cli_telemetry_events.workspace_id":        "retain:operational telemetry; audit.Service.Run invokes PurgeCLITelemetryEvents with the same retention",
	"workspace_creation_attempts.workspace_id": "retain:provisional workspace IDs precede tenants; ExpireWorkspaceCreationAttempts removes terminal finalized/expired rows after 30 days when the Stripe workspace cleaner is enabled",
	"agent_session_dispatches.workspace_id":    "retain:indefinite tombstones preserve late sandbox/policy cleanup after session deletion (ListAgentDispatchesDue/agentsessions.Completer.recoverDispatches); no age GC exists, and deleted-workspace rows may retry without a tenant key while workspace purgers own teardown",
}

func tenantScopeDispositions(ctx context.Context, db schemaQuerier) ([]string, map[string]string, error) {
	columns, err := schemaColumns(ctx, db, "^(tenant_id|workspace_id|owner_id)$")
	if err != nil {
		return nil, nil, err
	}
	// Follow the workspace column through each FK, including composite keys.
	// A cascade on a sibling column (e.g. service_event_index.app_id) cannot
	// prove that this column names the same workspace or is protected itself.
	rows, err := db.Query(ctx, `
		SELECT child.relname || '.' || child_col.attname,
		       parent.relname || '.' || parent_col.attname
		FROM pg_constraint fk
		JOIN pg_class child ON child.oid = fk.conrelid
		JOIN pg_namespace child_ns ON child_ns.oid = child.relnamespace
		JOIN pg_class parent ON parent.oid = fk.confrelid
		JOIN pg_namespace parent_ns ON parent_ns.oid = parent.relnamespace
		CROSS JOIN LATERAL generate_subscripts(fk.conkey, 1) slot
		JOIN pg_attribute child_col
		  ON child_col.attrelid = child.oid AND child_col.attnum = fk.conkey[slot]
		JOIN pg_attribute parent_col
		  ON parent_col.attrelid = parent.oid AND parent_col.attnum = fk.confkey[slot]
		WHERE fk.contype = 'f' AND fk.confdeltype = 'c' AND fk.convalidated
		  AND child_ns.nspname = 'public' AND parent_ns.nspname = 'public'
		  AND (fk.confmatchtype = 'f' OR NOT EXISTS (
		      SELECT 1 FROM pg_attribute sibling
		      WHERE sibling.attrelid = child.oid
		        AND sibling.attnum = ANY(fk.conkey)
		        AND sibling.attnum <> child_col.attnum
		        AND NOT sibling.attnotnull
		  ))
		ORDER BY 1, 2`)
	if err != nil {
		return nil, nil, err
	}
	edges, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) ([2]string, error) {
		var edge [2]string
		err := row.Scan(&edge[0], &edge[1])
		return edge, err
	})
	if err != nil {
		return nil, nil, err
	}
	paths := map[string]string{"tenants.id": "tenants.id"}
	for changed := true; changed; {
		changed = false
		for _, edge := range edges {
			if parentPath, reachable := paths[edge[1]]; reachable && paths[edge[0]] == "" {
				paths[edge[0]] = edge[0] + " -> " + parentPath
				changed = true
			}
		}
	}
	dispositions := make(map[string]string, len(columns))
	for column, reason := range tenantScopeRetention {
		dispositions[column] = reason
	}
	for _, column := range columns {
		if path, ok := paths[column]; ok {
			if _, retained := tenantScopeRetention[column]; retained {
				return nil, nil, fmt.Errorf("%s now cascades; remove its stale retention declaration", column)
			}
			dispositions[column] = "cascade:" + path
		}
	}
	return columns, dispositions, nil
}

func TestTenantScopeCensusRejectsNullableCompositeBypass(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	tx, err := st.Pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	prefix := "tenant_census_" + uniqueMachineRun()
	parent := pgx.Identifier{prefix + "_parent"}.Sanitize()
	if _, err := tx.Exec(ctx, "CREATE TABLE "+parent+` (
		tenant_id text REFERENCES tenants(id) ON DELETE CASCADE,
		key text, PRIMARY KEY (tenant_id, key))`); err != nil {
		t.Fatal(err)
	}
	workspaceID := ids.New(ids.Workspace)
	if _, err := tx.Exec(ctx, `INSERT INTO tenants (id, name, plan) VALUES ($1, $1, 'hobby')`, workspaceID); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, "INSERT INTO "+parent+" VALUES ($1, 'key')", workspaceID); err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name, nullable, match string
		protected             bool
	}{
		{"nullable", "", "SIMPLE", false},
		{"notnull", "NOT NULL", "SIMPLE", true},
		{"full", "", "FULL", true},
	} {
		table := prefix + "_" + tc.name
		quotedTable := pgx.Identifier{table}.Sanitize()
		if _, err := tx.Exec(ctx, "CREATE TABLE "+quotedTable+" (workspace_id text, key text "+tc.nullable+
			", FOREIGN KEY (workspace_id, key) REFERENCES "+parent+" (tenant_id, key) MATCH "+tc.match+" ON DELETE CASCADE)"); err != nil {
			t.Fatal(err)
		}
		var key any = "key"
		if !tc.protected {
			key = nil
		}
		if _, err := tx.Exec(ctx, "INSERT INTO "+quotedTable+" VALUES ($1, $2)", workspaceID, key); err != nil {
			t.Fatal(err)
		}
	}
	columns, dispositions, err := tenantScopeDispositions(ctx, tx)
	if err != nil {
		t.Fatal(err)
	}
	err = checkSchemaDispositions(columns, dispositions)
	if err == nil || !strings.Contains(err.Error(), "undeclared "+prefix+"_nullable.workspace_id") {
		t.Fatalf("census accepted bypassable composite FK: %v", err)
	}
	for _, name := range []string{"notnull", "full"} {
		if !strings.HasPrefix(dispositions[prefix+"_"+name+".workspace_id"], "cascade:") {
			t.Errorf("census did not recognize protected composite FK %s", name)
		}
	}
	if _, err := tx.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, workspaceID); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"nullable", "notnull", "full"} {
		var count int
		if err := tx.QueryRow(ctx, "SELECT count(*) FROM "+pgx.Identifier{prefix + "_" + name}.Sanitize()).Scan(&count); err != nil {
			t.Fatal(err)
		}
		want := 0
		if name == "nullable" {
			want = 1
		}
		if count != want {
			t.Errorf("%s composite FK leaves %d rows after tenant deletion, want %d", name, count, want)
		}
	}
	t.Logf("seeded composite bypass rejected: %v", err)
}

func TestTenantScopeCensus(t *testing.T) {
	st := newReplayTestStore(t)
	columns, dispositions, err := tenantScopeDispositions(context.Background(), st.Pool)
	if err != nil {
		t.Fatal(err)
	}
	if err := checkSchemaDispositions(columns, dispositions); err != nil {
		t.Fatal(err)
	}
	for _, column := range []string{"git_connections.workspace_id", "sandbox_tenant_keys.workspace_id"} {
		if !strings.HasPrefix(dispositions[column], "cascade:") {
			t.Errorf("later migration's cascade not discovered for %s: %s", column, dispositions[column])
		}
	}
	path := dispositions["push_deliveries.tenant_id"]
	if !strings.Contains(path, "push_notifications.tenant_id") || !strings.Contains(path, "tenant_members.tenant_id") {
		t.Errorf("transitive notification cascade not discovered: %s", path)
	}
	t.Logf("workspace census: %d columns, %d cascading, %d explicit retention", len(columns), len(columns)-len(tenantScopeRetention), len(tenantScopeRetention))
}

func TestTenantScopeCensusRejectsSeededColumns(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	for _, tc := range []struct {
		name, definition string
		undeclared       []string
	}{
		{"no_disposition", "workspace_id text, owner_id text", []string{"workspace_id", "owner_id"}},
		{"wrong_column_cascades", "workspace_id text, tenant_id text REFERENCES tenants(id) ON DELETE CASCADE", []string{"workspace_id"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			tx, err := st.Pool.Begin(ctx)
			if err != nil {
				t.Fatal(err)
			}
			defer func() { _ = tx.Rollback(ctx) }()
			table := "tenant_census_" + uniqueMachineRun()
			if _, err := tx.Exec(ctx, "CREATE TABLE "+pgx.Identifier{table}.Sanitize()+" ("+tc.definition+")"); err != nil {
				t.Fatal(err)
			}
			columns, dispositions, err := tenantScopeDispositions(ctx, tx)
			if err != nil {
				t.Fatal(err)
			}
			err = checkSchemaDispositions(columns, dispositions)
			if err == nil {
				t.Fatal("tenant census accepted an undeclared workspace column")
			}
			for _, column := range tc.undeclared {
				if !strings.Contains(err.Error(), "undeclared "+table+"."+column) {
					t.Errorf("census error %q does not name seeded %s.%s", err, table, column)
				}
			}
			if tc.name == "wrong_column_cascades" && !strings.HasPrefix(dispositions[table+".tenant_id"], "cascade:") {
				t.Fatal("the control tenant_id cascade was not recognized")
			}
			t.Logf("seeded violation rejected: %v", err)
		})
	}
}
