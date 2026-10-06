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
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCreateRefusesBuildSettingsItsBuildNeverReads (w5/090): create used to
// store a setting the build ignores, which update then refused, so a service
// could hold configuration that did nothing and could not be set back. Create
// now refuses each with update's answer. setter is the update call for the
// same setting on a service created without it, where one exists.
func TestCreateRefusesBuildSettingsItsBuildNeverReads(t *testing.T) {
	ctx := context.Background()
	repo := "https://github.com/acme/app"
	for _, tc := range []struct {
		name   string
		inert  CreateRequest
		base   CreateRequest // the same service without the setting
		setter func(svc *Service, name string) error
		want   string // create's own wording, where no update call takes the setting
	}{
		{name: "a builder on a prebuilt image",
			inert: CreateRequest{Name: "img", Image: "nginx:1", Builder: "dockerfile"},
			want:  "prebuilt image services cannot declare builder; remove it or deploy from repo instead"},
		{name: "a dockerfile path on a buildpack build",
			inert: CreateRequest{Name: "bp", Repo: repo, Builder: "buildpack", DockerfilePath: "Dockerfile.prod"},
			base:  CreateRequest{Name: "bp", Repo: repo, Builder: "buildpack"},
			setter: func(svc *Service, name string) error {
				_, err := svc.SetDockerfilePath(ctx, name, "Dockerfile.prod")
				return err
			}},
		{name: "a dockerfile path on a native runtime",
			inert: CreateRequest{Name: "nat", Repo: repo, Runtime: "node", BuildCommand: "npm ci", StartCommand: "npm start", DockerfilePath: "Dockerfile.prod"},
			base:  CreateRequest{Name: "nat", Repo: repo, Runtime: "node", BuildCommand: "npm ci", StartCommand: "npm start"},
			setter: func(svc *Service, name string) error {
				_, err := svc.SetDockerfilePath(ctx, name, "Dockerfile.prod")
				return err
			}},
		{name: "a docker context on a buildpack build",
			inert: CreateRequest{Name: "bpc", Repo: repo, Builder: "buildpack", DockerContext: "services/api"},
			want:  "docker context only applies to a Dockerfile-built service"},
		{name: "a docker context on a prebuilt image",
			inert: CreateRequest{Name: "imc", Image: "nginx:1", DockerContext: "services/api"},
			want:  "docker context only applies to a Dockerfile-built service"},
		{name: "a dockerfile path on a static site's native build",
			inert: CreateRequest{Name: "site-nat", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist", Runtime: "node", BuildCommand: "npm run build", DockerfilePath: "Dockerfile.site"},
			base:  CreateRequest{Name: "site-nat", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist", Runtime: "node", BuildCommand: "npm run build"},
			setter: func(svc *Service, name string) error {
				_, err := svc.SetDockerfilePath(ctx, name, "Dockerfile.site")
				return err
			}},
		{name: "a dockerfile path on a static site's buildpack build",
			inert: CreateRequest{Name: "site-bp", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist", Builder: "buildpack", DockerfilePath: "Dockerfile.site"},
			base:  CreateRequest{Name: "site-bp", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist", Builder: "buildpack"},
			setter: func(svc *Service, name string) error {
				_, err := svc.SetDockerfilePath(ctx, name, "Dockerfile.site")
				return err
			}},
		{name: "a docker context on a static site that only publishes",
			inert: CreateRequest{Name: "site-pub", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist", DockerContext: "site"},
			want:  "docker context only applies to a Dockerfile-built service"},
		{name: "a start command on a static site",
			inert: CreateRequest{Name: "site", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist", StartCommand: "serve dist"},
			base:  CreateRequest{Name: "site", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist"},
			setter: func(svc *Service, name string) error {
				start := "serve dist"
				_, err := svc.SetCommands(ctx, name, nil, &start)
				return err
			}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil)
			_, createErr := svc.Create(ctx, tc.inert)
			if !errors.Is(createErr, core.ErrBadRequest) {
				t.Fatalf("create = %v, want a 400", createErr)
			}
			want := core.ErrBadRequest.Error() + ": " + tc.want
			if tc.setter != nil {
				if _, err := svc.Create(ctx, tc.base); err != nil {
					t.Fatalf("create without the setting: %v", err)
				}
				setErr := tc.setter(svc, tc.base.Name)
				if setErr == nil {
					t.Fatal("update accepted the setting; the case does not pin update's refusal")
				}
				want = setErr.Error()
			}
			if createErr.Error() != want {
				t.Fatalf("create refusal = %q\nwant            %q", createErr, want)
			}
		})
	}

	// Where the build reads them, the same settings still create.
	for _, req := range []CreateRequest{
		{Name: "dockerfile", Repo: repo, Runtime: "docker", DockerfilePath: "Dockerfile.prod", DockerContext: "services/api"},
		{Name: "dockerfile-auto", Repo: repo, DockerfilePath: "Dockerfile.prod", DockerContext: "services/api"},
		{Name: "static-dockerfile", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist", DockerfilePath: "Dockerfile.site"},
		// A static site's native build needs its build command only.
		{Name: "static-native", Type: appv1alpha1.TypeStaticSite, Repo: repo, PublishPath: "dist", Runtime: "node", BuildCommand: "npm run build"},
	} {
		svc, _ := newService(nil)
		if _, err := svc.Create(ctx, req); err != nil {
			t.Errorf("%s: create: %v", req.Name, err)
		}
	}
}

// TestALegacyInertBuildSettingStaysSyncable (w5/090): a service created before
// create refused them can hold a Dockerfile-build setting on another build.
// Its export leaves the setting out, so the file still validates, and syncing
// it keeps the stored value, since omission preserves it.
func TestALegacyInertBuildSettingStaysSyncable(t *testing.T) {
	ctx := context.Background()
	svc, cl := newService(nil)
	req := CreateRequest{Name: "legacy", Repo: "https://github.com/acme/app", Runtime: "docker",
		DockerfilePath: "Dockerfile.prod", DockerContext: "services/api"}
	if _, err := svc.Create(ctx, req); err != nil {
		t.Fatal(err)
	}
	app := getApp(t, cl, req.Name)
	app.Spec.Runtime, app.Spec.Builder = "", "buildpack" // as an older create could store it
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{ServiceIDs: []string{req.Name}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if v, err := svc.ValidateBlueprint(ctx, "", out.Manifest, ""); err != nil || !v.Valid {
		t.Fatalf("the legacy service's export does not validate: %+v %v\n%s", v.Errors, err, out.Manifest)
	}
}

// TestBlueprintRefusesDockerfileSettingsBesideAnotherBuild (w5/090): the
// compiler locates each at its own field, as it does dockerCommand.
func TestBlueprintRefusesDockerfileSettingsBesideAnotherBuild(t *testing.T) {
	for _, tc := range []struct{ name, service, path string }{
		{"a docker context on a native runtime", "runtime: node\n    repo: https://github.com/acme/app\n    buildCommand: npm ci\n    startCommand: npm start\n    dockerContext: services/api", "#/services/0/dockerContext"},
		{"a dockerfile path on a buildpack build", "runtime: docker\n    repo: https://github.com/acme/app\n    dockerfilePath: Dockerfile.prod\n    x-bex:\n      builder: buildpack", "#/services/0/dockerfilePath"},
		{"a docker context on a prebuilt image", "runtime: image\n    image: {url: nginx:1}\n    dockerContext: services/api", "#/services/0/dockerContext"},
		{"a builder on a prebuilt image", "runtime: image\n    image: {url: nginx:1}\n    x-bex:\n      builder: dockerfile", "#/services/0/x-bex/builder"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, problems := CompileBlueprintSource("services:\n  - type: web\n    name: api\n    " + tc.service + "\n")
			problem := findBlueprintProblem(problems, "BLUEPRINT_CAPABILITY_INCOMPATIBLE")
			if problem == nil || problem.Path != tc.path {
				t.Fatalf("problems = %+v, want BLUEPRINT_CAPABILITY_INCOMPATIBLE at %s", problems, tc.path)
			}
		})
	}
}

// TestBlueprintSyncOffADockerfileBuildClearsItsSettings (w5/090): a file that
// moves a Dockerfile service to a native runtime and omits the Dockerfile
// settings no longer leaves them on a build that never reads them.
func TestBlueprintSyncOffADockerfileBuildClearsItsSettings(t *testing.T) {
	stored := appv1alpha1.AppSpec{Repo: "https://github.com/acme/app", Runtime: "docker", Builder: "dockerfile",
		DockerfilePath: "Dockerfile.prod", DockerContext: "services/api"}
	want := appv1alpha1.AppSpec{Runtime: "node", BuildCommand: "npm ci", StartCommand: "npm start"}
	fields := map[string]BlueprintField{"runtime": {}, "buildCommand": {}, "startCommand": {}}
	if _, err := ApplyBlueprintServiceSpec(&stored, want, fields); err != nil {
		t.Fatal(err)
	}
	if stored.DockerfilePath != "" || stored.DockerContext != "" {
		t.Fatalf("native service kept dockerfilePath %q, dockerContext %q", stored.DockerfilePath, stored.DockerContext)
	}
}
