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
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// buildfilter_test.go covers App.spec.buildFilter (Render's Build Filters, the
// glob-based git-push auto-deploy gate, w1/m34) settable + readable identically
// across REST, GraphQL, and MCP — the same adapter-parity depth rootdir_test.go
// holds spec.rootDir to.

// getBuildFilter reads back the two glob lists off an App's spec (nil-safe).
func getBuildFilter(t *testing.T, cl client.Client, name string) ([]string, []string) {
	t.Helper()
	a := getApp(t, cl, name)
	if a.Spec.BuildFilter == nil {
		return nil, nil
	}
	return a.Spec.BuildFilter.Paths, a.Spec.BuildFilter.IgnoredPaths
}

// mustSchema builds the composed query+mutation schema over a Service, failing
// the test on a schema error (shared by the GraphQL adapter cases below).
func mustSchema(t *testing.T, svc *Service) graphql.Schema {
	t.Helper()
	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
		Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
	})
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	return schema
}

func TestRESTCreateThreadsBuildFilterAndReadsItBack(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","repo":"https://github.com/x/mono","buildFilter":{"paths":["src/**"],"ignoredPaths":["docs/**"]}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	paths, ignored := getBuildFilter(t, cl, "web")
	if len(paths) != 1 || paths[0] != "src/**" || len(ignored) != 1 || ignored[0] != "docs/**" {
		t.Fatalf("spec.buildFilter = %v / %v, want [src/**] / [docs/**]", paths, ignored)
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("get => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		BuildFilter struct {
			Paths        []string `json:"paths"`
			IgnoredPaths []string `json:"ignoredPaths"`
		} `json:"buildFilter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode GET: %v (%s)", err, rec.Body)
	}
	if len(out.BuildFilter.Paths) != 1 || out.BuildFilter.Paths[0] != "src/**" ||
		len(out.BuildFilter.IgnoredPaths) != 1 || out.BuildFilter.IgnoredPaths[0] != "docs/**" {
		t.Errorf("GET buildFilter = %+v, want paths=[src/**] ignoredPaths=[docs/**]", out.BuildFilter)
	}
}

// TestRESTBuildFilterMarshalsArraysNotNull asserts the Render-required shape: a
// present buildFilter always carries both arrays (never JSON null), even when one
// side is empty.
func TestRESTBuildFilterMarshalsArraysNotNull(t *testing.T) {
	svc, _ := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := `{"name":"web","repo":"https://github.com/x/mono","buildFilter":{"paths":["src/**"]}}`
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(body)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => 201, got %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"ignoredPaths":[]`) {
		t.Errorf("a present buildFilter must marshal ignoredPaths as [] not null: %s", rec.Body)
	}
}

