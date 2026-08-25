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
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// unresolvable_dependency_test.go covers w7/m79/t002: carrying the operator's
// diagnosis of an unresolvable Secret/ConfigMap reference through to the deploy
// the user actually reads.
//
// The 2026-08-08 incident's first leg was a pod stuck in
// CreateContainerConfigError. The operator knew the exact missing object; the
// deploy closed with the generic health-gate timeout line. The cause existed, in
// the right place, and was discarded one step before anyone could see it.

func readyCondition(reason, message string, generation int64) appv1alpha1.App {
	return appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Generation: generation},
		Status: appv1alpha1.AppStatus{
			Phase: appv1alpha1.PhaseDeploying,
			Conditions: []metav1.Condition{{
				Type: "Ready", Status: metav1.ConditionFalse,
				Reason: reason, Message: message, ObservedGeneration: generation,
			}},
		},
	}
}

const missingSecretMessage = `container configuration cannot be resolved: secret "dpg-d9nqg9dcavls73fp8m2g-app" not found — ` +
	`a referenced Secret or ConfigMap must exist in this service's own namespace and carry the referenced key.`

func TestFailureReasonCarriesTheUnresolvableDependency(t *testing.T) {
	app := readyCondition("CreateContainerConfigError", missingSecretMessage, 7)

	reason, code := failureReasonFor(&app, DeployUpdateFailed)
	if reason != missingSecretMessage {
		t.Errorf("failure_reason = %q\nwant the operator's diagnosis naming the missing object", reason)
	}
	if strings.Contains(reason, "health-gate window") {
		t.Error("the deploy still closes with the generic timeout line — the diagnosis was discarded")
	}
	// No structured reason code: service_event_facts.reason_code is a CHECKed
	// closed vocabulary (migration 0043), and the deploy's free-text
	// failure_reason already carries the actionable detail. Adding a code here
	// would need a migration for no additional signal.
	if code != "" {
		t.Errorf("reason code = %q, want empty", code)
	}
}

// TestFailureReasonStillFallsBackWhenNothingWasDiagnosed pins the other half:
// a deploy that times out with no concrete diagnosis must keep its honest
// generic line rather than inventing a cause.
func TestFailureReasonStillFallsBackWhenNothingWasDiagnosed(t *testing.T) {
	app := readyCondition("RolloutProgressing", "waiting for the current Deployment revision", 3)

	reason, _ := failureReasonFor(&app, DeployUpdateFailed)
	if !strings.Contains(reason, "health-gate window") {
		t.Errorf("failure_reason = %q, want the generic health-gate line", reason)
	}
}

// TestConcreteContainerFailureClassification pins which Ready reasons count as a
// real failed instance while a deploy is still open. Getting this wrong is
// invisible in both directions: too narrow and the instance goes unobserved for
// the whole health-gate window; too wide and every ordinary rollout reports a
// failure, which trains users to ignore the signal.
func TestConcreteContainerFailureClassification(t *testing.T) {
	for _, reason := range []string{"ImagePullBackOff", "CrashLoopBackOff", "CreateContainerConfigError"} {
		if !concreteContainerFailure(reason) {
			t.Errorf("%s must count as a concrete container failure — the pod will never start", reason)
		}
	}
	for _, reason := range []string{
		"RolloutProgressing", "RolloutSettling", "Suspended", "AutoHibernated", "PreDeploy", "",
	} {
		if concreteContainerFailure(reason) {
			t.Errorf("%s is ordinary rollout progress and must not be reported as a failed instance", reason)
		}
	}
}

// TestUnresolvableDependencyIsDebouncedNotInstant pins that the transient case
// costs one tick rather than a false alarm: a Secret that has merely not been
// materialised yet must not page anyone on its first observation.
func TestUnresolvableDependencyIsDebouncedNotInstant(t *testing.T) {
	app := readyCondition("CreateContainerConfigError", missingSecretMessage, 7)
	app.Status.ActiveRevision = "rev-1" // an established revision, so availability is observed
	seen := map[string]bool{}

	first := debounceUnhealthy(observedServiceStateFor("srv-a", &app, true), seen)
	if first.AvailabilityObserved {
		t.Error("the first unresolvable-config pass recorded unhealthy; a not-yet-created Secret would false-alarm")
	}
	second := debounceUnhealthy(observedServiceStateFor("srv-a", &app, true), seen)
	if !second.AvailabilityObserved || second.Availability != "unhealthy" {
		t.Errorf("a persistent unresolvable dependency must be recorded on the second pass; got observed=%v availability=%q",
			second.AvailabilityObserved, second.Availability)
	}
}
