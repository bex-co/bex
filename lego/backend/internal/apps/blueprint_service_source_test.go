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
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The Blueprint lives in one public repo while its service declares another.
const (
	containingRepo   = "https://github.com/nodejs/node"
	containingBranch = "main"
	declaredRepo     = "https://github.com/expressjs/serve-static"
)

// declaredSourceManifest declares a repo and branch other than the Blueprint's.
var declaredSourceManifest = sourceServiceManifest("    repo: " + declaredRepo + "\n    branch: master\n")

func sourceServiceManifest(source string) string {
	return "services:\n  - name: own-source\n    type: web\n    runtime: node\n    plan: free\n    buildCommand: 'true'\n    startCommand: node server.js\n" + source
}

// nestSourceManifest moves a root services: list under one project
// environment, the nested declaration shape.
func nestSourceManifest(root string) string {
	body := strings.TrimSuffix(root, "\n")
	return "projects:\n  - name: own-project\n    environments:\n      - name: own-env\n        " + strings.ReplaceAll(body, "\n", "\n        ") + "\n"
}

type serviceSourceCase struct {
	name, manifest, repo, branch string
}

// serviceSourceCases are the declarations a containing Blueprint source must
// respect or fill, at the root and nested in a project environment.
func serviceSourceCases() []serviceSourceCase {
	root := []struct{ name, source, repo, branch string }{
		{"declared repo and branch", "    repo: " + declaredRepo + "\n    branch: master\n", declaredRepo, "master"},
		{"declared repo with the Blueprint's branch name", "    repo: " + declaredRepo + "\n    branch: main\n", declaredRepo, "main"},
		{"declared branch only", "    branch: master\n", containingRepo, "master"},
		{"same repo with another branch", "    repo: " + containingRepo + "\n    branch: master\n", containingRepo, "master"},
		{"omitted source inherits the Blueprint's", "", containingRepo, containingBranch},
		{"declared source equal to the default", "    repo: " + containingRepo + "\n    branch: main\n", containingRepo, containingBranch},
		{"respelled same repo inherits the Blueprint branch", "    repo: " + containingRepo + ".git\n", containingRepo + ".git", containingBranch},
	}
	out := make([]serviceSourceCase, 0, 2*len(root))
	for _, tc := range root {
		out = append(out, serviceSourceCase{"root/" + tc.name, sourceServiceManifest(tc.source), tc.repo, tc.branch})
		out = append(out, serviceSourceCase{"nested/" + tc.name, nestSourceManifest(sourceServiceManifest(tc.source)), tc.repo, tc.branch})
	}
	return out
}

func containingBlueprint() store.Blueprint {
	return store.Blueprint{ID: "blp-own", TenantID: "tea-a", Name: "own", Repo: containingRepo, Branch: containingBranch,
		Path: CanonicalBlueprintFilename, Status: store.BlueprintStatusInSync}
}

// A persistent sync's preparation keeps every explicit per-service repo and
// branch and fills only what the entry omits, while staging the exact
// authored YAML (w8/m78).
func TestBlueprintSyncPreparationKeepsDeclaredServiceSource(t *testing.T) {
	for _, tc := range serviceSourceCases() {
		t.Run(tc.name, func(t *testing.T) {
			b := containingBlueprint()
			fs := newFakeBlueprintStore(b)
			svc := newBlueprintValidator(t)
			svc.Blueprints = fs
			ctx := core.WithWorkspace(context.Background(), b.TenantID)
			admitted, run, err := fs.AdmitBlueprintSyncRun(ctx, b.ID, b.TenantID, store.BlueprintSync{State: store.BlueprintSyncStateRunning, StartedAt: time.Now().UTC()})
			if err != nil {
				t.Fatal(err)
			}
			staged, parsed, err := svc.prepareSyncManifest(ctx, admitted, run, tc.manifest, svc.newWorkspaceSnapshot(ctx))
			if err != nil {
				t.Fatal(err)
			}
			if staged.Manifest != tc.manifest || len(parsed.services) != 1 {
				t.Fatalf("staged %q with %d services, want the exact authored manifest and one service", staged.Manifest, len(parsed.services))
			}
			if got := parsed.services[0].req; got.Repo != tc.repo || got.Branch != tc.branch {
				t.Errorf("prepared source = (%q, %q), want (%q, %q)", got.Repo, got.Branch, tc.repo, tc.branch)
			}
		})
	}
}

