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
	"fmt"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// instance_exits_test.go is w8/m158 t001. A running service's process that
// exits or is killed for memory restarts in place within seconds. Nothing
// recorded why: the event, when there was one, said a health check failed.

func exitedPod(name, revision string, restarts int32, t *corev1.ContainerStateTerminated) corev1.Pod {
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{labelRevision: revision}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
			Name: appContainerName, RestartCount: restarts,
			LastTerminationState: corev1.ContainerState{Terminated: t},
		}}},
	}
}

func terminated(code int32, reason string, at time.Time) *corev1.ContainerStateTerminated {
	return &corev1.ContainerStateTerminated{ExitCode: code, Reason: reason, FinishedAt: metav1.NewTime(at)}
}

func TestInstanceExitsRecordTheCauseOfEachRestart(t *testing.T) {
	at := time.Date(2026, 11, 11, 13, 5, 14, 0, time.UTC)
	settled := at.Add(-time.Hour) // the release has been serving for an hour
	pods := []corev1.Pod{
		exitedPod("web-a", "rev-9", 1, terminated(3, "Error", at)),
		exitedPod("web-b", "rev-9", 2, terminated(137, "OOMKilled", at.Add(90*time.Second))),
		exitedPod("web-c", "rev-9", 1, terminated(0, "Completed", at.Add(3*time.Minute))),
	}
	got := withInstanceExits(nil, pods, "rev-9", settled, at.Add(4*time.Minute))
	want := []appv1alpha1.InstanceExit{
		{Instance: "web-c", RestartCount: 1, ExitCode: 0, FinishedAt: metav1.NewTime(at.Add(3 * time.Minute))},
		{Instance: "web-b", RestartCount: 2, ExitCode: 137, OOMKilled: true, FinishedAt: metav1.NewTime(at.Add(90 * time.Second))},
		{Instance: "web-a", RestartCount: 1, ExitCode: 3, FinishedAt: metav1.NewTime(at)},
	}
	if len(got) != len(want) {
		t.Fatalf("exits = %+v, want %+v", got, want)
	}
	for i := range want {
		if got[i].Instance != want[i].Instance || got[i].RestartCount != want[i].RestartCount || got[i].ExitCode != want[i].ExitCode ||
			got[i].OOMKilled != want[i].OOMKilled || !got[i].FinishedAt.Equal(&want[i].FinishedAt) {
			t.Errorf("exit %d = %+v, want %+v (newest first)", i, got[i], want[i])
		}
	}

	// The same pods on a later pass add nothing and leave the slice alone, so
	// an unchanged status is not rewritten.
	again := withInstanceExits(got, pods, "rev-9", settled, at.Add(6*time.Minute))
	if len(again) != len(got) || &again[0] != &got[0] {
		t.Errorf("an unchanged pass rebuilt the list: %+v", again)
	}
	// The same instance crashing again is a new exit.
	pods[0] = exitedPod("web-a", "rev-9", 2, terminated(7, "Error", at.Add(5*time.Minute)))
	next := withInstanceExits(got, pods, "rev-9", settled, at.Add(6*time.Minute))
	if len(next) != 4 || next[0].Instance != "web-a" || next[0].RestartCount != 2 || next[0].ExitCode != 7 {
		t.Errorf("a second crash of web-a = %+v, want it first of four", next)
	}
}

func TestInstanceExitsLeaveOutWhatIsNotTheApplicationExiting(t *testing.T) {
	at := time.Date(2026, 11, 11, 13, 5, 14, 0, time.UTC)
	settled := at.Add(-30 * time.Second) // the rollout completed half a minute before
	for name, pod := range map[string]corev1.Pod{
		// The kubelet stopping a container (a failed liveness check, a drain).
		"killed with SIGKILL":  exitedPod("web-a", "rev-9", 1, terminated(137, "Error", at)),
		"stopped with SIGTERM": exitedPod("web-a", "rev-9", 1, terminated(143, "Error", at)),
		// A pod of another release: a rollout reports through rolloutFailure.
		"another release": exitedPod("web-a", "rev-10", 1, terminated(3, "Error", at)),
		// A crash while the release was still rolling out: it came up after,
		// and the previous release served until then.
		"a start-up crash of the rollout": exitedPod("web-a", "rev-9", 1, terminated(3, "Error", at.Add(-time.Minute))),
		// An exit from long before this pass: the pod has carried it since.
		"an old exit": exitedPod("web-a", "rev-9", 1, terminated(3, "Error", at.Add(-time.Hour))),
		// A container that has not restarted has not exited.
		"never restarted": exitedPod("web-a", "rev-9", 0, nil),
		// Another container of the pod.
		"a sidecar": func() corev1.Pod {
			p := exitedPod("web-a", "rev-9", 1, terminated(3, "Error", at))
			p.Status.ContainerStatuses[0].Name = "edge-cache"
			return p
		}(),
	} {
		if got := withInstanceExits(nil, []corev1.Pod{pod}, "rev-9", settled, at.Add(time.Minute)); len(got) != 0 {
			t.Errorf("%s: recorded %+v", name, got)
		}
	}
}

