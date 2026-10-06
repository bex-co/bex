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
	"strings"
	"testing"
)

// TestBlueprintImageDockerCommandIsTheStartCommand pins w5/080: a render.yaml
// image service declares its command with dockerCommand, which becomes the
// spec's startCommand exactly as the API sets it, a re-apply plans no change,
// and Generate writes it back as dockerCommand. A native runtime's refusal is
// blueprint_compiler_test's.
func TestBlueprintImageDockerCommandIsTheStartCommand(t *testing.T) {
	const command = "nginx -g 'daemon off;'"
	manifest := "services:\n  - name: edge\n    type: web\n    runtime: image\n    image:\n      url: nginx:1.27\n    dockerCommand: \"" + command + "\"\n"
	svc, _ := connectionService(t)
	ctx := ownershipCtx()
	result, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
	if err != nil || len(result.Services) != 1 {
		t.Fatalf("apply = %+v, %v", result, err)
	}
	app := getTenantApp(t, svc.Client, connOwner, "edge")
	if app.Spec.Image != "nginx:1.27" || app.Spec.StartCommand != command {
		t.Fatalf("image service spec = image %q, startCommand %q; want nginx:1.27, %q", app.Spec.Image, app.Spec.StartCommand, command)
	}
	actions := planActionsForTest(ctx, t, svc, manifest)
	if len(actions) != 1 || actions[0].Operation != BlueprintPlanNoop {
		t.Fatalf("re-plan = %+v, want one noop", actions)
	}

	out, err := svc.GenerateBlueprint(ctx, GenerateBlueprintRequest{ServiceIDs: []string{result.Services[0].ID}})
	if err != nil {
		t.Fatalf("GenerateBlueprint: %v", err)
	}
	if !strings.Contains(out.Manifest, "dockerCommand:") || strings.Contains(out.Manifest, "startCommand:") {
		t.Fatalf("generated manifest spells the image command wrong:\n%s", out.Manifest)
	}

	// Declaring both spellings stays ambiguous for an image too.
	both := "services:\n  - {type: web, name: twice, runtime: image, image: {url: nginx:1.27}, dockerCommand: a, startCommand: b}\n"
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: both}); err == nil || !strings.Contains(err.Error(), "cannot set both dockerCommand and startCommand") {
		t.Fatalf("both spellings apply = %v, want the refusal", err)
	}
}
