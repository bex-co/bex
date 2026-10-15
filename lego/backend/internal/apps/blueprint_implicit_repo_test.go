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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// implicitSourceServices are sweep 318's minimal Git-backed services, one per
// build kind, with no repo: Render takes the repository containing the
// Blueprint. image is the prebuilt-image control kind.
var implicitSourceServices = map[string]string{
	"docker": "- type: web\n  name: qa318-docker\n  runtime: docker\n  plan: free\n",
	"node":   "- type: web\n  name: qa318-node\n  runtime: node\n  plan: free\n  buildCommand: node --version\n  startCommand: node server.js\n",
	"static": "- type: web\n  name: qa318-static\n  runtime: static\n  buildCommand: mkdir -p public\n  staticPublishPath: public\n",
	"image":  "- type: web\n  name: qa318-image\n  runtime: image\n  plan: free\n",
}

// sourceManifest is one implicitSourceServices entry plus extra fields, at
// the root or nested in a project environment.
func sourceManifest(kind, extra string, nested bool) string {
	service := implicitSourceServices[kind] + extra
	if !nested {
		return "services:\n" + service
	}
	indented := strings.ReplaceAll(strings.TrimRight(service, "\n"), "\n", "\n        ")
	return "projects:\n  - name: qa318-project\n    environments:\n      - name: production\n        services:\n        " + indented + "\n"
}

// workspaceValidator is a validator scoped to connOwner's workspace, as the
// CLI's multipart validate reaches it, which already holds apps.
func workspaceValidator(t *testing.T, apps ...*appv1alpha1.App) *Service {
	t.Helper()
	svc := newBlueprintValidator(t)
	svc.Workspace = fakeWorkspace{"id-a": connOwner}
	for _, app := range apps {
		if err := svc.Client.Create(ownershipCtx(), app); err != nil {
			t.Fatal(err)
		}
	}
	return svc
}

// TestValidateBlueprintAcceptsAnImplicitRepository (w8/m75, sweep 318): a
// Git-backed service that omits repo takes the repository containing the
// Blueprint. A manifest-only validation (the CLI uploads only ownerId and the
// file) does not know that repository, which is unresolved context rather
// than a missing source: it used to be refused with create's "one of repo or
// image is required". Authored source problems stay refusals.
func TestValidateBlueprintAcceptsAnImplicitRepository(t *testing.T) {
	const repo = "  repo: https://github.com/expressjs/serve-static\n"
	for _, tc := range []struct {
		name, kind, extra string
		want              string // "" = valid; else the refusal's message fragment
	}{
		{name: "implicit docker", kind: "docker"},
		{name: "implicit node", kind: "node"},
		{name: "implicit static", kind: "static"},
		{name: "explicit docker", kind: "docker", extra: repo},
		{name: "explicit node", kind: "node", extra: repo},
		{name: "explicit static", kind: "static", extra: repo},
		{name: "prebuilt image", kind: "image", extra: "  image:\n    url: nginx:1.27\n"},
		{name: "invalid repo", kind: "node", extra: "  repo: not-a-repository\n", want: "repo must be an https/ssh/git URL"},
		{name: "invalid branch", kind: "node", extra: "  branch: \"main;rm -rf\"\n", want: "branch must be a git ref"},
		{name: "missing image", kind: "image", want: "one of repo or image is required"},
	} {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nested=%v", tc.name, nested), func(t *testing.T) {
				manifest := sourceManifest(tc.kind, tc.extra, nested)
				v, err := workspaceValidator(t).ValidateBlueprint(ownershipCtx(), connOwner, manifest, "")
				if err != nil {
					t.Fatal(err)
				}
				if tc.want == "" {
					if !v.Valid {
						t.Fatalf("errors = %+v, want valid:\n%s", v.Errors, manifest)
					}
					if tc.extra == "" && v.Plan == nil {
						t.Fatal("valid validation has no plan")
					}
					return
				}
				if v.Valid || len(v.Errors) != 1 || !strings.Contains(v.Errors[0].Error, tc.want) {
					t.Fatalf("validation = %+v, want one refusal containing %q", v, tc.want)
				}
			})
		}
	}
}

// The validation reports the plan of what it could resolve; it never invents
// the enclosing repository into the response.
func TestValidateBlueprintImplicitRepositoryInventsNoSource(t *testing.T) {
	for kind := range implicitSourceServices {
		if kind == "image" {
			continue
		}
		v, err := workspaceValidator(t).ValidateBlueprint(ownershipCtx(), connOwner, sourceManifest(kind, "", false), "")
		if err != nil || !v.Valid {
			t.Fatalf("%s: %+v, %v", kind, v, err)
		}
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), "://") {
			t.Fatalf("%s validation carries a source URL: %s", kind, raw)
		}
	}
}

// The CLI's multipart upload, GraphQL and MCP share the decision: valid for
// the implicit repository, the same located refusal for a missing image.
func TestImplicitRepositoryAgreesAcrossValidationSurfaces(t *testing.T) {
	validate := validationSurfaces(t, workspaceValidator(t))
	for _, tc := range []struct {
		kind  string
		valid bool
	}{{"node", true}, {"static", true}, {"image", false}} {
		t.Run(tc.kind, func(t *testing.T) {
			direct, surfaces := validate(t, sourceManifest(tc.kind, "", true))
			if direct.Valid != tc.valid || len(direct.Errors) != map[bool]int{true: 0, false: 1}[tc.valid] {
				t.Fatalf("direct = %+v, want valid %v", direct, tc.valid)
			}
			for surface, got := range surfaces {
				if fmt.Sprint(got) != fmt.Sprint(direct) {
					t.Errorf("%s = %+v, want %+v", surface, got, direct)
				}
			}
		})
	}
}

