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
	"strings"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m128 (from w8/150, w8/151): dockerContext is relative to rootDir on the
// wire, as on Render; the stored spec keeps the repository-root path the build
// reads.

func TestDockerContextTranslation(t *testing.T) {
	for _, tc := range []struct{ root, wire, stored string }{
		{"backend", ".", ""}, // Render's monorepo export shape
		{"backend", "", ""},
		{"backend", "ctx", "backend/ctx"},
		{"backend", "./ctx", "backend/ctx"},
		{"", "app", "app"}, // no rootDir: unchanged
		{"", ".", ""},
		{"a/b", "c/d", "a/b/c/d"},
	} {
		if got := storedDockerContext(tc.root, tc.wire); got != tc.stored {
			t.Errorf("storedDockerContext(%q, %q) = %q, want %q", tc.root, tc.wire, got, tc.stored)
		}
	}
	for _, tc := range []struct{ root, stored, wire string }{
		{"backend", "", "."},
		{"backend", "backend", "."}, // the pre-m128 hello-go shape
		{"examples/hello-go", "examples/hello-go", "."},
		{"backend", "backend/ctx", "ctx"},
		{"", "app", "app"},
		{"backend", "other", "../other"}, // stored outside rootDir: reported honestly
	} {
		if got := wireDockerContext(tc.root, tc.stored); got != tc.wire {
			t.Errorf("wireDockerContext(%q, %q) = %q, want %q", tc.root, tc.stored, got, tc.wire)
		}
	}
	for _, tc := range []struct{ oldRoot, newRoot, stored, want string }{
		{"a", "b", "a/ctx", "b/ctx"},
		{"a", "b", "", ""},
		{"a", "b", "a", ""},
		{"", "b", "ctx", "b/ctx"},
		{"a", "b", "other", "other"}, // outside the old root: left as stored
	} {
		if got := rebaseDockerContext(tc.oldRoot, tc.newRoot, tc.stored); got != tc.want {
			t.Errorf("rebaseDockerContext(%q, %q, %q) = %q, want %q", tc.oldRoot, tc.newRoot, tc.stored, got, tc.want)
		}
	}
}

// Sweep 1266's shape builds from <repo>/<rootDir>, and reads back "." as on
// Render; the control without dockerContext reads back "." too, not rootDir.
func TestCreateStoresRootDirRelativeDockerContext(t *testing.T) {
	for _, tc := range []struct {
		name, context, stored, view string
	}{
		{"dot", ".", "", "."},
		{"unset", "", "", "."},
		{"nested", "ctx", "example-pserv/ctx", "ctx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			view, err := svc.Create(context.Background(), CreateRequest{
				Name: "web", Repo: "https://github.com/render-examples/twingate-example", Runtime: "docker",
				RootDir: "example-pserv", DockerfilePath: "./Dockerfile", DockerContext: tc.context,
			})
			if err != nil {
				t.Fatalf("Create: %v", err)
			}
			if got := getApp(t, cl, "web").Spec.DockerContext; got != tc.stored {
				t.Errorf("spec.dockerContext = %q, want %q", got, tc.stored)
			}
			if view.DockerContext != tc.view {
				t.Errorf("view dockerContext = %q, want %q", view.DockerContext, tc.view)
			}
			rendered, _ := json.Marshal(renderServiceDetails(view, view.Type, ""))
			if !strings.Contains(string(rendered), `"dockerContext":"`+tc.view+`"`) {
				t.Errorf("REST shape lacks dockerContext %q: %s", tc.view, rendered)
			}
		})
	}
}

func TestBlueprintExportReportsRootDirRelativeDockerContext(t *testing.T) {
	for _, tc := range []struct{ stored, want string }{
		{"", ""},
		{"backend", ""}, // pre-m128 equal-to-rootDir
		{"backend/ctx", "ctx"},
	} {
		a := dockerRepoApp("web")
		a.Spec.RootDir, a.Spec.DockerContext = "backend", tc.stored
		svc, _ := newService(nil, a)
		entry, err := svc.generateServiceEntry(context.Background(), a, nil, nil, nil, nil)
		if err != nil {
			t.Fatalf("generateServiceEntry: %v", err)
		}
		got, _ := entry["dockerContext"].(string)
		if got != tc.want {
			t.Errorf("stored %q exports dockerContext %q, want %q", tc.stored, got, tc.want)
		}
		// Re-import compiles to the stored spec.
		if want := storedDockerContext("backend", got); want != storedDockerContext("backend", wireDockerContext("backend", tc.stored)) {
			t.Errorf("stored %q does not round-trip: %q", tc.stored, want)
		}
	}
}

