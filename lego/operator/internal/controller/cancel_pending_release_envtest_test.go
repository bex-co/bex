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
	"fmt"
	"strconv"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w8/m117: a cancel ends one deploy. Canceling the newer of two open deploys —
// the release waiting in the latest-pending slot — must leave the older one,
// already in flight, to build and roll out (ADR060 §D1a). bex-api names that
// release when it stamps the cancel; settling the cancel at once instead
// rewound the App below both releases: the older deploy's build was abandoned
// and its row stranded until the gate timeout.
var _ = Describe("Canceling the pending release behind one in flight (w8/m117)", func() {
	const name = "cancel-pending"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default",
		}
	})
	pass := func(n int) {
		GinkgoHelper()
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	getApp := func() *appv1alpha1.App { GinkgoHelper(); return envtestApp(nn) }
	update := func(mutate func(*appv1alpha1.App)) {
		GinkgoHelper()
		a := getApp()
		if a.Annotations == nil {
			a.Annotations = map[string]string{}
		}
		mutate(a)
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
	}
	jobExists := func(rev string) bool { _, err := buildJobFor(name, rev); return err == nil }
	// trigger is a deploy as bex-api opens one: a fresh restartedAt is a new
	// release identity, stamped with the generation the update moves to.
	trigger := func(minute int) {
		GinkgoHelper()
		update(func(a *appv1alpha1.App) {
			a.Spec.RestartedAt = fmt.Sprintf("2026-10-18T08:%02d:00Z", minute)
			a.Annotations[appv1alpha1.AnnotationReleaseGeneration] = strconv.FormatInt(a.Generation+1, 10)
		})
	}
	// cancelLatest is bex-api's cancel stamp for the newest release, naming the
	// release it leaves running (0: none).
	cancelLatest := func(survivor int64) {
		GinkgoHelper()
		update(func(a *appv1alpha1.App) {
			a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = strconv.FormatInt(a.Generation, 10)
			delete(a.Annotations, appv1alpha1.AnnotationSurvivingReleaseGeneration)
			if survivor > 0 {
				a.Annotations[appv1alpha1.AnnotationSurvivingReleaseGeneration] = strconv.FormatInt(survivor, 10)
			}
		})
	}
	createApp := func() {
		GinkgoHelper()
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default",
				Annotations: map[string]string{appv1alpha1.AnnotationReleaseGeneration: "1"}},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main", Builder: "dockerfile",
				Port: 3000, Replicas: 1,
			},
		})).To(Succeed())
		pass(3)
	}
	// duplicateCanceled is the live repro: a new service's first deploy is
	// building when a second one is triggered and then canceled.
	duplicateCanceled := func(survivor int64) {
		GinkgoHelper()
		createApp()
		Expect(jobExists("gen-1")).To(BeTrue())
		trigger(54)
		pass(1)
		Expect(getApp().Status.PendingReleaseGeneration).To(Equal(int64(2)), "precondition: generation 2 is the pending slot")
		cancelLatest(survivor)
		pass(2)
	}

	AfterEach(func() {
		if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			saved := r.Registry
			r.Registry = ""
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
			r.Registry = saved
		}
		for _, rev := range []string{"gen-1", "gen-2", "gen-3", "gen-4"} {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, rev), Namespace: "default"}},
				client.PropagationPolicy(metav1.DeletePropagationBackground))
		}
	})

	It("a new service's first deploy builds and serves after its duplicate is canceled", func() {
		duplicateCanceled(1)

		app := getApp()
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseBuilding), "the first deploy is still building, not canceled")
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)), "the release in flight is not rewound")
		Expect(jobExists("gen-1")).To(BeTrue(), "the in-flight build keeps running")

		By("its build completes and rolls out; the canceled release never builds")
		completeBuildJob(name, "gen-1")
		pass(2)
		Expect(getApp().Status.PendingReleaseGeneration).To(BeZero(), "the canceled pending slot is dropped")
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(":gen-1"))
		markEnvtestDeploymentReady(nn)
		pass(3)

		app = getApp()
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"), "the first deploy serves")
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))
		Expect(jobExists("gen-2")).To(BeFalse(), "the canceled release never builds")
	})

	It("a release in flight over a served one keeps its own pod template", func() {
		createApp()
		completeBuildJob(name, "gen-1")
		pass(2)
		markEnvtestDeploymentReady(nn)
		pass(2)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-1"), "precondition: generation 1 served")

		By("generation 2 builds, generation 3 waits behind it and is canceled")
		trigger(1)
		pass(1)
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(2)))
		trigger(2)
		pass(1)
		cancelLatest(2)
		pass(2)
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(2)), "generation 2 stays in flight")

		By("generation 2 rolls out with its own image, not the served release's template")
		completeBuildJob(name, "gen-2")
		pass(2)
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(":gen-2"))
		markEnvtestDeploymentReady(nn)
		pass(3)
		Expect(getApp().Status.ActiveRevision).To(Equal("rev-2"))
		Expect(jobExists("gen-3")).To(BeFalse(), "the canceled release never builds")
	})

	It("an in-flight build that fails stays failed, not canceled", func() {
		duplicateCanceled(1)

		failBuildJob(name, "gen-1")
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		pass(2)
		app := getApp()
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseFailed), "the older deploy's own failure is the App's state")
		Expect(app.Status.ReleaseGeneration).To(Equal(int64(1)))
		Expect(jobExists("gen-2")).To(BeFalse(), "nothing is promoted: the pending release was canceled")
	})

	It("a cancel that leaves nothing running settles below both releases and stops both builds", func() {
		duplicateCanceled(0)

		app := getApp()
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseCanceled))
		Expect(app.Status.ReleaseGeneration).To(BeZero(), "no release has served")
		Expect(jobExists("gen-1")).To(BeFalse(), "the abandoned build would hold a build slot to the end")
		Expect(jobExists("gen-2")).To(BeFalse())
	})

	It("ending the release a cancel left running settles the cancel and stops its build", func() {
		duplicateCanceled(1)
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(1)), "precondition: generation 1 is carried")

		By("bex-api cancels the first deploy too, and its own stop of the build fails")
		update(func(a *appv1alpha1.App) { delete(a.Annotations, appv1alpha1.AnnotationSurvivingReleaseGeneration) })
		pass(2)

		app := getApp()
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseCanceled))
		Expect(app.Status.ReleaseGeneration).To(BeZero())
		Expect(jobExists("gen-1")).To(BeFalse(), "the operator stops the build itself")
		pass(2)
		Expect(jobExists("gen-1")).To(BeFalse(), "and never recreates it")
	})

	It("a suspend ends the carried release, and resuming does not bring it back", func() {
		duplicateCanceled(1)

		update(func(a *appv1alpha1.App) { a.Spec.Suspended = true })
		pass(2)
		Expect(getApp().Status.ReleaseGeneration).To(BeZero(), "the suspend settled the cancel")
		update(func(a *appv1alpha1.App) { a.Spec.Suspended = false })
		pass(2)

		app := getApp()
		Expect(app.Status.ReleaseGeneration).To(BeZero())
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseCanceled))
		Expect(app.Status.ActiveRevision).To(BeEmpty(), "neither release served")
	})
})
