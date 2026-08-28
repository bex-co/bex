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

	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w6/m124: a build failure on top of a released image is a
// deploy fact, not a service outage. The service kept serving the previous
// release the whole time (the rollout below the failed build never ran), so
// the coarse phase must keep describing that release — the same rule the
// cancel path applies (w6/m52, PhaseCanceled's doc). The Build condition stays
// the durable verdict bex-api closes the deploy row from, so the deploy still
// reads build_failed while the service reads Running.

func failedBuildApp(image string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
		Spec:       appv1alpha1.AppSpec{Repo: "https://example.invalid/repo.git"},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseBuilding, Image: image,
			ActiveRevision: "rev-2", ObservedGeneration: 2, ReleaseGeneration: 3,
		},
	}
}

func TestBuildFailureOverServingReleaseStaysRunning(t *testing.T) {
	ctx := context.Background()
	app := failedBuildApp("registry.example/app@sha256:live")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	if _, err := r.fail(ctx, app, appv1alpha1.ReasonBuildFailedUserError, errBuildBroke); err == nil {
		t.Fatal("fail must still propagate the underlying error")
	}

	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want %q — the previous release never stopped serving", stored.Status.Phase, appv1alpha1.PhaseRunning)
	}
	build := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionBuild)
	if build == nil || build.Reason != appv1alpha1.ReasonBuildFailedUserError || build.Message != errBuildBroke.Error() {
		t.Fatalf("Build condition = %+v, want the durable build verdict with its message", build)
	}
	if build.ObservedGeneration != 3 {
		t.Fatalf("Build condition generation = %d, want the release generation 3 (w6/m100 attribution)", build.ObservedGeneration)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != appv1alpha1.ReasonPriorReleaseServing {
		t.Fatalf("Ready condition = %+v, want True/PriorReleaseServing — the failed build must not read as an unhealthy instance", ready)
	}
	if stored.Status.ObservedGeneration != 2 {
		t.Fatalf("observedGeneration = %d, want 2 — a failed build must never advance it", stored.Status.ObservedGeneration)
	}
	if stored.Status.Image != "registry.example/app@sha256:live" {
		t.Fatalf("image = %q, the serving release's image must be untouched", stored.Status.Image)
	}
}

// TestBuildFailureOverParkedReleaseStaysHibernated: a prior release that is
// parked at 0 replicas (free-tier auto-sleep, manual suspension) is sleeping,
// not running — w6/m116–m119's territory, which this fix must not sweep into
// "running". The Deployment's desired scale is the mechanism's own record of
// the parked state.
func TestBuildFailureOverParkedReleaseStaysHibernated(t *testing.T) {
	ctx := context.Background()
	app := failedBuildApp("registry.example/app@sha256:live")
	zero := int32(0)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace},
		Spec:       appsv1.DeploymentSpec{Replicas: &zero},
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app, dep).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	_, _ = r.fail(ctx, app, appv1alpha1.ReasonBuildFailed, errBuildBroke)

	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseHibernated {
		t.Fatalf("phase = %q, want %q — a parked release must not be reported running", stored.Status.Phase, appv1alpha1.PhaseHibernated)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != appv1alpha1.ReasonAutoHibernated {
		t.Fatalf("Ready condition = %+v, want False/AutoHibernated", ready)
	}
	if build := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionBuild); build == nil {
		t.Fatal("the durable Build verdict must still be recorded for the parked case")
	}
}

// TestFirstBuildFailureStillReportsFailed guards the over-correction: with no
// released image there is nothing serving, so Failed is the truthful phase.
// (TestGenuineFailureStillReportsFailed covers the same branch through the
// generic reason; this pins it against the gated build-failure reasons.)
func TestFirstBuildFailureStillReportsFailed(t *testing.T) {
	ctx := context.Background()
	app := failedBuildApp("")
	app.Status.ActiveRevision = ""
	app.Status.ObservedGeneration = 0
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	_, _ = r.fail(ctx, app, appv1alpha1.ReasonBuildFailedUserError, errBuildBroke)

	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want %q — a first-deploy build failure has nothing to fall back to", stored.Status.Phase, appv1alpha1.PhaseFailed)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionFalse || ready.Reason != appv1alpha1.ReasonBuildFailedUserError {
		t.Fatalf("Ready condition = %+v, want False with the failure reason", ready)
	}
}

// TestNonBuildFailureKeepsFailedPhase scopes the gate: only a BUILD failure is
// a pure deploy fact. Every other r.fail reason (ingress, disk, publish, …)
// keeps its existing terminal, whether or not an image exists.
func TestNonBuildFailureKeepsFailedPhase(t *testing.T) {
	ctx := context.Background()
	app := failedBuildApp("registry.example/app@sha256:live")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	_, _ = r.fail(ctx, app, "IngressFailed", errors.New("ingress write refused"))

	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want %q — non-build failures are out of the w6/m124 gate's scope", stored.Status.Phase, appv1alpha1.PhaseFailed)
	}
}
