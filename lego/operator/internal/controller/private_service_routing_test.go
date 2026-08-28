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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m46 t002 — the operator's own gate on the private-service guarantee.
//
// Before this, nothing at the operator level distinguished a private_service
// from a web_service: both fell through to the same effectiveHosts →
// reconcileIngress path, and the ONLY thing keeping a private service off the
// public internet was spec.expose staying false forever. It did not — the
// control-plane projector re-stamped it true (t001) — and a real service served
// real traffic at a real .onbex.co URL with no authentication.
//
// These specs write the bad values DIRECTLY onto the CR, bypassing bex-api
// entirely, because the point is that no value of expose/host/hosts may publish
// a private service. They stand whether or not t001's fix holds.
var _ = Describe("private_service is never publicly routed", func() {
	ctx := context.Background()

	newReconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes,
			BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
		}
	}
	reconcileN := func(r *AppReconciler, nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	const name = "private-routing-app"
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	var r *AppReconciler

	BeforeEach(func() { r = newReconciler() })
	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			reconcileN(r, nn)
		}
	})

	// createPrivate lands a private_service App and lets mutate write whatever
	// exposure intent the case is proving harmless.
	createPrivate := func(mutate func(*appv1alpha1.AppSpec)) {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Type:  appv1alpha1.TypePrivateService,
				Image: "traefik/whoami", Port: 3000,
			},
		}
		mutate(&app.Spec)
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		reconcileN(r, nn)
	}

	expectPrivateOnly := func() {
		By("a ClusterIP Service exists — a private service IS reachable in-cluster")
		Expect(k8sClient.Get(ctx, nn, &corev1.Service{})).To(Succeed())

		By("but no Ingress object was ever created")
		Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, &networkingv1.Ingress{}))).To(BeTrue(),
			"a private_service must never be fronted by an Ingress")

		By("and status carries the in-cluster address, never a public https URL")
		got := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
		Expect(got.Status.URLs).To(BeEmpty())
		Expect(got.Status.URL).NotTo(HavePrefix("https://"))
	}

	It("ignores spec.expose=true forced directly onto the CR", func() {
		createPrivate(func(spec *appv1alpha1.AppSpec) { spec.Expose = true })
		expectPrivateOnly()
	})

	It("ignores a spec.host set directly on the CR", func() {
		createPrivate(func(spec *appv1alpha1.AppSpec) { spec.Host = "should-not-route.example.com" })
		expectPrivateOnly()
	})

	It("ignores spec.hosts custom domains set directly on the CR", func() {
		createPrivate(func(spec *appv1alpha1.AppSpec) {
			spec.Hosts = []string{"also-should-not-route.example.com"}
		})
		expectPrivateOnly()
	})

	It("ignores all three at once", func() {
		createPrivate(func(spec *appv1alpha1.AppSpec) {
			spec.Expose = true
			spec.Host = "should-not-route.example.com"
			spec.Hosts = []string{"also-should-not-route.example.com"}
		})
		expectPrivateOnly()
	})

	It("removes an Ingress a previously-exposed private service already had", func() {
		// The remediation path for services the incident already published: the
		// operator must take the live route away, not merely stop adding it.
		createPrivate(func(spec *appv1alpha1.AppSpec) {})
		ing := &networkingv1.Ingress{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: networkingv1.IngressSpec{
				Rules: []networkingv1.IngressRule{{Host: name + ".onbex.co"}},
			},
		}
		Expect(k8sClient.Create(ctx, ing)).To(Succeed())

		got := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
		got.Spec.Expose = true
		Expect(k8sClient.Update(ctx, got)).To(Succeed())
		reconcileN(r, nn)

		expectPrivateOnly()
	})

	It("keeps an idle free private service running because it has no wake path", func() {
		r.ActivatorService = "bex-activator"
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: "default",
				Annotations: map[string]string{annotLastActive: "2026-01-01T00:00:00Z"},
			},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypePrivateService, Image: "traefik/whoami", Port: 3000,
				Tier: "free", IdleTTLSeconds: 60,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		reconcileN(r, nn)

		deployment := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, deployment)).To(Succeed())
		Expect(deployment.Spec.Replicas).NotTo(BeNil())
		Expect(*deployment.Spec.Replicas).To(Equal(int32(1)))
		expectPrivateOnly()
	})
})