// A branch names a ref of one repository, so a different declared repo never
// borrows the Blueprint's branch; its omitted branch is that repository's own
// default (Render's rule), left here to the create/update branch default
// rather than certified (m76 owns the default-branch resolver).
func TestBlueprintContainingBranchIsNotLentToAnotherRepo(t *testing.T) {
	for _, manifest := range []string{
		sourceServiceManifest("    repo: " + declaredRepo + "\n"),
		nestSourceManifest(sourceServiceManifest("    repo: " + declaredRepo + "\n")),
	} {
		st, _, err := compileStack(DeployRequest{Repo: containingRepo, Branch: "canary", ContainingSource: true, Manifest: manifest})
		if err != nil {
			t.Fatal(err)
		}
		if got := st.services[0].req; got.Repo != declaredRepo || got.Branch != "" {
			t.Errorf("prepared source = (%q, %q), want (%q, \"\") with no borrowed branch", got.Repo, got.Branch, declaredRepo)
		}
	}
}

// A direct deploy's repo/branch remain the explicit target that overrides the
// file (the deploy-from-chat contract), and an image entry keeps its image in
// either context.
func TestDirectDeployOverrideAndImageSourceKeepTheirContracts(t *testing.T) {
	explicit := declaredSourceManifest
	st, _, err := compileStack(DeployRequest{Repo: containingRepo, Branch: containingBranch, Manifest: explicit})
	if err != nil {
		t.Fatal(err)
	}
	if got := st.services[0].req; got.Repo != containingRepo || got.Branch != containingBranch {
		t.Errorf("direct deploy source = (%q, %q), want the explicit target (%q, %q)", got.Repo, got.Branch, containingRepo, containingBranch)
	}
	image := "services:\n  - {name: own-image, type: web, runtime: image, plan: free, image: {url: 'docker.io/library/nginx:1.27'}}\n"
	for _, containing := range []bool{false, true} {
		st, _, err := compileStack(DeployRequest{Repo: containingRepo, Branch: containingBranch, ContainingSource: containing, Manifest: image})
		if err != nil {
			t.Fatal(err)
		}
		if got := st.services[0].req; got.Repo != "" || got.Branch != "" || got.Image != "docker.io/library/nginx:1.27" {
			t.Errorf("containing=%v image source = %+v", containing, got)
		}
	}
}

func ownImageBornApp() *appv1alpha1.App {
	a := blueprintIntentApp(true)
	a.Labels[core.LabelTenant] = "tea-a"
	a.Spec = imageBornSpec()
	return a
}

func ownSourceService(fs *fakeBlueprintStore, rec *recordingStore, apps ...*appv1alpha1.App) (*Service, context.Context) {
	// Nested declarations land in a project environment, so the store also
	// serves the grouping seams.
	groups := &blueprintGroupingTestStore{recordingStore: rec}
	svc, _ := newTenantStoreService(fakeWorkspace{"user-a": "tea-a"}, groups, apps...)
	svc.BlueprintGroups = groups
	svc.Environments = groups
	svc.Blueprints = fs
	svc.DomainOwnership = allowDomainOwnership{}
	return svc, core.WithIdentity(context.Background(), core.Identity{Subject: "user-a", Method: "oauth2"})
}

