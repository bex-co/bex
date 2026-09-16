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

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestQueuedWaitNeverPopulatesFailureReason is w2/m99's terminal-only guard.
//
// m99 makes a queued deploy narrate the operator's wait reason on every log
// surface. The tempting shortcut is to route that reason through the field the
// operator message ALREADY reaches — failure_reason — but failureReason means
// "this deploy failed"; populating it for a live wait would turn every capacity
// wait into a failed deploy for the dashboard, the CLI, the events feed and the
// deploy-failed notification. The narration therefore reads the App's Ready
// condition directly (internal/logs/progress.go), and this pins the other half:
// while the row is still open, the close path contributes nothing.
func TestQueuedWaitNeverPopulatesFailureReason(t *testing.T) {
	const gen = int64(4)
	open := Deploy{Generation: gen, Status: DeployCreated}

	for _, tc := range []struct {
		name    string
		reason  string
		message string
	}{
		{"workspace cap", appv1alpha1.ReasonBuildQueued, "workspace has 2/2 concurrent builds active; waiting for a slot"},
		{"cluster cap", appv1alpha1.ReasonBuildQueued, "cluster has 4/4 concurrent builds active; waiting for a slot"},
		{"scheduler wait", appv1alpha1.ReasonBuildQueued, "waiting for build capacity: 0/7 nodes are available"},
		{"registry credentials", appv1alpha1.ReasonRegistryCredsPending, "Waiting for the registry to accept this app's build credential"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := buildingApp(gen, tc.reason, tc.message)
			if got := observedDeployStatus(open, app, false); got != DeployQueued {
				t.Fatalf("status = %q, want %q — the fixture must be a live wait", got, DeployQueued)
			}
			reason, code := deployCloseFailureReason(app, open, DeployQueued, true)
			if reason != "" || code != "" {
				t.Errorf("a queued wait produced failure_reason %q / code %q — failureReason is terminal-only", reason, code)
			}
		})
	}

	// The control: once the wait times out and the row actually closes
	// build_failed, the same operator message DOES become the failure reason.
	app := buildingApp(gen, appv1alpha1.ReasonBuildQueued, "workspace has 2/2 concurrent builds active; waiting for a slot")
	if reason, _ := deployCloseFailureReason(app, open, DeployBuildFailed, true); reason == "" {
		t.Error("a timed-out queued row must still close with the operator's reason")
	}
}
