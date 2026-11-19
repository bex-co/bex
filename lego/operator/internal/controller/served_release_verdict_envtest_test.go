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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/042: the recovery write, through a whole reconcile against an API
// server. A served release that could not pull its image carries the Rollout
// verdict settleFailedRolloutMessage records for it, at its own generation,
// with the transition time an earlier failed rollout left (SetStatusCondition
// keeps it). Once its instances are ready again the verdict is dropped, so
// the next failure is written with a transition time of its own, which is what
// bex-api keys that failure's image_pull_failed by.
var _ = Describe("A served release that recovers from an image pull failure (w32/042)", func() {
	It("drops its own Rollout verdict once its instances are ready again", func() {
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		nn := types.NamespacedName{Name: "served-pull-recovery", Namespace: "default"}
		reconcileN := func() {
			for range 3 {
				_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
				Expect(err).NotTo(HaveOccurred())
			}
		}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000},
		})).To(Succeed())
		DeferCleanup(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN()
			}
		})
		reconcileN()
		serveEnvtestRelease(nn, reconcileN)

		By("the served release's instance could not pull its image")
		app := envtestApp(nn)
		served := app.Status.ReleaseGeneration
		earlier := metav1.NewTime(time.Now().Add(-21 * 24 * time.Hour).Truncate(time.Second))
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: imagePullBackOff,
			Message:            "image pull failed: the build this release runs on is no longer stored",
			ObservedGeneration: served, LastTransitionTime: earlier,
		})
		meta.SetStatusCondition(&app.Status.Conditions, metav1.Condition{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: reasonPriorReleaseServing,
			Message:            "the latest rollout failed: image pull failed; the previously deployed release keeps serving",
			ObservedGeneration: app.Generation,
		})
		Expect(k8sClient.Status().Update(ctx, app)).To(Succeed())

		By("its instances are ready again")
		markEnvtestDeploymentReady(nn)
		reconcileN()

		recovered := envtestApp(nn)
		Expect(recovered.Status.ActiveRevision).To(Equal(app.Status.ActiveRevision), "the same release serves")
		Expect(recovered.Status.ReleaseGeneration).To(Equal(served))
		Expect(meta.FindStatusCondition(recovered.Status.Conditions, appv1alpha1.ConditionRollout)).To(BeNil(),
			"the served release's verdict describes an outage that is over")
		Expect(recovered.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		ready := meta.FindStatusCondition(recovered.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal("Deployed"))
	})
})
