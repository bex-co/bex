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

// These tests pin w1/m149: w6/m124's rule for builds applies to the pre-deploy
// step. The step runs before the rollout, so when it fails over a released image
// the previous release never stopped serving (at filing time the phase read
// Failed while the URL answered 200). status.preDeploy stays the durable verdict
// bex-api closes the deploy row from, so the deploy still reads
// pre_deploy_failed while the service reads Running.

const exitThree = "the pre-deploy command exited with code 3; check the pre-deploy logs"

var errTestNetworkPolicy = errors.New("network policy write refused")

func preDeployApp(image, activeRevision string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 3},
		Spec:       appv1alpha1.AppSpec{Image: "registry.example/app:v2", PreDeployCommand: "exit 3"},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseDeploying, Image: image, ActiveRevision: activeRevision,
			ObservedGeneration: 2, ReleaseGeneration: 3,
		},
	}
}

func failedPreDeployStep() *appv1alpha1.PreDeployStatus {
	return &appv1alpha1.PreDeployStatus{
		Job: "predeploy-web-gen-3", Generation: 3, Status: appv1alpha1.PreDeployFailed, Message: exitThree,
	}
}

func storedApp(t *testing.T, cl client.Client, app *appv1alpha1.App) appv1alpha1.App {
	t.Helper()
	var stored appv1alpha1.App
	if err := cl.Get(context.Background(), client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	return stored
}

func TestPreDeployFailureOverServingReleaseStaysRunning(t *testing.T) {
	ctx := context.Background()
	app := preDeployApp("registry.example/app@sha256:live", "rev-2")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	_, halt, err := r.failPreDeploy(ctx, app, failedPreDeployStep())
	if !halt || err == nil || err.Error() != exitThree {
		t.Fatalf("failPreDeploy = halt %v, err %v; want the rollout blocked with the step's message", halt, err)
	}

	stored := storedApp(t, cl, app)
	if stored.Status.Phase != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want %q — the previous release never stopped serving", stored.Status.Phase, appv1alpha1.PhaseRunning)
	}
	pd := stored.Status.PreDeploy
	if pd == nil || pd.Status != appv1alpha1.PreDeployFailed || pd.Generation != 3 || pd.Message != exitThree {
		t.Fatalf("status.preDeploy = %+v, want the durable failed verdict for release 3", pd)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != metav1.ConditionTrue || ready.Reason != appv1alpha1.ReasonPriorReleaseServing {
		t.Fatalf("Ready condition = %+v, want True/PriorReleaseServing", ready)
	}
	if stored.Status.Image != "registry.example/app@sha256:live" || stored.Status.ObservedGeneration != 2 {
		t.Fatalf("serving release moved: image %q observedGeneration %d", stored.Status.Image, stored.Status.ObservedGeneration)
	}
}

// The active revision alone marks a serving release (an image-backed App's
// status.image may be empty).
func TestPreDeployFailureOverActiveRevisionStaysRunning(t *testing.T) {
	ctx := context.Background()
	app := preDeployApp("", "rev-2")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	_, _, _ = r.failPreDeploy(ctx, app, failedPreDeployStep())

	if got := storedApp(t, cl, app).Status.Phase; got != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want %q", got, appv1alpha1.PhaseRunning)
	}
}

func TestPreDeployFailureOverParkedReleaseStaysHibernated(t *testing.T) {
	ctx := context.Background()
	app := preDeployApp("registry.example/app@sha256:live", "rev-2")
	zero := int32(0)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: app.Name, Namespace: app.Namespace},
		Spec:       appsv1.DeploymentSpec{Replicas: &zero},
	}
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app, dep).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	_, _, _ = r.failPreDeploy(ctx, app, failedPreDeployStep())

	stored := storedApp(t, cl, app)
	if stored.Status.Phase != appv1alpha1.PhaseHibernated {
		t.Fatalf("phase = %q, want %q — a parked release must not be reported running", stored.Status.Phase, appv1alpha1.PhaseHibernated)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Message != "the latest pre-deploy command failed; the previously deployed release stays parked" {
		t.Fatalf("Ready condition = %+v, want the parked prior-release message", ready)
	}
}

// A first release that crash-looped leaves status.image behind without ever
// serving; a pre-deploy failure on the next deploy must not read Running.
func TestPreDeployFailureAfterANeverServedReleaseReportsFailed(t *testing.T) {
	ctx := context.Background()
	app := preDeployApp("registry.example/app@sha256:crashlooped", "")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	_, _, _ = r.failPreDeploy(ctx, app, failedPreDeployStep())

	if got := storedApp(t, cl, app).Status.Phase; got != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want %q — nothing ever served", got, appv1alpha1.PhaseFailed)
	}
}

// With nothing released there is nothing serving, so Failed is the truthful
// phase — and the Ready condition carries the step's message.
func TestFirstReleasePreDeployFailureReportsFailed(t *testing.T) {
	ctx := context.Background()
	app := preDeployApp("", "")
	cl := fake.NewClientBuilder().WithScheme(deletionScheme(t)).
		WithObjects(app).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}

	_, halt, err := r.failPreDeploy(ctx, app, failedPreDeployStep())
	if !halt || err == nil {
		t.Fatalf("failPreDeploy = halt %v, err %v; want the rollout blocked", halt, err)
	}

	stored := storedApp(t, cl, app)
	if stored.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want %q — a first release has nothing to fall back to", stored.Status.Phase, appv1alpha1.PhaseFailed)
	}
	ready := meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Reason != appv1alpha1.ReasonPreDeployFailed || ready.Message != exitThree {
		t.Fatalf("Ready condition = %+v, want False/PreDeployFailed with the step's message", ready)
	}
}

// A step that could not start is an infrastructure fault, never worded as the
// command's own exit.
func TestPreDeployNotStartedIsNotACommandFailure(t *testing.T) {
	msg := preDeployNotStarted(errTestNetworkPolicy)
	if msg != "the pre-deploy command could not be started: network policy write refused" {
		t.Fatalf("message = %q", msg)
	}
}
