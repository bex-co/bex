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

package store

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func newTestNamespaceReconciler(t *testing.T, sandboxes bool) (*NamespaceReconciler, *memStore, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	store := newMemStore()
	r := NewNamespaceReconciler(cl, store)
	r.Sandboxes = sandboxes
	return r, store, cl
}

func TestReconcileProvisionsHostingNamespaceWithBaseObjects(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, false)
	tn, err := store.CreateTenant(ctx, "acme", "free")
	if err != nil {
		t.Fatal(err)
	}
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	nsName := WorkspaceNamespace(tn.ID)
	var ns corev1.Namespace
	if err := cl.Get(ctx, client.ObjectKey{Name: nsName}, &ns); err != nil {
		t.Fatalf("hosting namespace not created: %v", err)
	}
	if ns.Labels[core.LabelWorkspace] != tn.ID {
		t.Errorf("workspace label = %q, want %q", ns.Labels[core.LabelWorkspace], tn.ID)
	}
	if ns.Labels[RegimeLabel] != RegimeHosting {
		t.Errorf("regime label = %q, want %q", ns.Labels[RegimeLabel], RegimeHosting)
	}
	if ns.Labels["pod-security.kubernetes.io/enforce"] != "baseline" {
		t.Errorf("PSS enforce label = %q, want baseline", ns.Labels["pod-security.kubernetes.io/enforce"])
	}

	var quota corev1.ResourceQuota
	if err := cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: "tenant-quota"}, &quota); err != nil {
		t.Fatalf("resource quota not created: %v", err)
	}
	if quota.Spec.Hard.Pods().IsZero() {
		t.Error("resource quota has no pod cap")
	}
	var lr corev1.LimitRange
	if err := cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: "tenant-limits"}, &lr); err != nil {
		t.Fatalf("limit range not created: %v", err)
	}
	var np networkingv1.NetworkPolicy
	if err := cl.Get(ctx, client.ObjectKey{Namespace: nsName, Name: "default-deny"}, &np); err != nil {
		t.Fatalf("default-deny NetworkPolicy not created: %v", err)
	}
	if len(np.Spec.Ingress) != 0 || len(np.Spec.Egress) != 0 {
		t.Error("default-deny policy must have no allow rules")
	}
	if len(np.Spec.PolicyTypes) != 2 {
		t.Errorf("default-deny must cover ingress+egress, got %v", np.Spec.PolicyTypes)
	}
}

func TestHostingNamespaceGetsAllowPoliciesSandboxSealed(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, true) // both regimes
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	has := func(ns, name string) bool {
		var np networkingv1.NetworkPolicy
		return cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &np) == nil
	}

	host := WorkspaceNamespace(tn.ID)
	for _, name := range []string{"default-deny", "allow-same-namespace", "allow-dns-egress", "allow-traefik-ingress", "allow-internet-egress"} {
		if !has(host, name) {
			t.Errorf("hosting namespace missing policy %s", name)
		}
	}

	sandbox := SandboxNamespace(tn.ID)
	// Every sandbox is its own trust domain. Only default-deny is native; Cilium
	// supplies exact-SA execd ingress plus narrowed DNS/FQDN egress.
	if !has(sandbox, "default-deny") {
		t.Error("sandbox namespace missing default-deny")
	}
	for _, name := range []string{"allow-same-namespace", "allow-dns-egress", "allow-traefik-ingress", "allow-internet-egress", "allow-opensandbox-server-execd"} {
		if has(sandbox, name) {
			t.Errorf("sandbox namespace must NOT have %s (per-sandbox boundary)", name)
		}
	}
}

func TestSandboxReconcilePrunesLegacyBroadPolicies(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, true)
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	namespace := SandboxNamespace(tn.ID)
	// These are the exact policies emitted for sandbox namespaces before m35.
	for _, legacy := range []*networkingv1.NetworkPolicy{
		sameNamespaceNetworkPolicy(namespace),
		dnsEgressNetworkPolicy(namespace),
		{ObjectMeta: npMeta(namespace, "allow-opensandbox-server-execd")},
	} {
		if err := cl.Create(ctx, legacy); err != nil {
			t.Fatal(err)
		}
	}
	// Ownership, not just an unfamiliar name, is the deletion boundary.
	unmanaged := &networkingv1.NetworkPolicy{ObjectMeta: metav1.ObjectMeta{Name: "tenant-custom", Namespace: namespace}}
	if err := cl.Create(ctx, unmanaged); err != nil {
		t.Fatal(err)
	}

	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"allow-same-namespace", "allow-dns-egress", "allow-opensandbox-server-execd"} {
		var policy networkingv1.NetworkPolicy
		err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: name}, &policy)
		if !apierrors.IsNotFound(err) {
			t.Errorf("obsolete sandbox policy %s survived reconcile: %v", name, err)
		}
	}
	var custom networkingv1.NetworkPolicy
	if err := cl.Get(ctx, client.ObjectKey{Namespace: namespace, Name: unmanaged.Name}, &custom); err != nil {
		t.Fatalf("unmanaged tenant policy was pruned: %v", err)
	}
}

