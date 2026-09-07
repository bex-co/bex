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

// TestServiceReadRoundTripCompleteness is the w9/m93 class guard for the w4/052
// failure ("read output insufficient to reconstruct a valid write"). For every
// service build-strategy shape bex's create/normalization path can persist, the
// read must carry a build contract the official CLI can round-trip on a partial
// `services update`: serviceDetails.runtime/env present, non-empty, and equal for
// every runnable (non-static) type; equal to the runtime bex recomputes for the
// spec (so an explicit --runtime resend is never a "switch"); and a runtime-keyed
// envSpecificDetails whose SHAPE agrees with the runtime (docker keys vs native
// keys vs none). Runtime-less shapes (buildpack extension, static site) stay
// empty on purpose. env mirrors runtime — Render's deprecated response alias.
//
// w4/052 was a Dockerfile-via-builder web service reading back an empty runtime;
// this matrix now also pins the prebuilt-image round-trip (runtime "image", no
// envSpecificDetails — the container is supplied whole via imagePath).
func TestServiceReadRoundTripCompleteness(t *testing.T) {
	repo := func(mut func(*appv1alpha1.App)) *appv1alpha1.App {
		a := repoApp("web", "https://github.com/x/web", "main")
		if mut != nil {
			mut(a)
		}
		return a
	}
	image := func(mut func(*appv1alpha1.App)) *appv1alpha1.App {
		a := sampleApp("web") // Image set, no Repo — a prebuilt-image service.
		if mut != nil {
			mut(a)
		}
		return a
	}
	cases := []struct {
		name    string
		app     *appv1alpha1.App
		want    string // effective runtime; "" => runtime/env omitted entirely
		wantESD string // envSpecificDetails shape: "docker" | "native" | "none"
	}{
		{"repo default builder derives docker", repo(nil), "docker", "docker"},
		{"repo auto builder derives docker", repo(func(a *appv1alpha1.App) { a.Spec.Builder = "auto" }), "docker", "docker"},
		{"repo dockerfile builder derives docker", repo(func(a *appv1alpha1.App) { a.Spec.Builder = "dockerfile" }), "docker", "docker"},
		{"explicit docker runtime", repo(func(a *appv1alpha1.App) { a.Spec.Runtime = "docker"; a.Spec.Builder = "dockerfile" }), "docker", "docker"},
		{"explicit native go", repo(func(a *appv1alpha1.App) { a.Spec.Runtime = "go"; a.Spec.Builder = "native" }), "go", "native"},
		{"explicit native node", repo(func(a *appv1alpha1.App) { a.Spec.Runtime = "node"; a.Spec.Builder = "native" }), "node", "native"},
		{"prebuilt image derives image", image(nil), "image", "none"},
		{"explicit image runtime", image(func(a *appv1alpha1.App) { a.Spec.Runtime = "image" }), "image", "none"},
		{"buildpack stays empty (bex extension)", repo(func(a *appv1alpha1.App) { a.Spec.Builder = "buildpack" }), "", "none"},
		{"static site stays empty", repo(func(a *appv1alpha1.App) { a.Spec.Type = appv1alpha1.TypeStaticSite; a.Spec.PublishPath = "dist" }), "", "none"},
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
			} else if runtime != tc.want || env != tc.want {
				t.Fatalf("serviceDetails runtime=%v env=%v, want both %q", runtime, env, tc.want)
			}
			// The envSpecificDetails shape must agree with the runtime so the CLI
			// reads one coherent build block (docker keys, native keys, or none).
			esd, _ := out.ServiceDetails["envSpecificDetails"].(map[string]any)
			switch tc.wantESD {
			case "none":
				if _, present := out.ServiceDetails["envSpecificDetails"]; present {
					t.Fatalf("envSpecificDetails present = %v, want absent", out.ServiceDetails["envSpecificDetails"])
				}
			case "docker":
				if _, ok := esd["dockerfilePath"]; !ok {
					t.Fatalf("envSpecificDetails = %v, want docker shape (dockerfilePath key)", esd)
				}
				if _, ok := esd["buildCommand"]; ok {
					t.Fatalf("envSpecificDetails carries a native buildCommand key: %v", esd)
				}
			case "native":
				if _, ok := esd["buildCommand"]; !ok {
					t.Fatalf("envSpecificDetails = %v, want native shape (buildCommand key)", esd)
				}
				if _, ok := esd["dockerfilePath"]; ok {
					t.Fatalf("envSpecificDetails carries a docker dockerfilePath key: %v", esd)
				}
			}
		})
	}
}

