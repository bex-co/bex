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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// datastore_namespace_test.go is w7/m77/t002's regression harness for the
// 2026-08-08 production defect: ADR043 D1 says App / Database / KeyValue all
// land in the workspace's `<ws>` namespace, but only the App half was
// implemented, so every fromDatabase/fromService link created after the fleet
// migration was broken (ADR043 D8).
//
// The incident's defining property was that each leg only became visible after
// the previous one was fixed by hand, so a single end-to-end assertion would
// have hidden two of three. Each leg therefore gets its own test that fails for
// its own reason:
//
//	leg 1  TestBlueprintDatastoresLandInTheAppsOwnNamespace
//	       a secretKeyRef is same-namespace-only, so a datastore Secret outside
//	       the App's namespace can never resolve => CreateContainerConfigError.
//	leg 3  TestInjectedDatastoreHostIsResolvableFromTheApp
//	       CNPG's `host` key holds a bare Service name, resolvable only
//	       in-namespace => "could not translate host name".
//
// Leg 2 (the tenant default-deny denies egress to a datastore ClusterIP,
// because every in-cluster allow is same-namespace) has no independent
// unit-testable failure at this layer: it is a consequence of placement, not a
// separate code path. Once leg 1 holds, `allow-same-namespace` covers it. It is
// guarded instead by TestHostingAllowSetMakesCoLocatedDatastoresReachable in
// internal/store, which pins that co-location remains sufficient, and proven at
// runtime by w7/m77/t013.

// linkedStackManifest is the incident's shape reduced to essentials: one web
// service wired to a managed Postgres and a managed Key Value by reference.
const linkedStackManifest = `
services:
  - name: forumkv
    type: redis
    ipAllowList: []
    plan: free
  - name: forum
    type: web
    runtime: image
    image: {url: forum:1}
    envVars:
      - key: DATABASE_URL
        fromDatabase: {name: forumdb, property: connectionString}
      - key: DATABASE_HOST
        fromDatabase: {name: forumdb, property: host}
      - key: REDIS_HOST
        fromService: {name: forumkv, type: redis, property: host}
databases:
  - name: forumdb
`

// deployLinkedStack applies linkedStackManifest as a tenant-bound caller — the
// only mode that reproduces the defect, since an unbound caller has no
// workspace namespace and everything collapses into the shared one.
func deployLinkedStack(t *testing.T) (client.Client, string) {
	t.Helper()
	const tenantID = "tea-forumtest"
	cl := fakeClient()
	svc := &Service{Base: &core.Base{
		Client:    cl,
		Namespace: "default",
		Workspace: fakeWorkspace{"id-a": tenantID},
	}}
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"})

	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: linkedStackManifest}); err != nil {
		t.Fatalf("DeployStack: %v", err)
	}
	return cl, tenantID
}

// oneApp / oneDatabase / oneKeyValue read back the single CR of each kind the
// stack created, cluster-wide — deliberately NOT scoped to a namespace, so the
// test observes where each landed instead of assuming it.
func oneApp(t *testing.T, cl client.Client) *appv1alpha1.App {
	t.Helper()
	var list appv1alpha1.AppList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatalf("list apps: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("got %d Apps, want 1", len(list.Items))
	}
	return &list.Items[0]
}

func oneDatabase(t *testing.T, cl client.Client) *appv1alpha1.Database {
	t.Helper()
	var list appv1alpha1.DatabaseList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatalf("list databases: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("got %d Databases, want 1", len(list.Items))
	}
	return &list.Items[0]
}

func oneKeyValue(t *testing.T, cl client.Client) *appv1alpha1.KeyValue {
	t.Helper()
	var list appv1alpha1.KeyValueList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatalf("list keyvalues: %v", err)
	}
	if len(list.Items) != 1 {
		t.Fatalf("got %d KeyValues, want 1", len(list.Items))
	}
	return &list.Items[0]
}

