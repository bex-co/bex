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
	"fmt"
	"slices"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/store"
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

// w8/316: two services that take each other's address cannot both be written
// after the other, so the cycle's back edge resolves from the sibling's
// predicted slug, and neither service is re-applied: each first deploy is its
// only one. Before, the back edge took the reference pass and a second deploy.
func TestMutualHostReferencesResolveInTheFirstWrite(t *testing.T) {
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
	if len(rec.appCreates) != 2 || len(rec.deployCalls) != 0 {
		t.Fatalf("creates = %d, redeploys = %+v; want two creates and no redeploy", len(rec.appCreates), rec.deployCalls)
	}
	if got := findEnv(t, getTenantApp(t, svc.Client, connOwner, "a").Spec.Env, "B_HOST").Value; got != "b" {
		t.Errorf("a's B_HOST = %q, want b's slug", got)
	}
	if got := findEnv(t, getTenantApp(t, svc.Client, connOwner, "b").Spec.Env, "A_HOST").Value; got != "a" {
		t.Errorf("b's A_HOST = %q, want a's slug", got)
	}
}

// w8/316: render-examples/ai-hedge-fund-on-render wires an API and a static
// site to each other's RENDER_EXTERNAL_URL. Both first writes carry the
// sibling's public URL, so the site's first build bakes the API's in.
func TestMutualExternalURLsResolveInTheFirstWrite(t *testing.T) {
	const manifest = `
services:
  - name: site
    type: web
    runtime: static
    repo: https://github.com/o/r
    staticPublishPath: dist
    envVars:
      - {key: VITE_API_URL, fromService: {name: api, type: web, envVarKey: RENDER_EXTERNAL_URL}}
  - name: api
    type: web
    runtime: image
    image: {url: "api:1"}
    envVars:
      - {key: FRONTEND_URL, fromService: {name: site, type: web, envVarKey: RENDER_EXTERNAL_URL}}
`
	rec := &recordingStore{}
	svc, _ := connectionService(t)
	svc.Store, svc.BaseDomain = rec, "onbex.test"
	if _, err := svc.DeployStack(ownershipCtx(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(rec.deployCalls) != 0 {
		t.Fatalf("redeploys = %+v, want none", rec.deployCalls)
	}
	if got := findEnv(t, getTenantApp(t, svc.Client, connOwner, "site").Spec.Env, "VITE_API_URL").Value; got != "https://api.onbex.test" {
		t.Errorf("site's VITE_API_URL = %q", got)
	}
	if got := findEnv(t, getTenantApp(t, svc.Client, connOwner, "api").Spec.Env, "FRONTEND_URL").Value; got != "https://site.onbex.test" {
		t.Errorf("api's FRONTEND_URL = %q", got)
	}
}

// suffixingStore mints a suffixed slug for one service, as store.CreateApp
// does when the bare name is taken.
type suffixingStore struct {
	*recordingStore
	name string
}

func (s suffixingStore) CreateApp(ctx context.Context, a store.App) (store.App, error) {
	if a.Name == s.name {
		a.Slug = a.Name + "-x7k2"
	}
	return s.recordingStore.CreateApp(ctx, a)
}

// w8/316: when the sibling's create mints another slug than the predicted bare
// name, the reference pass corrects the value with one redeploy, replacing the
// provisional variable rather than adding a second. RENDER_SERVICE_ID, the
// sibling's id, is unknown until its create and always takes that pass. In
// this cycle referencedFirst writes b first, so b's reference to a is the
// back edge.
func TestAMispredictedOrIDBackEdgeTakesTheReferencePass(t *testing.T) {
	for _, tc := range []struct {
		name, key, ref, want string
		suffixed             bool
	}{
		{"suffixed slug", "A_HOST", "property: host", "a-x7k2", true},
		{"service id", "A_ID", "envVarKey: RENDER_SERVICE_ID", "srv-test", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := fmt.Sprintf(`
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
      - {key: %s, fromService: {name: a, type: web, %s}}
`, tc.key, tc.ref)
			rec := &recordingStore{}
			svc, _ := connectionService(t)
			svc.Store = rec
			if tc.suffixed {
				svc.Store = suffixingStore{recordingStore: rec, name: "a"}
			}
			if _, err := svc.DeployStack(ownershipCtx(), DeployRequest{Manifest: manifest}); err != nil {
				t.Fatalf("DeployStack: %v", err)
			}
			if len(rec.deployCalls) != 1 {
				t.Fatalf("redeploys = %+v, want one for b", rec.deployCalls)
			}
			env := getTenantApp(t, svc.Client, connOwner, "b").Spec.Env
			if got := findEnv(t, env, tc.key).Value; got != tc.want {
				t.Errorf("b's %s = %q, want %q", tc.key, got, tc.want)
			}
			if n := len(slices.DeleteFunc(slices.Clone(env), func(e appv1alpha1.EnvVar) bool { return e.Name != tc.key })); n != 1 {
				t.Errorf("b carries %s %d times, want once", tc.key, n)
			}
		})
	}
}

// w8/316, ADR089 D4: a port back edge is never guessed. A Docker or image
// sibling learns its port only at runtime, so its created App has none yet; the
// variable stays out of the first write and the reference pass leaves it to the
// propagator, without a redeploy.
func TestAPortBackEdgeIsNeverGuessed(t *testing.T) {
	const manifest = `
services:
  - name: a
    type: web
    runtime: docker
    repo: https://github.com/o/r
    envVars:
      - {key: B_HOST, fromService: {name: b, type: pserv, property: host}}
  - name: b
    type: pserv
    runtime: docker
    repo: https://github.com/o/r
    envVars:
      - {key: A_HOSTPORT, fromService: {name: a, type: web, property: hostport}}
`
	rec := &recordingStore{}
	svc, _ := connectionService(t)
	svc.Store = rec
	if _, err := svc.DeployStack(ownershipCtx(), DeployRequest{Manifest: manifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	if len(rec.deployCalls) != 0 {
		t.Fatalf("redeploys = %+v, want none", rec.deployCalls)
	}
	for _, e := range getTenantApp(t, svc.Client, connOwner, "b").Spec.Env {
		if e.Name == "A_HOSTPORT" {
			t.Fatalf("b carries a guessed A_HOSTPORT = %q", e.Value)
		}
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
