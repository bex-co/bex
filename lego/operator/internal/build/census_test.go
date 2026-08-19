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
	"testing"
	"time"

	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const censusNS = "bex-build"

// censusJob is a dispatched build Job created `age` before the census instant.
func censusJob(name, workspace string, age time.Duration, finished bool) *batchv1.Job {
	job := &batchv1.Job{ObjectMeta: metav1.ObjectMeta{
		Name:              name,
		Namespace:         censusNS,
		CreationTimestamp: metav1.NewTime(signalsEpoch.Add(-age)),
		Labels: map[string]string{
			"app.bex.co/component": "build",
			"app.bex.co/workspace": workspace,
		},
	}}
	if finished {
		job.Status.Conditions = []batchv1.JobCondition{{Type: batchv1.JobComplete, Status: corev1.ConditionTrue}}
	}
	return job
}

func censusPod(name, jobName, nodeName string, phase corev1.PodPhase) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: censusNS,
			Labels:    map[string]string{"app.bex.co/component": "build", "job-name": jobName},
		},
		Spec:   corev1.PodSpec{NodeName: nodeName},
		Status: corev1.PodStatus{Phase: phase},
	}
}

// TestCountBuildsSeparatesQueuedFromRunning pins the gauges' definitions:
// queued ⊆ active, "placed on a node" (never pod phase) is the dividing line,
// and a finished build counts as neither.
func TestCountBuildsSeparatesQueuedFromRunning(t *testing.T) {
	cl := fakeClient(
		// Running: a pod is on a node. It reports phase Pending because the build
		// runs in initContainers — reading phase would call this queued.
		censusJob("bld-running", "tea-a", 3*time.Minute, false),
		censusPod("bld-running-x", "bld-running", "node-a", corev1.PodPending),
		// Queued: the pod exists but the scheduler could not place it.
		censusJob("bld-queued", "tea-b", 9*time.Minute, false),
		censusPod("bld-queued-x", "bld-queued", "", corev1.PodPending),
		// Queued: no pod at all yet — the window a cold scale-from-zero spends.
		censusJob("bld-nopod", "tea-c", 4*time.Minute, false),
		// Finished: counts toward nothing, though its pods linger until the TTL.
		censusJob("bld-done", "tea-a", 40*time.Minute, true),
		censusPod("bld-done-x", "bld-done", "node-a", corev1.PodSucceeded),
	)
	got, err := CountBuilds(context.Background(), cl, censusNS, signalsEpoch)
	if err != nil {
		t.Fatalf("CountBuilds: %v", err)
	}
	if got.Active != 3 {
		t.Errorf("Active = %d, want 3 (the finished build must not count)", got.Active)
	}
	if got.Queued != 2 {
		t.Errorf("Queued = %d, want 2 (unplaced pod + no pod yet)", got.Queued)
	}
	if got.OldestQueued != 9*time.Minute {
		t.Errorf("OldestQueued = %v, want 9m (the longest-waiting queued build)", got.OldestQueued)
	}
}

// TestCountBuildsIdleFleetIsZero: the gauges must return to zero, or a stranded
// value pages forever about a build that finished hours ago. Recomputing from a
// full listing is what guarantees it, so this is the assertion that the recount
// is a recount.
func TestCountBuildsIdleFleetIsZero(t *testing.T) {
	cl := fakeClient(
		censusJob("bld-done", "tea-a", time.Hour, true),
		censusPod("bld-done-x", "bld-done", "node-a", corev1.PodSucceeded),
	)
	got, err := CountBuilds(context.Background(), cl, censusNS, signalsEpoch)
	if err != nil {
		t.Fatalf("CountBuilds: %v", err)
	}
	if got.Active != 0 || got.Queued != 0 || got.OldestQueued != 0 {
		t.Errorf("Census = %+v on an idle fleet, want all zero", got)
	}
}

// TestActiveBuildsIsClusterWide: the cluster-wide ceiling exists precisely
// because N workspaces each under their own cap still saturate shared capacity,
// so this count must NOT be workspace-scoped.
func TestActiveBuildsIsClusterWide(t *testing.T) {
	cl := fakeClient(
		censusJob("bld-a", "tea-a", time.Minute, false),
		censusJob("bld-b", "tea-b", time.Minute, false),
		censusJob("bld-c", "tea-c", time.Minute, false),
	)
	total, err := ActiveBuilds(context.Background(), cl, censusNS)
	if err != nil {
		t.Fatalf("ActiveBuilds: %v", err)
	}
	if total != 3 {
		t.Errorf("ActiveBuilds = %d, want 3", total)
	}
	perWorkspace, err := ActiveWorkspaceBuilds(context.Background(), cl, censusNS, "tea-a")
	if err != nil {
		t.Fatalf("ActiveWorkspaceBuilds: %v", err)
	}
	if perWorkspace != 1 {
		t.Errorf("ActiveWorkspaceBuilds(tea-a) = %d, want 1 — the per-workspace cap must stay narrow", perWorkspace)
	}
}