// TestEffectiveRuntimeIsTotal (w9/m93) proves the spec→runtime projection is
// total: no build-strategy shape the create API accepts leaves a runnable service
// without a runtime — the silent-empty state that stranded the CLI in w4/052.
// Only the enumerated no-runtime shapes may read back empty (buildpack extension
// and native without an explicit language runtime name no Render runtime; a
// static site has none by design). The builder list mirrors the CRD Builder enum
// (types/v1alpha1 app_types.go: auto;buildpack;dockerfile;native) — a builder
// added there without a runtime mapping trips this.
func TestEffectiveRuntimeIsTotal(t *testing.T) {
	builders := []string{"", "auto", "buildpack", "dockerfile", "native"}
	runnable := []string{
		appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob,
	}
	// Builders that legitimately expose no Render runtime when spec.runtime is
	// empty: buildpack is a bex-only strategy, and native without a language
	// runtime cannot name one.
	noRuntime := map[string]bool{"buildpack": true, "native": true}
	for _, ty := range runnable {
		for _, b := range builders {
			got := effectiveRuntime(appv1alpha1.AppSpec{Type: ty, Repo: "https://github.com/x/web", Builder: b}, ty)
			if noRuntime[b] {
				continue
			}
			if got == "" {
				t.Errorf("effectiveRuntime(type=%s builder=%q repo) = empty; a runnable repo build must expose a runtime (w4/052)", ty, b)
			}
		}
		if got := effectiveRuntime(appv1alpha1.AppSpec{Type: ty, Image: "nginx:alpine"}, ty); got != "image" {
			t.Errorf("effectiveRuntime(type=%s image) = %q, want image", ty, got)
		}
	}
	if got := effectiveRuntime(appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/x/web"}, appv1alpha1.TypeStaticSite); got != "" {
		t.Errorf("static site effectiveRuntime = %q, want empty (bex has no static runtime)", got)
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

// TestImageServiceRuntimeIsConsistentAcrossSurfaces is the w9/m93 parity check
// for the prebuilt-image read change: a runtime-less image service reads back
// runtime "image" identically on REST, GraphQL and MCP (all fed by the one
// view() projection), with no envSpecificDetails, and without mutating the CR.
func TestImageServiceRuntimeIsConsistentAcrossSurfaces(t *testing.T) {
	svc, cl := newService(nil, sampleApp("web")) // Image set, no Repo, no runtime
	ctx := context.Background()

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", "/v1/services/web", nil))
	var out renderService
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.ServiceDetails["runtime"] != "image" || out.ServiceDetails["env"] != "image" {
		t.Fatalf("REST runtime=%v env=%v, want image", out.ServiceDetails["runtime"], out.ServiceDetails["env"])
	}
	if _, present := out.ServiceDetails["envSpecificDetails"]; present {
		t.Fatalf("REST envSpecificDetails present = %v, want absent for a prebuilt image", out.ServiceDetails["envSpecificDetails"])
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
	if got := res.Data.(map[string]any)["service"].(map[string]any)["runtime"]; got != "image" {
		t.Fatalf("GraphQL runtime = %v, want image", got)
	}

	_, mcpService, err := svc.serviceTool(svc.Get)(ctx, nil, serviceArgs{ServiceID: "web"})
	if err != nil {
		t.Fatalf("MCP get_service: %v", err)
	}
	if mcpService.ServiceDetails["runtime"] != "image" {
		t.Fatalf("MCP runtime = %v, want image", mcpService.ServiceDetails["runtime"])
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
