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
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelApp, name))
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelWorkspace, workspace))
		})

		It("reconciles a NetworkPolicy with the correct selectors", func() {
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())

			By("verifying the podSelector selects by app label")
			Expect(np.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue(labelApp, name))

			By("verifying policyTypes covers both Ingress and Egress")
			Expect(np.Spec.PolicyTypes).To(ConsistOf(
				networkingv1.PolicyTypeIngress,
				networkingv1.PolicyTypeEgress,
			))

			By("verifying ingress allows from same-workspace pods")
			Expect(np.Spec.Ingress).To(HaveLen(1))
			peers := np.Spec.Ingress[0].From
			var hasWorkspacePeer bool
			for _, p := range peers {
				if p.PodSelector != nil && p.PodSelector.MatchLabels[labelWorkspace] == workspace {
					hasWorkspacePeer = true
				}
			}
			Expect(hasWorkspacePeer).To(BeTrue(), "ingress must allow same-workspace pods")

			By("verifying egress has three rules: DNS, same-workspace, internet")
			Expect(np.Spec.Egress).To(HaveLen(3))
		})

		It("deletes the NetworkPolicy when the workspace label is removed", func() {
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())

			By("removing the workspace label from the App")
			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app)).To(Succeed())
			delete(app.Labels, labelWorkspace)
			Expect(k8sClient.Update(ctx, app)).To(Succeed())

			reconcileApp(r, name)

			By("verifying the NetworkPolicy is gone")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)
			Expect(errors.IsNotFound(err)).To(BeTrue(), "NetworkPolicy must be removed when workspace label is absent")
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

	Context("NetworkPolicy selector isolation", func() {
		const nameA = "isolation-np-a"
		const nameB = "isolation-np-b"
		const wsA = "tea-workspace-alpha-001"
		const wsB = "tea-workspace-beta-002"

		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes,
			}
			for _, tc := range []struct{ name, ws string }{{nameA, wsA}, {nameB, wsB}} {
				app := &appv1alpha1.App{
					ObjectMeta: metav1.ObjectMeta{
						Name: tc.name, Namespace: namespace,
						Labels: map[string]string{labelWorkspace: tc.ws},
					},
					Spec: appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 80},
				}
				Expect(k8sClient.Create(ctx, app)).To(Succeed())
			}
		})
		AfterEach(func() {
			for _, name := range []string{nameA, nameB} {
				app := &appv1alpha1.App{}
				if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app); err == nil {
					Expect(k8sClient.Delete(ctx, app)).To(Succeed())
					reconcileApp(r, name)
				}
			}
		})

		It("generates distinct workspace selectors for each app", func() {
			reconcileApp(r, nameA)
			reconcileApp(r, nameB)

			npA := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nameA, Namespace: namespace}, npA)).To(Succeed())
			npB := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: nameB, Namespace: namespace}, npB)).To(Succeed())

			ingressWSA := npA.Spec.Ingress[0].From
			ingressWSB := npB.Spec.Ingress[0].From
			// workspace selector in A's policy must reference wsA, not wsB
			for _, p := range ingressWSA {
				if p.PodSelector != nil {
					Expect(p.PodSelector.MatchLabels[labelWorkspace]).To(Equal(wsA),
						"App A's NetworkPolicy must not allow workspace B pods")
				}
			}
			// workspace selector in B's policy must reference wsB
			for _, p := range ingressWSB {
				if p.PodSelector != nil {
					Expect(p.PodSelector.MatchLabels[labelWorkspace]).To(Equal(wsB),
						"App B's NetworkPolicy must not allow workspace A pods")
				}
			}
		})
	})
})
