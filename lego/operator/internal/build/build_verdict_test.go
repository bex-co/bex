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

package build

import (
	"context"
	"fmt"
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// w33/m569: a build's verdict is its image push, not its Job's completion.
// cache-save runs beside the push and the Job reads Complete only once it
// exits, which on eden-dash-v3 held the release 70 s past `Build successful`.

// verdictJob is a dispatched build Job that started at the signals epoch and
// has reached no verdict of its own.
func verdictJob(o Options) *batchv1.Job {
	j := BuildJob(o, o.ImageRef())
	j.Status.StartTime = new(at(0))
	return j
}

// verdictPod is j's first attempt: its template's spec, placed on a node.
func verdictPod(j *batchv1.Job, opts ...podOpt) *corev1.Pod {
	pod := attempt("a", 0, opts...)
	pod.Namespace = j.Namespace
	pod.Labels = map[string]string{jobNameLabel: j.Name}
	pod.Spec = *j.Spec.Template.Spec.DeepCopy()
	pod.Spec.NodeName = "node-a"
	return &pod
}

func mainRunning(name string, start time.Duration) podOpt {
	return func(p *corev1.Pod) {
		p.Status.ContainerStatuses = append(p.Status.ContainerStatuses, corev1.ContainerStatus{
			Name: name, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: at(start)}},
		})
	}
}

func podPhase(phase corev1.PodPhase) podOpt {
	return func(p *corev1.Pod) { p.Status.Phase = phase }
}

// failedJob marks j Failed with reason, as the Job controller writes it.
func failedJob(j *batchv1.Job, reason string, when time.Duration) *batchv1.Job {
	j.Status.Conditions = []batchv1.JobCondition{
		{Type: batchv1.JobFailureTarget, Status: corev1.ConditionTrue, Reason: reason},
		{Type: batchv1.JobFailed, Status: corev1.ConditionTrue, Reason: reason, LastTransitionTime: at(when)},
	}
	return j
}

func observe(t *testing.T, o Options, objs ...client.Object) Observation {
	t.Helper()
	o.Client = fakeClient(objs...)
	obs, err := EnsureBuild(context.Background(), o)
	if err != nil {
		t.Fatalf("EnsureBuild: %v", err)
	}
	return obs
}

func TestABuildSucceedsAtItsImagePushWhileCacheSaveRuns(t *testing.T) {
	o := cacheOpts()
	j := verdictJob(o)
	building := initTerminated("buildkit", 0, "", 10*time.Second, 90*time.Second)

	t.Run("push still running is building", func(t *testing.T) {
		obs := observe(t, o, j, verdictPod(j, building, mainRunning(pushContainer, 90*time.Second), mainRunning(cacheSavePhase, 90*time.Second)))
		if obs.Phase != PhaseBuilding {
			t.Fatalf("phase = %v, want building while the image is not in the registry", obs.Phase)
		}
	})

	t.Run("push done, cache-save running is succeeded at the push", func(t *testing.T) {
		obs := observe(t, o, j, verdictPod(j, building,
			mainTerminated(pushContainer, 0, "Completed", 90*time.Second, 135*time.Second), mainRunning(cacheSavePhase, 90*time.Second)))
		if obs.Phase != PhaseSucceeded || obs.Image != o.ImageRef() {
			t.Fatalf("observation = %+v, want succeeded with the release's image once it is pushed", obs)
		}
		if !obs.FinishedAt.Equal(at(135*time.Second).Time) || !obs.StartedAt.Equal(at(0).Time) || obs.RunSeconds != 135 {
			t.Errorf("window = %v → %v (%vs), want the Job's start → the push's end (135s)", obs.StartedAt, obs.FinishedAt, obs.RunSeconds)
		}
	})

	// A disruption after the push is retried by the Job (DisruptionTarget is
	// Ignored), and the retry pushes the release's tag again. The build
	// succeeded the first time its image was in the registry.
	t.Run("the first pushed attempt dates the success", func(t *testing.T) {
		first := verdictPod(j, building, disrupted(), podPhase(corev1.PodFailed),
			mainTerminated(pushContainer, 0, "Completed", 90*time.Second, 135*time.Second),
			mainTerminated(cacheSavePhase, 143, "Error", 90*time.Second, 140*time.Second))
		retry := attempt("b", 3*time.Minute, initTerminated("buildkit", 0, "", 190*time.Second, 280*time.Second),
			mainTerminated(pushContainer, 0, "Completed", 280*time.Second, 300*time.Second), mainRunning(cacheSavePhase, 280*time.Second))
		retry.Namespace, retry.Labels = j.Namespace, map[string]string{jobNameLabel: j.Name}
		obs := observe(t, o, j, first, &retry)
		if obs.Phase != PhaseSucceeded || !obs.FinishedAt.Equal(at(135*time.Second).Time) {
			t.Fatalf("observation = %+v, want succeeded at the first push (135s)", obs)
		}
	})
}

