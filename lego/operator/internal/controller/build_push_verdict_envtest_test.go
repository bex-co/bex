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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w33/m569 against a real API server: a cached build's Job (its cache-save
// rule included) is accepted, its release rolls out once the push's container
// exits 0 while cache-save still runs, and a cache-save the kernel then kills
// fails the Job without failing the build.
var _ = Describe("A build's verdict is its image push (w33/m569)", func() {
	const name = "push-verdict-app"
	ctx := context.Background()
	nn := types.NamespacedName{Name: name, Namespace: "default"}
	podKey := types.NamespacedName{Name: build.JobName(name, "gen-1") + "-a", Namespace: "default"}

	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{
			Client: k8sClient, Scheme: k8sClient.Scheme(),
			Mode: ModeKubernetes, Registry: "zot.test:5000", BuildNamespace: "default", BuildCache: true,
		}
	})
	reconcile1 := func() {
		GinkgoHelper()
		_, err := r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
		Expect(err).NotTo(HaveOccurred())
	}
	// dispatch creates the App and returns the cached build Job it dispatches.
	dispatch := func() *batchv1.Job {
		GinkgoHelper()
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec: appv1alpha1.AppSpec{
				Repo: "https://github.com/bex-co/hello", Branch: "main", Builder: "dockerfile", Port: 3000,
			},
		})).To(Succeed())
		for range 3 {
			reconcile1()
		}
		job, err := buildJobFor(name, "gen-1")
		Expect(err).NotTo(HaveOccurred(), "the cached build Job was refused or never dispatched")
		return job
	}
	// writePod writes the Job's build pod with status, as the kubelet does.
	writePod := func(job *batchv1.Job, status corev1.PodStatus) {
		GinkgoHelper()
		pod := &corev1.Pod{}
		if k8sClient.Get(ctx, podKey, pod) != nil {
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: podKey.Name, Namespace: podKey.Namespace, Labels: map[string]string{"job-name": job.Name}},
				Spec:       *job.Spec.Template.Spec.DeepCopy(),
			}
			pod.Spec.NodeName = "node-a"
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		}
		pod.Status = status
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
	}

	AfterEach(func() {
		_ = k8sClient.Delete(ctx, &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: podKey.Name, Namespace: podKey.Namespace}})
		if a := (&appv1alpha1.App{}); k8sClient.Get(ctx, nn, a) == nil {
			Expect(k8sClient.Delete(ctx, a)).To(Succeed())
			r.Registry = "" // no registry server in this suite; deletion has focused tests
			for range 3 {
				_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn})
			}
		}
		_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, "gen-1"), Namespace: "default"}})
	})

	It("rolls the release out at the push and keeps it through a cache-save the kernel kills", func() {
		job := dispatch()
		Expect(job.Spec.Template.Spec.Containers).To(ContainElement(HaveField("Name", "cache-save")))

		By("the push exits 0 while cache-save still runs")
		started := time.Now().Add(-3 * time.Minute).Truncate(time.Second)
		pushStart, pushed := started.Add(90*time.Second), started.Add(135*time.Second)
		job.Status.StartTime = &metav1.Time{Time: started}
		Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
		push := phaseTerminated("push", 0, "Completed", pushStart, pushed)
		writePod(job, corev1.PodStatus{
			Phase: corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{push, {Name: "cache-save", State: corev1.ContainerState{
				Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(pushStart)},
			}}},
		})
		reconcile1()

		app := envtestApp(nn)
		Expect(app.Status.BuildRun).NotTo(BeNil())
		Expect(*app.Status.BuildRun).To(Equal(appv1alpha1.BuildRunStatus{
			Generation: 1, StartedAt: started.UTC().Format(time.RFC3339), FinishedAt: pushed.UTC().Format(time.RFC3339),
		}), "bex-api dates build_ended from the recorded window")
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseDeploying), "the release started at the push")
		dep := &appsv1.Deployment{}
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		built := dep.Spec.Template.Spec.Containers[0].Image
		Expect(built).To(ContainSubstring(":gen-1"))

		By("cache-save is OOM-killed and the Job fails on its rule")
		writePod(job, corev1.PodStatus{
			Phase:             corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{push, phaseTerminated("cache-save", 137, "OOMKilled", pushStart, pushed.Add(time.Minute))},
		})
		job, err := buildJobFor(name, "gen-1")
		Expect(err).NotTo(HaveOccurred())
		// failEnvtestJob would restamp the Job's start, which the API server
		// holds immutable once set.
		failed := "Container cache-save for pod default/" + podKey.Name + " failed with exit code 137 matching FailJob rule at index 2"
		job.Status.Conditions = []batchv1.JobCondition{
			{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: batchv1.JobReasonPodFailurePolicy, Message: failed},
			{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: batchv1.JobReasonPodFailurePolicy, Message: failed},
		}
		Expect(k8sClient.Status().Update(ctx, job)).To(Succeed())
		reconcile1()

		app = envtestApp(nn)
		verdict := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionBuild)
		Expect(verdict == nil || !appv1alpha1.IsBuildFailureReason(verdict.Reason)).To(BeTrue(),
			"Build condition %+v: a failed cache-save failed a build whose image was pushed", verdict)
		Expect(k8sClient.Get(ctx, nn, dep)).To(Succeed())
		Expect(dep.Spec.Template.Spec.Containers[0].Image).To(Equal(built))

		By("the release goes live")
		markEnvtestDeploymentReady(nn)
		reconcile1()
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-1"))
	})

	It("still fails a build whose push failed", func() {
		job := dispatch()
		started := time.Now().Add(-3 * time.Minute).Truncate(time.Second)
		writePod(job, corev1.PodStatus{
			Phase: corev1.PodFailed,
			ContainerStatuses: []corev1.ContainerStatus{
				phaseTerminated("push", build.ExitTenantError, "Error", started.Add(90*time.Second), started.Add(100*time.Second)),
				phaseTerminated("cache-save", 137, "OOMKilled", started.Add(90*time.Second), started.Add(150*time.Second)),
			},
		})
		failBuildJob(name, "gen-1")
		_, _ = r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) // a failed build answers its error

		verdict := meta.FindStatusCondition(envtestApp(nn).Status.Conditions, appv1alpha1.ConditionBuild)
		Expect(verdict).NotTo(BeNil())
		Expect(appv1alpha1.IsBuildFailureReason(verdict.Reason)).To(BeTrue(), "Build condition %+v, want the build failed", verdict)
		Expect(envtestApp(nn).Status.ArtifactImage).To(BeEmpty(), "a build whose push failed stored an artifact")
	})
})
