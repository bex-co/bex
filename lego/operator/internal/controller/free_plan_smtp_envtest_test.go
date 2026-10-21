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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/bex-co/bex/lego/operator/internal/execution"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m127 t002: a plan change converges the free-plan label on the pod
// template in both directions, so the SMTP egress deny follows the plan.
var _ = Describe("App free-plan SMTP label", func() {
	It("adds the label on free, drops it on upgrade and restores it on downgrade", func() {
		skipNameValidation := true
		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: k8sClient.Scheme(), Metrics: metricsserver.Options{BindAddress: "0"},
			Controller: controllerconfig.Controller{SkipNameValidation: &skipNameValidation},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect((&AppReconciler{Client: mgr.GetClient(), Scheme: mgr.GetScheme(), Mode: ModeKubernetes}).SetupWithManager(mgr)).To(Succeed())
		managerCtx, stop := context.WithCancel(ctx)
		done := make(chan error, 1)
		go func() { done <- mgr.Start(managerCtx) }()
		DeferCleanup(func() {
			stop()
			Eventually(done, 10*time.Second).Should(Receive(BeNil()))
		})

		nn := types.NamespacedName{Name: "free-plan-smtp", Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1, Tier: tierFree},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), app) })

		templateLabels := func(g Gomega) map[string]string {
			dep := &appsv1.Deployment{}
			g.Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			return dep.Spec.Template.Labels
		}
		setTier := func(tier string) {
			current := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			base := current.DeepCopy()
			current.Spec.Tier = tier
			Expect(k8sClient.Patch(ctx, current, client.MergeFrom(base))).To(Succeed())
		}

		Eventually(func(g Gomega) {
			g.Expect(templateLabels(g)).To(HaveKeyWithValue(execution.LabelFreePlan, execution.FreePlanValue))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		setTier("starter")
		Eventually(func(g Gomega) {
			g.Expect(templateLabels(g)).NotTo(HaveKey(execution.LabelFreePlan))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		setTier(tierFree)
		Eventually(func(g Gomega) {
			g.Expect(templateLabels(g)).To(HaveKeyWithValue(execution.LabelFreePlan, execution.FreePlanValue))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
	})
})