// With image signing on, the push is an init container and the signature the
// image's last phase (w6/006): the build is done once the image is signed.
func TestASignedBuildSucceedsAtItsSignature(t *testing.T) {
	o := cacheOpts()
	o.SignKeySecret = "signing"
	j := verdictJob(o)
	pushed := initTerminated(pushContainer, 0, "Completed", 90*time.Second, 135*time.Second)

	obs := observe(t, o, j, verdictPod(j, pushed, mainRunning(signContainer, 135*time.Second), mainRunning(cacheSavePhase, 135*time.Second)))
	if obs.Phase != PhaseBuilding {
		t.Fatalf("phase = %v, want building while the pushed image is not signed yet", obs.Phase)
	}
	obs = observe(t, o, j, verdictPod(j, pushed,
		mainTerminated(signContainer, 0, "Completed", 135*time.Second, 140*time.Second), mainRunning(cacheSavePhase, 135*time.Second)))
	if obs.Phase != PhaseSucceeded || !obs.FinishedAt.Equal(at(140*time.Second).Time) {
		t.Fatalf("observation = %+v, want succeeded at the signature's end (140s)", obs)
	}
}

// A Job whose pods are gone (reaped with a node, say) still answers from its
// own verdict and window.
func TestACompletedBuildWithNoPodLeftSucceedsFromItsJob(t *testing.T) {
	o := cacheOpts()
	j := verdictJob(o)
	j.Status.CompletionTime = new(at(200 * time.Second))
	j.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	obs := observe(t, o, j)
	if obs.Phase != PhaseSucceeded || !obs.FinishedAt.Equal(at(200*time.Second).Time) || obs.RunSeconds != 200 {
		t.Fatalf("observation = %+v, want succeeded over the Job's own window", obs)
	}
}

// A cache-save that fails after the push (the kernel's OOM kill, or any exit
// its script did not swallow) fails the pod, and buildPodFailurePolicy fails
// the Job at once. The build still succeeded: its image is in the registry.
func TestACacheSaveFailureAfterThePushLeavesTheBuildSucceeded(t *testing.T) {
	o := cacheOpts()
	for name, cacheSave := range map[string]podOpt{
		"oom killed": mainTerminated(cacheSavePhase, 137, "OOMKilled", 90*time.Second, 160*time.Second),
		"exit 1":     mainTerminated(cacheSavePhase, 1, "Error", 90*time.Second, 160*time.Second),
	} {
		t.Run(name, func(t *testing.T) {
			j := failedJob(verdictJob(o), batchv1.JobReasonPodFailurePolicy, 161*time.Second)
			obs := observe(t, o, j, verdictPod(j, podPhase(corev1.PodFailed),
				initTerminated("buildkit", 0, "", 10*time.Second, 90*time.Second),
				mainTerminated(pushContainer, 0, "Completed", 90*time.Second, 135*time.Second), cacheSave))
			if obs.Phase != PhaseSucceeded || !obs.FinishedAt.Equal(at(135*time.Second).Time) {
				t.Fatalf("observation = %+v, want succeeded at the push: the cache changes only speed", obs)
			}
		})
	}
}

