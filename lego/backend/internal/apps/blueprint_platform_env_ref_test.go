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
)

// Render's templates copy a sibling's platform variables with
// fromService.envVarKey (a frontend reading its API's RENDER_EXTERNAL_URL).
// bex now resolves them from the sibling's App (w5/m148).
func TestDeployStackCopiesASiblingsRenderPlatformVariables(t *testing.T) {
	manifest := `
services:
  - name: frontend
    type: web
    runtime: image
    image: {url: nginx}
    envVars:
      - {key: API_URL, fromService: {name: api, type: web, envVarKey: RENDER_EXTERNAL_URL}}
      - {key: API_NAME, fromService: {name: api, type: web, envVarKey: RENDER_SERVICE_NAME}}
      - {key: API_DISCOVERY, fromService: {name: api, type: web, envVarKey: RENDER_DISCOVERY_SERVICE}}
      - {key: API_OWN, fromService: {name: api, type: web, envVarKey: RENDER_CUSTOM}}
  - name: api
    type: web
    runtime: image
    image: {url: nginx}
    envVars:
      - {key: RENDER_CUSTOM, value: declared}
`
	svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	svc.BaseDomain = "onbex.test"
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
	res, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	var apiSlug string
	for _, v := range res.Services {
		if v.Name == "api" {
			apiSlug = v.Slug
		}
	}
	frontend := getTenantApp(t, cl, "tea-a", "frontend")
	for key, want := range map[string]string{
		"API_URL":       "https://" + apiSlug + ".onbex.test",
		"API_NAME":      apiSlug,
		"API_DISCOVERY": apiSlug + "-discovery",
		"API_OWN":       "declared",
	} {
		if got := findEnv(t, frontend.Spec.Env, key).Value; got != want {
			t.Errorf("%s = %q, want %q", key, got, want)
		}
	}
}

func TestDeployStackRefusesCopyingAPerDeployPlatformVariable(t *testing.T) {
	manifest := `
services:
  - name: frontend
    type: web
    runtime: image
    image: {url: nginx}
    envVars:
      - {key: API_SHA, fromService: {name: api, type: web, envVarKey: RENDER_GIT_COMMIT}}
  - {name: api, type: web, runtime: image, image: {url: nginx}}
`
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
	_, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest})
	if err == nil || !strings.Contains(err.Error(), "differs per deploy or instance") {
		t.Fatalf("DeployStack error = %v, want the per-deploy refusal", err)
	}
}

func TestDeployStackRefusesCopyingAVariableTheSiblingsTypeLacks(t *testing.T) {
	manifest := `
services:
  - name: frontend
    type: web
    runtime: image
    image: {url: nginx}
    envVars:
      - {key: API_URL, fromService: {name: api, type: pserv, envVarKey: RENDER_EXTERNAL_URL}}
  - {name: api, type: pserv, runtime: image, image: {url: nginx}}
`
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "identity-a", Method: "session"})
	_, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: manifest})
	if err == nil || !strings.Contains(err.Error(), "does not have") {
		t.Fatalf("DeployStack error = %v, want a named refusal: a private service has no external URL", err)
	}
}
