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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/yaml"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// gitServiceApp is a native Git service carrying every Git-only setting a
// later switch to a prebuilt image leaves on the App.
func gitServiceApp(name string, serviceType string) *appv1alpha1.App {
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: map[string]string{
			core.LabelServiceName: name, core.LabelAppID: "srv-" + name,
		}},
		Spec: appv1alpha1.AppSpec{
			Type: serviceType, Repo: "https://github.com/bex-co/bex", Branch: "main",
			Runtime: "node", RootDir: "examples/hello-node", BuildCommand: "npm install", StartCommand: "npm start",
			BuildFilter: &appv1alpha1.BuildFilterSpec{Paths: []string{"examples/hello-node/**"}},
		},
	}
	if serviceType == appv1alpha1.TypeCronJob {
		app.Spec.Schedule = "*/5 * * * *"
	}
	return app
}

func generatedService(t *testing.T, svc *Service, id string) (map[string]any, string) {
	t.Helper()
	out, err := svc.GenerateBlueprint(context.Background(), GenerateBlueprintRequest{ServiceIDs: []string{id}})
	if err != nil {
		t.Fatalf("GenerateBlueprint(%s) = %v", id, err)
	}
	if v, err := svc.ValidateBlueprint(context.Background(), "", out.Manifest, ""); err != nil || !v.Valid {
		t.Fatalf("generated manifest must validate: %+v err=%v\n%s", v, err, out.Manifest)
	}
	var doc struct {
		Services []map[string]any `json:"services"`
	}
	if err := yaml.Unmarshal([]byte(out.Manifest), &doc); err != nil || len(doc.Services) != 1 {
		t.Fatalf("manifest services = %d (err %v):\n%s", len(doc.Services), err, out.Manifest)
	}
	return doc.Services[0], out.Manifest
}

// w4/288: switching a Git service to a prebuilt image keeps its Git settings
// on the App for a switch back. The export still emitted rootDir beside
// runtime: image, which its own check refuses, so a working image service
// could not be exported at all. The source is switched through the real save.
func TestGenerateBlueprintAfterGitToImageSourceSwitch(t *testing.T) {
	const image = "docker.io/library/busybox:1.37"
	for _, serviceType := range []string{
		appv1alpha1.TypeWebService, appv1alpha1.TypePrivateService, appv1alpha1.TypeBackgroundWorker, appv1alpha1.TypeCronJob,
	} {
		t.Run(serviceType, func(t *testing.T) {
			ctx := context.Background()
			cl := fakeClient(gitServiceApp("svc", serviceType))
			svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}}
			target := image
			if _, err := svc.SetSourceAndRegistryCredential(ctx, "svc", sourcePatch{Image: &target}); err != nil {
				t.Fatalf("switch to image: %v", err)
			}
			var stored appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: "svc"}, &stored); err != nil {
				t.Fatal(err)
			}
			if stored.Spec.Image != image || stored.Spec.RootDir != "examples/hello-node" || stored.Spec.BuildFilter == nil {
				t.Fatalf("after the switch spec = image %q rootDir %q filter %v; want the image with the Git settings kept",
					stored.Spec.Image, stored.Spec.RootDir, stored.Spec.BuildFilter)
			}

			entry, manifest := generatedService(t, svc, "srv-svc")
			if entry["runtime"] != "image" || entry["image"].(map[string]any)["url"] != image {
				t.Fatalf("entry = %v, want the image service", entry)
			}
			for _, gitOnly := range []string{"rootDir", "buildFilter", "buildCommand", "repo", "branch"} {
				if _, declared := entry[gitOnly]; declared {
					t.Errorf("image entry declares %s:\n%s", gitOnly, manifest)
				}
			}
			if entry["dockerCommand"] != "npm start" {
				t.Errorf("dockerCommand = %v, want the saved command kept", entry["dockerCommand"])
			}

			// Back on Git, the kept settings export again.
			repo := "https://github.com/bex-co/bex"
			if _, err := svc.SetSourceAndRegistryCredential(ctx, "svc", sourcePatch{Repo: &repo}); err != nil {
				t.Fatalf("switch back to Git: %v", err)
			}
			entry, manifest = generatedService(t, svc, "srv-svc")
			if entry["rootDir"] != "examples/hello-node" || entry["buildFilter"] == nil || entry["runtime"] != "node" {
				t.Fatalf("Git entry lost its settings:\n%s", manifest)
			}
		})
	}
}

// Git-built shapes keep rootDir and buildFilter: native, Dockerfile and static.
func TestGenerateBlueprintGitSourcesKeepRootDirAndBuildFilter(t *testing.T) {
	native := gitServiceApp("native", appv1alpha1.TypeWebService)
	docker := gitServiceApp("docker", appv1alpha1.TypeWebService)
	docker.Spec.Runtime, docker.Spec.BuildCommand, docker.Spec.StartCommand = "docker", "", ""
	static := gitServiceApp("static", appv1alpha1.TypeStaticSite)
	static.Spec.Runtime, static.Spec.StartCommand, static.Spec.PublishPath = "", "", "dist"
	svc := &Service{Base: &core.Base{Client: fakeClient(native, docker, static), Namespace: "default"}}
	for _, id := range []string{"srv-native", "srv-docker", "srv-static"} {
		entry, manifest := generatedService(t, svc, id)
		if entry["rootDir"] != "examples/hello-node" || entry["buildFilter"] == nil {
			t.Errorf("%s lost rootDir or buildFilter:\n%s", id, manifest)
		}
	}
}

// REST, GraphQL and MCP share the verb: after the switch each returns the
// same valid manifest rather than BLUEPRINT_GENERATE_FAILED.
func TestGenerateBlueprintAfterSourceSwitchOnEverySurface(t *testing.T) {
	ctx := context.Background()
	svc := &Service{Base: &core.Base{Client: fakeClient(gitServiceApp("svc", appv1alpha1.TypeWebService)), Namespace: "default"}}
	image := "docker.io/library/busybox:1.37"
	if _, err := svc.SetSourceAndRegistryCredential(ctx, "svc", sourcePatch{Image: &image}); err != nil {
		t.Fatal(err)
	}
	_, want := generatedService(t, svc, "srv-svc")

	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/v1/blueprints/generate", strings.NewReader(`{"serviceIds":["srv-svc"]}`)))
	var restOut GenerateBlueprintResult
	if err := json.Unmarshal(rec.Body.Bytes(), &restOut); rec.Code != http.StatusOK || err != nil || restOut.Manifest != want {
		t.Fatalf("REST generate => %d (err %v): %s", rec.Code, err, rec.Body)
	}

	res := graphql.Do(graphql.Params{Schema: blueprintSchema(t, svc), Context: ctx,
		RequestString: `{ generateBlueprint(serviceIds: ["srv-svc"]) { manifest } }`})
	if len(res.Errors) > 0 || res.Data.(map[string]any)["generateBlueprint"].(map[string]any)["manifest"] != want {
		t.Fatalf("GraphQL generate: errors %v", res.Errors)
	}

	call, cleanup := appsMCPClient(t, svc)
	defer cleanup()
	if out := call("generate_blueprint", map[string]any{"serviceIds": []string{"srv-svc"}}); out["manifest"] != want {
		t.Fatalf("MCP manifest differs: %v", out)
	}
}
