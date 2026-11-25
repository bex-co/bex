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
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m633 against a real API server: a Deployment stored with the API
// server's default adopts the rolling strategy with its template unchanged,
// and every roll of a multi-instance service, a release that never becomes
// healthy included, is written under maxUnavailable: 0.
var _ = Describe("Rolling strategy keeps every old instance (w6/m633)", func() {
	const namespace = "default"
	ctx := context.Background()

	// keepsEveryOldInstance is the Deployment controller's arithmetic for the
	// stored strategy at its replica count: none of the old instances may go
	// before a new one is available, and at least one new one starts.
	keepsEveryOldInstance := func(dep *appsv1.Deployment) {
		GinkgoHelper()
		Expect(dep.Spec.Strategy).To(Equal(defaultRollingUpdateStrategy()))
		n := *dep.Spec.Replicas
		surge, unavailable := rolloutFenceposts(GinkgoT(), dep.Spec.Strategy, n)
		Expect(unavailable).To(BeZero(), "no old instance leaves before its replacement is available")
		Expect(surge).To(Equal(int(n+3)/4), "a quarter of the instances, at least one, start at once")
	}

	It("adopts the strategy on a Deployment the API server defaulted, without changing its template", func() {
		for _, replicas := range []int32{4, appv1alpha1.MaxReplicas} {
			name := fmt.Sprintf("rolling-adopt-%d", replicas)
			By(name)
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec:       appv1alpha1.AppSpec{Image: "ghcr.io/example/app:v1", Port: 3000, Tier: "standard", HealthCheckPath: "/healthz"},
			}
			params := deploymentParams{image: app.Spec.Image, port: 3000, replicas: replicas}
			nn := types.NamespacedName{Name: name, Namespace: namespace}

			// The operator before w6/m633 wrote no strategy for this service.
			before := &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
			applyDeploymentSpec(before, app, params)
			before.Spec.Strategy = appsv1.DeploymentStrategy{}
			Expect(k8sClient.Create(ctx, before)).To(Succeed())
			legacy := envtestDeployment(nn)
			Expect(legacy.Spec.Strategy).To(Equal(apiServerDefaultStrategy()),
				"the API server's default is what every such Deployment carries on production")

			after := legacy.DeepCopy()
			applyDeploymentSpec(after, app, params)
			Expect(k8sClient.Update(ctx, after)).To(Succeed())
			adopted := envtestDeployment(nn)
			keepsEveryOldInstance(adopted)
			Expect(adopted.Spec.Template).To(Equal(legacy.Spec.Template),
				"the strategy write must leave the template, and so its pod-template hash, unchanged: it rolls nothing")

			again := adopted.DeepCopy()
			applyDeploymentSpec(again, app, params)
			Expect(k8sClient.Update(ctx, again)).To(Succeed())
			Expect(envtestDeployment(nn).ResourceVersion).To(Equal(adopted.ResourceVersion),
				"the API server must store the strategy as projected, or every pass would write it again")
			Expect(k8sClient.Delete(ctx, adopted)).To(Succeed())
		}
	})

	It("writes every deploy, restart and failing release of a multi-instance service under maxUnavailable: 0", func() {
		for _, replicas := range []int32{4, appv1alpha1.MaxReplicas} {
			name := fmt.Sprintf("rolling-roll-%d", replicas)
			By(name)
			r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
			nn := types.NamespacedName{Name: name, Namespace: namespace}
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace},
				Spec: appv1alpha1.AppSpec{
					Image: "ghcr.io/example/app:v1", Port: 3000, Tier: "standard", HealthCheckPath: "/healthz", Replicas: replicas,
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
			reconcileToFixedPoint(run)
			first := envtestDeployment(nn)
			Expect(first.Spec.Replicas).To(HaveValue(Equal(replicas)))
			keepsEveryOldInstance(first)
			serveEnvtestRelease(nn, func() { reconcileToFixedPoint(run) })

			By("a code deploy")
			current := envtestApp(nn)
			current.Spec.Image = "ghcr.io/example/app:v2"
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
			reconcileToFixedPoint(run)
			deployed := envtestDeployment(nn)
			Expect(deployed.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/example/app:v2"))
			keepsEveryOldInstance(deployed)
			serveEnvtestRelease(nn, func() { reconcileToFixedPoint(run) })

			By("a restart")
			current = envtestApp(nn)
			current.Spec.RestartedAt = "2026-11-25T00:00:00Z"
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
			reconcileToFixedPoint(run)
			restarted := envtestDeployment(nn)
			Expect(restarted.Spec.Template.Annotations).To(HaveKey("app.bex.co/restarted-at"))
			keepsEveryOldInstance(restarted)
			serveEnvtestRelease(nn, func() { reconcileToFixedPoint(run) })

			By("a release whose instances never become healthy")
			current = envtestApp(nn)
			current.Spec.Image = "ghcr.io/example/app:broken"
			Expect(k8sClient.Update(ctx, current)).To(Succeed())
			reconcileToFixedPoint(run)
			failing := envtestDeployment(nn)
			Expect(failing.Spec.Template.Spec.Containers[0].Image).To(Equal("ghcr.io/example/app:broken"))
			keepsEveryOldInstance(failing)
			Expect(envtestApp(nn).Status.ActiveRevision).NotTo(Equal(failing.Spec.Template.Labels[labelRevision]),
				"the unhealthy release has not served; the old instances still do")

			deleteOwner(ctx, envtestApp(nn), run)
		}
	})
})
