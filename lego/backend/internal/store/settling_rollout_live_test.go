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

package store

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m320: on Render a deploy goes live when traffic moves to it. A rollout
// that the operator reports settling (every pod of the release Ready, only the
// old pods' reap pending) and whose release it recorded active closes live
// then, not when the Deployment status converges; a live probe saw 24 s
// between the instance switch and the row turning live.
func TestASettlingRolloutThatServesItsReleaseIsLive(t *testing.T) {
	deploying := func(reason, active string) *appv1alpha1.App {
		app := &appv1alpha1.App{}
		app.Generation = 3
		app.Status.Phase = appv1alpha1.PhaseDeploying
		app.Status.ReleaseGeneration = 3
		app.Status.ActiveRevision = active
		app.Status.Conditions = []metav1.Condition{{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse, Reason: reason, ObservedGeneration: 3}}
		return app
	}
	open := Deploy{Generation: 3, Status: DeployUpdateInProgress}
	for _, tc := range []struct {
		name, reason, active, want string
	}{
		{"settling, its release active", appv1alpha1.ReasonRolloutSettling, "rev-3", DeployLive},
		// A held web image release (w4/m320) or a rollout whose pods are not all
		// Ready has not served: the prior release is still the active one.
		{"settling, the prior release active", appv1alpha1.ReasonRolloutSettling, "rev-2", DeployUpdateInProgress},
		{"progressing", "RolloutProgressing", "rev-3", DeployUpdateInProgress},
	} {
		if got := observedDeployStatus(open, deploying(tc.reason, tc.active), false); got != tc.want {
			t.Errorf("%s: status = %q, want %q", tc.name, got, tc.want)
		}
	}
}
