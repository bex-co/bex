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
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w1/m179: a failed first release stops at zero instead of restarting until the
// service is deleted (w8/039).

// statusDrainedComplete is what the deployment controller reports once a
// Deployment is scaled to zero: every (zero) replica is updated and available,
// so the rollout reads complete — the shape that must never mark the release
// Running.
var statusDrainedComplete = appsv1.DeploymentStatus{
	Conditions: []appsv1.DeploymentCondition{
		{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: "NewReplicaSetAvailable"},
		{Type: appsv1.DeploymentAvailable, Status: corev1.ConditionTrue, Reason: "MinimumReplicasAvailable"},
	},
}

func firstReleaseApp(serviceType, tier string) *appv1alpha1.App {
	app := activeApp(defaultAppsNamespace)
	app.Spec.Type, app.Spec.Tier = serviceType, tier
	app.Spec.Expose = serviceType == appv1alpha1.TypeWebService
	return app
}

// stopFailedFirstRelease rolls the fixture's first release, lets it pass its
// progress deadline with no ready pod, and runs the passes that settle it and
// stop it.
func stopFailedFirstRelease(t *testing.T, r *AppReconciler, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("setup: first rollout replicas = %d, want 1", got)
	}
	setDeploymentStatus(t, cl, nn, statusRolloutFailedAt(0, time.Now().Add(-time.Minute)))
	reconcileOnce(t, r, nn)
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseFailed {
		t.Fatalf("setup: settled phase = %q, want Failed", got)
	}
	reconcileOnce(t, r, nn)
}

func assertStoppedFailed(t *testing.T, cl client.Client, nn types.NamespacedName) {
	t.Helper()
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want 0: a failed first release must stop, not restart until deleted", got)
	}
	live := liveApp(t, cl, nn)
	if live.Status.Phase != appv1alpha1.PhaseFailed || live.Status.ActiveRevision != "" {
		t.Fatalf("phase %q activeRevision %q, want Failed and never served", live.Status.Phase, live.Status.ActiveRevision)
	}
	ready := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionReady)
	if ready == nil || ready.Status != "False" || ready.Reason != reasonProgressDeadlineExceeded || ready.ObservedGeneration != live.Generation {
		t.Fatalf("Ready = %+v, want the recorded failure for the current generation", ready)
	}
}

func TestFailedFirstReleaseStopsAtZero(t *testing.T) {
	for _, tc := range []struct{ name, serviceType, tier string }{
		{"private service", appv1alpha1.TypePrivateService, ""},
		{"background worker", appv1alpha1.TypeBackgroundWorker, ""},
		{"paid web service", appv1alpha1.TypeWebService, "starter"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r, cl, nn := lifecycleFixture(t, firstReleaseApp(tc.serviceType, tc.tier))
			stopFailedFirstRelease(t, r, cl, nn)
			assertStoppedFailed(t, cl, nn)

			// The drained Deployment now reads as a complete rollout. Reporting it
			// would mark a release that never served Running.
			setDeploymentStatus(t, cl, nn, statusDrainedComplete)
			reconcileTwice(t, r, nn)
			assertStoppedFailed(t, cl, nn)
		})
	}
}

// A free web service goes idle while failed; the stop keeps it Failed rather
// than parking it as Hibernated, which would read as a healthy sleep.
func TestFailedFirstReleaseOfAFreeServiceStaysFailed(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, firstReleaseApp(appv1alpha1.TypeWebService, "free"))
	stopFailedFirstRelease(t, r, cl, nn)
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	setDeploymentStatus(t, cl, nn, statusDrainedComplete)
	reconcileTwice(t, r, nn)
	assertStoppedFailed(t, cl, nn)
}

// A newer release moves the release generation past the verdict and rolls.
func TestNewerReleaseRollsAStoppedFirstRelease(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, firstReleaseApp(appv1alpha1.TypePrivateService, ""))
	stopFailedFirstRelease(t, r, cl, nn)

	deployImageAt(t, cl, nn, "nginx:2", 2)
	// The pass that writes the new template reads the old rollout's verdict:
	// the deployment controller has not observed the new generation yet. The
	// fake client never bumps generation, so the test does it.
	dep := liveDeployment(t, cl, nn)
	dep.Generation = dep.Status.ObservedGeneration + 1
	if err := cl.Update(context.Background(), &dep); err != nil {
		t.Fatal(err)
	}
	reconcileOnce(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("replicas = %d after a new deploy, want 1", got)
	}
	if got := appPhase(t, cl, nn); got == appv1alpha1.PhaseFailed {
		t.Fatalf("phase stayed Failed after a new deploy rolled")
	}
	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	if live := liveApp(t, cl, nn); live.Status.Phase != appv1alpha1.PhaseRunning || live.Status.ActiveRevision == "" {
		t.Fatalf("phase %q activeRevision %q, want the new release Running", live.Status.Phase, live.Status.ActiveRevision)
	}
}

