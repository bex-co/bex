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
	"fmt"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/289: after a failed build, a save-only PATCH (Render's Update Service)
// replaced the serving instance of a web service with no deploy. The failed
// build leaves status.releaseGeneration on its generation; the save's pending
// marker skips buildFromSource and its failed-build hold, and the serving image
// was dispatched under the failed generation's revision, a new pod template.
var _ = Describe("A save after a failed build (w8/289)", func() {
	for _, tc := range []struct {
		name, serviceType string
		preDeploy         bool // the release planner's pre-deploy gate path
		canceledFirst     bool // a deploy canceled mid-rollout before the failing one
	}{
		{name: "web", serviceType: appv1alpha1.TypeWebService},
		{name: "pserv", serviceType: appv1alpha1.TypePrivateService},
		{name: "worker", serviceType: appv1alpha1.TypeBackgroundWorker},
		{name: "cron", serviceType: appv1alpha1.TypeCronJob},
		{name: "web-predeploy", serviceType: appv1alpha1.TypeWebService, preDeploy: true},
		{name: "web-canceled", serviceType: appv1alpha1.TypeWebService, canceledFirst: true},
	} {
		It("keeps the serving pods until the next deploy: "+tc.name, func() {
			name := "save-after-failed-build-" + tc.name
			nn := types.NamespacedName{Name: name, Namespace: "default"}
			cron := tc.serviceType == appv1alpha1.TypeCronJob
			r := &AppReconciler{
				Client: k8sClient, Scheme: k8sClient.Scheme(),
				Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default",
			}
			pass := func(n int) {
				GinkgoHelper()
				for range n {
					_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
					Expect(err).NotTo(HaveOccurred())
				}
			}
			var generations []int64
			buildExists := func(gen int64) bool {
				_, err := buildJobFor(name, appv1alpha1.BuildRevision(gen))
				return err == nil
			}
			preDeployJob := func(gen int64) (*batchv1.Job, error) {
				job := &batchv1.Job{}
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: predeploy.JobName(name, appv1alpha1.BuildRevision(gen))}, job)
				return job, err
			}
			// workload is the serving pod template: the Deployment's, or the
			// CronJob's job template for a cron job.
			workload := func() *corev1.PodTemplateSpec {
				GinkgoHelper()
				if cron {
					var cj batchv1.CronJob
					Expect(k8sClient.Get(ctx, nn, &cj)).To(Succeed())
					return cj.Spec.JobTemplate.Spec.Template.DeepCopy()
				}
				return envtestDeployment(nn).Spec.Template.DeepCopy()
			}
			unchanged := func(want *corev1.PodTemplateSpec, why string) {
				GinkgoHelper()
				got := workload()
				Expect(equality.Semantic.DeepEqual(*want, *got)).To(BeTrue(),
					"%s\nwant labels %v, containers %v\ngot  labels %v, containers %v", why, want.Labels, want.Spec.Containers, got.Labels, got.Spec.Containers)
			}
			active := func() string { GinkgoHelper(); return envtestApp(nn).Status.ActiveRevision }
			update := func(mutate func(*appv1alpha1.App)) int64 {
				GinkgoHelper()
				app := envtestApp(nn)
				mutate(app)
				Expect(k8sClient.Update(ctx, app)).To(Succeed())
				gen := envtestApp(nn).Generation
				generations = append(generations, gen)
				return gen
			}
			// deploy and saveOnly stamp what bex-api's deploy verbs and its
			// Update Service PATCH stamp for the generation the write produces.
			deploy := func(mutate func(*appv1alpha1.AppSpec)) int64 {
				GinkgoHelper()
				return update(func(app *appv1alpha1.App) {
					mutate(&app.Spec)
					app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
					metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(app.Generation+1))
				})
			}
			saveOnly := func(mutate func(*appv1alpha1.AppSpec)) int64 {
				GinkgoHelper()
				return update(func(app *appv1alpha1.App) {
					mutate(&app.Spec)
					metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
				})
			}
			// built completes gen's build and, with a pre-deploy command, its
			// pre-deploy Job: the release then rolls out.
			built := func(gen int64) {
				GinkgoHelper()
				completeBuildJob(name, appv1alpha1.BuildRevision(gen))
				pass(3)
				if tc.preDeploy {
					job, err := preDeployJob(gen)
					Expect(err).NotTo(HaveOccurred(), "the deploy runs its pre-deploy command")
					completeEnvtestJob(job, metav1.Now())
					pass(3)
				}
			}
			goLive := func(gen int64) {
				GinkgoHelper()
				Expect(buildExists(gen)).To(BeTrue(), "the deploy dispatches its build")
				built(gen)
				if !cron {
					markEnvtestDeploymentReady(nn)
				}
				pass(2)
				Expect(active()).To(Equal(fmt.Sprintf("rev-%d", gen)))
			}
			DeferCleanup(func() {
				if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
					r.Registry = "" // no registry server in this suite
					deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
				}
				background := metav1.DeletePropagationBackground
				for _, gen := range generations {
					rev := appv1alpha1.BuildRevision(gen)
					_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, rev), Namespace: "default"}})
					_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: predeploy.JobName(name, rev), Namespace: "default"}},
						&client.DeleteOptions{PropagationPolicy: &background})
				}
			})

			By("building and serving generation 1")
			spec := appv1alpha1.AppSpec{
				Type: tc.serviceType, Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Replicas: 1,
			}
			switch {
			case cron:
				spec.Schedule, spec.Command = "* * * * *", "echo run"
			case tc.serviceType != appv1alpha1.TypeBackgroundWorker:
				spec.Port = 3000
			}
			if tc.preDeploy {
				spec.PreDeployCommand = "echo migrate"
			}
			Expect(k8sClient.Create(ctx, &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
					Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
				Spec: spec,
			})).To(Succeed())
			generations = append(generations, 1)
			pass(3)
			goLive(1)
			served := workload()

			if tc.canceledFirst {
				By("canceling a deploy whose new instance never became ready")
				canceled := deploy(func(s *appv1alpha1.AppSpec) { s.StartCommand = "node canceled.js" })
				pass(2)
				Expect(buildExists(canceled)).To(BeTrue())
				built(canceled)
				Expect(equality.Semantic.DeepEqual(*served, *workload())).To(BeFalse(), "precondition: the canceled deploy was rolling out")
				update(func(app *appv1alpha1.App) {
					metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationCanceledReleaseGeneration, fmt.Sprint(canceled))
				})
				pass(3)
				Expect(active()).To(Equal("rev-1"))
				unchanged(served, "the cancel puts the served release back")

				By("saving right after the cancel: the canceled deploy's settings never start")
				saveOnly(func(s *appv1alpha1.AppSpec) { s.HealthCheckPath = "/saved" })
				pass(3)
				unchanged(served, "a save after a cancel must not roll the canceled release")
			}

			By("deploying a build command that fails")
			failing := deploy(func(s *appv1alpha1.AppSpec) { s.BuildCommand = "yarn install && exit 3" })
			pass(2)
			Expect(buildExists(failing)).To(BeTrue())
			failBuildJob(name, appv1alpha1.BuildRevision(failing))
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).To(HaveOccurred(), "the pass that sees the failure reports it")
			pass(2)
			Expect(terminalBuildFailureRecorded(envtestApp(nn))).To(BeTrue(), "status: %+v", envtestApp(nn).Status)
			Expect(active()).To(Equal("rev-1"))
			unchanged(served, "a failed build leaves the serving pods alone")

			By("saving a fixed build command without deploying it")
			saved := saveOnly(func(s *appv1alpha1.AppSpec) { s.BuildCommand = "yarn install" })
			pass(3)
			Expect(active()).To(Equal("rev-1"))
			unchanged(served, "a save must not replace the serving pods under the failed generation's revision")
			Expect(buildExists(saved)).To(BeFalse(), "a save builds nothing")
			if tc.preDeploy {
				_, err := preDeployJob(failing)
				Expect(apierrors.IsNotFound(err)).To(BeTrue(), "a save must not run the failed release's pre-deploy command")
			}

			By("deploying: the saved build command builds and goes live")
			next := deploy(func(*appv1alpha1.AppSpec) {})
			pass(3)
			goLive(next)
		})
	}
})
