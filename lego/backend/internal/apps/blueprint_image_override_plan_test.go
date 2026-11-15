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

package apps

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// w4/m418 (round 407): a Blueprint sync that rolls a service serving an
// imageUrl override replaces it with the declared image, and its plan named
// only envVars. The plan names image, with the running and declared images;
// an unchanged manifest stays a noop, as on Render.
func TestBlueprintPlanNamesAReplacedImageOverride(t *testing.T) {
	const running = "docker.io/library/busybox:1.37"
	manifest := emptyEnvironmentManifest("web")
	svc, cl := newService(&recordingStore{})
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	app := manage(getApp(t, cl, "empty-env"), id.New(id.Service))
	app.Generation = 4
	app.Spec.ReleaseConfig = &appv1alpha1.ReleaseConfigReference{Generation: 4, Image: running}
	app.Status.ReleaseGeneration, app.Status.SelectedReleaseGeneration = 4, 4
	svc.Client = fakeClient(serializedBlueprintApp(t, app))

	assertEmptyEnvironmentPlan(t, svc, manifest, app.Name, false)

	changed := manifest + "    envVars:\n      - {key: QA_BP, value: plan-vs-apply}\n"
	v, err := svc.ValidateBlueprint(ctx, "", changed, "")
	if err != nil || !v.Valid || v.Plan == nil || len(v.Plan.Actions) != 1 {
		t.Fatalf("plan=%+v err=%v", v, err)
	}
	action := v.Plan.Actions[0]
	paths := make([]string, 0, len(action.ChangedFields))
	for _, c := range action.ChangedFields {
		paths = append(paths, c.Path)
	}
	if action.Operation != BlueprintPlanUpdate || !slices.Equal(paths, []string{"envVars", "image"}) {
		t.Fatalf("action=%+v, want an update of envVars and image", action)
	}
	if !strings.Contains(action.Message, running) || !strings.Contains(action.Message, "nginx:1.27") {
		t.Fatalf("message=%q, want the running %s and the declared nginx:1.27", action.Message, running)
	}

	// Without an override the same change names envVars alone.
	app.Spec.ReleaseConfig = nil
	svc.Client = fakeClient(serializedBlueprintApp(t, app))
	v, err = svc.ValidateBlueprint(ctx, "", changed, "")
	if err != nil || v.Plan == nil || len(v.Plan.Actions) != 1 {
		t.Fatalf("plan=%+v err=%v", v, err)
	}
	if action := v.Plan.Actions[0]; len(action.ChangedFields) != 1 || action.ChangedFields[0].Path != "envVars" || action.Message != "" {
		t.Fatalf("action=%+v, want envVars alone and no message", action)
	}
}