func TestSetRootDirRebasesDockerContext(t *testing.T) {
	a := dockerRepoApp("web")
	a.Spec.RootDir, a.Spec.DockerContext = "a", "a/ctx"
	svc, cl := newService(nil, a)
	if _, err := svc.SetRootDir(context.Background(), "web", "b"); err != nil {
		t.Fatalf("SetRootDir: %v", err)
	}
	if got := getApp(t, cl, "web").Spec; got.RootDir != "b" || got.DockerContext != "b/ctx" {
		t.Fatalf("after rootDir change: rootDir %q dockerContext %q, want b, b/ctx", got.RootDir, got.DockerContext)
	}
}

// w8/151: PATCH accepts Render's envSpecificDetails.dockerContext.
func TestRESTPatchDockerContext(t *testing.T) {
	native := repoApp("native", "https://github.com/x/native", "main")
	native.Spec.Runtime, native.Spec.Builder = "node", "native"
	docker := dockerRepoApp("web")
	docker.Spec.RootDir = "backend"
	svc, cl := newService(nil, docker, native, sampleApp("image"))

	rec := serveRESTStrict(t, svc, http.MethodPatch, "/v1/services/web", `{"serviceDetails":{"envSpecificDetails":{"dockerContext":"ctx"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH dockerContext => %d: %s", rec.Code, rec.Body)
	}
	if !strings.Contains(rec.Body.String(), `"dockerContext":"ctx"`) {
		t.Errorf("PATCH response lacks dockerContext ctx: %s", rec.Body)
	}
	app := getApp(t, cl, "web")
	if app.Spec.DockerContext != "backend/ctx" || app.Spec.RestartedAt == "" {
		t.Fatalf("spec after PATCH: dockerContext %q restartedAt %q", app.Spec.DockerContext, app.Spec.RestartedAt)
	}

	// Both in one PATCH: the context resolves against the new root directory.
	rec = serveRESTStrict(t, svc, http.MethodPatch, "/v1/services/web", `{"rootDir":"api","serviceDetails":{"envSpecificDetails":{"dockerContext":"."}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("PATCH rootDir+dockerContext => %d: %s", rec.Code, rec.Body)
	}
	if got := getApp(t, cl, "web").Spec; got.RootDir != "api" || got.DockerContext != "" {
		t.Fatalf("after rootDir+context: rootDir %q dockerContext %q, want api, empty", got.RootDir, got.DockerContext)
	}

	for _, tc := range []struct{ name, body, want string }{
		{"web", `{"serviceDetails":{"envSpecificDetails":{"dockerContext":"../x"}}}`, "no '..'"},
		{"native", `{"serviceDetails":{"envSpecificDetails":{"dockerContext":"ctx"}}}`, "docker context"},
		{"image", `{"serviceDetails":{"envSpecificDetails":{"dockerContext":"ctx"}}}`, "docker context"},
	} {
		rec := serveRESTStrict(t, svc, http.MethodPatch, "/v1/services/"+tc.name, tc.body)
		if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), tc.want) {
			t.Errorf("PATCH %s %s => %d %s, want 400 naming %q", tc.name, tc.body, rec.Code, rec.Body, tc.want)
		}
	}

	before := getApp(t, cl, "web").ResourceVersion
	rec = serveRESTStrict(t, svc, http.MethodPatch, "/v1/services/web?dryRun=true", `{"serviceDetails":{"envSpecificDetails":{"dockerContext":"other"}}}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("dry-run PATCH => %d: %s", rec.Code, rec.Body)
	}
	if after := getApp(t, cl, "web").ResourceVersion; after != before {
		t.Errorf("dry-run wrote: resourceVersion %s -> %s", before, after)
	}
}

func TestSetDockerContextRefusesNonDockerBuilds(t *testing.T) {
	native := repoApp("native", "https://github.com/x/native", "main")
	native.Spec.Runtime, native.Spec.Builder = "node", "native"
	svc, _ := newService(nil, native)
	if _, err := svc.SetDockerContext(context.Background(), "native", "ctx"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("SetDockerContext on a native build = %v, want bad request", err)
	}
}

// A Blueprint that omits rootDir keeps the service's own (preserve on
// omission), so its dockerContext resolves inside that root directory; and a
// Blueprint that changes rootDir without naming dockerContext carries the
// context along, as SetRootDir does.
func TestBlueprintAppliersResolveDockerContextAgainstTheServiceRoot(t *testing.T) {
	applier := func(name string) func(*appv1alpha1.AppSpec, appv1alpha1.AppSpec) {
		for _, f := range blueprintServiceFieldAppliers {
			for _, n := range f.names {
				if n == name {
					return f.apply
				}
			}
		}
		t.Fatalf("no applier for %s", name)
		return nil
	}

	dst := appv1alpha1.AppSpec{RootDir: "backend", DockerContext: ""}
	want := appv1alpha1.AppSpec{DockerContext: storedDockerContext("", "ctx")} // file omits rootDir
	applier("dockerContext")(&dst, want)
	if dst.DockerContext != "backend/ctx" {
		t.Errorf("omitted rootDir: dockerContext = %q, want backend/ctx", dst.DockerContext)
	}
	if got := wireDockerContext(dst.RootDir, dst.DockerContext); got != "ctx" {
		t.Errorf("re-export reads %q, want ctx (no re-plan loop)", got)
	}

	dst = appv1alpha1.AppSpec{RootDir: "a", DockerContext: "a/ctx"}
	applier("rootDir")(&dst, appv1alpha1.AppSpec{RootDir: "b"})
	if dst.RootDir != "b" || dst.DockerContext != "b/ctx" {
		t.Errorf("rootDir change: %q / %q, want b / b/ctx", dst.RootDir, dst.DockerContext)
	}
}

// An unset dockerfilePath reads back Render's default "./Dockerfile" beside
// the "." context (w8/154); a stored path is reported as stored.
func TestRenderServiceDetailsDefaultsDockerfilePath(t *testing.T) {
	for _, tc := range []struct{ stored, want string }{
		{"", "./Dockerfile"},
		{"docker/Dockerfile.ci", "docker/Dockerfile.ci"},
	} {
		view := AppView{Runtime: "docker", DockerfilePath: tc.stored}
		details, _ := renderServiceDetails(view, appv1alpha1.TypeWebService, "")["envSpecificDetails"].(map[string]any)
		if got := details["dockerfilePath"]; got != tc.want {
			t.Errorf("stored %q reads %v, want %q", tc.stored, got, tc.want)
		}
	}
}

// GraphQL reads dockerContext back and createService accepts it the way REST
// does (production check 2026-10-29: Service had no dockerContext field and
// createService no dockerContext argument).
func TestGraphQLDockerContextMatchesREST(t *testing.T) {
	for _, tc := range []struct{ name, arg, want string }{
		{"dot", `, dockerContext: "."`, "."},
		{"unset", "", "."},
		{"nested", `, dockerContext: "ctx"`, "ctx"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil)
			schema := gqlSchema(t, svc)
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `mutation { createService(name: "web", repo: "https://github.com/render-examples/twingate-example", runtime: "docker", rootDir: "example-pserv", dockerfilePath: "./Dockerfile"` + tc.arg + `) { dockerContext } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("createService: %v", res.Errors)
			}
			if got := res.Data.(map[string]any)["createService"].(map[string]any)["dockerContext"]; got != tc.want {
				t.Errorf("createService dockerContext = %v, want %q", got, tc.want)
			}
			if got, want := getApp(t, cl, "web").Spec.DockerContext, storedDockerContext("example-pserv", tc.want); got != want {
				t.Errorf("spec.dockerContext = %q, want %q", got, want)
			}

			res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{ service(id: "web") { dockerContext } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("service query: %v", res.Errors)
			}
			gqlValue := res.Data.(map[string]any)["service"].(map[string]any)["dockerContext"]
			rec := serveRESTStrict(t, svc, http.MethodGet, "/v1/services/web", "")
			var rest struct {
				ServiceDetails struct {
					EnvSpecificDetails struct {
						DockerContext string `json:"dockerContext"`
					} `json:"envSpecificDetails"`
				} `json:"serviceDetails"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || rec.Code != http.StatusOK {
				t.Fatalf("GET => %d %v: %s", rec.Code, err, rec.Body)
			}
			if restValue := rest.ServiceDetails.EnvSpecificDetails.DockerContext; gqlValue != restValue || restValue != tc.want {
				t.Errorf("GraphQL dockerContext %v, REST %q, want both %q", gqlValue, restValue, tc.want)
			}
		})
	}

	// A context escaping the repository is refused with REST's message.
	const refusal = "dockerContext must be a relative path with no '..' components"
	svc, _ := newService(nil)
	rec := serveRESTStrict(t, svc, http.MethodPost, "/v1/services", `{"type":"web_service","name":"rest","repo":"https://github.com/x/web","serviceDetails":{"runtime":"docker","envSpecificDetails":{"dockerContext":"../../etc"}}}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), refusal) {
		t.Fatalf("REST create ../../etc => %d %s", rec.Code, rec.Body)
	}
	res := graphql.Do(graphql.Params{Schema: gqlSchema(t, svc), Context: context.Background(), RequestString: `mutation { createService(name: "web", repo: "https://github.com/x/web", runtime: "docker", dockerContext: "../../etc") { id } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, refusal) {
		t.Fatalf("GraphQL create ../../etc errors = %v, want REST's refusal", res.Errors)
	}
}
