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
	"strings"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m118: Render redirects every HTTP request to HTTPS. A web service's
// Ingress leads its middleware chain with an owned RedirectScheme, permanent,
// to https — Traefik applies the chain to the plain-HTTP router and its TLS
// sibling alike, and passes a request already on https through. A cluster that
// turns the redirect off (BEX_HTTPS_REDIRECT=false) loses the middleware.
func TestWebServiceIngressRedirectsHTTPToHTTPS(t *testing.T) {
	scheme := newIPAllowListScheme()
	app := appWithAllowList("plain", nil, appv1alpha1.TypeWebService)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod", HTTPSRedirect: true}
	ctx := context.Background()
	nn := types.NamespacedName{Name: "plain", Namespace: "default"}
	pass := func() {
		t.Helper()
		if _, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}); err != nil {
			t.Fatalf("reconcile: %v", err)
		}
	}
	redirect := func() (*unstructured.Unstructured, error) {
		o := &unstructured.Unstructured{}
		o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
		return o, cl.Get(ctx, types.NamespacedName{Name: "plain-https", Namespace: "default"}, o)
	}
	chain := func() string {
		t.Helper()
		ing := &networkingv1.Ingress{}
		if err := cl.Get(ctx, nn, ing); err != nil {
			t.Fatalf("Ingress: %v", err)
		}
		return ing.Annotations[traefikRouterMiddlewaresAnnotation]
	}

	pass()
	pass()
	mw, err := redirect()
	if err != nil {
		t.Fatalf("the HTTPS redirect middleware was not created: %v", err)
	}
	spec, _, _ := unstructured.NestedMap(mw.Object, "spec", "redirectScheme")
	if spec["scheme"] != "https" || spec["permanent"] != true {
		t.Errorf("redirectScheme = %v, want scheme https, permanent", spec)
	}
	if owners := mw.GetOwnerReferences(); len(owners) != 1 || owners[0].Name != "plain" || owners[0].Controller == nil || !*owners[0].Controller {
		t.Errorf("owners = %+v, want the App as controller", owners)
	}
	if got := chain(); !strings.HasPrefix(got, "default-plain-https@kubernetescrd,") {
		t.Errorf("middleware chain = %q, want the redirect first", got)
	}

	r.HTTPSRedirect = false
	pass()
	if _, err := redirect(); !apierrors.IsNotFound(err) {
		t.Errorf("turned off, the redirect must go, got %v", err)
	}
	if got := chain(); strings.Contains(got, "-https@") {
		t.Errorf("turned off, the chain still redirects: %q", got)
	}
}

// The Ingress writer adds the redirect whatever chain its caller passes — the
// activator route of a hibernated service passes none — and a service that
// stops serving hosts drops the redirect with its Ingress.
func TestEveryAppIngressRedirectsAndDropsTheRedirectWithItsHosts(t *testing.T) {
	scheme := newIPAllowListScheme()
	app := appWithAllowList("sleepy", nil, appv1alpha1.TypeWebService)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod", HTTPSRedirect: true}
	ctx := context.Background()
	hosts := []string{"sleepy.onbex.co"}

	if err := r.reconcileIngress(ctx, app, hosts, "activator", 8080, nil); err != nil {
		t.Fatal(err)
	}
	ing := &networkingv1.Ingress{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "sleepy", Namespace: "default"}, ing); err != nil {
		t.Fatal(err)
	}
	if got := ing.Annotations[traefikRouterMiddlewaresAnnotation]; got != "default-sleepy-https@kubernetescrd" {
		t.Errorf("activator route chain = %q, want just the redirect", got)
	}

	if err := r.reconcileIngress(ctx, app, nil, "activator", 8080, nil); err != nil {
		t.Fatal(err)
	}
	o := &unstructured.Unstructured{}
	o.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
	if err := cl.Get(ctx, types.NamespacedName{Name: "sleepy-https", Namespace: "default"}, o); !apierrors.IsNotFound(err) {
		t.Errorf("with no hosts the redirect must go, got %v", err)
	}
}