// TestTrimOvershootDeletesTheNewest is the self-correction the list-then-create
// race needs: the build that has been running longest survives, the racer that
// did the least work backs off, and the choice is deterministic so two racers
// converge on the same survivor instead of deleting each other.
func TestTrimOvershootDeletesTheNewest(t *testing.T) {
	cl := fakeClient(
		censusJob("bld-oldest", "tea-a", 10*time.Minute, false),
		censusJob("bld-middle", "tea-a", 5*time.Minute, false),
		censusJob("bld-newest", "tea-a", time.Minute, false),
	)
	deleted, err := TrimOvershoot(context.Background(), cl, censusNS, "tea-a", 2)
	if err != nil {
		t.Fatalf("TrimOvershoot: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "bld-newest" {
		t.Fatalf("deleted = %v, want [bld-newest]", deleted)
	}
	left, err := ActiveWorkspaceBuilds(context.Background(), cl, censusNS, "tea-a")
	if err != nil {
		t.Fatalf("ActiveWorkspaceBuilds: %v", err)
	}
	if left != 2 {
		t.Errorf("active after trim = %d, want 2 (back at the cap)", left)
	}
	var survivor batchv1.Job
	if err := cl.Get(context.Background(), client.ObjectKey{Namespace: censusNS, Name: "bld-oldest"}, &survivor); err != nil {
		t.Errorf("the longest-running build was deleted: %v", err)
	}
}

// TestTrimOvershootIsInertWithinTheCap: no cap, or no overshoot, must delete
// nothing — the correction may never become a source of build cancellations.
func TestTrimOvershootIsInertWithinTheCap(t *testing.T) {
	objs := []client.Object{
		censusJob("bld-a", "tea-a", 10*time.Minute, false),
		censusJob("bld-b", "tea-a", time.Minute, false),
	}
	for _, tc := range []struct {
		name  string
		limit int
	}{
		{"unset cap is unlimited", 0},
		{"exactly at the cap", 2},
		{"under the cap", 5},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cl := fakeClient(objs...)
			deleted, err := TrimOvershoot(context.Background(), cl, censusNS, "tea-a", tc.limit)
			if err != nil {
				t.Fatalf("TrimOvershoot: %v", err)
			}
			if len(deleted) != 0 {
				t.Errorf("deleted = %v, want none", deleted)
			}
		})
	}
}

// TestTrimOvershootClusterWideIgnoresWorkspaces: an empty workspace means the
// global ceiling, which must shed across tenants — that is the whole point of
// having a second cap.
func TestTrimOvershootClusterWideIgnoresWorkspaces(t *testing.T) {
	cl := fakeClient(
		censusJob("bld-a", "tea-a", 10*time.Minute, false),
		censusJob("bld-b", "tea-b", 5*time.Minute, false),
		censusJob("bld-c", "tea-c", time.Minute, false),
	)
	deleted, err := TrimOvershoot(context.Background(), cl, censusNS, "", 2)
	if err != nil {
		t.Fatalf("TrimOvershoot: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "bld-c" {
		t.Errorf("deleted = %v, want [bld-c] (newest, regardless of workspace)", deleted)
	}
}

// TestTrimOvershootNeverDeletesFinishedBuilds: a finished Job holds no capacity,
// so shedding one would be pure collateral damage — and it would destroy the
// completed build's own result before the reconcile that dispatched it observes
// it.
func TestTrimOvershootNeverDeletesFinishedBuilds(t *testing.T) {
	cl := fakeClient(
		censusJob("bld-old-running", "tea-a", 10*time.Minute, false),
		censusJob("bld-new-running", "tea-a", 2*time.Minute, false),
		censusJob("bld-newest-done", "tea-a", time.Minute, true),
	)
	deleted, err := TrimOvershoot(context.Background(), cl, censusNS, "tea-a", 1)
	if err != nil {
		t.Fatalf("TrimOvershoot: %v", err)
	}
	if len(deleted) != 1 || deleted[0] != "bld-new-running" {
		t.Errorf("deleted = %v, want [bld-new-running] — the newest ACTIVE build", deleted)
	}
}
