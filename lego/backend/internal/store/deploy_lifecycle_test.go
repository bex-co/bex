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

func TestDeployStatusVocabularyAndTransitionTable(t *testing.T) {
	want := []string{
		DeployCreated,
		DeployQueued,
		DeployBuildInProgress,
		DeployBuildFailed,
		DeployPreDeployInProgress,
		DeployPreDeployFailed,
		DeployUpdateInProgress,
		DeployUpdateFailed,
		DeployLive,
		DeployDeactivated,
		DeployCanceled,
	}
	got := DeployStatuses()
	if len(got) != len(want) {
		t.Fatalf("DeployStatuses = %v, want all eleven statuses", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("DeployStatuses[%d] = %q, want %q", i, got[i], want[i])
		}
	}

	valid := [][2]string{
		{DeployCreated, DeployQueued},
		{DeployQueued, DeployBuildInProgress},
		{DeployBuildInProgress, DeployPreDeployInProgress},
		{DeployPreDeployInProgress, DeployUpdateInProgress},
		{DeployUpdateInProgress, DeployLive},
		{DeployLive, DeployDeactivated},
		// Sampling may skip unobserved phases.
		{DeployCreated, DeployLive},
		{DeployQueued, DeployBuildFailed},
		{DeployBuildInProgress, DeployPreDeployFailed},
		{DeployCreated, DeployUpdateFailed},
		{DeployCreated, DeployCanceled},
	}
	for _, edge := range valid {
		if !CanTransitionDeploy(edge[0], edge[1]) {
			t.Errorf("expected legal transition %s -> %s", edge[0], edge[1])
		}
	}

	invalid := [][2]string{
		{DeployQueued, DeployCreated},
		{DeployBuildInProgress, DeployQueued},
		{DeployUpdateInProgress, DeployBuildInProgress},
		{DeployBuildFailed, DeployUpdateInProgress},
		{DeployPreDeployFailed, DeployLive},
		{DeployUpdateFailed, DeployLive},
		{DeployCanceled, DeployLive},
		{DeployDeactivated, DeployLive},
		{DeployLive, DeployUpdateFailed},
		{DeployLive, DeployLive},
	}
	for _, edge := range invalid {
		if CanTransitionDeploy(edge[0], edge[1]) {
			t.Errorf("unexpected legal transition %s -> %s", edge[0], edge[1])
		}
	}
}
