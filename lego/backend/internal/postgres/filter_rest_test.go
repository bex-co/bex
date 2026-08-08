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
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
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

// seedPGUpdatedAt seeds a Database whose UpdatedAt resolves to the given
// timestamp, so a time-window filter has something real to place.
func seedPGUpdatedAt(t *testing.T, svc *Service, name, updatedAt string) {
	t.Helper()
	db := &appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{
			Name:        name,
			Namespace:   "default",
			Annotations: map[string]string{resourcemeta.UpdatedAtAnnotation: updatedAt},
		},
		Spec: appv1alpha1.DatabaseSpec{Name: name, Plan: "free"},
	}
	if err := svc.Client.Create(t.Context(), db); err != nil {
		t.Fatalf("seed db %s: %v", name, err)
	}
}

func TestRESTListPostgresTimeWindowExcludes(t *testing.T) {
	// The 400 and legacy-passthrough cases above never prove the window
	// narrows anything. Stamped databases must actually fall in or out of it.
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	seedPGUpdatedAt(t, svc, "pg-old", "2026-01-01T00:00:00Z")
	seedPGUpdatedAt(t, svc, "pg-new", "2026-12-01T00:00:00Z")
	seedPGWithLabels(t, svc, "pg-unstamped", appv1alpha1.DatabaseSpec{Plan: "free"}, nil)

	cases := []struct {
		query string
		want  []string
	}{
		// pg-unstamped has no timestamp, so it rides along with every window.
		{"?updatedBefore=2026-06-01T00:00:00Z", []string{"pg-old", "pg-unstamped"}},
		{"?updatedAfter=2026-06-01T00:00:00Z", []string{"pg-new", "pg-unstamped"}},
		{"?updatedAfter=2026-06-01T00:00:00Z&updatedBefore=2027-01-01T00:00:00Z", []string{"pg-new", "pg-unstamped"}},
		{"?updatedAfter=2027-01-01T00:00:00Z", []string{"pg-unstamped"}},
	}
	for _, c := range cases {
		got := listPGNames(t, mux, c.query)
		slices.Sort(got)
		want := slices.Clone(c.want)
		slices.Sort(want)
		if !slices.Equal(got, want) {
			t.Errorf("GET /v1/postgres%s = %v, want %v", c.query, got, want)
		}
	}
}

func TestRESTListPostgresNameFilterAcceptsCommaAndRepeatedForms(t *testing.T) {
	// Render's form-style array encoding: the official CLI sends a multi-value
	// flag as one comma-separated value, hand-written clients repeat the key.
	svc, _ := newService()
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	for _, name := range []string{"pg-alpha", "pg-bravo", "pg-charlie"} {
		seedPGWithLabels(t, svc, name, appv1alpha1.DatabaseSpec{Plan: "free"}, nil)
	}

	for _, query := range []string{
		"?name=pg-alpha,pg-bravo",
		"?name=pg-alpha&name=pg-bravo",
		"?name=%20pg-alpha%20,pg-bravo",
	} {
		got := listPGNames(t, mux, query)
		slices.Sort(got)
		if !slices.Equal(got, []string{"pg-alpha", "pg-bravo"}) {
			t.Errorf("GET /v1/postgres%s = %v, want [pg-alpha pg-bravo]", query, got)
		}
	}
}
