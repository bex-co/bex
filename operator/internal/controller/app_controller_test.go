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
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/blockeden/bex/operator/api/v1alpha1"
)

var _ = Describe("App Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		typeNamespacedName := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default", // TODO(user):Modify as needed
		}
		service := &appv1alpha1.App{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind App")
			err := k8sClient.Get(ctx, typeNamespacedName, service)
			if err != nil && errors.IsNotFound(err) {
				resource := &appv1alpha1.App{
					ObjectMeta: metav1.ObjectMeta{
						Name:      resourceName,
						Namespace: "default",
					},
					// TODO(user): Specify other spec details if needed.
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			// TODO(user): Cleanup logic after each test, like removing the resource instance.
			resource := &appv1alpha1.App{}
			err := k8sClient.Get(ctx, typeNamespacedName, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance App")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			controllerReconciler := &AppReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: typeNamespacedName,
			})
			Expect(err).NotTo(HaveOccurred())
			// TODO(user): Add more specific assertions depending on your controller's reconciliation logic.
			// Example: If you expect a certain status condition after reconciliation, verify it here.
		})
	})

	Context("When reconciling an exposed App on the kubernetes runtime", func() {
		const name = "multi-host-app"
		ctx := context.Background()
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		// k8sClient is only set in BeforeSuite — build the reconciler lazily, never
		// in the container body (tree construction runs before the suite starts).
		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			}
		})
		// First pass only adds the finalizer and requeues; later passes stop at
		// Deploying (no kubelet in envtest), which is past the Ingress write.
		reconcileN := func() {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		getIngress := func() *networkingv1.Ingress {
			ing := &networkingv1.Ingress{}
			Expect(k8sClient.Get(ctx, nn, ing)).To(Succeed())
			return ing
		}

		AfterEach(func() {
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, nn, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN() // let the finalizer path run
			}
		})

		It("keeps the single-host Ingress byte-stable and grows it additively", func() {
			By("creating an App with only the legacy spec.host set")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami",
					Port:  3000,
					Host:  "app.1.2.3.4.sslip.io",
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN()

			By("asserting the live-App invariants: one rule, legacy TLS secret name")
			ing := getIngress()
			Expect(ing.Spec.Rules).To(HaveLen(1))
			Expect(ing.Spec.Rules[0].Host).To(Equal("app.1.2.3.4.sslip.io"))
			Expect(ing.Spec.TLS).To(HaveLen(1))
			Expect(ing.Spec.TLS[0].SecretName).To(Equal(name + "-tls"))
			Expect(ing.Annotations).To(HaveKeyWithValue("cert-manager.io/cluster-issuer", "letsencrypt-prod"))

			By("adding expose + a custom domain")
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			app.Spec.Expose = true
			app.Spec.Hosts = []string{"www.example.com"}
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()

			By("asserting hosts grew additively with per-host TLS secrets")
			ing = getIngress()
			Expect(ing.Spec.Rules).To(HaveLen(3))
			Expect(ing.Spec.Rules[0].Host).To(Equal("app.1.2.3.4.sslip.io"))
			Expect(ing.Spec.Rules[1].Host).To(Equal(name + ".onbex.co"))
			Expect(ing.Spec.Rules[2].Host).To(Equal("www.example.com"))
			Expect(ing.Spec.TLS).To(HaveLen(3))
			Expect(ing.Spec.TLS[0].SecretName).To(Equal(name+"-tls"), "first host must keep the legacy secret")
			Expect(ing.Spec.TLS[1].SecretName).To(Equal(name + "-tls-" + name + ".onbex.co"))
			Expect(ing.Spec.TLS[2].SecretName).To(Equal(name + "-tls-www.example.com"))
			for i, tls := range ing.Spec.TLS {
				Expect(tls.Hosts).To(Equal([]string{ing.Spec.Rules[i].Host}), "TLS entries pair 1:1 with rules")
			}

			By("clearing all exposure removes the Ingress")
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			app.Spec.Host = ""
			app.Spec.Expose = false
			app.Spec.Hosts = nil
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			err := k8sClient.Get(ctx, nn, &networkingv1.Ingress{})
			Expect(errors.IsNotFound(err)).To(BeTrue(), "Ingress should be deleted when no hosts remain")
		})
	})

	Context("Lifecycle verbs: restart, suspend, resume (kubernetes runtime)", func() {
		const name = "lifecycle-app"
		ctx := context.Background()
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		})
		reconcileN := func() {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		getDep := func() *appsv1.Deployment {
			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			return dep
		}
		getApp := func() *appv1alpha1.App {
			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			return app
		}

		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN()
			}
		})

		It("rolls on restartedAt, hibernates on suspend, restores on resume", func() {
			By("creating a running-shaped App with 2 replicas and a host")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000, Replicas: 2,
					Host: "lifecycle.1.2.3.4.sslip.io",
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN()
			dep := getDep()
			Expect(*dep.Spec.Replicas).To(Equal(int32(2)))
			Expect(dep.Spec.Template.Annotations).NotTo(HaveKey("app.bex.co/restarted-at"))

			By("restart: setting spec.restartedAt stamps the pod template")
			app = getApp()
			app.Spec.RestartedAt = "2026-07-05T00:00:00Z"
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			dep = getDep()
			Expect(dep.Spec.Template.Annotations).To(HaveKeyWithValue("app.bex.co/restarted-at", "2026-07-05T00:00:00Z"))
			Expect(*dep.Spec.Replicas).To(Equal(int32(2)), "restart must not touch scale")

			By("suspend: scales to 0, phase Hibernated, Ingress kept, spec.replicas kept")
			app = getApp()
			app.Spec.Suspended = true
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			Expect(*getDep().Spec.Replicas).To(Equal(int32(0)))
			app = getApp()
			Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))
			Expect(app.Spec.Replicas).To(Equal(int32(2)), "suspend must not rewrite the stored count")
			Expect(k8sClient.Get(ctx, nn, &networkingv1.Ingress{})).To(Succeed(), "suspend keeps the Ingress (host + certs)")

			By("resume: restores spec.replicas and leaves Hibernated")
			app.Spec.Suspended = false
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN()
			Expect(*getDep().Spec.Replicas).To(Equal(int32(2)))
			// envtest has no kubelet, so readiness never arrives: Deploying, not Running
			Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseDeploying))
		})
	})
})
