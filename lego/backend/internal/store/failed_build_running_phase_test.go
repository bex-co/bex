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

package store

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// These tests pin w6/m124's backend half: the deploy row's build_failed verdict
// and failure message come from the durable Build condition (w6/m100), NOT from
// the App reaching PhaseFailed — because a build failure over a still-serving
// release now leaves the phase Running, and the row must keep closing
// build_failed regardless.

// servingAppWithFailedBuild is the m124 fixture: the previous release (rev-2)
// keeps serving, the operator recorded gen-3's build failure on the Build
// condition, restored phase Running, and Ready describes the serving release.
func servingAppWithFailedBuild(gen int64, msg string) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Generation = gen
	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.ReleaseGeneration = gen
	app.Status.ActiveRevision = "rev-2"
	app.Status.Image = "registry.example/app@sha256:live"
	app.Status.Conditions = []metav1.Condition{
		{
			Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
			Reason: appv1alpha1.ReasonPriorReleaseServing, ObservedGeneration: gen,
		},
		{
			Type: appv1alpha1.ConditionBuild, Status: metav1.ConditionFalse,
			Reason: appv1alpha1.ReasonBuildFailedUserError, Message: msg,
			ObservedGeneration: gen,
		},
	}
	return app
}

func TestObservedDeployStatusBuildVerdictSurvivesRunningPhase(t *testing.T) {
	gen := int64(3)
	app := servingAppWithFailedBuild(gen, "COPY failed: Dockerfile.nope not found")

	for _, status := range []string{DeployQueued, DeployBuildInProgress} {
		if got := observedDeployStatus(Deploy{Generation: gen, Status: status}, app, false); got != DeployBuildFailed {
			t.Errorf("row %q with phase Running + Build condition => %q, want %q (the verdict must not depend on PhaseFailed)",
				status, got, DeployBuildFailed)
		}
	}
}

// TestObservedDeployStatusStaleBuildConditionDoesNotSettleNewerDeploy: a Build
// condition recorded for an EARLIER release must not close the deploy row of a
// newer one — the generation attribution is the whole point of w6/m100.
func TestObservedDeployStatusStaleBuildConditionDoesNotSettleNewerDeploy(t *testing.T) {
	gen := int64(5)
	app := servingAppWithFailedBuild(gen, "old failure")
	// The condition belongs to the previous release's build, the open row to a
	// fresh deploy the operator has not reported on yet.
	app.Status.Conditions[1].ObservedGeneration = gen - 1

	if got := observedDeployStatus(Deploy{Generation: gen, Status: DeployQueued}, app, false); got == DeployBuildFailed {
		t.Error("a stale Build condition must not settle a newer deploy as build_failed")
	}
}

func TestDeployCloseFailureReasonPrefersBuildCondition(t *testing.T) {
	gen := int64(3)
	msg := "COPY failed: Dockerfile.nope not found"
	app := servingAppWithFailedBuild(gen, msg)
	open := Deploy{Generation: gen, Status: DeployBuildInProgress}

	// The Build condition wins whether or not the release still matches: with
	// the phase held Running (w6/m124) the Ready condition describes the serving
	// release, so it is no longer a source for the build error.
	for _, matches := range []bool{true, false} {
		got, code := deployCloseFailureReason(app, open, DeployBuildFailed, matches)
		if got != msg || code != "" {
			t.Errorf("matches=%v: reason = (%q, %q), want the Build condition's message", matches, got, code)
		}
	}
}

func TestDeployCloseFailureReasonFallbacks(t *testing.T) {
	gen := int64(3)
	// Pre-m100 operator: no Build condition; the failure lives on the current
	// generation's Ready condition.
	legacy := &appv1alpha1.App{}
	legacy.Generation = gen
	legacy.Status.Phase = appv1alpha1.PhaseFailed
	legacy.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: appv1alpha1.ReasonBuildFailed, Message: "clone failed", ObservedGeneration: gen,
	}}
	open := Deploy{Generation: gen, Status: DeployBuildInProgress}

	if got, _ := deployCloseFailureReason(legacy, open, DeployBuildFailed, true); got != "clone failed" {
		t.Errorf("legacy matching close = %q, want the Ready condition's message", got)
	}
	if got, _ := deployCloseFailureReason(legacy, open, DeployBuildFailed, false); got != timedOutDeployReason(DeployBuildFailed) {
		t.Errorf("legacy superseded close = %q, want the build-window line", got)
	}

	// Non-build failing closes keep the matching-generation Ready sourcing.
	crash := &appv1alpha1.App{}
	crash.Generation = gen
	crash.Status.Phase = appv1alpha1.PhaseFailed
	crash.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: "CrashLoopBackOff", Message: "container crashed", ObservedGeneration: gen,
	}}
	if got, _ := deployCloseFailureReason(crash, open, DeployUpdateFailed, true); got != "container crashed" {
		t.Errorf("update_failed matching close = %q, want the Ready condition's message", got)
	}
	if got, _ := deployCloseFailureReason(crash, open, DeployUpdateFailed, false); got != "" {
		t.Errorf("update_failed superseded close = %q, want empty", got)
	}
}

// TestObservedServiceStateServingAfterFailedBuild pins the availability half:
// the m124 terminal (phase Running, Ready=True/PriorReleaseServing) reads as a
// healthy serving service, not as an instance that stopped passing readiness
// checks — the misleading event w6/m97 catalogued is simply never produced for
// a service whose prior release kept serving.
func TestObservedServiceStateServingAfterFailedBuild(t *testing.T) {
	app := servingAppWithFailedBuild(3, "COPY failed")
	obs := observedServiceStateFor("app-1", app, false)
	if obs.ServicePhase != string(appv1alpha1.PhaseRunning) {
		t.Errorf("service phase = %q, want Running", obs.ServicePhase)
	}
	if !obs.AvailabilityObserved || obs.Availability != "healthy" {
		t.Errorf("availability = (%q, observed=%v), want healthy — the serving release is what the service reports", obs.Availability, obs.AvailabilityObserved)
	}
	if obs.ReasonCode != "" {
		t.Errorf("reason code = %q, want none (no readiness_failed for a serving service)", obs.ReasonCode)
	}
}
