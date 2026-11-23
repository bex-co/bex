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
	"strconv"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// rolloutTemplate is a serving template labelled with rev.
func rolloutTemplate(rev string) corev1.PodTemplateSpec {
	return corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{labelRevision: rev}}}
}

// w32/m560: the first write of a release's own template starts its record,
// once. A resync, a cancel's revert to an older release and a template that is
// not the release's own leave it; a newer release replaces it. Whether the
// instance names its command (w32/m575) is decided with the start.
func TestStampReleaseRolloutStartsEachReleasesRecordOnce(t *testing.T) {
	t0 := time.Date(2026, 11, 22, 9, 11, 43, 123456000, time.UTC)
	app := &appv1alpha1.App{Status: appv1alpha1.AppStatus{ReleaseGeneration: 4}}

	stampReleaseRollout(app, rolloutTemplate("rev-3"), "npm start", true, 1, t0)
	if app.Status.ReleaseRollout != nil {
		t.Fatalf("the served release's template stamped release 4: %+v", app.Status.ReleaseRollout)
	}
	stampReleaseRollout(app, rolloutTemplate("rev-4"), "npm start", true, 1, t0)
	want := appv1alpha1.ReleaseRolloutStatus{Generation: 4, StartedAt: metav1.NewMicroTime(t0), Command: "npm start", WebConcurrency: 1, InstanceAnnounces: true}
	if rec := app.Status.ReleaseRollout; rec == nil || rec.Generation != want.Generation || !rec.StartedAt.Equal(&want.StartedAt) ||
		rec.Command != want.Command || rec.WebConcurrency != want.WebConcurrency || rec.InstanceAnnounces != want.InstanceAnnounces {
		t.Fatalf("first write: record = %+v, want %+v", rec, want)
	}
	app.Status.ReleaseRollout.AppStarts = []metav1.Time{metav1.NewTime(t0.Add(9 * time.Second).Truncate(time.Second))}

	stampReleaseRollout(app, rolloutTemplate("rev-4"), "node other.js", false, 2, t0.Add(time.Minute))
	if rec := app.Status.ReleaseRollout; !rec.StartedAt.Equal(&want.StartedAt) || rec.Command != "npm start" || len(rec.AppStarts) != 1 ||
		!rec.InstanceAnnounces {
		t.Fatalf("a resync moved the record: %+v", rec)
	}

	app.Status.ReleaseGeneration = 3
	stampReleaseRollout(app, rolloutTemplate("rev-3"), "npm start", false, 1, t0.Add(2*time.Minute))
	if rec := app.Status.ReleaseRollout; rec.Generation != 4 || !rec.StartedAt.Equal(&want.StartedAt) || !rec.InstanceAnnounces {
		t.Fatalf("a revert to release 3 restamped the record: %+v", rec)
	}

	app.Status.ReleaseGeneration = 5
	stampReleaseRollout(app, rolloutTemplate("rev-5"), "", true, 0, t0.Add(3*time.Minute))
	if rec := app.Status.ReleaseRollout; rec.Generation != 5 || !rec.StartedAt.Time.Equal(t0.Add(3*time.Minute)) ||
		rec.Command != "" || rec.WebConcurrency != 0 || len(rec.AppStarts) != 0 || rec.InstanceAnnounces {
		t.Fatalf("a newer release must replace the record whole, and a launch with no command announces nothing: %+v", rec)
	}

	app.Status.ReleaseGeneration = 6
	stampReleaseRollout(app, rolloutTemplate("rev-6"), "npm start", false, 1, t0.Add(4*time.Minute))
	if rec := app.Status.ReleaseRollout; rec.Generation != 6 || rec.Command != "npm start" || rec.InstanceAnnounces {
		t.Fatalf("a release on an image built before native-init's line must keep the narrated line: %+v", rec)
	}
}

