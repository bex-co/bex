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
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	"github.com/bex-co/bex/lego/operator/internal/build"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// ownBuildJob is the build Job the App dispatched for its release.
func ownBuildJob(t *testing.T, cl client.Client, nn types.NamespacedName) *batchv1.Job {
	t.Helper()
	app := liveApp(t, cl, nn)
	job := &batchv1.Job{}
	key := client.ObjectKey{Namespace: nn.Namespace, Name: build.JobName(nn.Name, releaseBuildRevision(&app))}
	if err := cl.Get(context.Background(), key, job); err != nil {
		t.Fatalf("setup: the release's build was never dispatched: %v", err)
	}
	return job
}

// writeBuildPod writes the build pod of job with status, as the kubelet does.
func writeBuildPod(t *testing.T, cl client.Client, job *batchv1.Job, status corev1.PodStatus) {
	t.Helper()
	ctx := context.Background()
	pod := &corev1.Pod{}
	key := client.ObjectKey{Namespace: job.Namespace, Name: job.Name + "-a"}
	if err := cl.Get(ctx, key, pod); err != nil {
		pod = &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: key.Name, Namespace: key.Namespace, Labels: map[string]string{"job-name": job.Name}},
			Spec:       *job.Spec.Template.Spec.DeepCopy(),
		}
		pod.Spec.NodeName = "node-a"
		if err := cl.Create(ctx, pod); err != nil {
			t.Fatal(err)
		}
	}
	pod.Status = status
	if err := cl.Status().Update(ctx, pod); err != nil {
		t.Fatal(err)
	}
}

func phaseTerminated(name string, exit int32, reason string, start, end time.Time) corev1.ContainerStatus {
	return corev1.ContainerStatus{Name: name, State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{
		ExitCode: exit, Reason: reason, StartedAt: metav1.NewTime(start), FinishedAt: metav1.NewTime(end),
	}}}
}

// TestABuildRollsOutAtItsImagePushWhileItsCacheSaveRuns (w33/m569): the
// release starts at the push's `Build successful`, its recorded window ends
// there, and a cache-save that then fails changes neither the verdict nor the
// rollout.
func TestABuildRollsOutAtItsImagePushWhileItsCacheSaveRuns(t *testing.T) {
	app := activeApp("tea-push-verdict")
	app.UID = "uid-web"
	r, cl, nn := lifecycleFixture(t, app)
	prior := buildingReleaseTwo(t, r, cl, nn)
	job := ownBuildJob(t, cl, nn)
	started := time.Now().Add(-3 * time.Minute).Truncate(time.Second)
	pushStart, pushed := started.Add(90*time.Second), started.Add(135*time.Second)
	job.Status.StartTime = &metav1.Time{Time: started}
	if err := cl.Status().Update(context.Background(), job); err != nil {
		t.Fatal(err)
	}
	buildkit := phaseTerminated("buildkit", 0, "Completed", started, pushStart)
	push := phaseTerminated("push", 0, "Completed", pushStart, pushed)
	writeBuildPod(t, cl, job, corev1.PodStatus{
		Phase:                 corev1.PodRunning,
		InitContainerStatuses: []corev1.ContainerStatus{buildkit},
		ContainerStatuses: []corev1.ContainerStatus{push, {Name: "cache-save", State: corev1.ContainerState{
			Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(pushStart)},
		}}},
	})

	reconcileOnce(t, r, nn)
	live := liveApp(t, cl, nn)
	want := appv1alpha1.BuildRunStatus{Generation: 2, StartedAt: started.UTC().Format(time.RFC3339), FinishedAt: pushed.UTC().Format(time.RFC3339)}
	if live.Status.BuildRun == nil || *live.Status.BuildRun != want {
		t.Fatalf("buildRun = %+v, want %+v: bex-api dates build_ended from it", live.Status.BuildRun, want)
	}
	built := live.Status.ArtifactImage
	if got := liveDeployment(t, cl, nn).Spec.Template.Spec.Containers[0].Image; built == "" || built == prior || got != built {
		t.Fatalf("the pod template runs %q, want release 2's built image %q while cache-save still runs", got, built)
	}

	// cache-save is OOM-killed and buildPodFailurePolicy fails the Job.
	writeBuildPod(t, cl, job, corev1.PodStatus{
		Phase:                 corev1.PodFailed,
		InitContainerStatuses: []corev1.ContainerStatus{buildkit},
		ContainerStatuses:     []corev1.ContainerStatus{push, phaseTerminated("cache-save", 137, "OOMKilled", pushStart, pushed.Add(time.Minute))},
	})
	failOwnBuild(t, cl, nn)
	reconcileOnce(t, r, nn)
	live = liveApp(t, cl, nn)
	if verdict := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionBuild); verdict != nil && appv1alpha1.IsBuildFailureReason(verdict.Reason) {
		t.Fatalf("Build condition = %+v: a failed cache-save failed a build whose image was pushed", verdict)
	}
	if got := liveDeployment(t, cl, nn).Spec.Template.Spec.Containers[0].Image; got != built || live.Status.ArtifactImage != built {
		t.Fatalf("after the cache-save failure the release runs %q (artifact %q), want %q unchanged", got, live.Status.ArtifactImage, built)
	}
	if live.Status.BuildRun == nil || *live.Status.BuildRun != want {
		t.Fatalf("buildRun = %+v, want %+v unchanged", live.Status.BuildRun, want)
	}

	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	if live = liveApp(t, cl, nn); live.Status.Phase != appv1alpha1.PhaseRunning || live.Status.ActiveRevision != "rev-2" {
		t.Fatalf("phase %q, active %q; want release 2 live", live.Status.Phase, live.Status.ActiveRevision)
	}
}

