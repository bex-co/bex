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

package postgres

// m133_declared_parameters_test.go — w6/m133. The dashboard's parameter editor
// was seeded from the pg_settings view, so a database nobody had configured
// arrived with ~48 operator-owned rows (CloudNativePG's archive/restore
// commands, the TLS paths, the replication settings) presented as the tenant's
// own editable overrides. Because a save is a full replacement, removing one row
// wrote all the rest into spec.parameters.
//
// Two halves are proven here THROUGH THE API rather than through the service
// methods, because the milestone's definition of done asks for exactly that: the
// declared set must be readable, and the operator-owned names must be refused
// even by a caller that never opens the dashboard.

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestDeclaredParametersAreReadableOverREST closes the write-only gap: before
// this, spec.parameters could be set and never read back, on any surface.
func TestDeclaredParametersAreReadableOverREST(t *testing.T) {
	svc, _ := newService()

	code, pg := createPG(t, svc, `{"name":"declared-pg","ownerId":"tea-1","plan":"free"}`)
	if code != http.StatusCreated {
		t.Fatalf("create => %d", code)
	}

	// A fresh database declares nothing. The pg_settings view would report ~48
	// rows for this same database — that difference is the whole milestone.
	if pg.ParameterOverrides != nil {
		t.Errorf("a fresh database declares %v, want no parameterOverrides key", pg.ParameterOverrides)
	}
	rec := serveREST(svc, http.MethodGet, "/v1/postgres/"+pg.ID+"/parameters", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /parameters => %d %s", rec.Code, rec.Body.String())
	}
	var fresh []ParameterSpecView
	if err := json.Unmarshal(rec.Body.Bytes(), &fresh); err != nil {
		t.Fatalf("decode /parameters: %v", err)
	}
	if len(fresh) != 0 {
		t.Errorf("GET /parameters on a fresh database = %v, want []", fresh)
	}

	// Declare two, then read them back on both the dedicated route and the
	// Postgres object itself (Render's postgresDetail carries the same key).
	put := serveREST(svc, http.MethodPut, "/v1/postgres/"+pg.ID+"/parameter-overrides",
		`{"parameters":{"work_mem":"16MB","max_connections":"200"}}`)
	if put.Code != http.StatusOK {
		t.Fatalf("PUT parameter-overrides => %d %s", put.Code, put.Body.String())
	}

	rec = serveREST(svc, http.MethodGet, "/v1/postgres/"+pg.ID+"/parameters", "")
	var declared []ParameterSpecView
	if err := json.Unmarshal(rec.Body.Bytes(), &declared); err != nil {
		t.Fatalf("decode /parameters: %v", err)
	}
	want := []ParameterSpecView{
		{Name: "max_connections", Value: "200"},
		{Name: "work_mem", Value: "16MB"},
	}
	if len(declared) != len(want) {
		t.Fatalf("GET /parameters = %v, want %v", declared, want)
	}
	for i := range want {
		if declared[i] != want[i] {
			t.Errorf("parameter %d = %+v, want %+v", i, declared[i], want[i])
		}
	}

	rec = serveREST(svc, http.MethodGet, "/v1/postgres/"+pg.ID, "")
	var detail PostgresView
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if detail.ParameterOverrides["work_mem"] != "16MB" || detail.ParameterOverrides["max_connections"] != "200" {
		t.Errorf("detail parameterOverrides = %v, want the two declared values", detail.ParameterOverrides)
	}
	// Exactly the declared set — not the observed configuration.
	if len(detail.ParameterOverrides) != 2 {
		t.Errorf("detail parameterOverrides has %d keys, want exactly the 2 declared", len(detail.ParameterOverrides))
	}
}

// TestOperatorParametersRefusedOverREST proves the guard from outside the
// dashboard, which is what the milestone's DoD asks for: the editor rebind alone
// would leave the API accepting these from any other client.
func TestOperatorParametersRefusedOverREST(t *testing.T) {
	svc, _ := newService()
	code, pg := createPG(t, svc, `{"name":"guard-rest-pg","ownerId":"tea-1","plan":"free"}`)
	if code != http.StatusCreated {
		t.Fatalf("create => %d", code)
	}

	for _, body := range []string{
		// The two the DoD names.
		`{"parameters":{"restore_command":"/bin/false %f %p"}}`,
		`{"parameters":{"ssl_key_file":"/tmp/attacker.key"}}`,
		// Backups, TLS and the ALTER SYSTEM escape hatch.
		`{"parameters":{"archive_command":"/bin/true"}}`,
		`{"parameters":{"allow_alter_system":"on"}}`,
		// Mixed with a legitimate one: the whole write is refused, not partly applied.
		`{"parameters":{"work_mem":"16MB","wal_level":"minimal"}}`,
	} {
		rec := serveREST(svc, http.MethodPut, "/v1/postgres/"+pg.ID+"/parameter-overrides", body)
		if rec.Code != http.StatusBadRequest {
			t.Errorf("PUT %s => %d, want 400", body, rec.Code)
		}
	}

	// Nothing landed.
	rec := serveREST(svc, http.MethodGet, "/v1/postgres/"+pg.ID, "")
	var detail PostgresView
	_ = json.Unmarshal(rec.Body.Bytes(), &detail)
	if len(detail.ParameterOverrides) != 0 {
		t.Errorf("refused writes still landed: %v", detail.ParameterOverrides)
	}

	// The same guard covers PATCH, not just the dedicated PUT route.
	patch := serveREST(svc, http.MethodPatch, "/v1/postgres/"+pg.ID,
		`{"parameterOverrides":{"archive_mode":"off"}}`)
	if patch.Code != http.StatusBadRequest {
		t.Errorf("PATCH with an operator parameter => %d, want 400", patch.Code)
	}
	if !strings.Contains(patch.Body.String(), "archive_mode") {
		t.Errorf("PATCH refusal should name the parameter: %s", patch.Body.String())
	}

	// And a legitimate tuning write still succeeds through the same path.
	ok := serveREST(svc, http.MethodPut, "/v1/postgres/"+pg.ID+"/parameter-overrides",
		`{"parameters":{"work_mem":"16MB"}}`)
	if ok.Code != http.StatusOK {
		t.Fatalf("legitimate parameter => %d %s", ok.Code, ok.Body.String())
	}
}
