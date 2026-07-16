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

// ip_allow_list_test.go pins the HTTP-layer ipAllowList middleware contract
// (w7/m32) for web_service and static_site Apps using a fake client (no envtest
// — the Traefik CRDs are not installed in the envtest environment, just as they
// aren't in TestKeyValueIPAllowListProjection). Covers: middleware creation with
// correct CIDRs, Ingress annotation wiring, middleware deletion on clear, and
// no-op for types without a public Ingress.

import (
	"context"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// appWithAllowList builds a minimal web_service App CR for a fake-client
// middleware test.
func appWithAllowList(name string, cidrs []string, typ string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec: appv1alpha1.AppSpec{
			Type:        typ,
			Image:       "nginx:v1",
			Expose:      typ == appv1alpha1.TypeWebService || typ == appv1alpha1.TypeStaticSite,
			IPAllowList: cidrs,
		},
	}
}

// newIPAllowListScheme registers the minimal types needed for a fake client
// to handle HTTP Middleware + Ingress writes (no envtest, no Traefik CRDs
// installed — only the runtime.Scheme registration matters here).
func newIPAllowListScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(traefikHTTPMiddlewareGVK, &unstructured.Unstructured{})
	return scheme
}

// TestWebServiceIPAllowListMiddlewareProjection drives the full reconcile for a
// web_service App with an ipAllowList: middleware is created with the correct
// sourceRange, the Ingress carries the middleware annotation, clearing the list
// removes the middleware and the annotation.
func TestWebServiceIPAllowListMiddlewareProjection(t *testing.T) {
	scheme := newIPAllowListScheme()
	app := appWithAllowList("ws-acl", []string{"203.0.113.0/24", "10.0.0.0/8"}, appv1alpha1.TypeWebService)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes,
		BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
	}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "ws-acl", Namespace: "default"}
	mwNN := types.NamespacedName{Name: "ws-acl-ip-allow", Namespace: "default"}
	ingressNN := types.NamespacedName{Name: "ws-acl", Namespace: "default"}

	reconcile1 := func() {
		t.Helper()
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
	getMW := func() (*unstructured.Unstructured, error) {
		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
		return o, cl.Get(ctx, mwNN, o)
	}
	getIngress := func() (*networkingv1.Ingress, error) {
		ing := &networkingv1.Ingress{}
		return ing, cl.Get(ctx, ingressNN, ing)
	}

	// --- allowlist set ---
	// Pass 1: adds the finalizer. Pass 2: reconcileKubernetes runs and creates the Middleware.
	reconcile1()
	reconcile1()

	mw, err := getMW()
	if err != nil {
		t.Fatalf("Middleware not created: %v", err)
	}
	ranges, _, _ := unstructured.NestedSlice(mw.Object, "spec", "ipAllowList", "sourceRange")
	if len(ranges) != 2 || ranges[0] != "203.0.113.0/24" || ranges[1] != "10.0.0.0/8" {
		t.Fatalf("Middleware.spec.ipAllowList.sourceRange = %v, want [203.0.113.0/24 10.0.0.0/8]", ranges)
	}

	ing, err := getIngress()
	if err != nil {
		t.Fatalf("Ingress not created: %v", err)
	}
	wantAnnotation := "default-ws-acl-ip-allow@kubernetescrd"
	if got := ing.Annotations[traefikRouterMiddlewaresAnnotation]; got != wantAnnotation {
		t.Fatalf("Ingress annotation %s = %q, want %q", traefikRouterMiddlewaresAnnotation, got, wantAnnotation)
	}

	// --- clear the allowlist ---
	if err := cl.Get(ctx, nn, app); err != nil {
		t.Fatalf("get app: %v", err)
	}
	app.Spec.IPAllowList = nil
	if err := cl.Update(ctx, app); err != nil {
		t.Fatalf("clear allowlist: %v", err)
	}
	reconcile1()

	if _, err := getMW(); !apierrors.IsNotFound(err) {
		t.Fatalf("Middleware must be deleted when allowlist is empty, got %v", err)
	}
	ing, err = getIngress()
	if err != nil {
		t.Fatalf("Ingress must survive after allowlist is cleared: %v", err)
	}
	if _, has := ing.Annotations[traefikRouterMiddlewaresAnnotation]; has {
		t.Fatalf("Ingress must lose the middleware annotation after allowlist is cleared: %v", ing.Annotations)
	}
}

