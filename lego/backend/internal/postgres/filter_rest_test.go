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

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// seedPGWithLabels seeds a minimal Database CR with the given labels.
func seedPGWithLabels(t *testing.T, svc *Service, name string, spec appv1alpha1.DatabaseSpec, labels map[string]string) {
	t.Helper()
	if spec.Name == "" {
		spec.Name = name
	}
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: "default",
			Labels:    labels,
		},
		Spec: spec,
	}
	if err := svc.Client.Create(t.Context(), db); err != nil {
		t.Fatalf("seed db %s: %v", name, err)
	}
}

func listPGNames(t *testing.T, mux *http.ServeMux, query string) []string {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/postgres"+query, nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /v1/postgres%s = %d: %s", query, rec.Code, rec.Body.String())
	}
	var page []postgresWithCursor
	if err := json.Unmarshal(rec.Body.Bytes(), &page); err != nil {
		t.Fatalf("decode: %v", err)
	}
	names := make([]string, 0, len(page))
	for _, p := range page {
		names = append(names, p.Postgres.Name)
	}
	return names
}

func TestRESTListPostgresSuspendedFilter(t *testing.T) {
	// GET /v1/postgres?suspended=suspended|not_suspended (w2/m53).
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedPGWithLabels(t, svc, "active-db", appv1alpha1.DatabaseSpec{Plan: "free"}, nil)
	seedPGWithLabels(t, svc, "susp-db", appv1alpha1.DatabaseSpec{Plan: "free", Suspended: true}, nil)

	got := listPGNames(t, mux, "?suspended=suspended")
	if len(got) != 1 || got[0] != "susp-db" {
		t.Errorf("suspended=suspended = %v, want [susp-db]", got)
	}

	got = listPGNames(t, mux, "?suspended=not_suspended")
	if len(got) != 1 || got[0] != "active-db" {
		t.Errorf("suspended=not_suspended = %v, want [active-db]", got)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/postgres?suspended=true", nil))
	if rec.Code != http.StatusBadRequest {
		t.Errorf("suspended=true (invalid enum) => 400, got %d", rec.Code)
	}
}

func TestRESTListPostgresEnvironmentIDFilter(t *testing.T) {
	// GET /v1/postgres?environmentId= (w2/m53).
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedPGWithLabels(t, svc, "pg-env-a", appv1alpha1.DatabaseSpec{Plan: "free"},
		map[string]string{core.LabelEnvironment: "env-staging"})
	seedPGWithLabels(t, svc, "pg-env-b", appv1alpha1.DatabaseSpec{Plan: "free"},
		map[string]string{core.LabelEnvironment: "env-prod"})
	seedPGWithLabels(t, svc, "pg-no-env", appv1alpha1.DatabaseSpec{Plan: "free"}, nil)

	got := listPGNames(t, mux, "?environmentId=env-staging")
	if len(got) != 1 || got[0] != "pg-env-a" {
		t.Errorf("environmentId=env-staging = %v, want [pg-env-a]", got)
	}

	got = listPGNames(t, mux, "?environmentId=env-staging&environmentId=env-prod")
	if len(got) != 2 {
		t.Errorf("multiple environmentId = %v, want 2 results", got)
	}
}

func TestRESTListPostgresTimeFilters(t *testing.T) {
	// Malformed timestamp → 400; empty timestamps on CRs (fake client has no
	// CreationTimestamp) pass any time window (w2/m53 — same rule as services).
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedPGWithLabels(t, svc, "pg-a", appv1alpha1.DatabaseSpec{Plan: "free"}, nil)
	seedPGWithLabels(t, svc, "pg-b", appv1alpha1.DatabaseSpec{Plan: "free"}, nil)

	for _, param := range []string{"createdBefore", "createdAfter", "updatedBefore", "updatedAfter"} {
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/postgres?"+param+"=yesterday", nil))
		if rec.Code != http.StatusBadRequest {
			t.Errorf("%s=yesterday => 400, got %d", param, rec.Code)
		}
	}

	// Empty CreatedAt/UpdatedAt on freshly seeded CRs passes any time window.
	got := listPGNames(t, mux, "?createdBefore=2020-01-01T00:00:00Z")
	if len(got) != 2 {
		t.Errorf("empty createdAt passes createdBefore filter: got %d, want 2", len(got))
	}
}