func TestResourceQuotaCarriesPlanScopedObjectCounts(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, false)
	// Free plan → Render Hobby anchors (25 services, 1 Postgres, 1 Key Value).
	free, _ := store.CreateTenant(ctx, "hobby", "free")
	// Paid plan → generous ceiling.
	paid, _ := store.CreateTenant(ctx, "team", "pro")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}

	countCap := func(nsID, res string) int64 {
		var q corev1.ResourceQuota
		if err := cl.Get(ctx, client.ObjectKey{Namespace: nsID, Name: "tenant-quota"}, &q); err != nil {
			t.Fatalf("quota for %s: %v", nsID, err)
		}
		v, ok := q.Spec.Hard[corev1.ResourceName(res)]
		if !ok {
			t.Fatalf("quota %s missing cap %s", nsID, res)
		}
		return v.Value()
	}

	if got := countCap(free.ID, "count/apps.app.bex.co"); got != 25 {
		t.Errorf("free apps cap = %d, want 25", got)
	}
	if got := countCap(free.ID, "count/databases.app.bex.co"); got != 1 {
		t.Errorf("free databases cap = %d, want 1", got)
	}
	if got := countCap(paid.ID, "count/apps.app.bex.co"); got != 100 {
		t.Errorf("paid apps cap = %d, want 100", got)
	}
}

func TestTenantRoleBindingsStampedPerNamespace(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, true) // both regimes
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	binding := func(ns, name string) (rbacv1.RoleBinding, bool) {
		var rb rbacv1.RoleBinding
		err := cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &rb)
		return rb, err == nil
	}

	// Hosting namespace binds all three roles, each to the right SA in bex-system.
	host := WorkspaceNamespace(tn.ID)
	rb, ok := binding(host, "bex-tenant-operator")
	if !ok {
		t.Fatal("hosting missing bex-tenant-operator binding")
	}
	if rb.RoleRef.Kind != "ClusterRole" || rb.RoleRef.Name != "bex-tenant-operator" {
		t.Errorf("operator binding roleRef = %+v", rb.RoleRef)
	}
	if len(rb.Subjects) != 1 || rb.Subjects[0].Name != "bex-controller-manager" || rb.Subjects[0].Namespace != "bex-system" {
		t.Errorf("operator binding subject = %+v", rb.Subjects)
	}
	for _, name := range []string{"bex-tenant-api", "bex-tenant-ssh-gateway"} {
		if _, ok := binding(host, name); !ok {
			t.Errorf("hosting missing %s binding", name)
		}
	}

	// Sandbox namespace binds the OpenSandbox server/controller roles and the
	// isolated SSH gateway's pods/exec role. The operator and bex-api never touch
	// a sandbox namespace; lifecycle SAs live in opensandbox-system.
	sandbox := SandboxNamespace(tn.ID)
	srvRB, ok := binding(sandbox, "bex-tenant-sandbox-server")
	if !ok {
		t.Fatal("sandbox missing bex-tenant-sandbox-server binding")
	}
	if srvRB.RoleRef.Name != "bex-tenant-sandbox-server" ||
		len(srvRB.Subjects) != 1 ||
		srvRB.Subjects[0].Name != "opensandbox-server" ||
		srvRB.Subjects[0].Namespace != "opensandbox-system" {
		t.Errorf("sandbox-server binding = ref %+v subjects %+v", srvRB.RoleRef, srvRB.Subjects)
	}
	controllerRB, ok := binding(sandbox, "bex-tenant-sandbox-controller")
	if !ok {
		t.Fatal("sandbox missing bex-tenant-sandbox-controller binding")
	}
	if controllerRB.RoleRef.Name != "bex-tenant-sandbox-controller" ||
		len(controllerRB.Subjects) != 1 ||
		controllerRB.Subjects[0].Name != "opensandbox-controller-manager" ||
		controllerRB.Subjects[0].Namespace != "opensandbox-system" {
		t.Errorf("sandbox-controller binding = ref %+v subjects %+v", controllerRB.RoleRef, controllerRB.Subjects)
	}
	gwRB, ok := binding(sandbox, "bex-tenant-ssh-gateway")
	if !ok || gwRB.RoleRef.Name != "bex-tenant-ssh-gateway" || len(gwRB.Subjects) != 1 ||
		gwRB.Subjects[0].Name != "bex-ssh-gateway" || gwRB.Subjects[0].Namespace != "bex-system" {
		t.Errorf("sandbox ssh-gateway binding missing/wrong: ok=%v ref=%+v subjects=%+v", ok, gwRB.RoleRef, gwRB.Subjects)
	}
	for _, name := range []string{"bex-tenant-operator", "bex-tenant-api"} {
		if _, ok := binding(sandbox, name); ok {
			t.Errorf("sandbox must NOT bind %s", name)
		}
	}
	// The hosting namespace must NOT bind the sandbox-server role.
	if _, ok := binding(host, "bex-tenant-sandbox-server"); ok {
		t.Error("hosting must NOT bind bex-tenant-sandbox-server")
	}
	if _, ok := binding(host, "bex-tenant-sandbox-controller"); ok {
		t.Error("hosting must NOT bind bex-tenant-sandbox-controller")
	}
}