// Suspend parks as usual; resume has no successful release to bring back, so
// the service returns to its stopped Failed state rather than restarting the
// failed pod.
func TestSuspendAndResumeKeepAStoppedFirstReleaseStopped(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, firstReleaseApp(appv1alpha1.TypePrivateService, ""))
	stopFailedFirstRelease(t, r, cl, nn)

	setSuspendedAt(t, cl, nn, true, 2)
	reconcileTwice(t, r, nn)
	// A suspend parks as every suspend does: the shared parked phase.
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated || deploymentReplicas(t, cl, nn) != 0 {
		t.Fatalf("phase = %q replicas %d while suspended, want the parked Hibernated at zero", got, deploymentReplicas(t, cl, nn))
	}
	setSuspendedAt(t, cl, nn, false, 3)
	reconcileTwice(t, r, nn)
	assertStoppedFailed(t, cl, nn)
}

// stopsFirstRelease decides from the Deployment the settle saw: only the
// current release, with no ready pod, of a service nothing served.
func TestStopsFirstReleaseOnlyForTheCurrentUnreadyRelease(t *testing.T) {
	app := &appv1alpha1.App{Status: appv1alpha1.AppStatus{ReleaseGeneration: 2}}
	current := progressDeadlineDep("web")
	current.Spec.Template.Labels[labelRevision] = "rev-2"
	if !stopsFirstRelease(app, current) {
		t.Fatal("a never-served current release with no ready pod must stop")
	}

	stale := current.DeepCopy()
	stale.Spec.Template.Labels[labelRevision] = "rev-1"
	if stopsFirstRelease(app, stale) {
		t.Fatal("a cached Deployment still on the previous release must not stop the new one")
	}
	partly := current.DeepCopy()
	partly.Status.ReadyReplicas = 1
	if stopsFirstRelease(app, partly) {
		t.Fatal("a release with a ready pod serves something; stopping it would take that away")
	}
	served := app.DeepCopy()
	served.Status.ActiveRevision = "rev-1"
	if stopsFirstRelease(served, current) {
		t.Fatal("over a served release the w1/m172 hold applies, not the stop")
	}
}

// The early permanent-pull settle reports Failed fast but does not stop the
// release: an auth error can still clear before the progress deadline.
func TestEarlyPullFailureSettleDoesNotStopAFirstRelease(t *testing.T) {
	ctx := context.Background()
	app := &appv1alpha1.App{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "default", Generation: 1},
		Status: appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseDeploying, ReleaseGeneration: 1}}
	dep := progressDeadlineDep("web")
	dep.Status.Conditions = nil
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-new", Namespace: "default",
		Labels: map[string]string{"app": "web", labelRevision: "rev-1"}, CreationTimestamp: metav1.NewTime(time.Now().Add(-2 * time.Minute))},
		Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{{Name: "app", Image: "example.org/web:1",
			State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ErrImagePull", Message: "unauthorized: authentication required"}}}}}}
	cl := fake.NewClientBuilder().WithScheme(rolloutFailScheme(t)).WithObjects(app, dep, pod).WithStatusSubresource(&appv1alpha1.App{}).Build()
	r := &AppReconciler{Client: cl, Scheme: cl.Scheme(), Mode: ModeKubernetes}
	if _, err := r.reportRolloutProgress(ctx, app, dep, 1, 3000, "waiting"); err != nil {
		t.Fatal(err)
	}
	var stored appv1alpha1.App
	if err := cl.Get(ctx, client.ObjectKeyFromObject(app), &stored); err != nil {
		t.Fatal(err)
	}
	if stored.Status.Phase != appv1alpha1.PhaseFailed {
		t.Fatalf("phase = %q, want the early pull failure reported", stored.Status.Phase)
	}
	if failedFirstRelease(&stored) {
		t.Fatalf("the early pull settle stopped the release: %+v", meta.FindStatusCondition(stored.Status.Conditions, appv1alpha1.ConditionRollout))
	}
}

// w1/132: a cancel over a rolling first release stops its pods too — nothing
// served, so the canceled release is all that would run — and a newer deploy
// rolls again.
func TestCanceledFirstReleaseStopsItsRollout(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, firstReleaseApp(appv1alpha1.TypePrivateService, ""))
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("setup: first rollout replicas = %d, want 1", got)
	}
	updateApp(t, cl, nn, func(a *appv1alpha1.App) {
		a.Annotations[appv1alpha1.AnnotationCanceledReleaseGeneration] = "1"
	})
	reconcileOnce(t, r, nn)
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseCanceled {
		t.Fatalf("phase = %q, want Canceled", got)
	}
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d after the cancel, want 0: the canceled first release must not keep running", got)
	}

	deployImageAt(t, cl, nn, "nginx:2", 2)
	dep := liveDeployment(t, cl, nn)
	dep.Generation = dep.Status.ObservedGeneration + 1
	if err := cl.Update(context.Background(), &dep); err != nil {
		t.Fatal(err)
	}
	reconcileOnce(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("replicas = %d after a new deploy, want 1", got)
	}
}
