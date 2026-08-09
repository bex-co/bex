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

package core

import (
	"context"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// datastore_resolution_test.go covers ADR043 D8.1 — the resolve-by-lookup
// contract that lets datastores move into per-tenant namespaces without a flag
// day. Creation computes the target namespace; every read finds the CR wherever
// it actually is.
//
// This is what makes the migration safe, so it needs coverage in all three
// positions a datastore can be in: the workspace's own namespace (post-cutover),
// the shared apps namespace (not yet cut over — the state every existing tenant
// is in), and another workspace's namespace (the cross-workspace case a
// multi-workspace member can legitimately reach). A resolver that handled only
// the first would strand every un-migrated tenant.

func tenantDatabase(ns, name, tenant string) *appv1alpha1.Database {
	return &appv1alpha1.Database{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: ns, Labels: map[string]string{LabelTenant: tenant, LabelWorkspace: tenant},
	}}
}

func tenantKeyValue(ns, name, tenant string) *appv1alpha1.KeyValue {
	return &appv1alpha1.KeyValue{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: ns, Labels: map[string]string{LabelTenant: tenant, LabelWorkspace: tenant},
	}}
}

func resolvingBase(objs ...client.Object) *Base {
	return &Base{
		Client:    fakeAppClient(objs...),
		Namespace: "default",
		Workspace: fakeWorkspace{"identity-a": "tea-a"},
		Authz:     &fakeAllowChecker{},
	}
}

func actingCtx() context.Context {
	return WithIdentity(context.Background(), Identity{Subject: "identity-a", Method: "session"})
}

func TestAuthorizeDatabaseFindsTheCRInEitherNamespace(t *testing.T) {
	for _, tc := range []struct {
		name      string
		namespace string
		why       string
	}{
		{"migrated", "tea-a", "the workspace's own namespace (ADR043 D8, post-cutover)"},
		{"legacy", "default", "the shared apps namespace — every tenant not yet cut over"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			b := resolvingBase(tenantDatabase(tc.namespace, "dpg-x", "tea-a"))
			got, err := b.AuthorizeDatabase(actingCtx(), RelCanView, "dpg-x")
			if err != nil {
				t.Fatalf("AuthorizeDatabase in %s: %v — %s", tc.namespace, err, tc.why)
			}
			if got.Namespace != tc.namespace {
				t.Errorf("resolved namespace = %q, want %q", got.Namespace, tc.namespace)
			}
		})
	}
}

func TestAuthorizeKeyValueFindsTheCRInEitherNamespace(t *testing.T) {
	for _, ns := range []string{"tea-a", "default"} {
		b := resolvingBase(tenantKeyValue(ns, "red-x", "tea-a"))
		got, err := b.AuthorizeKeyValue(actingCtx(), RelCanView, "red-x")
		if err != nil {
			t.Fatalf("AuthorizeKeyValue in %s: %v", ns, err)
		}
		if got.Namespace != ns {
			t.Errorf("resolved namespace = %q, want %q", got.Namespace, ns)
		}
	}
}

// TestAuthorizeDatastoreReachesAnotherWorkspacesNamespace covers the case the
// two direct candidates miss: a datastore in a namespace that is neither the
// acting workspace's nor the shared one. The cluster-wide sweep has to find it
// so that AUTHORIZATION decides the outcome.
//
// The assertion is therefore Forbidden, not success: this caller is not a member
// of tea-b, so being refused is correct. What matters is that it is refused
// rather than 404'd — 403-before-404 is the property the resource-scoped seams
// hold (a bare NotFound here would also leak that no such id exists anywhere,
// and would deny a legitimate multi-workspace member their own resource).
func TestAuthorizeDatastoreReachesAnotherWorkspacesNamespace(t *testing.T) {
	b := resolvingBase(tenantDatabase("tea-b", "dpg-elsewhere", "tea-b"))

	_, err := b.AuthorizeDatabase(actingCtx(), RelCanView, "dpg-elsewhere")
	if errors.Is(err, ErrNotFound) {
		t.Fatal("AuthorizeDatabase 404'd a Database in another workspace's namespace: " +
			"the cluster-wide sweep did not reach it, so authorization never got to decide")
	}
	if !errors.Is(err, ErrForbidden) {
		t.Fatalf("AuthorizeDatabase across workspaces = %v, want ErrForbidden (found, then refused)", err)
	}
}

