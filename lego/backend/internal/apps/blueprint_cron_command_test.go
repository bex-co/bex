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
	"fmt"
	"strings"
	"testing"
	"time"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestBlueprintCronCommandSerializedReapply(t *testing.T) {
	for _, tc := range []struct {
		name, runtime, field, build, command string
	}{
		{"native Bash startCommand", "go", "startCommand", "    buildCommand: go build -o app .\n", "[[ -f marker ]] && ./app"},
		{"docker whitespace dockerCommand", "docker", "dockerCommand", "", "   "},
	} {
		t.Run(tc.name, func(t *testing.T) {
			manifest := fmt.Sprintf("services:\n  - name: nightly\n    type: cron\n    runtime: %s\n    repo: https://github.com/bex-co/bex\n    schedule: '0 2 * * *'\n%s    %s: %q\n", tc.runtime, tc.build, tc.field, tc.command)
			recorder := &recordingStore{}
			svc, _ := connectionService(t)
			svc.Store = recorder
			svc.Client = blueprintSerializedClient{Client: svc.Client}
			now := time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC)
			svc.Clock = func() time.Time { return now }
			ctx := ownershipCtx()
			apply := func(manifest string) string {
				t.Helper()
				result, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
				if err != nil || len(result.Services) != 1 {
					t.Fatalf("apply = %+v, %v", result, err)
				}
				return result.Services[0].ID
			}
			firstID := apply(manifest)
			before := getTenantApp(t, svc.Client, connOwner, "nightly")
			if before.Spec.Type != appv1alpha1.TypeCronJob || before.Spec.Runtime != tc.runtime || before.Spec.Command != "" || before.Spec.StartCommand != tc.command {
				t.Errorf("initial cron command fields = %+v", before.Spec)
			}
			if tc.runtime == "go" && (before.Spec.Builder != "native" || before.Spec.BuildCommand != "go build -o app .") {
				t.Fatalf("native build configuration was lost: %+v", before.Spec)
			}
			if len(recorder.appCreates) != 1 || firstID == "" || recorder.appCreates[0].ID != firstID || recorder.appCreates[0].FirstDeployID == "" {
				t.Fatalf("initial service/deploy identity = %q, %+v", firstID, recorder.appCreates)
			}
			for range 2 {
				actions := planActionsForTest(ctx, t, svc, manifest)
				if len(actions) != 1 {
					t.Fatalf("plan actions = %+v, want one action", actions)
				}
				action := actions[0]
				if action.Kind != BlueprintResourceService || action.ResourceID != firstID || action.Operation != BlueprintPlanNoop || len(action.ChangedFields) != 0 {
					t.Errorf("plan action = %+v, want noop for %q", action, firstID)
				}
				if got := apply(manifest); got != firstID {
					t.Errorf("unchanged apply replaced service: %q -> %q", firstID, got)
				}
				assertBlueprintAppUnchanged(t, before, getTenantApp(t, svc.Client, connOwner, "nightly"))
				if len(recorder.appCreates) != 1 || len(recorder.deployCalls) != 0 {
					t.Errorf("unchanged apply created services/deploys: %d/%d", len(recorder.appCreates), len(recorder.deployCalls))
				}
			}
		})
	}
}

func TestDirectCronCreatePreservesCommandFields(t *testing.T) {
	for _, command := range []string{"", "  echo override  "} {
		t.Run(command, func(t *testing.T) {
			svc, cl := newService(nil)
			_, err := svc.Create(context.Background(), CreateRequest{
				Name: "nightly", Type: appv1alpha1.TypeCronJob,
				Repo: "https://github.com/bex-co/bex", Runtime: "go",
				BuildCommand: "go build -o app .", StartCommand: "echo fallback",
				Schedule: "0 2 * * *", Command: command,
			})
			if err != nil {
				t.Fatal(err)
			}
			app := getApp(t, cl, "nightly")
			if app.Spec.StartCommand != "echo fallback" || app.Spec.Command != strings.TrimSpace(command) {
				t.Fatalf("direct create changed command semantics: %+v", app.Spec)
			}
		})
	}
}
