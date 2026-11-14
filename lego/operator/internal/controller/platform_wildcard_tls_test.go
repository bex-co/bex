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
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/650: a host the platform's *.onbex.co wildcard covers orders no
// per-host certificate; custom domains and deeper names still do.

func TestPlatformWildcardCovers(t *testing.T) {
	r := &AppReconciler{BaseDomain: "onbex.co"}
	for host, want := range map[string]bool{
		"web.onbex.co":      true,
		"a.b.onbex.co":      false, // the wildcard matches one label only
		"onbex.co":          false,
		"*.onbex.co":        false,
		"web.example.com":   false,
		"web.notonbex.co":   false,
		"web.onbex.co.evil": false,
	} {
		if got := r.platformWildcardCovers(host); got != want {
			t.Errorf("platformWildcardCovers(%q) = %v, want %v", host, got, want)
		}
	}
	if (&AppReconciler{}).platformWildcardCovers("web.onbex.co") {
		t.Error("with no base domain nothing is covered")
	}
}

func TestPlatformHostOrdersNoPerHostCertificate(t *testing.T) {
	for _, tc := range []struct {
		name       string
		hosts      []string
		wantSecret map[string]string // host -> TLS Secret ("" = none)
		wantIssuer bool
	}{{
		name:       "platform host only",
		hosts:      []string{"web.onbex.co"},
		wantSecret: map[string]string{"web.onbex.co": ""},
	}, {
		name:       "custom domain only",
		hosts:      []string{"www.example.com"},
		wantSecret: map[string]string{"www.example.com": "web-tls"},
		wantIssuer: true,
	}, {
		name:  "platform host and custom domain",
		hosts: []string{"web.onbex.co", "www.example.com"},
		wantSecret: map[string]string{
			"web.onbex.co":    "",
			"www.example.com": appv1alpha1.TLSSecretName("web", false, "www.example.com"),
		},
		wantIssuer: true,
	}, {
		name:       "a name deeper than the wildcard",
		hosts:      []string{"api.web.onbex.co"},
		wantSecret: map[string]string{"api.web.onbex.co": "web-tls"},
		wantIssuer: true,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := newHostRedirectScheme()
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}
			cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
			r := &AppReconciler{Client: cl, Scheme: scheme, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod"}
			if err := r.reconcileIngress(ctx, app, tc.hosts, "web", 3000, nil); err != nil {
				t.Fatal(err)
			}
			var ing networkingv1.Ingress
			if err := cl.Get(ctx, types.NamespacedName{Name: "web", Namespace: "default"}, &ing); err != nil {
				t.Fatal(err)
			}
			got := map[string]string{}
			for _, entry := range ing.Spec.TLS {
				if len(entry.Hosts) != 1 {
					t.Fatalf("TLS entry %+v must name exactly one host", entry)
				}
				got[entry.Hosts[0]] = entry.SecretName
			}
			// Every host keeps a TLS entry: Traefik mints a host's HTTPS
			// router only for hosts in spec.tls.
			if len(got) != len(tc.wantSecret) {
				t.Fatalf("TLS entries = %+v, want one per host %v", ing.Spec.TLS, tc.wantSecret)
			}
			for host, want := range tc.wantSecret {
				if secret, ok := got[host]; !ok || secret != want {
					t.Errorf("TLS Secret for %s = %q (present %v), want %q", host, secret, ok, want)
				}
			}
			if _, ok := ing.Annotations[certManagerClusterIssuerAnnotation]; ok != tc.wantIssuer {
				t.Errorf("cluster-issuer annotation present = %v, want %v: %v", ok, tc.wantIssuer, ing.Annotations)
			}
		})
	}
}

// An Ingress written before w4/650 named a Secret for its platform host, and
// ingress-shim made a Certificate for it. Once the Ingress loses its issuer
// ingress-shim no longer removes that Certificate, so the operator deletes
// it, and only the one this Ingress controls.
func TestDroppedPlatformCertificateIsDeletedOnce(t *testing.T) {
	ctx := context.Background()
	scheme := newHostRedirectScheme()
	scheme.AddKnownTypeWithName(certManagerCertificateGVK, &unstructured.Unstructured{})
	listGVK := certManagerCertificateGVK
	listGVK.Kind += "List"
	scheme.AddKnownTypeWithName(listGVK, &unstructured.UnstructuredList{})

	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "app-uid"}}
	ing := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", UID: "ing-uid",
			Annotations: map[string]string{certManagerClusterIssuerAnnotation: "letsencrypt-prod"}},
		Spec: networkingv1.IngressSpec{TLS: []networkingv1.IngressTLS{
			{Hosts: []string{"web.onbex.co"}, SecretName: "web-tls"},
			{Hosts: []string{"old.example.com"}, SecretName: "web-tls-old.example.com"},
		}},
	}
	certificate := func(name string, controller *metav1.OwnerReference) *unstructured.Unstructured {
		c := &unstructured.Unstructured{}
		c.SetGroupVersionKind(certManagerCertificateGVK)
		c.SetNamespace("default")
		c.SetName(name)
		if controller != nil {
			c.SetOwnerReferences([]metav1.OwnerReference{*controller})
		}
		return c
	}
	byIngress := &metav1.OwnerReference{APIVersion: "networking.k8s.io/v1", Kind: "Ingress", Name: "web", UID: "ing-uid", Controller: new(true)}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app, ing,
		certificate("web-tls", byIngress),
		certificate("web-tls-old.example.com", nil),
	).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod"}

	if err := r.reconcileIngress(ctx, app, []string{"web.onbex.co"}, "web", 3000, nil); err != nil {
		t.Fatal(err)
	}
	exists := func(name string) bool {
		err := cl.Get(ctx, client.ObjectKey{Namespace: "default", Name: name}, certificate(name, nil))
		if err != nil && !apierrors.IsNotFound(err) {
			t.Fatal(err)
		}
		return err == nil
	}
	if exists("web-tls") {
		t.Error("the platform host's Certificate the Ingress controlled must be deleted once its TLS entry names no Secret")
	}
	if !exists("web-tls-old.example.com") {
		t.Error("a Certificate the Ingress does not control must be left alone")
	}
}
