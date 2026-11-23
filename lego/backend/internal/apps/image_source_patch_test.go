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
	"reflect"
	"strings"
	"testing"

	"github.com/graphql-go/graphql"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func managedImageApp(name string) *appv1alpha1.App {
	a := managedRepoApp(name)
	a.Spec.Repo, a.Spec.Runtime, a.Spec.BuildCommand = "", "image", ""
	a.Spec.Image = "docker.io/traefik/whoami:v1.10.3"
	return a
}

// An image service refuses the build-from-git settings its own create
// refuses, on every write surface (w4/418).
func TestImageServiceRefusesGitSettingsOnUpdate(t *testing.T) {
	ctx := context.Background()
	yes, no, build := true, false, "make"
	for _, tc := range []struct {
		name, field string
		edit        func(*Service) error
	}{
		{"patch autoDeploy", "autoDeploy", func(s *Service) error {
			_, err := s.ApplyServicePatch(ctx, "web", ServicePatch{AutoDeploy: &yes})
			return err
		}},
		{"graphql setAutoDeploy", "autoDeploy", func(s *Service) error {
			_, err := s.SetAutoDeploy(ctx, "web", true)
			return err
		}},
		{"patch buildCommand", "buildCommand", func(s *Service) error {
			_, err := s.ApplyServicePatch(ctx, "web", ServicePatch{BuildCommand: &build})
			return err
		}},
		{"graphql setBuildCommand", "buildCommand", func(s *Service) error {
			_, err := s.SetCommands(ctx, "web", &build, nil)
			return err
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, cl := newService(nil, managedImageApp("web"))
			err := tc.edit(svc)
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "prebuilt image services cannot declare "+tc.field) {
				t.Fatalf("err = %v, want the create refusal for %s", err, tc.field)
			}
			var got appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &got); err != nil || got.Spec.AutoDeploy || got.Spec.BuildCommand != "" {
				t.Fatalf("refused write changed the App: %+v, %v", got.Spec, err)
			}
		})
	}

	t.Run("disabling stays accepted", func(t *testing.T) {
		svc, _ := newService(nil, managedImageApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{AutoDeploy: &no}); err != nil {
			t.Fatal(err)
		}
	})
	// A branch is kept, hidden from reads, for a later switch to a repo: the
	// CLI sends image and branch together, and w8/088's two-step switch sets
	// branch first.
	t.Run("a staged branch stays accepted", func(t *testing.T) {
		branch := "main"
		svc, _ := newService(nil, managedImageApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Branch: &branch}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("switching to a repo with auto-deploy on in one patch works", func(t *testing.T) {
		repo := "https://github.com/bex-co/hello.git"
		svc, _ := newService(nil, managedImageApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Repo: &repo, AutoDeploy: &yes}); err != nil {
			t.Fatal(err)
		}
	})
	t.Run("a repo-backed service still toggles", func(t *testing.T) {
		svc, _ := newService(nil, managedRepoApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{AutoDeploy: &yes}); err != nil {
			t.Fatal(err)
		}
	})
}

// An image service stored with auto-deploy on reads back as off, as its
// branch does (w4/418).
func TestImageServiceReadsAutoDeployOff(t *testing.T) {
	a := managedImageApp("web")
	a.Spec.AutoDeploy = true
	svc, _ := newService(nil, a)
	v, err := svc.Get(context.Background(), "web")
	if err != nil || v.AutoDeploy || v.AutoDeployTrigger != "off" {
		t.Fatalf("view autoDeploy=%v trigger=%q (%v), want off", v.AutoDeploy, v.AutoDeployTrigger, err)
	}
}

