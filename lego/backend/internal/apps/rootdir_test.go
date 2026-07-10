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
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
)

// rootdir_test.go covers App.spec.rootDir (monorepo Root Directory support,
// w1/m18) settable+readable identically across REST, GraphQL, and MCP — the
// same field-threading test depth TestGraphQLCreateServiceTypeAndCronFields and
// TestMCPCreateWebServiceThreadsType apply to spec.type.

func TestRESTCreateThreadsRootDirAndReadsItBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","repo":"https://github.com/x/mono","rootDir":"services/api"}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec.RootDir; got != "services/api" {
		t.Fatalf("spec.rootDir = %q, want services/api", got)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		RootDir string `json:"rootDir"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.RootDir != "services/api" {
		t.Errorf("GET response = %s, want rootDir services/api", rec.Body)
	}
}

func TestGraphQLCreateAndReadRootDir(t *testing.T) {
	svc, cl := newService(nil)
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "worker", repo: "https://github.com/x/mono", rootDir: "services/worker") { rootDir } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	if got := getApp(t, cl, "worker").Spec.RootDir; got != "services/worker" {
		t.Errorf("createService did not thread rootDir onto the spec: got %q", got)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "worker") { rootDir } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	srv := res.Data.(map[string]any)["server"].(map[string]any)
	if srv["rootDir"] != "services/worker" {
		t.Errorf("server.rootDir = %+v, want services/worker", srv["rootDir"])
	}
}

func TestMCPCreateWebServiceThreadsRootDir(t *testing.T) {
	req := createWebServiceArgs{Name: "w", Repo: "https://github.com/x/mono", RootDir: "services/web"}.toCreateRequest()
	if req.RootDir != "services/web" {
		t.Errorf("create_web_service rootDir not threaded: %+v", req)
	}
}

func TestMCPCreateCronJobThreadsRootDir(t *testing.T) {
	req := createCronJobArgs{Name: "nightly", Schedule: "0 0 * * *", Repo: "https://github.com/x/mono", RootDir: "jobs/nightly"}.toCreateRequest()
	if req.RootDir != "jobs/nightly" {
		t.Errorf("create_cron_job rootDir not threaded: %+v", req)
	}
}

func TestDeployManifestThreadsRootDir(t *testing.T) {
	svc, cl := newService(nil)
	manifest := "apps:\n  - name: hello\n    repo: https://github.com/x/mono\n    rootDir: services/hello\n"
	if _, err := svc.Deploy(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("Deploy: %v", err)
	}
	if got := getApp(t, cl, "hello").Spec.RootDir; got != "services/hello" {
		t.Errorf("bex.yml rootDir not threaded: spec.rootDir = %q", got)
	}
}

func TestRedeployReappliesRootDir(t *testing.T) {
	// applyCreateToSpec re-applies every create-owned field wholesale on a
	// repeat Create (redeploy) — the same rule Repo/Branch/Builder already
	// follow, not a merge. Resending rootDir on redeploy keeps it in place.
	existing := sampleApp("web")
	existing.Spec.Repo = "https://github.com/x/mono"
	existing.Spec.RootDir = "services/api"
	existing.Spec.Image = ""
	svc, cl := newService(nil, existing)

	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Repo: "https://github.com/x/mono", RootDir: "services/api",
	}); err != nil {
		t.Fatalf("Create (update): %v", err)
	}
	if got := getApp(t, cl, "web").Spec.RootDir; got != "services/api" {
		t.Errorf("redeploy must re-apply the resent rootDir: got %q", got)
	}
}