// TestBlueprintDatastoresLandInTheAppsOwnNamespace pins leg 1.
//
// It asserts the RELATIONSHIP (datastore namespace == App namespace), never a
// namespace literal — a test that pinned "tea-forumtest" would keep passing if
// a future change moved both to some third namespace, which would break links
// just as thoroughly.
func TestBlueprintDatastoresLandInTheAppsOwnNamespace(t *testing.T) {
	cl, _ := deployLinkedStack(t)
	app := oneApp(t, cl)
	db := oneDatabase(t, cl)
	kv := oneKeyValue(t, cl)

	// A secretKeyRef resolves only within the consuming pod's own namespace, so
	// this equality is exactly the condition under which the injected env can
	// resolve at all (ADR043 D8 leg 1).
	if db.Namespace != app.Namespace {
		t.Errorf("Database landed in %q but its App is in %q: a secretKeyRef cannot cross namespaces, so the pod fails CreateContainerConfigError: secret %q not found",
			db.Namespace, app.Namespace, db.Name+"-app")
	}
	if kv.Namespace != app.Namespace {
		t.Errorf("KeyValue landed in %q but its App is in %q: same cross-namespace secretKeyRef failure, for secret %q",
			kv.Namespace, app.Namespace, kv.Name)
	}
}

// TestEveryDatastoreSecretRefIsResolvableFromTheApp pins leg 1 at the level the
// pod actually experiences: it walks the env the Blueprint injected and checks
// each secretKeyRef against the namespace of the CR that owns that Secret.
//
// This is the assertion that would have caught the defect at review time —
// the App CR looked correct in isolation (the refs were present and
// well-formed), and only the pairing with the owning CR's namespace reveals
// that nothing can resolve them.
func TestEveryDatastoreSecretRefIsResolvableFromTheApp(t *testing.T) {
	cl, _ := deployLinkedStack(t)
	app := oneApp(t, cl)
	db := oneDatabase(t, cl)
	kv := oneKeyValue(t, cl)

	// secretName -> the namespace the owning CR puts that Secret in.
	owner := map[string]string{
		db.Name + "-app":        db.Namespace,
		db.Name + "-pooler-app": db.Namespace,
		kv.Name:                 kv.Namespace,
	}

	refs := 0
	for _, e := range app.Spec.Env {
		if e.ValueFrom == nil || e.ValueFrom.SecretKeyRef == nil {
			continue
		}
		secretName := e.ValueFrom.SecretKeyRef.Name
		ns, known := owner[secretName]
		if !known {
			continue // an env-group / user Secret, not a datastore link
		}
		refs++
		if ns != app.Namespace {
			t.Errorf("env %q references Secret %q, which lives in namespace %q while the App runs in %q — unresolvable",
				e.Name, secretName, ns, app.Namespace)
		}
	}
	// Guard the guard: if the manifest ever stops producing datastore refs, the
	// loop above would vacuously pass and this test would silently stop testing.
	if refs == 0 {
		t.Fatal("no datastore secretKeyRefs found in the deployed App — the fixture stopped exercising the linked path")
	}
}

// TestInjectedDatastoreHostIsResolvableFromTheApp pins leg 3.
//
// CNPG writes a BARE Service name into the `host` key, and the Valkey
// reconciler does the same. A bare name resolves through the pod's search
// domains only when the consumer sits in the same namespace; a fully-qualified
// name resolves from anywhere. So the injected value is correct iff it is
// qualified OR the datastore is co-located — which is what this asserts,
// rather than asserting one specific spelling.
func TestInjectedDatastoreHostIsResolvableFromTheApp(t *testing.T) {
	cl, _ := deployLinkedStack(t)
	app := oneApp(t, cl)
	db := oneDatabase(t, cl)
	kv := oneKeyValue(t, cl)

	for _, tc := range []struct {
		what      string
		host      string
		namespace string
	}{
		{"Postgres", db.Name + "-rw", db.Namespace},
		{"Key Value", kv.Name, kv.Namespace},
	} {
		if !hostResolvableFrom(tc.host, tc.namespace, app.Namespace) {
			t.Errorf("%s host %q is a bare Service name in namespace %q, but the App runs in %q: DNS gives PG::ConnectionBad / could not translate host name",
				tc.what, tc.host, tc.namespace, app.Namespace)
		}
	}
}

// hostResolvableFrom reports whether host, a Service name published in
// serviceNamespace, resolves from a pod running in fromNamespace. A qualified
// name (any dot) resolves cluster-wide; a bare name relies on the pod's
// namespace search domain and so requires co-location.
func hostResolvableFrom(host, serviceNamespace, fromNamespace string) bool {
	if strings.Contains(host, ".") {
		return true
	}
	return serviceNamespace == fromNamespace
}
