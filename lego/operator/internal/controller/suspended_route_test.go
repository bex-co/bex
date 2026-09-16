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
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// suspendedWebApp is a manually suspended PAID web service in a tenant
// namespace: no auto-sleep window, so nothing but spec.suspended can put it on
// the activator route.
func suspendedWebApp() *appv1alpha1.App {
	app := hibernatingApp("tea-abc123")
	app.Annotations = nil
	app.Spec.Tier = "starter"
	app.Spec.IdleTTLSeconds = 0
	app.Spec.Suspended = true
	return app
}

// TestSuspendedWebServiceRoutesToActivator is the w2/m98 routing contract. A
// suspended web service keeps its Ingress and certificate but is parked at zero
// replicas, so pointing the public host at its own endpoint-less Service is
// exactly what made Traefik answer with its raw "503 no available server"
// (w1/094 pass 13). The backend must be the activator alias — which the
// activator answers with a content-negotiated bex 503, without waking anything.
func TestSuspendedWebServiceRoutesToActivator(t *testing.T) {
	scheme := wakeScheme()
	app := suspendedWebApp()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

	alias := activatorAliasName(app.Name)
	if got := ingressBackendName(t, cl, nn); got != alias {
		t.Fatalf("suspended backend = %q, want the activator alias %q — its own Service has no endpoint, so Traefik answers with raw text", got, alias)
	}
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("suspended replicas = %d, want 0 — routing must not change the workload", got)
	}

	// An Ingress backend resolves only inside the Ingress's namespace, so the
	// alias Service must exist there (the w6/m47 defect, repeated for suspend).
	var svc corev1.Service
	if err := cl.Get(ctx, types.NamespacedName{Name: alias, Namespace: app.Namespace}, &svc); err != nil {
		t.Fatalf("activator alias must exist in the App namespace: %v", err)
	}
	if svc.Spec.Type != corev1.ServiceTypeExternalName {
		t.Fatalf("activator alias = %+v, want an ExternalName to the platform activator", svc.Spec)
	}

	// The certificate is the same one it had while running: the Ingress keeps
	// its per-host TLS entry rather than being deleted and re-issued.
	var ing networkingv1.Ingress
	if err := cl.Get(ctx, nn, &ing); err != nil {
		t.Fatal(err)
	}
	if len(ing.Spec.TLS) == 0 || len(ing.Spec.Rules) != 1 || ing.Spec.Rules[0].Host != "web.onbex.co" {
		t.Fatalf("suspended Ingress = %+v, want the same host + TLS it served on", ing.Spec)
	}
}

// TestResumeReturnsSuspendedRouteToOwnService: clearing spec.suspended must
// hand the public host straight back to the App's own Service, with no extra
// reconcile step — ingressBackend is a pure function of the current spec.
func TestResumeReturnsSuspendedRouteToOwnService(t *testing.T) {
	scheme := wakeScheme()
	app := suspendedWebApp()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("suspended backend = %q, want the activator alias", got)
	}

	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Spec.Suspended = false
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("resumed backend = %q, want the App's own Service %q", got, app.Name)
	}
}

// TestMaintenanceWinsOverSuspendedRouting pins the m98 precedence: maintenance
// → suspended → sleep → own Service. An owner who configured a maintenance page
// still gets that page while the service is also suspended.
func TestMaintenanceWinsOverSuspendedRouting(t *testing.T) {
	scheme := wakeScheme()
	app := suspendedWebApp()
	app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

	if got := ingressBackendName(t, cl, nn); got != maintenanceAliasName(app.Name) {
		t.Fatalf("suspended + maintenance backend = %q, want the maintenance alias %q", got, maintenanceAliasName(app.Name))
	}
}

// TestSuspendedPrivateServiceHasNoIngress is the control for a type with no
// public host: suspending it must not mint an activator alias or an Ingress.
func TestSuspendedPrivateServiceHasNoIngress(t *testing.T) {
	scheme := wakeScheme()
	app := suspendedWebApp()
	app.Spec.Type = appv1alpha1.TypePrivateService
	app.Spec.Expose = false
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

	var ing networkingv1.Ingress
	if err := cl.Get(ctx, nn, &ing); !apierrors.IsNotFound(err) {
		t.Fatalf("suspended private service Ingress err = %v, want NotFound", err)
	}
	var svc corev1.Service
	key := types.NamespacedName{Name: activatorAliasName(app.Name), Namespace: app.Namespace}
	if err := cl.Get(ctx, key, &svc); !apierrors.IsNotFound(err) {
		t.Fatalf("suspended private service activator alias err = %v, want NotFound", err)
	}
}

// TestSuspendedRoutingWithoutActivatorKeepsOwnService: with no activator
// configured there is nothing to route to, so the App keeps its own Service and
// the behavior is byte-identical to before m98 rather than a dangling backend.
func TestSuspendedRoutingWithoutActivatorKeepsOwnService(t *testing.T) {
	scheme := wakeScheme()
	app := suspendedWebApp()
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	r.ActivatorService = ""
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("backend with no activator = %q, want the App's own Service %q", got, app.Name)
	}
}

// TestSleepingFreeServiceStillRoutesToActivator is the w6/m94 control: the
// sleep branch is unchanged by the suspended branch sitting above it.
func TestSleepingFreeServiceStillRoutesToActivator(t *testing.T) {
	scheme := wakeScheme()
	app := hibernatingApp("tea-abc123")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("sleeping backend = %q, want the activator alias", got)
	}
}