// renderExample is a checked-in unchanged render-examples Blueprint.
func renderExample(t *testing.T, name string) string {
	t.Helper()
	manifest, err := os.ReadFile("testdata/render-" + name + "/render.yaml")
	if err != nil {
		t.Fatal(err)
	}
	return string(manifest)
}

// The unchanged Ghost and WordPress examples (and Mattermost once its 1GB
// service disk meets Bex's 10GB floor) validate standalone; the original
// Mattermost receives only that documented floor, which the source refusal
// used to mask (ADR082).
func TestValidateBlueprintAcceptsUpstreamExamplesWithoutRepository(t *testing.T) {
	mattermost := renderExample(t, "mattermost")
	floor := strings.Replace(mattermost, "    mountPath: /mattermost/config\n    sizeGB: 1\n", "    mountPath: /mattermost/config\n    sizeGB: 10\n", 1)
	if floor == mattermost {
		t.Fatal("the Mattermost service disk declaration moved")
	}
	for name, manifest := range map[string]string{
		"ghost":                  renderExample(t, "ghost"),
		"wordpress":              renderExample(t, "wordpress"),
		"mattermost-volume-10GB": floor,
	} {
		t.Run(name, func(t *testing.T) {
			v, err := workspaceValidator(t).ValidateBlueprint(ownershipCtx(), connOwner, manifest, "")
			if err != nil || !v.Valid {
				t.Fatalf("validation = %+v, %v; want valid", v.Errors, err)
			}
		})
	}
	t.Run("mattermost", func(t *testing.T) {
		v, err := workspaceValidator(t).ValidateBlueprint(ownershipCtx(), connOwner, mattermost, "")
		if err != nil {
			t.Fatal(err)
		}
		if v.Valid || len(v.Errors) != 1 || !strings.Contains(v.Errors[0].Error, "sizeGB must be at least 10") {
			t.Fatalf("validation = %+v, want only the documented disk floor", v.Errors)
		}
	})
}

// Creation still needs the source: a direct apply of the same manifests has no
// enclosing repository and refuses before writing anything, and a known
// repository (preview, Blueprint create/sync) supplies it.
func TestImplicitRepositoryStaysRequiredToApply(t *testing.T) {
	for _, kind := range []string{"docker", "node", "static"} {
		for _, nested := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/nested=%v", kind, nested), func(t *testing.T) {
				svc := workspaceValidator(t)
				_, err := svc.DeployStack(ownershipCtx(), DeployRequest{OwnerID: connOwner, Manifest: sourceManifest(kind, "", nested)})
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "one of repo or image is required") {
					t.Fatalf("DeployStack = %v, want the source requirement", err)
				}
				var apps appv1alpha1.AppList
				if err := svc.Client.List(ownershipCtx(), &apps); err != nil || len(apps.Items) != 0 {
					t.Fatalf("refused apply left %d Apps (%v)", len(apps.Items), err)
				}
			})
		}
	}
	t.Run("known repository", func(t *testing.T) {
		svc := workspaceValidator(t)
		ctx := core.WithWorkspace(ownershipCtx(), connOwner)
		for name, manifest := range map[string]string{"ghost": renderExample(t, "ghost"), "node": sourceManifest("node", "", false)} {
			v, err := svc.blueprintValidationFor(ctx, "https://github.com/render-examples/"+name, "master", manifest, "")
			if err != nil || !v.Valid {
				t.Fatalf("%s with its repository = %+v, %v", name, v.Errors, err)
			}
		}
		if _, err := svc.DeployStack(ownershipCtx(), DeployRequest{OwnerID: connOwner, Repo: "https://github.com/acme/app", Manifest: sourceManifest("node", "", false)}); err != nil {
			t.Fatalf("DeployStack with a repository: %v", err)
		}
		var apps appv1alpha1.AppList
		if err := svc.Client.List(ownershipCtx(), &apps); err != nil || len(apps.Items) != 1 {
			t.Fatalf("apps = %d (%v), want the one created", len(apps.Items), err)
		}
		if app := apps.Items[0]; app.Spec.Repo != "https://github.com/acme/app" || app.Spec.Branch != appv1alpha1.DefaultBranch || !app.Spec.AutoDeploy {
			t.Fatalf("created source = repo %q branch %q autoDeploy %v", app.Spec.Repo, app.Spec.Branch, app.Spec.AutoDeploy)
		}
	})
}

// An existing service's plan keeps its current source when the file omits
// it: the unresolved repository is no source change.
func TestValidateBlueprintImplicitRepositoryPlansExistingService(t *testing.T) {
	existing := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: core.CRName(connOwner, "qa318-node"), Namespace: "default",
			Labels: map[string]string{core.LabelTenant: connOwner, core.LabelServiceName: "qa318-node"}},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Repo: "https://github.com/acme/app", Branch: "main", Runtime: "node",
			BuildCommand: "node --version", StartCommand: "node server.js", Tier: "free", Port: 3000, Replicas: 1, AutoDeploy: true},
	}
	svc := workspaceValidator(t, existing)
	v, err := svc.ValidateBlueprint(ownershipCtx(), connOwner, sourceManifest("node", "", false), "")
	if err != nil || !v.Valid || v.Plan == nil {
		t.Fatalf("validation = %+v, %v", v, err)
	}
	if v.Plan.Mode != "current_state" || len(v.Plan.Actions) != 1 || v.Plan.Actions[0].Operation == BlueprintPlanCreate {
		t.Fatalf("plan = %+v, want the existing service planned against its current state", v.Plan)
	}
	for _, action := range v.Plan.Actions {
		for _, change := range action.ChangedFields {
			if change.Path == "repo" || change.Path == "branch" || change.Path == "image" {
				t.Fatalf("plan changes the existing source: %+v", action)
			}
		}
	}
}