func TestGraphQLCreateAndReadBuildFilter(t *testing.T) {
	svc, cl := newService(nil)
	schema := mustSchema(t, svc)

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { createService(name: "worker", repo: "https://github.com/x/mono", buildFilter: {paths: ["src/**"], ignoredPaths: ["docs/**"]}) { buildFilter { paths ignoredPaths } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("createService: %v", res.Errors)
	}
	paths, ignored := getBuildFilter(t, cl, "worker")
	if len(paths) != 1 || paths[0] != "src/**" || len(ignored) != 1 || ignored[0] != "docs/**" {
		t.Errorf("createService did not thread buildFilter: %v / %v", paths, ignored)
	}

	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ server(id: "worker") { buildFilter { paths ignoredPaths } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("server query: %v", res.Errors)
	}
	srv := res.Data.(map[string]any)["server"].(map[string]any)
	bf, ok := srv["buildFilter"].(map[string]any)
	if !ok {
		t.Fatalf("server.buildFilter = %+v, want an object", srv["buildFilter"])
	}
	if got := bf["paths"].([]any); len(got) != 1 || got[0] != "src/**" {
		t.Errorf("server.buildFilter.paths = %+v, want [src/**]", bf["paths"])
	}
}

func TestGraphQLSetBuildFilter(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))
	schema := mustSchema(t, svc)

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setBuildFilter(id: "web", buildFilter: {paths: ["api/**"], ignoredPaths: []}) { buildFilter { paths ignoredPaths } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("setBuildFilter: %v", res.Errors)
	}
	if paths, _ := getBuildFilter(t, cl, "web"); len(paths) != 1 || paths[0] != "api/**" {
		t.Errorf("setBuildFilter did not patch spec.buildFilter: %v", paths)
	}

	// Clearing: empty arrays canonicalize to a nil filter (no empty-but-present object).
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `mutation { setBuildFilter(id: "web", buildFilter: {paths: [], ignoredPaths: []}) { buildFilter { paths } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("setBuildFilter(clear): %v", res.Errors)
	}
	if getApp(t, cl, "web").Spec.BuildFilter != nil {
		t.Error("clearing the filter (empty arrays) must leave spec.buildFilter nil")
	}
}

func TestMCPCreateWebServiceThreadsBuildFilter(t *testing.T) {
	req := createWebServiceArgs{
		Name: "w", Repo: "https://github.com/x/mono",
		BuildFilter: &buildFilterArg{Paths: []string{"src/**"}, IgnoredPaths: []string{"docs/**"}},
	}.toCreateRequest()
	if req.BuildFilter == nil || len(req.BuildFilter.Paths) != 1 || req.BuildFilter.Paths[0] != "src/**" {
		t.Errorf("create_web_service buildFilter not threaded: %+v", req.BuildFilter)
	}
	// Omitted arg => nil filter, not a create-time error.
	if got := (createWebServiceArgs{Name: "w2", Repo: "https://github.com/x/mono"}.toCreateRequest()); got.BuildFilter != nil {
		t.Errorf("omitted buildFilter should thread nil, got %+v", got.BuildFilter)
	}
}

func TestMCPSetBuildFilter(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))

	v, err := svc.SetBuildFilter(context.Background(), "web", &BuildFilterView{Paths: []string{"api/**"}})
	if err != nil {
		t.Fatalf("SetBuildFilter: %v", err)
	}
	// MCP projects through the same renderService as REST — parity by construction.
	out := toRenderService(v)
	if out.BuildFilter == nil || len(out.BuildFilter.Paths) != 1 || out.BuildFilter.Paths[0] != "api/**" {
		t.Errorf("set_build_filter (via renderService) = %+v, want paths=[api/**]", out.BuildFilter)
	}
	if paths, _ := getBuildFilter(t, cl, "web"); len(paths) != 1 || paths[0] != "api/**" {
		t.Errorf("spec.buildFilter = %v, want [api/**]", paths)
	}
}

func TestDeployManifestThreadsBuildFilter(t *testing.T) {
	svc, cl := newService(nil)
	manifest := "services:\n  - name: hello\n    repo: https://github.com/x/mono\n    buildFilter:\n      paths:\n        - src/**\n      ignoredPaths:\n        - docs/**\n"
	if _, err := svc.DeployStack(context.Background(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	paths, ignored := getBuildFilter(t, cl, "hello")
	if len(paths) != 1 || paths[0] != "src/**" || len(ignored) != 1 || ignored[0] != "docs/**" {
		t.Errorf("bex.yml buildFilter not threaded: %v / %v", paths, ignored)
	}
}

// --- SetBuildFilter validation + guards ---

func TestSetBuildFilterRejectsImageBackedApp(t *testing.T) {
	svc, cl := newService(nil, sampleApp("img")) // image-backed, no repo

	_, err := svc.SetBuildFilter(context.Background(), "img", &BuildFilterView{Paths: []string{"src/**"}})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("SetBuildFilter on an image-backed App should be core.ErrBadRequest, got %v", err)
	}
	if getApp(t, cl, "img").Spec.BuildFilter != nil {
		t.Error("a rejected SetBuildFilter must not touch spec.buildFilter")
	}
}

func TestSetBuildFilterRejectsBadGlobs(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))

	// An unclosed character class is not a compilable doublestar pattern; a
	// traversal or absolute pattern escapes the repo. All are rejected at the
	// boundary so the webhook matcher never sees them.
	for _, bad := range [][]string{{"src/["}, {"../escape/**"}, {"/etc/passwd"}} {
		if _, err := svc.SetBuildFilter(context.Background(), "web", &BuildFilterView{Paths: bad}); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("SetBuildFilter(%v) should be core.ErrBadRequest, got %v", bad, err)
		}
	}
	if getApp(t, cl, "web").Spec.BuildFilter != nil {
		t.Error("rejected SetBuildFilter calls must not mutate spec.buildFilter")
	}
}

func TestSetBuildFilterDropsBlankEntries(t *testing.T) {
	svc, cl := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))

	// Blank/whitespace entries are dropped; a filter that is all-blanks clears.
	if _, err := svc.SetBuildFilter(context.Background(), "web", &BuildFilterView{Paths: []string{"src/**", "  ", ""}}); err != nil {
		t.Fatalf("SetBuildFilter: %v", err)
	}
	if paths, _ := getBuildFilter(t, cl, "web"); len(paths) != 1 || paths[0] != "src/**" {
		t.Errorf("blank entries not dropped: %v", paths)
	}
	if _, err := svc.SetBuildFilter(context.Background(), "web", &BuildFilterView{Paths: []string{"  "}}); err != nil {
		t.Fatalf("SetBuildFilter(blanks): %v", err)
	}
	if getApp(t, cl, "web").Spec.BuildFilter != nil {
		t.Error("an all-blank filter must clear spec.buildFilter to nil")
	}
}

func TestRESTPatchServiceBuildFilter(t *testing.T) {
	svc, _ := newService(nil, repoApp("web", "https://github.com/x/mono", "main"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	body := strings.NewReader(`{"buildFilter":{"paths":["api/**"],"ignoredPaths":[]}}`)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("PATCH", "/v1/services/web", body))
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH buildFilter => 200, got %d: %s", rec.Code, rec.Body)
	}
	var out struct {
		BuildFilter struct {
			Paths []string `json:"paths"`
		} `json:"buildFilter"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode PATCH: %v (%s)", err, rec.Body)
	}
	if len(out.BuildFilter.Paths) != 1 || out.BuildFilter.Paths[0] != "api/**" {
		t.Errorf("PATCH response buildFilter = %+v, want paths=[api/**]", out.BuildFilter)
	}
}
