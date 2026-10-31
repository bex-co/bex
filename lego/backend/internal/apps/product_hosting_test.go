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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// currentlyHosting is the hosting sample's only proof: a current, successful
// Ready condition on a running, unsuspended App. Every other shape is no proof.
func TestCurrentlyHostingRequiresACurrentSuccessfulReadyCondition(t *testing.T) {
	ready := func(status metav1.ConditionStatus, generation int64) []metav1.Condition {
		return []metav1.Condition{{
			Type: appv1alpha1.ConditionReady, Status: status, ObservedGeneration: generation,
		}}
	}
	for _, tc := range []struct {
		name string
		app  *appv1alpha1.App
		want bool
	}{
		{
			name: "running with a current ready condition",
			app: &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseRunning, Conditions: ready(metav1.ConditionTrue, 3),
				},
			},
			want: true,
		},
		{
			name: "no ready condition",
			app: &appv1alpha1.App{
				Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
			},
		},
		{
			name: "ready condition is false",
			app: &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseRunning, Conditions: ready(metav1.ConditionFalse, 3),
				},
			},
		},
		{
			name: "ready condition is from an older generation",
			app: &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Generation: 4},
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseRunning, Conditions: ready(metav1.ConditionTrue, 3),
				},
			},
		},
		{
			name: "suspended app",
			app: &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Spec:       appv1alpha1.AppSpec{Suspended: true},
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseRunning, Conditions: ready(metav1.ConditionTrue, 3),
				},
			},
		},
		{
			name: "app is not running",
			app: &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Generation: 3},
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseDeploying, Conditions: ready(metav1.ConditionTrue, 3),
				},
			},
		},
	} {
		if got := currentlyHosting(tc.app); got != tc.want {
			t.Errorf("%s: currentlyHosting = %v, want %v", tc.name, got, tc.want)
		}
	}
}
