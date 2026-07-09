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

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func newTestReconciler(t *testing.T) (*Reconciler, *memStore, client.Client) {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithStatusSubresource(&appv1alpha1.App{}).Build()
	store := newMemStore()
	return NewReconciler(cl, store, "default"), store, cl
}

// getApp fetches the one CR every test projects: tenant "acme" + app "web".
func getApp(t *testing.T, cl client.Client) *appv1alpha1.App {
	t.Helper()
	var app appv1alpha1.App
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "acme-web"}, &app); err != nil {
		t.Fatalf("get App acme-web: %v", err)
	}
	return &app
}

func TestReconcileCreatesAppCR(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{
		TenantID: ten.ID, Name: "web", Image: "traefik/whoami", Branch: "main",
		Port: 80, Replicas: 2, Tier: "starter",
	})

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	app := getApp(t, cl)
	if app.Labels[LabelManagedBy] != ManagedByValue || app.Labels[LabelAppID] != row.ID || app.Labels[LabelTenant] != ten.ID {
		t.Errorf("labels = %v", app.Labels)
	}
	if app.Spec.Image != "traefik/whoami" || app.Spec.Port != 80 || app.Spec.Replicas != 2 ||
		app.Spec.Tier != "starter" || !app.Spec.Expose {
		t.Errorf("spec = %+v", app.Spec)
	}
}

func TestReconcileUpdatesOwnedFieldsOnly(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img:1", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	// Simulate a field the control plane doesn't own (set by bex-api / defaulting).
	app := getApp(t, cl)
	app.Spec.Builder = "dockerfile"
	app.Spec.RestartedAt = "2026-07-05T00:00:00Z"
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}

	// Change owned state in the DB: scale up + add domains.
	store.mu.Lock()
	a := store.apps[row.ID]
	a.Replicas, a.Image = 3, "img:2"
	store.apps[row.ID] = a
	store.mu.Unlock()
	if _, err := store.CreateDomain(ctx, row.ID, "extra.example.com", false); err != nil {
		t.Fatal(err)
	}
	if _, err := store.CreateDomain(ctx, row.ID, "web.example.com", true); err != nil {
		t.Fatal(err)
	}

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	app = getApp(t, cl)
	if app.Spec.Replicas != 3 || app.Spec.Image != "img:2" {
		t.Errorf("owned fields not updated: %+v", app.Spec)
	}
	if app.Spec.Host != "web.example.com" || len(app.Spec.Hosts) != 1 || app.Spec.Hosts[0] != "extra.example.com" {
		t.Errorf("hosts: host=%q hosts=%v", app.Spec.Host, app.Spec.Hosts)
	}
	if app.Spec.Builder != "dockerfile" || app.Spec.RestartedAt != "2026-07-05T00:00:00Z" {
		t.Errorf("unowned fields stomped: builder=%q restartedAt=%q", app.Spec.Builder, app.Spec.RestartedAt)
	}
}

func TestReconcileDeletesRemovedRowsOnly(t *testing.T) {
	ctx := context.Background()
	rec, store, cl := newTestReconciler(t)
	ten, _ := store.CreateTenant(ctx, "acme", "free")
	row, _ := store.CreateApp(ctx, App{TenantID: ten.ID, Name: "web", Image: "img", Branch: "main", Port: 80, Replicas: 1, Tier: "free"})

	// A hand-applied App (no managed-by label) must never be touched.
	manual := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "hand-applied", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Image: "img"},
	}
	if err := cl.Create(ctx, manual); err != nil {
		t.Fatal(err)
	}

	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if err := store.DeleteApp(ctx, row.ID); err != nil {
		t.Fatal(err)
	}
	if err := rec.ReconcileOnce(ctx); err != nil {
		t.Fatalf("reconcile: %v", err)
	}

	var apps appv1alpha1.AppList
	if err := cl.List(ctx, &apps, client.InNamespace("default")); err != nil {
		t.Fatal(err)
	}
	if len(apps.Items) != 1 || apps.Items[0].Name != "hand-applied" {
		names := make([]string, 0, len(apps.Items))
		for _, a := range apps.Items {
			names = append(names, a.Name)
		}
		t.Errorf("want only hand-applied to survive, got %v", names)
	}
}
