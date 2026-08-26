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
	"github.com/prometheus/client_golang/prometheus/testutil"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// This suite pins w6/m100: the deploy queued behind a build that FAILS must
// still run. ADR060 §D1a already handles the sibling case — a build that
// SUCCEEDS hands off to the pending slot (build_supersede_envtest_test.go) —
// but the failure branch attributed its terminal marker to metadata.generation,
// which the queued deploy's own spec patch had already bumped. The gate at
// buildFromSource's entry then read gen-1's failure as gen-2's verdict and
// halted with no error and no requeue: gen-2's build was never dispatched,
// status.releaseGeneration never left gen-1, and (no further generation bump
// arriving) no reconcile was ever scheduled for the App again. Live on
// production that showed up as a deploy frozen at status "queued" with empty
// startedAt/finishedAt/failureReason, recoverable only by a human clicking
// Cancel.
//
// Run against the pre-fix operator, the "advances to the pending generation"
// assertions below fail: releaseGeneration stays 1 and no gen-2 Job exists.
var _ = Describe("A deploy queued behind a failed build (w6/m100)", func() {
	const name = "queued-behind-failure-app"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default",
		}
	})

	reconcile1 := func() {
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
	reconcileN := func(n int) {
		for range n {
			reconcile1()
		}
	}
	getApp := func() *appv1alpha1.App {
		a := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, a)).To(Succeed())
		return a
	}
	buildJob := func(rev string) (*batchv1.Job, error) { return buildJobFor(name, rev) }
	jobExists := func(rev string) bool {
		_, err := buildJob(rev)
		return err == nil
	}
	failBuild := func(rev string) { failBuildJob(name, rev) }
	completeBuild := func(rev string) { completeBuildJob(name, rev) }
	cond := func(a *appv1alpha1.App, t string) *metav1.Condition {
		return meta.FindStatusCondition(a.Status.Conditions, t)
	}
	// trigger stands in for every backend verb that opens a new release while a
	// previous one may still be building — Manual Deploy, Rollback, webhook
	// auto-deploy, Blueprint sync, Restart, an env-var change. They all reduce to
	// the same patchApp: a fresh spec.restartedAt bumping metadata.generation and
	// the release identity (see the milestone's Blast radius).
	trigger := func(at string) {
		a := getApp()
		a.Spec.RestartedAt = at
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
	}

	AfterEach(func() {
		if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			saved := r.Registry
			r.Registry = "" // no registry server in this suite; deletion has focused tests
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
			r.Registry = saved
		}
		for _, rev := range []string{"gen-1", "gen-2", "gen-3"} {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
				Name: build.JobName(name, rev), Namespace: "default"}})
		}
	})

	It("advances to the pending generation once the blocking build's failure is recorded", func() {
		By("creating a repo-backed App — dispatches the gen-1 build")
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		reconcileN(3) // first pass stamps the finalizer; a later pass dispatches the build
		Expect(jobExists("gen-1")).To(BeTrue(), "the gen-1 build Job must be dispatched")
		Expect(getApp().Status.Phase).To(Equal(appv1alpha1.PhaseBuilding))

		By("triggering a second deploy while gen-1 is still building — it queues in the pending slot")
		trigger("2026-08-26T05:23:47Z")
		reconcile1()
		queued := getApp()
		Expect(queued.Generation).To(Equal(int64(2)), "the second trigger's spec patch bumped metadata.generation")
		Expect(queued.Status.ReleaseGeneration).To(Equal(int64(1)), "the running build stays pinned (ADR060 D1a)")
		Expect(queued.Status.PendingReleaseGeneration).To(Equal(int64(2)))
		Expect(jobExists("gen-2")).To(BeFalse(), "the queued deploy must not start a second build yet")

		By("failing the gen-1 build — the terminal marker belongs to gen-1, not to the queued gen-2")
		outcomesBefore := testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeUserFailed))
		failBuild("gen-1")
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).To(HaveOccurred(), "the first failed observation still surfaces the failure to controller-runtime")
		failed := getApp()
		Expect(failed.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		durable := cond(failed, appv1alpha1.ConditionBuild)
		Expect(durable).NotTo(BeNil(), "a build failure records the durable per-release-generation verdict")
		Expect(durable.Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))
		Expect(durable.ObservedGeneration).To(Equal(int64(1)),
			"the verdict is attributed to the release generation that actually built, not to metadata.generation")
		Expect(durable.Message).NotTo(BeEmpty(), "bex-api closes the failed deploy row with this message")
		Expect(cond(failed, appv1alpha1.ConditionReady).Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))

		By("the next reconcile adopts the pending generation and builds it")
		reconcile1()
		advanced := getApp()
		Expect(advanced.Status.ReleaseGeneration).To(Equal(int64(2)),
			"the queued deploy's generation must become the active release once the blocking build is settled")
		Expect(advanced.Status.PendingReleaseGeneration).To(BeZero())
		Expect(advanced.Status.Phase).To(Equal(appv1alpha1.PhaseBuilding))
		Expect(jobExists("gen-2")).To(BeTrue(), "the queued deploy's build must actually be dispatched")

		By("gen-1's verdict survives gen-2 starting — bex-api still has it a resync later")
		Expect(cond(advanced, appv1alpha1.ConditionBuild).ObservedGeneration).To(Equal(int64(1)))
		Expect(cond(advanced, appv1alpha1.ConditionReady).Reason).NotTo(
			Equal(appv1alpha1.ReasonBuildFailedUserError),
			"Ready has already moved on to gen-2's progress, which is exactly why it cannot be the durable record")

		By("no double metering: gen-1's failure counted once across the advance")
		Expect(testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeUserFailed))).To(
			Equal(outcomesBefore + 1))

		By("gen-2 completes and rolls out — the once-stranded deploy reaches a real terminal state")
		completeBuild("gen-2")
		reconcileN(2)
		Expect(getApp().Status.Image).To(ContainSubstring(":gen-2"))
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(":gen-2"))
	})

	It("keeps a solo failed generation settled: no re-dispatch, no re-fail, no re-meter", func() {
		By("creating a repo-backed App and failing its only build")
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		reconcileN(3)
		outcomesBefore := testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeUserFailed))
		failBuild("gen-1")
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).To(HaveOccurred())

		By("w2/m82: with nothing queued behind it the release must NOT advance and the App must quiesce")
		for range 5 {
			res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred(), "an already-recorded terminal failure must not re-error")
			Expect(res.RequeueAfter).To(BeZero(), "the quiesced pass must not schedule its own requeue")
		}
		settled := getApp()
		Expect(settled.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		Expect(settled.Status.ReleaseGeneration).To(Equal(int64(1)),
			"a failed build with no pending generation has nothing to advance to")
		Expect(jobExists("gen-2")).To(BeFalse())
		Expect(testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeUserFailed))).To(
			Equal(outcomesBefore + 1))
	})

	It("does not re-dispatch a failed generation when only operational churn bumps the generation", func() {
		By("creating a repo-backed App and failing its only build")
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000, Replicas: 1,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		reconcileN(3)
		failBuild("gen-1")
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).To(HaveOccurred())

		By("scaling — metadata.generation moves but the release identity does not")
		a := getApp()
		a.Spec.Replicas = 2
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
		reconcileN(3)
		scaled := getApp()
		Expect(scaled.Generation).To(BeNumerically(">", int64(1)))
		Expect(scaled.Status.ReleaseGeneration).To(Equal(int64(1)))
		Expect(jobExists("gen-2")).To(BeFalse(),
			"operational churn is not a new release, so the doomed build must not be re-run under a new revision")
		Expect(scaled.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
	})
})
