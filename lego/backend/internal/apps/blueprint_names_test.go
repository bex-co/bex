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

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestBlueprintResourceSlugIsStableValidAndKeepsResourceNames(t *testing.T) {
	long1 := "gitpod-openvscode-server-example"
	long2 := "gitpod-openvscode-server-example-two"
	for in, want := range map[string]string{
		"api":       "api",
		"pgAdmin":   "pgadmin",
		"AlphaClaw": "alphaclaw",
		"My App!":   "my-app",
		"Ünï Form!": "uni-form",
	} {
		if got := blueprintResourceSlug(in); got != want {
			t.Errorf("blueprintResourceSlug(%q) = %q, want %q", in, got, want)
		}
	}
	a, b := blueprintResourceSlug(long1), blueprintResourceSlug(long2)
	if !appv1alpha1.ValidResourceName(a) || !appv1alpha1.ValidResourceName(b) || a == b {
		t.Errorf("long names must map to distinct valid resource names: %q, %q", a, b)
	}
	if blueprintResourceSlug(long1) != a {
		t.Error("the mapping must be stable across syncs")
	}
	if blueprintDisplayName("api") != "" || blueprintDisplayName("pgAdmin") != "pgAdmin" {
		t.Error("only a name that is not already a resource name keeps a display name")
	}
}

// Render's pgadmin template names its service "pgAdmin" and a sibling can
// reference it by that name; bex deploys it as the resource "pgadmin" labeled
// "pgAdmin", and a re-sync updates the same service (w5/m150).
func TestDeployStackDeploysFreeFormNamesUnderTheirLabel(t *testing.T) {
	manifest := `
services:
  - name: pgAdmin
    type: web
    runtime: image
    image: {url: dpage/pgadmin4}
    envVars:
      - {key: discovery.type, value: single-node}
  - name: frontend
    type: web
    runtime: image
    image: {url: nginx}
    envVars:
      - {key: ADMIN_HOST, fromService: {name: pgAdmin, type: web, property: host}}
`
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
	for range 2 {
		if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest}); err != nil {
			t.Fatalf("DeployStack: %v", err)
		}
	}
	admin := getTenantApp(t, cl, "tea-a", "pgadmin")
	if admin.Spec.DisplayName != "pgAdmin" {
		t.Errorf("display name = %q, want the Blueprint's pgAdmin", admin.Spec.DisplayName)
	}
	if got := findEnv(t, admin.Spec.Env, "discovery.type").Value; got != "single-node" {
		t.Errorf("dotted key = %q, want single-node", got)
	}
	frontend := getTenantApp(t, cl, "tea-a", "frontend")
	if findEnv(t, frontend.Spec.Env, "ADMIN_HOST").Value == "" {
		t.Error("a reference by the free-form name must resolve")
	}
	var apps appv1alpha1.AppList
	if err := cl.List(ctx, &apps); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 2 {
		t.Errorf("a re-sync created duplicates: %d Apps, want 2", len(apps.Items))
	}
}

// Two Blueprint names that map onto one resource name are a duplicate, caught
// where every duplicate is, rather than one service applied twice.
func TestBlueprintNamesThatShareAResourceNameAreDuplicates(t *testing.T) {
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	manifest := `services:
  - {name: My API, type: web, runtime: image, image: {url: nginx}}
  - {name: my-api, type: web, runtime: image, image: {url: nginx}}
`
	v, err := svc.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || len(v.Errors) == 0 || v.Errors[0].Code != "BLUEPRINT_DUPLICATE_RESOURCE" {
		t.Fatalf("validation = %+v, want BLUEPRINT_DUPLICATE_RESOURCE", v.Errors)
	}
}

// Render's create takes a free-form name (w5/m153): bex creates the resource
// under its slug and keeps the name as the label; a dry run shows the label.
func TestCreateMapsAFreeFormNameOntoASlugAndKeepsItAsTheLabel(t *testing.T) {
	svc, cl := newService(nil)
	v, err := svc.Create(context.Background(), CreateRequest{Name: "My API", Image: "nginx:1"})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if v.DisplayName != "My API" {
		t.Errorf("view display name = %q, want My API", v.DisplayName)
	}
	if a := getApp(t, cl, "my-api"); a.Spec.DisplayName != "My API" {
		t.Errorf("spec.displayName = %q, want My API", a.Spec.DisplayName)
	}
	dry, err := svc.Create(context.Background(), CreateRequest{Name: "Other App", Image: "nginx:1", DryRun: true})
	if err != nil || dry.DisplayName != "Other App" || dry.Name != "other-app" {
		t.Fatalf("dry run = %+v, %v; want other-app labeled Other App", dry, err)
	}
}

// w4/385: a name with nothing in a–z or 0–9 no longer collapses onto one fixed
// resource name, so a workspace can hold several.
func TestBlueprintResourceSlugGivesNonLatinNamesDistinctStableNames(t *testing.T) {
	names := []string{"日本語のみ", "中文名", "🚀🚀", "__"}
	seen := map[string]string{}
	for _, name := range names {
		slug := blueprintResourceSlug(name)
		if !appv1alpha1.ValidResourceName(slug) || !strings.HasPrefix(slug, "resource-") {
			t.Errorf("blueprintResourceSlug(%q) = %q, want a valid resource-<hash> name", name, slug)
		}
		if other, dup := seen[slug]; dup {
			t.Errorf("%q and %q both map to %q", name, other, slug)
		}
		seen[slug] = name
		if blueprintResourceSlug(name) != slug {
			t.Errorf("blueprintResourceSlug(%q) is not stable", name)
		}
	}
	if got := legacyBlueprintResourceSlug("中文名"); got != "resource" {
		t.Errorf("legacy mapping = %q, want resource", got)
	}
}

// A service created before the fix is labelled "resource" with its display
// name; the plan must still find it under the name the manifest now maps to.
func TestSnapshotFindsAServiceCreatedUnderTheLegacyName(t *testing.T) {
	legacy := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{DisplayName: "中文名"}}
	accent := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{DisplayName: "Ünï Form"}}
	other := &appv1alpha1.App{}
	byName := map[string]*appv1alpha1.App{"resource": legacy, "n-form": accent, "api": other}
	aliasLegacyResourceNames(byName)
	if byName[blueprintResourceSlug("中文名")] != legacy || byName[blueprintResourceSlug("Ünï Form")] != accent {
		t.Fatalf("legacy services are not reachable under their new names: %v", byName)
	}
	if byName["api"] != other || len(byName) != 5 {
		t.Fatalf("unrelated entries changed: %v", byName)
	}
}

func TestCreateKeepsSeveralNonLatinNamesAndNamesTheTypedNameOnAConflict(t *testing.T) {
	svc, _ := newService(nil)
	ctx := context.Background()
	for _, name := range []string{"日本語のみ", "中文名"} {
		if _, err := svc.Create(ctx, CreateRequest{Name: name, Image: "nginx:1"}); err != nil {
			t.Fatalf("Create(%q): %v", name, err)
		}
	}
	_, err := svc.Create(ctx, CreateRequest{Name: "中文名", Image: "nginx:1"})
	if err == nil || !strings.Contains(err.Error(), `"中文名"`) || !strings.Contains(err.Error(), "maps to resource name") {
		t.Fatalf("duplicate create error = %v, want one naming the typed name and its resource name", err)
	}
}
