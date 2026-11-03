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

import "testing"

func TestBuildRunStatusFoldsDeployStatuses(t *testing.T) {
	for status, want := range map[string]string{
		DeployCreated: BuildRunCreated, DeployQueued: BuildRunCreated,
		DeployBuildInProgress: BuildRunInProgress, DeployPreDeployInProgress: BuildRunInProgress, DeployUpdateInProgress: BuildRunInProgress,
		DeployLive: BuildRunSucceeded, DeployDeactivated: BuildRunSucceeded,
		DeployBuildFailed: BuildRunFailed, DeployPreDeployFailed: BuildRunFailed, DeployUpdateFailed: BuildRunFailed,
		DeployCanceled: BuildRunCanceled,
	} {
		if got := BuildRunStatus(status); got != want {
			t.Errorf("BuildRunStatus(%q) = %q, want %q", status, got, want)
		}
	}
	for _, status := range DeployStatuses() {
		if BuildRunStatus(status) == "" {
			t.Errorf("BuildRunStatus(%q) is empty; every deploy status needs a buildRun status", status)
		}
	}
}