func TestInstanceExitsKeepTheNewestTen(t *testing.T) {
	at := time.Date(2026, 11, 11, 13, 0, 0, 0, time.UTC)
	var recorded []appv1alpha1.InstanceExit
	for i := range 14 {
		pod := exitedPod("web-a", "rev-9", int32(i+1), terminated(1, "Error", at.Add(time.Duration(i)*time.Minute)))
		recorded = withInstanceExits(recorded, []corev1.Pod{pod}, "rev-9", at.Add(-time.Hour), at.Add(time.Duration(i)*time.Minute))
	}
	if len(recorded) != appv1alpha1.MaxInstanceExits {
		t.Fatalf("kept %d exits, want %d", len(recorded), appv1alpha1.MaxInstanceExits)
	}
	if recorded[0].RestartCount != 14 || recorded[len(recorded)-1].RestartCount != 5 {
		t.Errorf("kept restarts %s, want 14 down to 5", fmt.Sprint(recorded[0].RestartCount, "..", recorded[len(recorded)-1].RestartCount))
	}
}

// A rollout that is still in progress has no settled time, and nothing is the
// served release's to report until it has.
func TestRolloutSettledAtIsTheTimeTheNewReplicaSetBecameAvailable(t *testing.T) {
	done := metav1.NewTime(time.Date(2026, 11, 11, 13, 0, 0, 0, time.UTC))
	dep := &appsv1.Deployment{}
	if _, ok := rolloutSettledAt(dep); ok {
		t.Error("a Deployment with no Progressing condition reads as settled")
	}
	dep.Status.Conditions = []appsv1.DeploymentCondition{{Type: appsv1.DeploymentProgressing, Reason: "ReplicaSetUpdated", LastUpdateTime: done}}
	if _, ok := rolloutSettledAt(dep); ok {
		t.Error("a rollout in progress reads as settled")
	}
	dep.Status.Conditions[0].Reason = reasonNewReplicaSetAvailable
	if at, ok := rolloutSettledAt(dep); !ok || !at.Equal(done.Time) {
		t.Errorf("settled at %v (ok %v), want %v", at, ok, done.Time)
	}
}

// An exit that follows the outage is recorded as such, so bex-api writes no
// event for it: the kubelet restarting an instance that kept failing its
// health check, and every exit of a crash loop after its first. The mark is
// decided the first time the exit is seen and kept.
func TestInstanceExitsMarkAnExitThatFollowedTheOutage(t *testing.T) {
	at := time.Date(2026, 11, 11, 13, 5, 14, 0, time.UTC)
	settled := at.Add(-time.Hour)
	unreadySince := func(p corev1.Pod, since time.Time) corev1.Pod {
		p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse, LastTransitionTime: metav1.NewTime(since)}}
		return p
	}
	for name, tc := range map[string]struct {
		pod  corev1.Pod
		want bool
	}{
		// The process died and took readiness with it in the same second.
		"a crash": {unreadySince(exitedPod("web-a", "rev-9", 1, terminated(3, "Error", at)), at), false},
		// Two minutes of failed checks, then the platform restarted it.
		"a health-check restart": {unreadySince(exitedPod("web-a", "rev-9", 1, terminated(0, "Completed", at)), at.Add(-2*time.Minute)), true},
		// The second exit of a loop: unready since the first.
		"a crash loop's second exit": {unreadySince(exitedPod("web-a", "rev-9", 2, terminated(3, "Error", at)), at.Add(-20*time.Second)), true},
		// Seen only once it is ready again: taken as the cause.
		"already recovered": {func() corev1.Pod {
			p := exitedPod("web-a", "rev-9", 1, terminated(3, "Error", at))
			p.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(at.Add(10 * time.Second))}}
			return p
		}(), false},
	} {
		got := withInstanceExits(nil, []corev1.Pod{tc.pod}, "rev-9", settled, at.Add(30*time.Second))
		if len(got) != 1 || got[0].AfterUnready != tc.want {
			t.Errorf("%s: exits = %+v, want one with afterUnready %v", name, got, tc.want)
			continue
		}
		// The mark does not change when the pod recovers and the exit is seen again.
		recovered := tc.pod
		recovered.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(at.Add(time.Minute))}}
		if again := withInstanceExits(got, []corev1.Pod{recovered}, "rev-9", settled, at.Add(2*time.Minute)); len(again) != 1 || again[0].AfterUnready != tc.want {
			t.Errorf("%s: the mark changed once the pod recovered: %+v", name, again)
		}
	}
}