// TestStaticSiteIPAllowListMiddlewareProjection verifies that
// reconcileIPAllowListMiddleware — the same function called for both
// reconcileKubernetes and reconcileStaticSite — creates and removes the HTTP
// Middleware for a static_site App. The full reconcileStaticSite path is tested
// in the envtest Ginkgo suite; here we drive the shared reconcile primitive
// directly to avoid the S3 dependency that would abort reconcileStaticSite
// before reaching the middleware step.
func TestStaticSiteIPAllowListMiddlewareProjection(t *testing.T) {
	scheme := newIPAllowListScheme()
	app := appWithAllowList("site-acl", []string{"203.0.113.0/24"}, appv1alpha1.TypeStaticSite)
	app.Spec.PublishPath = "dist"
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{
		Client: cl, Scheme: scheme, Mode: ModeKubernetes,
		BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
	}
	ctx := context.Background()
	mwNN := types.NamespacedName{Name: "site-acl-ip-allow", Namespace: "default"}

	getMW := func() (*unstructured.Unstructured, error) {
		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
		return o, cl.Get(ctx, mwNN, o)
	}

	// Phase 1: allowlist set → middleware created.
	mwNames, err := r.reconcileIPAllowListMiddleware(ctx, app)
	if err != nil {
		t.Fatalf("reconcileIPAllowListMiddleware: %v", err)
	}
	if len(mwNames) != 1 || mwNames[0] != "site-acl-ip-allow" {
		t.Fatalf("mwNames = %v, want [site-acl-ip-allow]", mwNames)
	}
	mw, err := getMW()
	if err != nil {
		t.Fatalf("Middleware not created for static_site: %v", err)
	}
	ranges, _, _ := unstructured.NestedSlice(mw.Object, "spec", "ipAllowList", "sourceRange")
	if len(ranges) != 1 || ranges[0] != "203.0.113.0/24" {
		t.Fatalf("Middleware.spec.ipAllowList.sourceRange = %v, want [203.0.113.0/24]", ranges)
	}

	// Phase 2: clear → middleware removed and no names remain.
	app.Spec.IPAllowList = nil
	mwNames, err = r.reconcileIPAllowListMiddleware(ctx, app)
	if err != nil {
		t.Fatalf("reconcileIPAllowListMiddleware(clear): %v", err)
	}
	if len(mwNames) != 0 {
		t.Fatalf("cleared mwNames = %v, want empty", mwNames)
	}
	if _, err := getMW(); !apierrors.IsNotFound(err) {
		t.Fatalf("Middleware must be deleted when allowlist is empty for static_site, got %v", err)
	}
}

// TestNonIngressTypesGetNoIPAllowListMiddleware proves that background_worker
// and cron_job (no public Ingress) never trigger the middleware reconcile —
// the ipAllowList field on their spec is accepted by the CRD but ignored at
// reconcile time.
func TestNonIngressTypesGetNoIPAllowListMiddleware(t *testing.T) {
	for _, typ := range []string{
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
	} {
		t.Run(typ, func(t *testing.T) {
			scheme := newIPAllowListScheme()
			app := appWithAllowList("no-ingress", []string{"203.0.113.0/24"}, typ)
			if typ == appv1alpha1.TypeCronJob {
				app.Spec.Schedule = "0 * * * *"
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
				WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{
				Client: cl, Scheme: scheme, Mode: ModeKubernetes,
				BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			}
			ctx := context.Background()
			nn := types.NamespacedName{Name: "no-ingress", Namespace: "default"}
			mwNN := types.NamespacedName{Name: "no-ingress-ip-allow", Namespace: "default"}

			if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
				t.Fatalf("reconcile: %v", err)
			}

			o := &unstructured.Unstructured{}
			o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
			if err := cl.Get(ctx, mwNN, o); !apierrors.IsNotFound(err) {
				t.Fatalf("%s with ipAllowList must not create an HTTP Middleware (no public Ingress): got %v", typ, err)
			}
		})
	}
}

