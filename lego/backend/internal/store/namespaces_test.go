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
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func newTestNamespaceReconciler(t *testing.T) (*NamespaceReconciler, *memStore, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).Build()
	store := newMemStore()
	r := NewNamespaceReconciler(cl, store)
	return r, store, cl
}

func TestReconcileProvisionsHostingNamespaceWithBaseObjects(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
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
	r, store, cl := newTestNamespaceReconciler(t)
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	has := func(ns, name string) bool {
		var np networkingv1.NetworkPolicy
		return cl.Get(ctx, client.ObjectKey{Namespace: ns, Name: name}, &np) == nil
	}

	host := WorkspaceNamespace(tn.ID)
	for _, name := range []string{"default-deny", "allow-same-namespace", "allow-dns-egress", "allow-traefik-ingress", "allow-internet-egress", "allow-datastore-control-ingress"} {
		if !has(host, name) {
			t.Errorf("hosting namespace missing policy %s", name)
		}
	}

	sandbox := SandboxNamespace(tn.ID)
	// Every sandbox is its own trust domain. Only default-deny is native at the
	// k8s-NetworkPolicy layer; the sanctioned gateway->driver ingress is a
	// cluster-wide Cilium policy (deploy/gitops), not a per-namespace k8s allow
	// bex-api creates — the ADR045 admission control forbids the latter.
	if !has(sandbox, "default-deny") {
		t.Errorf("sandbox namespace missing default-deny")
	}
	for _, name := range []string{"allow-same-namespace", "allow-dns-egress", "allow-traefik-ingress", "allow-internet-egress", "allow-datastore-control-ingress", "allow-opensandbox-server-execd", "allow-gateway-driver-ingress"} {
		if has(sandbox, name) {
			t.Errorf("sandbox namespace must NOT have %s (per-sandbox boundary / Cilium-managed)", name)
		}
	}
}

func TestSandboxReconcilePrunesLegacyBroadPolicies(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
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
	r, store, cl := newTestNamespaceReconciler(t)
	// Legacy "free" alias → Render Hobby anchors (25 services, 1 Postgres, 1 Key Value).
	free, _ := store.CreateTenant(ctx, "hobby-legacy", "free")
	// The real stored plan name every workspace actually gets (NormalizePlan
	// never persists "free" — this is the exact case that regressed silently
	// before QuotaCapsForPlan existed: quotaForPlan used to match only "",
	// "free", so every real Hobby workspace fell through to the paid ceiling).
	hobby, _ := store.CreateTenant(ctx, "hobby", "hobby")
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
	if got := countCap(hobby.ID, "count/apps.app.bex.co"); got != 25 {
		t.Errorf("hobby apps cap = %d, want 25", got)
	}
	if got := countCap(hobby.ID, "count/databases.app.bex.co"); got != 1 {
		t.Errorf("hobby databases cap = %d, want 1", got)
	}
	if got := countCap(paid.ID, "count/apps.app.bex.co"); got != 100 {
		t.Errorf("paid apps cap = %d, want 100", got)
	}
}

