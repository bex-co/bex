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
	"strconv"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
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
	const repo = "https://github.com/bex-co/bex"
	nn := types.NamespacedName{Name: name, Namespace: ns}

	reconcileN := func(nn types.NamespacedName) {
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
	expectPhase := func(nn types.NamespacedName, phase appv1alpha1.AppPhase, reason, says string) {
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(phase))
		ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready).NotTo(BeNil())
		Expect(ready.Reason).To(Equal(reason))
		Expect(ready.Message).To(ContainSubstring(says))
	}
	// job is the current release's publish Job (envtest has no Job controller
	// and no kubelet, so the test plays both).
	job := func(nn types.NamespacedName) *batchv1.Job {
		jobNN := types.NamespacedName{Name: publish.JobName(nn.Name, releaseRevision(envtestApp(nn))), Namespace: ns}
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
	endClone := func(pod *corev1.Pod, exit int32, said string, started, finished metav1.Time) {
		pod.Status.InitContainerStatuses = []corev1.ContainerStatus{{
			Name: "clone",
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
				ExitCode: exit, Message: said, StartedAt: started, FinishedAt: finished,
			}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
	}
	// createSite creates a static site with no build command, so it publishes
	// straight from a clone, plus the object-store credential its publish reads.
	createSite := func(nn types.NamespacedName, branch string) {
		credential := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{Name: "static-s3", Namespace: ns},
			Data:       map[string][]byte{"AWS_ACCESS_KEY_ID": []byte("a"), "AWS_SECRET_ACCESS_KEY": []byte("s")},
		}
		if err := k8sClient.Create(ctx, credential); err == nil {
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, credential) })
		} else {
			Expect(apierrors.IsAlreadyExists(err)).To(BeTrue())
		}
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: ns},
			Spec: appv1alpha1.AppSpec{
				Type: appv1alpha1.TypeStaticSite, Repo: repo, Branch: branch,
				RootDir: "examples/static-site", PublishPath: ".", Expose: true,
			},
		}
		Expect(k8sClient.Create(ctx, app)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, app) })
	}
	const said = `could not fetch "no-such-branch-b97f" from this service's repository — check that the branch or commit exists and that bex still has access to the repo`

	It("reports the clone as the build and fails the build for a branch it cannot fetch", func() {
		createSite(nn, "")

		By("queueing the build until the publish pod is on a node")
		reconcileN(nn)
		expectPhase(nn, appv1alpha1.PhaseBuilding, reasonBuildQueued, "waiting for its build pod to be created")

		By("building while the clone step runs")
		first := job(nn)
		pod := placePod(first)
		reconcileN(nn)
		expectPhase(nn, appv1alpha1.PhaseBuilding, reasonBuilding, "Building the site from "+repo)

		By("rolling out once the clone step exited 0")
		endClone(pod, 0, "", metav1.Now(), metav1.Now())
		reconcileN(nn)
		expectPhase(nn, appv1alpha1.PhaseDeploying, reasonPublishing, "Publishing static output")

		By("serving the published release")
		completeEnvtestJob(first, metav1.Now())
		reconcileN(nn)
		served := envtestApp(nn)
		Expect(served.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		servedRev := served.Status.ActiveRevision
		Expect(servedRev).To(Equal(releaseRevision(served)))

		By("failing the build of a replacement whose branch cannot be fetched")
		latest := envtestApp(nn)
		latest.Spec.Branch = "no-such-branch-b97f"
		Expect(k8sClient.Update(ctx, latest)).To(Succeed())
		reconcileN(nn)
		failedGen := releaseGeneration(envtestApp(nn))
		replacement := job(nn)
		Expect(replacement.Name).NotTo(Equal(first.Name))
		endClone(placePod(replacement), 66, said+"\n", metav1.Now(), metav1.Now())
		failEnvtestJob(replacement, "BackoffLimitExceeded", "")
		reconcileN(nn)
		failed := envtestApp(nn)
		Expect(failed.Status.ActiveRevision).To(Equal(servedRev), "the failed revision must never become active")
		build := meta.FindStatusCondition(failed.Status.Conditions, appv1alpha1.ConditionBuild)
		Expect(build).NotTo(BeNil())
		Expect(build.Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))
		Expect(build.Message).To(Equal(said))
		Expect(build.ObservedGeneration).To(Equal(failedGen))
		Expect(meta.FindStatusCondition(failed.Status.Conditions, appv1alpha1.ConditionRollout)).To(BeNil(),
			"a clone failure is the build's verdict, not the rollout's")
		expectPhase(nn, appv1alpha1.PhaseRunning, reasonPriorReleaseServing, "the latest build failed")
	})

	// deploy is bex deploys create of commit: bex-api writes the commit and a
	// restart stamp, and stamps the release generation it writes. mutate is a
	// setting the deploy ships with it.
	deploy := func(nn types.NamespacedName, commit, at string, mutate func(*appv1alpha1.AppSpec)) int64 {
		GinkgoHelper()
		app := envtestApp(nn)
		app.Spec.BuildCommit, app.Spec.RestartedAt = commit, at
		if mutate != nil {
			mutate(&app.Spec)
		}
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(app.Generation+1, 10))
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		return envtestApp(nn).Generation
	}
	// cloneSource is the commit and directory a publish Job clones.
	cloneSource := func(j *batchv1.Job) (ref, dir string) {
		GinkgoHelper()
		for _, c := range j.Spec.Template.Spec.InitContainers {
			if c.Name == "clone" {
				return envValue(c.Env, "REF"), envValue(c.Env, "SRC_DIR")
			}
		}
		Fail("the publish Job has no clone step")
		return "", ""
	}
	rev := func(gen int64) string { return "rev-" + strconv.FormatInt(gen, 10) }
	jobAbsent := func(nn types.NamespacedName, gen int64) bool {
		GinkgoHelper()
		err := k8sClient.Get(ctx, types.NamespacedName{Name: publish.JobName(nn.Name, rev(gen)), Namespace: ns}, &batchv1.Job{})
		return apierrors.IsNotFound(err)
	}
	// publishFirst serves the site's first release.
	publishFirst := func(nn types.NamespacedName) {
		GinkgoHelper()
		createSite(nn, "main")
		reconcileN(nn)
		first := job(nn)
		endClone(placePod(first), 0, "", metav1.Now(), metav1.Now())
		completeEnvtestJob(first, metav1.Now())
		reconcileN(nn)
		Expect(envtestApp(nn).Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
	}
	const commitA, commitB = "1ca0898f8d0a4c2c5bd8b1e6e5d1ad7a0b3c9f21", "42c50892e22b8d1f0c3e6a4b7d9e2f5a1c8b3d60"

	// w8/392: under Render's default Wait the deploy in progress finishes and
	// then the newest deploy runs. A publish with no build command was never
	// pinned, so a deploy written once the operator had adopted one replaced
	// it: the first closed "Superseded by" and never served.
	It("finishes a publish in flight on its own commit before a newer deploy publishes", func() {
		site := types.NamespacedName{Name: "site-direct-wait", Namespace: ns}
		publishFirst(site)

		By("adopting a deploy of commit A, whose clone runs")
		first := deploy(site, commitA, "2026-11-22T06:43:36Z", nil)
		reconcileN(site)
		firstJob := job(site)
		Expect(firstJob.Name).To(Equal(publish.JobName(site.Name, rev(first))))
		pod := placePod(firstJob)
		reconcileN(site)
		expectPhase(site, appv1alpha1.PhaseBuilding, reasonBuilding, "Building the site from "+repo)

		By("holding a deploy of commit B with another publish directory behind it")
		second := deploy(site, commitB, "2026-11-22T06:43:46Z", func(s *appv1alpha1.AppSpec) { s.PublishPath = "sub" })
		reconcileN(site)
		app := envtestApp(site)
		Expect(app.Status.ReleaseGeneration).To(Equal(first), "the publish in flight keeps its release")
		Expect(app.Status.PendingReleaseGeneration).To(Equal(second), "the newer deploy waits in the pending slot")
		Expect(jobAbsent(site, second)).To(BeTrue(), "two releases never publish at once")
		expectPhase(site, appv1alpha1.PhaseBuilding, reasonBuilding, "Building the site from "+repo)

		By("recreating a lost publish Job from the release's own view, not the saved spec")
		Expect(k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0))).To(Succeed())
		Eventually(func() bool {
			return apierrors.IsNotFound(k8sClient.Get(ctx, client.ObjectKeyFromObject(pod), &corev1.Pod{}))
		}, "10s", "100ms").Should(BeTrue())
		background := metav1.DeletePropagationBackground
		Expect(k8sClient.Delete(ctx, firstJob, &client.DeleteOptions{PropagationPolicy: &background})).To(Succeed())
		Eventually(func() bool { return jobAbsent(site, first) }, "10s", "100ms").Should(BeTrue())
		reconcileN(site)
		firstJob = job(site)
		ref, dir := cloneSource(firstJob)
		Expect(ref).To(Equal(commitA))
		Expect(dir).To(Equal("examples/static-site"))

		By("serving commit A, then publishing commit B")
		endClone(placePod(firstJob), 0, "", metav1.Now(), metav1.Now())
		completeEnvtestJob(firstJob, metav1.Now())
		reconcileN(site)
		app = envtestApp(site)
		Expect(app.Status.ActiveRevision).To(Equal(rev(first)), "status: %+v", app.Status)
		Expect(app.Status.ReleaseGeneration).To(Equal(second), "the newer deploy proceeds once the first served")
		Expect(app.Status.PendingReleaseGeneration).To(BeZero())
		secondJob := job(site)
		ref, dir = cloneSource(secondJob)
		Expect(ref).To(Equal(commitB))
		Expect(dir).To(Equal("examples/static-site/sub"))
		endClone(placePod(secondJob), 0, "", metav1.Now(), metav1.Now())
		completeEnvtestJob(secondJob, metav1.Now())
		reconcileN(site)
		app = envtestApp(site)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.ActiveRevision).To(Equal(rev(second)))
	})

	// Render's Override cancels the deploy in progress and starts the new one
	// (w4/653): the operator holds nothing for an App the projector labels so.
	It("replaces the publish in flight at once under the Override policy", func() {
		site := types.NamespacedName{Name: "site-direct-override", Namespace: ns}
		publishFirst(site)
		app := envtestApp(site)
		app.Labels = map[string]string{appv1alpha1.LabelOverlappingDeployPolicy: appv1alpha1.OverlappingDeployPolicyOverride}
		Expect(k8sClient.Update(ctx, app)).To(Succeed())

		deploy(site, commitA, "2026-11-22T06:43:36Z", nil)
		reconcileN(site)
		placePod(job(site))
		reconcileN(site)
		second := deploy(site, commitB, "2026-11-22T06:43:46Z", nil)
		reconcileN(site)
		app = envtestApp(site)
		Expect(app.Status.ReleaseGeneration).To(Equal(second))
		ref, _ := cloneSource(job(site))
		Expect(ref).To(Equal(commitB))
	})

	// w32/034: a reconcile of the App that comes later than the publish Job's
	// whole life (a missing branch fails it within about 16 s) first sees the
	// Job already failed. The clone step's own window still reaches
	// status.buildRun for the release, in the same write as the Build verdict,
	// so bex-api reports the build it never saw running.
	It("records the clone's window as the build when it first sees the publish already failed", func() {
		late := types.NamespacedName{Name: "site-direct-build-window", Namespace: ns}
		createSite(late, "no-such-branch-b97f")
		reconcileN(late)
		expectPhase(late, appv1alpha1.PhaseBuilding, reasonBuildQueued, "waiting for its build pod to be created")
		gen := releaseGeneration(envtestApp(late))
		Expect(envtestApp(late).Status.BuildRun).To(BeNil())

		By("cloning, failing and ending the Job between two reconciles")
		started := metav1.NewTime(time.Date(2026, 11, 16, 3, 6, 1, 0, time.UTC))
		finished := metav1.NewTime(started.Add(14 * time.Second))
		publishJob := job(late)
		endClone(placePod(publishJob), 66, said+"\n", started, finished)
		failEnvtestJob(publishJob, "BackoffLimitExceeded", "")
		reconcileN(late)

		failed := envtestApp(late)
		build := meta.FindStatusCondition(failed.Status.Conditions, appv1alpha1.ConditionBuild)
		Expect(build).NotTo(BeNil())
		Expect(build.Reason).To(Equal(appv1alpha1.ReasonBuildFailedUserError))
		Expect(build.ObservedGeneration).To(Equal(gen))
		Expect(failed.Status.BuildRun).To(Equal(&appv1alpha1.BuildRunStatus{
			Generation: gen,
			StartedAt:  "2026-11-16T03:06:01Z",
			FinishedAt: "2026-11-16T03:06:15Z",
		}), "the clone step's window is the build the deploy reports")
	})
})