// A cron job's command and schedule are refused by name on every other type,
// instead of being dropped (w4/424). A cron job is unchanged.
func TestCreateRefusesCronFieldsOnOtherTypes(t *testing.T) {
	ctx := context.Background()
	for _, svcType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker} {
		for field, req := range map[string]CreateRequest{
			"command":  {Name: "x", Type: svcType, Image: "busybox:1", Command: "httpd -f -p 10000"},
			"schedule": {Name: "x", Type: svcType, Image: "busybox:1", Schedule: "*/5 * * * *"},
		} {
			svc, _ := newService(nil)
			req.DryRun = true
			_, err := svc.Create(ctx, req)
			var coded *core.CodedError
			if !errors.As(err, &coded) || coded.Params["field"] != field || !strings.Contains(err.Error(), "cron_job") {
				t.Errorf("%s with %s = %v, want a named refusal", svcType, field, err)
			}
			if field == "command" && !strings.Contains(err.Error(), "startCommand") {
				t.Errorf("%s command refusal does not point at startCommand: %v", svcType, err)
			}
		}
	}
	svc, _ := newService(nil)
	if _, err := svc.Create(ctx, CreateRequest{Name: "cron", Type: appv1alpha1.TypeCronJob, Image: "busybox:1", Command: "echo hi", Schedule: "*/5 * * * *", DryRun: true}); err != nil {
		t.Fatalf("cron create = %v, want accepted", err)
	}
	rec := serveRESTStrict(t, svc, http.MethodPost, "/v1/services",
		`{"type":"web_service","name":"web","image":{"imagePath":"busybox:1"},"command":"httpd -f"}`)
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "startCommand") {
		t.Fatalf("REST web create with command = %d %s, want a named 400", rec.Code, rec.Body)
	}
}

