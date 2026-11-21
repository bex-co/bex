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
	"crypto/sha256"
	"fmt"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// requireDistinctTraefikRouters models how Traefik v3.7.17's Kubernetes
// Ingress provider builds its routers from every Ingress
// (pkg/provider/kubernetes/ingress loadConfigurationFromIngresses). It builds
// one router per path, keyed by v1alpha1.TraefikRouterKey, and appends a hash
// of the rule to a key that repeats inside one Ingress. Across Ingresses, a
// later router with the same key replaces the earlier one. The test fails when
// two Ingresses' routes share a key, because one of their hosts would lose its
// route, or when a host in hosts has no route at all.
func requireDistinctTraefikRouters(t *testing.T, cl client.Client, hosts ...string) {
	t.Helper()
	var list networkingv1.IngressList
	if err := cl.List(context.Background(), &list); err != nil {
		t.Fatal(err)
	}
	routes := map[string]string{} // router key -> Ingress/host it routes
	routed := map[string]bool{}
	for _, ing := range list.Items {
		counts := map[string]int{}
		for _, rule := range ing.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				counts[appv1alpha1.TraefikRouterKey(ing.Namespace, ing.Name, rule.Host, path.Path)]++
			}
		}
		for _, rule := range ing.Spec.Rules {
			for _, path := range rule.HTTP.Paths {
				key := appv1alpha1.TraefikRouterKey(ing.Namespace, ing.Name, rule.Host, path.Path)
				if counts[key] > 1 {
					key = fmt.Sprintf("%s-%.10x", key, sha256.Sum256(fmt.Appendf(nil, "Host(%q) && PathPrefix(%q)", rule.Host, path.Path)))
				}
				route := ing.Namespace + "/" + ing.Name + " " + rule.Host
				if other, ok := routes[key]; ok && other != route {
					t.Fatalf("Traefik router %q serves both %s and %s: one of them goes unrouted", key, other, route)
				}
				routes[key] = route
				routed[rule.Host] = true
			}
		}
	}
	for _, host := range hosts {
		if !routed[host] {
			t.Errorf("%s has no route", host)
		}
	}
}

// ingressPaths is how many paths route host on the Ingress name.
func ingressPaths(t *testing.T, cl client.Client, namespace, name, host string) int {
	t.Helper()
	var ing networkingv1.Ingress
	if err := cl.Get(context.Background(), types.NamespacedName{Namespace: namespace, Name: name}, &ing); err != nil {
		t.Fatal(err)
	}
	for _, rule := range ing.Spec.Rules {
		if rule.Host == host {
			return len(rule.HTTP.Paths)
		}
	}
	t.Fatalf("Ingress %s/%s does not route %s", namespace, name, host)
	return 0
}

// A concrete "wildcard.<base>" beside the App's own "*.<base>" gives the
// serving Ingress and the wildcard Ingress one Traefik router key. Whichever
// Ingress Traefik read last kept the only router, so either the concrete host
// or every name under the wildcard went unrouted.
func TestWildcardAndItsWildcardLabelGetTheirOwnTraefikRouters(t *testing.T) {
	ctx := context.Background()
	scheme := newHostRedirectScheme()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, ClusterIssuer: "letsencrypt-prod", WildcardClusterIssuer: "letsencrypt-dns01-delegated"}
	hosts := []string{"app.onbex.co", "wildcard.example.com", "*.example.com"}
	for pass := 1; pass <= 3; pass++ {
		if err := r.reconcileIngress(ctx, app, hosts, "app", 3000, nil); err != nil {
			t.Fatalf("pass %d: %v", pass, err)
		}
		requireDistinctTraefikRouters(t, cl, hosts...)
	}
	// A host no other Ingress's key matches keeps its single path, and so its
	// router name and metric series.
	if n := ingressPaths(t, cl, "default", "app", "app.onbex.co"); n != 1 {
		t.Errorf("app.onbex.co has %d paths, want 1", n)
	}
}

// App "web" serving "api.example.com" and App "web-api" serving
// "example.com" in one namespace give one Traefik router key. The two hosts
// took turns answering 404 as Traefik rebuilt its configuration. Once the
// other App goes, the route returns to a single path.
func TestAppsWhoseNamesAndHostsNormalizeAlikeGetTheirOwnTraefikRouters(t *testing.T) {
	ctx := context.Background()
	scheme := newHostRedirectScheme()
	const ns = "tea-d0000000000000000000"
	web := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: ns + "-web", Namespace: ns}}
	webAPI := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: ns + "-web-api", Namespace: ns}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(web, webAPI).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, ClusterIssuer: "letsencrypt-prod"}
	webHosts := []string{"web.onbex.co", "api.example.com"}
	reconcile := func(app *appv1alpha1.App, hosts ...string) {
		t.Helper()
		if err := r.reconcileIngress(ctx, app, hosts, app.Name, 10000, nil); err != nil {
			t.Fatal(err)
		}
	}

	reconcile(web, webHosts...)
	reconcile(webAPI, "example.com")
	requireDistinctTraefikRouters(t, cl, "web.onbex.co", "api.example.com", "example.com")
	reconcile(web, webHosts...)
	requireDistinctTraefikRouters(t, cl, "web.onbex.co", "api.example.com", "example.com")

	reconcile(webAPI)
	reconcile(web, webHosts...)
	if n := ingressPaths(t, cl, ns, web.Name, "api.example.com"); n != 1 {
		t.Errorf("api.example.com has %d paths once web-api is gone, want 1", n)
	}
}
