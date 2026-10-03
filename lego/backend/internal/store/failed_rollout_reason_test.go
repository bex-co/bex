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

// w8/m44: a rollout that failed over a release that had served closed only at
// the health-gate timeout, as "did not become healthy … check the service
// logs", although the operator had named the image pull. The operator's
// ConditionRollout verdict now closes the row as soon as it is written, with
// its message.

const pullFailing = `image pull is failing: Back-off pulling image "docker.io/x/y:does-not-exist-999"`

func servingAppWithFailedRollout(gen int64, reason, msg string) *appv1alpha1.App {
	app := &appv1alpha1.App{}
	app.Generation = gen
	app.Status.Phase = appv1alpha1.PhaseRunning
	app.Status.ReleaseGeneration = gen
	app.Status.ActiveRevision = "rev-2"
	app.Status.Conditions = []metav1.Condition{
		{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
			Reason: appv1alpha1.ReasonPriorReleaseServing, ObservedGeneration: gen,
			Message: "the latest rollout failed: " + msg + "; the previously deployed release keeps serving"},
		{Type: appv1alpha1.ConditionRollout, Status: metav1.ConditionFalse, Reason: reason, Message: msg, ObservedGeneration: gen},
	}
	return app
}

func TestRolloutFailureOverServingReleaseClosesNowWithItsDiagnosis(t *testing.T) {
	for reason, wantCode := range map[string]string{
		"ImagePullBackOff":   EventReasonImagePullBackoff,
		"HealthCheckFailing": "",
	} {
		app := servingAppWithFailedRollout(3, reason, pullFailing)
		open := Deploy{Generation: 3, Status: DeployUpdateInProgress}
		// Not timed out: the verdict alone closes the row.
		if got := observedDeployStatus(open, app, false); got != DeployUpdateFailed {
			t.Fatalf("%s: status = %q, want %q before the gate timeout", reason, got, DeployUpdateFailed)
		}
		for _, matches := range []bool{true, false} {
			if got, code := deployCloseFailureReason(app, open, DeployUpdateFailed, matches); got != pullFailing || code != wantCode {
				t.Errorf("%s matches=%v: reason = (%q, %q), want (%q, %q)", reason, matches, got, code, pullFailing, wantCode)
			}
		}
	}
}

// Another release's verdict never names or closes this row.
func TestRolloutVerdictForAnotherReleaseIsIgnored(t *testing.T) {
	app := servingAppWithFailedRollout(3, "ImagePullBackOff", pullFailing)
	app.Status.ReleaseGeneration = 4
	app.Generation = 4
	open := Deploy{Generation: 4, Status: DeployUpdateInProgress}
	if got := observedDeployStatus(open, app, false); got == DeployUpdateFailed {
		t.Errorf("release 4's row closed on release 3's rollout verdict")
	}
	if got, _ := deployCloseFailureReason(app, open, DeployUpdateFailed, false); got != "" {
		t.Errorf("reason = %q, want none from release 3's verdict", got)
	}
}

// w4/m155: a static site's failed replacement publish is the same shape — the
// operator keeps the prior revision Running and records ConditionRollout with
// Reason PublishFailed. The row closes update_failed with the exact clone
// message, also after suspend/resume moved metadata.generation past the
// failed release (w6/m100), and a later recovered release reads live.
func TestStaticPublishFailureOverServedReleaseClosesUpdateFailed(t *testing.T) {
	const missing = `clone: the publish directory "examples/static-site/qa-r23-missing-again" does not exist in the repository at "main" (exit 2)`
	app := servingAppWithFailedRollout(4, "PublishFailed", missing)
	app.Status.ActiveRevision = "rev-3"
	app.Generation = 6 // suspend + resume after the failure
	open := Deploy{Generation: 4, Status: DeployUpdateInProgress}
	if got := observedDeployStatus(open, app, false); got != DeployUpdateFailed {
		t.Fatalf("status = %q, want %q", got, DeployUpdateFailed)
	}
	if got, code := deployCloseFailureReason(app, open, DeployUpdateFailed, false); got != missing || code != "" {
		t.Errorf("reason = (%q, %q), want (%q, \"\")", got, code, missing)
	}

	// Recovery: release 5 published and is active; release 4's verdict stays in
	// its slot but cannot fail the recovered row.
	app.Status.ReleaseGeneration = 5
	app.Status.ActiveRevision = "rev-5"
	app.Generation = 7
	app.Status.Conditions[0] = metav1.Condition{Type: appv1alpha1.ConditionReady, Status: metav1.ConditionTrue,
		Reason: "Published", ObservedGeneration: 7}
	recovered := Deploy{Generation: 5, Status: DeployUpdateInProgress}
	if got := observedDeployStatus(recovered, app, false); got != DeployLive {
		t.Errorf("recovered status = %q, want %q", got, DeployLive)
	}
}