// A first crash waits out a 10 s back-off with a restart count of 0, and the
// exit used to be read only once the container ran again: after bex-api had
// already recorded the outage it caused as a failed health check (w8/m158
// t009). Read before the restart, the exit carries the count the next run
// will have, so the restart itself adds nothing.
func TestInstanceExitsReadAnExitBeforeTheRestart(t *testing.T) {
	at := time.Date(2026, 11, 12, 7, 33, 48, 0, time.UTC)
	settled := at.Add(-time.Hour)
	pod := func(cs corev1.ContainerStatus) corev1.Pod {
		cs.Name = appContainerName
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: "web-a", Labels: map[string]string{labelRevision: "rev-9"}},
			Status:     corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{cs}},
		}
	}
	crash := terminated(3, "Error", at)

	crash.ContainerID = "containerd://first"
	waiting := pod(corev1.ContainerStatus{
		RestartCount:         0,
		ContainerID:          "containerd://first",
		State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		LastTerminationState: corev1.ContainerState{Terminated: crash},
	})
	got := withInstanceExits(nil, []corev1.Pod{waiting}, "rev-9", settled, at.Add(2*time.Second))
	if len(got) != 1 || got[0].RestartCount != 1 || got[0].ExitCode != 3 || !got[0].FinishedAt.Equal(&crash.FinishedAt) {
		t.Fatalf("exits in back-off = %+v, want the exit under restart count 1", got)
	}

	running := pod(corev1.ContainerStatus{
		RestartCount:         1,
		ContainerID:          "containerd://second",
		State:                corev1.ContainerState{Running: &corev1.ContainerStateRunning{StartedAt: metav1.NewTime(at.Add(10 * time.Second))}},
		LastTerminationState: corev1.ContainerState{Terminated: crash},
	})
	again := withInstanceExits(got, []corev1.Pod{running}, "rev-9", settled, at.Add(15*time.Second))
	if len(again) != 1 || &again[0] != &got[0] {
		t.Errorf("the restart recorded the same exit again: %+v", again)
	}

	// A container that has just stopped, before any back-off: the same count.
	stopped := pod(corev1.ContainerStatus{RestartCount: 0, ContainerID: "containerd://first", State: corev1.ContainerState{Terminated: crash}})
	if got := withInstanceExits(nil, []corev1.Pod{stopped}, "rev-9", settled, at.Add(time.Second)); len(got) != 1 || got[0].RestartCount != 1 {
		t.Errorf("exits of a just-stopped container = %+v, want the exit under restart count 1", got)
	}
	// A later crash of a container that has run since: one more.
	second := terminated(3, "Error", at.Add(time.Minute))
	second.ContainerID = "containerd://second"
	if got := withInstanceExits(again, []corev1.Pod{pod(corev1.ContainerStatus{
		RestartCount:         1,
		ContainerID:          "containerd://second",
		State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
		LastTerminationState: corev1.ContainerState{Terminated: second},
	})}, "rev-9", settled, at.Add(time.Minute+time.Second)); len(got) != 2 || got[0].RestartCount != 2 {
		t.Errorf("exits after a second crash = %+v, want the new exit under restart count 2 on top", got)
	}

	// A new container created but not yet started after the restart is already
	// counted: its last termination keeps the count it has.
	created := pod(corev1.ContainerStatus{
		RestartCount:         1,
		ContainerID:          "containerd://second",
		State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{}},
		LastTerminationState: corev1.ContainerState{Terminated: crash},
	})
	if got := withInstanceExits(nil, []corev1.Pod{created}, "rev-9", settled, at.Add(5*time.Second)); len(got) != 1 || got[0].RestartCount != 1 {
		t.Errorf("exits of a created, not yet started container = %+v, want the exit under restart count 1", got)
	}
	// Recorded under one count, the same exit read under another is known by
	// its finish time.
	if got := withInstanceExits([]appv1alpha1.InstanceExit{{Instance: "web-a", RestartCount: 7, ExitCode: 3, FinishedAt: crash.FinishedAt}},
		[]corev1.Pod{running}, "rev-9", settled, at.Add(15*time.Second)); len(got) != 1 {
		t.Errorf("the same exit under another count was recorded again: %+v", got)
	}

	// A container waiting to start for the first time has no exit.
	creating := pod(corev1.ContainerStatus{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}})
	if got := withInstanceExits(nil, []corev1.Pod{creating}, "rev-9", settled, at); len(got) != 0 {
		t.Errorf("exits of a container that never ran = %+v, want none", got)
	}
}

// A pod the platform stops (a scale-in, a drain, a rollout) or evicts is not
// restarted, so its container stays Terminated with its answer to SIGTERM.
// That is not the application exiting (review of w8/m158 t009).
func TestInstanceExitsLeaveOutAPodThePlatformStopped(t *testing.T) {
	at := time.Date(2026, 11, 12, 8, 0, 0, 0, time.UTC)
	settled := at.Add(-time.Hour)
	stopped := func(name string) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{Name: name, Labels: map[string]string{labelRevision: "rev-9"}},
			Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{
				Name: appContainerName, ContainerID: "containerd://x",
				State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0, Reason: "Completed", ContainerID: "containerd://x", FinishedAt: metav1.NewTime(at)}},
			}}},
		}
	}
	deleting := stopped("web-scaled-in")
	deleting.DeletionTimestamp = &metav1.Time{Time: at.Add(-30 * time.Second)}
	evicted := stopped("web-evicted")
	evicted.Status.Phase = corev1.PodFailed
	evicted.Status.Reason = "Evicted"
	if got := withInstanceExits(nil, []corev1.Pod{deleting, evicted}, "rev-9", settled, at.Add(time.Second)); len(got) != 0 {
		t.Errorf("exits = %+v, want none from pods the platform stopped", got)
	}
}
