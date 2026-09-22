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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Cron controls over a failed newer build", func() {
	It("suspends and resumes the prior CronJob without promoting the failed release", func() {
		const name = "cron-pending-artifact"
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob, Image: "busybox:1.37", Schedule: "* * * * *", Command: "echo prior"},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, Registry: "zot.test:5000"}
		pass := func() {
			GinkgoHelper()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
		for range 3 {
			pass()
		}
		var prior batchv1.CronJob
		Expect(k8sClient.Get(ctx, nn, &prior)).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))

		By("dispatching and recording the failed second build through the real API server")
		app.Spec.Image = ""
		app.Spec.Repo = "https://github.com/bex-co/hello"
		app.Spec.Builder = "dockerfile"
		app.Spec.Command = "echo unbuilt"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		pass()
		failBuildJob(name, "gen-2")
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).To(HaveOccurred())
		pass()

		for _, suspended := range []bool{true, false} {
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			app.Spec.Suspended = suspended
			app.Spec.Schedule = "*/5 * * * *"
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			pass()
			var cron batchv1.CronJob
			Expect(k8sClient.Get(ctx, nn, &cron)).To(Succeed())
			Expect(cron.Spec.Suspend).To(HaveValue(Equal(suspended)))
			Expect(cron.Spec.Schedule).To(Equal("*/5 * * * *"))
			Expect(cron.Spec.JobTemplate).To(Equal(prior.Spec.JobTemplate), "the pending command must not reach the old image")
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
			Expect(app.Status.Image).To(Equal("busybox:1.37"))
			Expect(app.Status.ReleaseGeneration).To(Equal(int64(2)))
			failure := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionBuild)
			Expect(failure).NotTo(BeNil())
			Expect(failure.Status).To(Equal(metav1.ConditionFalse))
			Expect(failure.ObservedGeneration).To(Equal(int64(2)))
			phase := appv1alpha1.PhaseRunning
			if suspended {
				phase = appv1alpha1.PhaseHibernated
			}
			Expect(app.Status.Phase).To(Equal(phase))
		}
	})
})
