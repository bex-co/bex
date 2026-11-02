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

// w8/m146: newDeploy.id comes only from the cancel reasons bex itself writes.
func TestSupersedingDeployIDReadsOnlyBexsOwnReasons(t *testing.T) {
	for reason, want := range map[string]string{
		supersededByReason("dep-abc123"):    "dep-abc123",
		restartCanceledReason("dep-abc123"): "dep-abc123",
		"Superseded by a newer release":     "",
		suspendCancelReason:                 "",
		"":                                  "",
		"Superseded by dep-x, and more":     "",
		"superseded by dep-abc":             "",
	} {
		if got := SupersedingDeployID(reason); got != want {
			t.Errorf("SupersedingDeployID(%q) = %q, want %q", reason, got, want)
		}
	}
}

// The close records only its own release's structured cause.
func TestDeployCloseEndIsReleaseAttributed(t *testing.T) {
	exit := int32(2)
	app := appWithRolloutFailure(7, &exit, 900)
	if got := deployCloseEnd(app, Deploy{Generation: 7}, DeployUpdateFailed); got.ExitCode == nil || *got.ExitCode != 2 || got.TimedOutSeconds != 900 {
		t.Fatalf("own release = %+v", got)
	}
	for name, got := range map[string]DeployEnd{
		"other release": deployCloseEnd(app, Deploy{Generation: 6}, DeployUpdateFailed),
		"not a rollout": deployCloseEnd(app, Deploy{Generation: 7}, DeployBuildFailed),
		"no generation": deployCloseEnd(app, Deploy{}, DeployUpdateFailed),
	} {
		if got != (DeployEnd{}) {
			t.Errorf("%s = %+v, want nothing", name, got)
		}
	}
}

func appWithRolloutFailure(generation int64, exit *int32, timedOut int32) *appv1alpha1.App {
	return &appv1alpha1.App{Status: appv1alpha1.AppStatus{RolloutFailure: &appv1alpha1.RolloutFailureStatus{
		Generation: generation, ExitCode: exit, TimedOutSeconds: timedOut,
	}}}
}
