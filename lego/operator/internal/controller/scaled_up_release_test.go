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
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w1/124. A free service woken onto a release that had never
// served stayed Deploying behind the activator for 2h 17m and never slept. The
// release was started by scaling its Deployment up, not by rolling it, and the
// deployment controller times only rollouts: the Deployment keeps the
// Progressing=True NewReplicaSetAvailable it reached at zero replicas, so no
// ProgressDeadlineExceeded ever arrives (measured on kind, Kubernetes 1.36).

// statusScaledUpUnavailableSince is that shape: one pod asked for and none
// ready since at, with Progressing still reading the completed rollout.
func statusScaledUpUnavailableSince(at time.Time) appsv1.DeploymentStatus {
	return appsv1.DeploymentStatus{
		Replicas: 1, UnavailableReplicas: 1,
		Conditions: []appsv1.DeploymentCondition{
			{Type: appsv1.DeploymentProgressing, Status: corev1.ConditionTrue, Reason: reasonNewReplicaSetAvailable},
			{
				Type: appsv1.DeploymentAvailable, Status: corev1.ConditionFalse, Reason: "MinimumReplicasUnavailable",
				LastTransitionTime: metav1.NewTime(at),
			},
		},
	}
}

// pastRolloutBudget is a moment the rollout budget, counted from it, has passed.
func pastRolloutBudget() time.Time {
	return time.Now().Add(-time.Duration(rolloutBudgetSeconds)*time.Second - time.Minute)
}

// A newer release scaled up on its own gets the verdict a rollout would have
// had once the budget passes, whether or not traffic keeps the service awake:
// the served release's template goes back and serves.
func TestScaledUpNewerReleaseSettlesFailedAfterTheBudget(t *testing.T) {
	app := activeApp("tea-124")
	r, cl, nn := lifecycleFixture(t, app)
	served := rollFailingReleaseTwo(t, r, cl, nn)
	stampLastActiveAt(t, cl, nn, time.Now())

	// Within the budget it is an ordinary start: no verdict yet.
	setDeploymentStatus(t, cl, nn, statusScaledUpUnavailableSince(time.Now().Add(-time.Minute)))
	reconcileTwice(t, r, nn)
	if got := deploymentTemplateRevision(t, cl, nn); got != "rev-2" {
		t.Fatalf("template revision within the budget = %q, want release 2 still starting", got)
	}
	if rollout := meta.FindStatusCondition(liveApp(t, cl, nn).Status.Conditions, appv1alpha1.ConditionRollout); rollout != nil && rollout.ObservedGeneration == 2 {
		t.Fatalf("Rollout = %+v within the budget, want no verdict for release 2 yet", rollout)
	}

	setDeploymentStatus(t, cl, nn, statusScaledUpUnavailableSince(pastRolloutBudget()))
	reconcileTwice(t, r, nn)
	assertServedReleaseHeld(t, cl, nn, served, "after the budget passed")

	setDeploymentStatus(t, cl, nn, statusRolledOut)
	reconcileTwice(t, r, nn)
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseRunning {
		t.Fatalf("phase = %q, want Running on the served release", got)
	}
	if got := ingressBackendName(t, cl, nn); got != app.Name {
		t.Fatalf("backend = %q, want the App's own Service %q", got, app.Name)
	}
}

// The filed symptom: idle, the service now parks on the served release instead
// of deferring the park for a verdict that never comes.
func TestIdleScaledUpNewerReleaseParksOnTheServedRelease(t *testing.T) {
	app := activeApp("tea-124")
	r, cl, nn := lifecycleFixture(t, app)
	served := rollFailingReleaseTwo(t, r, cl, nn)
	updateApp(t, cl, nn, func(a *appv1alpha1.App) { a.Annotations[annotReleaseWakeGeneration] = "2" })
	stampLastActiveAt(t, cl, nn, time.Now().Add(-time.Hour))
	setDeploymentStatus(t, cl, nn, statusScaledUpUnavailableSince(pastRolloutBudget()))

	reconcileTwice(t, r, nn)
	reconcileTwice(t, r, nn)
	if got := deploymentReplicas(t, cl, nn); got != 0 {
		t.Fatalf("replicas = %d, want parked", got)
	}
	if got := appPhase(t, cl, nn); got != appv1alpha1.PhaseHibernated {
		t.Fatalf("phase = %q, want Hibernated", got)
	}
	assertServedReleaseHeld(t, cl, nn, served, "parked")
}

// A first release resumed from a suspend starts the same way; past the budget
// it stops Failed, as its rollout would have (w1/m179).
func TestScaledUpFirstReleaseStopsFailedAfterTheBudget(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, firstReleaseApp(appv1alpha1.TypeWebService, "starter"))
	reconcileTwice(t, r, nn)
	setDeploymentStatus(t, cl, nn, statusScaledUpUnavailableSince(pastRolloutBudget()))
	reconcileTwice(t, r, nn)
	assertStoppedFailed(t, cl, nn)
}

// A served release whose pods go unready is not a rollout: it gets no deploy
// verdict, however long it stays down.
func TestUnreadyServedReleaseGetsNoRolloutVerdict(t *testing.T) {
	r, cl, nn := lifecycleFixture(t, firstReleaseApp(appv1alpha1.TypeWebService, "starter"))
	serveReleaseOne(t, r, cl, nn)
	setDeploymentStatus(t, cl, nn, statusScaledUpUnavailableSince(time.Now().Add(-time.Hour)))
	reconcileTwice(t, r, nn)

	live := liveApp(t, cl, nn)
	if rollout := meta.FindStatusCondition(live.Status.Conditions, appv1alpha1.ConditionRollout); rollout != nil && rollout.Status == metav1.ConditionFalse {
		t.Fatalf("Rollout = %+v, want no failed verdict for a release that served", rollout)
	}
	if live.Status.Phase == appv1alpha1.PhaseFailed || live.Status.ActiveRevision != "rev-1" {
		t.Fatalf("phase %q activeRevision %q, want release 1 still the served release", live.Status.Phase, live.Status.ActiveRevision)
	}
	if got := deploymentReplicas(t, cl, nn); got != 1 {
		t.Fatalf("replicas = %d, want 1", got)
	}
}
