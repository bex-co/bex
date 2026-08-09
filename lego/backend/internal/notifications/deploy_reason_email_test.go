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

package notifications

import (
	"strings"
	"testing"
)

// deploy_reason_email_test.go covers the w7/m79 addition to the failure email.
//
// w7/m44 enriched this email with the commit message and a logs link, but it
// never said what actually went wrong — even though the operator had already
// diagnosed it and every API surface carried the answer. The email is the first
// thing most people read when a deploy breaks, so it was the surface most worth
// telling and the one still saying least.

const configErrorReason = `container configuration cannot be resolved: secret "dpg-d9x-app" not found — ` +
	`a referenced Secret or ConfigMap must exist in this service's own namespace and carry the referenced key.`

func TestFailureEmailNamesTheCause(t *testing.T) {
	_, msg := deployEmail("forum", deployMailFailed, deployDetails{
		deployID:      "dep-1",
		failureReason: configErrorReason,
	}, "https://dashboard.bex.co/services/srv-1/deploys/dep-1")

	body := msg.Text()
	if !strings.Contains(body, "dpg-d9x-app") {
		t.Errorf("the failure email does not name the cause:\n%s", body)
	}
	// Impact framing still leads — the reason supplements it rather than
	// replacing the register w7/m44 deliberately matched to Render's.
	if !strings.Contains(body, "didn't complete successfully") {
		t.Errorf("the failure email lost its impact framing:\n%s", body)
	}
}

// TestSuccessAndStartedEmailsCarryNoReason pins that the reason is failure-only.
// A "deploy started" mail explaining a failure would be incoherent, and a
// succeeded one carrying stale text from a previous attempt would be worse.
func TestSuccessAndStartedEmailsCarryNoReason(t *testing.T) {
	for _, kind := range []deployMailKind{deployMailStarted, deployMailSucceeded} {
		_, msg := deployEmail("forum", kind, deployDetails{
			deployID: "dep-1", failureReason: configErrorReason,
		}, "")
		if strings.Contains(msg.Text(), "dpg-d9x-app") {
			t.Errorf("a non-failure email (%v) carried a failure reason:\n%s", kind, msg.Text())
		}
	}
}

// TestFailureEmailWithoutADiagnosisIsUnchanged pins the byte-identical path for
// a failure the operator could not explain: no empty paragraph, no placeholder.
func TestFailureEmailWithoutADiagnosisIsUnchanged(t *testing.T) {
	_, with := deployEmail("forum", deployMailFailed, deployDetails{deployID: "dep-1"}, "")
	_, without := deployEmail("forum", deployMailFailed, deployDetails{
		deployID: "dep-1", failureReason: "   ",
	}, "")

	if with.Text() != without.Text() {
		t.Errorf("a blank failure reason changed the email:\n--- with ---\n%s\n--- blank ---\n%s",
			with.Text(), without.Text())
	}
}
