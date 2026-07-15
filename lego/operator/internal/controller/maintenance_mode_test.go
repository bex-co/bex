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

package controller

import (
	"context"
	"reflect"
	"strings"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestMaintenanceModeOnlySwitchesPublicIngress(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Image: "nginx:1", Tier: "starter",
			Port: 3000, Replicas: 2, Expose: true, Hosts: []string{"custom.example.com"},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		MaintenanceService: "bex-activator", MaintenanceNamespace: "default", MaintenancePort: 8888,
	}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "web", Namespace: "default"}
	reconcileApp := func() {
		t.Helper()
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
	reconcileApp() // finalizer
	reconcileApp() // resources

	var beforeDep appsv1.Deployment
	var beforeSvc corev1.Service
	if err := cl.Get(ctx, nn, &beforeDep); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, nn, &beforeSvc); err != nil {
		t.Fatal(err)
	}

	if err := cl.Get(ctx, nn, app); err != nil {
		t.Fatal(err)
	}
	app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	reconcileApp()

	var ing networkingv1.Ingress
	if err := cl.Get(ctx, nn, &ing); err != nil {
		t.Fatal(err)
	}
	if len(ing.Spec.Rules) != 2 {
		t.Fatalf("Ingress hosts = %d, want platform + custom", len(ing.Spec.Rules))
	}
	for _, rule := range ing.Spec.Rules {
		backend := rule.HTTP.Paths[0].Backend.Service
		if backend.Name != "bex-activator" || backend.Port.Number != 8888 {
			t.Fatalf("maintenance backend = %+v", backend)
		}
	}
	var afterDep appsv1.Deployment
	var afterSvc corev1.Service
	if err := cl.Get(ctx, nn, &afterDep); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, nn, &afterSvc); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(beforeDep.Spec, afterDep.Spec) || !reflect.DeepEqual(beforeSvc.Spec, afterSvc.Spec) {
		t.Fatal("maintenance mode changed the workload Deployment or private Service")
	}

	// Suspension scales the workload independently but the maintenance responder
	// keeps owning public traffic; resume restores replicas without clearing it.
	if err := cl.Get(ctx, nn, app); err != nil {
		t.Fatal(err)
	}
	app.Spec.Suspended = true
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	reconcileApp()
	if err := cl.Get(ctx, nn, &afterDep); err != nil {
		t.Fatal(err)
	}
	if afterDep.Spec.Replicas == nil || *afterDep.Spec.Replicas != 0 {
		t.Fatalf("suspended maintenance replicas = %v, want 0", afterDep.Spec.Replicas)
	}
	if err := cl.Get(ctx, nn, &ing); err != nil {
		t.Fatal(err)
	}
	for _, rule := range ing.Spec.Rules {
		if got := rule.HTTP.Paths[0].Backend.Service.Name; got != "bex-activator" {
			t.Fatalf("suspended maintenance backend = %q", got)
		}
	}
	if err := cl.Get(ctx, nn, app); err != nil {
		t.Fatal(err)
	}
	app.Spec.Suspended = false
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	reconcileApp()
	if err := cl.Get(ctx, nn, &afterDep); err != nil {
		t.Fatal(err)
	}
	if afterDep.Spec.Replicas == nil || *afterDep.Spec.Replicas != 2 {
		t.Fatalf("resumed maintenance replicas = %v, want 2", afterDep.Spec.Replicas)
	}

	if err := cl.Get(ctx, nn, app); err != nil {
		t.Fatal(err)
	}
	app.Spec.MaintenanceMode.Enabled = false
	if err := cl.Update(ctx, app); err != nil {
		t.Fatal(err)
	}
	reconcileApp()
	if err := cl.Get(ctx, nn, &ing); err != nil {
		t.Fatal(err)
	}
	for _, rule := range ing.Spec.Rules {
		if got := rule.HTTP.Paths[0].Backend.Service.Name; got != "web" {
			t.Fatalf("disabled backend = %q, want web", got)
		}
	}
}

func TestMaintenanceModePrecedesAutoHibernate(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "legacy-free-maintenance",
			Namespace: "default",
			Annotations: map[string]string{
				annotLastActive: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Image: "nginx:1", Tier: "free",
			Port: 3000, Replicas: 1, Expose: true, IdleTTLSeconds: 1,
			MaintenanceMode: &appv1alpha1.MaintenanceModeSpec{Enabled: true},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		ActivatorService: "wake-activator", ActivatorPort: 7777,
		MaintenanceService: "maintenance-responder", MaintenanceNamespace: "default", MaintenancePort: 8888,
	}
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	for range 2 {
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}

	var dep appsv1.Deployment
	if err := cl.Get(ctx, nn, &dep); err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 0 {
		t.Fatalf("auto-hibernating replicas = %v, want 0", dep.Spec.Replicas)
	}
	var ing networkingv1.Ingress
	if err := cl.Get(ctx, nn, &ing); err != nil {
		t.Fatal(err)
	}
	backend := ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	if backend.Name != "maintenance-responder" || backend.Port.Number != 8888 {
		t.Fatalf("combined maintenance/auto-sleep backend = %+v, want maintenance responder", backend)
	}
}

func TestCrossNamespaceMaintenanceUsesOwnedExternalNameAlias(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Image: "nginx:1", Tier: "starter",
			Port: 3000, Replicas: 1, Expose: true,
			MaintenanceMode: &appv1alpha1.MaintenanceModeSpec{Enabled: true},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		MaintenanceService: "bex-activator", MaintenanceNamespace: "bex-system", MaintenancePort: 8888,
	}
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	for range 2 {
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}

	aliasName := maintenanceAliasName(app.Name)
	var alias corev1.Service
	if err := cl.Get(ctx, types.NamespacedName{Name: aliasName, Namespace: app.Namespace}, &alias); err != nil {
		t.Fatal(err)
	}
	if alias.Spec.Type != corev1.ServiceTypeExternalName ||
		alias.Spec.ExternalName != "bex-activator.bex-system.svc.cluster.local" ||
		len(alias.Spec.Ports) != 1 || alias.Spec.Ports[0].Port != 8888 {
		t.Fatalf("maintenance alias = %+v", alias.Spec)
	}
	if len(alias.OwnerReferences) != 1 || alias.OwnerReferences[0].UID != app.UID {
		t.Fatalf("maintenance alias ownerRefs = %+v, want App owner", alias.OwnerReferences)
	}
	var ing networkingv1.Ingress
	if err := cl.Get(ctx, nn, &ing); err != nil {
		t.Fatal(err)
	}
	backend := ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service
	if backend.Name != aliasName || backend.Port.Number != 8888 {
		t.Fatalf("cross-namespace maintenance backend = %+v, want alias %q", backend, aliasName)
	}
	var tenantService corev1.Service
	if err := cl.Get(ctx, nn, &tenantService); err != nil {
		t.Fatal(err)
	}
	if tenantService.Spec.Type == corev1.ServiceTypeExternalName || tenantService.Spec.Selector[labelApp] != app.Name {
		t.Fatalf("tenant Service was replaced by maintenance alias: %+v", tenantService.Spec)
	}

	longName := strings.Repeat("a", 63)
	got := maintenanceAliasName(longName)
	if len(got) > 63 || got == maintenanceAliasName(strings.Repeat("a", 62)+"b") {
		t.Fatalf("long maintenance alias is invalid or colliding: %q", got)
	}
}
