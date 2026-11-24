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
	"strings"
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

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Render's Update Service saves and deploys nothing (w8/m135): bex-api holds a
// Settings PATCH for the next deploy with the pending marker. The release
// serving keeps running the settings it was adopted with — probe, shutdown
// delay, command — and runs no pre-deploy; the next deploy applies every held
// field together. Before, the operator projected the saved spec straight into
// the template, so a held field rolled the pods with no deploy.
var _ = Describe("Saved settings wait for the next deploy (w8/m135)", func() {
	// Each spec runs its own App: a deleted one lingers in envtest, which runs
	// no garbage collector.
	var name string
	var nn types.NamespacedName
	use := func(n string) { name, nn = n, types.NamespacedName{Name: n, Namespace: "default"} }
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	pass := func(n int) {
		GinkgoHelper()
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	getApp := func() *appv1alpha1.App { GinkgoHelper(); return envtestApp(nn) }
	template := func() *corev1.PodTemplateSpec {
		GinkgoHelper()
		return envtestDeployment(nn).Spec.Template.DeepCopy()
	}
	appContainer := func(t *corev1.PodTemplateSpec) corev1.Container {
		GinkgoHelper()
		c := findContainer(t.Spec.Containers, appContainerName)
		Expect(c).NotTo(BeNil())
		return *c
	}
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		app := getApp()
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	// saveOnly is a Render Update Service PATCH as bex-api writes it: the
	// fields and the pending marker for the generation the write produces.
	saveOnly := func(mutate func(*appv1alpha1.AppSpec)) {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			mutate(&app.Spec)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationPendingSourceGeneration, fmt.Sprint(app.Generation+1))
		})
	}
	// deploy is a deploy verb: it stamps the generation it opens. selecting,
	// when set, selects a recorded release as a Restart or rollback does.
	deploy := func(selecting func(app *appv1alpha1.App, gen int64)) int64 {
		GinkgoHelper()
		update(func(app *appv1alpha1.App) {
			app.Spec.RestartedAt = time.Now().UTC().Format(time.RFC3339Nano)
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(app.Generation+1))
			if selecting != nil {
				selecting(app, app.Generation+1)
			}
		})
		return getApp().Generation
	}
	undeployed := func() bool {
		GinkgoHelper()
		_, err := r.reconcileSavedConfigurationStatus(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
		return getApp().Status.UndeployedChanges
	}
	preDeployJob := func(gen int64) (*batchv1.Job, error) {
		job := &batchv1.Job{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: predeploy.JobName(name, fmt.Sprintf("gen-%d", gen))}, job)
		return job, err
	}

	AfterEach(func() {
		if app := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, app) == nil {
			deleteOwner(ctx, app, func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) })
		}
		background := metav1.DeletePropagationBackground
		for gen := range 8 {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: predeploy.JobName(name, fmt.Sprintf("gen-%d", gen)), Namespace: "default"}},
				&client.DeleteOptions{PropagationPolicy: &background})
		}
	})

	It("holds every saved field until a deploy, which applies them together", func() {
		use("save-only-settings")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1, HealthCheckPath: "/hz"},
		})).To(Succeed())
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 serves")
		Expect(undeployed()).To(BeFalse())
		served := template()

		By("save-only PATCHes of the probe, the shutdown delay and the command")
		// The shutdown delay is outside the record's selected fields: only the
		// served release's settings fingerprint can tell it is not deployed.
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.MaxShutdownDelaySeconds = new(int32(40)) })
		pass(2)
		Expect(undeployed()).To(BeTrue(), "a saved shutdown delay is reported as not yet deployed")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.HealthCheckPath = "/hz2" })
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.StartCommand = "node index.js" })
		pass(3)
		app := getApp()
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)), "a save opens no release")
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		Expect(equality.Semantic.DeepEqual(*served, *template())).To(BeTrue(),
			"the serving pods must keep the settings they were deployed with")

		By("a save-only PATCH of the pre-deploy command runs nothing")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.PreDeployCommand = "echo x" })
		pass(3)
		app = getApp()
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))
		_, err := preDeployJob(app.Generation)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "a save runs no pre-deploy command")
		Expect(equality.Semantic.DeepEqual(*served, *template())).To(BeTrue())
		Expect(undeployed()).To(BeTrue(), "the saved settings are reported as not yet deployed")

		By("a deploy that is canceled leaves the served release and the saved settings pending")
		canceled := deploy(nil)
		pass(2)
		_, err = preDeployJob(canceled)
		Expect(err).NotTo(HaveOccurred(), "the deploy runs the saved pre-deploy command")
		update(func(app *appv1alpha1.App) {
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationCanceledReleaseGeneration, fmt.Sprint(canceled))
		})
		pass(3)
		Expect(equality.Semantic.DeepEqual(*served, *template())).To(BeTrue(), "the cancel keeps the served release")
		Expect(undeployed()).To(BeTrue(), "a canceled deploy leaves the saved settings pending")

		By("the next deploy applies every saved field together")
		next := deploy(nil)
		pass(2)
		job, err := preDeployJob(next)
		Expect(err).NotTo(HaveOccurred())
		completeEnvtestJob(job, metav1.Now())
		pass(3)
		applied := template()
		c := appContainer(applied)
		Expect(probedHealthCheckPath(c)).To(HaveValue(Equal("/hz2")))
		Expect(fmt.Sprint(c.Command, c.Args)).To(ContainSubstring("node index.js"))
		Expect(*applied.Spec.TerminationGracePeriodSeconds).To(Equal(*served.Spec.TerminationGracePeriodSeconds+10),
			"the 40-second shutdown delay replaces the 30-second default")
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", next)))
		Expect(undeployed()).To(BeFalse(), "the deploy cleared the pending settings")

		By("a later save of the same values is no change")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.HealthCheckPath = "/hz2" })
		pass(2)
		Expect(undeployed()).To(BeFalse())
	})

	// w6/163: a dashboard Settings save deploys at once, and an Update Service
	// save seconds later waits in bex-api until the operator has adopted that
	// deploy (rollout.AwaitAdoptionBeforeHold). Written in that order, the
	// deploy reaches live with the shutdown delay it was opened for, and the
	// later save waits for the next deploy. Written before the adoption, the
	// save held the deploy, which stayed created until another deploy
	// canceled it.
	It("runs a deploy adopted before a later save with its own settings, and holds the save (w6/163)", func() {
		use("save-behind-adopted-deploy")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1},
		})).To(Succeed())
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 serves")
		served := *template().Spec.TerminationGracePeriodSeconds
		delay := func() int64 { GinkgoHelper(); return *template().Spec.TerminationGracePeriodSeconds - served + 30 }

		By("an Update Service save, then a Settings save that deploys 300 seconds")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.MaxShutdownDelaySeconds = new(int32(1)) })
		update(func(app *appv1alpha1.App) {
			app.Spec.MaxShutdownDelaySeconds = new(int32(300))
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, fmt.Sprint(app.Generation+1))
		})
		settings := getApp().Generation
		pass(1)
		Expect(getApp().Status.ReleaseGeneration).To(Equal(settings), "the operator adopted the Settings deploy")

		By("an Update Service save of 120 seconds once the deploy is adopted")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.MaxShutdownDelaySeconds = new(int32(120)) })
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", settings)), "the Settings deploy reached live")
		Expect(delay()).To(Equal(int64(300)), "the deploy runs the delay it was opened for")
		Expect(undeployed()).To(BeTrue(), "the 120-second save waits for the next deploy")

		By("the next deploy applies the saved delay")
		next := deploy(nil)
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", next)))
		Expect(delay()).To(Equal(int64(120)))
		Expect(undeployed()).To(BeFalse())
	})

	// runsPlan reads the plan the serving template runs from its app
	// container's CPU and memory, its serving pool and WEB_CONCURRENCY.
	runsPlan := func() (tier, pool, concurrency string) {
		GinkgoHelper()
		t := template()
		tier, ok := templateTier(*t)
		Expect(ok).To(BeTrue(), "the template runs a catalog plan")
		if a := t.Spec.Affinity; a != nil && a.NodeAffinity != nil && a.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution != nil {
			pool = a.NodeAffinity.RequiredDuringSchedulingIgnoredDuringExecution.NodeSelectorTerms[0].MatchExpressions[0].Values[0]
		}
		for _, env := range appContainer(t).Env {
			if env.Name == webConcurrencyEnvName {
				concurrency = env.Value
			}
		}
		return tier, pool, concurrency
	}

	// Render's Update Service saves a plan without deploying it, and "your
	// service's compute plan does not change until the first successful deploy
	// after you update it" (render.com/docs/compute-plans). bex-api holds a
	// REST or MCP plan change like any other saved field (w6/m609): the
	// serving pods keep their plan's CPU, memory, node pool and worker count
	// until a deploy, and status.servedTier, which billing reads, keeps naming
	// it.
	It("holds a saved plan until a deploy applies it, through a cancel (w6/m609)", func() {
		use("save-only-plan")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1, Tier: "starter", PreDeployCommand: "echo migrate"},
		})).To(Succeed())
		pass(2)
		job, err := preDeployJob(1)
		Expect(err).NotTo(HaveOccurred())
		completeEnvtestJob(job, metav1.Now())
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 serves")
		tier, pool, concurrency := runsPlan()
		Expect([]string{tier, pool, concurrency}).To(Equal([]string{"starter", "", "1"}))
		Expect(getApp().Status.ServedTier).To(Equal("starter"))
		served := template()

		By("an Update Service save of Pro Plus rolls nothing and runs no pre-deploy")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.Tier = "pro-plus" })
		pass(3)
		app := getApp()
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)), "a save opens no release")
		_, err = preDeployJob(app.Generation)
		Expect(apierrors.IsNotFound(err)).To(BeTrue(), "a save runs no pre-deploy command")
		tier, pool, concurrency = runsPlan()
		Expect(tier).To(Equal("starter"), "the pods keep Starter's CPU and memory")
		Expect(pool).To(BeEmpty(), "and stay off Pro Plus's xl pool")
		Expect(concurrency).To(Equal("1"), "and keep Starter's WEB_CONCURRENCY")
		Expect(equality.Semantic.DeepEqual(*served, *template())).To(BeTrue())
		Expect(app.Status.ServedTier).To(Equal("starter"), "billing keeps reading Starter")
		Expect(undeployed()).To(BeTrue(), "the saved plan is reported as not yet deployed")

		By("a deploy that is canceled keeps Starter and the saved plan pending")
		canceled := deploy(nil)
		pass(2)
		_, err = preDeployJob(canceled)
		Expect(err).NotTo(HaveOccurred(), "the deploy runs its pre-deploy command first")
		update(func(app *appv1alpha1.App) {
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationCanceledReleaseGeneration, fmt.Sprint(canceled))
		})
		pass(3)
		Expect(equality.Semantic.DeepEqual(*served, *template())).To(BeTrue(), "the cancel keeps the served release")
		Expect(getApp().Status.ServedTier).To(Equal("starter"))
		Expect(undeployed()).To(BeTrue())

		By("the next deploy applies Pro Plus in one release")
		next := deploy(nil)
		pass(2)
		job, err = preDeployJob(next)
		Expect(err).NotTo(HaveOccurred())
		completeEnvtestJob(job, metav1.Now())
		pass(3)
		tier, pool, concurrency = runsPlan()
		Expect(tier).To(Equal("pro-plus"))
		Expect(pool).To(Equal("xl"))
		Expect(concurrency).To(Equal("4"))
		Expect(getApp().Status.ServedTier).To(Equal("pro-plus"), "billing moves to Pro Plus with the rollout")
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", next)))
		Expect(undeployed()).To(BeFalse())

		By("a Restart applies a plan saved since, as the special form of manual deploy Render's is")
		// render.com/docs/deploys: "a service restart is actually a special
		// form of manual deploy", and the plan changes with "the first
		// successful deploy after you update it". The Restart runs the live
		// release's commit and recorded settings with the saved plan, as it
		// did before w6/m609 (ADR004 § Settings saves wait for a deploy).
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.Tier = "starter" })
		pass(3)
		tier, _, _ = runsPlan()
		Expect(tier).To(Equal("pro-plus"), "the save alone rolls nothing")
		Expect(undeployed()).To(BeTrue())
		restart := deploy(func(app *appv1alpha1.App, gen int64) {
			app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: gen, SourceGeneration: next,
				Image: app.Status.Image, PreserveGroupValues: true}
		})
		pass(2)
		if job, err := preDeployJob(restart); err == nil {
			completeEnvtestJob(job, metav1.Now())
		}
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", restart)))
		tier, _, _ = runsPlan()
		Expect(tier).To(Equal("starter"))
		Expect(getApp().Status.ServedTier).To(Equal("starter"))
		Expect(undeployed()).To(BeFalse())
	})

	// A plan saved on a suspended service waits through Resume, which deploys
	// nothing on Render either: the resumed pods keep the plan they were
	// suspended on (ADR004 § Resume, then deploy). A rollback is a deploy, so
	// it applies the saved plan with its target's image, as a Restart does
	// (w6/m609 t002, ADR004 § Settings saves wait for a deploy).
	It("keeps a saved plan through Suspend and Resume, and a rollback applies it (w6/m609)", func() {
		use("save-only-plan-resume")
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1, Tier: "starter"},
		})).To(Succeed())
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 serves")
		first := strings.TrimPrefix(getApp().Status.Image, "/") // no registry in this suite

		By("a deploy of a newer image serves on Starter")
		second := deploy(func(app *appv1alpha1.App, _ int64) { app.Spec.Image = "busybox:1.37" })
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", second)))
		Expect(appContainer(template()).Image).To(HaveSuffix("busybox:1.37"))

		By("Standard saved while suspended stays saved through Resume")
		update(func(app *appv1alpha1.App) { app.Spec.Suspended = true })
		pass(3)
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.Tier = "standard" })
		pass(3)
		update(func(app *appv1alpha1.App) { app.Spec.Suspended = false })
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		app := getApp()
		Expect(app.Status.ReleaseGeneration).To(Equal(second), "a resume opens no release")
		Expect(app.Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", second)))
		tier, _, _ := runsPlan()
		Expect(tier).To(Equal("starter"), "the resumed pods keep Starter's CPU and memory")
		Expect(app.Status.ServedTier).To(Equal("starter"), "billing keeps reading Starter")
		Expect(undeployed()).To(BeTrue(), "the saved plan is still not deployed")

		By("a rollback to generation 1 runs its image on the saved Standard plan")
		rollback := deploy(func(app *appv1alpha1.App, gen int64) {
			app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: gen, SourceGeneration: 1,
				Image: first, PreserveGroupValues: true}
		})
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", rollback)))
		Expect(appContainer(template()).Image).To(HaveSuffix("busybox:1.36"), "the rollback runs its target's image")
		tier, _, _ = runsPlan()
		Expect(tier).To(Equal("standard"))
		Expect(getApp().Status.ServedTier).To(Equal("standard"), "billing moves to Standard with the rollback")
	})

	It("keeps a hibernating Free service asleep on Free while a saved Starter plan waits (w6/m609)", func() {
		use("save-only-plan-free")
		// The activator is the wake path a Free service sleeps behind.
		r.ActivatorService, r.ActivatorPort = "bex-activator", 8888
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{Image: "busybox:1.36", Port: 3000, Replicas: 1, Tier: "free"},
		})).To(Succeed())
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 serves")
		replicas := func() int32 { GinkgoHelper(); return *envtestDeployment(nn).Spec.Replicas }

		By("idle past Free's window, it hibernates")
		update(func(app *appv1alpha1.App) {
			metav1.SetMetaDataAnnotation(&app.ObjectMeta, annotLastActive, time.Now().Add(-time.Hour).UTC().Format(time.RFC3339))
		})
		pass(3)
		Expect(replicas()).To(BeZero())
		Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))

		By("a saved Starter plan leaves it asleep on Free")
		saveOnly(func(spec *appv1alpha1.AppSpec) { spec.Tier = "starter" })
		pass(3)
		Expect(replicas()).To(BeZero(), "a saved paid plan does not stop Free's spin-down")
		Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseHibernated))
		tier, _, _ := runsPlan()
		Expect(tier).To(Equal("free"))
		Expect(getApp().Status.ServedTier).To(Equal("free"))
		Expect(undeployed()).To(BeTrue())

		By("the deploy applies Starter, which never spins down")
		next := deploy(nil)
		pass(3)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal(fmt.Sprintf("rev-%d", next)))
		Expect(replicas()).To(Equal(int32(1)))
		tier, _, _ = runsPlan()
		Expect(tier).To(Equal("starter"))
		Expect(getApp().Status.ServedTier).To(Equal("starter"))
		Expect(undeployed()).To(BeFalse())
	})
})