// A push that fails fails the build whatever cache-save does beside it, and
// the failure names the push, never the cache.
func TestAFailedPushFailsTheBuildWhateverCacheSaveDoes(t *testing.T) {
	o := cacheOpts()
	for _, tc := range []struct {
		name      string
		reason    string
		pushExit  int32
		cacheSave podOpt
		fault     Fault
	}{
		{"transient push, retries spent", batchv1.JobReasonBackoffLimitExceeded, ExitTransient,
			mainTerminated(cacheSavePhase, 0, "Completed", 90*time.Second, 150*time.Second), FaultInfra},
		// cache-save's own FailJob rule ended the Job, so the reason reads
		// PodFailurePolicy without any tenant-classified exit: the platform's.
		{"transient push, cache-save killed", batchv1.JobReasonPodFailurePolicy, ExitTransient,
			mainTerminated(cacheSavePhase, 137, "OOMKilled", 90*time.Second, 150*time.Second), FaultInfra},
		{"tenant push, cache-save killed", batchv1.JobReasonPodFailurePolicy, ExitTenantError,
			mainTerminated(cacheSavePhase, 137, "OOMKilled", 90*time.Second, 150*time.Second), FaultTenant},
	} {
		t.Run(tc.name, func(t *testing.T) {
			j := failedJob(verdictJob(o), tc.reason, 151*time.Second)
			obs := observe(t, o, j, verdictPod(j, podPhase(corev1.PodFailed),
				initTerminated("buildkit", 0, "", 10*time.Second, 90*time.Second),
				mainTerminated(pushContainer, tc.pushExit, "Error", 90*time.Second, 120*time.Second), tc.cacheSave))
			if obs.Phase != PhaseFailed {
				t.Fatalf("phase = %v, want failed: the image never reached the registry", obs.Phase)
			}
			if obs.FailedStep != buildStepNames[pushContainer] {
				t.Errorf("failed step = %q, want the push's %q", obs.FailedStep, buildStepNames[pushContainer])
			}
			if obs.Fault != tc.fault {
				t.Errorf("fault = %q, want %q", obs.Fault, tc.fault)
			}
		})
	}
}

// Only a failed cache-save moves a policy failure off the tenant: a pod whose
// exits cannot be read leaves the Job's own reason in charge.
func TestAPolicyFailureWithNoExitToReadStaysTheTenants(t *testing.T) {
	o := cacheOpts()
	j := failedJob(verdictJob(o), batchv1.JobReasonPodFailurePolicy, 151*time.Second)
	if obs := observe(t, o, j, verdictPod(j, podPhase(corev1.PodFailed))); obs.Phase != PhaseFailed || obs.Fault != FaultTenant {
		t.Fatalf("observation = %+v, want a tenant failure", obs)
	}
}

// The cache-save rule fails the Job at once rather than letting it retry a
// build whose push already settled it, and rides only with the phase: the API
// server refuses a rule naming a container the pod does not have (the
// envtest round trip submits both shapes).
func TestTheFailurePolicyFailsTheJobOnAFailedCacheSave(t *testing.T) {
	isCacheSaveRule := func(r batchv1.PodFailurePolicyRule) bool {
		return r.OnExitCodes != nil && r.OnExitCodes.ContainerName != nil && *r.OnExitCodes.ContainerName == cacheSavePhase
	}
	rules := BuildJob(cacheOpts(), "img:1").Spec.PodFailurePolicy.Rules
	found := -1
	for i, r := range rules {
		if isCacheSaveRule(r) {
			found = i
		}
	}
	if found < 0 {
		t.Fatal("a build with cache-save has no rule for it: a cache-save OOM retries the whole build and re-pushes the release's tag")
	}
	r := rules[found]
	if r.Action != batchv1.PodFailurePolicyActionFailJob || r.OnExitCodes.Operator != batchv1.PodFailurePolicyOnExitCodesOpNotIn ||
		len(r.OnExitCodes.Values) != 1 || r.OnExitCodes.Values[0] != 0 {
		t.Errorf("cache-save rule = %+v, want FailJob on any non-zero exit", r)
	}
	// After the disruption rule, so an eviction is still retried for free, and
	// after the tenant rule, so a tenant failure keeps its own index.
	if found != len(rules)-1 {
		t.Errorf("cache-save rule at %d of %d, want it last", found, len(rules))
	}
	for _, r := range BuildJob(opts(), "img:1").Spec.PodFailurePolicy.Rules {
		if r.OnExitCodes != nil && r.OnExitCodes.ContainerName != nil {
			t.Errorf("a build without a cache names container %q in its failure policy", *r.OnExitCodes.ContainerName)
		}
	}
}

