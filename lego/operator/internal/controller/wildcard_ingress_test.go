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

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestWildcardDomainGetsItsOwnIngress (w4/m326): a wildcard custom domain is
// routed by its own Ingress, at the lowest router priority so a concrete host
// under it keeps its own route, with its certificate from the DNS-01 issuer.
// The concrete hosts stay on the serving Ingress and its HTTP-01 issuer, with
// their certificate Secrets unchanged. Dropping the wildcard deletes its
// Ingress.
func TestWildcardDomainGetsItsOwnIngress(t *testing.T) {
	ctx := context.Background()
	scheme := newHostRedirectScheme()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, ClusterIssuer: "letsencrypt-prod", WildcardClusterIssuer: "letsencrypt-dns01-delegated"}
	hosts := []string{"app.onbex.co", "*.example.com", "api.example.com"}
	if err := r.reconcileIngress(ctx, app, hosts, "app", 3000, []string{"app-ipallow"}); err != nil {
		t.Fatalf("reconcileIngress: %v", err)
	}

	var main, wild networkingv1.Ingress
	if err := cl.Get(ctx, types.NamespacedName{Name: "app", Namespace: "default"}, &main); err != nil {
		t.Fatal(err)
	}
	if got := ingressHosts(&main); len(got) != 2 || got[0] != "app.onbex.co" || got[1] != "api.example.com" {
		t.Errorf("serving Ingress hosts = %v, want the concrete hosts", got)
	}
	if main.Annotations[certManagerClusterIssuerAnnotation] != "letsencrypt-prod" || main.Annotations[traefikRouterPriorityAnnotation] != "" {
		t.Errorf("serving Ingress annotations = %v", main.Annotations)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: wildcardIngressName("app"), Namespace: "default"}, &wild); err != nil {
		t.Fatalf("wildcard Ingress: %v", err)
	}
	if got := ingressHosts(&wild); len(got) != 1 || got[0] != "*.example.com" {
		t.Errorf("wildcard Ingress hosts = %v", got)
	}
	if len(wild.Spec.TLS) != 1 || wild.Spec.TLS[0].SecretName != appv1alpha1.TLSSecretName("app", false, "*.example.com") {
		t.Errorf("wildcard TLS = %+v", wild.Spec.TLS)
	}
	if wild.Annotations[certManagerClusterIssuerAnnotation] != "letsencrypt-dns01-delegated" || wild.Annotations[traefikRouterPriorityAnnotation] != "1" {
		t.Errorf("wildcard Ingress annotations = %v", wild.Annotations)
	}
	// The App's middlewares (an IP allow list here) apply to every host.
	if wild.Annotations[traefikRouterMiddlewaresAnnotation] != main.Annotations[traefikRouterMiddlewaresAnnotation] {
		t.Errorf("wildcard middlewares %q, serving %q", wild.Annotations[traefikRouterMiddlewaresAnnotation], main.Annotations[traefikRouterMiddlewaresAnnotation])
	}

	if err := r.reconcileIngress(ctx, app, []string{"app.onbex.co"}, "app", 3000, nil); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: wildcardIngressName("app"), Namespace: "default"}, &wild); !apierrors.IsNotFound(err) {
		t.Errorf("wildcard Ingress after the wildcard was removed: %v", err)
	}
}

// Without a DNS-01 issuer a wildcard routes without a certificate request:
// HTTP-01 could never issue one.
func TestWildcardIngressWithoutDNS01IssuerRequestsNoCertificate(t *testing.T) {
	ctx := context.Background()
	scheme := newHostRedirectScheme()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, ClusterIssuer: "letsencrypt-prod"}
	if err := r.reconcileIngress(ctx, app, []string{"*.example.com"}, "app", 3000, nil); err != nil {
		t.Fatal(err)
	}
	var main, wild networkingv1.Ingress
	if err := cl.Get(ctx, types.NamespacedName{Name: "app", Namespace: "default"}, &main); !apierrors.IsNotFound(err) {
		t.Errorf("serving Ingress with only a wildcard host: %v", err)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: wildcardIngressName("app"), Namespace: "default"}, &wild); err != nil {
		t.Fatal(err)
	}
	if _, ok := wild.Annotations[certManagerClusterIssuerAnnotation]; ok {
		t.Errorf("wildcard Ingress requests a certificate: %v", wild.Annotations)
	}
}

func ingressHosts(ing *networkingv1.Ingress) []string {
	hosts := make([]string, 0, len(ing.Spec.Rules))
	for _, rule := range ing.Spec.Rules {
		hosts = append(hosts, rule.Host)
	}
	return hosts
}

// setStatusURLs names no wildcard: it is no single site (w4/m326).
func TestStatusURLsLeaveWildcardsOut(t *testing.T) {
	app := &appv1alpha1.App{}
	setStatusURLs(app, []string{"*.example.com", "app.onbex.co", "api.example.com"})
	if app.Status.URL != "https://app.onbex.co" || len(app.Status.URLs) != 2 {
		t.Errorf("URL %q, URLs %v", app.Status.URL, app.Status.URLs)
	}
}

// A health probe never sends a wildcard as its Host: it names no host a
// request could carry, so the next custom host, else the platform host, is
// probed (w4/m326).
func TestHealthProbeHostSkipsAWildcard(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "app"},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeWebService, Expose: true, Hosts: []string{"*.example.com", "api.example.com"}}}
	if got := healthProbeHost(app, "onbex.co"); got != "api.example.com" {
		t.Errorf("healthProbeHost = %q, want api.example.com", got)
	}
	app.Spec.Hosts = []string{"*.example.com"}
	if got := healthProbeHost(app, "onbex.co"); got != "app.onbex.co" {
		t.Errorf("healthProbeHost with only a wildcard = %q, want the platform host", got)
	}
}

// The wildcard Ingress's name can never be another App's serving Ingress:
// App names carry no dot.
func TestWildcardIngressNameCannotBeAnAppName(t *testing.T) {
	if got := wildcardIngressName("tea-x"); got != "tea-x.wildcard" {
		t.Errorf("wildcardIngressName = %q", got)
	}
}
