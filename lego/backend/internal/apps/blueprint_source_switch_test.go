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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// sweep294's own source switch fixtures (w8/m70).
const (
	switchRepo  = "https://github.com/expressjs/serve-static"
	switchImage = "docker.io/mendhak/http-https-echo:35"
)

func imageBornSpec() appv1alpha1.AppSpec {
	return appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Runtime: "image", Image: switchImage,
		Branch: "main", Tier: "free", Replicas: 1, Port: 3000}
}

func gitBornSpec() appv1alpha1.AppSpec {
	return appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Runtime: "node", Repo: switchRepo,
		Branch: "master", Builder: buildNative, BuildCommand: "true", StartCommand: "node server.js", Tier: "free", Replicas: 1, Port: 3000}
}

// switchSource applies manifest to a managed App holding initial through the
// real compile, update and canonical-row path, then reapplies it unchanged.
func switchSource(t *testing.T, initial appv1alpha1.AppSpec, manifest string) (appv1alpha1.AppSpec, *recordingStore) {
	t.Helper()
	return switchSourceWith(t, initial, DeployRequest{Manifest: manifest})
}

func switchSourceWith(t *testing.T, initial appv1alpha1.AppSpec, req DeployRequest) (appv1alpha1.AppSpec, *recordingStore) {
	t.Helper()
	st, _, err := compileStack(req)
	if err != nil {
		t.Fatal(err)
	}
	decl := st.services[0]
	want, err := specFromCreate(decl.req)
	if err != nil {
		t.Fatal(err)
	}
	existing := blueprintIntentApp(true)
	existing.Spec = initial
	cl := fakeClient(existing)
	rec := &recordingStore{}
	svc := &Service{Base: &core.Base{Client: cl, Namespace: "default"}, Store: rec}
	if _, err := svc.updateStackService(context.Background(), decl.req, decl.fields, want, existing); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, cl, existing.Name)
	deploys := len(rec.deployCalls)
	if _, err := svc.updateStackService(context.Background(), decl.req, decl.fields, want, got); err != nil {
		t.Fatal(err)
	}
	if len(rec.deployCalls) != deploys {
		t.Errorf("identical reapply opened another deploy: %d -> %d", deploys, len(rec.deployCalls))
	}
	return got.Spec, rec
}

// A declared source replaces the former one, in the CR and the canonical row,
// so the deploy builds or pulls what the file declares; the same source
// changes nothing.
func TestBlueprintSourceSwitchReplacesTheFormerSource(t *testing.T) {
	native := "services:\n- {name: own-source, type: web, runtime: node, repo: " + switchRepo + ", branch: master, plan: free, buildCommand: 'true', startCommand: 'node server.js'}\n"
	docker := "services:\n- {name: own-source, type: web, runtime: docker, repo: " + switchRepo + ", branch: master, plan: free}\n"
	image := "services:\n- {name: own-source, type: web, runtime: image, image: {url: '" + switchImage + "'}, plan: free}\n"
	for _, tc := range []struct {
		name, manifest               string
		initial                      appv1alpha1.AppSpec
		repo, image, branch, runtime string
		deploys                      int
	}{
		{"image to native Git", native, imageBornSpec(), switchRepo, "", "master", "node", 1},
		{"image to Dockerfile Git", docker, imageBornSpec(), switchRepo, "", "master", "docker", 1},
		{"Git to image", image, gitBornSpec(), "", switchImage, "master", "image", 1},
		{"same Git", native, gitBornSpec(), switchRepo, "", "master", "node", 0},
		{"same image", image, imageBornSpec(), "", switchImage, "main", "image", 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, rec := switchSource(t, tc.initial, tc.manifest)
			if got.Repo != tc.repo || got.Image != tc.image || got.Branch != tc.branch || got.Runtime != tc.runtime {
				t.Errorf("source = repo %q image %q branch %q runtime %q, want %q %q %q %q",
					got.Repo, got.Image, got.Branch, got.Runtime, tc.repo, tc.image, tc.branch, tc.runtime)
			}
			if len(rec.deployCalls) != tc.deploys {
				t.Errorf("deploys = %d, want %d", len(rec.deployCalls), tc.deploys)
			}
			if tc.deploys > 0 {
				if len(rec.sourceWrites) != 1 || rec.sourceWrites[0].repo != tc.repo || rec.sourceWrites[0].image != tc.image {
					t.Errorf("canonical source writes = %+v, want one (%q, %q)", rec.sourceWrites, tc.repo, tc.image)
				}
			}
		})
	}
}