func TestSandboxReconcilePrunesLegacyOperatorBinding(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, true)
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}

	sandbox := SandboxNamespace(tn.ID)
	legacy := tenantRoleBinding(sandbox, "bex-tenant-operator", operatorSA)
	if err := cl.Create(ctx, legacy); err != nil {
		t.Fatal(err)
	}
	custom := tenantRoleBinding(sandbox, "customer-binding", operatorSA)
	delete(custom.Labels, LabelManagedBy)
	if err := cl.Create(ctx, custom); err != nil {
		t.Fatal(err)
	}

	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var got rbacv1.RoleBinding
	if err := cl.Get(ctx, client.ObjectKey{Namespace: sandbox, Name: legacy.Name}, &got); !apierrors.IsNotFound(err) {
		t.Errorf("legacy sandbox operator binding survived reconcile: %v", err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Namespace: sandbox, Name: custom.Name}, &got); err != nil {
		t.Errorf("unmanaged custom binding was pruned: %v", err)
	}
}

func TestReconcileProvisionsSandboxNamespaceOnlyWhenEnabled(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, false)
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var ns corev1.Namespace
	err := cl.Get(ctx, client.ObjectKey{Name: SandboxNamespace(tn.ID)}, &ns)
	if !apierrors.IsNotFound(err) {
		t.Fatalf("sandbox namespace should not exist when disabled, got err=%v", err)
	}

	// Enable sandboxes and re-reconcile: now both regimes exist.
	r.Sandboxes = true
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Name: SandboxNamespace(tn.ID)}, &ns); err != nil {
		t.Fatalf("sandbox namespace missing after enable: %v", err)
	}
	if ns.Labels[RegimeLabel] != RegimeSandbox {
		t.Errorf("regime = %q, want sandbox", ns.Labels[RegimeLabel])
	}
}

func TestReconcileIsIdempotent(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, true)
	store.CreateTenant(ctx, "acme", "pro")
	for i := 0; i < 3; i++ {
		if err := r.ReconcileOnce(ctx); err != nil {
			t.Fatalf("reconcile pass %d: %v", i, err)
		}
	}
	var list corev1.NamespaceList
	if err := cl.List(ctx, &list, client.MatchingLabels{LabelManagedBy: ManagedByValue}); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 2 {
		t.Fatalf("want 2 managed namespaces (hosting+sandbox), got %d", len(list.Items))
	}
}

func TestReconcilePrunesNamespacesForDeletedWorkspace(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, true)
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	// Delete the workspace from the store, then reconcile.
	store.mu.Lock()
	delete(store.tenants, tn.ID)
	store.mu.Unlock()
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{WorkspaceNamespace(tn.ID), SandboxNamespace(tn.ID)} {
		var ns corev1.Namespace
		err := cl.Get(ctx, client.ObjectKey{Name: name}, &ns)
		// The fake client honors deletion immediately (no finalizers set here).
		if err == nil && ns.DeletionTimestamp == nil {
			t.Errorf("namespace %s should have been pruned", name)
		}
	}
}

func TestPruneNeverTouchesUnmanagedNamespaces(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, false)
	// A platform namespace with no managed-by label must survive prune even
	// though it maps to no workspace.
	platform := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "bex-system"}}
	if err := cl.Create(ctx, platform); err != nil {
		t.Fatal(err)
	}
	store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var ns corev1.Namespace
	if err := cl.Get(ctx, client.ObjectKey{Name: "bex-system"}, &ns); err != nil {
		t.Fatalf("platform namespace was wrongly pruned: %v", err)
	}
}

func TestApplyObjectRefusesToClobberUnmanaged(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t, false)
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	// Pre-create the hosting namespace WITHOUT the managed-by label — a
	// pre-existing object the reconciler must not overwrite.
	pre := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: WorkspaceNamespace(tn.ID)}}
	if err := cl.Create(ctx, pre); err != nil {
		t.Fatal(err)
	}
	err := r.ReconcileOnce(ctx)
	if err == nil {
		t.Fatal("expected an error when the namespace exists unmanaged")
	}
}
