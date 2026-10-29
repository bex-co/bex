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
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestIngressTLSKeepsReadyCertificatesAheadOfPending(t *testing.T) {
	for _, tc := range []struct {
		name       string
		ready      []int
		incomplete int
		order      []int
	}{
		{"platform pending", []int{1, 2}, -1, []int{1, 2, 0}},
		{"custom pending", []int{0, 2}, -1, []int{0, 2, 1}},
		{"first custom pending", []int{2}, -1, []int{2, 0, 1}},
		{"incomplete platform", []int{1, 2}, 0, []int{1, 2, 0}},
		{"all ready", []int{0, 1, 2}, -1, []int{0, 1, 2}},
		{"all pending", nil, -1, []int{0, 1, 2}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			scheme := newHostRedirectScheme()
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: "tenant"}}
			hosts := []string{"site.onbex.test", "first.example.test", "last.example.test"}
			cached := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
			// Workspace Secrets are absent from the manager's Secret cache.
			direct := fake.NewClientBuilder().WithScheme(scheme).Build()
			for i, host := range hosts {
				if !slices.Contains(tc.ready, i) && tc.incomplete != i {
					continue
				}
				secret := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Namespace: app.Namespace,
					Name: appv1alpha1.TLSSecretName(app.Name, i == 0, host)},
					Data: map[string][]byte{corev1.TLSCertKey: []byte("certificate")}}
				if slices.Contains(tc.ready, i) {
					secret.Data[corev1.TLSPrivateKeyKey] = []byte("key")
				}
				if err := direct.Create(ctx, secret); err != nil {
					t.Fatal(err)
				}
			}
			r := &AppReconciler{Client: cached, BuildClient: direct, Scheme: scheme, ClusterIssuer: "issuer"}
			if err := r.reconcileIngress(ctx, app, hosts, app.Name, 10000, nil); err != nil {
				t.Fatal(err)
			}
			var ing networkingv1.Ingress
			if err := cached.Get(ctx, client.ObjectKeyFromObject(app), &ing); err != nil {
				t.Fatal(err)
			}
			for i, original := range tc.order {
				if ing.Spec.TLS[i].Hosts[0] != hosts[original] ||
					ing.Spec.TLS[i].SecretName != appv1alpha1.TLSSecretName(app.Name, original == 0, hosts[original]) {
					t.Fatalf("TLS references = %+v, expected original host indices %v", ing.Spec.TLS, tc.order)
				}
				if ing.Spec.Rules[i].Host != hosts[i] {
					t.Fatal("certificate availability changed serving rules or router attribution")
				}
			}
		})
	}
}

func TestIngressTLSRechecksIssuanceAndDeletion(t *testing.T) {
	ctx := context.Background()
	scheme := newHostRedirectScheme()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: "default"}}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme}
	hosts := []string{"site.onbex.test", "custom.example.test"}
	custom := &corev1.Secret{ObjectMeta: metav1.ObjectMeta{Name: appv1alpha1.TLSSecretName(app.Name, false, hosts[1]), Namespace: app.Namespace},
		Data: map[string][]byte{corev1.TLSCertKey: []byte("certificate"), corev1.TLSPrivateKeyKey: []byte("key")}}
	check := func(first string) {
		t.Helper()
		if err := r.reconcileIngress(ctx, app, hosts, app.Name, 10000, nil); err != nil {
			t.Fatal(err)
		}
		var ing networkingv1.Ingress
		if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &ing); err != nil {
			t.Fatal(err)
		}
		if ing.Spec.TLS[0].Hosts[0] != first {
			t.Fatalf("first certificate = %q, want %q", ing.Spec.TLS[0].Hosts[0], first)
		}
	}
	check(hosts[0])
	if err := cl.Create(ctx, custom); err != nil {
		t.Fatal(err)
	}
	check(hosts[1])
	if err := cl.Delete(ctx, custom); err != nil {
		t.Fatal(err)
	}
	check(hosts[0])
}

func TestSleepingMultiHostServiceRechecksTLS(t *testing.T) {
	for _, custom := range []bool{false, true} {
		scheme := newIPAllowListScheme()
		app := appWithAllowList("sleepy", nil, appv1alpha1.TypeWebService)
		app.Spec.Suspended = true
		if custom {
			app.Spec.Hosts = []string{"custom.example.test"}
		}
		cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).Build()
		r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.test"}
		result, err := r.dispatchRuntime(context.Background(), app, "nginx:1", 8080)
		if err != nil {
			t.Fatal(err)
		}
		if custom && (result.RequeueAfter <= 0 || result.RequeueAfter > childHealthRequeue) {
			t.Fatalf("sleeping multi-host service would not recheck certificate changes: %+v", result)
		}
		if !custom && result.RequeueAfter != 0 {
			t.Fatalf("single-host service acquired certificate polling: %+v", result)
		}
	}
}

func TestDirectStaticSiteRechecksTLSWithoutPodEvents(t *testing.T) {
	scheme := newIPAllowListScheme()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "site", Namespace: "default"},
		Spec: appv1alpha1.AppSpec{Type: appv1alpha1.TypeStaticSite, Repo: "https://example.test/site", PublishPath: "dist",
			Expose: true, Hosts: []string{"custom.example.test"}}}
	app.Status.ActiveRevision = releaseRevision(app)
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).WithStatusSubresource(app).Build()
	r := &AppReconciler{Client: cl, Scheme: scheme, Mode: ModeKubernetes, BaseDomain: "onbex.test",
		StaticStore:         publish.Store{Bucket: "static", Endpoint: "https://s3.example.test", Secret: "publisher"},
		StaticServerService: "static-server", StaticServerPort: 8080}
	// Direct publication and static restores call this path without dispatchRuntime.
	result, err := r.reconcileKubernetes(context.Background(), app, "", 8080)
	if err != nil {
		t.Fatal(err)
	}
	if result.RequeueAfter <= 0 || result.RequeueAfter > childHealthRequeue {
		t.Fatalf("direct-published site would not observe later certificate issuance: %+v", result)
	}
}
