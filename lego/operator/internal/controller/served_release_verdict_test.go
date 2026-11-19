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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w32/042: bex-api keys the served release's image_pull_failed by the Rollout
// condition's transition time. The operator only ever wrote that condition
// False, so after an App's first failed rollout the time never moved again:
// a release that could not pull, recovered, and could not pull again weeks
// later wrote the same key twice, and the second outage had no event, webhook
// or email. The recovery now drops the release's own verdict, so the next
// failure is written with a transition time of its own.

const servedVerdictImage = "zot.bex-registry.svc:5000/ws/web:gen-3@sha256:aaaa"

// servedVerdictPod is an instance of the served release (rev-3), created five
// minutes ago, either unable to pull its image or ready.
func servedVerdictPod(pulls bool) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: "web-a", Namespace: "default", CreationTimestamp: metav1.NewTime(time.Now().Add(-5 * time.Minute)),
		Labels: map[string]string{"app": "web", labelRevision: "rev-3"},
	}}
	if pulls {
		pod.Status = corev1.PodStatus{
			Phase:             corev1.PodRunning,
			Conditions:        []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionTrue}},
			ContainerStatuses: []corev1.ContainerStatus{{Name: appContainerName, Image: servedVerdictImage, Ready: true, State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}},
		}
		return pod
	}
	pod.Status = corev1.PodStatus{
		Phase:      corev1.PodPending,
		Conditions: []corev1.PodCondition{{Type: corev1.PodReady, Status: corev1.ConditionFalse}},
		ContainerStatuses: []corev1.ContainerStatus{{Name: appContainerName, Image: servedVerdictImage, State: corev1.ContainerState{
			Waiting: &corev1.ContainerStateWaiting{Reason: "ImagePullBackOff",
				Message: `Back-off pulling image "` + servedVerdictImage + `": ErrImagePull: rpc error: code = NotFound desc = failed to resolve image: not found`},
		}}},
	}
	return pod
}

// servedVerdictDep is the served release's Deployment, with its one instance
// ready or not.
func servedVerdictDep(ready bool) *appsv1.Deployment {
	one := int32(1)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 4},
		Spec: appsv1.DeploymentSpec{
			Replicas: &one,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}},
			Template: corev1.PodTemplateSpec{ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "web", labelRevision: "rev-3"}}},
		},
		Status: appsv1.DeploymentStatus{ObservedGeneration: 4, Replicas: 1, UpdatedReplicas: 1},
	}
	if ready {
		dep.Status.ReadyReplicas, dep.Status.AvailableReplicas = 1, 1
	}
	return dep
}

