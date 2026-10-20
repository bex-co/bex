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
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// w6/084: a queued build names the scarce resources and never the platform's
// layout (node counts, pool taints) from the scheduler's raw verdict.
func TestUnschedulableReasonHidesPlatformLayout(t *testing.T) {
	pending := func(message string) []corev1.Pod {
		return []corev1.Pod{{Status: corev1.PodStatus{Phase: corev1.PodPending, Conditions: []corev1.PodCondition{{
			Type: corev1.PodScheduled, Status: corev1.ConditionFalse, Message: message,
		}}}}}
	}
	for _, tc := range []struct{ scheduler, want string }{
		{"0/3 nodes are available: 1 Insufficient memory, 2 node(s) had untolerated taint {bex.co/pool: platform}.",
			"no build node has room for it yet (not enough memory free)"},
		{"0/3 nodes are available: 3 node(s) didn't match Pod's node affinity/selector.",
			"no build node has room for it yet"},
	} {
		if got := unschedulableReason(pending(tc.scheduler)); got != tc.want {
			t.Errorf("unschedulableReason(%q) = %q, want %q", tc.scheduler, got, tc.want)
		}
	}
}
