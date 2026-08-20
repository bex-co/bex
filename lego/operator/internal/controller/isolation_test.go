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
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Tenant isolation (w7/m1)", func() {
	const namespace = "default"
	ctx := context.Background()

	reconcileApp := func(r *AppReconciler, name string) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{Name: name, Namespace: namespace},
			})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	Context("workspace label propagation to pod template", func() {
		const name = "isolation-ws-label"
		const workspace = "tea-testworkspace0001"

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes,
			}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    map[string]string{labelWorkspace: workspace},
				},
				Spec: appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 80},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
		})
		AfterEach(func() {
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileApp(r, name) // finalizer path
			}
		})

		It("propagates the workspace label to the Deployment pod template", func() {
			reconcileApp(r, name)

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, dep)).To(Succeed())

			By("verifying the pod template carries the workspace label")
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelWorkspace, workspace))

			By("verifying the Deployment selector does NOT include the workspace label")
			Expect(dep.Spec.Selector.MatchLabels).NotTo(HaveKey(labelWorkspace),
				"selector must remain stable — adding labelWorkspace here would break existing Deployments")
		})

		It("keeps the pod template app label alongside the workspace label", func() {
			reconcileApp(r, name)

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, dep)).To(Succeed())
			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app)).To(Succeed())
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelApp, name))
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelWorkspace, workspace))
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelRevision, fmt.Sprintf("rev-%d", app.Generation)))
		})

		It("never creates a per-App NetworkPolicy — the `<ws>` namespace policies are the boundary (ADR043, w3/m34)", func() {
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)
			Expect(errors.IsNotFound(err)).To(BeTrue(), "per-App NetworkPolicy is redundant under per-tenant namespace isolation")
		})

		It("cleans up a stray per-App NetworkPolicy left by a prior version", func() {
			By("simulating a leftover legacy per-App policy from before ADR043")
			legacy := &networkingv1.NetworkPolicy{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: networkingv1.NetworkPolicySpec{
					PodSelector: metav1.LabelSelector{MatchLabels: map[string]string{labelApp: name}},
				},
			}
			Expect(k8sClient.Create(ctx, legacy)).To(Succeed())

			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)
			Expect(errors.IsNotFound(err)).To(BeTrue(), "the stray legacy policy must be deleted on reconcile")
		})
	})

	Context("legacy App without workspace label", func() {
		const name = "isolation-legacy"

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes,
			}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 80},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
		})
		AfterEach(func() {
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileApp(r, name)
			}
		})

		It("reconciles without error and creates no NetworkPolicy", func() {
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)
			Expect(errors.IsNotFound(err)).To(BeTrue(), "legacy App must not get a NetworkPolicy")
		})

		It("does not carry the workspace label in the pod template", func() {
			reconcileApp(r, name)

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, dep)).To(Succeed())
			Expect(dep.Spec.Template.Labels).NotTo(HaveKey(labelWorkspace))
		})
	})

	Context("environment-scoped label propagation (w6/m19 protected-environment ACLs)", func() {
		const name = "isolation-env-scoped"
		const workspace = "tea-testworkspace0002"
		const environment = "env-testenv0001"

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes,
			}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
					Labels:    map[string]string{labelWorkspace: workspace, labelNetworkIsolation: environment},
				},
				Spec: appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 80},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
		})
		AfterEach(func() {
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileApp(r, name)
			}
		})

		It("propagates the environment label to the Deployment pod template", func() {
			reconcileApp(r, name)

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, dep)).To(Succeed())
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelNetworkIsolation, environment))
			Expect(dep.Spec.Selector.MatchLabels).NotTo(HaveKey(labelNetworkIsolation),
				"selector must remain stable, same rationale as labelWorkspace")
		})

		It("removing the environment label clears it from the pod template", func() {
			reconcileApp(r, name)

			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app)).To(Succeed())
			delete(app.Labels, labelNetworkIsolation)
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileApp(r, name)

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, dep)).To(Succeed())
			Expect(dep.Spec.Template.Labels).NotTo(HaveKey(labelNetworkIsolation))
		})

		It("allows only same-environment peers for a protected App", func() {
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())
			Expect(np.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue(labelApp, name))
			Expect(np.Spec.PolicyTypes).To(ConsistOf(networkingv1.PolicyTypeIngress, networkingv1.PolicyTypeEgress))
			Expect(np.Spec.Ingress).To(HaveLen(1))
			Expect(np.Spec.Egress).To(HaveLen(1))
			for _, rule := range np.Spec.Ingress {
				Expect(rule.From).To(HaveLen(2))
				Expect(rule.From[0].PodSelector.MatchLabels).To(HaveKeyWithValue(labelNetworkIsolation, environment))
				Expect(rule.From[1].PodSelector.MatchLabels).To(HaveKeyWithValue(labelEnvironment, environment))
			}
			for _, rule := range np.Spec.Egress {
				Expect(rule.To).To(HaveLen(2))
				Expect(rule.To[0].PodSelector.MatchLabels).To(HaveKeyWithValue(labelNetworkIsolation, environment))
				Expect(rule.To[1].PodSelector.MatchLabels).To(HaveKeyWithValue(labelEnvironment, environment))
			}
		})

		It("removes the protected policy when isolation is disabled", func() {
			reconcileApp(r, name)

			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app)).To(Succeed())
			delete(app.Labels, labelNetworkIsolation)
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})
