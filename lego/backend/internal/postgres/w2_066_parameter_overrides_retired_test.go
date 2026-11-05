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

// w2_066_parameter_overrides_retired_test.go — w2/066 (user decision D2=A).
// Render's 2026-11-02 OpenAPI dropped Postgres parameterOverrides (and the
// postgresParameterOverrides schema), so bex retired it on every surface:
// create and update refuse it by name with SQL guidance, the detail view no
// longer returns it, and the bex-only PUT route is gone. A database that
// already declares spec.parameters keeps them untouched (zero disruption), and
// the declared set stays readable on GET /parameters.

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func declaredParametersFixture(t *testing.T) (*Service, client.Client) {
	t.Helper()
	return newService(&appv1alpha1.Database{
		ObjectMeta: metav1.ObjectMeta{Name: "dpg-declared", Namespace: "default"},
		Spec: appv1alpha1.DatabaseSpec{
			Name: "declared-pg", Plan: "free",
			Parameters: map[string]string{"work_mem": "16MB", "max_connections": "200"},
		},
		Status: appv1alpha1.DatabaseStatus{Phase: appv1alpha1.DBPhaseReady},
	})
}

func assertRetiredRefusal(t *testing.T, what string, code int, body string) {
	t.Helper()
	if code != http.StatusBadRequest {
		t.Fatalf("%s => %d %s, want 400", what, code, body)
	}
	if !strings.Contains(body, "POSTGRES_PARAMETER_OVERRIDES_UNSUPPORTED") || !strings.Contains(body, "parameterOverrides") || !strings.Contains(body, "ALTER DATABASE") {
		t.Fatalf("%s refusal must name parameterOverrides and point at ALTER DATABASE: %s", what, body)
	}
}

func TestRESTCreatePostgresRefusesParameterOverrides(t *testing.T) {
	for _, body := range []string{
		`{"name":"po-pg","ownerId":"tea-1","plan":"free","parameterOverrides":{"work_mem":"64MB"}}`,
		`{"name":"po-pg","ownerId":"tea-1","plan":"free","parameterOverrides":{}}`,
		`{"name":"po-pg","ownerId":"tea-1","plan":"free","parameterOverrides":{"work_mem":"64MB"},"dryRun":true}`,
	} {
		svc, cl := newService()
		rec := serveREST(svc, http.MethodPost, "/v1/postgres", body)
		assertRetiredRefusal(t, "POST "+body, rec.Code, rec.Body.String())
		var dbs appv1alpha1.DatabaseList
		if err := cl.List(context.Background(), &dbs); err != nil {
			t.Fatal(err)
		}
		if len(dbs.Items) != 0 {
			t.Fatalf("refused create still wrote %d Database CRs", len(dbs.Items))
		}
	}

	// An explicit null is absence, not the field: the create goes through and
	// declares nothing.
	svc, cl := newService()
	code, pg := createPG(t, svc, `{"name":"po-pg","ownerId":"tea-1","plan":"free","parameterOverrides":null}`)
	if code != http.StatusCreated {
		t.Fatalf("create with parameterOverrides:null => %d", code)
	}
	if got := getDatabase(t, cl, pg.ID).Spec.Parameters; len(got) != 0 {
		t.Fatalf("create declared parameters %v, want none", got)
	}
}