// TestResourceQuotaCarriesStoragePVCandLBCaps pins the storage/PVC/LoadBalancer
// axis (ADR045 Finding 4, w7/m59): every provisioned namespace's quota bounds
// requests.storage + persistentvolumeclaims and zero-caps billable cloud
// Services. A dropped dimension turns this red.
func TestResourceQuotaCarriesStoragePVCandLBCaps(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
	free, _ := store.CreateTenant(ctx, "hobby", "free")
	paid, _ := store.CreateTenant(ctx, "team", "pro")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}

	quotaFor := func(nsID string) corev1.ResourceQuota {
		var q corev1.ResourceQuota
		if err := cl.Get(ctx, client.ObjectKey{Namespace: nsID, Name: "tenant-quota"}, &q); err != nil {
			t.Fatalf("quota for %s: %v", nsID, err)
		}
		return q
	}
	hard := func(q corev1.ResourceQuota, res corev1.ResourceName) resource.Quantity {
		v, ok := q.Spec.Hard[res]
		if !ok {
			t.Fatalf("quota %s missing cap %s", q.Namespace, res)
		}
		return v
	}

	for _, tc := range []struct {
		nsID        string
		wantStorage string
		wantPVCs    int64
	}{
		{free.ID, "20Gi", 4},
		{paid.ID, "5Ti", 200},
	} {
		q := quotaFor(tc.nsID)
		if got := hard(q, corev1.ResourceRequestsStorage); got.String() != tc.wantStorage {
			t.Errorf("%s requests.storage = %s, want %s", tc.nsID, got.String(), tc.wantStorage)
		}
		if got := hard(q, corev1.ResourcePersistentVolumeClaims); got.Value() != tc.wantPVCs {
			t.Errorf("%s persistentvolumeclaims = %d, want %d", tc.nsID, got.Value(), tc.wantPVCs)
		}
		// Billable cloud Services are denied outright in every tenant namespace.
		if got := hard(q, corev1.ResourceServicesLoadBalancers); got.Value() != 0 {
			t.Errorf("%s services.loadbalancers = %d, want 0", tc.nsID, got.Value())
		}
		if got := hard(q, corev1.ResourceServicesNodePorts); got.Value() != 0 {
			t.Errorf("%s services.nodeports = %d, want 0", tc.nsID, got.Value())
		}
	}
}

// TestResourceQuotaConvergesExistingNamespaceToNewShape proves the reconciler's
// update path rewrites an already-provisioned quota that predates the
// storage/PVC/LB dimensions — the non-disruptive rollout t003 relies on (a
// create-only reconcile would leave old namespaces uncapped, turning this red).
func TestResourceQuotaConvergesExistingNamespaceToNewShape(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
	tn, _ := store.CreateTenant(ctx, "legacy", "free")

	// Seed a managed quota in the pre-m59 shape (pods only, no storage axis).
	stale := &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "tenant-quota",
			Namespace: WorkspaceNamespace(tn.ID),
			Labels:    r.managedLabels(tn.ID),
		},
		Spec: corev1.ResourceQuotaSpec{Hard: corev1.ResourceList{
			corev1.ResourcePods: resource.MustParse("50"),
		}},
	}
	// The namespace must exist first for the quota to live in it.
	if err := cl.Create(ctx, r.workspaceNamespaceObject(WorkspaceNamespace(tn.ID), tn, RegimeHosting)); err != nil {
		t.Fatal(err)
	}
	if err := cl.Create(ctx, stale); err != nil {
		t.Fatal(err)
	}

	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}

	var q corev1.ResourceQuota
	if err := cl.Get(ctx, client.ObjectKey{Namespace: WorkspaceNamespace(tn.ID), Name: "tenant-quota"}, &q); err != nil {
		t.Fatal(err)
	}
	if _, ok := q.Spec.Hard[corev1.ResourceRequestsStorage]; !ok {
		t.Error("reconcile did not converge existing quota to the new storage shape")
	}
	if _, ok := q.Spec.Hard[corev1.ResourceServicesLoadBalancers]; !ok {
		t.Error("reconcile did not add the services.loadbalancers cap to an existing quota")
	}
}