// A build metered at its push leaves its cache-save's outcome to the census
// collector, which counts it once the Job has finished, and only once.
func TestACacheSaveStillRunningAtTheVerdictIsMeteredOnceItEnds(t *testing.T) {
	o := s3CacheOpts()
	o.Namespace = censusNS
	j := verdictJob(o)
	running := verdictPod(j, mainTerminated(pushContainer, 0, "Completed", 90*time.Second, 135*time.Second),
		mainRunning(cacheSavePhase, 90*time.Second))
	if sig := signalsFrom(j, []corev1.Pod{*running}); !sig.CacheSavePending || len(sig.CachePhases) != 0 {
		t.Fatalf("signals = %+v, want the running cache-save pending and unmetered", sig)
	}
	cl := fakeClient(j, running)
	ctx := context.Background()
	if err := StampMetering(ctx, cl, censusNS, j.Name, Signals{CacheSavePending: true}, FaultNone); err != nil {
		t.Fatal(err)
	}
	if saves, err := PendingCacheSaves(ctx, cl, censusNS); err != nil || len(saves) != 0 {
		t.Fatalf("while the Job runs: saves %v, err %v; want none yet", saves, err)
	}
	if got := jobLabels(t, cl, j.Name); got[cacheSavePendingLabel] != "true" {
		t.Fatalf("labels = %v, want the running Job still marked", got)
	}

	// cache-save ends and the Job completes.
	done := verdictPod(j, podPhase(corev1.PodSucceeded), mainTerminated(pushContainer, 0, "Completed", 90*time.Second, 135*time.Second),
		mainTerminated(cacheSavePhase, 0, "Completed", 90*time.Second, 160*time.Second))
	done.Status.ContainerStatuses[1].State.Terminated.Message = fmt.Sprintf("%s 1234\n", CacheSaved)
	stored := &corev1.Pod{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(done), stored); err != nil {
		t.Fatal(err)
	}
	stored.Status = done.Status
	if err := cl.Status().Update(ctx, stored); err != nil {
		t.Fatal(err)
	}
	finished := &batchv1.Job{}
	if err := cl.Get(ctx, client.ObjectKeyFromObject(j), finished); err != nil {
		t.Fatal(err)
	}
	finished.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	if err := cl.Status().Update(ctx, finished); err != nil {
		t.Fatal(err)
	}
	saves, err := PendingCacheSaves(ctx, cl, censusNS)
	if err != nil || len(saves) != 1 || saves[0].Phase != cacheSavePhase || saves[0].Outcome != CacheSaved || saves[0].Bytes != 1234 || saves[0].Seconds != 70 {
		t.Fatalf("saves %+v, err %v; want the one save, 1234 bytes in 70s", saves, err)
	}
	if got := jobLabels(t, cl, j.Name); got[cacheSavePendingLabel] != "" {
		t.Errorf("labels = %v, want the mark cleared", got)
	}
	if saves, err := PendingCacheSaves(ctx, cl, censusNS); err != nil || len(saves) != 0 {
		t.Errorf("second pass: saves %v, err %v; want the save counted once", saves, err)
	}
}

// A cache-save that already ended, or never ran, is not pending.
func TestOnlyARunningCacheSaveIsPending(t *testing.T) {
	o := cacheOpts()
	j := verdictJob(o)
	for name, pod := range map[string]*corev1.Pod{
		"ended": verdictPod(j, mainTerminated(pushContainer, 0, "Completed", 90*time.Second, 135*time.Second),
			mainTerminated(cacheSavePhase, 0, "Completed", 90*time.Second, 100*time.Second)),
		"pod over before it started": verdictPod(j, podPhase(corev1.PodFailed), initTerminated("buildkit", ExitTenantError, "Error", 10*time.Second, 20*time.Second)),
		"no cache-save in the build": verdictPod(BuildJob(opts(), "img:1"), mainRunning(pushContainer, 90*time.Second)),
	} {
		if signalsFrom(j, []corev1.Pod{*pod}).CacheSavePending {
			t.Errorf("%s: reported pending", name)
		}
	}
}

func jobLabels(t *testing.T, cl client.Client, name string) map[string]string {
	t.Helper()
	var job batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: censusNS, Name: name}, &job); err != nil {
		t.Fatal(err)
	}
	return job.Labels
}
