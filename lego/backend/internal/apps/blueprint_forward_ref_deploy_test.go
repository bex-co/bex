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
	"slices"
	"testing"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/306: a service taking the address of a sibling declared after it was
// written without the reference, then re-applied with it, and that re-apply
// opened a second, blueprint_sync deploy that canceled the create's first
// deploy (render-examples/movie-context-provider). The sibling is now created
// first, so the reference resolves in the service's first write.
func TestForwardHostReferenceOpensOneFirstDeploy(t *testing.T) {
	const manifest = `
services:
  - name: web
    type: web
    runtime: image
    image: {url: "web:1"}
    envVars:
      - {key: API_HOST, fromService: {name: api, type: web, property: host}}
  - {name: api, type: web, runtime: image, image: {url: "api:1"}}
`
	rec := &recordingStore{}
	svc, _ := connectionService(t)
	svc.Store = rec
	res, err := svc.DeployStack(ownershipCtx(), DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(rec.appCreates) != 2 || len(rec.deployCalls) != 0 {
		t.Fatalf("creates = %d, redeploys = %+v; want two creates and no redeploy", len(rec.appCreates), rec.deployCalls)
	}
	api := getTenantApp(t, svc.Client, connOwner, "api")
	if got, want := findEnv(t, getTenantApp(t, svc.Client, connOwner, "web").Spec.Env, "API_HOST").Value, api.Spec.PlatformSubdomain(api.Name); got != want {
		t.Fatalf("web API_HOST = %q, want api's slug %q", got, want)
	}
	if len(res.Services) != 2 || res.Services[0].Name != "web" || res.Services[1].Name != "api" {
		t.Errorf("result services = %+v, want them in file order", res.Services)
	}
}

// Two services that take each other's address cannot both be written after
// the other: the second write still resolves through the reference pass.
func TestMutualHostReferencesStillResolve(t *testing.T) {
	const manifest = `
services:
  - name: a
    type: web
    runtime: image
    image: {url: "a:1"}
    envVars:
      - {key: B_HOST, fromService: {name: b, type: web, property: host}}
  - name: b
    type: web
    runtime: image
    image: {url: "b:1"}
    envVars:
      - {key: A_HOST, fromService: {name: a, type: web, property: host}}
`
	rec := &recordingStore{}
	svc, _ := connectionService(t)
	svc.Store = rec
	if _, err := svc.DeployStack(ownershipCtx(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(rec.deployCalls) != 1 {
		t.Errorf("redeploys = %d, want 1: only the cycle's back edge takes the second pass", len(rec.deployCalls))
	}
	if findEnv(t, getTenantApp(t, svc.Client, connOwner, "a").Spec.Env, "B_HOST").Value == "" || findEnv(t, getTenantApp(t, svc.Client, connOwner, "b").Spec.Env, "A_HOST").Value == "" {
		t.Fatalf("a mutual reference was left unresolved")
	}
}

// A target that already exists resolves in any order, so a re-sync keeps file
// order: the apply's host-claim index lets a later service take a domain an
// earlier one gives up, which a reordering would refuse.
func TestReferencedFirstReordersOnlyForATargetThisApplyCreates(t *testing.T) {
	web := parsedService{req: CreateRequest{Name: "web"}, hostRefs: []hostRef{{key: "API_HOST", target: "api", property: serviceRefPropertyHost}}}
	api := parsedService{req: CreateRequest{Name: "api"}}
	names := func(services []parsedService) []string {
		out := make([]string, 0, len(services))
		for _, s := range services {
			out = append(out, s.req.Name)
		}
		return out
	}
	if got := names(referencedFirst([]parsedService{web, api}, nil)); !slices.Equal(got, []string{"api", "web"}) {
		t.Errorf("new target: order = %v, want [api web]", got)
	}
	existing := map[string]*appv1alpha1.App{"api": {}}
	if got := names(referencedFirst([]parsedService{web, api}, existing)); !slices.Equal(got, []string{"web", "api"}) {
		t.Errorf("existing target: order = %v, want file order [web api]", got)
	}
}