// w32/m560: Render's `==> Running '<command>'` names a native runtime's start
// command, as configured; a Docker or prebuilt image's program is not named,
// with or without a Docker Command.
func TestNativeLaunchCommandIsOnlyANativeImagesStartCommand(t *testing.T) {
	const command = "uvicorn app.main:app --host 0.0.0.0 --port $PORT"
	const built = "zot/web@sha256:a"
	source := func(runtime, builder string) *appv1alpha1.App {
		return &appv1alpha1.App{Spec: appv1alpha1.AppSpec{
			Repo: "https://github.com/x/y", Runtime: runtime, Builder: builder, StartCommand: command,
		}}
	}
	prebuilt := &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Image: "docker.io/library/python:3.13", StartCommand: command}}
	for _, tc := range []struct {
		name   string
		app    *appv1alpha1.App
		image  string
		launch launchMode
		want   string
	}{
		{"a native build's CMD", source("python", ""), built, launchUnknown, command},
		{"a native image recorded as launching its own CMD", source("python", ""), built, launchImage, command},
		{"a native image given the command as arguments", source("docker", ""), built, launchNativeArgs, command},
		{"a native build without a start command", &appv1alpha1.App{Spec: appv1alpha1.AppSpec{Repo: "https://github.com/x/y", Runtime: "node"}}, built, launchUnknown, ""},
		{"a Dockerfile build's Docker Command", source("docker", ""), built, launchUnknown, ""},
		{"a recorded shell launch", source("python", ""), built, launchShell, ""},
		{"a prebuilt image's Docker Command", prebuilt, prebuilt.Spec.Image, launchUnknown, ""},
	} {
		if got := nativeLaunchCommand(tc.app, tc.image, tc.launch); got != tc.want {
			t.Errorf("%s: command = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// appPod is a pod of revision whose app container shows state and, after a
// restart, last.
func appPod(name, revision string, state, last corev1.ContainerState) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{labelRevision: revision}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{
			{Name: appContainerName, State: state, LastTerminationState: last},
		}},
	}
}

func runningSince(t time.Time) corev1.ContainerState {
	return corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(t)}}
}

func exitedRun(started, finished time.Time) corev1.ContainerState {
	return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
		ExitCode: 1, StartedAt: metav1.NewTime(started), FinishedAt: metav1.NewTime(finished),
	}}
}

// startTimes is starts as time values, for comparison.
func startTimes(starts []metav1.Time) []time.Time {
	out := make([]time.Time, 0, len(starts))
	for _, s := range starts {
		out = append(out, s.UTC())
	}
	return out
}

// w32/m560: each run of the release's app container is one start, read from
// its current state and, after a restart, the run before. Another release's
// pods, a pod being deleted and a sidecar are not the release's app.
func TestWithAppStartsReadsEachRunOfTheReleasesAppContainerOnce(t *testing.T) {
	t0 := time.Date(2026, 11, 22, 22, 14, 58, 0, time.UTC)
	at := func(s int) time.Time { return t0.Add(time.Duration(s) * time.Second) }
	none := corev1.ContainerState{}

	deleting := appPod("web-gone", "rev-4", runningSince(at(1)), none)
	deleting.DeletionTimestamp = &metav1.Time{Time: at(30)}
	sidecar := appPod("web-side", "rev-4", none, none)
	sidecar.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "health", State: runningSince(at(2))}}
	pods := []corev1.Pod{
		appPod("web-old", "rev-3", runningSince(at(-600)), none),
		deleting,
		sidecar,
		appPod("web-a", "rev-4", runningSince(at(8)), exitedRun(at(0), at(4))),
		appPod("web-b", "rev-4", exitedRun(at(0), at(5)), none),
	}
	got := withAppStarts(nil, pods, "rev-4")
	if want := []time.Time{at(0), at(8)}; !equalTimes(startTimes(got), want) {
		t.Fatalf("starts = %v, want %v: the first run once, then the restart", startTimes(got), want)
	}
	if again := withAppStarts(got, pods, "rev-4"); &again[0] != &got[0] || len(again) != len(got) {
		t.Fatalf("an unchanged reading must return the record itself, got %v", startTimes(again))
	}

	// A start read late is placed in order, and the oldest are kept.
	full := make([]metav1.Time, 0, appv1alpha1.MaxReleaseRolloutAppStarts-1)
	for i := range appv1alpha1.MaxReleaseRolloutAppStarts - 1 {
		full = append(full, metav1.NewTime(at(100+i)))
	}
	got = withAppStarts(full, []corev1.Pod{appPod("web-a", "rev-4", runningSince(at(500)), exitedRun(at(50), at(60)))}, "rev-4")
	if len(got) != appv1alpha1.MaxReleaseRolloutAppStarts || !got[0].Time.Equal(at(50)) || !got[len(got)-1].Time.Equal(at(108)) {
		t.Fatalf("bounded starts = %v, want the oldest %d from %v", startTimes(got), appv1alpha1.MaxReleaseRolloutAppStarts, at(50))
	}
	if again := withAppStarts(got, []corev1.Pod{appPod("web-a", "rev-4", runningSince(at(900)), none)}, "rev-4"); len(again) != len(got) {
		t.Fatalf("a full record must not grow: %v", startTimes(again))
	}
}