func TestRESTPatchPostgresRefusesParameterOverrides(t *testing.T) {
	for _, body := range []string{
		`{"parameterOverrides":{"work_mem":"8MB"}}`,
		`{"parameterOverrides":{}}`,
		`{"parameterOverrides":{"work_mem":"8MB"},"dryRun":true}`,
		`{"name":"renamed","parameterOverrides":{"work_mem":"8MB"}}`,
	} {
		svc, cl := declaredParametersFixture(t)
		rec := serveREST(svc, http.MethodPatch, "/v1/postgres/dpg-declared", body)
		assertRetiredRefusal(t, "PATCH "+body, rec.Code, rec.Body.String())
		spec := getDatabase(t, cl, "dpg-declared").Spec
		if spec.Name != "declared-pg" || len(spec.Parameters) != 2 ||
			spec.Parameters["work_mem"] != "16MB" || spec.Parameters["max_connections"] != "200" {
			t.Fatalf("refused PATCH changed the database: name=%q params=%v", spec.Name, spec.Parameters)
		}
	}

	// A PATCH that does not mention the field leaves existing declarations be.
	svc, cl := declaredParametersFixture(t)
	if rec := serveREST(svc, http.MethodPatch, "/v1/postgres/dpg-declared", `{"name":"renamed"}`); rec.Code != http.StatusOK {
		t.Fatalf("rename => %d %s", rec.Code, rec.Body.String())
	}
	if got := getDatabase(t, cl, "dpg-declared").Spec.Parameters; got["work_mem"] != "16MB" || len(got) != 2 {
		t.Fatalf("rename disturbed the declared parameters: %v", got)
	}
}

func TestPostgresDetailOmitsParameterOverrides(t *testing.T) {
	svc, _ := declaredParametersFixture(t)
	rec := serveREST(svc, http.MethodGet, "/v1/postgres/dpg-declared", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET detail => %d %s", rec.Code, rec.Body.String())
	}
	var detail map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &detail); err != nil {
		t.Fatalf("decode detail: %v", err)
	}
	if _, ok := detail["parameterOverrides"]; ok {
		t.Fatalf("detail still returns parameterOverrides: %s", rec.Body.String())
	}
}

func TestParameterOverridesPutRouteIsGone(t *testing.T) {
	svc, cl := declaredParametersFixture(t)
	rec := serveREST(svc, http.MethodPut, "/v1/postgres/dpg-declared/parameter-overrides", `{"parameters":{"work_mem":"8MB"}}`)
	if rec.Code != http.StatusMethodNotAllowed && rec.Code != http.StatusNotFound {
		t.Fatalf("PUT parameter-overrides => %d %s, want the route gone (404/405)", rec.Code, rec.Body.String())
	}
	if got := getDatabase(t, cl, "dpg-declared").Spec.Parameters; got["work_mem"] != "16MB" {
		t.Fatalf("removed route still wrote: %v", got)
	}
}

// TestDeclaredParametersStayReadableOverREST: the declared set a database
// already carries remains readable (read-only) on GET /parameters.
func TestDeclaredParametersStayReadableOverREST(t *testing.T) {
	svc, _ := declaredParametersFixture(t)
	rec := serveREST(svc, http.MethodGet, "/v1/postgres/dpg-declared/parameters", "")
	if rec.Code != http.StatusOK {
		t.Fatalf("GET /parameters => %d %s", rec.Code, rec.Body.String())
	}
	var declared []ParameterSpecView
	if err := json.Unmarshal(rec.Body.Bytes(), &declared); err != nil {
		t.Fatalf("decode /parameters: %v", err)
	}
	want := []ParameterSpecView{
		{Name: "max_connections", Value: "200", ObservationStatus: "unavailable"},
		{Name: "work_mem", Value: "16MB", ObservationStatus: "unavailable"},
	}
	if len(declared) != len(want) {
		t.Fatalf("GET /parameters = %v, want %v", declared, want)
	}
	for i := range want {
		if declared[i] != want[i] {
			t.Errorf("parameter %d = %+v, want %+v", i, declared[i], want[i])
		}
	}

	// A database that declares nothing reads back [].
	svc, _ = newService()
	code, pg := createPG(t, svc, `{"name":"fresh-pg","ownerId":"tea-1","plan":"free"}`)
	if code != http.StatusCreated {
		t.Fatalf("create => %d", code)
	}
	rec = serveREST(svc, http.MethodGet, "/v1/postgres/"+pg.ID+"/parameters", "")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Fatalf("GET /parameters on a fresh database = %d %s, want []", rec.Code, rec.Body.String())
	}
}
