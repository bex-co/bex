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
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestRenderNativeBuildContractAcrossCreateSurfaces(t *testing.T) {
	t.Run("rest", func(t *testing.T) {
		svc, cl := newService(nil)
		mux := http.NewServeMux()
		svc.RegisterREST(mux)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(`{
			"type":"web_service",
			"name":"web",
			"repo":"https://github.com/x/web",
			"serviceDetails":{
				"runtime":"node",
				"envSpecificDetails":{"buildCommand":"npm ci","startCommand":"npm start"}
			}
		}`)))
		if rec.Code != http.StatusCreated {
			t.Fatalf("create => %d: %s", rec.Code, rec.Body)
		}
		app := getApp(t, cl, "web")
		if app.Spec.Builder != "native" || app.Spec.Runtime != "node" || app.Spec.BuildCommand != "npm ci" || app.Spec.StartCommand != "npm start" {
			t.Fatalf("spec = %+v", app.Spec)
		}
		var out struct {
			Service struct {
				Builder        string         `json:"builder"`
				ServiceDetails map[string]any `json:"serviceDetails"`
			} `json:"service"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil || out.Service.Builder != "" || out.Service.ServiceDetails["runtime"] != "node" {
			t.Fatalf("response = %s", rec.Body)
		}
	})

	t.Run("graphql", func(t *testing.T) {
		svc, cl := newService(nil)
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { createService(name:"web", repo:"https://github.com/x/web", runtime:"go", buildCommand:"go build -o app .", startCommand:"./app") { runtime buildCommand startCommand builder } }`})
		if len(res.Errors) > 0 {
			t.Fatal(res.Errors)
		}
		app := getApp(t, cl, "web")
		if app.Spec.Builder != "native" || app.Spec.Runtime != "go" {
			t.Fatalf("spec = %+v", app.Spec)
		}
	})

	t.Run("mcp", func(t *testing.T) {
		req := createWebServiceArgs{
			Name: "web", Repo: "https://github.com/x/web", Runtime: "python",
			BuildCommand: "pip install -r requirements.txt", StartCommand: "gunicorn app:app",
		}.toCreateRequest()
		if req.Runtime != "python" || req.BuildCommand == "" || req.StartCommand == "" {
			t.Fatalf("request = %+v", req)
		}
	})
}

func TestExplicitBuildpackRemainsBexExtension(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{Name: "web", Repo: "https://github.com/x/web", Builder: "buildpack"}); err != nil {
		t.Fatal(err)
	}
	if got := getApp(t, cl, "web").Spec.Builder; got != "buildpack" {
		t.Fatalf("spec.builder = %q", got)
	}
}

