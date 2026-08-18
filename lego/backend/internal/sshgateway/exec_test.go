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
}
