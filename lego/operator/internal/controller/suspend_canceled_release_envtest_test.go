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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w5/m114: a user suspend that interrupts a rollout now ends its release — the
// backend stamps it canceled the way Cancel does. The suspend's own spec write
// has already moved metadata.generation past the release by then, so the stamp
// must still read as the latest request, or Resume rolls the release (and runs
// its pre-deploy) that the deploy history reads as canceled.
var _ = Describe("Suspending mid-rollout, then resuming (w5/m114)", func() {
	const name = "suspend-mid-rollout"
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	pass := func() {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := envtestApp(nn)
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			Expect(k8sClient.Delete(ctx, app)).To(Succeed())
		}
	})

	It("resumes the served release, never the interrupted one", func() {
		By("serving release 1")
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

		By("deploying release 2, whose pre-deploy step starts")
		update(func(app *appv1alpha1.App) {
			app.Spec.Image, app.Spec.PreDeployCommand = "nginx:2", "migrate"
		})
		update(func(app *appv1alpha1.App) {
			app.Annotations = map[string]string{appv1alpha1.AnnotationReleaseGeneration: strconv.FormatInt(app.Generation, 10)}
		})
		release := envtestApp(nn).Generation
		pass()
		Expect(envtestApp(nn).Status.ReleaseGeneration).To(Equal(release), "precondition: release 2 adopted mid-rollout")
		migration := types.NamespacedName{Name: predeploy.JobName(name, appv1alpha1.BuildRevision(release)), Namespace: "default"}
		Expect(k8sClient.Get(ctx, migration, &batchv1.Job{})).To(Succeed(), "precondition: release 2's pre-deploy started")

		By("suspending, then the backend's cancel stamp for release 2")
		update(func(app *appv1alpha1.App) { app.Spec.Suspended = true })
		pass()
		update(func(app *appv1alpha1.App) {
			app.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = strconv.FormatInt(release, 10)
		})
		for range 2 {
			pass()
		}

		By("resuming; the pre-deploy that was already running finishes")
		update(func(app *appv1alpha1.App) { app.Spec.Suspended = false })
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, migration, job)).To(Succeed())
		now := metav1.Now()
		job.Status.StartTime, job.Status.CompletionTime = &now, &now
		job.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		}
		Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
		for range 3 {
			pass()
		}
		markEnvtestDeploymentReady(nn)
		pass()

		By("release 1 serves again; release 2 never reaches the Deployment, even with its pre-deploy passed")
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(Equal("nginx:1"))
		app := envtestApp(nn)
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))
	})
})
