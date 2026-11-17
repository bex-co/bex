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
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// The initial deploy hook against a real API server (w33/m435): the Job the
// operator writes is admitted with its disk mount and affinity, the CRD keeps
// status.initialDeployHook, and the failure message is read off the Job's own
// pod. envtest has no kubelet, so the specs end each Job by hand.
var _ = Describe("Initial deploy hook (kubernetes runtime)", func() {
	const image = "ghcr.io/example/app:v1"

	// endJob ends the Job with exitCode, as its one pod would report it.
	endJob := func(nn types.NamespacedName, exitCode int32) {
		GinkgoHelper()
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, nn, job)).To(Succeed())
		if exitCode == 0 {
			completeEnvtestJob(job, metav1.Now())
			return
		}
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name: nn.Name + "-x7k2p", Namespace: nn.Namespace,
				Labels: map[string]string{batchv1.ControllerUidLabel: string(job.UID)},
			},
			Spec: corev1.PodSpec{
				RestartPolicy: corev1.RestartPolicyNever,
				Containers:    []corev1.Container{{Name: "predeploy", Image: image}},
			},
		}
		Expect(k8sClient.Create(ctx, pod)).To(Succeed())
		DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod) })
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{{
			Name: "predeploy", Image: image,
			State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: exitCode, Reason: "Error"}},
		}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
		failEnvtestJob(job, "BackoffLimitExceeded", "Job has reached the specified backoff limit")
	}
	jobExists := func(nn types.NamespacedName) bool {
		GinkgoHelper()
		err := k8sClient.Get(ctx, nn, &batchv1.Job{})
		if err != nil {
			Expect(errors.IsNotFound(err)).To(BeTrue(), "reading Job %s: %v", nn.Name, err)
		}
		return err == nil
	}

	It("runs the pre-deploy command before the rollout and the hook, with the service's disk, only once it is live", func() {
		nn := types.NamespacedName{Name: "hook-disk", Namespace: "default"}
		stepNN := types.NamespacedName{Name: predeploy.JobName(nn.Name, "gen-1"), Namespace: nn.Namespace}
		hookNN := types.NamespacedName{Name: predeploy.HookJobName(nn.Name, "gen-1"), Namespace: nn.Namespace}
		r := &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: nn.Name, Namespace: nn.Namespace},
			Spec: appv1alpha1.AppSpec{
				Image: image, Type: appv1alpha1.TypeWebService, Port: 3000, Tier: "starter", Replicas: 1,
				PreDeployCommand: "echo migrate", InitialDeployHook: "echo seed > /var/data/seeded",
				Disk: &appv1alpha1.DiskSpec{Name: "data", MountPath: "/var/data", SizeGB: 10},
			},
		})).To(Succeed())
		DeferCleanup(func() {
			deleteOwner(ctx, envtestApp(nn), run)
			for _, job := range []types.NamespacedName{stepNN, hookNN} {
				_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: job.Name, Namespace: job.Namespace}})
			}
		})

		By("the first release's pre-deploy step being its own command")
		reconcileToFixedPoint(run)
		step := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, stepNN, step)).To(Succeed())
		Expect(step.Spec.Template.Spec.Containers[0].Command).To(Equal([]string{"echo", "migrate"}))
		Expect(jobExists(hookNN)).To(BeFalse(), "the hook must not run before the rollout")

		By("the release rolling out: still no hook")
		endJob(stepNN, 0)
		reconcileToFixedPoint(run)
		Expect(envtestDeployment(nn).Spec.Template.Spec.Containers[0].Image).To(Equal(image))
		Expect(jobExists(hookNN)).To(BeFalse(), "the hook must not run before the release is live")

		By("the release going live: the hook starts beside the instance, on its disk")
		simulateTemplatePod(nn, "live", true)
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
		app := envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		Expect(app.Status.ActiveRevision).To(Equal("rev-1"))
		hook := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, hookNN, hook)).To(Succeed())
		pod := hook.Spec.Template.Spec
		Expect(pod.Containers[0].Command).To(Equal([]string{"/bin/sh", "-c", "echo seed > /var/data/seeded"}))
		Expect(pod.Containers[0].VolumeMounts).To(ContainElement(corev1.VolumeMount{Name: diskVolumeName, MountPath: "/var/data"}))
		Expect(pod.Volumes).To(ContainElement(HaveField("VolumeSource.PersistentVolumeClaim.ClaimName", diskPVCName(nn.Name))))
		Expect(pod.Affinity.PodAffinity.RequiredDuringSchedulingIgnoredDuringExecution).To(HaveLen(1))
		Expect(pod.HostUsers).To(Equal(envtestDeployment(nn).Spec.Template.Spec.HostUsers), "the hook mounts the volume as the instance does")
		Expect(step.Spec.Template.Spec.Volumes).NotTo(ContainElement(HaveField("VolumeSource.PersistentVolumeClaim", Not(BeNil()))),
			"the pre-deploy command has no disk access")
		Expect(app.Status.InitialDeployHook).NotTo(BeNil(), "the CRD must keep status.initialDeployHook")
		Expect(app.Status.InitialDeployHook.Status).To(Equal(appv1alpha1.PreDeployRunning))
		Expect(app.Status.InitialDeployHook.Job).To(Equal(hookNN.Name))

		By("the hook exiting 3: the release stays live and the log line names the hook and its code")
		endJob(hookNN, 3)
		reconcileToFixedPoint(run)
		app = envtestApp(nn)
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
		ready := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
		Expect(ready.Status).To(Equal(metav1.ConditionTrue))
		Expect(app.Status.InitialDeployHookCompleted).To(BeFalse())
		Expect(app.Status.InitialDeployHook.Status).To(Equal(appv1alpha1.PreDeployFailed))
		Expect(app.Status.InitialDeployHook.Message).To(Equal("the initial deploy hook exited with code 3"))
		Expect(app.Status.InitialDeployHook.FinishedAt).NotTo(BeEmpty())
		Expect(app.Status.PreDeploy.Status).To(Equal(appv1alpha1.PreDeploySucceeded), "the step's own record is untouched")

		By("the next release going live: the hook runs again, and its success ends it for good")
		app.Spec.RestartedAt = "2026-11-16T00:00:00Z"
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		step2NN := types.NamespacedName{Name: predeploy.JobName(nn.Name, "gen-2"), Namespace: nn.Namespace}
		hook2NN := types.NamespacedName{Name: predeploy.HookJobName(nn.Name, "gen-2"), Namespace: nn.Namespace}
		DeferCleanup(func() {
			for _, job := range []types.NamespacedName{step2NN, hook2NN} {
				_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: job.Name, Namespace: job.Namespace}})
			}
		})
		reconcileToFixedPoint(run)
		Expect(jobExists(step2NN)).To(BeTrue(), "the second release runs its pre-deploy command")
		Expect(jobExists(hook2NN)).To(BeFalse(), "the retry waits for the second release to go live")
		endJob(step2NN, 0)
		reconcileToFixedPoint(run)
		simulateTemplatePod(nn, "live2", true)
		markEnvtestDeploymentReady(nn)
		reconcileToFixedPoint(run)
		Expect(envtestApp(nn).Status.ActiveRevision).To(Equal("rev-2"))
		Expect(jobExists(hook2NN)).To(BeTrue(), "the owed hook runs after the second release is live")
		endJob(hook2NN, 0)
		reconcileToFixedPoint(run)
		app = envtestApp(nn)
		Expect(app.Status.InitialDeployHookCompleted).To(BeTrue())
		Expect(app.Status.InitialDeployHook.Status).To(Equal(appv1alpha1.PreDeploySucceeded))
		Expect(app.Status.InitialDeployHook.Generation).To(BeEquivalentTo(2))
		Expect(app.Status.Phase).To(Equal(appv1alpha1.PhaseRunning))
	})
})
