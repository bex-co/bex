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

package apps

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// healthcheckpath_test.go covers spec.healthCheckPath (Render's health check
// path → ReadinessProbe gating) settable+readable identically across REST,
// GraphQL, and MCP — mirrors rootdir_test.go depth.

func TestRESTCreateThreadsHealthCheckPathAndReadsItBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","image":{"imagePath":"nginx:v1"},"serviceDetails":{"healthCheckPath":"/healthz"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.HealthCheckPath; got != "/healthz" {
		t.Fatalf("spec.healthCheckPath = %q, want /healthz", got)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		HealthCheckPath string `json:"healthCheckPath"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.HealthCheckPath != "/healthz" {
		t.Errorf("GET healthCheckPath = %q, want /healthz", out.HealthCheckPath)
	}
}

// --- REST PATCH ---

func TestRESTPatchServiceHealthCheckPath(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// healthCheckPath is a top-level PATCH field (not nested in serviceDetails).
	body := strings.NewReader(`{"healthCheckPath":"/ready"}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH healthCheckPath => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.HealthCheckPath; got != "/ready" {
		t.Errorf("spec.healthCheckPath = %q, want /ready", got)
	}
	var out struct {
		HealthCheckPath string `json:"healthCheckPath"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if out.HealthCheckPath != "/ready" {
		t.Errorf("response healthCheckPath = %q, want /ready", out.HealthCheckPath)
	}
}

func TestRESTPatchServiceHealthCheckPathEmpty(t *testing.T) {
	app := sampleApp("web")
	app.Spec.HealthCheckPath = "/healthz"
	svc, cl := newService(nil, app)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Explicit "" normalizes to "/" (the platform default stored in spec).
	body := strings.NewReader(`{"healthCheckPath":""}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH healthCheckPath='' => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.HealthCheckPath; got != "/" {
		t.Errorf("spec.healthCheckPath = %q, want / (platform default)", got)
	}
}

// --- SetHealthCheckPath service method ---

func TestSetHealthCheckPathSetsSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	v, err := svc.SetHealthCheckPath(context.Background(), "web", "/ready")
	if err != nil {
		t.Fatalf("SetHealthCheckPath: %v", err)
	}
	if v.HealthCheckPath != "/ready" {
		t.Errorf("view HealthCheckPath = %q, want /ready", v.HealthCheckPath)
	}
	if got := getApp(t, cl, "web").Spec.HealthCheckPath; got != "/ready" {
		t.Errorf("spec.healthCheckPath = %q, want /ready", got)
	}
}

func TestSetHealthCheckPathEmptyNormalizesToDefault(t *testing.T) {
	app := sampleApp("web")
	app.Spec.HealthCheckPath = "/healthz"
	svc, cl := newService(nil, app)

	// Empty normalizes to "/" and stores that in the spec.
	if _, err := svc.SetHealthCheckPath(context.Background(), "web", ""); err != nil {
		t.Fatalf("SetHealthCheckPath(''): %v", err)
	}
	if got := getApp(t, cl, "web").Spec.HealthCheckPath; got != "/" {
		t.Errorf("spec.healthCheckPath = %q, want / (platform default)", got)
	}
}

func TestSetHealthCheckPathRejectsNonSlashPrefix(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))

	if _, err := svc.SetHealthCheckPath(context.Background(), "web", "healthz"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("SetHealthCheckPath('healthz') should be ErrBadRequest, got %v", err)
	}
}

// --- GraphQL ---

func TestGraphQLCreateServiceThreadsHealthCheckPath(t *testing.T) {
	svc, cl := newService(nil)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "web", image: "nginx:v1", healthCheckPath: "/healthz") { healthCheckPath } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	if got := getApp(t, cl, "web").Spec.HealthCheckPath; got != "/healthz" {
		t.Errorf("createService did not thread healthCheckPath onto the spec: got %q", got)
	}
	srv := res.Data.(map[string]any)["createService"].(map[string]any)
	if srv["healthCheckPath"] != "/healthz" {
		t.Errorf("createService response healthCheckPath = %v, want /healthz", srv["healthCheckPath"])
	}
}

func TestGraphQLSetHealthCheckPath(t *testing.T) {
	svc, _ := newService(nil, sampleApp("web"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setHealthCheckPath(id: "web", path: "/ready") { healthCheckPath } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["setHealthCheckPath"].(map[string]any)
	if got["healthCheckPath"] != "/ready" {
		t.Errorf("healthCheckPath = %v, want /ready", got["healthCheckPath"])
	}
}

func TestGraphQLServerQueryReturnsHealthCheckPath(t *testing.T) {
	app := sampleApp("web")
	app.Spec.HealthCheckPath = "/healthz"
	svc, _ := newService(nil, app)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "web") { healthCheckPath } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	srv := res.Data.(map[string]any)["server"].(map[string]any)
	if srv["healthCheckPath"] != "/healthz" {
		t.Errorf("server.healthCheckPath = %v, want /healthz", srv["healthCheckPath"])
	}
}

// --- MCP ---

func TestMCPCreateWebServiceThreadsHealthCheckPath(t *testing.T) {
	req := createWebServiceArgs{Name: "w", Image: "nginx:v1", HealthCheckPath: "/healthz"}.toCreateRequest()
	if req.HealthCheckPath != "/healthz" {
		t.Errorf("create_web_service healthCheckPath not threaded: %+v", req)
	}
}

func TestMCPSetHealthCheckPathUpdatesSpec(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web"))

	v, err := svc.SetHealthCheckPath(context.Background(), "web", "/ready")
	if err != nil {
		t.Fatalf("SetHealthCheckPath: %v", err)
	}
	out := toRenderService(v)
	if out.HealthCheckPath != "/ready" {
		t.Errorf("set_health_check_path healthCheckPath = %q, want /ready", out.HealthCheckPath)
	}
	if got := getApp(t, cl, "web").Spec.HealthCheckPath; got != "/ready" {
		t.Errorf("spec.healthCheckPath = %q, want /ready", got)
	}
}