func equalTimes(a, b []time.Time) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !a[i].Equal(b[i]) {
			return false
		}
	}
	return true
}

// w32/m560: starts are the deploy's only while its rollout runs. Once the
// release serves, fails or is canceled over the served release, a restart
// is the service's own; a template that is not the release's is not its
// rollout.
func TestReleaseRolloutRunsUntilTheReleaseSettles(t *testing.T) {
	rolling := func() *appv1alpha1.App {
		return &appv1alpha1.App{Status: appv1alpha1.AppStatus{
			ReleaseGeneration: 4, ActiveRevision: "rev-3",
			ReleaseRollout: &appv1alpha1.ReleaseRolloutStatus{Generation: 4},
		}}
	}
	dep := func(rev string) *appsv1.Deployment {
		return &appsv1.Deployment{Spec: appsv1.DeploymentSpec{Template: rolloutTemplate(rev)}}
	}
	if !releaseRolloutRunning(rolling(), dep("rev-4")) {
		t.Fatal("a release rolling out over the served one must be running")
	}
	for _, tc := range []struct {
		name   string
		mutate func(*appv1alpha1.App)
		rev    string
	}{
		{"no record", func(a *appv1alpha1.App) { a.Status.ReleaseRollout = nil }, "rev-4"},
		{"an older release's record", func(a *appv1alpha1.App) { a.Status.ReleaseRollout.Generation = 3 }, "rev-4"},
		{"the served template restored", func(*appv1alpha1.App) {}, "rev-3"},
		{"the release serves", func(a *appv1alpha1.App) { a.Status.ActiveRevision = "rev-4" }, "rev-4"},
		{"the rollout failed with an exit", func(a *appv1alpha1.App) {
			a.Status.RolloutFailure = &appv1alpha1.RolloutFailureStatus{Generation: 4}
		}, "rev-4"},
		{"the rollout's verdict is recorded", func(a *appv1alpha1.App) {
			a.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, ObservedGeneration: 4}}
		}, "rev-4"},
		{"canceled over the served release", func(a *appv1alpha1.App) {
			a.Annotations = map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "4"}
			a.Generation = 4
		}, "rev-4"},
	} {
		app := rolling()
		tc.mutate(app)
		if releaseRolloutRunning(app, dep(tc.rev)) {
			t.Errorf("%s: rollout still running", tc.name)
		}
	}
	app := rolling()
	app.Status.RolloutFailure = &appv1alpha1.RolloutFailureStatus{Generation: 3}
	app.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, ObservedGeneration: 3}}
	if !releaseRolloutRunning(app, dep("rev-4")) {
		t.Error("an earlier release's failure must not end this rollout")
	}
}

