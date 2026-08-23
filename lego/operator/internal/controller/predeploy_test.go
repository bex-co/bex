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

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The pre-deploy gate (w1/m33): spec.preDeployCommand runs as a Job against the
// new revision's image and the rollout waits on it. envtest has no kubelet, so
// the Job never runs on its own — the specs drive its terminal condition by
// hand to exercise the success and failure branches, matching the build
// completion specs.
var _ = Describe("Pre-deploy gate (kubernetes runtime)", func() {
	ctx := context.Background()

	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})

	reconcile1 := func(nn types.NamespacedName) {
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
	getApp := func(nn types.NamespacedName) *appv1alpha1.App {
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		return app
	}
	getDep := func(nn types.NamespacedName) (*appsv1.Deployment, error) {
		dep := &appsv1.Deployment{}
		err := k8sClient.Get(ctx, nn, dep)
		return dep, err
	}
	// completeJob / failJob drive a pre-deploy Job to a terminal condition,
	// standing in for the pod outcome envtest can't produce. The companion
	// conditions (SuccessCriteriaMet / FailureTarget) and the completionTime rule
	// are what the apiserver's Job status validation requires.
	completeJob := func(nn types.NamespacedName) {
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, nn, job)).To(Succeed())
		now := metav1.Now()
		job.Status.StartTime = &now
		job.Status.CompletionTime = &now
		job.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		}
		Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
	}
	failJob := func(nn types.NamespacedName, msg string) {
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, nn, job)).To(Succeed())
		now := metav1.Now()
		job.Status.StartTime = &now
		job.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: "Test", Message: msg},
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "Test", Message: msg},
		}
		Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
	}

	Context("when spec.preDeployCommand is set", func() {
		const name = "predeploy-app"
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		jobNN := types.NamespacedName{Name: predeploy.JobName(name, "gen-1"), Namespace: "default"}

		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcile1(nn) // release the finalizer
			}
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: jobNN.Name, Namespace: "default"}})
		})

		It("gates the rollout on the Job, then deploys once it succeeds", func() {
			By("creating a prebuilt-image App with a pre-deploy command")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000,
					PreDeployCommand: "echo migrating",
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			By("reconciling: the pre-deploy Job is created and the Deployment is withheld")
			reconcile1(nn) // finalizer
			reconcile1(nn) // pre-deploy gate
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, jobNN, job)).To(Succeed())
			Expect(job.Spec.Template.Spec.Containers[0].Image).To(Equal("traefik/whoami"))
			Expect(job.Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"sh", "-c", "echo migrating"}))
			_, err := getDep(nn)
			Expect(errors.IsNotFound(err)).To(BeTrue(), "Deployment must not roll until pre-deploy succeeds")
			Expect(getApp(nn).Status.PreDeploy.Status).To(Equal(appv1alpha1.PreDeployRunning))

			By("the Job succeeding: the Deployment now rolls to the image")
			completeJob(jobNN)
			reconcile1(nn)
			dep, err := getDep(nn)
			Expect(err).NotTo(HaveOccurred())
			Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("traefik/whoami"))
			got := getApp(nn)
			Expect(got.Status.PreDeploy.Status).To(Equal(appv1alpha1.PreDeploySucceeded))
			Expect(got.Status.PreDeploy.FinishedAt).NotTo(BeEmpty())
		})
	})

	Context("when a pre-deploy fails after a prior revision is live", func() {
		const name = "predeploy-fail-app"
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		job2NN := types.NamespacedName{Name: predeploy.JobName(name, "gen-2"), Namespace: "default"}

		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcile1(nn)
			}
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: job2NN.Name, Namespace: "default"}})
		})

		It("fails the deploy and leaves the previous revision serving", func() {
			By("deploying revision 1 with no pre-deploy step")
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami:v1", Port: 3000},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcile1(nn) // finalizer
			reconcile1(nn) // rollout
			dep, err := getDep(nn)
			Expect(err).NotTo(HaveOccurred())
			Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("traefik/whoami:v1"))

			By("updating to revision 2: a new image plus a pre-deploy command")
			app = getApp(nn)
			app.Spec.Image = "traefik/whoami:v2"
			app.Spec.PreDeployCommand = "exit 1"
			Expect(k8sClient.Update(ctx, app)).To(Succeed())

			By("reconciling: the Deployment still runs v1 while the migration is pending")
			reconcile1(nn)
			dep, err = getDep(nn)
			Expect(err).NotTo(HaveOccurred())
			Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("traefik/whoami:v1"))

			By("the migration failing: the deploy fails and v1 keeps serving")
			failJob(job2NN, "boom")
			_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).To(HaveOccurred()) // a failed migration surfaces as a reconcile error
			dep, derr := getDep(nn)
			Expect(derr).NotTo(HaveOccurred())
			Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("traefik/whoami:v1"),
				"the previous revision must stay live on a pre-deploy failure")
			got := getApp(nn)
			Expect(got.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
			Expect(got.Status.PreDeploy.Status).To(Equal(appv1alpha1.PreDeployFailed))
			Expect(got.Status.PreDeploy.Message).To(ContainSubstring("boom"))
		})
	})

	Context("when BEX_BUILD_NAMESPACE is set", func() {
		const name = "predeploy-colocated-app"
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		jobNN := types.NamespacedName{Name: predeploy.JobName(name, "gen-1"), Namespace: "default"}

		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcile1(nn) // release the finalizer
			}
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: jobNN.Name, Namespace: "default"}})
		})

		It("still runs the Job in the App's own namespace (ADR043 D8)", func() {
			// A build namespace no longer routes the pre-deploy step: a Job in the
			// shared build namespace cannot reach the workspace's managed Postgres
			// through the tenant default-deny, so the migration's connections timed
			// out there. The Job runs beside the App instead — no secret mirroring,
			// no cross-namespace egress exception.
			r.BuildNamespace = "bex-build"
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec: appv1alpha1.AppSpec{
					Image: "traefik/whoami", Port: 3000,
					PreDeployCommand: "echo migrating",
					EnvFromSecret:    "predeploy-colocated-env",
				},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())

			reconcile1(nn) // finalizer
			reconcile1(nn) // pre-deploy gate
			job := &batchv1.Job{}
			Expect(k8sClient.Get(ctx, jobNN, job)).To(Succeed(), "the Job must be co-located with the App")
			err := k8sClient.Get(ctx, types.NamespacedName{Name: jobNN.Name, Namespace: "bex-build"}, &batchv1.Job{})
			Expect(errors.IsNotFound(err)).To(BeTrue(), "no pre-deploy Job may be created in the build namespace")

			By("not mirroring the referenced env Secret into the build namespace")
			err = k8sClient.Get(ctx, types.NamespacedName{Name: "predeploy-colocated-env", Namespace: "bex-build"}, &corev1.Secret{})
			Expect(errors.IsNotFound(err)).To(BeTrue())

			By("not creating the cross-namespace execution egress exception")
			err = k8sClient.Get(ctx, types.NamespacedName{
				Name: build.JobName(name, "execution-egress"), Namespace: "bex-build",
			}, &networkingv1.NetworkPolicy{})
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})

	Context("when spec.preDeployCommand is unset", func() {
		const name = "no-predeploy-app"
		nn := types.NamespacedName{Name: name, Namespace: "default"}

		AfterEach(func() {
			if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
				Expect(k8sClient.Delete(ctx, app)).To(Succeed())
				reconcile1(nn)
			}
		})

		It("rolls the Deployment immediately with no pre-deploy Job", func() {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
				Spec:       appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000},
			}
			Expect(k8sClient.Create(ctx, app)).To(Succeed())
			reconcile1(nn) // finalizer
			reconcile1(nn) // rollout
			dep, err := getDep(nn)
			Expect(err).NotTo(HaveOccurred())
			Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal("traefik/whoami"))
			Expect(getApp(nn).Status.PreDeploy).To(BeNil())
			// No pre-deploy Job exists for an unset command.
			job := &batchv1.Job{}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: predeploy.JobName(name, "gen-1"), Namespace: "default"}, job)
			Expect(errors.IsNotFound(err)).To(BeTrue())
		})
	})
})
