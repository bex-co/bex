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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrlmetrics "sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ADR060 D6: after D1 made observation non-blocking, admission is the build
// plane's ONLY concurrency control — the reconcile-worker count no longer
// accidentally bounds total fan-out. The per-workspace cap bounds one tenant;
// this suite pins the cluster-wide ceiling that bounds the estate, and pins that
// leaving it unset changes nothing.
var _ = Describe("Cluster-wide build admission ceiling (ADR060 D6)", func() {
	ctx := context.Background()

	// A build namespace of this spec's own. The cluster-wide count is exactly
	// that — cluster-wide — so sharing the suite's default namespace would count
	// every other spec's leftover build Jobs against this ceiling.
	var buildNS string
	buildNSSeq := 0
	BeforeEach(func() {
		buildNSSeq++
		buildNS = fmt.Sprintf("build-ceiling-%d", buildNSSeq)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: buildNS},
		})).To(Succeed())
	})

	// App namespace == workspace namespace (canonicalNamespace binding).
	nn := func(name, workspace string) types.NamespacedName {
		return types.NamespacedName{Name: name, Namespace: workspace}
	}
	// Two Apps in DIFFERENT workspaces: the per-workspace cap can never be what
	// holds the second one back, so anything observed here is the global ceiling.
	// Each App lives in its OWN workspace namespace — the canonical ADR043
	// placement (a labeled App in the shared namespace is refused by
	// canonicalNamespace since codex-security 2026-08 F11, so this suite must
	// model what production actually writes).
	createApp := func(name, workspace string) {
		_ = k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: workspace},
		}) // AlreadyExists is fine across specs
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{
				Name: name, Namespace: workspace,
				Labels: map[string]string{labelWorkspace: workspace},
			},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
	}
	getApp := func(name, workspace string) *appv1alpha1.App {
		a := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn(name, workspace), a)).To(Succeed())
		return a
	}
	// The reason/message a tenant sees ride the Ready condition, not bare status
	// fields — so that is where the ceiling's explanation has to land.
	readyCondition := func(name, workspace string) *metav1.Condition {
		c := meta.FindStatusCondition(getApp(name, workspace).Status.Conditions, appv1alpha1.ConditionReady)
		Expect(c).NotTo(BeNil(), "the App must carry a Ready condition")
		return c
	}
	jobExists := func(name, workspace string) bool {
		a := getApp(name, workspace)
		j := &batchv1.Job{}
		return k8sClient.Get(ctx, client.ObjectKey{
			Namespace: buildNS, Name: build.JobName(name, releaseBuildRevision(a)),
		}, j) == nil
	}
	reconcileN := func(r *AppReconciler, name, workspace string, n int) {
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn(name, workspace)})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	newReconciler := func(ceiling int) *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS,
			MaxActiveBuilds: ceiling,
		}
	}
	cleanup := func(apps ...[2]string) {
		for _, pair := range apps {
			name, workspace := pair[0], pair[1]
			a := &appv1alpha1.App{}
			if k8sClient.Get(ctx, nn(name, workspace), a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				r := newReconciler(0)
				r.Registry = "" // no registry server in this suite
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn(name, workspace)})
				}
			}
			for _, rev := range []string{"gen-1", "gen-2"} {
				_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
					Name: build.JobName(name, rev), Namespace: buildNS}})
			}
		}
	}

	It("holds the second workspace's build when the cluster is at the ceiling", func() {
		defer cleanup([2]string{"ceiling-a", "tea-aaa"}, [2]string{"ceiling-b", "tea-bbb"})
		r := newReconciler(1)

		createApp("ceiling-a", "tea-aaa")
		reconcileN(r, "ceiling-a", "tea-aaa", 3)
		Expect(jobExists("ceiling-a", "tea-aaa")).To(BeTrue(), "the first build must dispatch normally")

		createApp("ceiling-b", "tea-bbb")
		reconcileN(r, "ceiling-b", "tea-bbb", 3)
		Expect(jobExists("ceiling-b", "tea-bbb")).To(BeFalse(),
			"a second workspace's build must not dispatch past the cluster-wide ceiling")

		Expect(getApp("ceiling-b", "tea-bbb").Status.Phase).To(Equal(appv1alpha1.PhaseBuilding))
		cond := readyCondition("ceiling-b", "tea-bbb")
		Expect(cond.Reason).To(Equal(reasonBuildQueued))
		Expect(cond.Message).To(ContainSubstring("cluster has 1/1"),
			"the tenant-visible message must say the CLUSTER is full, not their workspace")
	})

	It("is byte-identical when unset: both builds dispatch", func() {
		defer cleanup([2]string{"uncapped-a", "tea-aaa"}, [2]string{"uncapped-b", "tea-bbb"})
		r := newReconciler(0)

		createApp("uncapped-a", "tea-aaa")
		reconcileN(r, "uncapped-a", "tea-aaa", 3)
		createApp("uncapped-b", "tea-bbb")
		reconcileN(r, "uncapped-b", "tea-bbb", 3)

		Expect(jobExists("uncapped-a", "tea-aaa")).To(BeTrue())
		Expect(jobExists("uncapped-b", "tea-bbb")).To(BeTrue(),
			"an unset ceiling must not gate anything — 0 means unlimited")
	})

	It("never stalls an App that is already building (it would deadlock on itself)", func() {
		defer cleanup([2]string{"selfgate-a", "tea-aaa"})
		r := newReconciler(1)

		createApp("selfgate-a", "tea-aaa")
		reconcileN(r, "selfgate-a", "tea-aaa", 3)
		Expect(jobExists("selfgate-a", "tea-aaa")).To(BeTrue())

		// Its own build now occupies the only slot. Observing it must keep working:
		// the cap gates a NEW dispatch, never an App's observation of its own build.
		//
		// Asserted on the MESSAGE, not the reason: envtest runs no Job controller,
		// so this Job never gets a pod, and since w6/m95 a dispatched Job with no
		// pod honestly reports BuildQueued in its own right. What must never
		// appear is the CAP's verdict — that is the self-deadlock this guards.
		reconcileN(r, "selfgate-a", "tea-aaa", 3)
		cond := readyCondition("selfgate-a", "tea-aaa")
		Expect(cond.Message).NotTo(ContainSubstring("concurrent builds active"),
			"the cap must not report an App as queued behind its own running build")
		Expect(cond.Message).NotTo(ContainSubstring("cluster has"),
			"the cluster-wide ceiling must not gate an App's own in-flight build either")
	})
})