func TestRenderDockerDetailsMapToDockerfileBuild(t *testing.T) {
	svc, cl := newService(nil)
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/services", strings.NewReader(`{
		"type":"web_service",
		"name":"web",
		"repo":"https://github.com/x/web",
		"serviceDetails":{
			"runtime":"docker",
			"envSpecificDetails":{
				"dockerCommand":"bin/server",
				"dockerContext":"services/web",
				"dockerfilePath":"docker/Dockerfile.prod"
			}
		}
	}`)))
	if rec.Code != http.StatusCreated {
		t.Fatalf("create => %d: %s", rec.Code, rec.Body)
	}
	app := getApp(t, cl, "web")
	// dockerContext is its own spec field since w8/m19 — the pre-m19 fold into
	// RootDir was a lossy approximation of Render's semantics.
	if app.Spec.Builder != "dockerfile" || app.Spec.Runtime != "docker" ||
		app.Spec.RootDir != "" || app.Spec.DockerContext != "services/web" ||
		app.Spec.DockerfilePath != "docker/Dockerfile.prod" ||
		app.Spec.StartCommand != "bin/server" {
		t.Fatalf("spec = %+v", app.Spec)
	}
}

// TestEffectiveRuntimeReadBack pins w4/052: bex's App CR leaves spec.runtime
// empty for a Dockerfile build it expresses through the (default/auto/dockerfile)
// builder — the shape a dashboard, Blueprint, or hand-applied App produces. The
// official CLI reads serviceDetails.runtime to round-trip a partial `services
// update`, so an empty value stranded such a service: `--health-check-path`
// alone failed client-side ("unsupported runtime \"\""), and adding
// --runtime docker was rejected as a forbidden switch ("cannot switch runtimes
// via the CLI"). The read surfaces now derive "docker" for those builds while
// leaving genuinely runtime-less shapes (prebuilt image, buildpack, static site)
// empty as before. env mirrors runtime — Render's deprecated response alias.
func TestEffectiveRuntimeReadBack(t *testing.T) {
	staticSite := func() *appv1alpha1.App {
		a := repoApp("web", "https://github.com/x/web", "main")
		a.Spec.Type = appv1alpha1.TypeStaticSite
		a.Spec.PublishPath = "dist"
		return a
	}
	buildpack := func() *appv1alpha1.App {
		a := repoApp("web", "https://github.com/x/web", "main")
		a.Spec.Builder = "buildpack"
		return a
	}
	nativeGo := func() *appv1alpha1.App {
		a := repoApp("web", "https://github.com/x/web", "main")
		a.Spec.Runtime = "go"
		a.Spec.Builder = "native"
		return a
	}
	autoBuilder := func() *appv1alpha1.App {
		a := repoApp("web", "https://github.com/x/web", "main")
		a.Spec.Builder = "auto"
		return a
	}
	dockerfileBuilder := func() *appv1alpha1.App {
		a := repoApp("web", "https://github.com/x/web", "main")
		a.Spec.Builder = "dockerfile"
		return a
	}
	cases := []struct {
		name string
		app  *appv1alpha1.App
		want string // "" => runtime/env omitted entirely
	}{
		{"repo default builder derives docker", repoApp("web", "https://github.com/x/web", "main"), "docker"},
		{"repo auto builder derives docker", autoBuilder(), "docker"},
		{"repo dockerfile builder derives docker", dockerfileBuilder(), "docker"},
		{"explicit native runtime wins", nativeGo(), "go"},
		{"buildpack stays empty (bex extension)", buildpack(), ""},
		{"prebuilt image stays empty", sampleApp("web"), ""},
		{"static site stays empty", staticSite(), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil, tc.app)
			mux := http.NewServeMux()
			svc.RegisterREST(mux)
			rec := httptest.NewRecorder()
			mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
			if rec.Code != http.StatusOK {
				t.Fatalf("GET => %d: %s", rec.Code, rec.Body)
			}
			var out renderService
			if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
				t.Fatalf("decode: %v", err)
			}
			runtime, hasRuntime := out.ServiceDetails["runtime"]
			env, hasEnv := out.ServiceDetails["env"]
			if tc.want == "" {
				if hasRuntime || hasEnv {
					t.Fatalf("serviceDetails runtime=%v env=%v, want both omitted", runtime, env)
				}
				return
			}
			if runtime != tc.want || env != tc.want {
				t.Fatalf("serviceDetails runtime=%v env=%v, want both %q", runtime, env, tc.want)
			}
		})
	}
}

