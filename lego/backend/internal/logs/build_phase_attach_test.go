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
// "Build successful") finishes between two polls; it is seen as finished and
// still attached, while a phase that ended before the tail's lower bound is not
// (w8/m159 t008). Phases come back in the pod's own order, the ended ones
// before the running one, so a phase read late never prints after its
// successor.
func TestBuildTailAttachesAPhaseThatFinishedBetweenPolls(t *testing.T) {
	opened := time.Date(2026, 11, 12, 10, 58, 0, 0, time.UTC)
	ended := func(at time.Time) corev1.ContainerState {
		return corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{FinishedAt: metav1.NewTime(at)}}
	}
	running := corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}
	atAttach := corev1.Pod{Status: corev1.PodStatus{
		InitContainerStatuses: []corev1.ContainerStatus{
			{Name: "clone", State: ended(opened.Add(-time.Minute))},
			{Name: "prepare", State: ended(opened.Add(time.Second))},
			{Name: "buildkit", State: running},
		},
		ContainerStatuses: []corev1.ContainerStatus{{Name: "push"}},
	}}
	before := terminatedBefore(atAttach, opened)
	started := map[string]bool{}
	gotEnded, gotRunning := phasesToAttach(atAttach, started, before)
	if !slices.Equal(gotEnded, []string{"prepare"}) || !slices.Equal(gotRunning, []string{"buildkit"}) {
		t.Fatalf("at attach: ended %v running %v, want the phase that ended after the bound, then the running one", gotEnded, gotRunning)
	}
	for _, name := range append(gotEnded, gotRunning...) {
		started[name] = true
	}
	later := atAttach
	later.Status.InitContainerStatuses = []corev1.ContainerStatus{
		{Name: "clone", State: ended(opened.Add(-time.Minute))},
		{Name: "prepare", State: ended(opened.Add(time.Second))},
		{Name: "buildkit", State: ended(opened.Add(time.Minute))},
	}
	later.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: "push", State: ended(opened.Add(2 * time.Minute))}}
	if gotEnded, gotRunning = phasesToAttach(later, started, before); !slices.Equal(gotEnded, []string{"push"}) || len(gotRunning) != 0 {
		t.Fatalf("later: ended %v running %v, want only the push", gotEnded, gotRunning)
	}
}
