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

	corev1 "k8s.io/api/core/v1"
)

// w4/582: a build phase shorter than the tail's poll (the push, which prints
// "Build successful") finishes between two polls; it is seen as finished and
// still attached, while a phase that was over before the tail attached is not.
func TestBuildTailAttachesAPhaseThatFinishedBetweenPolls(t *testing.T) {
	ended := corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 0}}
	running := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	atAttach := corev1.Pod{Status: corev1.PodStatus{
		InitContainerStatuses: []corev1.ContainerStatus{{Name: "clone", State: ended}, {Name: "buildkit", State: running}},
		ContainerStatuses:     []corev1.ContainerStatus{{Name: "push"}},
	}}
	before := terminatedContainers(atAttach)
	later := corev1.Pod{Status: corev1.PodStatus{
		InitContainerStatuses: []corev1.ContainerStatus{{Name: "clone", State: ended}, {Name: "buildkit", State: ended}},
		ContainerStatuses:     []corev1.ContainerStatus{{Name: "push", State: ended}},
	}}
	var attach []string
	for name := range terminatedContainers(later) {
		if !before[name] {
			attach = append(attach, name)
		}
	}
	slices.Sort(attach)
	if !slices.Equal(attach, []string{"buildkit", "push"}) {
		t.Fatalf("attach %v, want the phases that ended after the tail attached", attach)
	}
}
