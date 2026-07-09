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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

var _ = Describe("Additional service types (kubernetes runtime)", func() {
	ctx := context.Background()

	newReconciler := func() *AppReconciler {
		return &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	}
	reconcileN := func(r *AppReconciler, nn types.NamespacedName) {
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}

	Context("background_worker", func() {
		const name = "worker-app"
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		var r *AppReconciler
		BeforeEach(func() { r = newReconciler() })
		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN(r, nn)
			}
		})

		It("reconciles to a Deployment only — no Service, no Ingress, no URL", func() {
			By("creating a background_worker App")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Type:  appv1alpha1.TypeBackgroundWorker,
					Image: "traefik/whoami", Port: 3000,
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN(r, nn)

			By("a Deployment exists with no container port")
			dep := &appsv1.Deployment{}
			Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
			Expect(dep.Spec.Template.Spec.Containers[0].Ports).To(BeEmpty())

			By("no Service and no Ingress are created")
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, &corev1.Service{}))).To(BeTrue())
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, &networkingv1.Ingress{}))).To(BeTrue())

			By("status carries no URL")
			got := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
			Expect(got.Status.URL).To(BeEmpty())
			Expect(got.Status.URLs).To(BeEmpty())
		})
	})

	Context("cron_job", func() {
		const name = "cron-app"
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		var r *AppReconciler
		BeforeEach(func() { r = newReconciler() })
		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcileN(r, nn)
			}
			// envtest has no GC controller, so owned CronJob/Jobs aren't collected.
			_ = k8sClient.Delete(ctx, &batchv1.CronJob{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"}})
		})

		It("reconciles to a CronJob (no Deployment/Service/Ingress) and records run history", func() {
			By("creating a cron_job App with a schedule")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Type:     appv1alpha1.TypeCronJob,
					Schedule: "*/5 * * * *",
					Image:    "busybox:latest", Port: 3000,
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcileN(r, nn)

			By("a CronJob exists carrying the schedule; no Deployment/Service/Ingress")
			cj := &batchv1.CronJob{}
			Expect(k8sClient.Get(ctx, nn, cj)).To(Succeed())
			Expect(cj.Spec.Schedule).To(Equal("*/5 * * * *"))
			Expect(cj.Spec.JobTemplate.Spec.Template.Spec.Containers[0].Image).To(Equal("busybox:latest"))
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, &appsv1.Deployment{}))).To(BeTrue())
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, &corev1.Service{}))).To(BeTrue())
			Expect(errors.IsNotFound(k8sClient.Get(ctx, nn, &networkingv1.Ingress{}))).To(BeTrue())

			By("triggering a one-off run via spec.runAt materializes a labeled Job")
			app = &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
			app.Spec.RunAt = "2026-07-09T10:00:00Z"
			Expect(k8sClient.Update(ctx, app)).To(Succeed())
			reconcileN(r, nn)

			jobName := manualRunJobName(name, "2026-07-09T10:00:00Z")
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, types.NamespacedName{Name: jobName, Namespace: "default"}, job)).To(Succeed())
			Expect(job.Labels).To(HaveKeyWithValue(labelApp, name))

			By("run history shows the run once the Job reports Complete")
			now := metav1.Now()
			job.Status.StartTime = &now
			job.Status.CompletionTime = &now
			job.Status.Conditions = []batchv1.JobCondition{
				{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue, LastTransitionTime: now},
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue, LastTransitionTime: now},
			}
			Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
			reconcileN(r, nn)

			got := &appv1alpha1.App{}
			Expect(k8sClient.Get(ctx, nn, got)).To(Succeed())
			Expect(got.Status.Runs).To(HaveLen(1))
			Expect(got.Status.Runs[0].Name).To(Equal(jobName))
			Expect(got.Status.Runs[0].Status).To(Equal("Succeeded"))
			Expect(got.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
			Expect(got.Status.URL).To(BeEmpty())
		})
	})
})