// TestDockerfileServiceRuntimeIsConsistentAcrossSurfaces proves the w4/052
// derivation is a pure read projection (spec.runtime stays empty) and that REST,
// GraphQL and MCP all report the derived "docker" runtime with the docker (not
// buildpack) envSpecificDetails shape, so the CLI reads one coherent service no
// matter which surface answers.
func TestDockerfileServiceRuntimeIsConsistentAcrossSurfaces(t *testing.T) {
	app := repoApp("web", "https://github.com/x/web", "main")
	app.Spec.DockerfilePath = "docker/Dockerfile.prod"
	svc, cl := newService(nil, app)
	ctx := context.Background()

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("GET => %d: %s", rec.Code, rec.Body)
	}
	var out renderService
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ServiceDetails["runtime"] != "docker" || out.ServiceDetails["env"] != "docker" {
		t.Fatalf("REST runtime=%v env=%v, want docker", out.ServiceDetails["runtime"], out.ServiceDetails["env"])
	}
	esd, _ := out.ServiceDetails["envSpecificDetails"].(map[string]any)
	if _, ok := esd["dockerfilePath"]; !ok {
		t.Fatalf("REST envSpecificDetails = %v, want docker shape (dockerfilePath key)", out.ServiceDetails["envSpecificDetails"])
	}
	if _, ok := esd["buildCommand"]; ok {
		t.Fatalf("REST envSpecificDetails carries a buildpack buildCommand key: %v", esd)
	}

	schema, err := graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{ service(id:"web") { runtime } }`})
	if len(res.Errors) > 0 {
		t.Fatal(res.Errors)
	}
	if got := res.Data.(map[string]any)["service"].(map[string]any)["runtime"]; got != "docker" {
		t.Fatalf("GraphQL runtime = %v, want docker", got)
	}

	// MCP get_service shares toRenderService with REST's GET.
	_, mcpService, err := svc.serviceTool(svc.Get)(ctx, nil, serviceArgs{ServiceID: "web"})
	if err != nil {
		t.Fatalf("MCP get_service: %v", err)
	}
	if mcpService.ServiceDetails["runtime"] != "docker" {
		t.Fatalf("MCP runtime = %v, want docker", mcpService.ServiceDetails["runtime"])
	}

	if got := getApp(t, cl, "web").Spec.Runtime; got != "" {
		t.Fatalf("spec.runtime = %q, want empty (a read derivation must not mutate the CR)", got)
	}
}

// TestDockerfilePathAcrossCreateSurfaces pins that dockerfilePath (Render's
// Dockerfile Path, relative to rootDir) round-trips identically through
// GraphQL's createService mutation and MCP's create_web_service tool — REST's
// equivalent is already covered by TestRenderDockerDetailsMapToDockerfileBuild.
func TestDockerfilePathAcrossCreateSurfaces(t *testing.T) {
	t.Run("graphql", func(t *testing.T) {
		svc, cl := newService(nil)
		schema, err := graphql.NewSchema(graphql.SchemaConfig{
			Query:    graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
			Mutation: graphql.NewObject(graphql.ObjectConfig{Name: "Mutation", Fields: svc.GraphQLMutation()}),
		})
		if err != nil {
			t.Fatal(err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { createService(name:"web", repo:"https://github.com/x/web", runtime:"docker", dockerfilePath:"docker/Dockerfile.prod") { dockerfilePath } }`})
		if len(res.Errors) > 0 {
			t.Fatal(res.Errors)
		}
		app := getApp(t, cl, "web")
		if app.Spec.DockerfilePath != "docker/Dockerfile.prod" {
			t.Fatalf("spec.dockerfilePath = %q, want docker/Dockerfile.prod", app.Spec.DockerfilePath)
		}
		data, _ := res.Data.(map[string]any)
		created, _ := data["createService"].(map[string]any)
		if created["dockerfilePath"] != "docker/Dockerfile.prod" {
			t.Fatalf("response dockerfilePath = %v", created["dockerfilePath"])
		}
	})

	t.Run("mcp", func(t *testing.T) {
		req := createWebServiceArgs{
			Name: "web", Repo: "https://github.com/x/web", Runtime: "docker",
			DockerfilePath: "docker/Dockerfile.prod",
		}.toCreateRequest()
		if req.DockerfilePath != "docker/Dockerfile.prod" {
			t.Fatalf("request.DockerfilePath = %q", req.DockerfilePath)
		}
	})
}

func TestCreateRejectsUnknownBuilder(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{Name: "web", Repo: "https://github.com/x/web", Builder: "magic"})
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("error = %v, want bad request", err)
	}
}
