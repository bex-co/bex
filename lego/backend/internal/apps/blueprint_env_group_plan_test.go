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
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/core/coretest"
	"github.com/bex-co/bex/lego/backend/internal/envgroups"
	"github.com/bex-co/bex/lego/backend/internal/envgroups/envgroupstest"
)

// w8/376: an existing env group plans as an update only when the apply would
// write one of its variables. Until then every existing group planned as an
// update of envVars and name, so an unchanged Blueprint with a group never
// read no_changes, and approving that plan wrote nothing.

const planGroupManifest = `envVarGroups:
  - name: shared
    envVars:
      - {key: LOG_LEVEL, value: info}
      - {key: TOKEN, generateValue: true}
`

// envGroupPlanService is a Service over the real env-groups service, wired as
// the composition root wires it, with planGroupManifest applied.
func envGroupPlanService(t *testing.T) (*Service, *envgroups.Service, context.Context) {
	t.Helper()
	svc, _ := newService(nil)
	svc.Workspace = coretest.Workspaces{core.DefaultTenant}
	groups := &envgroups.Service{Base: svc.Base, Store: newMemKV(), Groups: envgroupstest.New()}
	svc.EnvGroups = groups
	svc.EnvGroupWritesOwed = envgroups.GroupWritesOwed(groups)
	ctx := core.WithIdentity(context.Background(), core.Identity{Subject: "owner", Method: "session"})
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: planGroupManifest}); err != nil {
		t.Fatal(err)
	}
	return svc, groups, ctx
}

// groupPlan is the plan's one env-group action for manifest.
func groupPlan(t *testing.T, svc *Service, ctx context.Context, manifest string) BlueprintPlanAction {
	t.Helper()
	v, err := svc.ValidateBlueprint(ctx, "", manifest, "")
	if err != nil || !v.Valid || v.Plan == nil || v.Plan.Mode != "current_state" {
		t.Fatalf("validate = %+v err=%v, want a current-state plan", v, err)
	}
	for _, action := range v.Plan.Actions {
		if action.Kind == BlueprintResourceEnvVarGroup {
			return action
		}
	}
	t.Fatalf("plan %+v has no env group action", v.Plan.Actions)
	return BlueprintPlanAction{}
}

func TestBlueprintPlanOfAnUnchangedEnvGroupIsNoop(t *testing.T) {
	svc, _, ctx := envGroupPlanService(t)
	for range 2 {
		action := groupPlan(t, svc, ctx, planGroupManifest)
		if action.Operation != BlueprintPlanNoop || len(action.ChangedFields) != 0 || action.Message != "" {
			t.Fatalf("unchanged group planned %+v, want noop", action)
		}
		if action.ResourceID == "" {
			t.Errorf("noop group carries no resource id: %+v", action)
		}
		if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: planGroupManifest}); err != nil {
			t.Fatal(err)
		}
	}
	// A sync:false entry in a group is ignored, as on Render (w8/293): it
	// owes nothing either.
	manual := planGroupManifest + "      - {key: MANUAL, sync: false}\n"
	if action := groupPlan(t, svc, ctx, manual); action.Operation != BlueprintPlanNoop {
		t.Errorf("a group's sync:false entry planned %+v, want noop", action)
	}
}

func TestBlueprintPlanListsAnEnvGroupWriteTheApplyOwes(t *testing.T) {
	for _, tc := range []struct{ name, manifest string }{
		{"changed literal", strings.Replace(planGroupManifest, "value: info", "value: debug", 1)},
		{"new literal", planGroupManifest + "      - {key: REGION, value: eu}\n"},
		{"empty literal", planGroupManifest + "      - {key: EMPTY, value: \"\"}\n"},
		{"missing generate", planGroupManifest + "      - {key: SIGNING_KEY, generateValue: true}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _, ctx := envGroupPlanService(t)
			action := groupPlan(t, svc, ctx, tc.manifest)
			if action.Operation != BlueprintPlanUpdate || len(action.ChangedFields) != 1 || action.ChangedFields[0].Path != "envVars" || action.Message != "" {
				t.Fatalf("planned %+v, want an update of envVars alone", action)
			}
			// The apply writes what the plan listed, and the next plan of the
			// same file is a noop.
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.manifest}); err != nil {
				t.Fatal(err)
			}
			if action := groupPlan(t, svc, ctx, tc.manifest); action.Operation != BlueprintPlanNoop {
				t.Fatalf("after the apply the same file planned %+v, want noop", action)
			}
		})
	}
}

// A value edited outside the Blueprint is a value the next sync re-writes.
func TestBlueprintPlanListsAnEnvGroupValueEditedSinceTheSync(t *testing.T) {
	svc, groups, ctx := envGroupPlanService(t)
	ids, err := groups.GroupIDsByName(ctx)
	if err != nil || ids["shared"] == "" {
		t.Fatalf("group ids = %v, %v", ids, err)
	}
	if _, err := groups.SetEnvGroupVar(ctx, ids["shared"], "LOG_LEVEL", "trace"); err != nil {
		t.Fatal(err)
	}
	if action := groupPlan(t, svc, ctx, planGroupManifest); action.Operation != BlueprintPlanUpdate || len(action.ChangedFields) != 1 || action.ChangedFields[0].Path != "envVars" {
		t.Fatalf("planned %+v, want an update of envVars", action)
	}
	// A generated value edited since is kept: generate-once.
	if _, err := groups.SetEnvGroupVar(ctx, ids["shared"], "LOG_LEVEL", "info"); err != nil {
		t.Fatal(err)
	}
	if _, err := groups.SetEnvGroupVar(ctx, ids["shared"], "TOKEN", "rotated"); err != nil {
		t.Fatal(err)
	}
	if action := groupPlan(t, svc, ctx, planGroupManifest); action.Operation != BlueprintPlanNoop {
		t.Fatalf("an edited generated value planned %+v, want noop", action)
	}
}

// A caller who may not read the group's values cannot learn from the plan
// whether a declared value matches: the group plans as an update of envVars
// that says why, whatever the values are.
func TestBlueprintPlanOfAGroupWhoseValuesTheCallerCannotReadListsEnvVars(t *testing.T) {
	svc, _, ctx := envGroupPlanService(t)
	svc.Authz = denyRelationChecker{deny: core.RelCanViewSensitive}
	for _, manifest := range []string{planGroupManifest, strings.Replace(planGroupManifest, "value: info", "value: debug", 1)} {
		action := groupPlan(t, svc, ctx, manifest)
		if action.Operation != BlueprintPlanUpdate || len(action.ChangedFields) != 1 || action.ChangedFields[0].Path != "envVars" || action.Message != envGroupValuesUnread {
			t.Fatalf("a viewer's plan = %+v, want an update of envVars with %q", action, envGroupValuesUnread)
		}
	}
	svc.Authz = nil
	svc.EnvGroupWritesOwed = nil
	if action := groupPlan(t, svc, ctx, planGroupManifest); action.Operation != BlueprintPlanUpdate || action.Message != envGroupValuesUnread {
		t.Fatalf("without the seam the plan = %+v, want the update it cannot rule out", action)
	}
}
