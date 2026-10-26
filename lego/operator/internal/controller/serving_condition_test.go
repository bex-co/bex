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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

func TestServingRevisionIgnoresFailingReplacement(t *testing.T) {
	for _, tc := range []struct {
		name, active string
		ready        bool
		want         metav1.ConditionStatus
	}{
		{"healthy prior release", "rev-old", true, metav1.ConditionTrue},
		{"crashed serving release", "rev-old", false, metav1.ConditionFalse},
		{"first deploy", "", true, metav1.ConditionUnknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Generation: 2}, Status: appv1alpha1.AppStatus{ActiveRevision: tc.active}}
			dep := progressDeadlineDep("web")
			old := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "old", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: "rev-old"}}}
			if tc.ready {
				old.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			}
			replacement := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "new", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: "rev-1"}}, Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}}}}
			r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(old, replacement).Build()}
			r.observeServingRevision(context.Background(), app, dep, 1)
			c := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionServing)
			if c == nil || c.Status != tc.want || c.ObservedGeneration != 2 {
				t.Fatalf("serving = %+v, want %s", c, tc.want)
			}
			// A new observation must not retain a healthy condition after the pods vanish.
			if err := r.Delete(context.Background(), old); err != nil {
				t.Fatal(err)
			}
			r.observeServingRevision(context.Background(), app, dep, 1)
			if meta.IsStatusConditionTrue(app.Status.Conditions, appv1alpha1.ConditionServing) {
				t.Fatal("retained stale healthy signal")
			}
		})
	}
}

// While a new release builds on top of a live one (w1/m185), the build poll
// re-observes Serving: the live pod gone and its replacement never Ready reads
// False and reports a change exactly once, so the build path persists it.
func TestRefreshServingWhileBuildingSeesTheLivePodGone(t *testing.T) {
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 7},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseBuilding, ActiveRevision: "rev-old"}}
	dep := progressDeadlineDep("web")
	stuck := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-rescheduled", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: "rev-old"}},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff"}}}}}}
	r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(dep, stuck).Build()}

	if !r.refreshServingWhileBuilding(context.Background(), app) {
		t.Fatal("first observation reported no change")
	}
	c := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionServing)
	if c == nil || c.Status != metav1.ConditionFalse || c.Reason != appv1alpha1.ReasonServingRevisionUnavailable || c.ObservedGeneration != 7 {
		t.Fatalf("serving = %+v, want False/ServingRevisionUnavailable at generation 7", c)
	}
	if r.refreshServingWhileBuilding(context.Background(), app) {
		t.Error("an unchanged observation reported a change (would rewrite status every build poll)")
	}
}

// A first deploy has no live revision to observe, and an App whose
// Deployment is gone has nothing serving to read: neither touches Serving.
func TestRefreshServingWhileBuildingWithoutALiveRevision(t *testing.T) {
	r := &AppReconciler{Client: fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).Build()}
	first := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}}
	noDep := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default"}, Status: appv1alpha1.AppStatus{ActiveRevision: "rev-old"}}
	for name, app := range map[string]*appv1alpha1.App{"first deploy": first, "no deployment": noDep} {
		if r.refreshServingWhileBuilding(context.Background(), app) || meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionServing) != nil {
			t.Errorf("%s: Serving touched", name)
		}
	}
}

// TestRolloutProgressStampsActiveRevisionOncePodsServe is w4/405: when every
// pod of the rolling revision is Ready, the Service already routes to it, so
// the progress stamp records it as the active revision. A newer release that
// supersedes before the Deployment status converges then closes the older
// deploy's row live (it served), not canceled. A revision whose pods are not
// all ready stamps nothing.
func TestRolloutProgressStampsActiveRevisionOncePodsServe(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name  string
		ready bool
		want  string
	}{
		{"pods ready, deployment still converging", true, "rev-3"},
		{"pods not ready", false, ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
				Spec:       appv1alpha1.AppSpec{Image: "busybox:1.37"},
				Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, ReleaseGeneration: 3},
			}
			dep := progressDeadlineDep("web")
			dep.Spec.Template.Labels[labelRevision] = "rev-3"
			dep.Status.Conditions = nil // no progress-deadline verdict
			pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-0", Namespace: "default", Labels: map[string]string{"app": "web", labelRevision: "rev-3"}}}
			if tc.ready {
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}}
			}
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
			if stored.Status.ActiveRevision != tc.want {
				t.Fatalf("ActiveRevision = %q, want %q", stored.Status.ActiveRevision, tc.want)
			}
		})
	}
}
