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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/300: a sync's plan of an existing service probed only the file's literal
// env vars. References resolve at apply (resolveServiceRefs), so a sync that
// added a fromDatabase, Key Value or fromService host variable, or pointed one
// at another target, planned no envVars change and the approval applied it
// unlisted. render-examples/flue's sessionStore branch is the live case.

// refsManifest is a stack whose web service holds a literal plus refs, beside
// the databases db and db2, the service api, and any extra resources.
func refsManifest(refs, extraServices, extraDatabases string) string {
	return `
services:
  - name: web
    type: web
    runtime: image
    image: {url: nginx:1.27}
    envVars:
      - {key: APP_ENV, value: prod}` + refs + `
  - {name: api, type: web, runtime: image, image: {url: nginx:1.27}}` + extraServices + `
databases:
  - name: db
  - name: db2` + extraDatabases + "\n"
}

func dbRef(name string) string {
	return "\n      - {key: DATABASE_URL, fromDatabase: {name: " + name + ", property: connectionString}}"
}

func keyValueRef(name string) string {
	return "\n      - {key: CACHE_URL, fromService: {name: " + name + ", type: keyvalue, property: connectionString}}"
}

func hostRefTo(name string) string {
	return "\n      - {key: API_HOST, fromService: {name: " + name + ", type: web, property: host}}"
}

const (
	api2Service   = "\n  - {name: api2, type: web, runtime: image, image: {url: nginx:1.27}}"
	freshDatabase = "\n  - name: fresh"
)

// newRefsPlanService has applied deployed, beside two Key Values the file
// does not declare.
func newRefsPlanService(t *testing.T, deployed string) (*Service, context.Context) {
	t.Helper()
	svc, cl := newService(nil)
	ctx := context.Background()
	for _, name := range []string{"cache", "cache2"} {
		keyValue := &appv1alpha1.KeyValue{
			ObjectMeta: metav1.ObjectMeta{Name: "red-" + name, Namespace: "default"},
			Spec:       appv1alpha1.KeyValueSpec{Name: name},
		}
		if err := cl.Create(ctx, keyValue); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: deployed}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	return svc, ctx
}

func TestBlueprintPlanListsAReferenceItWillAddOrRetarget(t *testing.T) {
	cases := []struct {
		name, deployed, planned string
		created                 string // a resource the planned sync creates
	}{
		{name: "adds fromDatabase", deployed: refsManifest("", "", ""), planned: refsManifest(dbRef("db"), "", "")},
		{name: "adds a Key Value reference", deployed: refsManifest("", "", ""), planned: refsManifest(keyValueRef("cache"), "", "")},
		{name: "adds a host reference", deployed: refsManifest("", "", ""), planned: refsManifest(hostRefTo("api"), "", "")},
		// The slug is minted at create, so the apply's second pass writes it.
		{name: "adds a host reference to a service the sync creates", deployed: refsManifest("", "", ""), planned: refsManifest(hostRefTo("api2"), api2Service, ""), created: "api2"},
		{name: "points fromDatabase at another database", deployed: refsManifest(dbRef("db"), "", ""), planned: refsManifest(dbRef("db2"), "", "")},
		// flue's shape: the database's Secret is named only once the apply
		// mints its id.
		{name: "points fromDatabase at a database the sync creates", deployed: refsManifest(dbRef("db"), "", ""), planned: refsManifest(dbRef("fresh"), "", freshDatabase), created: "fresh"},
		{name: "points a Key Value reference at another Key Value", deployed: refsManifest(keyValueRef("cache"), "", ""), planned: refsManifest(keyValueRef("cache2"), "", "")},
		{name: "points a host reference at another service", deployed: refsManifest(hostRefTo("api"), api2Service, ""), planned: refsManifest(hostRefTo("api2"), api2Service, "")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctx := newRefsPlanService(t, tc.deployed)
			actions := planActionsForTest(ctx, t, svc, tc.planned)
			web := planAction(t, actions, "web")
			if web.Operation != BlueprintPlanUpdate || len(web.ChangedFields) != 1 || web.ChangedFields[0].Path != "envVars" {
				t.Fatalf("web plan = %+v, want an update of envVars alone", web)
			}
			if tc.created != "" {
				if created := planAction(t, actions, tc.created); created.Operation != BlueprintPlanCreate {
					t.Fatalf("%s plan = %+v, want a create", tc.created, created)
				}
			}
		})
	}
}

func TestBlueprintPlanOfAppliedReferencesIsNoChange(t *testing.T) {
	manifest := refsManifest(dbRef("db")+keyValueRef("cache")+hostRefTo("api"), "", "")
	svc, ctx := newRefsPlanService(t, manifest)
	if web := planAction(t, planActionsForTest(ctx, t, svc, manifest), "web"); web.Operation != BlueprintPlanNoop || len(web.ChangedFields) != 0 {
		t.Fatalf("re-plan of the applied file = %+v, want no change", web)
	}
}

// A reference to a database neither declared nor in the workspace is the
// apply's refusal; the plan says so rather than planning an envVars update.
func TestBlueprintPlanRefusesAReferenceToNothing(t *testing.T) {
	svc, ctx := newRefsPlanService(t, refsManifest("", "", ""))
	st, _, err := compileStack(DeployRequest{Manifest: refsManifest(dbRef("nowhere"), "", "")})
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := newBlueprintActionResolver(ctx, svc, st, svc.newWorkspaceSnapshot(ctx))
	if err != nil {
		t.Fatal(err)
	}
	web, ok := parsedBlueprintService(resolver.parsed, "web")
	if !ok {
		t.Fatal("web not parsed")
	}
	if _, _, err := resolver.withPlannedRefs(web); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("withPlannedRefs = %v, want the apply's ErrBadRequest", err)
	}
}
