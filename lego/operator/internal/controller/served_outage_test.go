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
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestServedReleaseOutageIsNotADeploy (w4/m327): a live release whose pods
// lose readiness, with no newer release rolling, is that release's outage. Its
// phase stays Running while Ready=False names the cause. A newer release
// rolling, and the live release starting after a wake, still read Deploying.
func TestServedReleaseOutageIsNotADeploy(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name          string
		phase         appv1alpha1.AppPhase
		active        string
		release       int64
		template      string
		want          appv1alpha1.AppPhase
		wantApplyable bool
	}{
		{"the live release loses readiness", appv1alpha1.PhaseRunning, "rev-3", 3, "rev-3", appv1alpha1.PhaseRunning, false},
		{"a newer release rolls", appv1alpha1.PhaseRunning, "rev-2", 3, "rev-3", appv1alpha1.PhaseDeploying, true},
		{"the live release starts after a wake", appv1alpha1.PhaseHibernated, "rev-3", 3, "rev-3", appv1alpha1.PhaseDeploying, false},
		{"a first release starts", appv1alpha1.PhasePending, "", 1, "rev-1", appv1alpha1.PhaseDeploying, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: tc.release},
				Spec:       appv1alpha1.AppSpec{Image: "busybox:1.37"},
				Status:     appv1alpha1.AppStatus{Phase: tc.phase, ActiveRevision: tc.active, ReleaseGeneration: tc.release},
			}
			dep := progressDeadlineDep("web")
			dep.Spec.Template.Labels[labelRevision] = tc.template
			dep.Status.Conditions = nil
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-0", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: tc.template}}}
			cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
				WithObjects(app, dep, pod).WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
			if _, err := r.reportRolloutProgress(ctx, app, dep, 1, 3000, "waiting"); err != nil {
				t.Fatalf("reportRolloutProgress: %v", err)
			}
			var stored appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
				t.Fatal(err)
			}
			if stored.Status.Phase != tc.want {
				t.Errorf("phase = %q, want %q", stored.Status.Phase, tc.want)
			}
			if ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady); ready == nil || ready.Status != metav1.ConditionFalse {
				t.Errorf("Ready = %+v, want False", ready)
			}
			// A Save during it applies nothing unless a newer release rolls.
			if got := releaseRequestedOrRolling(&stored, stored.Generation+1); got != tc.wantApplyable {
				t.Errorf("releaseRequestedOrRolling = %v, want %v", got, tc.wantApplyable)
			}
		})
	}
}

// A spec write that leaves a Running App on its live release (a scale, a
// domain, a setting that waits for a deploy) rolls nothing, so it never
// stamps Deploying, during that release's outage or not (w4/m327). A newer
// release, or a new image, does.
func TestRolloutPendingOnlyForANewRelease(t *testing.T) {
	for _, tc := range []struct {
		name     string
		active   string
		release  int64
		template string
		image    string
		want     bool
	}{
		{"a spec write on the live release", "rev-3", 3, "rev-3", "busybox:1.37", false},
		{"a newer release", "rev-2", 3, "rev-3", "busybox:1.37", true},
		{"a template on the live release behind a pending one", "rev-3", 4, "rev-3", "busybox:1.37", true},
		{"a new image", "rev-3", 3, "rev-3", "busybox:1.38", true},
	} {
		app := &appv1alpha1.App{
			ObjectMeta: metav1.ObjectMeta{Name: "web", Generation: 5},
			Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning, ObservedGeneration: 4,
				Image: "busybox:1.37", ActiveRevision: tc.active, ReleaseGeneration: tc.release},
		}
		dep := progressDeadlineDep("web")
		dep.Spec.Template.Labels[labelRevision] = tc.template
		if got := rolloutPending(app, dep, tc.image); got != tc.want {
			t.Errorf("%s: rolloutPending = %v, want %v", tc.name, got, tc.want)
		}
	}
}
