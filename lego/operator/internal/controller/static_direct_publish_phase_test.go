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
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/bex-co/bex/lego/operator/internal/publish"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/028: a static site published without a build command clones its
// repository as its build, as on Render. Its status reads a build (queued, then
// building) until the clone step exits 0, a rollout while it uploads, and a
// branch the clone cannot fetch fails the build, not the rollout.
var _ = Describe("a direct static publish (w32/028)", func() {
	const ns = "default"
	const name = "site-direct-build-phase"
	nn := types.NamespacedName{Name: name, Namespace: ns}

	reconcileN := func() {
		r := &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, BaseDomain: "onbex.co", ClusterIssuer: "letsencrypt-prod",
			StaticStore:         publish.Store{Bucket: "bex-static", Endpoint: "https://s3.example.com", Secret: "static-s3"},
			StaticServerService: "bex-static-server", StaticServerPort: 8080,
		}
		for range 3 {
			_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		}
	}
	expectPhase := func(phase appv1alpha1.AppPhase, reason, says string) {
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(phase))
		ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal(reason))
		Expect(ready.Message).To(ContainSubstring(says))
	}
	// job is the current release's publish Job (envtest has no Job controller
	// and no kubelet, so the test plays both).
	job := func() *batchv1.Job {
		jobNN := types.NamespacedName{Name: publish.JobName(name, releaseRevision(envtestApp(nn))), Namespace: ns}
		var j batchv1.Job
		Eventually(func() error { return k8sClient.Get(ctx, jobNN, &j) }, "30s", "250ms").Should(Succeed())
		return &j
	}
	placePod := func(j *batchv1.Job) *corev1.Pod {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: j.Name + "-abcde", Namespace: ns, Labels: map[string]string{"job-name": j.Name}},
			Spec: corev1.PodSpec{
				NodeName:       "node-a",
				InitContainers: []corev1.Container{{Name: "clone", Image: "x"}},
				Containers:     []corev1.Container{{Name: "upload", Image: "x"}},
			},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod) })
		return pod
	}
	endClone := func(pod *corev1.Pod, exit int32, said string) {
		pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
			Name:  "clone",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: exit, Message: said}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
	}

	It("reports the clone as the build and fails the build for a branch it cannot fetch", func() {
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3", Namespace: ns},
			Data:       map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("a"), "AWS_SECRET_ACCESS_KEY": []byte("s")},
		}
		if err := k8sClient.Create(ctx, credential); err == nil {
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })
		} else {
			Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
		}
		const repo = "https://github.com/bex-co/bex"
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeStaticSite, Repo: repo,
				RootDir: "examples/static-site", PublishPath: ".", Expose: true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, app) })

		By("queueing the build until the publish pod is on a node")
		reconcileN()
		expectPhase(appv1alpha1.PhaseBuilding, reasonBuildQueued, "waiting for its build pod to be created")

		By("building while the clone step runs")
		first := job()
		pod := placePod(first)
		reconcileN()
		expectPhase(appv1alpha1.PhaseBuilding, reasonBuilding, "Building the site from "+repo)

		By("rolling out once the clone step exited 0")
		endClone(pod, 0, "")
		reconcileN()
		expectPhase(appv1alpha1.PhaseDeploying, reasonPublishing, "Publishing static output")

		By("serving the published release")
		completeEnvtestJob(first, metav1.Now())
		reconcileN()
		served := envtestApp(nn)
		Expect(served.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		servedRev := served.Status.ActiveRevision
		Expect(servedRev).To(Equal(releaseRevision(served)))

		By("failing the build of a replacement whose branch cannot be fetched")
		latest := envtestApp(nn)
		latest.Spec.Branch = "no-such-branch-b97f"
		Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		reconcileN()
		failedGen := releaseGeneration(envtestApp(nn))
		replacement := job()
		Expect(replacement.Name).NotTo(Equal(first.Name))
		const said = `could not fetch "no-such-branch-b97f" from this service's repository — check that the branch or commit exists and that bex still has access to the repo`
		endClone(placePod(replacement), 66, said+"\n")
		failEnvtestJob(replacement, "BackoffLimitExceeded", "")
		reconcileN()
		failed := envtestApp(nn)
		Expect(failed.Status.ActiveRevision).To(Equal(servedRev), "the failed revision must never become active")
		build := meta.FindStatusCondition(failed.Status.Conditions, appv1alpha1.ConditionBuild)
		Expect(build).NotTo(BeNil())
		Expect(build.Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))
		Expect(build.Message).To(Equal(said))
		Expect(build.ObservedGeneration).To(Equal(failedGen))
		Expect(meta.FindStatusCondition(failed.Status.Conditions, appv1alpha1.ConditionRollout)).To(BeNil(),
			"a clone failure is the build's verdict, not the rollout's")
		expectPhase(appv1alpha1.PhaseRunning, reasonPriorReleaseServing, "the latest build failed")
	})
})
