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
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/id"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Exercise the public apply and planner together after JSON has dropped empty
// env lists, as the API server does. The fake client alone preserves slice shape.
func TestBlueprintEmptyEnvironmentSerializedReapply(t *testing.T) {
	for _, kind := range []string{"web", "static", "worker", "pserv", "cron"} {
		for _, env := range []struct{ name, declaration string }{
			{"empty", "    envVars: []\n"},
			{"omitted", ""},
			{"group", "    envVars:\n      - fromGroup: shared\n"},
			{"seed", "    envVars:\n      - {key: GENERATED, generateValue: true}\n      - {key: INITIAL, value: once, sync: false}\n"},
		} {
			t.Run(kind+"/"+env.name, func(t *testing.T) {
				manifest := emptyEnvironmentManifest(kind) + env.declaration
				if env.name == "group" {
					manifest = "envVarGroups:\n  - name: shared\n    envVars: [{key: GROUP_VALUE, value: retained}]\n" + manifest
				}
				rec := &recordingStore{}
				svc, cl := newService(rec)
				groups, seeds := newFakeEnvGroups(), &fakeSeeder{}
				svc.EnvGroups, svc.EnvSeeder = groups, seeds
				ctx := context.Background()
				result, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
				if err != nil {
					t.Fatal(err)
				}
				if len(result.Services) != 1 {
					t.Fatalf("initial services: %+v", result)
				}
				initial := manage(getApp(t, cl, "empty-env"), id.New(id.Service))
				initial.Generation = 7
				initial.Status.ActiveRevision = "rev-7"
				initial.Spec.RestartedAt = "2026-10-01T00:00:00Z"
				svc.Clock = func() time.Time { return time.Date(2026, 10, 2, 0, 0, 0, 0, time.UTC) }
				baseline := serializedBlueprintApp(t, initial)
				if baseline.Spec.Env != nil {
					t.Fatalf("fixture env must deserialize nil: %#v", baseline.Spec.Env)
				}
				for attempt := range 2 {
					// Reload between attempts so no process-local representation hides drift.
					svc.Client = fakeClient(serializedBlueprintApp(t, baseline))
					assertEmptyEnvironmentPlan(t, svc, manifest, baseline.Name, env.name == "group")
					applied, applyErr := svc.DeployStack(ctx, DeployRequest{Manifest: manifest})
					if applyErr != nil {
						t.Fatal(applyErr)
					}
					after := getApp(t, svc.Client, baseline.Name)
					if len(applied.Services) != 1 || applied.Services[0].ID != svc.view(baseline).ID {
						t.Errorf("attempt %d changed identity: %+v", attempt, applied)
					}
					if !reflect.DeepEqual(after.Spec, baseline.Spec) || after.Generation != baseline.Generation || after.Status.ActiveRevision != baseline.Status.ActiveRevision || after.ResourceVersion != baseline.ResourceVersion {
						t.Errorf("attempt %d mutated unchanged service: restart %q -> %q, version %s -> %s, generation %d -> %d, revision %q -> %q", attempt, baseline.Spec.RestartedAt, after.Spec.RestartedAt, baseline.ResourceVersion, after.ResourceVersion, baseline.Generation, after.Generation, baseline.Status.ActiveRevision, after.Status.ActiveRevision)
					}
					if len(rec.deployCalls) != 0 {
						t.Errorf("attempt %d opened %d deploy records", attempt, len(rec.deployCalls))
					}
					baseline = serializedBlueprintApp(t, after)
				}
				if env.name == "group" && len(groups.links) != 3 {
					t.Errorf("link requests=%v; expected initial and two idempotent relinks", groups.links)
				}
				if env.name == "seed" && (len(seeds.seeds) != 3 || seeds.seeds[2].literals["INITIAL"] != "once" || !reflect.DeepEqual(seeds.seeds[2].generates, []string{"GENERATED"})) {
					t.Errorf("seed ownership lost: %+v", seeds.seeds)
				}
			})
		}
	}
}

func emptyEnvironmentManifest(kind string) string {
	if kind == "static" {
		return "services:\n  - name: empty-env\n    type: web\n    runtime: static\n    repo: https://github.com/bex-co/bex\n    staticPublishPath: .\n"
	}
	manifest := fmt.Sprintf("services:\n  - name: empty-env\n    type: %s\n    runtime: image\n    image: {url: nginx:1.27}\n", kind)
	if kind == "cron" {
		manifest += "    schedule: '* * * * *'\n"
	}
	return manifest
}

