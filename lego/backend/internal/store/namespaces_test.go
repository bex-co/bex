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
