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

// w1/m149: a failed pre-deploy row closes with the operator's durable
// status.preDeploy message. At filing time an immediate `exit 3` closed with
// "the pre-deploy command did not finish within its window", because the
// reason was read only from a Ready condition whose observedGeneration had to
// equal metadata.generation.

const preDeployExit3 = "the pre-deploy command exited with code 3; check the pre-deploy logs"

// servingAppWithFailedPreDeploy: release 3's pre-deploy step failed, the prior
// release (rev-2) keeps serving, and the operator held the phase Running.
func servingAppWithFailedPreDeploy(gen int64) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Generation = gen
	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.ReleaseGeneration = gen
	app.Status.ActiveRevision = "rev-2"
	app.Status.Image = "registry.example/app@sha256:live"
	app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
		Job: "predeploy-web-gen-3", Generation: gen, Status: appv1alpha1.PreDeployFailed, Message: preDeployExit3,
	}
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
		Reason: appv1alpha1.ReasonPriorReleaseServing, ObservedGeneration: gen,
	}}
	return app
}

func TestPreDeployFailureOverServingReleaseClosesWithItsMessage(t *testing.T) {
	gen := int64(3)
	app := servingAppWithFailedPreDeploy(gen)
	open := Deploy{Generation: gen, Status: DeployPreDeployInProgress}

	if got := observedDeployStatus(open, app, false); got != DeployPreDeployFailed {
		t.Fatalf("status = %q, want %q with the phase held Running", got, DeployPreDeployFailed)
	}
	for _, matches := range []bool{true, false} {
		if got, code := deployCloseFailureReason(app, open, DeployPreDeployFailed, matches); got != preDeployExit3 || code != "" {
			t.Errorf("matches=%v: reason = (%q, %q), want the pre-deploy verdict %q", matches, got, code, preDeployExit3)
		}
	}
	obs := observedServiceStateFor("app-1", app, false)
	if obs.ServicePhase != string(appv1alpha1.PhaseRunning) || obs.Availability != "healthy" {
		t.Errorf("service = (%q, %q), want Running and healthy — the prior release keeps serving", obs.ServicePhase, obs.Availability)
	}
}

// The filing-time case: metadata.generation moved past the Ready condition that
// carried the failure, so the old lookup fell through to the timeout line.
func TestPreDeployFailureReasonSurvivesAGenerationBump(t *testing.T) {
	app := &appv1alpha1.App{}
	app.Generation = 4
	app.Status.Phase = appv1alpha1.PhaseFailed
	app.Status.ReleaseGeneration = 3
	app.Status.PreDeploy = &appv1alpha1.PreDeployStatus{
		Generation: 3, Status: appv1alpha1.PreDeployFailed, Message: preDeployExit3,
	}
	app.Status.Conditions = []metav1.Condition{{
		Type: appv1alpha1.ConditionReady, Status: metav1.ConditionFalse,
		Reason: appv1alpha1.ReasonPreDeployFailed, Message: preDeployExit3, ObservedGeneration: 3,
	}}
	open := Deploy{Generation: 3, Status: DeployPreDeployInProgress}

	got, _ := deployCloseFailureReason(app, open, DeployPreDeployFailed, true)
	if got != preDeployExit3 {
		t.Fatalf("reason = %q, want %q (never %q for an immediate exit)", got, preDeployExit3, timedOutDeployReason(DeployPreDeployFailed))
	}
}

// A verdict recorded for an earlier release must not name a newer row's failure.
func TestPreDeployVerdictForAnotherReleaseIsIgnored(t *testing.T) {
	app := servingAppWithFailedPreDeploy(3)
	open := Deploy{Generation: 4, Status: DeployPreDeployInProgress}

	if got, _ := deployCloseFailureReason(app, open, DeployPreDeployFailed, false); got != "" {
		t.Errorf("reason = %q, want none from release 3's verdict", got)
	}
}

// Controls: the build and rollout timeout lines are unchanged.
func TestTimedOutDeployReasonsUnchanged(t *testing.T) {
	for status, want := range map[string]string{
		DeployBuildFailed:     "the build did not finish within the build window; check the build logs",
		DeployPreDeployFailed: "the pre-deploy command did not finish within its window; check the pre-deploy logs",
		DeployUpdateFailed:    "the deploy did not become healthy within the health-gate window; check the service logs",
	} {
		if got := timedOutDeployReason(status); got != want {
			t.Errorf("%s: %q, want %q", status, got, want)
		}
	}
}
