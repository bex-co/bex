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
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// w6/080: a pod that never starts — unschedulable, or stuck creating its
// container — is named on the rollout instead of the bare progress deadline,
// and only once it has been stuck long enough not to be ordinary progress.
func TestPendingPodMessage(t *testing.T) {
	now := time.Date(2026, 10, 20, 7, 0, 0, 0, time.UTC)
	const scheduler = "0/3 nodes are available: 1 Insufficient cpu, 1 Insufficient memory, 2 node(s) had untolerated taint {bex.co/pool: platform}. preemption: 0/3 nodes are available"
	unschedulable := func(since time.Duration) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-since))},
			Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{
				Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Reason: corev1.PodReasonUnschedulable,
				Message: scheduler, LastTransitionTime: metav1.NewTime(now.Add(-since)),
			}}},
		}
	}
	// Created 20 minutes ago, placed `since` ago: the stall clock starts at
	// placement, not creation, so time spent waiting for a node is not
	// counted as a slow container start.
	creating := func(since time.Duration) corev1.Pod {
		return corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{CreationTimestamp: metav1.NewTime(now.Add(-20 * time.Minute))},
			Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{
				Type: corev1.PodScheduled, Status: corev1.ConditionTrue, LastTransitionTime: metav1.NewTime(now.Add(-since)),
			}}, ContainerStatuses: []corev1.ContainerStatus{{
				Name: appContainerName, State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}},
			}}},
		}
	}
	terminating := unschedulable(10 * time.Minute)
	terminating.DeletionTimestamp = &metav1.Time{Time: now}

	for _, tc := range []struct {
		name   string
		pod    corev1.Pod
		reason string
	}{
		{"unschedulable past the grace", unschedulable(2 * time.Minute), appv1alpha1.ReasonUnschedulable},
		{"unschedulable within a scheduling retry", unschedulable(30 * time.Second), ""},
		{"creating for a long time", creating(6 * time.Minute), appv1alpha1.ReasonContainerCreating},
		{"creating normally after a long queue", creating(2 * time.Minute), ""},
		{"terminating", terminating, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			reason, msg := pendingPodMessage([]corev1.Pod{tc.pod}, now)
			if reason != tc.reason || (reason == "") != (msg == "") {
				t.Fatalf("reason=%q msg=%q, want reason %q", reason, msg, tc.reason)
			}
		})
	}

	_, msg := pendingPodMessage([]corev1.Pod{unschedulable(2 * time.Minute)}, now)
	if !strings.Contains(msg, "not enough cpu, memory free") || strings.Contains(msg, "taint") || strings.Contains(msg, "bex.co/pool") {
		t.Fatalf("unschedulable message must keep the resource verdict and drop platform layout: %q", msg)
	}
	if final := terminalStallMessage(appv1alpha1.ReasonUnschedulable, msg); strings.Contains(final, unschedulableWaiting) || !strings.Contains(final, unschedulableFailed) {
		t.Fatalf("the failed deploy's record still promises a start: %q", final)
	}
}

// A pod scheduled (or reaped) just before the deadline leaves nothing for the
// settle-time scan; the last recorded pending diagnosis must still reach the
// failed deploy instead of the bare progress deadline.
func TestLastStallDiagnosisKeepsPendingReasons(t *testing.T) {
	for _, reason := range []string{appv1alpha1.ReasonUnschedulable, appv1alpha1.ReasonContainerCreating} {
		app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Generation: 3}}
		app.Status.Conditions = []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, Message: "m", ObservedGeneration: 3,
		}}
		if got, msg := lastStallDiagnosis(app); got != reason || msg != "m" {
			t.Errorf("%s: lastStallDiagnosis = %q %q", reason, got, msg)
		}
	}
}
