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

package logs

import (
	"slices"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// w4/582: a build phase shorter than the tail's poll (the push, which prints
// "Build successful") finishes between two polls; it is still attached, while
// a phase that ran before the tail attached is not replayed.
func TestBuildTailAttachesAPhaseThatFinishedBetweenPolls(t *testing.T) {
	attached := time.Date(2026, 11, 12, 3, 0, 0, 0, time.UTC)
	ended := func(started time.Time) corev1.ContainerState {
		return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{StartedAt: metav1.NewTime(started), FinishedAt: metav1.NewTime(started.Add(time.Second))}}
	}
	pod := corev1.Pod{Status: corev1.PodStatus{
		InitContainerStatuses: []corev1.ContainerStatus{
			{Name: "clone", State: ended(attached.Add(-time.Minute))},
			{Name: "buildkit", State: ended(attached.Add(-time.Second))},
		},
		ContainerStatuses: []corev1.ContainerStatus{
			{Name: "push", State: ended(attached.Add(3 * time.Second))},
			{Name: "cache-save", State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}},
		},
	}}
	if got := containersStartedSince(pod, attached); !slices.Equal(got, []string{"cache-save", "push"}) {
		t.Fatalf("attached %v, want the running cache-save and the push that finished since", got)
	}
}
