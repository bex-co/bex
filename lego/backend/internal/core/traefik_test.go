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

package core

import (
	"context"
	"regexp"
	"strings"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func traefikTestIngress(name, backend string, hosts ...string) *networkingv1.Ingress {
	class := traefikIngressClass
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Spec:       networkingv1.IngressSpec{IngressClassName: &class},
	}
	for _, host := range hosts {
		ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: backend,
					}},
				}},
			}},
		})
	}
	return ingress
}

func TestTraefikRouterNamesUseIngressIdentityNotBackendService(t *testing.T) {
	ingress := traefikTestIngress("tea-one-static", "shared-static-server", "site.onbex.co", "www.example.com")
	names, err := TraefikRouterNamesForIngress(ingress)
	if err != nil {
		t.Fatalf("TraefikRouterNamesForIngress: %v", err)
	}
	want := []string{
		"default-tea-one-static-site-onbex-co@kubernetes",
		"default-tea-one-static-www-example-com@kubernetes",
	}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Fatalf("router names: got %v, want %v", names, want)
	}
}

func TestTraefikRouterNamesPinNormalizedCollisionHashes(t *testing.T) {
	ingress := traefikTestIngress("web", "web", "foo.bar", "foo-bar")
	names, err := TraefikRouterNamesForIngress(ingress)
	if err != nil {
		t.Fatalf("TraefikRouterNamesForIngress: %v", err)
	}
	want := []string{
		"default-web-foo-bar-beb37b712c8473ef7afa@kubernetes",
		"default-web-foo-bar-ef6c149f61f9c8903f31@kubernetes",
	}
	if len(names) != len(want) || names[0] != want[0] || names[1] != want[1] {
		t.Fatalf("collision names: got %v, want pinned Traefik v3.7.5 names %v", names, want)
	}
}

func TestTraefikRouterMatcherDoesNotOverlapSimilarNames(t *testing.T) {
	matcher := TraefikRouterMatcher([]string{
		"default-web-web-onbex-co@kubernetes",
		"default-web-web-example-com@kubernetes",
	})
	re := regexp.MustCompile(matcher)
	if !re.MatchString("default-web-web-onbex-co@kubernetes") {
		t.Fatalf("matcher %q did not match its exact router", matcher)
	}
	for _, other := range []string{
		"default-web-api-web-onbex-co@kubernetes",
		"default-web-web-onbex-co-extra@kubernetes",
		"prefix-default-web-web-onbex-co@kubernetes",
	} {
		if re.MatchString(other) {
			t.Errorf("matcher %q crossed into %q", matcher, other)
		}
	}
}

func TestTraefikRouterNamesDistinguishPrivateZeroFromMissingPublicIngress(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	base := &Base{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Namespace: "default"}

	private := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "worker", Namespace: "default"}}
	names, err := base.TraefikRouterNames(context.Background(), private)
	if err != nil || len(names) != 0 {
		t.Fatalf("private App: got names=%v err=%v, want successful empty", names, err)
	}

	public := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Host: "web.example.com"},
	}
	if _, err := base.TraefikRouterNames(context.Background(), public); err == nil || !strings.Contains(err.Error(), "expected Ingress") {
		t.Fatalf("public App missing Ingress: got %v, want unresolved error", err)
	}
}

func TestTraefikRouterNamesRejectCrossIngressNormalizedCollision(t *testing.T) {
	scheme := runtime.NewScheme()
	if err := networkingv1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	requested := traefikTestIngress("web", "web", "api-x.example.com")
	other := traefikTestIngress("web-api", "web-api", "x.example.com")
	base := &Base{
		Client:    fake.NewClientBuilder().WithScheme(scheme).WithObjects(requested, other).Build(),
		Namespace: "default",
	}
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{Host: "api-x.example.com"},
	}
	if _, err := base.TraefikRouterNames(context.Background(), app); err == nil || !strings.Contains(err.Error(), "ambiguous") {
		t.Fatalf("cross-Ingress normalized collision: got %v, want ambiguity error", err)
	}
}
