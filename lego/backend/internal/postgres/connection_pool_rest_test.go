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

// connection_pool_rest_test.go — w2/024: Render's connectionPool enum
// (pgbouncer|none) on create/PATCH/read, folded onto the pooler bool.

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// createPG POSTs the body and returns the decoded view + status.
func createPG(t *testing.T, svc *Service, body string) (int, PostgresView) {
	t.Helper()
	rec := serveREST(svc, http.MethodPost, "/v1/postgres", body)
	var pg PostgresView
	_ = json.Unmarshal(rec.Body.Bytes(), &pg)
	return rec.Code, pg
}

func specPooler(t *testing.T, cl client.Client, name string) bool {
	t.Helper()
	var got appv1alpha1.Database
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: "default", Name: name}, &got); err != nil {
		t.Fatalf("get database %s: %v", name, err)
	}
	return got.Spec.Pooler
}

func TestRESTCreatePostgresConnectionPoolEnum(t *testing.T) {
	svc, cl := newService()

	// Render's enum on create maps onto spec.pooler and round-trips in the view.
	code, pg := createPG(t, svc, `{"name":"pooled-db","plan":"free","connectionPool":"pgbouncer"}`)
	if code != http.StatusCreated {
		t.Fatalf("create pgbouncer => 201, got %d", code)
	}
	if pg.ConnectionPool != "pgbouncer" || !pg.PoolerEnabled {
		t.Fatalf("view = connectionPool %q poolerEnabled %v, want pgbouncer/true", pg.ConnectionPool, pg.PoolerEnabled)
	}
	if !specPooler(t, cl, pg.ID) {
		t.Error("spec.pooler = false, want true")
	}

	// "none" and an omitted field both mean no pooler.
	code, pg = createPG(t, svc, `{"name":"plain-db","plan":"free","connectionPool":"none"}`)
	if code != http.StatusCreated || pg.ConnectionPool != "none" || pg.PoolerEnabled {
		t.Fatalf("create none => %d view %+v", code, pg)
	}
	if specPooler(t, cl, pg.ID) {
		t.Error("spec.pooler = true, want false")
	}

	// The legacy pooler bool still works and reads back as the enum.
	code, pg = createPG(t, svc, `{"name":"legacy-db","plan":"free","pooler":true}`)
	if code != http.StatusCreated || pg.ConnectionPool != "pgbouncer" || !specPooler(t, cl, pg.ID) {
		t.Fatalf("create pooler:true => %d view %+v", code, pg)
	}

	// Both fields with identical intent are fine.
	code, pg = createPG(t, svc, `{"name":"both-db","plan":"free","pooler":true,"connectionPool":"pgbouncer"}`)
	if code != http.StatusCreated || pg.ConnectionPool != "pgbouncer" || !specPooler(t, cl, pg.ID) {
		t.Fatalf("create identical dual-field => %d view %+v", code, pg)
	}
}

func TestRESTCreatePostgresConnectionPoolNamedErrors(t *testing.T) {
	svc, cl := newService()

	for _, tt := range []struct {
		name     string
		body     string
		wantCode string
	}{
		{"contradictory", `{"name":"conflict-db","plan":"free","pooler":true,"connectionPool":"none"}`, "POSTGRES_CONNECTION_POOL_CONFLICT"},
		{"contradictory false", `{"name":"conflict-db2","plan":"free","pooler":false,"connectionPool":"pgbouncer"}`, "POSTGRES_CONNECTION_POOL_CONFLICT"},
		{"unknown enum", `{"name":"unknown-db","plan":"free","connectionPool":"pgpool"}`, "POSTGRES_CONNECTION_POOL_UNKNOWN"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			rec := serveREST(svc, http.MethodPost, "/v1/postgres", tt.body)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("=> 400, got %d: %s", rec.Code, rec.Body.String())
			}
			var body map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &body)
			if body["code"] != tt.wantCode {
				t.Errorf("error code = %v, want %s", body["code"], tt.wantCode)
			}
		})
	}
	if n := countDatabases(t, cl); n != 0 {
		t.Fatalf("rejected creates must not write a CR, got %d", n)
	}
}

func TestRESTPatchPostgresConnectionPool(t *testing.T) {
	svc, cl := newService()
	seedPGWithLabels(t, svc, "patch-pool", appv1alpha1.DatabaseSpec{Plan: "free"}, nil)

	// Render's enum toggles the pooler both ways and round-trips in the view.
	rec := serveREST(svc, http.MethodPatch, "/v1/postgres/patch-pool", `{"connectionPool":"pgbouncer"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("patch pgbouncer => 200, got %d: %s", rec.Code, rec.Body.String())
	}
	var pg PostgresView
	_ = json.Unmarshal(rec.Body.Bytes(), &pg)
	if pg.ConnectionPool != "pgbouncer" || !specPooler(t, cl, "patch-pool") {
		t.Fatalf("after pgbouncer: view %q spec %v", pg.ConnectionPool, specPooler(t, cl, "patch-pool"))
	}
	rec = serveREST(svc, http.MethodPatch, "/v1/postgres/patch-pool", `{"connectionPool":"none"}`)
	if rec.Code != http.StatusOK || specPooler(t, cl, "patch-pool") {
		t.Fatalf("patch none => %d, spec.pooler %v", rec.Code, specPooler(t, cl, "patch-pool"))
	}

	// Contradictory and unknown values are named 400s and leave the CR alone.
	for _, tt := range []struct {
		body     string
		wantCode string
	}{
		{`{"pooler":true,"connectionPool":"none"}`, "POSTGRES_CONNECTION_POOL_CONFLICT"},
		{`{"connectionPool":"supavisor"}`, "POSTGRES_CONNECTION_POOL_UNKNOWN"},
	} {
		rec = serveREST(svc, http.MethodPatch, "/v1/postgres/patch-pool", tt.body)
		if rec.Code != http.StatusBadRequest {
			t.Fatalf("patch %s => 400, got %d", tt.body, rec.Code)
		}
		var body map[string]any
		_ = json.Unmarshal(rec.Body.Bytes(), &body)
		if body["code"] != tt.wantCode {
			t.Errorf("patch %s: error code = %v, want %s", tt.body, body["code"], tt.wantCode)
		}
	}
}

func TestRESTReadAndListPostgresConnectionPool(t *testing.T) {
	svc, _ := newService()
	seedPGWithLabels(t, svc, "read-pooled", appv1alpha1.DatabaseSpec{Plan: "free", Pooler: true}, nil)
	seedPGWithLabels(t, svc, "read-plain", appv1alpha1.DatabaseSpec{Plan: "free"}, nil)

	var pg PostgresView
	rec := serveREST(svc, http.MethodGet, "/v1/postgres/read-pooled", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &pg)
	if pg.ConnectionPool != "pgbouncer" {
		t.Errorf("get connectionPool = %q, want pgbouncer", pg.ConnectionPool)
	}

	var page []postgresWithCursor
	rec = serveREST(svc, http.MethodGet, "/v1/postgres", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("list => 200, got %d", rec.Code)
	}
	_ = json.Unmarshal(rec.Body.Bytes(), &page)
	got := map[string]string{}
	for _, item := range page {
		got[item.Postgres.Name] = item.Postgres.ConnectionPool
	}
	if got["read-pooled"] != "pgbouncer" || got["read-plain"] != "none" {
		t.Errorf("list connectionPool = %v", got)
	}
}