func TestTenantRoleBindingsStampedPerNamespace(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
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
	// Snapshot resume-pull minting (w3/m42): the operator gets
	// get/create/update/patch on Secrets ONLY through this per-sandbox-namespace
	// binding (update/patch = protected-label backfill, w2/m82).
	snapRB, ok := binding(sandbox, "bex-operator-snapshot-pull")
	if !ok || snapRB.RoleRef.Name != "bex-operator-snapshot-pull" || len(snapRB.Subjects) != 1 ||
		snapRB.Subjects[0].Name != "bex-controller-manager" || snapRB.Subjects[0].Namespace != "bex-system" {
		t.Errorf("sandbox snapshot-pull binding missing/wrong: ok=%v ref=%+v subjects=%+v", ok, snapRB.RoleRef, snapRB.Subjects)
	}
	for _, name := range []string{"bex-tenant-operator", "bex-tenant-api"} {
		if _, ok := binding(sandbox, name); ok {
			t.Errorf("sandbox must NOT bind %s", name)
		}
	}
	if _, ok := binding(host, "bex-operator-snapshot-pull"); ok {
		t.Error("hosting must NOT bind bex-operator-snapshot-pull")
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
	r, store, cl := newTestNamespaceReconciler(t)
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

// TestReconcileAlwaysProvisionsSandboxNamespace proves the hosting and sandbox
// namespaces are provisioned together, unconditionally (the transitional
// Sandboxes toggle was retired in w3/m34 once BEX_TENANT_SANDBOX_NAMESPACES
// was always on in production).
func TestReconcileAlwaysProvisionsSandboxNamespace(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var ns corev1.Namespace
	if err := cl.Get(ctx, client.ObjectKey{Name: SandboxNamespace(tn.ID)}, &ns); err != nil {
		t.Fatalf("sandbox namespace missing: %v", err)
	}
	if ns.Labels[RegimeLabel] != RegimeSandbox {
		t.Errorf("regime = %q, want sandbox", ns.Labels[RegimeLabel])
	}
}

func TestReconcileIsIdempotent(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
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
	r, store, cl := newTestNamespaceReconciler(t)
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
	r, store, cl := newTestNamespaceReconciler(t)
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
	r, store, cl := newTestNamespaceReconciler(t)
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

// --- Control-plane identity (w6/m39) ---------------------------------------

// reconcilerOn builds a second reconciler sharing cl but with its own store and
// control-plane identity — the shape of two dev-N harnesses on one cluster.
func reconcilerOn(cl client.Client, identity string) (*NamespaceReconciler, *memStore) {
	store := newMemStore()
	r := NewNamespaceReconciler(cl, store)
	r.Identity = identity
	return r, store
}

func namespaceExists(t *testing.T, cl client.Client, name string) bool {
	t.Helper()
	var ns corev1.Namespace
	err := cl.Get(context.Background(), client.ObjectKey{Name: name}, &ns)
	if apierrors.IsNotFound(err) {
		return false
	}
	if err != nil {
		t.Fatalf("get namespace %s: %v", name, err)
	}
	// The fake client honors deletion immediately when no finalizer is set, but
	// treat a tombstoned namespace as gone either way.
	return ns.DeletionTimestamp == nil
}

func TestNamespaceCarriesControlPlaneIdentity(t *testing.T) {
	ctx := context.Background()
	r, store, cl := newTestNamespaceReconciler(t)
	tn, _ := store.CreateTenant(ctx, "acme", "free")
	if err := r.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	var ns corev1.Namespace
	if err := cl.Get(ctx, client.ObjectKey{Name: WorkspaceNamespace(tn.ID)}, &ns); err != nil {
		t.Fatal(err)
	}
	// An unconfigured control plane must stamp the default, not an empty value:
	// an empty label would match nothing and silently disable its own prune.
	if got := ns.Labels[ControlPlaneLabel]; got != DefaultControlPlaneIdentity {
		t.Errorf("control-plane label = %q, want %q", got, DefaultControlPlaneIdentity)
	}

	// A configured harness stamps its own identity. Use a separate reconciler:
	// flipping Identity on one that already owns production-stamped namespaces
	// is precisely what applyObject's foreign-owner guard refuses.
	six, storeSix := reconcilerOn(cl, "dev-6")
	tn2, _ := storeSix.CreateTenant(ctx, "beta", "free")
	if err := six.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, client.ObjectKey{Name: WorkspaceNamespace(tn2.ID)}, &ns); err != nil {
		t.Fatal(err)
	}
	if got := ns.Labels[ControlPlaneLabel]; got != "dev-6" {
		t.Errorf("control-plane label = %q, want dev-6", got)
	}
}

// A control plane must refuse to CONVERGE another identity's namespace, not
// just refuse to prune it: re-stamping it would let the delete pass reap it
// later, bypassing the ownership guard in one hop. The hazard is concrete —
// a harness raised from another control plane's bex-db dump inherits its
// tenant rows and would otherwise steal every namespace they name.
func TestApplyObjectRefusesAnotherControlPlanesNamespace(t *testing.T) {
	ctx := context.Background()
	_, _, cl := newTestNamespaceReconciler(t)
	five, storeFive := reconcilerOn(cl, "dev-5")
	tn, _ := storeFive.CreateTenant(ctx, "shared", "free")
	if err := five.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}

	// dev-6 inherits the same tenant row (the bex-db dump case).
	six, storeSix := reconcilerOn(cl, "dev-6")
	storeSix.mu.Lock()
	storeSix.tenants[tn.ID] = tn
	storeSix.mu.Unlock()

	if err := six.ReconcileOnce(ctx); err == nil {
		t.Fatal("dev-6 converged a namespace owned by dev-5")
	}
	var ns corev1.Namespace
	if err := cl.Get(ctx, client.ObjectKey{Name: WorkspaceNamespace(tn.ID)}, &ns); err != nil {
		t.Fatal(err)
	}
	if got := ns.Labels[ControlPlaneLabel]; got != "dev-5" {
		t.Errorf("namespace was re-stamped to %q; owner must stay dev-5", got)
	}
}

// The mirror of the rule above: an UNLABELED namespace stays adoptable by any
// identity, which is how a pre-m39 namespace gains its label at all. Adoption
// and pruning deliberately disagree about the unlabeled case.
func TestApplyObjectAdoptsUnlabeledNamespace(t *testing.T) {
	ctx := context.Background()
	_, _, cl := newTestNamespaceReconciler(t)
	six, storeSix := reconcilerOn(cl, "dev-6")
	tn, _ := storeSix.CreateTenant(ctx, "legacy", "free")
	pre := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
		Name: WorkspaceNamespace(tn.ID),
		Labels: map[string]string{
			LabelManagedBy:              ManagedByValue,
			core.LabelWorkspace:         tn.ID,
			"app.kubernetes.io/part-of": "bex",
		},
	}}
	if err := cl.Create(ctx, pre); err != nil {
		t.Fatal(err)
	}
	if err := six.ReconcileOnce(ctx); err != nil {
		t.Fatalf("dev-6 refused to adopt an unlabeled namespace: %v", err)
	}
	var ns corev1.Namespace
	if err := cl.Get(ctx, client.ObjectKey{Name: WorkspaceNamespace(tn.ID)}, &ns); err != nil {
		t.Fatal(err)
	}
	if got := ns.Labels[ControlPlaneLabel]; got != "dev-6" {
		t.Errorf("adopted namespace label = %q, want dev-6", got)
	}
}