// The same repo, however spelled, keeps a rollback's image override, and an
// unchanged one opens no deploy; a new repo replaces it, as the source setter
// does.
func TestBlueprintSourceSwitchKeepsARollbackOverrideOnTheSameRepo(t *testing.T) {
	rolledBack := gitBornSpec()
	rolledBack.Image = "registry.example/own-source@sha256:rollback"
	native := func(repo string) string {
		return "services:\n- {name: own-source, type: web, runtime: node, repo: " + repo + ", branch: master, plan: free, buildCommand: 'true', startCommand: 'node server.js'}\n"
	}
	if got, rec := switchSource(t, rolledBack, native(switchRepo)); got.Image != rolledBack.Image || len(rec.deployCalls) != 0 {
		t.Errorf("same repo: image %q, %d deploys; want the override kept and no deploy", got.Image, len(rec.deployCalls))
	}
	// A respelling is the same repo: the declared spelling is written, but
	// the override stays.
	if got, _ := switchSource(t, rolledBack, native(switchRepo+".git")); got.Image != rolledBack.Image {
		t.Errorf("respelled repo dropped the rollback override: image %q", got.Image)
	}
	if got, _ := switchSource(t, rolledBack, native("https://github.com/expressjs/express")); got.Image != "" {
		t.Errorf("new repo kept the rollback override %q", got.Image)
	}
}

// An image's registry credential and the old origin's clone token do not
// follow a switch to Git; a credential the file declares does, a Git-to-Git
// move keeps a Dockerfile build's credential, and an image-born service with
// no branch gets the declared default rather than an empty ref.
func TestBlueprintImageToGitDropsImageOnlySettings(t *testing.T) {
	gitFields := map[string]BlueprintField{"repo": {}, "runtime": {}}
	want := appv1alpha1.AppSpec{Repo: switchRepo, Runtime: "docker", Branch: appv1alpha1.DefaultBranch}

	imageBorn := imageBornSpec()
	imageBorn.Branch = ""
	imageBorn.RegistryCredentialID = new("rc-image")
	imageBorn.CloneSecret = "old-clone"
	got := imageBorn
	if _, err := ApplyBlueprintServiceSpec(&got, want, gitFields); err != nil {
		t.Fatal(err)
	}
	if got.RegistryCredentialID != nil || got.CloneSecret != "" || got.Branch != appv1alpha1.DefaultBranch {
		t.Errorf("after image->Git: credential %v clone secret %q branch %q, want none, none, %q",
			got.RegistryCredentialID, got.CloneSecret, got.Branch, appv1alpha1.DefaultBranch)
	}

	declared := imageBorn
	withCreds := want
	withCreds.RegistryCredentialID = new("rc-declared")
	if _, err := ApplyBlueprintServiceSpec(&declared, withCreds, map[string]BlueprintField{"repo": {}, "runtime": {}, "registryCredential": {}}); err != nil {
		t.Fatal(err)
	}
	if declared.RegistryCredentialID == nil || *declared.RegistryCredentialID != "rc-declared" {
		t.Errorf("declared credential = %v, want rc-declared", declared.RegistryCredentialID)
	}

	dockerfile := gitBornSpec()
	dockerfile.Runtime, dockerfile.Builder = "docker", ""
	dockerfile.RegistryCredentialID = new("rc-from")
	moved := want
	moved.Repo = "https://github.com/expressjs/express"
	if _, err := ApplyBlueprintServiceSpec(&dockerfile, moved, gitFields); err != nil {
		t.Fatal(err)
	}
	if dockerfile.RegistryCredentialID == nil || *dockerfile.RegistryCredentialID != "rc-from" {
		t.Errorf("Git->Git dropped the Dockerfile build's credential: %v", dockerfile.RegistryCredentialID)
	}
}

