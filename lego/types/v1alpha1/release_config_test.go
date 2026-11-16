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
package v1alpha1

import "testing"

func TestReleaseConfigExpiresOnlyWhenAnotherReleaseIsRequested(t *testing.T) {
	app := &App{}
	app.Generation = 8
	app.Status.ReleaseGeneration = 7
	app.Annotations = map[string]string{AnnotationReleaseGeneration: "7"}
	app.Spec.ReleaseConfig = &ReleaseConfigReference{Generation: 7, SourceGeneration: 3, Image: "image:A"}
	if app.ActiveReleaseConfig() == nil {
		t.Fatal("operational mutation expired the running selection")
	}
	app.Annotations[AnnotationReleaseGeneration] = "9"
	if app.ActiveReleaseConfig() != nil {
		t.Fatal("normal deploy retained historical selection")
	}
	app.Status.SelectedReleaseGeneration = 7
	if app.ActiveReleaseConfig() == nil {
		t.Fatal("the adopted selection the operator still runs expired before it adopted the newer release (w4/661)")
	}
	app.Status.SelectedReleaseGeneration = 0
	app.Spec.ReleaseConfig.Generation = 9
	if app.ActiveReleaseConfig() == nil {
		t.Fatal("fresh restart selection was ignored before operator adoption")
	}
	app.Status.ReleaseGeneration = 10
	if app.ActiveReleaseConfig() != nil {
		t.Fatal("operator-adopted direct CR release retained stale selection")
	}
}

// The running release's port, shutdown delay and pre-deploy command come from
// its selection while one is active, and from the saved spec once a deploy
// expires it (w4/m433).
func TestRuntimeSpecRunsTheSelectionsRecordedSettings(t *testing.T) {
	app := &App{}
	app.Generation, app.Status.ReleaseGeneration = 4, 4
	app.Spec = AppSpec{Type: TypeWebService, Port: 9000, MaxShutdownDelaySeconds: new(int32(40)), PreDeployCommand: "echo two"}
	app.Spec.ReleaseConfig = &ReleaseConfigReference{Generation: 4, SourceGeneration: 1, Image: "image:A",
		Runtime: &ReleaseRuntimeSettings{Port: 3000, PreDeployCommand: "echo one"}}
	spec := app.RuntimeSpec()
	if spec.Port != 3000 || spec.MaxShutdownDelaySeconds != nil || spec.PreDeployCommand != "echo one" {
		t.Fatalf("selected runtime = port %d delay %v pre-deploy %q, want the recorded 3000, default, echo one", spec.Port, spec.MaxShutdownDelaySeconds, spec.PreDeployCommand)
	}
	if command, _ := app.PreDeployStepCommand(); command != "echo one" || app.ServingPort() != 3000 {
		t.Fatalf("readers see pre-deploy %q, port %d", command, app.ServingPort())
	}
	if app.Spec.Port != 9000 || *app.Spec.MaxShutdownDelaySeconds != 40 {
		t.Fatal("RuntimeSpec changed the saved spec")
	}

	app.Spec.ReleaseConfig.Runtime = nil
	if spec := app.RuntimeSpec(); spec.Port != 9000 || spec.PreDeployCommand != "echo two" {
		t.Fatal("a record without settings must run the saved ones")
	}

	app.Spec.ReleaseConfig.Runtime = &ReleaseRuntimeSettings{Port: 3000}
	app.Annotations = map[string]string{AnnotationReleaseGeneration: "5"}
	if spec := app.RuntimeSpec(); spec.Port != 9000 {
		t.Fatal("a requested deploy runs the saved settings")
	}
}

// A recorded port mode never moves a service between configured and image
// ports: that follows its source, which a selection does not change.
func TestRuntimeSettingsKeepThePortFamily(t *testing.T) {
	spec := AppSpec{Type: TypePrivateService, PortMode: PortModeImageConfiguredV1, Port: 8080}
	(&ReleaseRuntimeSettings{Port: 7000, PortMode: PortModeImageV1}).ApplyTo(&spec)
	if spec.Port != 7000 || spec.PortMode != PortModeImageV1 {
		t.Fatalf("same family = %d %q, want the recorded port and mode", spec.Port, spec.PortMode)
	}
	(&ReleaseRuntimeSettings{Port: 3000, PortMode: "configured-v1"}).ApplyTo(&spec)
	if spec.Port != 7000 || spec.PortMode != PortModeImageV1 {
		t.Fatalf("other family = %d %q, want the saved port kept", spec.Port, spec.PortMode)
	}
}
