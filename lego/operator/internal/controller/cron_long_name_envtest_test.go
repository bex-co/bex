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
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/types/k8sname"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w7/m154: the real API server rejects CronJob names over 52 characters, which
// the fake client never enforced — a 54-character cron App reported Failed on
// every reconcile in production.
var _ = Describe("Cron App names at the CronJob length boundary", func() {
	reconcileCron := func(name string) (*AppReconciler, types.NamespacedName) {
		GinkgoHelper()
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace},
			Spec:       appv1alpha1.AppSpec{Type: appv1alpha1.TypeCronJob, Image: "busybox:1.37", Schedule: "* * * * *", Command: "echo hi"},
		})).To(Succeed())
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, Registry: "zot.test:5000"}
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
		return r, nn
	}

	It("keeps a name that fits as the CronJob's identity", func() {
		name := "fit-" + strings.Repeat("a", k8sname.MaxCronJob-4)
		_, nn := reconcileCron(name)
		Expect(k8sClient.Get(ctx, nn, &batchv1.CronJob{})).To(Succeed())
	})

	It("schedules, suspends, resumes and manually runs a longer accepted name", func() {
		const name = "tea-daif693dqjvc73e7as3g-qa-20260922-cd9361-longcronxx"
		r, nn := reconcileCron(name)
		pass := func() {
			GinkgoHelper()
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
		cronKey := types.NamespacedName{Name: appv1alpha1.CronJobName(name), Namespace: nn.Namespace}
		Expect(len(cronKey.Name)).To(BeNumerically("<=", k8sname.MaxCronJob))
		var cron batchv1.CronJob
		Expect(k8sClient.Get(ctx, cronKey, &cron)).To(Succeed())
		Expect(cron.OwnerReferences).To(ContainElement(HaveField("Name", name)))
		var crons batchv1.CronJobList
		Expect(k8sClient.List(ctx, &crons)).To(Succeed())
		owned := 0
		for _, c := range crons.Items {
			for _, o := range c.OwnerReferences {
				if o.Name == name {
					owned++
				}
			}
		}
		Expect(owned).To(Equal(1), "repeated reconciles must not create a second schedule")

		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))

		for _, suspended := range []bool{true, false} {
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			app.Spec.Suspended = suspended
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			pass()
			Expect(k8sClient.Get(ctx, cronKey, &cron)).To(Succeed())
			Expect(cron.Spec.Suspend).To(HaveValue(Equal(suspended)))
		}

		By("materializing a manual run whose Job name fits a label")
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		app.Spec.RunAt = "2026-09-28T07:00:00Z"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		pass()
		jobName := appv1alpha1.ManualCronRunJobName(name, app.Spec.RunAt)
		Expect(len(jobName)).To(BeNumerically("<=", k8sname.MaxLabel))
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: nn.Namespace}, &batchv1.Job{})).To(Succeed())
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		Expect(app.Status.Runs).To(ContainElement(HaveField("Name", jobName)))
		Expect(k8sClient.Get(ctx, cronKey, &cron)).To(Succeed())
		Expect(cron.Spec.Suspend).To(HaveValue(BeTrue()), "the schedule pauses while the manual run is active")
	})
})
