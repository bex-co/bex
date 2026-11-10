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
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w6/m306 (docs/ADR060 §D5/§D6): a release the cluster ceiling holds has no
// build Job, so its wait lives in status.buildHold; a freed slot goes to the
// workspace running the fewest builds, then to the longest wait; and a build
// ending wakes the held releases instead of leaving them to a 30 s requeue.
var _ = Describe("Build holds and fair order at the cluster ceiling (w6/m306)", func() {
	ctx := context.Background()

	var buildNS string
	seq := 0
	var created [][2]string
	BeforeEach(func() {
		seq++
		buildNS = fmt.Sprintf("build-fair-%d", seq)
		Expect(k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: buildNS}})).To(Succeed())
		created = nil
	})
	AfterEach(func() {
		for _, pair := range created {
			a := &appv1alpha1.App{}
			if k8sClient.Get(ctx, types.NamespacedName{Name: pair[0], Namespace: pair[1]}, a) == nil {
				Expect(k8sClient.Delete(ctx, a)).To(Succeed())
				r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes, BuildNamespace: buildNS}
				for range 3 {
					_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: types.NamespacedName{Name: pair[0], Namespace: pair[1]}})
				}
			}
		}
	})

	nn := func(name, workspace string) types.NamespacedName {
		return types.NamespacedName{Name: name, Namespace: workspace}
	}
	createApp := func(name, workspace string) {
		_ = k8sClient.Create(ctx, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: workspace}})
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: workspace, Labels: map[string]string{labelWorkspace: workspace}},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main", Builder: "dockerfile", Port: 3000,
			},
		})).To(Succeed())
		created = append(created, [2]string{name, workspace})
	}
	getApp := func(name, workspace string) *appv1alpha1.App { return envtestApp(nn(name, workspace)) }
	jobFor := func(name, workspace string) (*batchv1.Job, bool) {
		j := &batchv1.Job{}
		err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: buildNS, Name: build.JobName(name, releaseBuildRevision(getApp(name, workspace))),
		}, j)
		return j, err == nil
	}
	reconcileN := func(r *AppReconciler, name, workspace string, n int) {
		for range n {
			_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn(name, workspace)})
			Expect(err).NotTo(HaveOccurred())
		}
	}
	// endBuild fails an App's dispatched build and lets the App observe it,
	// which is what frees its slot.
	endBuild := func(r *AppReconciler, name, workspace string) {
		j, ok := jobFor(name, workspace)
		Expect(ok).To(BeTrue(), "%s must have dispatched", name)
		failEnvtestJob(j, batchv1.JobReasonPodFailurePolicy, "container exit code 90")
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn(name, workspace)})
	}
	newReconciler := func(ceiling int) *AppReconciler {
		return &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: buildNS,
			MaxActiveBuilds: ceiling,
		}
	}
	setHoldSince := func(name, workspace string, since time.Time) {
		a := getApp(name, workspace)
		Expect(a.Status.BuildHold).NotTo(BeNil())
		a.Status.BuildHold.Since = metav1.NewTime(since)
		Expect(k8sClient.Status().Update(ctx, a)).To(Succeed())
	}

	It("records a held release's wait and observes it once the build dispatches", func() {
		r := newReconciler(1)
		createApp("hold-a", "tea-hold-a")
		reconcileN(r, "hold-a", "tea-hold-a", 3)
		createApp("hold-b", "tea-hold-b")
		reconcileN(r, "hold-b", "tea-hold-b", 3)

		held := getApp("hold-b", "tea-hold-b")
		Expect(held.Status.BuildHold).NotTo(BeNil(), "the held release must record its wait in status")
		Expect(held.Status.BuildHold.Cause).To(Equal(appv1alpha1.BuildHoldAdmission))
		Expect(held.Status.BuildHold.Generation).To(Equal(releaseGeneration(held)))
		Expect(heldBuild(held)).NotTo(BeNil(), "the census must see it")
		since := held.Status.BuildHold.Since

		reconcileN(r, "hold-b", "tea-hold-b", 2)
		Expect(getApp("hold-b", "tea-hold-b").Status.BuildHold.Since).To(Equal(since),
			"a requeue must not restart the wait")

		before := holdObservations(appv1alpha1.BuildHoldAdmission)
		endBuild(r, "hold-a", "tea-hold-a")
		reconcileN(r, "hold-b", "tea-hold-b", 1)
		_, dispatched := jobFor("hold-b", "tea-hold-b")
		Expect(dispatched).To(BeTrue(), "the freed slot must go to the held release")
		Expect(getApp("hold-b", "tea-hold-b").Status.BuildHold).To(BeNil(), "the hold ends when the build exists")
		Expect(holdObservations(appv1alpha1.BuildHoldAdmission)).To(Equal(before+1), "the whole hold is observed exactly once")
	})

	It("gives a freed slot to the workspace running fewer builds", func() {
		r := newReconciler(2)
		createApp("busy-1", "tea-busy")
		reconcileN(r, "busy-1", "tea-busy", 3)
		createApp("busy-2", "tea-busy")
		reconcileN(r, "busy-2", "tea-busy", 3)
		createApp("busy-3", "tea-busy")
		reconcileN(r, "busy-3", "tea-busy", 3)
		createApp("quiet-1", "tea-quiet")
		reconcileN(r, "quiet-1", "tea-quiet", 3)
		_, busy3 := jobFor("busy-3", "tea-busy")
		_, quiet1 := jobFor("quiet-1", "tea-quiet")
		Expect(busy3 || quiet1).To(BeFalse(), "both wait while the ceiling is full")
		// busy-3 has waited longer; the quieter workspace still goes first.
		setHoldSince("busy-3", "tea-busy", time.Now().Add(-10*time.Minute))

		endBuild(r, "busy-1", "tea-busy")
		reconcileN(r, "busy-3", "tea-busy", 1)
		_, busy3 = jobFor("busy-3", "tea-busy")
		Expect(busy3).To(BeFalse(), "a workspace already running a build must not take the slot ahead of one running none")
		cond := meta.FindStatusCondition(getApp("busy-3", "tea-busy").Status.Conditions, appv1alpha1.ConditionReady)
		Expect(cond.Message).To(Equal(clusterFullMessage), "a free slot taken by an earlier hold reads as a full cluster")

		reconcileN(r, "quiet-1", "tea-quiet", 1)
		_, quiet1 = jobFor("quiet-1", "tea-quiet")
		Expect(quiet1).To(BeTrue(), "the quieter workspace's held release takes the slot")

		reconcileN(r, "busy-3", "tea-busy", 1)
		_, busy3 = jobFor("busy-3", "tea-busy")
		Expect(busy3).To(BeFalse(), "the ceiling is full again")
	})

	It("between equally busy workspaces, gives the slot to the longer wait", func() {
		r := newReconciler(1)
		createApp("first", "tea-eq-x")
		reconcileN(r, "first", "tea-eq-x", 3)
		// Names chosen so the tie-break by name would pick the newer wait.
		createApp("a-newer", "tea-eq-a")
		reconcileN(r, "a-newer", "tea-eq-a", 3)
		createApp("z-older", "tea-eq-z")
		reconcileN(r, "z-older", "tea-eq-z", 3)
		setHoldSince("z-older", "tea-eq-z", time.Now().Add(-5*time.Minute))
		setHoldSince("a-newer", "tea-eq-a", time.Now().Add(-1*time.Minute))

		endBuild(r, "first", "tea-eq-x")
		reconcileN(r, "a-newer", "tea-eq-a", 1)
		_, newer := jobFor("a-newer", "tea-eq-a")
		Expect(newer).To(BeFalse(), "the newer wait must not jump the queue")
		reconcileN(r, "z-older", "tea-eq-z", 1)
		_, older := jobFor("z-older", "tea-eq-z")
		Expect(older).To(BeTrue(), "the longer wait takes the slot")
	})

	It("wakes the releases first in line when a build ends, not every held one", func() {
		r := newReconciler(1)
		r.buildSlotFreed = make(chan event.GenericEvent, buildSlotWakeBuffer)
		createApp("wake-a", "tea-wake-a")
		reconcileN(r, "wake-a", "tea-wake-a", 3)
		for _, name := range []string{"wake-b", "wake-c", "wake-d"} {
			createApp(name, "tea-"+name)
			reconcileN(r, name, "tea-"+name, 3)
		}
		setHoldSince("wake-c", "tea-wake-c", time.Now().Add(-3*time.Minute))
		setHoldSince("wake-b", "tea-wake-b", time.Now().Add(-2*time.Minute))
		setHoldSince("wake-d", "tea-wake-d", time.Now().Add(-1*time.Minute))
		Expect(r.buildSlotFreed).To(BeEmpty(), "nothing freed yet")

		endBuild(r, "wake-a", "tea-wake-a")
		var woken []string
		for len(r.buildSlotFreed) > 0 {
			woken = append(woken, (<-r.buildSlotFreed).Object.GetName())
		}
		Expect(woken).To(Equal([]string{"wake-c", "wake-b"}),
			"one slot freed: wake the release first in line, plus one in case it moved on — in fair order")
	})
})