// A Git entry that inherits its repo from the Blueprint switches an
// image-backed service as a declared repo would.
func TestBlueprintInheritedRepoSwitchesAnImageService(t *testing.T) {
	got, rec := switchSourceWith(t, imageBornSpec(), DeployRequest{Repo: switchRepo, Branch: "master", ContainingSource: true,
		Manifest: "services:\n- {name: own-source, type: web, runtime: node, plan: free, buildCommand: 'true', startCommand: 'node server.js'}\n"})
	if got.Repo != switchRepo || got.Image != "" || got.Runtime != "node" || len(rec.deployCalls) != 1 {
		t.Errorf("inherited repo: repo %q image %q runtime %q, %d deploys; want the Git source, node, one deploy",
			got.Repo, got.Image, got.Runtime, len(rec.deployCalls))
	}
}

// A native start command was baked into the image it built, so a switch to
// a prebuilt image that declares no command drops it rather than running it
// as the image's /bin/sh override; a Dockerfile build's command already is
// that override and stays, as the source setter keeps it (w4/m202).
func TestBlueprintGitToImageDropsABakedStartCommand(t *testing.T) {
	image := "services:\n- {name: own-source, type: web, runtime: image, image: {url: '" + switchImage + "'}, plan: free}\n"
	if got, _ := switchSource(t, gitBornSpec(), image); got.StartCommand != "" {
		t.Errorf("native->image kept start command %q", got.StartCommand)
	}
	dockerfile := gitBornSpec()
	dockerfile.Runtime, dockerfile.Builder, dockerfile.BuildCommand = "docker", "", ""
	if got, _ := switchSource(t, dockerfile, image); got.StartCommand != "node server.js" {
		t.Errorf("Dockerfile->image start command = %q, want the docker command kept", got.StartCommand)
	}
	cron := gitBornSpec()
	cron.Type, cron.Schedule = appv1alpha1.TypeCronJob, "0 0 1 1 *"
	got, _ := switchSource(t, cron, "services:\n- {name: own-source, type: cron, runtime: image, image: {url: '"+switchImage+"'}, schedule: '0 0 1 1 *', plan: starter}\n")
	if got.Repo != "" || got.Image != switchImage || got.StartCommand != "" {
		t.Errorf("native cron->image: repo %q image %q start %q, want the image alone", got.Repo, got.Image, got.StartCommand)
	}
}

// The plan attributes the switch to the declared repo and runtime, the fields
// the apply changes, so preview and apply agree.
func TestBlueprintPlanAttributesASourceSwitchToRepo(t *testing.T) {
	svc, _ := newService(nil)
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: "services:\n- {name: own-source, type: web, runtime: image, image: {url: '" + switchImage + "'}, plan: free}\n"}); err != nil {
		t.Fatal(err)
	}
	manifest := "services:\n- {name: own-source, type: web, runtime: node, repo: " + switchRepo + ", branch: master, plan: free, buildCommand: 'true', startCommand: 'node server.js'}\n"
	actions := planActionsForTest(ctx, t, svc, manifest)
	if len(actions) != 1 || actions[0].Operation != BlueprintPlanUpdate {
		t.Fatalf("plan = %+v, want one update", actions)
	}
	listed := map[string]bool{}
	for _, change := range actions[0].ChangedFields {
		listed[change.Path] = true
	}
	if !listed["repo"] || !listed["runtime"] {
		t.Errorf("changed fields = %+v, want repo and runtime among them", actions[0].ChangedFields)
	}
}
