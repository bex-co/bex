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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/prometheus/client_golang/prometheus/testutil"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// This suite pins w2/m82 t002: once a generation's terminal build failure is
// recorded on the Ready condition, re-reconciling that same generation must
// return nil — NOT r.fail's error. Before the fix, every pass returned the
// build-failure error, so controller-runtime logged `Reconciler error`
// (~20×/h per failed App until the Job's TTL reaped it) and re-metered the
// same failure into the queue-wait histogram the p95 capacity alert pages on.
// ERROR logs must mean real failures, not re-reported terminal outcomes.
var _ = Describe("Terminal build failure quiescence (w2/m82 t002)", func() {
	const name = "terminal-failed-app"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}

	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default",
		}
	})

	getApp := func() *appv1alpha1.App {
		a := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, a)).To(Succeed())
		return a
	}
	buildJob := func(rev string) (*batchv1.Job, error) {
		j := &batchv1.Job{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: build.JobName(name, rev)}, j)
		return j, err
	}
	// failBuild marks the dispatched build Job Failed with the tenant-classified
	// reason (exit 90 → PodFailurePolicy), the envtest stand-in for a real broken
	// tenant build. That classification is what faultFromJob keys on. The API
	// server's Job status grammar requires FailureTarget before Failed and no
	// completionTime on a failed Job.
	failBuild := func(rev string) {
		j, err := buildJob(rev)
		Expect(err).NotTo(HaveOccurred(), "build Job for %s must have been dispatched", rev)
		now := metav1.Now()
		j.Status.StartTime = &now
		j.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue,
				Reason: batchv1.JobReasonPodFailurePolicy, Message: "container exit code 90"},
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue,
				Reason: batchv1.JobReasonPodFailurePolicy, Message: "container exit code 90"},
		}
		Expect(k8sClient.Status().Update(ctx, j)).To(Succeed())
	}
	readyCond := func(a *appv1alpha1.App) *metav1.Condition {
		return meta.FindStatusCondition(a.Status.Conditions, appv1alpha1.ConditionReady)
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
		for _, rev := range []string{"gen-1", "gen-2"} {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
				Name: build.JobName(name, rev), Namespace: "default"}})
		}
	})

	It("records the failure once, then quiesces instead of re-failing forever", func() {
		By("creating a repo-backed App — dispatches the gen-1 build")
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		for range 3 { // first pass stamps the finalizer; a later pass dispatches the build
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
		_, err := buildJob("gen-1")
		Expect(err).NotTo(HaveOccurred(), "the gen-1 build Job must be dispatched")

		By("failing the build as tenant-classified (exit 90 → PodFailurePolicy)")
		outcomesBefore := testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeUserFailed))
		failBuild("gen-1")

		By("the FIRST failed observation runs r.fail exactly once: phase Failed, Ready reason BuildFailedUserError, error returned")
		_, err = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).To(HaveOccurred(), "the first pass must still surface the failure to controller-runtime")
		failed := getApp()
		Expect(failed.Status.Phase).To(Equal(appv1alpha1.PhaseFailed))
		cond := readyCond(failed)
		Expect(cond).NotTo(BeNil())
		Expect(cond.Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))
		Expect(cond.ObservedGeneration).To(Equal(failed.Generation),
			"the Ready condition is the durable this-generation marker")
		Expect(failed.Status.ObservedGeneration).To(BeZero(),
			"a failed build must never advance ObservedGeneration (successfulReleaseGeneration derives the last good release from it)")

		By("every subsequent reconcile of the same generation returns nil — no Reconciler error, no re-fail")
		for range 5 {
			res, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred(),
				"an already-recorded terminal failure must not re-error (this was the ERROR spam)")
			Expect(res.RequeueAfter).To(BeZero(), "the quiesced pass must not schedule its own requeue")
		}
		still := getApp()
		Expect(still.Status.Phase).To(Equal(appv1alpha1.PhaseFailed), "the App stays Failed")
		Expect(readyCond(still).Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))

		By("w2/m82 t006: TTL-reaping the failed Job (delete stands in for the reaper) must NOT re-dispatch")
		j1, err := buildJob("gen-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(k8sClient.Delete(ctx, j1)).To(Succeed())
		// The API server stamps an `orphan` finalizer on the deleting Job, and
		// envtest runs no garbage collector to process it — clear it by hand so
		// the Job is truly gone (the TTL reaper's end state).
		Eventually(func() []string {
			j, err := buildJob("gen-1")
			if err != nil {
				return nil
			}
			return j.Finalizers
		}, "10s", "250ms").Should(ContainElement("orphan"))
		fresh, err := buildJob("gen-1")
		Expect(err).NotTo(HaveOccurred())
		fresh.Finalizers = nil
		Expect(k8sClient.Update(ctx, fresh)).To(Succeed())
		Eventually(func() error {
			_, err := buildJob("gen-1")
			return err
		}, "10s", "250ms").ShouldNot(Succeed(), "the failed Job must actually be gone before we test the gate")
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
		_, err = buildJob("gen-1")
		Expect(err).To(HaveOccurred(), "a terminally failed generation must never re-create its build Job (the doomed-rebuild loop)")
		afterReap := getApp()
		Expect(afterReap.Status.Phase).To(Equal(appv1alpha1.PhaseFailed),
			"the re-dispatch path's setPhase(Building) must not overwrite the durable Failed marker")
		Expect(readyCond(afterReap).Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))

		By("no double metering: one terminal failure, one outcome increment")
		Expect(testutil.ToFloat64(buildOutcomesTotal.WithLabelValues(buildOutcomeUserFailed))).To(
			Equal(outcomesBefore+1),
			"recordBuildOutcome must fire exactly once per terminal failure across re-reconciles and Job reap")

		By("a new generation (tenant bumps restartedAt) dispatches a fresh build — the gate is per-generation")
		a := getApp()
		a.Spec.RestartedAt = "2026-08-20T00:01:00Z"
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
		for range 3 {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			Expect(err).NotTo(HaveOccurred())
		}
		_, err = buildJob("gen-2")
		Expect(err).NotTo(HaveOccurred(), "the new generation must dispatch its own build Job")
	})
})
