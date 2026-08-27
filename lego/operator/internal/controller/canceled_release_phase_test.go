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

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestCanceledFirstReleaseReportsCanceledNotFailed is w6/m52's core guarantee.
// Canceling a service's very first deploy left the service reporting phase
// Failed while its own deploy row read "canceled" — a resource contradicting
// its own history, and a red error badge for something the user did on purpose.
// The Ready condition always knew better ("BuildCanceled"); only the coarse
// phase lied.
var errBuildBroke = errors.New("build exited 1")

func TestCanceledFirstReleaseReportsCanceledNotFailed(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 2},
		Spec:       appv1alpha1.AppSpec{Repo: "https://example.invalid/repo.git"},
		// Image "" is what makes this the first-ever deploy: no release has ever
		// succeeded, so there is nothing to fall back to.
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseBuilding, ObservedGeneration: 1},
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	if _, err := r.settleCanceledRelease(ctx, app, 3000); err != nil {
		t.Fatalf("settleCanceledRelease: %v", err)
	}

	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase == appv1alpha1.PhaseFailed {
		t.Fatal("a user-initiated cancel must not report Failed — that is what made the service contradict its own deploy row")
	}
	if stored.Status.Phase != appv1alpha1.PhaseCanceled {
		t.Fatalf("phase = %q, want %q", stored.Status.Phase, appv1alpha1.PhaseCanceled)
	}
	if stored.Status.ObservedGeneration != 2 {
		t.Fatalf("observedGeneration = %d, want the settled generation 2", stored.Status.ObservedGeneration)
	}
	// The condition already carried the right semantics before this milestone and
	// must keep doing so — it is what a client reads for the reason.
	cond := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if cond == nil || cond.Status != metav1.ConditionFalse || cond.Reason != "BuildCanceled" {
		t.Fatalf("Ready condition = %+v, want False/BuildCanceled", cond)
	}
}

// TestCanceledReleaseWithAPriorReleaseKeepsServing is the control case on the
// other branch: a cancel when an earlier release IS live must not touch the
// phase at all — that release keeps serving, exactly as the Cancel dialog
// promises ("The last successful deploy remains live"). This milestone only
// changes the no-release branch.
func TestCanceledReleaseWithAPriorReleaseKeepsServing(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
		Spec:       appv1alpha1.AppSpec{Repo: "https://example.invalid/repo.git"},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning, Image: "registry.example/app@sha256:live",
			ActiveRevision: "rev-2", ObservedGeneration: 2,
		},
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	// dispatchRuntime is what this branch calls; it needs more of the cluster
	// than this unit fixture provides, so assert the branch choice rather than
	// its outcome: the no-release terminal must not have been written.
	_, _ = r.settleCanceledRelease(ctx, app, 3000)
	if app.Status.Phase == appv1alpha1.PhaseCanceled {
		t.Fatal("a cancel with a live prior release must keep serving it, not report Canceled")
	}
	cond := meta.FindStatusCondition(app.Status.Conditions, appv1alpha1.ConditionReady)
	if cond != nil && cond.Reason == "BuildCanceled" {
		t.Fatal("the no-release cancel terminal must not be written when a release is live")
	}
}

// TestGenuineFailureStillReportsFailed is the adjacent class this fix must not
// blur: a real build or deploy error keeps the red Failed badge. Without this,
// "Canceled" could quietly swallow actual failures.
func TestGenuineFailureStillReportsFailed(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 2},
		Spec:       appv1alpha1.AppSpec{Repo: "https://example.invalid/repo.git"},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseBuilding},
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}

	if _, err := r.fail(ctx, app, "BuildFailed", errBuildBroke); err == nil {
		t.Fatal("fail must propagate the underlying error")
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want %q — a genuine failure is still a failure", stored.Status.Phase, appv1alpha1.PhaseFailed)
	}
}

// TestImageBackedCancelSettlesToCanceled is the w6/m104 regression on the
// no-prior-release branch for an IMAGE-backed App — the exact shape that stayed
// stuck. Nothing in the canceled-release path is repo-gated: given the stamp the
// backend now writes for image sources too, prepareAppReleaseDecision must reach
// the canceled branch and settleCanceledRelease must then report PhaseCanceled,
// not leave the service Deploying against a crash-looping image forever.
func TestImageBackedCancelSettlesToCanceled(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker", Namespace: "default", Generation: 2,
			Annotations: map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "2"},
		},
		// Image-backed (Repo == ""): the source kind that never reached settle
		// before m104. Image "" on Status makes this a first-ever deploy.
		Spec:   appv1alpha1.AppSpec{Image: "docker.io/library/nginx:alpine"},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, ObservedGeneration: 1},
	}
	if decision := prepareAppReleaseDecision(app); !decision.canceled {
		t.Fatal("image-backed App with a canceled-release stamp must yield a canceled decision — the operator half of the m104 fix")
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
	if _, err := r.settleCanceledRelease(ctx, app, 3000); err != nil {
		t.Fatalf("settleCanceledRelease: %v", err)
	}
	if app.Status.Phase != appv1alpha1.PhaseCanceled {
		t.Fatalf("phase = %q, want %q — an image-backed first-deploy cancel must settle Canceled", app.Status.Phase, appv1alpha1.PhaseCanceled)
	}
}

// TestImageBackedCancelWithPriorReleaseKeepsServing is the other m104 branch for
// an image-backed App: a cancel when an earlier release IS live must revert to
// it, never report Canceled — matching the Cancel dialog's "The last successful
// deploy remains live." settleCanceledRelease dispatches Status.Image; assert the
// branch choice (the no-release terminal is NOT written) as the repo-backed
// control does, since dispatchRuntime needs more cluster than this fixture has.
func TestImageBackedCancelWithPriorReleaseKeepsServing(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{
			Name: "worker", Namespace: "default", Generation: 3,
			Annotations: map[string]string{appv1alpha1.AnnotationCanceledReleaseGeneration: "3"},
		},
		Spec: appv1alpha1.AppSpec{Image: "docker.io/library/nginx:alpine"}, // image-backed
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseRunning, Image: "docker.io/library/redis:alpine@sha256:live",
			ActiveRevision: "rev-2", ObservedGeneration: 2,
		},
	}
	if decision := prepareAppReleaseDecision(app); !decision.canceled {
		t.Fatal("image-backed App with a canceled-release stamp must yield a canceled decision")
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme()}
	_, _ = r.settleCanceledRelease(ctx, app, 3000)
	if app.Status.Phase == appv1alpha1.PhaseCanceled {
		t.Fatal("an image-backed cancel with a live prior release must keep serving it, not report Canceled")
	}
}
