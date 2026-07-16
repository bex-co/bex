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
	"regexp"
	"testing"

	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func newHostRedirectScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	scheme.AddKnownTypeWithName(traefikHTTPMiddlewareGVK, &unstructured.Unstructured{})
	scheme.AddKnownTypeWithName(schema.GroupVersionKind{
		Group: traefikHTTPMiddlewareGVK.Group, Version: traefikHTTPMiddlewareGVK.Version, Kind: "MiddlewareList",
	}, &unstructured.UnstructuredList{})
	return scheme
}

func TestHostRedirectProjectionMatrix(t *testing.T) {
	cases := []struct {
		name      string
		hosts     []string
		redirects map[string]string
		source    string
		target    string
	}{
		{
			name:      "apex explicitly added redirects www",
			hosts:     []string{"app.onbex.co", "example.com", "www.example.com"},
			redirects: map[string]string{"www.example.com": "example.com"},
			source:    "www.example.com", target: "example.com",
		},
		{
			name:      "www explicitly added redirects apex",
			hosts:     []string{"app.onbex.co", "www.example.com", "example.com"},
			redirects: map[string]string{"example.com": "www.example.com"},
			source:    "example.com", target: "www.example.com",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := newHostRedirectScheme()
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
				Spec:       appv1alpha1.AppSpec{HostRedirects: tc.redirects},
			}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
			r := &AppReconciler{Client: cl, Scheme: scheme, ClusterIssuer: "letsencrypt-prod"}
			if err := r.reconcileIngress(context.Background(), app, tc.hosts, "app", 3000, ""); err != nil {
				t.Fatalf("reconcileIngress: %v", err)
			}

			main := &networkingv1.Ingress{}
			if err := cl.Get(context.Background(), types.NamespacedName{Name: "app", Namespace: "default"}, main); err != nil {
				t.Fatalf("get serving Ingress: %v", err)
			}
			for _, rule := range main.Spec.Rules {
				if rule.Host == tc.source {
					t.Fatalf("redirect source %q must not remain on the serving router", tc.source)
				}
			}

			redirectIngress := &networkingv1.Ingress{}
			if err := cl.Get(context.Background(), types.NamespacedName{Name: hostRedirectIngressName("app"), Namespace: "default"}, redirectIngress); err != nil {
				t.Fatalf("get redirect Ingress: %v", err)
			}
			if len(redirectIngress.Spec.Rules) != 1 || redirectIngress.Spec.Rules[0].Host != tc.source {
				t.Fatalf("redirect rules = %+v, want source %q", redirectIngress.Spec.Rules, tc.source)
			}
			if len(redirectIngress.Spec.TLS) != 1 || redirectIngress.Spec.TLS[0].Hosts[0] != tc.source {
				t.Fatalf("redirect TLS = %+v, want certificate for %q", redirectIngress.Spec.TLS, tc.source)
			}
			wantMiddleware := "default-" + hostRedirectResourceName("app", tc.source) + "@kubernetescrd"
			if got := redirectIngress.Annotations[traefikRouterMiddlewaresAnnotation]; got != wantMiddleware {
				t.Fatalf("redirect Ingress middleware annotation = %q, want %q", got, wantMiddleware)
			}

			middleware := &unstructured.Unstructured{}
			middleware.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
			if err := cl.Get(context.Background(), types.NamespacedName{
				Name: hostRedirectResourceName("app", tc.source), Namespace: "default",
			}, middleware); err != nil {
				t.Fatalf("get redirect middleware: %v", err)
			}
			got, _, _ := unstructured.NestedMap(middleware.Object, "spec", "redirectRegex")
			if got["replacement"] != "https://"+tc.target+"/${1}" || got["permanent"] != true {
				t.Fatalf("redirectRegex = %v, want permanent redirect to %q preserving suffix", got, tc.target)
			}
		})
	}
}

func TestHostRedirectRemovalAndExplicitBothServeDirectly(t *testing.T) {
	scheme := newHostRedirectScheme()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "app", Namespace: "default"},
		Spec:       appv1alpha1.AppSpec{HostRedirects: map[string]string{"www.example.com": "example.com"}},
	}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}
	ctx := context.Background()
	hosts := []string{"example.com", "www.example.com"}
	if err := r.reconcileIngress(ctx, app, hosts, "app", 3000, ""); err != nil {
		t.Fatal(err)
	}

	// Explicitly claiming the auto-added sibling clears HostRedirects. Both
	// hosts move onto the plain serving Ingress and redirect artifacts disappear.
	app.Spec.HostRedirects = nil
	if err := r.reconcileIngress(ctx, app, hosts, "app", 3000, ""); err != nil {
		t.Fatal(err)
	}
	main := &networkingv1.Ingress{}
	if err := cl.Get(ctx, types.NamespacedName{Name: "app", Namespace: "default"}, main); err != nil {
		t.Fatal(err)
	}
	if len(main.Spec.Rules) != 2 {
		t.Fatalf("explicit-both serving rules = %d, want 2", len(main.Spec.Rules))
	}
	staleIngress := &networkingv1.Ingress{}
	if err := cl.Get(ctx, types.NamespacedName{Name: hostRedirectIngressName("app"), Namespace: "default"}, staleIngress); !apierrors.IsNotFound(err) {
		t.Fatalf("redirect Ingress must be removed, got %v", err)
	}
	staleMiddleware := &unstructured.Unstructured{}
	staleMiddleware.SetGroupVersionKind(traefikHTTPMiddlewareGVK)
	if err := cl.Get(ctx, types.NamespacedName{Name: hostRedirectResourceName("app", "www.example.com"), Namespace: "default"}, staleMiddleware); !apierrors.IsNotFound(err) {
		t.Fatalf("redirect Middleware must be removed, got %v", err)
	}

	// A stale mapping whose target was deleted is also ignored and cleaned: the
	// surviving source serves directly instead of redirecting to a dead host.
	app.Spec.HostRedirects = map[string]string{"www.example.com": "example.com"}
	if err := r.reconcileIngress(ctx, app, []string{"www.example.com"}, "app", 3000, ""); err != nil {
		t.Fatal(err)
	}
	if err := cl.Get(ctx, types.NamespacedName{Name: "app", Namespace: "default"}, main); err != nil {
		t.Fatal(err)
	}
	if len(main.Spec.Rules) != 1 || main.Spec.Rules[0].Host != "www.example.com" {
		t.Fatalf("surviving host rules = %+v, want direct www.example.com", main.Spec.Rules)
	}
}

func TestRedirectRegexPreservesPathAndQueryCapture(t *testing.T) {
	spec := redirectRegexHTTPMiddlewareSpec("www.example.com", "example.com")
	redirect := spec["redirectRegex"].(map[string]any)
	rx := regexp.MustCompile(redirect["regex"].(string))
	got := rx.ReplaceAllString("https://www.example.com/a/b?x=one%20two&y=3", redirect["replacement"].(string))
	if got != "https://example.com/a/b?x=one%20two&y=3" {
		t.Fatalf("redirect replacement = %q", got)
	}
}