// TestTwoControlPlanesOnOneClusterPruneOnlyTheirOwn is the regression this
// milestone exists for: two dev-N harnesses sharing the CAPD mock cluster used
// to delete each other's tenant namespaces within one resync, because
// pruneOrphans is cluster-scoped and each control plane sees only its own
// database (.pm/w3/017.md, observed live in both directions).
func TestTwoControlPlanesOnOneClusterPruneOnlyTheirOwn(t *testing.T) {
	ctx := context.Background()
	_, _, cl := newTestNamespaceReconciler(t)

	five, storeFive := reconcilerOn(cl, "dev-5")
	six, storeSix := reconcilerOn(cl, "dev-6")
	tnFive, _ := storeFive.CreateTenant(ctx, "five-tenant", "free")
	tnSix, _ := storeSix.CreateTenant(ctx, "six-tenant", "free")

	if err := five.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := six.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}

	// Each harness's tenant is an "orphan" from the other's point of view — its
	// workspace is absent from that database — so a prune pass in either
	// direction must leave the other's namespaces alone.
	for pass := 0; pass < 2; pass++ {
		if err := five.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if err := six.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
	}

	for _, tc := range []struct {
		who string
		id  string
	}{{"dev-5", tnFive.ID}, {"dev-6", tnSix.ID}} {
		for _, name := range []string{WorkspaceNamespace(tc.id), SandboxNamespace(tc.id)} {
			if !namespaceExists(t, cl, name) {
				t.Errorf("%s namespace %s was pruned by the other control plane", tc.who, name)
			}
		}
	}
}

