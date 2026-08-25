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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// hibernatingApp is an idle free-tier web service past its idle TTL: exactly
// the shape desiredReplicas auto-hibernates.
func hibernatingApp(namespace string) *appv1alpha1.App {
	// A tenant App is canonical only when its workspace label matches its
	// namespace (the confused-deputy guard in Reconcile); the bootstrap apps
	// namespace needs no label.
	labels := map[string]string{}
	if namespace != defaultAppsNamespace {
		labels[labelWorkspace] = namespace
	}
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: namespace, Labels: labels,
			Annotations: map[string]string{
				annotLastActive: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339),
			},
		},
		Spec: appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeWebService, Image: "nginx:1", Tier: "free",
			Port: 3000, Replicas: 1, Expose: true, IdleTTLSeconds: 1,
		},
	}
}

func reconcileTwice(t *testing.T, r *AppReconciler, nn types.NamespacedName) {
	t.Helper()
	for range 2 {
		if _, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
}

// TestAutoHibernateIngressBackendResolvesInTenantNamespace is the regression
// test for w6/m47 t001. An Ingress backend resolves only within the Ingress's
// own namespace, so naming the platform activator Service directly left every
// auto-hibernated App in a per-tenant namespace (ADR043) pointing at a Service
// that does not exist there: Traefik answered the public URL with its own
// default-backend 404, the activator never saw the request, and the App never
// woke. The backend must be an App-owned ExternalName alias in the App's own
// namespace instead.
func TestAutoHibernateIngressBackendResolvesInTenantNamespace(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := hibernatingApp("tea-abc123")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		ActivatorService: "bex-activator", ActivatorNamespace: "bex-system", ActivatorPort: 8888,
	}
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

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
	aliasName := activatorAliasName(app.Name)
	if backend.Name != aliasName || backend.Port.Number != 8888 {
		t.Fatalf("hibernated backend = %+v, want in-namespace alias %q:8888", backend, aliasName)
	}

	// The backend must actually exist in the Ingress's namespace — the whole
	// point of the alias. A name Traefik cannot resolve is the original bug.
	var alias corev1.Service
	if err := cl.Get(ctx, types.NamespacedName{Name: backend.Name, Namespace: app.Namespace}, &alias); err != nil {
		t.Fatalf("ingress backend Service must exist in the App namespace: %v", err)
	}
	if alias.Spec.Type != corev1.ServiceTypeExternalName ||
		alias.Spec.ExternalName != "bex-activator.bex-system.svc.cluster.local" ||
		len(alias.Spec.Ports) != 1 || alias.Spec.Ports[0].Port != 8888 {
		t.Fatalf("wake alias = %+v, want ExternalName to the platform activator:8888", alias.Spec)
	}
	// Labels/owner are the admission identity operator-alias-admission.yaml
	// requires; drifting from them means the policy denies the create in
	// production while every fake-client test still passes.
	if len(alias.OwnerReferences) != 1 || alias.OwnerReferences[0].UID != app.UID {
		t.Fatalf("wake alias ownerRefs = %+v, want App owner", alias.OwnerReferences)
	}
	if alias.Labels[labelApp] != app.Name ||
		alias.Labels[labelPlatformAliasPurpose] != platformAliasActivator ||
		alias.Labels["app.kubernetes.io/managed-by"] != "bex-operator" {
		t.Fatalf("wake alias labels = %+v, want exact admission identity", alias.Labels)
	}
}

