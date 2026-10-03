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
	"testing"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m155: a replacement publish that fails over a static release that already
// served is a deploy fact, not an outage. The immutable prefix keeps serving
// the prior release, so the phase must say Running (or Suspended), while the
// failed release's exact diagnosis survives on ConditionRollout for bex-api to
// close the deploy row update_failed.
var _ = Describe("a failed static publish over a served release (w4/m155)", func() {
	const ns = "default"
	const name = "site-publish-over-served"
	nn := types.NamespacedName{Name: name, Namespace: ns}

	reconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			StaticStore: publish.Store{
				Bucket: "bex-static", Endpoint: "https://s3.example.com", Secret: "static-s3",
			},
			StaticServerService: "bex-static-server", StaticServerPort: 8080,
		}
	}
	reconcileN := func() {
		r := reconciler()
		for range 3 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
	}
	get := func() *appv1alpha1.App {
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn, app)).To(Succeed())
		return app
	}
	// settleJob finishes the current release's publish Job by hand (envtest has
	// no kubelet), with the clone container's own words on failure.
	settleJob := func(failMessage string) {
		jobNN := types.NamespacedName{Name: "pub-" + name + "-" + releaseRevision(get()), Namespace: ns}
		var job batchv1.Job
		Eventually(func() error { return k8sClient.Get(ctx, jobNN, &job) }, "30s", "250ms").Should(Succeed())
		now := metav1.Now()
		job.Status.StartTime = &now
		if failMessage == "" {
			job.Status.CompletionTime = &now
			job.Status.Conditions = append(job.Status.Conditions,
				batchv1.JobCondition{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
				batchv1.JobCondition{Type: batchv1.JobComplete, Status: corev1.ConditionTrue})
		} else {
			pod := &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: job.Name + "-abcde", Namespace: ns, Labels: map[string]string{"job-name": job.Name}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "upload", Image: "x"}}},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod) })
			pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
				Name:  "clone",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 2, Message: failMessage + "\n"}},
			}}
			Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
			job.Status.Conditions = append(job.Status.Conditions,
				batchv1.JobCondition{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded"},
				batchv1.JobCondition{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: "BackoffLimitExceeded"})
		}
		Expect(k8sClient.Status().Update(ctx, &job)).To(Succeed())
	}
	update := func(mutate func(*appv1alpha1.App)) {
		app := get()
		mutate(app)
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
	}
	expectServingPrior := func(phase appv1alpha1.AppPhase, revision, prefix string) {
		app := get()
		Expect(app.Status.Phase).To(Equal(phase))
		Expect(app.Status.ActiveRevision).To(Equal(revision), "the failed revision must never become active")
		Expect(app.Status.StaticPrefix).To(Equal(prefix), "the served immutable prefix must not move")
	}
	expectVerdict := func(generation int64, dir string) {
		app := get()
		cond := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout)
		Expect(cond).NotTo(BeNil(), "the failed publish's diagnosis must outlive the Ready condition")
		Expect(cond.Status).To(Equal(metav1.ConditionFalse))
		Expect(cond.Reason).To(Equal(reasonPublishFailed))
		Expect(cond.Message).To(ContainSubstring(dir))
		Expect(cond.ObservedGeneration).To(Equal(generation), "attributed to the release that failed to publish")
	}

	It("keeps the prior release Running, records the failure, and survives suspend, resume and recovery", func() {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeStaticSite, Repo: "https://github.com/bex-co/bex",
				RootDir: "examples/static-site", PublishPath: ".", Expose: true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, app) })
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3", Namespace: ns},
			Data:       map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("a"), "AWS_SECRET_ACCESS_KEY": []byte("s")},
		}
		Expect(k8sClient.Create(ctx, credential)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })

		By("publishing the first release")
		reconcileN()
		settleJob("")
		reconcileN()
		first := get()
		Expect(first.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		servedRev, servedPrefix := first.Status.ActiveRevision, first.Status.StaticPrefix
		Expect(servedRev).NotTo(BeEmpty())

		By("failing a replacement publish to a missing directory")
		update(func(a *appv1alpha1.App) { a.Spec.PublishPath = "qa-missing-output" })
		reconcileN()
		failedGen := releaseGeneration(get())
		Expect(releaseRevision(get())).NotTo(Equal(servedRev))
		settleJob(`the publish directory "examples/static-site/qa-missing-output" does not exist in the repository at "main"`)
		reconcileN()
		expectServingPrior(appv1alpha1.PhaseRunning, servedRev, servedPrefix)
		ready := meta.FindStatusCondition(get().Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(ready.Reason).To(Equal(reasonPriorReleaseServing))
		Expect(ready.Message).To(ContainSubstring("the latest publish failed"))
		expectVerdict(failedGen, "qa-missing-output")

		By("suspending while the latest publish is failed")
		update(func(a *appv1alpha1.App) { a.Spec.Suspended = true })
		reconcileN()
		expectServingPrior(appv1alpha1.PhaseHibernated, servedRev, servedPrefix)
		Expect(meta.FindStatusCondition(get().Status.Conditions, appv1alpha1.ConditionReady).Reason).To(Equal(reasonSuspended))
		expectVerdict(failedGen, "qa-missing-output")

		By("resuming without activating the failed revision")
		update(func(a *appv1alpha1.App) { a.Spec.Suspended = false })
		reconcileN()
		expectServingPrior(appv1alpha1.PhaseRunning, servedRev, servedPrefix)
		expectVerdict(failedGen, "qa-missing-output")

		By("recovering with a valid publish directory")
		update(func(a *appv1alpha1.App) { a.Spec.PublishPath = "." })
		reconcileN()
		settleJob("")
		reconcileN()
		recovered := get()
		Expect(recovered.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(recovered.Status.ActiveRevision).To(Equal(releaseRevision(recovered)))
		Expect(recovered.Status.ActiveRevision).NotTo(Equal(servedRev))
		Expect(recovered.Status.StaticPrefix).NotTo(Equal(servedPrefix))
		// The earlier verdict stays bound to its own generation; it cannot read as
		// the recovered release's failure.
		expectVerdict(failedGen, "qa-missing-output")
		Expect(releaseGeneration(recovered)).NotTo(Equal(failedGen))
	})
})

// A retried publish Job that succeeds for the same generation must not leave
// that generation's failure verdict on the now-live release; another
// generation's verdict, or a non-publish rollout verdict, is not ours to drop.
func TestClearPublishVerdictOnlyForThePublishedRelease(t *testing.T) {
	for _, tc := range []struct {
		name   string
		reason string
		gen    int64
		kept   bool
	}{
		{"same release", reasonPublishFailed, 4, false},
		{"older release", reasonPublishFailed, 3, true},
		{"rollout verdict", "ImagePullBackOff", 4, true},
	} {
		app := &appv1alpha1.App{}
		app.Status.ReleaseGeneration = 4
		app.Status.Conditions = []metav1.Condition{{
			Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: tc.reason, ObservedGeneration: tc.gen,
		}}
		clearPublishVerdict(app)
		if kept := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionRollout) != nil; kept != tc.kept {
			t.Errorf("%s: kept = %v, want %v", tc.name, kept, tc.kept)
		}
	}
}
