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
	"maps"

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

var _ = Describe("Tenant isolation (w7/m1)", func() {
	// Canonical ADR043 placement: namespace == workspace label (a labeled App
	// in the shared namespace is refused since codex-security 2026-08 F11).
	const namespace = "tea-testworkspace0001"
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
			_ = k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: namespace},
			}) // AlreadyExists is fine across specs
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
		// The unlabeled legacy shape is canonical in the BOOTSTRAP namespace
		// only (codex-security 2026-08 F11); in a workspace namespace an
		// unlabeled App is a stray and is refused.
		const legacyNS = "default"

		reconcileLegacy := func(r *AppReconciler, name string) {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: name, Namespace: legacyNS},
				})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes,
			}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: legacyNS},
				Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 80},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
		})
		AfterEach(func() {
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: legacyNS}, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileLegacy(r, name)
			}
		})

		It("reconciles without error and creates no NetworkPolicy", func() {
			reconcileLegacy(r, name)

			np := &networkingv1.NetworkPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: legacyNS}, np)
			Expect(errors.IsNotFound(err)).To(BeTrue(), "legacy App must not get a NetworkPolicy")
		})

		It("does not carry the workspace label in the pod template", func() {
			reconcileLegacy(r, name)

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: legacyNS}, dep)).To(Succeed())
			Expect(dep.Spec.Template.Labels).NotTo(HaveKey(labelWorkspace))
		})
	})

	Context("environment-scoped label propagation (w6/m19 protected-environment ACLs)", func() {
		const name = "isolation-env-scoped"
		const workspace = "tea-testworkspace0002"
		const environment = "env-testenv0001"
		// Canonical placement for THIS workspace (namespace == label).
		const envNS = workspace

		reconcileEnv := func(r *AppReconciler, name string) {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{
					NamespacedName: types.NamespacedName{Name: name, Namespace: envNS},
				})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		var r *AppReconciler
		BeforeEach(func() {
			r = &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes,
			}
			_ = k8sClient.Create(ctx, &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{Name: envNS},
			}) // AlreadyExists is fine across specs
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: envNS,
					Labels:    map[string]string{labelWorkspace: workspace, labelEnvironment: environment, labelNetworkIsolation: environment},
				},
				Spec: appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 80},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
		})
		AfterEach(func() {
			app := &appv1alpha1.App{}
			if err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: envNS}, app); err == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileEnv(r, name)
			}
		})

		It("carries the environment id, never the isolation label, on the pod template", func() {
			reconcileEnv(r, name)

			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: envNS}, dep)).To(Succeed())
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelEnvironment, environment))
			Expect(dep.Spec.Template.Labels).NotTo(HaveKey(labelNetworkIsolation),
				"an isolation toggle must never be a template change (w4/m342)")
			Expect(dep.Spec.Selector.MatchLabels).NotTo(HaveKey(labelEnvironment),
				"selector must remain stable, same rationale as labelWorkspace")
		})

		// w4/m342: round 177 saw every isolation toggle roll each service of the
		// environment, because the label was copied onto the pod template.
		It("relabels running pods in place on a toggle or a move and never changes the template", func() {
			reconcileEnv(r, name)
			nn := types.NamespacedName{Name: name, Namespace: envNS}
			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			template, generation := dep.Spec.Template.DeepCopy(), dep.Generation

			controller := true
			podLabels := maps.Clone(dep.Spec.Template.Labels)
			delete(podLabels, labelEnvironment) // a pod from before environment-id
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{
					Name: name + "-pod", Namespace: envNS, Labels: podLabels,
					OwnerReferences: []metav1.OwnerReference{{
						APIVersion: "apps/v1", Kind: "ReplicaSet", Name: name + "-rs", UID: "rs-uid", Controller: &controller,
					}},
				},
				Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "traefik/whoami"}}},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), pod) })
			podLabelsNow := func() map[string]string {
				live := &corev1.Pod{}
				Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: envNS}, live)).To(Succeed())
				return live.Labels
			}
			setLabels := func(labels map[string]string) {
				app := &appv1alpha1.App{}
				Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
				for k, v := range labels {
					if v == "" {
						delete(app.Labels, k)
					} else {
						app.Labels[k] = v
					}
				}
				Expect(k8sClient.Update(ctx, app)).To(Succeed())
				reconcileEnv(r, name)
			}
			templateUnchanged := func(when string) {
				Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
				Expect(dep.Generation).To(Equal(generation), "%s must not roll the Deployment", when)
				Expect(dep.Spec.Template.Labels).To(Equal(template.Labels), "%s must not change the pod template", when)
			}

			reconcileEnv(r, name)
			Expect(podLabelsNow()).To(HaveKeyWithValue(labelEnvironment, environment),
				"an existing pod gains environment-id in place, with no rollout")
			Expect(podLabelsNow()).To(HaveKeyWithValue(labelNetworkIsolation, environment))

			setLabels(map[string]string{labelNetworkIsolation: ""})
			templateUnchanged("turning isolation off")
			Expect(podLabelsNow()).NotTo(HaveKey(labelNetworkIsolation))
			Expect(podLabelsNow()).To(HaveKeyWithValue(labelEnvironment, environment))

			setLabels(map[string]string{labelNetworkIsolation: environment})
			templateUnchanged("turning isolation on")
			Expect(podLabelsNow()).To(HaveKeyWithValue(labelNetworkIsolation, environment))

			setLabels(map[string]string{labelEnvironment: "env-testenv0002", labelNetworkIsolation: ""})
			templateUnchanged("a move")
			Expect(podLabelsNow()).To(HaveKeyWithValue(labelEnvironment, "env-testenv0002"))
			Expect(podLabelsNow()).NotTo(HaveKey(labelNetworkIsolation))

			// The template adopts the current environment on its next real change.
			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			app.Spec.RestartedAt = "2026-11-14T00:00:00Z"
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileEnv(r, name)
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelEnvironment, "env-testenv0002"))
		})

		It("allows only same-environment peers for a protected App", func() {
			reconcileEnv(r, name)

			np := &networkingv1.NetworkPolicy{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: envNS}, np)).To(Succeed())
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
			reconcileEnv(r, name)

			app := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: envNS}, app)).To(Succeed())
			delete(app.Labels, labelNetworkIsolation)
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileEnv(r, name)

			np := &networkingv1.NetworkPolicy{}
			err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: envNS}, np)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})
