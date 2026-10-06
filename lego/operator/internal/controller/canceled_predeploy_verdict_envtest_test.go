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
	"strconv"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/085: a cancel leaves a running pre-deploy step to finish, so the step's
// outcome is still the canceled release's record — bex-api projects it onto the
// closed deploy, which otherwise reads "canceled" for a migration that ran. The
// operator keeps reading the Job until it ends and records the verdict under
// the canceled release's generation; nothing else moves.
var _ = Describe("A canceled release's running pre-deploy step (w5/085)", func() {
	// Each spec has its own App: envtest runs no finalizer, so a deleted App
	// keeps its name.
	var nn types.NamespacedName
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	pass := func() reconcile.Result {
		GinkgoHelper()
		res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		return res
	}
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := envtestApp(nn)
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	stampRelease := func(app *appv1alpha1.App) {
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(app.Generation, 10))
	}
	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
		}
	})

	// startStep runs the pass that starts the release's pre-deploy step.
	startStep := func() (int64, *batchv1.Job) {
		GinkgoHelper()
		release := envtestApp(nn).Generation
		pass()
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: predeploy.JobName(nn.Name, appv1alpha1.BuildRevision(release)), Namespace: "default",
		}, job)).To(Succeed(), "precondition: the release's pre-deploy started")
		Expect(envtestApp(nn).Status.PreDeploy.Status).To(Equal(appv1alpha1.PreDeployRunning))
		return release, job
	}
	// cancel stamps the release canceled as the backend's Cancel (or a suspend
	// that ended the rollout) does. The step still runs, so the pass records no
	// verdict, and its result says when the step is read again.
	cancel := func(release int64) reconcile.Result {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationCanceledReleaseGeneration, strconv.FormatInt(release, 10))
		})
		res := pass()
		Expect(envtestApp(nn).Status.PreDeploy.Status).To(Equal(appv1alpha1.PreDeployRunning))
		return res
	}
	// serveThenStartStep serves release 1, then deploys release 2, whose
	// pre-deploy step starts.
	serveThenStartStep := func(name string) (int64, *batchv1.Job) {
		GinkgoHelper()
		nn = types.NamespacedName{Name: name, Namespace: "default"}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1},
		})).To(Succeed())
		for range 3 {
			pass()
		}
		markEnvtestDeploymentReady(nn)
		pass()
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-1"), "precondition: release 1 served")
		update(func(app *appv1alpha1.App) {
			app.Spec.Image, app.Spec.PreDeployCommand = "nginx:2", "migrate"
		})
		update(stampRelease)
		return startStep()
	}
	expectServedReleaseUntouched := func() {
		GinkgoHelper()
		app := envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.Phase).NotTo(Equal(appv1alpha1.PhaseFailed))
		Expect(meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady).Reason).
			NotTo(Equal(appv1alpha1.ReasonPreDeployFailed))
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:1"))
	}

	It("records a step that succeeds after the cancel, and still serves the prior release", func() {
		release, job := serveThenStartStep("canceled-step-succeeds")
		Expect(cancel(release).RequeueAfter).To(BeNumerically("~", childHealthRequeue/2, childHealthRequeue/2),
			"a canceled step still running is read again")

		finished := metav1.NewTime(time.Now().Add(-time.Minute))
		completeEnvtestJob(job, finished)
		pass()

		pd := envtestApp(nn).Status.PreDeploy
		Expect(pd.Generation).To(Equal(release))
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeploySucceeded))
		Expect(pd.FinishedAt).To(Equal(finished.UTC().Format(time.RFC3339)), "dated when the Job finished, not when it was read")
		expectServedReleaseUntouched()
	})

	It("records a step that fails after the cancel without failing the service", func() {
		release, job := serveThenStartStep("canceled-step-fails")
		cancel(release)

		failEnvtestJob(job, batchv1.JobReasonBackoffLimitExceeded, "Job has reached the specified backoff limit")
		pass()

		pd := envtestApp(nn).Status.PreDeploy
		Expect(pd.Generation).To(Equal(release))
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeployFailed))
		Expect(pd.Message).NotTo(BeEmpty())
		expectServedReleaseUntouched()
	})

	It("reads the step of a release a suspend ended, though a parked pass requeues nothing", func() {
		release, job := serveThenStartStep("suspended-step")
		update(func(app *appv1alpha1.App) { app.Spec.Suspended = true })
		pass()
		Expect(cancel(release).RequeueAfter).To(Equal(childHealthRequeue))

		completeEnvtestJob(job, metav1.Now())
		pass()

		pd := envtestApp(nn).Status.PreDeploy
		Expect(pd.Generation).To(Equal(release))
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeploySucceeded))
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:1"))
	})

	It("records the step of a canceled first release, which stays canceled", func() {
		nn = types.NamespacedName{Name: "canceled-first-step", Namespace: "default"}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: "default"},
			Spec:       appv1alpha1.AppSpec{Image: "nginx:1", Port: 3000, Replicas: 1, PreDeployCommand: "migrate"},
		})).To(Succeed())
		pass() // the finalizer
		update(stampRelease)
		release, job := startStep()
		Expect(cancel(release).RequeueAfter).To(Equal(childHealthRequeue), "nothing else requeues a canceled first release")
		Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseCanceled))

		completeEnvtestJob(job, metav1.Now())
		Expect(pass().RequeueAfter).To(BeZero(), "nothing left to read")

		app := envtestApp(nn)
		Expect(app.Status.PreDeploy.Generation).To(Equal(release))
		Expect(app.Status.PreDeploy.Status).To(Equal(appv1alpha1.PreDeploySucceeded))
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseCanceled))
	})
})
