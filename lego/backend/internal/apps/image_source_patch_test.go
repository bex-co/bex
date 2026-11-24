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
	yes, no, build, dev := true, false, "make", "dev"
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
		{"patch branch", "branch", func(s *Service) error {
			_, err := s.ApplyServicePatch(ctx, "web", ServicePatch{Branch: &dev})
			return err
		}},
		{"graphql setBranch", "branch", func(s *Service) error {
			_, err := s.SetSourceAndRegistryCredential(ctx, "web", sourcePatch{Branch: &dev})
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
			if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "web"}, &got); err != nil || got.Spec.AutoDeploy || got.Spec.BuildCommand != "" || got.Spec.Branch != appv1alpha1.DefaultBranch {
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
	// Echoing the read value ("") for branch stays a no-op; non-empty branch
	// values on a prebuilt-image service are refused (w32/110, superseding
	// w4/418's staged-branch rationale after w32/108).
	t.Run("empty branch echo stays accepted as a no-op", func(t *testing.T) {
		empty := ""
		svc, _ := newService(nil, managedImageApp("web"))
		if _, err := svc.ApplyServicePatch(ctx, "web", ServicePatch{Branch: &empty}); err != nil {
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

// Echoing a switched-image service's read build settings is a no-op on every
// write surface: the plan stays empty, the apply writes nothing (no
// restartedAt bump), and the retained spec fields survive for a switch back.
// Real changes to those frozen settings are still refused. Extends w32/103's
// rootDir echo (and w9/068's maintenanceMode, w9/113's no-op) to the sibling
// projections of the same contract: dockerfilePath, dockerContext,
// buildFilter, runtime, and buildCommand.
func TestSwitchedImageServiceEchoesSiblingBuildSettings(t *testing.T) {
	ctx := context.Background()
	const (
		image = "docker.io/library/nginx:1.27-alpine"
		root  = "examples/hello-node"
	)
	// A Dockerfile build with every retained field populated. (A Dockerfile
	// build's own build command does not survive the switch:
	// settleSourceSwitch drops it from the spec since an image service
	// refuses it; only native commands move to annotations. The echo case
	// for a retained build command is covered below with a legacy CR.)
	switched := func(t *testing.T) (*Service, client.Client) {
		t.Helper()
		app := gitServiceApp("svc", appv1alpha1.TypeWebService)
		app.Spec.Tier = "starter"
		app.Spec.Replicas = 1
		app.Spec.Runtime = "docker"
		app.Spec.Builder = "dockerfile"
		app.Spec.BuildCommand = ""
		app.Spec.StartCommand = ""
		app.Spec.DockerfilePath = "docker/Dockerfile.prod"
		app.Spec.DockerContext = root + "/ctx"
		app.Spec.RootDir = root
		svc, cl := newService(nil, app)
		target := image
		if _, err := svc.ApplyServicePatch(ctx, "svc", ServicePatch{Image: &target}); err != nil {
			t.Fatalf("switch to image: %v", err)
		}
		stored := getApp(t, cl, "svc").Spec
		if stored.DockerfilePath != "docker/Dockerfile.prod" || stored.DockerContext != root+"/ctx" ||
			stored.BuildFilter == nil ||
			stored.Runtime != "docker" || stored.Builder != "dockerfile" {
			t.Fatalf("switch dropped retained Git fields: %+v", stored)
		}
		return svc, cl
	}
	// The apply must write nothing: restartedAt stays as it was, retained
	// fields byte for byte.
	assertNoop := func(t *testing.T, cl client.Client, before appv1alpha1.AppSpec) {
		t.Helper()
		after := getApp(t, cl, "svc").Spec
		if after.RestartedAt != before.RestartedAt {
			t.Errorf("echo bumped restartedAt to %q (was %q): a no-op echo rebuilt the service", after.RestartedAt, before.RestartedAt)
		}
		if after.DockerfilePath != before.DockerfilePath || after.DockerContext != before.DockerContext ||
			!reflect.DeepEqual(after.BuildFilter, before.BuildFilter) || after.BuildCommand != before.BuildCommand ||
			after.Runtime != before.Runtime || after.Builder != before.Builder || after.RootDir != before.RootDir {
			t.Errorf("echo changed retained fields:\nbefore: %+v\n after: %+v", before, after)
		}
	}

	t.Run("echoes are accepted with no write", func(t *testing.T) {
		empty, blank := "", "   "
		rtImage := "image"
		for _, tc := range []struct {
			name  string
			patch ServicePatch
		}{
			{"dockerfilePath", ServicePatch{DockerfilePath: &empty}},
			{"dockerContext", ServicePatch{DockerContext: &empty}},
			{"buildFilter nil", ServicePatch{BuildFilter: nil}},
			{"runtime image", ServicePatch{Runtime: &rtImage}},
			{"buildCommand empty", ServicePatch{BuildCommand: &blank}},
		} {
			t.Run("patch/"+tc.name, func(t *testing.T) {
				svc, cl := switched(t)
				before := getApp(t, cl, "svc").Spec
				if _, err := svc.ApplyServicePatch(ctx, "svc", tc.patch); err != nil {
					t.Fatalf("PATCH echo %s = %v, want accepted no-op", tc.name, err)
				}
				assertNoop(t, cl, before)
			})
		}
		// An explicitly empty (all-blank) filter also normalizes to unset.
		t.Run("patch/buildFilter blank", func(t *testing.T) {
			svc, cl := switched(t)
			before := getApp(t, cl, "svc").Spec
			if _, err := svc.ApplyServicePatch(ctx, "svc", ServicePatch{BuildFilter: &BuildFilterView{Paths: []string{"  "}}}); err != nil {
				t.Fatalf("PATCH echo blank buildFilter = %v, want accepted no-op", err)
			}
			assertNoop(t, cl, before)
		})
		t.Run("verbs", func(t *testing.T) {
			svc, cl := switched(t)
			before := getApp(t, cl, "svc").Spec
			if _, err := svc.SetDockerfilePath(ctx, "svc", ""); err != nil {
				t.Fatalf("SetDockerfilePath echo = %v, want accepted no-op", err)
			}
			if _, err := svc.SetDockerContext(ctx, "svc", ""); err != nil {
				t.Fatalf("SetDockerContext echo = %v, want accepted no-op", err)
			}
			if _, err := svc.SetBuildFilter(ctx, "svc", nil); err != nil {
				t.Fatalf("SetBuildFilter echo = %v, want accepted no-op", err)
			}
			if _, err := svc.SetRuntime(ctx, "svc", "image"); err != nil {
				t.Fatalf("SetRuntime echo = %v, want accepted no-op", err)
			}
			if _, err := svc.SetCommands(ctx, "svc", &empty, nil); err != nil {
				t.Fatalf("SetCommands echo = %v, want accepted no-op", err)
			}
			assertNoop(t, cl, before)
		})
		// The REST read shape round-trips through PATCH unchanged.
		t.Run("rest", func(t *testing.T) {
			svc, cl := switched(t)
			before := getApp(t, cl, "svc").Spec
			for _, body := range []string{
				`{"serviceDetails":{"envSpecificDetails":{"dockerfilePath":""}}}`,
				`{"serviceDetails":{"envSpecificDetails":{"dockerContext":""}}}`,
				`{"serviceDetails":{"runtime":"image"}}`,
				`{"buildFilter":{}}`,
			} {
				rec := serveRESTStrict(t, svc, http.MethodPatch, "/v1/services/svc", body)
				if rec.Code != http.StatusOK {
					t.Fatalf("PATCH %s = %d: %s, want accepted no-op", body, rec.Code, rec.Body.String())
				}
			}
			assertNoop(t, cl, before)
		})
	})

	t.Run("real changes are still refused and change nothing", func(t *testing.T) {
		otherPath, otherCtx, makeCmd := "other/Dockerfile", "other", "make"
		rtDocker := "docker"
		for _, tc := range []struct {
			name  string
			patch ServicePatch
			want  string
		}{
			{"dockerfilePath", ServicePatch{DockerfilePath: &otherPath}, "dockerfile path only applies to"},
			{"dockerContext", ServicePatch{DockerContext: &otherCtx}, "docker context only applies to"},
			{"buildFilter", ServicePatch{BuildFilter: &BuildFilterView{Paths: []string{"docs/**"}}}, "build filters only apply to"},
			{"buildCommand", ServicePatch{BuildCommand: &makeCmd}, "prebuilt image services cannot declare buildCommand"},
			{"runtime", ServicePatch{Runtime: &rtDocker}, "runtime docker requires repo and no image"},
		} {
			t.Run(tc.name, func(t *testing.T) {
				svc, cl := switched(t)
				before := getApp(t, cl, "svc").Spec
				if _, err := svc.ApplyServicePatch(ctx, "svc", tc.patch); err == nil || !strings.Contains(err.Error(), tc.want) {
					t.Fatalf("PATCH %s = %v, want refusal containing %q", tc.name, err, tc.want)
				}
				assertNoop(t, cl, before)
			})
		}
	})

	t.Run("echoes preserve switch back", func(t *testing.T) {
		svc, cl := switched(t)
		empty, rtImage := "", "image"
		for _, p := range []ServicePatch{
			{DockerfilePath: &empty},
			{DockerContext: &empty},
			{BuildFilter: nil},
			{Runtime: &rtImage},
			{BuildCommand: &empty},
		} {
			if _, err := svc.ApplyServicePatch(ctx, "svc", p); err != nil {
				t.Fatalf("PATCH echo %+v = %v", p, err)
			}
		}
		targetRepo, targetBranch := "https://github.com/bex-co/bex", "main"
		if _, err := svc.ApplyServicePatch(ctx, "svc", ServicePatch{Repo: &targetRepo, Branch: &targetBranch}); err != nil {
			t.Fatalf("switch back to Git: %v", err)
		}
		back := getApp(t, cl, "svc").Spec
		if back.Runtime != "docker" || back.Builder != "dockerfile" ||
			back.DockerfilePath != "docker/Dockerfile.prod" || back.DockerContext != root+"/ctx" ||
			back.RootDir != root || back.BuildFilter == nil {
			t.Errorf("switch back lost retained fields: %+v", back)
		}
	})

	// A prebuilt-image CR that still carries a build command directly (legacy
	// records predate the create refusal, w4/418): echoing the read value ""
	// must not clear what reads hide.
	t.Run("legacy retained buildCommand echo", func(t *testing.T) {
		a := managedImageApp("legacy")
		a.Spec.BuildCommand = "npm ci"
		svc, cl := newService(nil, a)
		empty := ""
		if _, err := svc.SetCommands(ctx, "legacy", &empty, nil); err != nil {
			t.Fatalf("SetCommands echo = %v, want accepted no-op", err)
		}
		if _, err := svc.ApplyServicePatch(ctx, "legacy", ServicePatch{BuildCommand: &empty}); err != nil {
			t.Fatalf("PATCH echo = %v, want accepted no-op", err)
		}
		if got := getApp(t, cl, "legacy").Spec.BuildCommand; got != "npm ci" {
			t.Errorf("echo cleared retained buildCommand: %q", got)
		}
	})
}

// Echoing a repo-backed service's read build settings changes nothing either:
// before this fix, the plan stayed empty while the apply bumped restartedAt
// and started a spurious rebuild.
func TestGitServiceEchoesBuildSettingsWithoutRebuild(t *testing.T) {
	ctx := context.Background()
	mk := func(t *testing.T) *Service {
		t.Helper()
		app := gitServiceApp("svc", appv1alpha1.TypeWebService)
		app.Spec.Tier = "starter"
		app.Spec.Replicas = 1
		app.Spec.Runtime = "docker"
		app.Spec.Builder = "dockerfile"
		app.Spec.DockerfilePath = "docker/Dockerfile.prod"
		app.Spec.DockerContext = "examples/hello-node/ctx"
		app.Spec.RootDir = "examples/hello-node"
		svc, _ := newService(nil, app)
		return svc
	}
	t.Run("verbs", func(t *testing.T) {
		svc := mk(t)
		before := getApp(t, svc.Client, "svc").Spec
		if _, err := svc.SetDockerfilePath(ctx, "svc", "docker/Dockerfile.prod"); err != nil {
			t.Fatalf("echo dockerfilePath = %v", err)
		}
		if _, err := svc.SetDockerContext(ctx, "svc", "ctx"); err != nil {
			t.Fatalf("echo dockerContext = %v", err)
		}
		if _, err := svc.SetRootDir(ctx, "svc", "examples/hello-node"); err != nil {
			t.Fatalf("echo rootDir = %v", err)
		}
		if _, err := svc.SetBuildFilter(ctx, "svc", &BuildFilterView{Paths: []string{"examples/hello-node/**"}}); err != nil {
			t.Fatalf("echo buildFilter = %v", err)
		}
		after := getApp(t, svc.Client, "svc").Spec
		if after.RestartedAt != before.RestartedAt {
			t.Errorf("echo bumped restartedAt to %q (was %q)", after.RestartedAt, before.RestartedAt)
		}
		if !reflect.DeepEqual(after, before) {
			t.Errorf("echo changed the spec:\nbefore: %+v\n after: %+v", before, after)
		}
	})
	t.Run("patch", func(t *testing.T) {
		svc := mk(t)
		before := getApp(t, svc.Client, "svc").Spec
		path, wireCtx, rootDir := "docker/Dockerfile.prod", "ctx", "examples/hello-node"
		if _, err := svc.ApplyServicePatch(ctx, "svc", ServicePatch{
			DockerfilePath: &path, DockerContext: &wireCtx, RootDir: &rootDir,
			BuildFilter: &BuildFilterView{Paths: []string{"examples/hello-node/**"}},
		}); err != nil {
			t.Fatalf("PATCH echo = %v", err)
		}
		after := getApp(t, svc.Client, "svc").Spec
		if after.RestartedAt != before.RestartedAt {
			t.Errorf("PATCH echo bumped restartedAt to %q (was %q)", after.RestartedAt, before.RestartedAt)
		}
		if !reflect.DeepEqual(after, before) {
			t.Errorf("PATCH echo changed the spec:\nbefore: %+v\n after: %+v", before, after)
		}
	})
}
