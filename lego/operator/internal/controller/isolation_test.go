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

		It("excepts RFC1918/CGNAT and link-local from the public-internet egress rule", func() {
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())

			By("finding the public-internet egress rule (the 0.0.0.0/0 ipBlock)")
			var internet *networkingv1.IPBlock
			for _, rule := range np.Spec.Egress {
				for _, peer := range rule.To {
					if peer.IPBlock != nil && peer.IPBlock.CIDR == "0.0.0.0/0" {
						internet = peer.IPBlock
					}
				}
			}
			Expect(internet).NotTo(BeNil(), "expected an egress rule allowing 0.0.0.0/0")

			By("verifying the except list blocks in-cluster platforms and the cloud-metadata range")
			// link-local 169.254.0.0/16 (cloud instance-metadata, 169.254.169.254)
			// must be excepted — its absence is the w7/m4 SSRF hole. The four
			// RFC1918/CGNAT ranges keep in-cluster platform services unreachable.
			Expect(internet.Except).To(ConsistOf(
				"10.0.0.0/8",
				"172.16.0.0/12",
				"192.168.0.0/16",
				"100.64.0.0/10",
				"169.254.0.0/16",
			), "the public-internet egress except list must include link-local (metadata) and every RFC1918/CGNAT range")
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

		It("skips and cleans up the per-App NetworkPolicy when tenant-namespace isolation is active (m31 t005)", func() {
			By("first reconciling without isolation to create the legacy per-App policy")
			reconcileApp(r, name)
			np := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())

			By("enabling per-tenant namespace isolation and reconciling again")
			r.TenantNamespaces = true
			reconcileApp(r, name)

			By("verifying the redundant per-App NetworkPolicy is deleted and none is recreated")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)
			Expect(errors.IsNotFound(err)).To(BeTrue(),
				"the `<ws>` namespace policies are the boundary; the per-App policy must be removed")
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

	Context("environment-scoped NetworkPolicy (w6/m19 protected-environment ACLs)", func() {
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

		It("scopes ingress/egress to same-environment pods instead of same-workspace", func() {
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())

			By("verifying ingress allows same-environment pods, not same-workspace ones")
			var hasEnvPeer, hasWorkspacePeer bool
			for _, p := range np.Spec.Ingress[0].From {
				if p.PodSelector == nil {
					continue
				}
				if p.PodSelector.MatchLabels[labelNetworkIsolation] == environment {
					hasEnvPeer = true
				}
				if _, ok := p.PodSelector.MatchLabels[labelWorkspace]; ok {
					hasWorkspacePeer = true
				}
			}
			Expect(hasEnvPeer).To(BeTrue(), "ingress must allow same-environment pods")
			Expect(hasWorkspacePeer).To(BeFalse(), "ingress must NOT fall back to the same-workspace selector when scoped to an environment")

			By("verifying the same-scope egress rule selects by environment, not workspace")
			var egressHasEnvPeer, egressHasWorkspacePeer bool
			for _, rule := range np.Spec.Egress {
				for _, p := range rule.To {
					if p.PodSelector == nil {
						continue
					}
					if p.PodSelector.MatchLabels[labelNetworkIsolation] == environment {
						egressHasEnvPeer = true
					}
					if _, ok := p.PodSelector.MatchLabels[labelWorkspace]; ok {
						egressHasWorkspacePeer = true
					}
				}
			}
			Expect(egressHasEnvPeer).To(BeTrue(), "egress must allow same-environment pods/services")
			Expect(egressHasWorkspacePeer).To(BeFalse(), "egress must NOT fall back to the same-workspace selector when scoped to an environment")

			By("verifying DNS and public-internet egress rules are unaffected")
			Expect(np.Spec.Egress).To(HaveLen(3))
		})

		It("falls back to the same-workspace selector when the environment label is removed", func() {
			reconcileApp(r, name)

			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, app)).To(Succeed())
			delete(app.Labels, labelNetworkIsolation)
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileApp(r, name)

			np := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, np)).To(Succeed())
			var hasWorkspacePeer bool
			for _, p := range np.Spec.Ingress[0].From {
				if p.PodSelector != nil && p.PodSelector.MatchLabels[labelWorkspace] == workspace {
					hasWorkspacePeer = true
				}
			}
			Expect(hasWorkspacePeer).To(BeTrue(), "removing the environment label must fall back to same-workspace ingress")

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, dep)).To(Succeed())
			Expect(dep.Spec.Template.Labels).NotTo(HaveKey(labelNetworkIsolation))
		})
	})
})