// An App that already shares the activator's namespace needs no alias — the
// Service is directly resolvable there.
func TestAutoHibernateInActivatorNamespaceUsesActivatorDirectly(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := hibernatingApp(defaultAppsNamespace)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		ActivatorService: "bex-activator", ActivatorNamespace: defaultAppsNamespace, ActivatorPort: 8888,
	}
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

	var ing networkingv1.Ingress
	if err := cl.Get(ctx, nn, &ing); err != nil {
		t.Fatal(err)
	}
	if backend := ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service; backend.Name != "bex-activator" {
		t.Fatalf("same-namespace hibernated backend = %+v, want bex-activator", backend)
	}
	var alias corev1.Service
	err := cl.Get(ctx, types.NamespacedName{Name: activatorAliasName(app.Name), Namespace: app.Namespace}, &alias)
	if err == nil {
		t.Fatal("no alias should be created when the App shares the activator namespace")
	}
}

// Waking restores the App's own Service as the public backend: the alias is a
// sleeping-state detail, not a permanent indirection. w6/m94 added the
// precondition — the swap back waits until a pod is actually ready, so the
// route is never handed to a Service that has no endpoint to serve it.
func TestWakeRestoresAppServiceAsIngressBackend(t *testing.T) {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	app := hibernatingApp("tea-abc123")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		ActivatorService: "bex-activator", ActivatorNamespace: "bex-system", ActivatorPort: 8888,
	}
	ctx := context.Background()
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}
	reconcileTwice(t, r, nn)

	// What the activator does on a public request: stamp last-active so the
	// next reconcile stops hibernating.
	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Annotations[annotLastActive] = time.Now().UTC().Format(time.RFC3339)
	if err := cl.Update(ctx, &live); err != nil {
		t.Fatal(err)
	}
	reconcileTwice(t, r, nn)

	// Still waking: replicas are restored but no pod is ready yet, so the
	// activator keeps the route. Handing it back here is what made a client
	// retrying at the advertised Retry-After hit Traefik's raw "no available
	// server" instead of the wake interstitial (w6/m94).
	var waking networkingv1.Ingress
	if err := cl.Get(ctx, nn, &waking); err != nil {
		t.Fatal(err)
	}
	if backend := waking.Spec.Rules[0].HTTP.Paths[0].Backend.Service; backend.Name == app.Name {
		t.Fatalf("backend swapped to %q before any pod was ready", backend.Name)
	}

	// The Deployment reports a ready pod — what the deployment controller does
	// once the workload is actually up, and what the operator watches for
	// (Owns(Deployment) uses ResourceVersionChangedPredicate precisely so a
	// status-only change like this re-reconciles).
	markDeploymentReady(t, cl, nn)
	reconcileTwice(t, r, nn)

	var ing networkingv1.Ingress
	if err := cl.Get(ctx, nn, &ing); err != nil {
		t.Fatal(err)
	}
	if backend := ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service; backend.Name != app.Name {
		t.Fatalf("woken backend = %+v, want the App's own Service %q", backend, app.Name)
	}
	var dep appsv1.Deployment
	if err := cl.Get(ctx, nn, &dep); err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Replicas == nil || *dep.Spec.Replicas != 1 {
		t.Fatalf("woken replicas = %v, want 1", dep.Spec.Replicas)
	}
}

// markDeploymentReady reports one ready replica on the App's Deployment, the
// state the deployment controller writes once a pod passes its probes. The fake
// client runs no controllers, so a test that depends on readiness must say so.
func markDeploymentReady(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	dep.Status.ReadyReplicas = 1
	dep.Status.AvailableReplicas = 1
	dep.Status.Replicas = 1
	if err := cl.Status().Update(context.Background(), &dep); err != nil {
		t.Fatal(err)
	}
}

// activeApp is hibernatingApp's awake sibling: same free-tier, auto-sleep
// eligible service, but last seen just now, so it is serving rather than idle.
func activeApp(namespace string) *appv1alpha1.App {
	app := hibernatingApp(namespace)
	app.Annotations[annotLastActive] = time.Now().UTC().Format(time.RFC3339)
	app.Spec.IdleTTLSeconds = 300
	return app
}

func wakeReconciler(cl client.Client, scheme *runtime.Scheme) *AppReconciler {
	return &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co",
		ActivatorService: "bex-activator", ActivatorNamespace: "bex-system", ActivatorPort: 8888,
	}
}

func wakeScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return scheme
}

func ingressBackendName(t *testing.T, cl client.Client, nn types.NamespacedName) string {
	t.Helper()
	var ing networkingv1.Ingress
	if err := cl.Get(context.Background(), nn, &ing); err != nil {
		t.Fatal(err)
	}
	return ing.Spec.Rules[0].HTTP.Paths[0].Backend.Service.Name
}

func deploymentReplicas(t *testing.T, cl client.Client, nn types.NamespacedName) int32 {
	t.Helper()
	var dep appsv1.Deployment
	if err := cl.Get(context.Background(), nn, &dep); err != nil {
		t.Fatal(err)
	}
	if dep.Spec.Replicas == nil {
		return 1
	}
	return *dep.Spec.Replicas
}

// TestHibernateSwapsRouteBeforeDrainingPods is w6/m94's hibernate-direction
// fix. Scaling to 0 and swapping the Ingress to the activator are two separate,
// non-atomic API writes, and Traefik ingests them independently — so doing both
// in one pass leaves a window where the route still names the App's Service
// while its last endpoint is going away, and a request in that window gets
// Traefik's own "no available server" instead of bex's interstitial.
//
// The operator cannot observe Traefik's config propagation, so it buys time
// instead: write the route, keep the pods, come back. The App is already idle
// for its whole TTL, so the wait costs nothing.
func TestHibernateSwapsRouteBeforeDrainingPods(t *testing.T) {
	scheme := wakeScheme()
	app := activeApp("tea-abc123")
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	// Serving normally: the route is the App's own Service.
	reconcileTwice(t, r, nn)
	markDeploymentReady(t, cl, nn)
	reconcileTwice(t, r, nn)
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("serving backend = %q, want the App's own Service %q", got, app.Name)
	}

	// Now it goes idle past its TTL.
	var live appv1alpha1.App
	if err := cl.Get(context.Background(), nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Annotations[annotLastActive] = time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)
	if err := cl.Update(context.Background(), &live); err != nil {
		t.Fatal(err)
	}

	// The hibernate pass moves the route and NOTHING else: the pods that are
	// still serving must outlive the route change, not disappear beside it.
	res, err := r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn})
	if err != nil {
		t.Fatalf("reconcile: %v", err)
	}
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("backend = %q, want the activator alias — the route must move first", got)
	}
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("replicas = %d, want the pods kept until the route has propagated", got)
	}
	if res.RequeueAfter != hibernateRoutingGrace {
		t.Fatalf("requeueAfter = %v, want the routing grace %v", res.RequeueAfter, hibernateRoutingGrace)
	}

	// The grace has passed (the requeue fires): now it is safe to drain.
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want 0 once the activator route is live", got)
	}
	if got := ingressBackendName(t, cl, nn); got != activatorAliasName(app.Name) {
		t.Fatalf("backend = %q, want the activator alias to stay", got)
	}
}

// TestMaintenanceHibernatesWithoutRoutingHold: maintenance owns the public
// route, so the App's Service is not what a visitor reaches and draining it
// cannot surface a raw 503. Holding there would also never resolve — the
// Ingress will never name the activator while maintenance owns it — so the
// workload would stop hibernating entirely, breaking the independence of
// routing and replica policy that ingressBackend documents.
func TestMaintenanceHibernatesWithoutRoutingHold(t *testing.T) {
	scheme := wakeScheme()
	app := hibernatingApp("tea-abc123")
	app.Spec.MaintenanceMode = &appv1alpha1.MaintenanceModeSpec{Enabled: true}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	r.MaintenanceService, r.MaintenanceNamespace, r.MaintenancePort = "bex-maintenance", "bex-system", 8080
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	reconcileTwice(t, r, nn)

	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want 0 — maintenance routing must not block auto-sleep", got)
	}
}