// Through the real provided-manifest sync, an own image-born service under a Blueprint for nodejs/node@main declares
// serve-static@master. The App, the canonical row and the first deploy take
// the declared source; an identical reapply opens no deploy (w8/m78).
func TestPersistentBlueprintSyncDeploysTheDeclaredServiceSource(t *testing.T) {
	for _, tc := range []struct{ name, manifest string }{
		{"root", declaredSourceManifest},
		{"nested", nestSourceManifest(declaredSourceManifest)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			existing := ownImageBornApp()
			appID := existing.Labels[core.LabelAppID]
			fs := newFakeBlueprintStore(containingBlueprint())
			rec := &recordingStore{}
			svc, ctx := ownSourceService(fs, rec, existing)

			if _, err := svc.SyncBlueprint(ctx, "blp-own", "tea-a", tc.manifest, "", nil); err != nil {
				t.Fatalf("SyncBlueprint: %v", err)
			}
			if got := fs.blueprints["blp-own"].Manifest; got != tc.manifest {
				t.Errorf("stored manifest = %q, want the exact authored YAML", got)
			}
			got := getApp(t, svc.Client, "own-source").Spec
			if got.Repo != declaredRepo || got.Branch != "master" || got.Image != "" {
				t.Errorf("App source = repo %q branch %q image %q, want %q master and no image", got.Repo, got.Branch, got.Image, declaredRepo)
			}
			wantWrite := struct{ id, repo, image, branch string }{appID, declaredRepo, "", "master"}
			if len(rec.sourceWrites) != 1 || rec.sourceWrites[0] != wantWrite {
				t.Errorf("canonical source writes = %+v, want %+v", rec.sourceWrites, wantWrite)
			}
			if len(rec.deployCalls) != 1 {
				t.Fatalf("deploys = %d, want one", len(rec.deployCalls))
			}

			if _, err := svc.SyncBlueprint(ctx, "blp-own", "tea-a", tc.manifest, "", nil); err != nil {
				t.Fatalf("identical SyncBlueprint: %v", err)
			}
			if len(rec.deployCalls) != 1 || len(rec.sourceWrites) != 1 {
				t.Errorf("identical reapply: %d deploys, %d source writes; want no new ones", len(rec.deployCalls), len(rec.sourceWrites))
			}
		})
	}
}

// A Git-backed create builds a new service from its declared source, not the
// repo the Blueprint file was fetched from (w8/m78).
func TestCreateBlueprintCreatesTheDeclaredServiceSource(t *testing.T) {
	for _, tc := range serviceSourceCases() {
		t.Run(tc.name, func(t *testing.T) {
			fs := newFakeBlueprintStore()
			svc, ctx := ownSourceService(fs, &recordingStore{})
			svc.GitFetcher = fakeBlueprintFetcher{contents: tc.manifest}
			if _, err := svc.CreateBlueprint(ctx, "tea-a", CreateBlueprintRequest{Repo: containingRepo, Branch: containingBranch}); err != nil {
				t.Fatalf("CreateBlueprint: %v", err)
			}
			list := &appv1alpha1.AppList{}
			if err := svc.Client.List(ctx, list); err != nil || len(list.Items) != 1 {
				t.Fatalf("apps = %d (%v), want one", len(list.Items), err)
			}
			if got := list.Items[0].Spec; got.Repo != tc.repo || got.Branch != tc.branch {
				t.Errorf("created source = (%q, %q), want (%q, %q)", got.Repo, got.Branch, tc.repo, tc.branch)
			}
		})
	}
}

// Preview plans what the apply does: a service already on its declared
// source is a no-op under a Blueprint for another repo, not a repo change.
func TestPreviewBlueprintPlansTheDeclaredServiceSource(t *testing.T) {
	existing := ownImageBornApp()
	existing.Spec = gitBornSpec()
	manifest := declaredSourceManifest
	svc, ctx := ownSourceService(newFakeBlueprintStore(), &recordingStore{}, existing)
	svc.GitFetcher = fakeBlueprintFetcher{contents: manifest}
	preview, err := svc.PreviewBlueprint(ctx, "tea-a", containingRepo, containingBranch, "", "")
	if err != nil || preview.Validation == nil || preview.Validation.Plan == nil {
		t.Fatalf("PreviewBlueprint: %+v %v", preview, err)
	}
	for _, action := range preview.Validation.Plan.Actions {
		for _, change := range action.ChangedFields {
			if change.Path == "repo" || change.Path == "branch" {
				t.Errorf("preview plans a %s change on a service already on its declared source: %+v", change.Path, action)
			}
		}
	}
}