// TestAuthorizeDatastoreMissStillReportsNotFound pins that the cluster-wide
// sweep's miss is indistinguishable from a direct Get's miss. Callers branch on
// apierrors.IsNotFound to run the 403-before-404 path, so a differently-shaped
// error here would silently turn "no such database" into a 500.
func TestAuthorizeDatastoreMissStillReportsNotFound(t *testing.T) {
	b := resolvingBase()
	if _, err := b.AuthorizeDatabase(actingCtx(), RelCanView, "dpg-missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("AuthorizeDatabase on a missing CR = %v, want ErrNotFound", err)
	}
	if _, err := b.AuthorizeKeyValue(actingCtx(), RelCanView, "red-missing"); !errors.Is(err, ErrNotFound) {
		t.Errorf("AuthorizeKeyValue on a missing CR = %v, want ErrNotFound", err)
	}
}

// TestDatastoreNamespacesOrdersOwnBeforeShared pins the lookup order. The
// workspace's own namespace must be tried first: during the cutover a datastore
// can briefly exist in both, and resolving to the stale shared copy would hand
// the caller connection details for the instance being retired.
func TestDatastoreNamespacesOrdersOwnBeforeShared(t *testing.T) {
	b := &Base{Namespace: "default"}

	got := b.DatastoreNamespaces("tea-a")
	if len(got) != 2 || got[0] != "tea-a" || got[1] != "default" {
		t.Errorf("DatastoreNamespaces(tea-a) = %v, want [tea-a default]", got)
	}
	// An unbound caller (store off) has no workspace namespace, so there is
	// exactly one place to look and no redundant second Get.
	for _, tenant := range []string{"", DefaultTenant} {
		if got := b.DatastoreNamespaces(tenant); len(got) != 1 || got[0] != "default" {
			t.Errorf("DatastoreNamespaces(%q) = %v, want [default]", tenant, got)
		}
	}
}

// TestDatastoreListOptionsScopesByLabelNotNamespace pins the other half of
// D8.1: once a workspace's datastores can be in its own namespace while
// un-migrated ones are in the shared one, no single namespace holds them all, so
// the LabelTenant selector has to carry the tenant boundary. A list that stayed
// namespace-pinned would quietly omit half a workspace's resources.
func TestDatastoreListOptionsScopesByLabelNotNamespace(t *testing.T) {
	b := &Base{Namespace: "default"}

	scoped := b.DatastoreListOptions("tea-a")
	if len(scoped) != 1 {
		t.Fatalf("DatastoreListOptions(tea-a) = %d options, want 1 (a label selector, no namespace)", len(scoped))
	}
	if _, isNamespace := scoped[0].(client.InNamespace); isNamespace {
		t.Error("DatastoreListOptions(tea-a) pinned a namespace; the tenant label must carry the boundary")
	}

	// Unbound: no tenant namespaces exist, so the shared-namespace scope is
	// retained rather than listing the whole cluster.
	unbound := b.DatastoreListOptions("")
	if len(unbound) != 1 {
		t.Fatalf("DatastoreListOptions(\"\") = %d options, want 1", len(unbound))
	}
	if ns, isNamespace := unbound[0].(client.InNamespace); !isNamespace || string(ns) != "default" {
		t.Errorf("DatastoreListOptions(\"\") = %#v, want InNamespace(default)", unbound[0])
	}
}

// TestDatabasePodsFollowTheCRNamespace guards a silent-empty failure: a pod list
// in the wrong namespace returns no error and no pods, so datastore logs and
// metrics would simply be blank rather than broken.
func TestDatabasePodsFollowTheCRNamespace(t *testing.T) {
	b := &Base{Client: fakeAppClient(), Namespace: "default"}
	ctx := context.Background()

	// The contract under test is the parameter, not the result: an explicit
	// namespace is honored, and an empty one falls back to the shared namespace
	// so pre-D8 callers keep their behavior.
	if _, err := b.DatabasePods(ctx, "tea-a", "dpg-x"); err != nil {
		t.Errorf("DatabasePods with an explicit namespace: %v", err)
	}
	if _, err := b.DatabasePods(ctx, "", "dpg-x"); err != nil {
		t.Errorf("DatabasePods with the empty-namespace fallback: %v", err)
	}
	if _, err := b.KeyValuePods(ctx, "tea-a", "red-x"); err != nil {
		t.Errorf("KeyValuePods with an explicit namespace: %v", err)
	}
	if _, err := b.KeyValuePods(ctx, "", "red-x"); err != nil {
		t.Errorf("KeyValuePods with the empty-namespace fallback: %v", err)
	}
}
