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
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
)

func TestBlueprintCronCommandClearingAndUpdateReapply(t *testing.T) {
	for _, tc := range []struct{ runtime, alias string }{{"docker", "startCommand"}, {"docker", "dockerCommand"}, {"go", "startCommand"}} {
		alias := tc.alias
		t.Run(tc.runtime+"/"+alias, func(t *testing.T) {
			manifest := fmt.Sprintf("services:\n  - name: cron-command\n    type: cron\n    runtime: %s\n    buildCommand: go build .\n    repo: https://github.com/bex-co/bex\n    schedule: '* * * * *'\n    %s: echo original\n", tc.runtime, alias)
			if tc.runtime == "docker" {
				manifest = strings.Replace(manifest, "    buildCommand: go build .\n", "", 1)
			}
			rec := &recordingStore{}
			svc, cl := newService(rec)
			ctx := context.Background()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
				t.Fatal(err)
			}
			initial := manage(getApp(t, cl, "cron-command"), id.New(id.Service))
			initial.Spec.RestartedAt = "2026-10-01T00:00:00Z"
			initial.Generation = 7
			initial.Status.ActiveRevision = "rev-7"
			svc.Clock = func() time.Time { return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC) }
			baseline := serializedBlueprintApp(t, initial)
			svc.Client = fakeClient(baseline)
			changed := strings.Replace(manifest, "echo original", "echo updated", 1)
			beforeDeploys := len(rec.deployCalls)
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: changed}); err != nil {
				t.Fatal(err)
			}
			after := getApp(t, svc.Client, baseline.Name)
			if after.Spec.Command != "echo updated" || after.Spec.StartCommand != baseline.Spec.StartCommand || after.Spec.RestartedAt == initial.Spec.RestartedAt || len(rec.deployCalls) != beforeDeploys+1 {
				t.Fatalf("command update failed: spec=%+v deploys=%d", after.Spec, len(rec.deployCalls))
			}
			svc.Client = fakeClient(serializedBlueprintApp(t, after))
			assertEmptyEnvironmentPlan(t, svc, changed, baseline.Name, false)
			beforeDeploys = len(rec.deployCalls)
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: changed}); err != nil {
				t.Fatal(err)
			}
			if got := getApp(t, svc.Client, baseline.Name); !reflect.DeepEqual(got.Spec, after.Spec) || got.ResourceVersion != after.ResourceVersion || len(rec.deployCalls) != beforeDeploys {
				t.Fatal("repeated command update changed cron")
			}

			cleared := strings.Replace(changed, "echo updated", "''", 1)
			if tc.runtime != "docker" {
				validation, err := svc.ValidateBlueprint(ctx, "", cleared, "")
				if err == nil && validation.Valid {
					t.Fatal("native empty command accepted")
				}
				return
			}
			omitted := strings.Replace(changed, "    "+alias+": echo updated\n", "", 1)
			assertEmptyEnvironmentPlan(t, svc, omitted, baseline.Name, false)
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: omitted}); err != nil {
				t.Fatal(err)
			}
			if got := getApp(t, svc.Client, baseline.Name); !reflect.DeepEqual(got.Spec, after.Spec) {
				t.Fatal("omitted command changed cron")
			}
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: cleared}); err != nil {
				t.Fatal(err)
			}
			after = getApp(t, svc.Client, baseline.Name)
			if after.Spec.Command != "" || after.Spec.StartCommand != "" {
				t.Fatalf("clear retained runtime command: %+v", after.Spec)
			}
			svc.Client = fakeClient(serializedBlueprintApp(t, after))
			assertEmptyEnvironmentPlan(t, svc, cleared, baseline.Name, false)
			beforeDeploys = len(rec.deployCalls)
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: cleared}); err != nil {
				t.Fatal(err)
			}
			if got := getApp(t, svc.Client, baseline.Name); !reflect.DeepEqual(got.Spec, after.Spec) || got.ResourceVersion != after.ResourceVersion || len(rec.deployCalls) != beforeDeploys {
				t.Fatal("repeated clear changed cron")
			}

		})
	}
}