// w32/103: switching a Git service with rootDir, buildFilter, dockerfilePath,
// dockerContext, or builder to a prebuilt image retains those fields on
// App.Spec for a switch back, but every read surface (REST, GraphQL, MCP) must
// hide them so `services create --from` can clone the switched image service
// and switching back to Git restores them intact.
func TestSwitchedImageServiceHidesGitBuildFieldsAcrossSurfacesAndClones(t *testing.T) {
	const (
		image = "docker.io/library/nginx:1.27-alpine"
		repo  = "https://github.com/bex-co/bex"
	)
	type shape struct {
		name           string
		runtime        string
		builder        string
		buildCommand   string
		startCommand   string
		dockerfilePath string
		dockerContext  string // stored repo-root-relative value
		wantContext    string // wire rootDir-relative value when on Git
	}
	shapes := []shape{
		{
			name:         "native",
			runtime:      "node",
			builder:      "native",
			buildCommand: "npm install",
			startCommand: "node main.js",
		},
		{
			name:           "dockerfile",
			runtime:        "docker",
			builder:        "dockerfile",
			dockerfilePath: "docker/Dockerfile.prod",
			dockerContext:  "examples/hello-node/ctx",
			wantContext:    "ctx",
		},
	}
	types := []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
	}

	gqlRead := func(t *testing.T, svc *Service, id string) map[string]any {
		t.Helper()
		res := graphql.Do(graphql.Params{
			Schema:  gqlSchema(t, svc),
			Context: context.Background(),
			RequestString: `{ service(id: "` + id + `") {
				runtime rootDir dockerfilePath dockerContext builder buildCommand branch autoDeploy autoDeployTrigger
				buildFilter { paths ignoredPaths }
			} }`,
		})
		if len(res.Errors) > 0 {
			t.Fatalf("GraphQL service(%s): %v", id, res.Errors)
		}
		return res.Data.(map[string]any)["service"].(map[string]any)
	}

	for _, sh := range shapes {
		for _, svcType := range types {
			t.Run(sh.name+"/"+svcType, func(t *testing.T) {
				ctx := context.Background()
				app := gitServiceApp("svc", svcType)
				app.Spec.Tier = "starter"
				app.Spec.Replicas = 1
				app.Spec.AutoDeploy = true
				app.Spec.Runtime = sh.runtime
				app.Spec.Builder = sh.builder
				app.Spec.BuildCommand = sh.buildCommand
				app.Spec.StartCommand = sh.startCommand
				app.Spec.DockerfilePath = sh.dockerfilePath
				app.Spec.DockerContext = sh.dockerContext
				app.Spec.BuildFilter = &appv1alpha1.BuildFilterSpec{
					Paths:        []string{"examples/hello-node/**"},
					IgnoredPaths: []string{"docs/**"},
				}
				if svcType == appv1alpha1.TypeCronJob && sh.runtime == "docker" {
					app.Spec.Command = "echo tick"
				}

				svc, cl := newService(nil, app)
				targetImage := image
				if _, err := svc.ApplyServicePatch(ctx, "svc", ServicePatch{Image: &targetImage}); err != nil {
					t.Fatalf("switch to image: %v", err)
				}

				// Retained on App.Spec for a switch back.
				stored := getApp(t, cl, "svc").Spec
				if stored.RootDir != "examples/hello-node" || stored.BuildFilter == nil ||
					stored.Builder != sh.builder || stored.DockerfilePath != sh.dockerfilePath ||
					stored.DockerContext != sh.dockerContext {
					t.Fatalf("spec lost retained Git fields after image switch: %+v", stored)
				}

				// 1. REST read: rootDir must be present and "", buildFilter omitted,
				// branch omitted/"", autoDeploy "no", envSpecificDetails build fields "".
				rec := serveRESTStrict(t, svc, http.MethodGet, "/v1/services/svc", "")
				if rec.Code != http.StatusOK {
					t.Fatalf("GET /v1/services/svc = %d: %s", rec.Code, rec.Body.String())
				}
				var restRead map[string]any
				if err := json.Unmarshal(rec.Body.Bytes(), &restRead); err != nil {
					t.Fatal(err)
				}
				rootDirVal, hasRootDir := restRead["rootDir"]
				if !hasRootDir || rootDirVal != "" {
					t.Errorf("REST rootDir = %v (present=%v), want empty string present", rootDirVal, hasRootDir)
				}
				if _, hasBF := restRead["buildFilter"]; hasBF {
					t.Errorf("REST buildFilter leaked on image service: %v", restRead["buildFilter"])
				}
				if b, _ := restRead["branch"].(string); b != "" {
					t.Errorf("REST branch = %q, want empty", b)
				}
				if restRead["autoDeploy"] != "no" || restRead["autoDeployTrigger"] != "off" {
					t.Errorf("REST autoDeploy=%v trigger=%v, want no/off", restRead["autoDeploy"], restRead["autoDeployTrigger"])
				}
				details, envSpec := serviceDetailsOf(restRead)
				if details["runtime"] != "image" || envSpec["dockerfilePath"] != "" || envSpec["dockerContext"] != "" {
					t.Errorf("REST serviceDetails runtime=%v envSpecificDetails=%v", details["runtime"], envSpec)
				}

				// 2. GraphQL read agrees on every hidden Git-only field.
				gql := gqlRead(t, svc, "svc")
				wantGQL := map[string]any{
					"runtime":           "image",
					"rootDir":           "",
					"dockerfilePath":    "",
					"dockerContext":     "",
					"builder":           "",
					"buildCommand":      "",
					"branch":            "",
					"autoDeploy":        false,
					"autoDeployTrigger": "off",
					"buildFilter":       nil,
				}
				if !reflect.DeepEqual(gql, wantGQL) {
					t.Errorf("GraphQL switched image service =\n  %#v\nwant:\n  %#v", gql, wantGQL)
				}

				// 3. MCP get_service agrees.
				call, cleanup := appsMCPClient(t, svc)
				defer cleanup()
				mcpRead := call("get_service", map[string]any{"serviceId": "svc"})
				if mcpRead["rootDir"] != "" {
					t.Errorf("MCP rootDir = %v, want \"\"", mcpRead["rootDir"])
				}
				if _, hasBF := mcpRead["buildFilter"]; hasBF {
					t.Errorf("MCP buildFilter leaked: %v", mcpRead["buildFilter"])
				}
				mcpDetails, mcpEnv := serviceDetailsOf(mcpRead)
				if mcpDetails["runtime"] != "image" || mcpEnv["dockerfilePath"] != "" || mcpEnv["dockerContext"] != "" {
					t.Errorf("MCP serviceDetails runtime=%v envSpecificDetails=%v", mcpDetails["runtime"], mcpEnv)
				}

				// 4. Replaying the REST read through `POST /v1/services` (as
				// `bex services create --from` does, pkg/service/clone.go)
				// succeeds with 201.
				cloneDetails := map[string]any{
					"runtime":            details["runtime"],
					"plan":               details["plan"],
					"envSpecificDetails": envSpec,
				}
				if svcType == appv1alpha1.TypeCronJob {
					cloneDetails["schedule"] = details["schedule"]
					if cmd, ok := details["command"]; ok {
						cloneDetails["command"] = cmd
					}
				}
				clonePayload := map[string]any{
					"name":           "svc-clone",
					"type":           restRead["type"],
					"autoDeploy":     restRead["autoDeploy"],
					"rootDir":        restRead["rootDir"],
					"image":          map[string]any{"imagePath": restRead["imagePath"], "ownerId": ""},
					"serviceDetails": cloneDetails,
				}
				if bf, ok := restRead["buildFilter"]; ok {
					clonePayload["buildFilter"] = bf
				}
				if b, ok := restRead["branch"]; ok {
					clonePayload["branch"] = b
				}
				cloneBytes, err := json.Marshal(clonePayload)
				if err != nil {
					t.Fatal(err)
				}
				cloneRec := serveRESTStrict(t, svc, http.MethodPost, "/v1/services", string(cloneBytes))
				if cloneRec.Code != http.StatusCreated {
					t.Fatalf("clone POST /v1/services = %d: %s", cloneRec.Code, cloneRec.Body.String())
				}

				// 5. Echoing rootDir "" in a PATCH (as Terraform does) is a no-op
				// that preserves retained spec.RootDir; sending the old rootDir is
				// refused as a Git-only setting.
				noopRec := serveRESTStrict(t, svc, http.MethodPatch, "/v1/services/svc", `{"rootDir":""}`)
				if noopRec.Code != http.StatusOK {
					t.Fatalf("PATCH rootDir \"\" on switched image service = %d: %s", noopRec.Code, noopRec.Body.String())
				}
				if got := getApp(t, cl, "svc").Spec.RootDir; got != "examples/hello-node" {
					t.Errorf("PATCH rootDir \"\" cleared retained spec.RootDir: %q", got)
				}
				badRec := serveRESTStrict(t, svc, http.MethodPatch, "/v1/services/svc", `{"rootDir":"examples/hello-node"}`)
				if badRec.Code != http.StatusBadRequest || !strings.Contains(badRec.Body.String(), "root directory only applies to build-from-git") {
					t.Fatalf("PATCH retained rootDir on switched image service = %d: %s", badRec.Code, badRec.Body.String())
				}

				// 6. Switching back to Git restores the retained fields on all
				// three surfaces.
				targetRepo, targetBranch := repo, "main"
				if _, err := svc.ApplyServicePatch(ctx, "svc", ServicePatch{Repo: &targetRepo, Branch: &targetBranch}); err != nil {
					t.Fatalf("switch back to Git: %v", err)
				}
				backRec := serveRESTStrict(t, svc, http.MethodGet, "/v1/services/svc", "")
				if backRec.Code != http.StatusOK {
					t.Fatalf("GET after switch back = %d: %s", backRec.Code, backRec.Body.String())
				}
				var backREST map[string]any
				if err := json.Unmarshal(backRec.Body.Bytes(), &backREST); err != nil {
					t.Fatal(err)
				}
				if backREST["rootDir"] != "examples/hello-node" || backREST["buildFilter"] == nil {
					t.Errorf("REST after switch back lost rootDir/buildFilter: rootDir=%v buildFilter=%v", backREST["rootDir"], backREST["buildFilter"])
				}
				backGQL := gqlRead(t, svc, "svc")
				if backGQL["rootDir"] != "examples/hello-node" || backGQL["builder"] != sh.builder ||
					backGQL["dockerfilePath"] != sh.dockerfilePath || backGQL["dockerContext"] != sh.wantContext ||
					backGQL["buildCommand"] != sh.buildCommand || backGQL["buildFilter"] == nil {
					t.Errorf("GraphQL after switch back = %#v, want restored Git settings", backGQL)
				}
				backMCP := call("get_service", map[string]any{"serviceId": "svc"})
				if backMCP["rootDir"] != "examples/hello-node" || backMCP["buildFilter"] == nil {
					t.Errorf("MCP after switch back lost rootDir/buildFilter: rootDir=%v buildFilter=%v", backMCP["rootDir"], backMCP["buildFilter"])
				}
			})
		}
	}

	// Defense in depth: even a prebuilt-image CR that retains spec.BuildCommand
	// directly (not via settleSourceSwitch) hides it in view(a).
	t.Run("legacy retained buildCommand hidden on image CR", func(t *testing.T) {
		a := managedImageApp("legacy")
		a.Spec.BuildCommand = "npm ci"
		a.Spec.RootDir = "apps/web"
		a.Spec.Builder = "native"
		a.Spec.DockerfilePath = "Dockerfile.alt"
		a.Spec.DockerContext = "apps/web/ctx"
		a.Spec.BuildFilter = &appv1alpha1.BuildFilterSpec{Paths: []string{"apps/web/**"}}
		v := view(a)
		if v.BuildCommand != "" || v.Builder != "" || v.RootDir != "" || v.DockerfilePath != "" || v.DockerContext != "" || v.BuildFilter != nil {
			t.Errorf("view(legacy image CR) leaked Git fields: %+v", v)
		}
	})

	// Control: native, Dockerfile, and static Git services read back their Git
	// build settings unchanged across REST, GraphQL, and MCP.
	t.Run("git controls unchanged", func(t *testing.T) {
		native := gitServiceApp("ctrl-native", appv1alpha1.TypeWebService)
		native.Spec.Builder = "native"
		docker := gitServiceApp("ctrl-docker", appv1alpha1.TypeWebService)
		docker.Spec.Runtime, docker.Spec.Builder, docker.Spec.BuildCommand, docker.Spec.StartCommand = "docker", "dockerfile", "", ""
		docker.Spec.DockerfilePath = "docker/Dockerfile.prod"
		docker.Spec.DockerContext = "examples/hello-node/ctx"
		static := gitServiceApp("ctrl-static", appv1alpha1.TypeStaticSite)
		static.Spec.Runtime, static.Spec.Builder, static.Spec.StartCommand, static.Spec.PublishPath = "", "native", "", "dist"

		svc, _ := newService(nil, native, docker, static)
		call, cleanup := appsMCPClient(t, svc)
		defer cleanup()

		for _, tc := range []struct {
			id, wantBuilder, wantBuildCmd, wantDockerfile, wantContext string
		}{
			{"ctrl-native", "native", "npm install", "", ""},
			{"ctrl-docker", "dockerfile", "", "docker/Dockerfile.prod", "ctx"},
			{"ctrl-static", "native", "npm install", "", ""},
		} {
			rec := serveRESTStrict(t, svc, http.MethodGet, "/v1/services/"+tc.id, "")
			if rec.Code != http.StatusOK {
				t.Fatalf("GET %s = %d: %s", tc.id, rec.Code, rec.Body.String())
			}
			var restRead map[string]any
			if err := json.Unmarshal(rec.Body.Bytes(), &restRead); err != nil {
				t.Fatal(err)
			}
			if restRead["rootDir"] != "examples/hello-node" || restRead["buildFilter"] == nil {
				t.Errorf("REST %s lost rootDir/buildFilter: %v / %v", tc.id, restRead["rootDir"], restRead["buildFilter"])
			}
			gql := gqlRead(t, svc, tc.id)
			if gql["rootDir"] != "examples/hello-node" || gql["builder"] != tc.wantBuilder ||
				gql["buildCommand"] != tc.wantBuildCmd || gql["dockerfilePath"] != tc.wantDockerfile ||
				gql["dockerContext"] != tc.wantContext || gql["buildFilter"] == nil {
				t.Errorf("GraphQL %s = %#v", tc.id, gql)
			}
			mcpRead := call("get_service", map[string]any{"serviceId": tc.id})
			if mcpRead["rootDir"] != "examples/hello-node" || mcpRead["buildFilter"] == nil {
				t.Errorf("MCP %s lost rootDir/buildFilter: %v / %v", tc.id, mcpRead["rootDir"], mcpRead["buildFilter"])
			}
		}
	})
}