var _ = Describe("The operator records each release's rollout (w32/m560)", func() {
	ctx := context.Background()
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
				_ = k8sClient.Delete(ctx, &batchv1.Job{ObjectMeta: metav1.ObjectMeta{Name: build.JobName(name, rev), Namespace: nn.Namespace}},
					&client.DeleteOptions{PropagationPolicy: &background})
			}
		})
		reconcileToFixedPoint(run)
		return nn, run
	}
	rollout := func(nn types.NamespacedName) *appv1alpha1.ReleaseRolloutStatus {
		GinkgoHelper()
		return envtestApp(nn).Status.ReleaseRollout
	}
	// appPodOf stands in for the kubelet: a pod of the Deployment's current
	// template whose app container is in state after last.
	appPodOf := func(nn types.NamespacedName, name string, ready bool, restarts int32, state, last corev1.ContainerState) {
		GinkgoHelper()
		pod := &corev1.Pod{}
		err := k8sClient.Get(ctx, types.NamespacedName{Name: name, Namespace: nn.Namespace}, pod)
		if err != nil {
			pod = &corev1.Pod{
				ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: nn.Namespace, Labels: envtestDeployment(nn).Spec.Template.Labels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: appContainerName, Image: "traefik/whoami"}}},
			}
			Expect(k8sClient.Create(ctx, pod)).To(Succeed())
			DeferCleanup(func() { _ = k8sClient.Delete(ctx, pod, client.GracePeriodSeconds(0)) })
		}
		readiness := corev1.ConditionFalse
		if ready {
			readiness = corev1.ConditionTrue
		}
		pod.Status = corev1.PodStatus{
			Phase:      corev1.PodRunning,
			Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: readiness}},
			ContainerStatuses: []corev1.ContainerStatus{{
				Name: appContainerName, Ready: ready, RestartCount: restarts, State: state, LastTerminationState: last,
			}},
		}
		Expect(k8sClient.Status().Update(ctx, pod)).To(Succeed())
	}
	// starts are the recorded app starts in Unix seconds, the kubelet's
	// precision.
	starts := func(nn types.NamespacedName) []int64 {
		GinkgoHelper()
		recorded := rollout(nn).AppStarts
		out := make([]int64, 0, len(recorded))
		for _, s := range recorded {
			out = append(out, s.Unix())
		}
		return out
	}

	It("stamps an image web service's rollout once, adds its app container's starts until it serves, and replaces the record for a newer release", func() {
		nn, run := create("rollout-web", appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Tier: "pro"})
		app := envtestApp(nn)
		rec := app.Status.ReleaseRollout
		Expect(rec).NotTo(BeNil(), "the first template write starts the record")
		Expect(rec.Generation).To(Equal(releaseGeneration(app)))
		Expect(rec.StartedAt.IsZero()).To(BeFalse())
		Expect(rec.Command).To(BeEmpty(), "an image runs its own program")
		Expect(rec.WebConcurrency).To(Equal(int32(2)), "a new pro web service gets its plan's CPU count")
		Expect(rec.AppStarts).To(BeEmpty(), "no instance has started")
		first := rec.Generation
		started := rec.StartedAt

		By("a resync keeps the start")
		reconcileToFixedPoint(run)
		Expect(rollout(nn).StartedAt.Equal(&started)).To(BeTrue(), "the start moved to %v", rollout(nn).StartedAt)

		By("each start of the app container while the rollout runs")
		t0 := time.Now().Add(-2 * time.Minute).Truncate(time.Second)
		appPodOf(nn, "rollout-web-a", false, 0, runningSince(t0), corev1.ContainerState{})
		reconcileToFixedPoint(run)
		Expect(starts(nn)).To(Equal([]int64{t0.Unix()}))
		restarted := t0.Add(20 * time.Second)
		appPodOf(nn, "rollout-web-a", true, 1, runningSince(restarted), exitedRun(t0, t0.Add(4*time.Second)))
		reconcileToFixedPoint(run)
		Expect(starts(nn)).To(Equal([]int64{t0.Unix(), restarted.Unix()}), "the restart adds its start")

		By("once the release serves, a restart is the service's own")
		serveEnvtestRelease(nn, func() { reconcileToFixedPoint(run) })
		appPodOf(nn, "rollout-web-a", true, 2, runningSince(restarted.Add(time.Minute)), exitedRun(restarted, restarted.Add(50*time.Second)))
		reconcileToFixedPoint(run)
		Expect(starts(nn)).To(Equal([]int64{t0.Unix(), restarted.Unix()}))
		Expect(rollout(nn).StartedAt.Equal(&started)).To(BeTrue())

		By("a newer release replaces the record")
		app = envtestApp(nn)
		app.Spec.Tier = "pro-ultra"
		metav1.SetMetaDataAnnotation(&app.ObjectMeta, appv1alpha1.AnnotationReleaseGeneration, strconv.FormatInt(app.Generation+1, 10))
		Expect(k8sClient.Update(ctx, app)).To(Succeed())
		reconcileToFixedPoint(run)
		app = envtestApp(nn)
		rec = app.Status.ReleaseRollout
		Expect(rec.Generation).To(Equal(releaseGeneration(app)))
		Expect(rec.Generation).To(BeNumerically(">", first))
		Expect(rec.StartedAt.After(started.Time)).To(BeTrue())
		Expect(rec.WebConcurrency).To(Equal(int32(8)), "the new plan's default")
		Expect(rec.AppStarts).To(BeEmpty(), "the served release's pod is not the new release's")
	})

	It("records no WEB_CONCURRENCY default for a worker or a service that sets its own", func() {
		worker, _ := create("rollout-worker", appv1alpha1.AppSpec{Image: "traefik/whoami", Tier: "pro", Type: appv1alpha1.TypeBackgroundWorker})
		own, _ := create("rollout-own-wc", appv1alpha1.AppSpec{Image: "traefik/whoami", Port: 3000, Tier: "pro",
			Env: []appv1alpha1.EnvVar{{Name: webConcurrencyEnvName, Value: "4"}}})
		for _, nn := range []types.NamespacedName{worker, own} {
			rec := rollout(nn)
			Expect(rec).NotTo(BeNil(), "%s rolls out an instance", nn.Name)
			Expect(rec.WebConcurrency).To(BeZero(), "%s gets no default", nn.Name)
		}
	})

	It("records the start command only for a release whose native image launches it", func() {
		native, nativeRun := create("rollout-native", appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 3000, Replicas: 1,
			Runtime: "node", Builder: build.BuilderNative, BuildCommand: "npm ci", StartCommand: "node main.js --port $PORT",
		})
		docker, dockerRun := create("rollout-docker", appv1alpha1.AppSpec{
			Repo: "https://github.com/bex-co/hello", Branch: "main", Port: 3000, Replicas: 1,
			Builder: "dockerfile", StartCommand: "./server gateway",
		})
		Expect(rollout(native)).To(BeNil(), "nothing rolls out while it builds")
		completeBuildJob(native.Name, "gen-1")
		reconcileToFixedPoint(nativeRun)
		completeBuildJob(docker.Name, "gen-1")
		reconcileToFixedPoint(dockerRun)

		nativeApp := envtestDeployment(native).Spec.Template.Spec.Containers[0]
		Expect(nativeApp.Command).To(BeEmpty(), "the native image's entrypoint, native-init, runs its CMD")
		Expect(rollout(native)).NotTo(BeNil())
		Expect(rollout(native).Command).To(Equal("node main.js --port $PORT"), "quoted as configured")
		Expect(envtestDeployment(docker).Spec.Template.Spec.Containers[0].Command).NotTo(BeEmpty(), "the Docker Command replaces the image's")
		Expect(rollout(docker)).NotTo(BeNil())
		Expect(rollout(docker).Command).To(BeEmpty(), "Render names no Docker Command")
	})

	It("records nothing for a cron job, which runs no instance at deploy", func() {
		nn, _ := create("rollout-cron", appv1alpha1.AppSpec{
			Type: appv1alpha1.TypeCronJob, Image: "traefik/whoami", Schedule: "0 * * * *", Command: "/whoami --version",
		})
		cj := &batchv1.CronJob{}
		Expect(k8sClient.Get(ctx, types.NamespacedName{Name: appv1alpha1.CronJobName(nn.Name), Namespace: nn.Namespace}, cj)).To(Succeed())
		Expect(rollout(nn)).To(BeNil())
	})
})