// TestEnvironmentIPAllowListChaining pins w4/m28's layered composition: an App
// carrying BOTH its own ipAllowList and an environment-projected
// environmentIPAllowList renders two middlewares, and the Ingress annotation
// chains them (comma-joined — Traefik ANDs a chain, so a source must pass
// every layer: Render's every-applicable-rule intersection). Table-driven over
// the layer combinations.
func TestEnvironmentIPAllowListChaining(t *testing.T) {
	cases := []struct {
		name      string
		own, env  []string
		wantAnnot string
		wantOwnMW bool
		wantEnvMW bool
	}{
		{"both layers chain", []string{"10.0.0.0/8"}, []string{"203.0.113.0/24"},
			"default-chain-acl-ip-allow@kubernetescrd,default-chain-acl-env-ip-allow@kubernetescrd", true, true},
		{"own only", []string{"10.0.0.0/8"}, nil,
			"default-chain-acl-ip-allow@kubernetescrd", true, false},
		{"env only", nil, []string{"203.0.113.0/24"},
			"default-chain-acl-env-ip-allow@kubernetescrd", false, true},
		{"env deny-all placeholder", nil, []string{"255.255.255.255/32"},
			"default-chain-acl-env-ip-allow@kubernetescrd", false, true},
		{"no layers", nil, nil, "", false, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := newIPAllowListScheme()
			app := appWithAllowList("chain-acl", tc.own, appv1alpha1.TypeWebService)
			app.Spec.EnvironmentIPAllowList = tc.env
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
				WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{
				Client: cl, Scheme: scheme, Mode: ModeKubernetes,
				BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			}
			ctx := context.Background()
			// Pass 1 adds the finalizer; pass 2 reconciles the workloads.
			for range 2 {
				if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: "chain-acl", Namespace: "default"}}); err != nil {
					t.Fatalf("Reconcile: %v", err)
				}
			}

			getMW := func(name string) error {
				o := &unstructured.Unstructured{}
				o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
				return cl.Get(ctx, types.NamespacedName{Name: name, Namespace: "default"}, o)
			}
			if err := getMW("chain-acl-ip-allow"); (err == nil) != tc.wantOwnMW {
				t.Errorf("own middleware present=%v, want %v (err %v)", err == nil, tc.wantOwnMW, err)
			}
			if err := getMW("chain-acl-env-ip-allow"); (err == nil) != tc.wantEnvMW {
				t.Errorf("env middleware present=%v, want %v (err %v)", err == nil, tc.wantEnvMW, err)
			}
			ing := &networkingv1.Ingress{}
			if err := cl.Get(ctx, types.NamespacedName{Name: "chain-acl", Namespace: "default"}, ing); err != nil {
				t.Fatalf("Ingress: %v", err)
			}
			if got := ing.Annotations[traefikRouterMiddlewaresAnnotation]; got != tc.wantAnnot {
				t.Errorf("annotation = %q, want %q", got, tc.wantAnnot)
			}
		})
	}
}

// TestDatastoreEnvironmentLayerChaining pins the TCP side of w4/m28: a
// KeyValue with both its own entry list and an environment layer renders two
// MiddlewareTCPs, both referenced by the SNI route.
func TestDatastoreEnvironmentLayerChaining(t *testing.T) {
	spec := ipAllowListMiddlewareSpec([]appv1alpha1.IPAllowEntry{{CIDR: "10.0.0.0/8", Description: "office"}})
	ranges, _, _ := unstructured.NestedStringSlice(map[string]any{"spec": spec}, "spec", "ipAllowList", "sourceRange")
	if len(ranges) != 1 || ranges[0] != "10.0.0.0/8" {
		t.Fatalf("entries spec = %v", spec)
	}
	envSpec := cidrMiddlewareSpec([]string{"255.255.255.255/32"})
	envRanges, _, _ := unstructured.NestedStringSlice(map[string]any{"spec": envSpec}, "spec", "ipAllowList", "sourceRange")
	if len(envRanges) != 1 || envRanges[0] != "255.255.255.255/32" {
		t.Fatalf("env spec = %v", envSpec)
	}
	// Route-level chaining: both middlewares referenced.
	route := ingressRouteTCPSpec("postgres", "db.example.com", "db-rw", 5432, []any{
		map[string]any{"name": "db-allow", "namespace": "ns"},
		map[string]any{"name": "db-env-allow", "namespace": "ns"},
	})
	routes, _ := route["routes"].([]any)
	if len(routes) != 1 {
		t.Fatalf("routes = %v", routes)
	}
	mws, _ := routes[0].(map[string]any)["middlewares"].([]any)
	if len(mws) != 2 {
		t.Fatalf("route middlewares = %v, want both layers chained", mws)
	}
}
