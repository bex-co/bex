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
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	controllerconfig "sigs.k8s.io/controller-runtime/pkg/config"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m185: an Environment's isolation toggle changes only the App's
// network-isolation label, which bumps no generation, so the manager filtered
// the event and the saved boundary waited for an unrelated reconcile, up to a
// Free service's 15-minute idle timer. The label edit alone must converge the
// owned policy and the pod template, well inside the ordinary requeue.
var _ = Describe("App network isolation label", func() {
	It("converges the protected policy and pod labels from a label-only edit", func() {
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

		nn := types.NamespacedName{Name: "isolation-label", Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), app) })
		dep := &appsv1.Deployment{}
		Eventually(func() error { return k8sClient.Get(ctx, nn, dep) }, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		Expect(dep.Spec.Template.Labels).NotTo(HaveKey(labelNetworkIsolation))
		// Settle the App to Running so the manager is idle on its ordinary
		// requeue; a Deploying App polls and would mask a filtered event.
		dep.Status.ObservedGeneration = dep.Generation
		dep.Status.Replicas, dep.Status.UpdatedReplicas = 1, 1
		dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1
		Expect(k8sClient.Status().Update(ctx, dep)).To(Succeed())
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name + "-pod", Namespace: nn.Namespace, Labels: dep.Spec.Template.Labels},
			Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "app", Image: "nginx:1"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(context.Background(), pod) })
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, pod)).To(Succeed())
		pod.Status.Phase = corev1.PodRunning
		pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		Eventually(func(g Gomega) {
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(current.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
		time.Sleep(2 * time.Second) // let the settling passes drain

		setLabel := func(value string) int64 {
			current := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			base := current.DeepCopy()
			if current.Labels == nil {
				current.Labels = map[string]string{}
			}
			if value == "" {
				delete(current.Labels, labelNetworkIsolation)
			} else {
				current.Labels[labelNetworkIsolation] = value
			}
			Expect(k8sClient.Patch(ctx, current, client.MergeFrom(base))).To(Succeed())
			return current.Generation
		}

		generation := setLabel("evm-a")
		Eventually(func(g Gomega) {
			np := &networkingv1.NetworkPolicy{}
			g.Expect(k8sClient.Get(ctx, nn, np)).To(Succeed())
			g.Expect(np.Spec.PodSelector.MatchLabels).To(HaveKeyWithValue(labelApp, nn.Name))
			g.Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			g.Expect(dep.Spec.Template.Labels).To(HaveKeyWithValue(labelNetworkIsolation, "evm-a"))
			current := &appv1alpha1.App{}
			g.Expect(k8sClient.Get(ctx, nn, current)).To(Succeed())
			g.Expect(current.Generation).To(Equal(generation), "a boundary change is not a release")
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())

		setLabel("")
		Eventually(func(g Gomega) {
			err := k8sClient.Get(ctx, nn, &networkingv1.NetworkPolicy{})
			g.Expect(apierrors.IsNotFound(err)).To(BeTrue(), "policy must be removed: %v", err)
			g.Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			g.Expect(dep.Spec.Template.Labels).NotTo(HaveKey(labelNetworkIsolation))
		}, 10*time.Second, 100*time.Millisecond).Should(Succeed())
	})
})
