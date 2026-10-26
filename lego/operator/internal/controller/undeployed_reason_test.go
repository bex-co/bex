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
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestUndeployedChangesReason is w4/414: the reason names the actual cause —
// a per-deploy image override is not a saved change, a rollback keeps the
// historical meaning, and any genuine saved difference wins as saved.
func TestUndeployedChangesReason(t *testing.T) {
	appWithRef := func(ref *appv1alpha1.ReleaseConfigReference) *appv1alpha1.App {
		return &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
			Spec:       appv1alpha1.AppSpec{Image: "img:v1", ReleaseConfig: ref},
			Status:     appv1alpha1.AppStatus{ReleaseGeneration: 3},
		}
	}
	override := &appv1alpha1.ReleaseConfigReference{Generation: 3, Image: "img:v2"}
	rollback := &appv1alpha1.ReleaseConfigReference{Generation: 3, SourceGeneration: 1, Image: "img:v0"}

	for _, tc := range []struct {
		name      string
		app       *appv1alpha1.App
		different bool
		imageOnly bool
		want      string
	}{
		{"no difference", appWithRef(override), false, false, ""},
		{"override only", appWithRef(override), true, true, appv1alpha1.UndeployedReasonImageOverride},
		{"override plus a saved change", appWithRef(override), true, false, appv1alpha1.UndeployedReasonSaved},
		{"rollback to an older image", appWithRef(rollback), true, true, appv1alpha1.UndeployedReasonHistoricalRelease},
		{"plain saved change", appWithRef(nil), true, false, appv1alpha1.UndeployedReasonSaved},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := undeployedChangesReason(tc.app, tc.different, tc.imageOnly); got != tc.want {
				t.Errorf("reason = %q, want %q", got, tc.want)
			}
		})
	}
}
