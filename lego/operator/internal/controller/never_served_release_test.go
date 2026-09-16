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
	"errors"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w1/m160. "A prior release exists" was decided from
// status.image, which a first release that never served still leaves behind: a
// crash-looping first release stamps the image before the readiness gate, while
// markRunning — the only writer of status.activeRevision — never runs. A failed
// or canceled build over that state then reported the service Running (or
// re-dispatched the image) for something that had never served a request.
// settleFailedRollout and failPreDeploy already keyed on activeRevision, and
// this milestone extracted that shared decision into releaseHasServed.
//
// The served-release control lives in failed_build_prior_release_phase_test.go
// (TestBuildFailureOverServingReleaseStaysRunning), which asserts more than a
// duplicate here would.

// neverServedApp is a first release that stamped its image and then never
// became active: status.image set, status.activeRevision empty.
func neverServedApp(namespace string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: namespace, Generation: 2},
		Spec:       appv1alpha1.AppSpec{Repo: "https://example.invalid/repo.git", Port: 3000},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseDeploying,
			Image: "ghcr.io/bex-co/api@sha256:neverserved",
			// ActiveRevision deliberately empty.
			ObservedGeneration: 1,
		},
	}
}

func TestBuildFailureOverNeverServedReleaseReportsFailed(t *testing.T) {
	ctx := context.Background()
	app := neverServedApp("tea-m160")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	if _, err := r.fail(ctx, app, appv1alpha1.ReasonBuildFailedUserError, errors.New("build broke")); err == nil {
		t.Fatal("fail must return the build error")
	}
	if app.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want %q — no release ever served, so there is none to keep serving",
			app.Status.Phase, appv1alpha1.PhaseFailed)
	}
}

// A cancel over a never-served first release must settle Canceled instead of
// re-dispatching the image that never served — the same decision the failure
// path makes above (w6/m52 set the Canceled rule; w1/m160 fixes which fact it
// reads).
func TestCancelOverNeverServedReleaseSettlesCanceled(t *testing.T) {
	ctx := context.Background()
	app := neverServedApp("tea-m160")
	app.Annotations = map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "2"}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	if _, err := r.settleCanceledRelease(ctx, app, 3000); err != nil {
		t.Fatalf("settleCanceledRelease: %v", err)
	}
	if app.Status.Phase != appv1alpha1.PhaseCanceled {
		t.Fatalf("phase = %q, want %q — a cancel with nothing ever served is Canceled, not a revert",
			app.Status.Phase, appv1alpha1.PhaseCanceled)
	}
}

// A background worker's autoscaling transition must reach Ended once its
// Deployment has the target replicas ready. Until w1/m160 only the web path
// completed one, so a worker's first scale left the transition Started forever
// and applyAutoscaling kept returning its recorded ToReplicas without ever
// reading metrics again (w1/105).
func TestAutoscaledWorkerEndsScalingTransition(t *testing.T) {
	ctx := context.Background()
	scheme := wakeScheme()
	app := heldWorkerApp("tea-m160")
	app.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 1, MaxReplicas: 3}
	cl := fake.NewClientBuilder().WithScheme(scheme).WithObjects(app).
		WithStatusSubresource(&appv1alpha1.App{}, &appsv1.Deployment{}).Build()
	r := wakeReconciler(cl, scheme)
	nn := types.NamespacedName{Name: app.Name, Namespace: app.Namespace}

	reconcileTwice(t, r, nn)
	markDeploymentRolledOut(t, cl, nn)

	var live appv1alpha1.App
	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	live.Status.Autoscaling = &appv1alpha1.AutoscalingStatus{
		TransitionID: "tr-m160", FromReplicas: 0, ToReplicas: 1,
		State:     appv1alpha1.AutoscalingTransitionStarted,
		StartedAt: time.Now().UTC().Format(time.RFC3339Nano),
	}
	if err := cl.Status().Update(ctx, &live); err != nil {
		t.Fatal(err)
	}

	reconcileTwice(t, r, nn)

	if err := cl.Get(ctx, nn, &live); err != nil {
		t.Fatal(err)
	}
	transition := live.Status.Autoscaling
	if transition == nil || transition.State != appv1alpha1.AutoscalingTransitionEnded {
		t.Fatalf("worker transition = %+v, want Ended once the target replicas are ready", transition)
	}
	if transition.FinishedAt == "" {
		t.Fatal("an ended transition must carry finishedAt")
	}
}