// TestAFailedPushStillFailsTheBuild: whatever cache-save does, a push that
// failed leaves no image, and the release's build fails.
func TestAFailedPushStillFailsTheBuild(t *testing.T) {
	app := activeApp("tea-push-failed")
	app.UID = "uid-web"
	r, cl, nn := lifecycleFixture(t, app)
	buildingReleaseTwo(t, r, cl, nn)
	job := ownBuildJob(t, cl, nn)
	started := time.Now().Add(-3 * time.Minute).Truncate(time.Second)
	writeBuildPod(t, cl, job, corev1.PodStatus{
		Phase:                 corev1.PodFailed,
		InitContainerStatuses: []corev1.ContainerStatus{phaseTerminated("buildkit", 0, "Completed", started, started.Add(90*time.Second))},
		ContainerStatuses: []corev1.ContainerStatus{
			phaseTerminated("push", build.ExitTenantError, "Error", started.Add(90*time.Second), started.Add(100*time.Second)),
			phaseTerminated("cache-save", 0, "Completed", started.Add(90*time.Second), started.Add(150*time.Second)),
		},
	})
	failOwnBuild(t, cl, nn)
	_, _ = r.Reconcile(context.Background(), reconcile.Request{NamespacedName: nn}) // a failed build answers its error
	live := liveApp(t, cl, nn)
	verdict := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionBuild)
	if verdict == nil || !appv1alpha1.IsBuildFailureReason(verdict.Reason) || verdict.ObservedGeneration != 2 {
		t.Fatalf("Build condition = %+v, want release 2's build failed", verdict)
	}
}

// TestTheBuildCensusMetersACacheSaveThatOutlivedItsBuild: a build metered at
// its push leaves its cache-save's outcome to the census collector, which
// records it once the Job has finished, and once. The store-outage alert
// (BuildCacheStoreErrors) reads that series.
func TestTheBuildCensusMetersACacheSaveThatOutlivedItsBuild(t *testing.T) {
	ctx := context.Background()
	job := build.BuildJob(build.Options{
		Name: "web", Namespace: censusBuildNS, Revision: "gen-2", Repo: "https://example.invalid/repo.git",
		Registry: "zot.test:5000", AppUID: "uid-web", Workspace: "tea-a", BuildCache: true,
	}, "zot.test:5000/web:gen-2")
	cl := fake.NewClientBuilder().WithScheme(wakeScheme()).WithObjects(job).WithStatusSubresource(&batchv1.Job{}).Build()
	if err := build.StampMetering(ctx, cl, censusBuildNS, job.Name, build.Signals{CacheSavePending: true}, build.FaultNone); err != nil {
		t.Fatal(err)
	}
	start := time.Now().Add(-time.Minute)
	save := phaseTerminated("cache-save", 0, "Completed", start, start.Add(40*time.Second))
	save.State.Terminated.Message = build.CacheError + "\n"
	writeBuildPod(t, cl, job, corev1.PodStatus{Phase: corev1.PodSucceeded, ContainerStatuses: []corev1.ContainerStatus{
		phaseTerminated("push", 0, "Completed", start, start.Add(10*time.Second)), save,
	}})
	saveError := buildCachePhaseSeconds.WithLabelValues("cache-save", build.CacheKindLayer, build.CacheError)
	before := histogramCount(t, saveError)
	collector := &BuildCensusCollector{Client: cl, Namespace: censusBuildNS}

	collector.collect(ctx) // the Job still runs: nothing to count yet
	if got := histogramCount(t, saveError) - before; got != 0 {
		t.Fatalf("counted %d cache-save outcomes before the Job finished", got)
	}
	finished := &batchv1.Job{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(job), finished); err != nil {
		t.Fatal(err)
	}
	finished.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := cl.Status().Update(ctx, finished); err != nil {
		t.Fatal(err)
	}
	collector.collect(ctx)
	collector.collect(ctx)
	if got := histogramCount(t, saveError) - before; got != 1 {
		t.Fatalf("counted the cache-save's outcome %d times, want once", got)
	}
}