func TestPruneReclaimsOwnOrphanButNotFriendlyIdentities(t *testing.T) {
	ctx := context.Background()
	_, _, cl := newTestNamespaceReconciler(t)
	six, storeSix := reconcilerOn(cl, "dev-6")
	five, storeFive := reconcilerOn(cl, "dev-5")

	mine, _ := storeSix.CreateTenant(ctx, "mine", "free")
	theirs, _ := storeFive.CreateTenant(ctx, "theirs", "free")
	if err := six.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := five.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}

	// dev-6 loses its own workspace: its namespaces are a genuine orphan and
	// must still be reclaimed — the identity filter narrows the prune, it must
	// not disable it.
	storeSix.mu.Lock()
	delete(storeSix.tenants, mine.ID)
	storeSix.mu.Unlock()
	if err := six.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if namespaceExists(t, cl, WorkspaceNamespace(mine.ID)) {
		t.Error("dev-6 failed to reclaim its OWN orphan namespace")
	}
	if !namespaceExists(t, cl, WorkspaceNamespace(theirs.ID)) {
		t.Error("dev-6 pruned dev-5's live namespace")
	}
}

// TestUnlabeledNamespaceIsPrunableOnlyByTheDefaultIdentity pins the deliberate
// asymmetry: namespaces provisioned before w6/m39 carry no identity label, so
// production must still reclaim them, while a dev-N harness must never delete a
// namespace it cannot prove it created. Both halves fail in the safe direction.
func TestUnlabeledNamespaceIsPrunableOnlyByTheDefaultIdentity(t *testing.T) {
	ctx := context.Background()

	legacyNamespace := func(cl client.Client, name string) {
		ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{
			Name: name,
			Labels: map[string]string{
				LabelManagedBy:              ManagedByValue,
				core.LabelWorkspace:         name,
				"app.kubernetes.io/part-of": "bex",
				// No ControlPlaneLabel — the pre-m39 shape.
			},
		}}
		if err := cl.Create(ctx, ns); err != nil {
			t.Fatal(err)
		}
	}

	t.Run("production reclaims it", func(t *testing.T) {
		r, _, cl := newTestNamespaceReconciler(t)
		legacyNamespace(cl, "tea-legacyorphan00000")
		if err := r.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if namespaceExists(t, cl, "tea-legacyorphan00000") {
			t.Error("production must still reclaim a pre-m39 unlabeled orphan")
		}
	})

	t.Run("a dev harness leaves it alone", func(t *testing.T) {
		_, _, cl := newTestNamespaceReconciler(t)
		legacyNamespace(cl, "tea-legacyorphan00000")
		six, _ := reconcilerOn(cl, "dev-6")
		if err := six.ReconcileOnce(ctx); err != nil {
			t.Fatal(err)
		}
		if !namespaceExists(t, cl, "tea-legacyorphan00000") {
			t.Error("a dev-N harness must not delete a namespace it cannot prove it created")
		}
	})
}

// Production must not delete another identity's namespace either: leaking a
// stale dev namespace is recoverable, deleting a live tenant's is not.
func TestDefaultIdentityDoesNotPruneAnotherIdentitysNamespace(t *testing.T) {
	ctx := context.Background()
	prod, _, cl := newTestNamespaceReconciler(t)
	six, storeSix := reconcilerOn(cl, "dev-6")
	tn, _ := storeSix.CreateTenant(ctx, "devtenant", "free")
	if err := six.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if err := prod.ReconcileOnce(ctx); err != nil {
		t.Fatal(err)
	}
	if !namespaceExists(t, cl, WorkspaceNamespace(tn.ID)) {
		t.Error("production pruned a dev-N harness's namespace")
	}
}
