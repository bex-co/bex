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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// tenant_namespace_routing_envtest_test.go is w7/m78/t001's reproduction, and
// it disproves the milestone's original premise.
//
// The 2026-08-08 incident left two production web services with running pods,
// no Ingress, and `serviceDetails.url: null`. m78 was filed on the hypothesis
// that some step between the Deployment apply and the Ingress apply returned
// early — reconcileIngress is unconditional and not health-gated, so a
// crashlooping pod should still have been routed.
//
// No step fails. The reconcile completes successfully and creates no Ingress
// ON PURPOSE, because production runs with BEX_BASE_DOMAIN unset — a deliberate
// security decision recorded in three manifests (config/manager/manager.yaml,
// config/api/deployment.yaml, config/prod/static-config.yaml): onbex.co is a
// registrable domain, so tenant JavaScript could set parent cookies a sibling
// tenant receives. With no base domain, AppSpec.EffectiveHosts contributes no
// platform host, and a service with no custom domain of its own has zero
// effective hosts — so reconcileIngress correctly writes nothing and
// status.URL is never populated.
//
// These specs pin both halves so the behavior can never again be mistaken for
// a failure: the enabled configuration routes, and the production configuration
// deliberately does not.
var _ = Describe("Public routing for a store-shaped App in a tenant namespace (w7/m78)", func() {
	const (
		workspace = "tea-d9routingtest00000"
		slug      = "forum"
	)
	// The control-plane projector names a CR CRName(tenant, slug), lands it in
	// the workspace's own namespace, and carries the slug as spec.subdomain so
	// the platform host stays `<slug>.<baseDomain>` rather than the prefixed
	// object name (store/reconciler.go).
	name := workspace + "-" + slug
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: workspace}

	ensureNamespace := func() {
		if err := k8sClient.Get(ctx, types.NamespacedName{Name: workspace}, &corev1.Namespace{}); err != nil {
			Expect(k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: workspace},
			})).To(Succeed())
		}
	}

	// storeShapedApp is what the projector writes for a web service created
	// today: exposed, slug-subdomained, and carrying NO custom domain of its
	// own — the shape both stranded forums had.
	storeShapedApp := func() *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name:      name,
				Namespace: workspace,
				Labels:    map[string]string{labelWorkspace: workspace},
			},
			Spec: appv1alpha1.AppSpec{
				Image:     "zot.test:5000/forum:1",
				Port:      3000,
				Replicas:  1,
				Type:      appv1alpha1.TypeWebService,
				Expose:    true,
				Subdomain: slug,
			},
		}
	}

	reconcileWith := func(baseDomain string, app *appv1alpha1.App) {
		ensureNamespace()
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, BaseDomain: baseDomain, ClusterIssuer: "letsencrypt-prod",
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		// First pass adds the finalizer and requeues; later passes stop at
		// Deploying (no kubelet in envtest), which is past the Ingress write.
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred(),
				"the reconcile must not error — the incident was not a failed step")
		}
	}

	AfterEach(func() {
		app := &appv1alpha1.App{}
		if err := k8sClient.Get(ctx, nn, app); err == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
			r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		}
	})

	Context("with a platform base domain configured", func() {
		It("creates the ClusterIP Service, the slug alias, and the Ingress", func() {
			reconcileWith("onbex.co", storeShapedApp())

			By("the ClusterIP Service fronting the pods exists")
			Expect(k8sClient.Get(ctx, nn, &corev1.Service{})).To(Succeed(),
				"no ClusterIP Service: the reconcile returned before applyClusterIPService")

			By("the slug-named alias exists (ADR041 internal address)")
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: slug, Namespace: workspace},
				&corev1.Service{})).To(Succeed())

			By("the Ingress exists at the platform host")
			ing := &networkingv1.Ingress{}
			Expect(k8sClient.Get(ctx, nn, ing)).To(Succeed(),
				"no Ingress despite a configured base domain: the reconcile returned before reconcileIngress")
			Expect(ing.Spec.Rules).To(HaveLen(1))
			Expect(ing.Spec.Rules[0].Host).To(Equal(slug+".onbex.co"),
				"the platform host must derive from spec.subdomain, not the tenant-prefixed object name")
		})
	})

	Context("with BEX_BASE_DOMAIN unset, as production runs it", func() {
		It("reproduces the incident: routing succeeds, yet no Ingress and no URL", func() {
			reconcileWith("", storeShapedApp())

			By("the in-cluster Service is still created — the workload is addressable")
			Expect(k8sClient.Get(ctx, nn, &corev1.Service{})).To(Succeed())

			By("no Ingress, and NOT because anything failed")
			err := k8sClient.Get(ctx, nn, &networkingv1.Ingress{})
			Expect(apierrors.IsNotFound(err)).To(BeTrue(),
				"a service with no custom domain has no effective host when the platform "+
					"subdomain is disabled, so reconcileIngress deliberately writes nothing")

			By("status carries no URL — what the API reports as serviceDetails.url: null")
			reconciled := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, reconciled)).To(Succeed())
			Expect(reconciled.Status.URL).To(BeEmpty())
			Expect(reconciled.Status.Phase).NotTo(Equal(appv1alpha1.PhaseFailed),
				"the App is not Failed — nothing in the routing path errored, which is why "+
					"no status reason ever named a cause")
		})

		It("routes a service that brings its own custom domain", func() {
			app := storeShapedApp()
			app.Spec.Hosts = []string{"forum.example.com"}
			reconcileWith("", app)

			By("an explicit host is a host regardless of the platform subdomain")
			ing := &networkingv1.Ingress{}
			Expect(k8sClient.Get(ctx, nn, ing)).To(Succeed(),
				"a custom domain must route even with the platform subdomain disabled — "+
					"this is why the pre-existing reference tenant had a working URL")
			Expect(ing.Spec.Rules).To(HaveLen(1))
			Expect(ing.Spec.Rules[0].Host).To(Equal("forum.example.com"))
		})
	})
})
