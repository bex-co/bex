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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w21/m617 against a real API server: the spread is stored exactly as
// projected, the operator upgrade that ships it writes no stored Deployment,
// and a scalable service gains it on its next roll and never on a scale.
var _ = Describe("Instance spread (w21/m617)", func() {
	const namespace = "default"
	ctx := context.Background()

	It("stores what the operator projects, and the upgrade writes no stored Deployment", func() {
		for _, tc := range []struct {
			name     string
			replicas int32
			spec     appv1alpha1.AppSpec
			spread   bool
		}{
			{"spread-multi", 3, appv1alpha1.AppSpec{Image: "ghcr.io/example/app:v1", Port: 3000, Tier: "standard"}, true},
			{"spread-single", 1, appv1alpha1.AppSpec{Image: "ghcr.io/example/app:v1", Port: 3000, Tier: "starter"}, true},
			{"spread-free", 1, appv1alpha1.AppSpec{Image: "ghcr.io/example/app:v1", Port: 3000, Tier: tierFree}, false},
			{"spread-disk", 1, appv1alpha1.AppSpec{
				Image: "ghcr.io/example/app:v1", Port: 3000, Tier: "starter",
				Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10},
			}, false},
		} {
			By(tc.name)
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: namespace}, Spec: tc.spec}
			params := deploymentParams{image: tc.spec.Image, port: 3000, replicas: tc.replicas}
			nn := types.NamespacedName{Name: tc.name, Namespace: namespace}

			// A new Deployment: what is stored is what was projected.
			fresh := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: namespace}}
			applyDeploymentSpec(fresh, app, params)
			Expect(fresh.Spec.Template.Spec.TopologySpreadConstraints != nil).To(Equal(tc.spread))
			projected := fresh.Spec.Template.DeepCopy()
			Expect(k8sClient.Create(ctx, fresh)).To(Succeed())
			stored := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, stored)).To(Succeed())
			Expect(stored.Spec.Template).To(Equal(*projected),
				"the API server must store the spread unchanged, or every pass would write it again")
			Expect(k8sClient.Delete(ctx, stored)).To(Succeed())
			Eventually(func() error { return k8sClient.Get(ctx, nn, &appsv1.Deployment{}) }).ShouldNot(Succeed())

			// The operator before w21/m617 stored no spread; the upgrade
			// re-projects that Deployment exactly as CreateOrUpdate would.
			before := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: tc.name, Namespace: namespace}}
			applyDeploymentSpec(before, app, params)
			before.Spec.Template.Spec.TopologySpreadConstraints = nil
			Expect(k8sClient.Create(ctx, before)).To(Succeed())
			storedBefore := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, storedBefore)).To(Succeed())

			after := storedBefore.DeepCopy()
			applyDeploymentSpec(after, app, params)
			Expect(k8sClient.Update(ctx, after)).To(Succeed())
			storedAfter := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, storedAfter)).To(Succeed())
			Expect(storedAfter.Spec.Template).To(Equal(storedBefore.Spec.Template),
				"the upgrade must leave the stored template, and so its pod-template hash, unchanged")
			Expect(storedAfter.ResourceVersion).To(Equal(storedBefore.ResourceVersion),
				"the upgrade must not even write")
			Expect(k8sClient.Delete(ctx, storedAfter)).To(Succeed())
		}
	})

	It("reaches a scalable service with its next roll, not with the upgrade or a scale", func() {
		const name = "spread-adopt"
		rec := &writeRecorder{Client: k8sClient}
		r := &AppReconciler{Client: rec, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		nn := types.NamespacedName{Name: name, Namespace: namespace}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
			Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/example/app:v1", Port: 3000, Tier: "starter", Replicas: 1},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		reconcileToFixedPoint(run)
		DeferCleanup(func() { deleteOwner(ctx, app, run) })
		serveEnvtestRelease(nn, func() { reconcileToFixedPoint(run) })

		Expect(envtestDeployment(nn).Spec.Template.Spec.TopologySpreadConstraints).To(
			Equal([]corev1.TopologySpreadConstraint{instanceSpread(name)}),
			"a new paid service carries the spread at one instance, so a later scale needs no roll")

		By("a template stored before w21/m617 survives the upgraded operator's pass unwritten")
		dep := envtestDeployment(nn)
		dep.Spec.Template.Spec.TopologySpreadConstraints = nil
		Expect(k8sClient.Update(ctx, dep)).To(Succeed())
		markEnvtestDeploymentReady(nn)
		legacy := envtestDeployment(nn)
		rec.reset()
		reconcileToFixedPoint(run)
		Expect(rec.sorted()).NotTo(ContainElement("update Deployment/"+name), "the upgrade must not write the Deployment")
		Expect(envtestDeployment(nn).Spec.Template).To(Equal(legacy.Spec.Template))

		By("a scale to three changes the replicas and rolls nothing")
		current := envtestApp(nn)
		current.Spec.Replicas = 3
		Expect(k8sClient.Update(ctx, current)).To(Succeed())
		reconcileToFixedPoint(run)
		scaled := envtestDeployment(nn)
		Expect(scaled.Spec.Replicas).To(HaveValue(Equal(int32(3))))
		Expect(scaled.Spec.Template).To(Equal(legacy.Spec.Template), "a scale must not change the pod template")

		By("a restart, a roll the service makes anyway, adopts the spread")
		current = envtestApp(nn)
		current.Spec.RestartedAt = "2026-11-24T00:00:00Z"
		Expect(k8sClient.Update(ctx, current)).To(Succeed())
		reconcileToFixedPoint(run)
		restarted := envtestDeployment(nn)
		Expect(restarted.Spec.Template.Annotations).To(HaveKey("app.bex.co/restarted-at"))
		Expect(restarted.Spec.Template.Spec.TopologySpreadConstraints).To(
			Equal([]corev1.TopologySpreadConstraint{instanceSpread(name)}))
	})
})
