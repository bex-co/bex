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

// predeploy_test.go covers App.spec.preDeployCommand (Render's Pre-Deploy
// Command, w1/m33) threaded + readable identically across bex.yml, REST,
// GraphQL, and MCP, plus the SetPreDeployCommand update verb — the same
// field-threading depth rootdir_test.go applies to spec.rootDir.

func TestRESTCreateThreadsPreDeployCommandAndReadsItBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","repo":"https://github.com/x/mono","preDeployCommand":"npm run migrate"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Fatalf("spec.preDeployCommand = %q, want npm run migrate", got)
	}

	// Read-back: Render nests it under serviceDetails.preDeployCommand.
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	var out struct {
		ServiceDetails struct {
			PreDeployCommand string `json:"preDeployCommand"`
		} `json:"serviceDetails"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.ServiceDetails.PreDeployCommand != "npm run migrate" {
		t.Errorf("GET response = %s, want serviceDetails.preDeployCommand npm run migrate", rec.Body)
	}
}

func TestRESTCreateAcceptsPreDeployUnderServiceDetails(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	// Render-faithful nesting; top-level absent, so the nested value wins.
	body := `{"name":"web","repo":"https://github.com/x/mono","serviceDetails":{"preDeployCommand":"rails db:migrate"}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "rails db:migrate" {
		t.Errorf("serviceDetails.preDeployCommand not threaded: spec.preDeployCommand = %q", got)
	}
}

func TestGraphQLCreateAndReadPreDeployCommand(t *testing.T) {
	svc, cl := newService(nil)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "web", repo: "https://github.com/x/mono", preDeployCommand: "npm run migrate") { preDeployCommand } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Errorf("createService did not thread preDeployCommand onto the spec: got %q", got)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "web") { preDeployCommand } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	srv := res.Data.(map[string]any)["server"].(map[string]any)
	if srv["preDeployCommand"] != "npm run migrate" {
		t.Errorf("server.preDeployCommand = %+v, want npm run migrate", srv["preDeployCommand"])
	}
}

func TestMCPCreateWebServiceThreadsPreDeployCommand(t *testing.T) {
	req := createWebServiceArgs{Name: "w", Repo: "https://github.com/x/mono", PreDeployCommand: "npm run migrate"}.toCreateRequest()
	if req.PreDeployCommand != "npm run migrate" {
		t.Errorf("create_web_service preDeployCommand not threaded: %+v", req)
	}
}

func TestDeployManifestThreadsPreDeployCommand(t *testing.T) {
	svc, cl := newService(nil)
	manifest := "services:\n  - name: hello\n    type: web\n    runtime: docker\n    repo: https://github.com/x/mono\n    preDeployCommand: npm run migrate\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if got := getApp(t, cl, "hello").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Errorf("bex.yml preDeployCommand not threaded: spec.preDeployCommand = %q", got)
	}
}

// --- SetPreDeployCommand (update-after-create) ---

func TestSetPreDeployCommandSetsAndClears(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))

	v, err := svc.SetPreDeployCommand(context.Background(), "web", "  npm run migrate  ")
	if err != nil {
		t.Fatalf("SetPreDeployCommand: %v", err)
	}
	// Trimmed before it reaches the spec.
	if v.PreDeployCommand != "npm run migrate" {
		t.Errorf("view PreDeployCommand = %q, want npm run migrate", v.PreDeployCommand)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Errorf("spec.preDeployCommand = %q, want npm run migrate", got)
	}

	// Empty clears the step.
	if _, err := svc.SetPreDeployCommand(context.Background(), "web", ""); err != nil {
		t.Fatalf("SetPreDeployCommand(\"\"): %v", err)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "" {
		t.Errorf("spec.preDeployCommand = %q, want empty (cleared)", got)
	}
}

func TestSetPreDeployCommandRejectsCronAndStatic(t *testing.T) {
	svc, cl := newService(nil, cronApp("nightly"))
	if _, err := svc.SetPreDeployCommand(context.Background(), "nightly", "npm run migrate"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("SetPreDeployCommand on a cron_job should be core.ErrBadRequest, got %v", err)
	}
	if got := getApp(t, cl, "nightly").Spec.PreDeployCommand; got != "" {
		t.Errorf("a rejected SetPreDeployCommand must not touch the spec, got %q", got)
	}
}

func TestRESTPatchServicePreDeployCommand(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", strings.NewReader(`{"preDeployCommand":"npm run migrate"}`)))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH preDeployCommand => 200, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.PreDeployCommand; got != "npm run migrate" {
		t.Errorf("PATCH did not set spec.preDeployCommand, got %q", got)
	}
}