func serializedBlueprintApp(t *testing.T, a *appv1alpha1.App) *appv1alpha1.App {
	t.Helper()
	raw, err := json.Marshal(a)
	if err != nil {
		t.Fatal(err)
	}
	var out appv1alpha1.App
	if err = json.Unmarshal(raw, &out); err != nil {
		t.Fatal(err)
	}
	return &out
}

func assertEmptyEnvironmentPlan(t *testing.T, s *Service, manifest, name string, hasGroup bool) {
	t.Helper()
	v, err := s.ValidateBlueprint(context.Background(), "", manifest, "")
	if err != nil || !v.Valid || v.Plan == nil {
		t.Fatalf("validation=%+v err=%v", v, err)
	}
	serviceFound, groupFound := false, false
	for _, action := range v.Plan.Actions {
		switch action.Kind {
		case BlueprintResourceService:
			serviceFound = true
			if action.Name != name || action.Operation != BlueprintPlanNoop || len(action.ChangedFields) != 0 {
				t.Errorf("unchanged service action=%+v", action)
			}
		case BlueprintResourceEnvVarGroup:
			groupFound = true
			if action.Operation != BlueprintPlanUpdate {
				t.Errorf("existing group must retain conservative update: %+v", action)
			}
		default:
			t.Errorf("unexpected action=%+v", action)
		}
	}
	if !serviceFound || groupFound != hasGroup {
		t.Fatalf("missing expected actions: %+v", v.Plan)
	}
}

func TestBlueprintEnvironmentOwnershipAndRealChange(t *testing.T) {
	manifest := emptyEnvironmentManifest("web") + "    envVars:\n      - {key: OWNED, value: original}\n"
	rec := &recordingStore{}
	svc, cl := newService(rec)
	ctx := context.Background()
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: manifest}); err != nil {
		t.Fatal(err)
	}
	app := manage(getApp(t, cl, "empty-env"), id.New(id.Service))
	app.Spec.Env = append(app.Spec.Env,
		appv1alpha1.EnvVar{Name: "MUTABLE", Value: "dashboard"},
		appv1alpha1.EnvVar{Name: "SECRET", ValueFrom: &appv1alpha1.EnvVarSource{SecretKeyRef: &appv1alpha1.SecretKeySelector{Name: "owned-secret", Key: "token"}}})
	app = serializedBlueprintApp(t, app)
	svc.Client = fakeClient(app)
	// Empty/omitted declarations own no keys, so retain literals and references.
	for _, declaration := range []string{"", "    envVars: []\n"} {
		unchanged := emptyEnvironmentManifest("web") + declaration
		assertEmptyEnvironmentPlan(t, svc, unchanged, app.Name, false)
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: unchanged}); err != nil {
			t.Fatal(err)
		}
		if got := getApp(t, svc.Client, app.Name); !reflect.DeepEqual(got.Spec.Env, app.Spec.Env) {
			t.Fatalf("undeclared env changed: %#v", got.Spec.Env)
		}
	}
	changed := strings.Replace(manifest, "value: original", "value: updated", 1) + "      - {key: APPENDED, value: last}\n"
	v, err := svc.ValidateBlueprint(ctx, "", changed, "")
	if err != nil || !v.Valid || v.Plan == nil || len(v.Plan.Actions) != 1 {
		t.Fatalf("changed plan=%+v err=%v", v, err)
	}
	action := v.Plan.Actions[0]
	if action.Operation != BlueprintPlanUpdate || len(action.ChangedFields) != 1 || action.ChangedFields[0].Path != "envVars" {
		t.Fatalf("real env diff=%+v", action)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: changed}); err != nil {
		t.Fatal(err)
	}
	got := getApp(t, svc.Client, app.Name)
	want := append([]appv1alpha1.EnvVar(nil), app.Spec.Env...)
	want[0].Value = "updated"
	want = append(want, appv1alpha1.EnvVar{Name: "APPENDED", Value: "last"})
	if !reflect.DeepEqual(got.Spec.Env, want) {
		t.Fatalf("merge order/ownership: got %#v want %#v", got.Spec.Env, want)
	}
	if got.Spec.RestartedAt == "" || len(rec.deployCalls) != 1 {
		t.Fatalf("real change did not deploy: restart=%q deploys=%+v", got.Spec.RestartedAt, rec.deployCalls)
	}
}