// ADR060 D5's series must count BUILDS, not reconciles. Reconciliation is
// level-triggered and a failed build's Job lives an hour (TTL), so the terminal
// branch is re-entered many times for one build — and every re-entry would
// re-observe the SAME queue wait into bex_build_queue_seconds, which is what the
// p95 capacity alert pages on. This suite is the only thing standing between a
// single slow-queued failure and a percentile that reports it twenty times.
var _ = Describe("Build metering is once per build, not once per reconcile (ADR060 D5)", func() {
	ctx := context.Background()
	// Canonical ADR043 placement: the App lives in its own workspace namespace
	// (a labeled App in the shared namespace is refused since 2026-08 F11).
	const ns = "tea-meter"

	var buildNS string
	meterNSSeq := 0
	BeforeEach(func() {
		meterNSSeq++
		buildNS = fmt.Sprintf("build-metering-%d", meterNSSeq)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: buildNS},
		})).To(Succeed())
		_ = k8sClient.Create(ctx, &corev1.Namespace{
			ObjectMeta: metav1.ObjectMeta{Name: ns},
		}) // AlreadyExists is fine across specs
	})

	nn := func(name string) types.NamespacedName {
		return types.NamespacedName{Name: name, Namespace: ns}
	}
	newReconciler := func() *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS,
		}
	}
	// finishBuild drives the dispatched Job to a terminal state the envtest way
	// (no kubelet runs it), including the pod whose PodScheduled condition is the
	// queue observation under test.
	finishBuild := func(name string, complete bool) {
		app := &appv1alpha1.App{}
		Expect(k8sClient.Get(ctx, nn(name), app)).To(Succeed())
		jobName := build.JobName(name, releaseBuildRevision(app))
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, client.ObjectKey{Namespace: buildNS, Name: jobName}, job)).To(Succeed())

		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: jobName + "-x", Namespace: buildNS,
				Labels: map[string]string{"job-name": jobName, "app.bex.co/component": "build"},
			},
			Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "push", Image: "busybox"}}},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		scheduledAt := metav1.NewTime(job.CreationTimestamp.Add(4 * time.Minute))
		pod.Status.Conditions = []corev1.PodCondition{
			{Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: scheduledAt},
		}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())

		now := metav1.Now()
		job.Status.StartTime = &now
		if complete {
			// completionTime is only valid alongside Complete=True (apiserver-enforced).
			job.Status.CompletionTime = &now
			job.Status.Conditions = []batchv1.JobCondition{
				{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
				{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
			}
		} else {
			job.Status.Conditions = []batchv1.JobCondition{
				{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: batchv1.JobReasonPodFailurePolicy},
				{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: batchv1.JobReasonPodFailurePolicy},
			}
		}
		Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
	}
	queueObservations := func() uint64 {
		families, err := ctrlmetrics.Registry.Gather()
		Expect(err).NotTo(HaveOccurred())
		for _, f := range families {
			if f.GetName() == "bex_build_queue_seconds" && len(f.GetMetric()) > 0 {
				return f.GetMetric()[0].GetHistogram().GetSampleCount()
			}
		}
		return 0
	}
	createApp := func(name string) {
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns,
				Labels: map[string]string{labelWorkspace: ns}},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main",
				Builder: "dockerfile", Port: 3000,
			},
		})).To(Succeed())
	}

	DescribeTable("one terminal build yields exactly one queue observation",
		func(name string, complete bool) {
			r := newReconciler()
			createApp(name)
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn(name)})
			}
			finishBuild(name, complete)

			before := queueObservations()
			// Re-reconcile the way production does while the finished Job lives out
			// its hour: pod churn, Deployment churn, and (for a failure) the error
			// requeue all re-enter this branch.
			for range 6 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn(name)})
			}
			Expect(queueObservations()).To(Equal(before+1),
				"the same build's queue wait was observed more than once — bex_build_queue_seconds' p95 would count one slow build many times")
		},
		Entry("a succeeded build", "meter-ok", true),
		Entry("a failed build", "meter-fail", false),
	)
})