func TestServedReleaseRecoveryGivesTheNextPullFailureItsOwnTransition(t *testing.T) {
	ctx := context.Background()
	// Release 3 serves. Release 2's rollout failed three weeks ago, and its
	// verdict is the Rollout condition the App still carries.
	earlier := metav1.NewTime(time.Now().Add(-21 * 24 * time.Hour).Truncate(time.Second))
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
		Spec:       appv1alpha1.AppSpec{Image: "registry.example/web:v3"},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning, ActiveRevision: "rev-3", ReleaseGeneration: 3, ObservedGeneration: 3,
			Conditions: []metav1.Condition{
				{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue, Reason: "Deployed", Message: "1/1 replicas ready",
					ObservedGeneration: 3, LastTransitionTime: earlier},
				{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: reasonProgressDeadlineExceeded,
					Message: "rollout did not become healthy", ObservedGeneration: 2, LastTransitionTime: earlier},
			},
		},
	}
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).
		WithObjects(app, servedVerdictDep(false), servedVerdictPod(false)).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes, Registry: "zot.bex-registry.svc:5000"}
	stored := func() *appv1alpha1.App {
		t.Helper()
		var a appv1alpha1.App
		if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &a); err != nil {
			t.Fatal(err)
		}
		return &a
	}
	setPod := func(pulls bool) {
		t.Helper()
		pod := servedVerdictPod(pulls)
		var cur corev1.Pod
		if err := cl.Get(ctx, client.ObjectKeyFromObject(pod), &cur); err != nil {
			t.Fatal(err)
		}
		cur.Status = pod.Status
		if err := cl.Status().Update(ctx, &cur); err != nil {
			t.Fatal(err)
		}
	}
	pullFails := func() *metav1.Condition {
		t.Helper()
		setPod(false)
		if _, err := r.reportRolloutProgress(ctx, stored(), servedVerdictDep(false), 1, 3000, "waiting"); err != nil {
			t.Fatalf("reportRolloutProgress: %v", err)
		}
		verdict := meta.FindStatusCondition(stored().Status.Conditions, appv1alpha1.ConditionRollout)
		if verdict == nil || verdict.Status != metav1.ConditionFalse || verdict.Reason != imagePullBackOff || verdict.ObservedGeneration != 3 {
			t.Fatalf("Rollout = %+v, want the served release's ImagePullBackOff at release generation 3", verdict)
		}
		return verdict
	}

	// The first outage: SetStatusCondition keeps the transition of the False
	// condition already there, release 2's.
	first := pullFails()
	if !first.LastTransitionTime.Equal(&earlier) {
		t.Fatalf("first verdict's transition = %s, want release 2's %s kept", first.LastTransitionTime, earlier)
	}

	// The pull recovers: the instance is ready again and the release settles
	// Running. The verdict describes an outage that is over.
	setPod(true)
	if err := r.markRunning(ctx, stored(), servedVerdictDep(true), 1, "app running (kubernetes)"); err != nil {
		t.Fatalf("markRunning: %v", err)
	}
	recovered := stored()
	if verdict := meta.FindStatusCondition(recovered.Status.Conditions, appv1alpha1.ConditionRollout); verdict != nil {
		t.Fatalf("Rollout = %+v after the served release recovered, want it dropped", verdict)
	}
	if ready := meta.FindStatusCondition(recovered.Status.Conditions, appv1alpha1.ConditionReady); recovered.Status.Phase != appv1alpha1.PhaseRunning ||
		ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != "Deployed" {
		t.Fatalf("phase %q, Ready %+v; want Running and Deployed", recovered.Status.Phase, ready)
	}

	// The second outage is written afresh, with its own transition time:
	// bex-api's key for it differs from the first's.
	secondAt := time.Now().Truncate(time.Second)
	second := pullFails()
	if second.LastTransitionTime.Time.Before(secondAt) {
		t.Fatalf("second verdict's transition = %s, want the second outage's (from %s), not the first's %s",
			second.LastTransitionTime, secondAt, first.LastTransitionTime)
	}
}

// Only the served release's own verdict goes when it is ready: a release that
// has not served keeps its own, and an earlier release's stays for bex-api to
// close that release's deploy row.
func TestReadyReleaseKeepsVerdictsThatAreNotItsOwnRecovery(t *testing.T) {
	for name, tc := range map[string]struct {
		active  string
		verdict int64
	}{
		"an earlier release's verdict":      {active: "rev-3", verdict: 2},
		"a release that had not served yet": {active: "rev-2", verdict: 3},
	} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			app := &appv1alpha1.App{
				ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
				Status: appv1alpha1.AppStatus{
					Phase: appv1alpha1.PhaseRunning, ActiveRevision: tc.active, ReleaseGeneration: 3,
					Conditions: []metav1.Condition{{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse,
						Reason: imagePullBackOff, Message: "image pull failed", ObservedGeneration: tc.verdict, LastTransitionTime: metav1.Now()}},
				},
			}
			cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
			r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
			if err := r.markRunning(ctx, app, servedVerdictDep(true), 1, "app running (kubernetes)"); err != nil {
				t.Fatal(err)
			}
			var stored appv1alpha1.App
			if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
				t.Fatal(err)
			}
			if verdict := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionRollout); verdict == nil || verdict.ObservedGeneration != tc.verdict {
				t.Fatalf("Rollout = %+v, want release %d's verdict kept", verdict, tc.verdict)
			}
		})
	}
}
