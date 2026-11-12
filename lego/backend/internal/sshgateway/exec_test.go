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

package sshgateway

import (
	"context"
	"errors"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/bex-co/bex/lego/backend/internal/apps"
)

func TestKubeExecutorRecognizesTerminalTarget(t *testing.T) {
	for _, pod := range []*corev1.Pod{
		{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"}, Status: corev1.PodStatus{Phase: corev1.PodFailed}},
		{ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"}, Status: corev1.PodStatus{Phase: corev1.PodRunning, ContainerStatuses: []corev1.ContainerStatus{{Name: "sandbox", State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}}}}}},
	} {
		executor := &KubeExecutor{Client: fake.NewSimpleClientset(pod)}
		err := executor.checkTarget(context.Background(), apps.SSHInstanceTarget{
			PodName: "pod", Namespace: "ns", Container: "sandbox",
		})
		if !errors.Is(err, ErrTargetTerminated) {
			t.Fatalf("phase=%s err=%v, want ErrTargetTerminated", pod.Status.Phase, err)
		}
	}
}

func TestKubeExecutorTreatsMissingTargetAsTerminal(t *testing.T) {
	executor := &KubeExecutor{Client: fake.NewSimpleClientset()}
	err := executor.checkTarget(context.Background(), apps.SSHInstanceTarget{
		PodName: "gone", Namespace: "ns", Container: "sandbox",
	})
	if !errors.Is(err, ErrTargetTerminated) {
		t.Fatalf("err=%v, want ErrTargetTerminated", err)
	}
	var terminated *TargetTerminatedError
	if !errors.As(err, &terminated) || terminated.Reason != "" || terminated.ExitCode != nil {
		t.Fatalf("a missing pod recorded a termination: %#v", err)
	}
}

// readyPod is a Running sandbox Pod whose Ready condition reads status.
func readyPod(status corev1.ConditionStatus) *corev1.Pod {
	return &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "pod", Namespace: "ns"},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: status}},
			ContainerStatuses: []corev1.ContainerStatus{{Name: "sandbox", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
		},
	}
}

// A Pod on a NotReady or unreachable node still reads Running; the
// node lifecycle controller marks it Ready=False (or Unknown), and the gateway
// reports it as not ready, never as terminated or as a generic failure.
func TestKubeExecutorReportsANotReadyTarget(t *testing.T) {
	noConditions := readyPod(corev1.ConditionTrue)
	noConditions.Status.Conditions = nil
	for name, pod := range map[string]*corev1.Pod{
		"node NotReady":      readyPod(corev1.ConditionFalse),
		"node unreachable":   readyPod(corev1.ConditionUnknown),
		"no Ready condition": noConditions,
	} {
		t.Run(name, func(t *testing.T) {
			executor := &KubeExecutor{Client: fake.NewSimpleClientset(pod)}
			err := executor.checkTarget(context.Background(), apps.SSHInstanceTarget{PodName: "pod", Namespace: "ns", Container: "sandbox"})
			if !errors.Is(err, ErrTargetNotReady) || errors.Is(err, ErrTargetTerminated) {
				t.Fatalf("err=%v, want ErrTargetNotReady only", err)
			}
		})
	}
	executor := &KubeExecutor{Client: fake.NewSimpleClientset(readyPod(corev1.ConditionTrue))}
	if err := executor.checkTarget(context.Background(), apps.SSHInstanceTarget{PodName: "pod", Namespace: "ns", Container: "sandbox"}); err != nil {
		t.Fatalf("a Ready target was classified: %v", err)
	}
}

// A terminal target names how its container ended, so a lost
// session sandbox fails with its cause.
func TestKubeExecutorRecordsHowATerminalTargetEnded(t *testing.T) {
	oom := readyPod(corev1.ConditionFalse)
	oom.Status.Phase = corev1.PodFailed
	oom.Status.ContainerStatuses[0].State = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "OOMKilled", ExitCode: 137}}
	restarted := readyPod(corev1.ConditionTrue)
	restarted.Status.Phase = corev1.PodSucceeded
	restarted.Status.ContainerStatuses[0].LastTerminationState = corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{Reason: "Error", ExitCode: 3}}
	evicted := readyPod(corev1.ConditionFalse)
	evicted.Status.Phase, evicted.Status.Reason = corev1.PodFailed, "Evicted"
	evicted.Status.ContainerStatuses = nil
	// A Pod on a node NotReady past its toleration is deleted by the taint
	// manager while its container still reads Running.
	tainted := readyPod(corev1.ConditionFalse)
	tainted.DeletionTimestamp = &metav1.Time{Time: time.Now()}
	tainted.Finalizers = []string{"test/keep"} // the fake tracker refuses a deleting object without one
	tainted.Status.Conditions = append(tainted.Status.Conditions, corev1.PodCondition{
		Type: corev1.DisruptionTarget, Status: corev1.ConditionTrue, Reason: "DeletionByTaintManager",
	})
	for _, tc := range []struct {
		name   string
		pod    *corev1.Pod
		reason string
		code   int
	}{
		{"OOM killed", oom, "OOMKilled", 137},
		{"earlier exit", restarted, "Error", 3},
		{"evicted pod", evicted, "Evicted", -1},
		{"taint eviction", tainted, "DeletionByTaintManager", -1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			executor := &KubeExecutor{Client: fake.NewSimpleClientset(tc.pod)}
			err := executor.checkTarget(context.Background(), apps.SSHInstanceTarget{PodName: "pod", Namespace: "ns", Container: "sandbox"})
			var terminated *TargetTerminatedError
			if !errors.As(err, &terminated) || !errors.Is(err, ErrTargetTerminated) {
				t.Fatalf("err=%v, want a TargetTerminatedError", err)
			}
			code := -1
			if terminated.ExitCode != nil {
				code = *terminated.ExitCode
			}
			if terminated.Reason != tc.reason || code != tc.code {
				t.Fatalf("termination = %q exit %d, want %q exit %d", terminated.Reason, code, tc.reason, tc.code)
			}
		})
	}
}
