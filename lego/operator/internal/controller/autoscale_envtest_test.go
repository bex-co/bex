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
	"errors"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m364 through the real reconcile loop: a refused metrics read is visible on
// the App, a manual scale while autoscaling is on is stored but leaves the
// running count to the autoscaler, the autoscaler scales up once reads work,
// and turning autoscaling off runs the stored count.
var _ = Describe("Autoscaling with a refused metrics read (w4/m364)", func() {
	It("surfaces the refusal, ignores a manual scale at runtime, and scales once reads work", func() {
		var refused atomic.Bool
		refused.Store(true)
		limit, _ := tierLimits("starter")
		reader := func(context.Context, string, string) ([]PodUsage, error) {
			if refused.Load() {
				return nil, errors.New(`pods.metrics.k8s.io is forbidden`)
			}
			// Two pods at their CPU limit, twice the 50% target: four replicas.
			return []PodUsage{{Pod: "a", CPUCores: limit}, {Pod: "b", CPUCores: limit}}, nil
		}

		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: k8sClient.Scheme(), Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&AppReconciler{
			Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Mode: ModeKubernetes, MetricsReader: reader,
		}).SetupWithManager(mgr)).To(Succeed())
		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			stop()
			Eventually(done, 10*time.Second).Should(Receive(BeNil()))
		})

		nn := types.NamespacedName{Name: "autoscale-metrics-refused", Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: "nginx:1", Port: 3000, Replicas: 2, Tier: "starter",
				Autoscaling: &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 1, MaxReplicas: 4, TargetCPUPercent: new(int32(50))},
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), app) })

		deploymentReplicas := func(g Gomega) int32 {
			dep := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			g.Expect(dep.Spec.Replicas).NotTo(BeNil())
			return *dep.Spec.Replicas
		}
		metricsCondition := func(g Gomega) *metav1.Condition {
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			return meta.FindStatusCondition(current.Status.Conditions, appv1alpha1.ConditionAutoscalingMetricsAvailable)
		}
		updateApp := func(mutate func(*appv1alpha1.App)) {
			Eventually(func() error {
				current := &appv1alpha1.App{}
				if err := k8sClient.Get(ctx, nn, current); err != nil {
					return err
				}
				mutate(current)
				return k8sClient.Update(ctx, current)
			}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		}

		By("the refused read holds the count, records it, and says so on the App")
		Eventually(func(g Gomega) {
			g.Expect(deploymentReplicas(g)).To(Equal(int32(2)))
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(current.Annotations).To(HaveKeyWithValue(annotAutoscaleReplicas, "2"))
			cond := metricsCondition(g)
			g.Expect(cond).NotTo(BeNil())
			g.Expect(cond.Status).To(Equal(metav1.ConditionFalse))
			g.Expect(cond.Reason).To(Equal(appv1alpha1.AutoscalingMetricsUnavailableReason))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		By("a manual scale to 3 is stored and leaves the running count")
		updateApp(func(a *appv1alpha1.App) { a.Spec.Replicas = 3 })
		Consistently(func(g Gomega) int32 { return deploymentReplicas(g) }, 3*time.Second, 200*time.Millisecond).Should(Equal(int32(2)))

		By("once reads work the autoscaler scales up and the condition clears")
		refused.Store(false)
		// Any spec write starts a pass; the autoscaler's own 30 s poll would too.
		updateApp(func(a *appv1alpha1.App) { a.Spec.Autoscaling.MaxReplicas = 5 })
		Eventually(func(g Gomega) {
			g.Expect(deploymentReplicas(g)).To(Equal(int32(4)))
			g.Expect(metricsCondition(g)).To(BeNil())
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		By("turning autoscaling off runs the stored manual count")
		updateApp(func(a *appv1alpha1.App) { a.Spec.Autoscaling.Enabled = false })
		Eventually(func(g Gomega) int32 { return deploymentReplicas(g) }, 10*time.Second, 100*time.Millisecond).Should(Equal(int32(3)))
	})
})
