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

package apps

import (
	"context"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w2/m253: a crash-loop suspension reports Render's stuck_crashlooping
// suspender on every read, and an owner's resume — unlike a Free-hours one —
// is allowed and clears it.
func TestCrashLoopSuspensionReportsAndResumes(t *testing.T) {
	ctx := context.Background()
	a := sampleApp("looping")
	a.Spec.Suspended = true
	a.Annotations = map[string]string{core.AnnotCrashLoopSuspended: "2026-11-03T12:00:00Z"}
	svc, cl := newService(nil, a)

	v, err := svc.Get(ctx, "looping")
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Join(core.SuspendersFor(v.Suspended, v.FreeHoursSuspended, v.CrashLoopSuspended), ","); got != "stuck_crashlooping" {
		t.Fatalf("suspenders = %q, want stuck_crashlooping", got)
	}
	if got := toRenderService(v).Suspenders; strings.Join(got, ",") != "stuck_crashlooping" {
		t.Errorf("REST suspenders = %v", got)
	}

	if _, err := svc.Resume(ctx, "looping"); err != nil {
		t.Fatalf("resume: %v", err)
	}
	after := getApp(t, cl, "looping")
	if after.Spec.Suspended || after.Annotations[core.AnnotCrashLoopSuspended] != "" {
		t.Errorf("resume left suspended=%v annotation=%q", after.Spec.Suspended, after.Annotations[core.AnnotCrashLoopSuspended])
	}

	// A user suspension stays "user".
	if got := core.SuspendersFor(true, false, false); strings.Join(got, ",") != "user" {
		t.Errorf("user suspenders = %v", got)
	}
}

// The status half of ADR096's rule: which App statuses read as a crash loop.
// The operator reports a served release whose pods crash as Deploying, so the
// phase alone never separates a crash loop from a rollout; the release does.
func TestCrashLoopFailingSinceStatusRule(t *testing.T) {
	since := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	status := func(phase appv1alpha1.AppPhase, active string, release int64, reason string, extra ...metav1.Condition) appv1alpha1.AppStatus {
		return appv1alpha1.AppStatus{
			Phase: phase, ActiveRevision: active, ReleaseGeneration: release,
			Conditions: append([]metav1.Condition{{
				Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, LastTransitionTime: metav1.NewTime(since),
			}}, extra...),
		}
	}
	verdict := func(gen int64) metav1.Condition {
		return metav1.Condition{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: appv1alpha1.ReasonCrashLoopBackOff, ObservedGeneration: gen}
	}
	crash := appv1alpha1.ReasonCrashLoopBackOff
	for _, tc := range []struct {
		name   string
		status appv1alpha1.AppStatus
		want   bool
	}{
		{"served release crash-looping (Deploying)", status(appv1alpha1.PhaseDeploying, "rev-4", 4, crash), true},
		{"served release failing its health check", status(appv1alpha1.PhaseDeploying, "rev-4", 4, appv1alpha1.ReasonHealthCheckFailing), true},
		{"served release, Running", status(appv1alpha1.PhaseRunning, "rev-4", 4, crash), true},
		{"served release whose restore failed (Failed)", status(appv1alpha1.PhaseFailed, "rev-4", 4, crash), true},
		{"newer release rolled out failed over the served one", status(appv1alpha1.PhaseDeploying, "rev-4", 5, crash, verdict(5)), true},
		{"legacy status without a release generation", status(appv1alpha1.PhaseDeploying, "rev-4", 0, crash), true},
		{"newer release still rolling out", status(appv1alpha1.PhaseDeploying, "rev-4", 5, crash), false},
		{"newer release rolling out past an older verdict", status(appv1alpha1.PhaseDeploying, "rev-4", 6, crash, verdict(5)), false},
		{"first release stopped at zero (w1/m179)", status(appv1alpha1.PhaseFailed, "", 1, crash, verdict(1)), false},
		{"first release still rolling out", status(appv1alpha1.PhaseDeploying, "", 1, crash), false},
		{"building", status(appv1alpha1.PhaseBuilding, "rev-4", 4, crash), false},
		{"hibernated", status(appv1alpha1.PhaseHibernated, "rev-4", 4, crash), false},
		{"not a start failure", status(appv1alpha1.PhaseDeploying, "rev-4", 4, "Unschedulable"), false},
	} {
		a := sampleApp("svc")
		a.Status = tc.status
		got, ok := CrashLoopFailingSince(a)
		if ok != tc.want || (ok && !got.Equal(since)) {
			t.Errorf("%s: failing=%v since=%v, want %v", tc.name, ok, got, tc.want)
		}
	}
	a := sampleApp("svc")
	a.Status = status(appv1alpha1.PhaseDeploying, "rev-4", 4, crash)
	a.Spec.Suspended = true
	if _, ok := CrashLoopFailingSince(a); ok {
		t.Error("a suspended service is never failing")
	}
}

// The pod half: restarts continue and no instance is Ready.
func TestCrashLoopRestartingReadsThePods(t *testing.T) {
	p := func(restarts int32, waiting string, ready bool, mut ...func(*corev1.Pod)) corev1.Pod {
		pod := corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodRunning}}
		cs := corev1.ContainerStatus{Name: core.AppContainer, RestartCount: restarts}
		if waiting != "" {
			cs.State.Waiting = &corev1.ContainerStateWaiting{Reason: waiting}
		}
		pod.Status.ContainerStatuses = []corev1.ContainerStatus{cs}
		if ready {
			pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
		}
		for _, m := range mut {
			m(&pod)
		}
		return pod
	}
	crash := appv1alpha1.ReasonCrashLoopBackOff
	sidecar := func(pod *corev1.Pod) { pod.Status.ContainerStatuses[0].Name = "edge-cache" }
	finished := func(pod *corev1.Pod) { pod.Status.Phase = corev1.PodFailed }
	for _, tc := range []struct {
		name string
		pods []corev1.Pod
		want bool
	}{
		{"no pods (parked at zero)", nil, false},
		{"back-off", []corev1.Pod{p(5, crash, false)}, true},
		{"restarted, running unready", []corev1.Pod{p(3, "", false)}, true},
		{"first start, not ready yet", []corev1.Pod{p(0, "", false)}, false},
		{"one instance Ready", []corev1.Pod{p(5, crash, false), p(0, "", true)}, false},
		{"only a sidecar restarts", []corev1.Pod{p(5, crash, false, sidecar)}, false},
		{"only a finished pod restarted", []corev1.Pod{p(5, crash, false, finished)}, false},
	} {
		if got := CrashLoopRestarting(tc.pods); got != tc.want {
			t.Errorf("%s: restarting=%v, want %v", tc.name, got, tc.want)
		}
	}
}
