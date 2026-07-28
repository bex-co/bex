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
	manifest := "services:\n  - name: hello\n    repo: https://github.com/x/mono\n    rootDir: services/hello\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if got := getApp(t, cl, "hello").Spec.RootDir; got != "services/hello" {
		t.Errorf("bex.yml rootDir not threaded: spec.rootDir = %q", got)
	}
}

// --- SetRootDir (update-after-create, not covered by create-time threading above) ---

func TestSetRootDirSetsAndBumpsRestartedAt(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))

	v, err := svc.SetRootDir(context.Background(), "web", "api")
	if err != nil {
		t.Fatalf("SetRootDir: %v", err)
	}
	if v.RootDir != "api" {
		t.Errorf("view RootDir = %q, want %q", v.RootDir, "api")
	}
	a := getApp(t, cl, "web")
	if a.Spec.RootDir != "api" {
		t.Errorf("spec.rootDir = %q, want %q", a.Spec.RootDir, "api")
	}
	if a.Spec.RestartedAt == "" {
		t.Error("SetRootDir must bump restartedAt so the new subdirectory actually gets built")
	}

	// Empty restores the repo root.
	if _, err := svc.SetRootDir(context.Background(), "web", ""); err != nil {
		t.Fatalf("SetRootDir(\"\"): %v", err)
	}
	if got := getApp(t, cl, "web").Spec.RootDir; got != "" {
		t.Errorf("spec.rootDir = %q, want empty (repo root)", got)
	}
}

func TestSetRootDirRejectsImageBackedApp(t *testing.T) {
	svc, cl := newService(nil, sampleApp("img")) // sampleApp is image-backed, no repo

	if _, err := svc.SetRootDir(context.Background(), "img", "api"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("SetRootDir on an image-backed App should be core.ErrBadRequest, got %v", err)
	}
	if got := getApp(t, cl, "img").Spec.RootDir; got != "" {
		t.Errorf("a rejected SetRootDir must not touch spec.rootDir, got %q", got)
	}
}

// TestSetRootDirRejectsTraversal is the w6/m6 t003 regression: an untrusted
// rootDirectory must never escape the cloned repo. A traversal or absolute path
// is rejected at the API boundary rather than forwarded to BuildKit.
func TestSetRootDirRejectsTraversal(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))

	for _, bad := range []string{"../escape", "apps/../escape", "/etc/passwd", "apps\n/web"} {
		if _, err := svc.SetRootDir(context.Background(), "web", bad); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("SetRootDir(%q) should be core.ErrBadRequest, got %v", bad, err)
		}
	}
	if got := getApp(t, cl, "web").Spec.RootDir; got != "" {
		t.Errorf("rejected SetRootDir calls must not mutate spec.rootDir, got %q", got)
	}
}

func TestRESTPatchServiceRootDir(t *testing.T) {
	svc, _ := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"rootDir":"api"}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH rootDir => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		RootDir string `json:"rootDir"`
		Repo    string `json:"repo"`
		Branch  string `json:"branch"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.RootDir != "api" {
		t.Errorf("response = %s, want rootDir api", rec.Body)
	}
	if out.Repo != "https://github.com/x/mono" || out.Branch != "main" {
		t.Errorf("repo/branch not surfaced on read: %+v", out)
	}
}

func TestGraphQLSetRootDir(t *testing.T) {
	svc, _ := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setRootDir(id: "web", rootDir: "api") { rootDir repo branch } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	got := res.Data.(map[string]any)["setRootDir"].(map[string]any)
	if got["rootDir"] != "api" {
		t.Errorf("rootDir = %v, want api", got["rootDir"])
	}
	if got["repo"] != "https://github.com/x/mono" || got["branch"] != "main" {
		t.Errorf("repo/branch not surfaced: %+v", got)
	}
}

func TestMCPSetRootDirectory(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))

	v, err := svc.SetRootDir(context.Background(), "web", "api")
	if err != nil {
		t.Fatalf("SetRootDir: %v", err)
	}
	out := toRenderService(v)
	if out.RootDir != "api" {
		t.Errorf("set_root_directory (via renderService projection) rootDir = %q, want api", out.RootDir)
	}
	if got := getApp(t, cl, "web").Spec.RootDir; got != "api" {
		t.Errorf("spec.rootDir = %q, want api", got)
	}
}

// TestCreateWithRootDirConflictsOnExistingApp is the w4/m19 update: a repeat
// Create — even one resending the same rootDir — is a conflict, not a
// redeploy; applyCreateToSpec's wholesale (not merge) re-apply of rootDir
// still matters for the paths that legitimately redeploy in place (Deploy,
// the stack path's applyCreate — see TestDeployStackChangedServiceRedeploys),
// just not for Create anymore.
func TestCreateWithRootDirConflictsOnExistingApp(t *testing.T) {
	existing := sampleApp("web")
	existing.Spec.Repo = "https://github.com/x/mono"
	existing.Spec.RootDir = "services/api"
	existing.Spec.Image = ""
	svc, cl := newService(nil, existing)

	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "web", Repo: "https://github.com/x/mono", RootDir: "services/other",
	})
	if !errors.Is(err, core.ErrConflict) {
		t.Fatalf("Create on an existing name: got %v, want ErrConflict", err)
	}
	if got := getApp(t, cl, "web").Spec.RootDir; got != "services/api" {
		t.Errorf("a rejected create must not touch the existing App's rootDir: got %q", got)
	}
}
