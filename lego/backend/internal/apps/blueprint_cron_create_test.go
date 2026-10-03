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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
)

func TestBlueprintCronFirstReapplyPreservesCommand(t *testing.T) {
	for _, tc := range []struct{ runtime, alias string }{{"docker", "startCommand"}, {"docker", "dockerCommand"}, {"node", "startCommand"}} {
		t.Run(tc.runtime+"/"+tc.alias, func(t *testing.T) {
			manifest := "services:\n  - name: cron-command\n    type: cron\n    runtime: " + tc.runtime + "\n    repo: https://github.com/acme/jobs\n    schedule: '0 * * * *'\n    " + tc.alias + ": echo original\n"
			if tc.runtime == "node" {
				manifest += "    buildCommand: echo build\n"
			}
			rec := &recordingStore{}
			svc, cl := newService(rec)
			if _, err := svc.DeployStack(t.Context(), DeployRequest{Manifest: manifest}); err != nil {
				t.Fatal(err)
			}
			baseline := manage(getApp(t, cl, "cron-command"), id.New(id.Service))
			baseline.Spec.RestartedAt = "2026-10-01T00:00:00Z"
			baseline = serializedBlueprintApp(t, baseline)
			svc.Clock = func() time.Time { return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC) }
			for attempt := range 2 {
				svc.Client = fakeClient(serializedBlueprintApp(t, baseline))
				assertEmptyEnvironmentPlan(t, svc, manifest, baseline.Name, false)
				result, err := svc.DeployStack(t.Context(), DeployRequest{Manifest: manifest})
				if err != nil {
					t.Fatal(err)
				}
				after := getApp(t, svc.Client, baseline.Name)
				if len(result.Services) != 1 || result.Services[0].ID != svc.view(baseline).ID || !reflect.DeepEqual(after.Spec, baseline.Spec) || after.ResourceVersion != baseline.ResourceVersion || len(rec.deployCalls) != 0 {
					t.Fatalf("attempt %d changed unchanged cron: before=%+v after=%+v deploys=%d", attempt, baseline.Spec, after.Spec, len(rec.deployCalls))
				}
				baseline = serializedBlueprintApp(t, after)
			}
			changed := strings.Replace(manifest, "echo original", "echo changed", 1)
			validation, err := svc.ValidateBlueprint(t.Context(), "", changed, "")
			if err != nil || !validation.Valid || validation.Plan == nil || len(validation.Plan.Actions) != 1 || validation.Plan.Actions[0].Operation != BlueprintPlanUpdate {
				t.Fatalf("changed plan=%+v err=%v", validation, err)
			}
			if _, err := svc.DeployStack(t.Context(), DeployRequest{Manifest: changed}); err != nil {
				t.Fatal(err)
			}
			after := getApp(t, svc.Client, baseline.Name)
			if after.Spec.Command != "echo changed" || after.Spec.RestartedAt == baseline.Spec.RestartedAt || len(rec.deployCalls) != 1 {
				t.Fatalf("genuine command change not deployed: %+v deploys=%d", after.Spec, len(rec.deployCalls))
			}
		})
	}
}
