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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	"github.com/bex-co/bex/lego/operator/internal/predeploy"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/m572: Render's deploy log narrates a pre-deploy step as `==> Starting
// pre-deploy: <command>`, `==> Running '<command>'` on a native runtime, the
// command's output, then `==> Pre-deploy complete!` or `==> Pre-deploy has
// failed` and `==> Exited with status N`. bex-api writes those lines from the
// operator's record of the step in status.preDeploy: its command and launch,
// its start (microseconds), its container's start, the moment its verdict
// was settled and its exit status. envtest runs no kubelet, so each spec
// stands in for the step's pod.
var _ = Describe("The operator records a release's pre-deploy run (w32/m572)", func() {
	var r *AppReconciler
	BeforeEach(func() {
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
	})
	create := func(name string, spec appv1alpha1.AppSpec) (types.NamespacedName, func() (ctrl.Result, error)) {
		GinkgoHelper()
		nn := types.NamespacedName{Name: name, Namespace: "default"}
		Expect(k8sClient.Create(ctx, &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace}, Spec: spec})).To(Succeed())
		run := func() (ctrl.Result, error) { return r.Reconcile(ctx, reconcile.Request{NamespacedName: nn}) }
		DeferCleanup(func() {
			deleteOwner(ctx, envtestApp(nn), run)
			background := metav1.DeletePropagationBackground
			for _, rev := range []string{"gen-1", "gen-2"} {
				for _, job := range []string{build.JobName(name, rev), predeploy.JobName(name, rev)} {
					_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: job, Namespace: nn.Namespace}},
						&client.DeleteOptions{PropagationPolicy: &background})
				}
			}
		})
		return nn, run
	}
	step := func(nn types.NamespacedName) *appv1alpha1.PreDeployStatus {
		GinkgoHelper()
		pd := envtestApp(nn).Status.PreDeploy
		Expect(pd).NotTo(BeNil(), "the step has a record")
		return pd
	}
	stepJob := func(nn types.NamespacedName, gen int64) *batchv1.Job {
		GinkgoHelper()
		job := &batchv1.Job{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{
			Name: predeploy.JobName(nn.Name, appv1alpha1.BuildRevision(gen)), Namespace: nn.Namespace,
		}, job)).To(Succeed(), "the step's Job was created")
		return job
	}
	// stepPod stands in for the kubelet: the step's pod, its container in state.
	stepPod := func(job *batchv1.Job, state corev1.ContainerState) {
		GinkgoHelper()
		pod := &corev1.Pod{}
		key := types.NamespacedName{Name: job.Name + "-pod", Namespace: job.Namespace}
		if err := k8sClient.Get(ctx, key, pod); err != nil {
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace,
					Labels: map[string]string{batchv1.ControllerUidLabel: string(job.UID)}},
				Spec: corev1.PodSpec{RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{{Name: "predeploy", Image: "traefik/whoami"}}},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0)) })
		}
		pod.Status = corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "predeploy", State: state}}}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
	}
	exited := func(code int32, reason string, started time.Time) corev1.ContainerState {
		return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
			ExitCode: code, Reason: reason, StartedAt: metav1.NewTime(started), FinishedAt: metav1.NewTime(started.Add(4 * time.Second)),
		}}
	}
	// settle runs passes until two in a row return no error: the pass that
	// records a failed step returns the failure as its error.
	settle := func(run func() (ctrl.Result, error)) {
		GinkgoHelper()
		quiet := 0
		for range 10 {
			if _, err := run(); err != nil {
				quiet = 0
				continue
			}
			if quiet++; quiet == 2 {
				return
			}
		}
		Fail("the reconciler did not settle")
	}
	// sameMoment compares two recorded moments at the precision the API
	// server keeps them.
	sameMoment := func(a, b *metav1.MicroTime) bool { return a != nil && b != nil && a.Equal(b) }

	It("stamps the command, launch and start once, then the container's start, the verdict's moment and the exit status", func() {
		nn, run := create("pdr-exit", appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, PreDeployCommand: "./migrate up --env $ENV"})
		before := time.Now()
		settle(run)
		gen := releaseGeneration(envtestApp(nn))
		pd := step(nn)
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeployRunning))
		Expect(pd.Command).To(Equal("./migrate up --env $ENV"), "quoted as configured")
		Expect(pd.Native).To(BeFalse(), "an image runs the command in place of its program")
		Expect(pd.LaunchedAt).NotTo(BeNil())
		Expect(pd.LaunchedAt.Time).To(BeTemporally(">=", before.Truncate(time.Microsecond)))
		Expect(pd.StartedAt).To(Equal(pd.LaunchedAt.UTC().Format(time.RFC3339)), "the launch is the step's start, below the second")
		Expect(pd.ContainerStartedAt).To(BeNil(), "no container has started")
		Expect(pd.SettledAt).To(BeNil())
		Expect(pd.ExitCode).To(BeNil())
		launched := *pd.LaunchedAt

		By("a requeue and a manager restart keep the launch")
		settle(run)
		r = &AppReconciler{Client: k8sClient, Scheme: k8sClient.Scheme(), Mode: ModeKubernetes}
		settle(run)
		Expect(sameMoment(step(nn).LaunchedAt, &launched)).To(BeTrue(), "the launch moved to %v", step(nn).LaunchedAt)

		By("the step's container starts")
		job := stepJob(nn, gen)
		t0 := time.Now().Add(-30 * time.Second).Truncate(time.Second)
		stepPod(job, runningSince(t0))
		settle(run)
		Expect(step(nn).ContainerStartedAt).NotTo(BeNil())
		Expect(step(nn).ContainerStartedAt.Time).To(BeTemporally("==", t0))
		Expect(step(nn).ExitCode).To(BeNil(), "a running command has no exit status")

		By("the command exits 1")
		stepPod(job, exited(1, "Error", t0))
		failEnvtestJob(job, batchv1.JobReasonBackoffLimitExceeded, "Job has reached the specified backoff limit")
		settle(run)
		pd = step(nn)
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeployFailed))
		Expect(pd.Message).To(ContainSubstring("exited with code 1"))
		Expect(pd.ExitCode).To(HaveValue(Equal(int32(1))))
		Expect(pd.OOMKilled).To(BeFalse())
		Expect(pd.SettledAt).NotTo(BeNil())
		Expect(pd.SettledAt.After(launched.Time)).To(BeTrue())
		Expect(pd.ContainerStartedAt.Time).To(BeTemporally("==", t0))
		Expect(sameMoment(pd.LaunchedAt, &launched)).To(BeTrue())
		Expect(pd.Command).To(Equal("./migrate up --env $ENV"))
		settled := *pd.SettledAt

		By("a re-check of the persisted verdict moves nothing")
		settle(run)
		Expect(sameMoment(step(nn).SettledAt, &settled)).To(BeTrue())

		By("a newer release replaces the record")
		app := envtestApp(nn)
		app.Spec.PreDeployCommand = "./migrate up --v2"
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(app.Generation+1, 10))
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		settle(run)
		pd = step(nn)
		Expect(pd.Generation).To(BeNumerically(">", gen))
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeployRunning))
		Expect(pd.Command).To(Equal("./migrate up --v2"))
		Expect(pd.LaunchedAt.After(settled.Time)).To(BeTrue())
		Expect(pd.ContainerStartedAt).To(BeNil(), "the older release's pod is not this step's")
		Expect(pd.SettledAt).To(BeNil())
		Expect(pd.ExitCode).To(BeNil())
	})

	It("settles a passing step before the release's rollout starts", func() {
		nn, run := create("pdr-pass", appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, PreDeployCommand: "echo migrated"})
		settle(run)
		job := stepJob(nn, releaseGeneration(envtestApp(nn)))
		t0 := time.Now().Add(-10 * time.Second).Truncate(time.Second)
		stepPod(job, exited(0, "Completed", t0))
		completeEnvtestJob(job, metav1.Now())
		settle(run)
		app := envtestApp(nn)
		pd := app.Status.PreDeploy
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeploySucceeded))
		Expect(pd.ExitCode).To(HaveValue(BeZero()))
		Expect(pd.ContainerStartedAt.Time).To(BeTemporally("==", t0))
		Expect(pd.SettledAt).NotTo(BeNil())
		Expect(app.Status.ReleaseRollout).NotTo(BeNil(), "the rollout starts once the step passed")
		Expect(app.Status.ReleaseRollout.StartedAt.After(pd.SettledAt.Time)).To(BeTrue(),
			"`==> Pre-deploy complete!` comes before `==> Deploying...`: settled %v, rollout %v", pd.SettledAt, app.Status.ReleaseRollout.StartedAt)
	})

	It("records the kernel's memory kill with its exit status", func() {
		nn, run := create("pdr-oom", appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, PreDeployCommand: "./migrate"})
		settle(run)
		job := stepJob(nn, releaseGeneration(envtestApp(nn)))
		stepPod(job, exited(137, "OOMKilled", time.Now().Add(-time.Minute)))
		failEnvtestJob(job, batchv1.JobReasonBackoffLimitExceeded, "Job has reached the specified backoff limit")
		settle(run)
		pd := step(nn)
		Expect(pd.Message).To(ContainSubstring("ran out of memory"), "the message keeps its own wording")
		Expect(pd.ExitCode).To(HaveValue(Equal(int32(137))))
		Expect(pd.OOMKilled).To(BeTrue())
	})

	It("records no container start and no exit status for a command that never ran", func() {
		By("an image that cannot be pulled")
		pull, pullRun := create("pdr-pull", appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, PreDeployCommand: "./migrate"})
		settle(pullRun)
		stepPod(stepJob(pull, releaseGeneration(envtestApp(pull))), corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{
			Reason: "InvalidImageName", Message: `Failed to apply default image tag "traefik/Whoami": invalid reference format`,
		}})
		settle(pullRun)
		pd := step(pull)
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeployFailed))
		Expect(pd.Command).To(Equal("./migrate"), "the step was launched")
		Expect(pd.LaunchedAt).NotTo(BeNil())
		Expect(pd.SettledAt).NotTo(BeNil())
		Expect(pd.ContainerStartedAt).To(BeNil())
		Expect(pd.ExitCode).To(BeNil())

		By("a program the runtime could not launch")
		launch, launchRun := create("pdr-start-error", appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, PreDeployCommand: "./migrate"})
		settle(launchRun)
		job := stepJob(launch, releaseGeneration(envtestApp(launch)))
		stepPod(job, corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 128, Reason: "StartError",
			Message: `exec: "/bin/sh": stat /bin/sh: no such file or directory`}})
		failEnvtestJob(job, batchv1.JobReasonBackoffLimitExceeded, "Job has reached the specified backoff limit")
		settle(launchRun)
		pd = step(launch)
		Expect(pd.Message).To(ContainSubstring("could not start"))
		Expect(pd.ContainerStartedAt).To(BeNil())
		Expect(pd.ExitCode).To(BeNil())

		By("a step refused before its Job existed")
		minutes, minutesRun := create("pdr-minutes", appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, PreDeployCommand: "./migrate"})
		app := envtestApp(minutes)
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationBuildMinutesExhausted, appv1alpha1.BuildMinutesMonth(time.Now()))
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		settle(minutesRun)
		pd = step(minutes)
		Expect(pd.Status).To(Equal(appv1alpha1.PreDeployFailed))
		Expect(pd.Message).To(Equal(appv1alpha1.BuildMinutesExhaustedMessage))
		Expect(pd.Command).To(BeEmpty(), "nothing was launched")
		Expect(pd.LaunchedAt).To(BeNil())
		Expect(pd.ContainerStartedAt).To(BeNil())
		Expect(pd.ExitCode).To(BeNil())
	})

	It("marks the launch native only where the command runs under a native image's entrypoint", func() {
		native, nativeRun := create("pdr-native", appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 3000, Replicas: 1,
			Runtime: "node", Builder: build.BuilderNative, BuildCommand: "npm ci", StartCommand: "node main.js",
			PreDeployCommand: "node migrate.js",
		})
		docker, dockerRun := create("pdr-docker", appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 3000, Replicas: 1,
			Builder: "dockerfile", StartCommand: "./server", PreDeployCommand: "./migrate",
		})
		settle(nativeRun)
		settle(dockerRun)
		Expect(envtestApp(native).Status.PreDeploy).To(BeNil(), "nothing runs while it builds")
		completeBuildJob(native.Name, "gen-1")
		completeBuildJob(docker.Name, "gen-1")
		settle(nativeRun)
		settle(dockerRun)

		Expect(stepJob(native, 1).Spec.Template.Spec.Containers[0].Command).To(BeEmpty(), "native-init runs it")
		Expect(step(native).Command).To(Equal("node migrate.js"))
		Expect(step(native).Native).To(BeTrue())
		Expect(stepJob(docker, 1).Spec.Template.Spec.Containers[0].Command).NotTo(BeEmpty())
		Expect(step(docker).Command).To(Equal("./migrate"))
		Expect(step(docker).Native).To(BeFalse())
	})
})
