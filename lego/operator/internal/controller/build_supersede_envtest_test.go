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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// This suite pins ADR060 §D1a: supersede acts on the pending slot only. Under a
// push cadence faster than build duration, the in-flight build runs to
// completion and rolls out, intermediate generations are coalesced, and the App
// converges to the latest commit — the behavior the old cancel-the-running-build
// newest-wins could not provide (it livelocked, completing nothing). It fails
// against that pre-w2/m72 semantics because that path deleted bld-gen-1 the moment
// gen-2 arrived and never let a build finish.
var _ = Describe("Build supersede semantics (ADR060 D1a)", func() {
	const name = "supersede-app"
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
	buildJob := func(rev string) (*batchv1.Job, error) {
		j := &batchv1.Job{}
		err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "default", Name: build.JobName(name, rev)}, j)
		return j, err
	}
	jobExists := func(rev string) bool {
		_, err := buildJob(rev)
		return err == nil
	}
	// completeBuild marks the already-dispatched build Job for rev as Complete,
	// the envtest stand-in for a finished in-cluster build (no kubelet runs it).
	completeBuild := func(rev string) {
		j, err := buildJob(rev)
		Expect(err).NotTo(HaveOccurred(), "build Job for %s must have been dispatched", rev)
		now := metav1.Now()
		j.Status.StartTime = &now
		j.Status.CompletionTime = &now
		j.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobSuccessCriteriaMet, Status: corev1.ConditionTrue},
			{Type: batchv1.JobComplete, Status: corev1.ConditionTrue},
		}
		Expect(k8sClient.Status().Update(ctx, j)).To(Succeed())
	}
	// push simulates a git push / redeploy: a fresh restartedAt bumps the release
	// identity (and metadata.generation), exactly like a webhook redeploy.
	push := func(minute int) {
		a := getApp()
		a.Spec.RestartedAt = fmt.Sprintf("2026-08-16T00:%02d:00Z", minute)
		Expect(k8sClient.Update(ctx, a)).To(Succeed())
	}

	AfterEach(func() {
		if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			saved := r.Registry
			r.Registry = "" // no registry server in this unit suite; deletion has focused tests
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
			r.Registry = saved
		}
		for _, rev := range []string{"gen-1", "gen-2", "gen-3", "gen-4"} {
			_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
				Name: build.JobName(name, rev), Namespace: "default"}})
		}
	})

	It("runs the in-flight build to completion, coalesces newer pushes, converges to latest", func() {
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
		Expect(getApp().Status.ReleaseGeneration).To(Equal(int64(1)))

		By("pushing twice while gen-1 is still building — newer pushes coalesce")
		push(1)
		reconcile1()
		push(2)
		reconcile1()

		By("the running gen-1 build is NOT canceled (run-to-completion)")
		j1, err := buildJob("gen-1")
		Expect(err).NotTo(HaveOccurred())
		Expect(j1.DeletionTimestamp.IsZero()).To(BeTrue(), "a newer push must never delete the running build")

		By("intermediate generations start no build of their own (coalesced into one pending slot)")
		Expect(jobExists("gen-2")).To(BeFalse())
		Expect(jobExists("gen-3")).To(BeFalse())

		By("the release stays pinned to the running build; the latest push is the pending slot")
		pinned := getApp()
		Expect(pinned.Status.ReleaseGeneration).To(Equal(int64(1)))
		Expect(pinned.Status.PendingReleaseGeneration).To(Equal(int64(3)))

		By("completing the gen-1 build — it rolls out (point of no return: image pushed)")
		completeBuild("gen-1")
		reconcile1()
		Expect(getApp().Status.Image).To(ContainSubstring(":gen-1"))
		Expect(jobExists("gen-1")).To(BeTrue(), "the completed build is not deleted by the rollout")

		By("the next reconcile advances to the latest pending generation, skipping gen-2")
		reconcile1()
		advanced := getApp()
		Expect(advanced.Status.ReleaseGeneration).To(Equal(int64(3)))
		Expect(advanced.Status.PendingReleaseGeneration).To(Equal(int64(0)))
		Expect(jobExists("gen-3")).To(BeTrue(), "the latest generation now builds")
		Expect(jobExists("gen-2")).To(BeFalse(), "the coalesced intermediate generation never builds")

		By("completing gen-3 — the App converges to the latest commit")
		completeBuild("gen-3")
		reconcile1()
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template.Spec.Containers[0].Image).To(ContainSubstring(":gen-3"))
		Expect(getApp().Status.Image).To(ContainSubstring(":gen-3"))
	})
})
