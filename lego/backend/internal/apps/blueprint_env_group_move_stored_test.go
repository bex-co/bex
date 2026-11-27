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
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// storedEnvGroups is fakeEnvGroups with a scripted control-plane environment
// per service App ID: the stored row the apply's scoping guards judge by,
// which the reconciler copies onto the App label on its next pass. Tests
// script the two apart to hold the reconciler-lag window open.
type storedEnvGroups struct {
	*fakeEnvGroups
	stored map[string]string // LabelAppID -> storage environment id
	fail   error
}

// StoredServiceEnvironment answers the scripted row, falling back to the App
// label when the script holds no row: the same store-first contract the real
// service's accessor keeps.
func (f *storedEnvGroups) StoredServiceEnvironment(_ context.Context, a *appv1alpha1.App) (string, error) {
	if f.fail != nil {
		return "", f.fail
	}
	if env, ok := f.stored[a.Labels[core.LabelAppID]]; ok {
		return env, nil
	}
	return a.Labels[core.LabelEnvironment], nil
}

// storedMoveFixture deploys the linked stack (web in staging, linked to cfg
// in staging, the service redeclarable out of the file), stamps the service's
// App ID the way the reconciler does, then holds the reconciler-lag window
// open: the label names labelEnv while the scripted store holds storedEnv.
func storedMoveFixture(t *testing.T, labelEnv, storedEnv string, fail error) (*Service, context.Context, *storedEnvGroups) {
	t.Helper()
	svc, base, _, ctx, st := groupingStackService(t)
	groups := &storedEnvGroups{fakeEnvGroups: newFakeEnvGroups(), stored: map[string]string{}, fail: fail}
	svc.EnvGroups = groups
	svc.EnvGroupPlacements = func(context.Context) (map[string]string, error) { return groups.environments, nil }
	svc.EnvGroupWritesOwed = groups.writesOwed
	if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: splitPlacement("staging", "staging")}); err != nil {
		t.Fatalf("deploy linked stack: %v", err)
	}
	byName := map[string]string{}
	for _, e := range st.environments {
		byName[e.Name] = e.ID
	}
	if byName["staging"] == "" || byName["production"] == "" {
		t.Fatalf("fixture environments = %v, want staging and production", st.environments)
	}
	app := getTenantApp(t, base, "tea-a", "web")
	app.Spec.EnvFromSecrets = []string{"evg-cfg-env"}
	if app.Labels == nil {
		app.Labels = map[string]string{}
	}
	app.Labels[core.LabelAppID] = "srv-web"
	app.Labels[core.LabelEnvironment] = byName[labelEnv]
	if err := base.Update(ctx, app); err != nil {
		t.Fatalf("stage the label window: %v", err)
	}
	groups.stored["srv-web"] = byName[storedEnv]
	return svc, ctx, groups
}

// The plan judges each linked service by the same source the apply's
// validateLinkedServiceEnvironments does — the control-plane store first, the
// App label only as fallback — so the plan refuses exactly what the apply
// refuses. The apply side is pinned by
// TestGroupMoveFollowsTheStoredServiceEnvironment (envgroups): the store
// already moved reads allowed, the store still behind reads refused, the
// label ignored in both. Each cell below moves only the group (the service
// stays out of the file, judged on its live state) and runs the real preview
// verb; allow-cells also run the real apply verb.
func TestValidateBlueprintJudgesLinkedServicesByTheStoredEnvironment(t *testing.T) {
	for _, tc := range []struct {
		name      string
		stored    string // fixture environment the scripted store holds
		label     string // fixture environment the App label still names
		wantValid bool
	}{
		{"both agree the service stays behind", "staging", "staging", false},
		{"both agree the service already moved", "production", "production", true},
		{"the store moved, the label lags", "production", "staging", true},
		{"the store moved away, the label lags", "staging", "production", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, ctx, groups := storedMoveFixture(t, tc.label, tc.stored, nil)
			v, err := svc.ValidateBlueprint(ctx, "tea-a", splitPlacement("", "production"), "")
			if err != nil {
				t.Fatalf("validate: %v", err)
			}
			if tc.wantValid {
				if !v.Valid {
					t.Fatalf("stored=%s label=%s refused with %+v, want the move planned: the store already holds the service in production",
						tc.stored, tc.label, v.Errors)
				}
				if got := plannedUpdates(v.Plan.Actions); got != "update env_var_group cfg: environment" {
					t.Fatalf("stored=%s label=%s planned %q, want the group move named", tc.stored, tc.label, got)
				}
				// The approved plan applies cleanly through the real verb.
				if _, err := svc.DeployStack(ctx, DeployRequest{OwnerID: "tea-a", Manifest: splitPlacement("", "production")}); err != nil {
					t.Fatalf("deploy the planned move: %v", err)
				}
				if got := groups.environments["cfg"]; got == "" {
					t.Fatalf("the applied group move recorded no environment")
				}
				return
			}
			if v.Valid || len(v.Errors) != 1 {
				t.Fatalf("stored=%s label=%s valid=%v errors=%+v, want one refusal", tc.stored, tc.label, v.Valid, v.Errors)
			}
			if v.Errors[0].Code != "ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES" {
				t.Fatalf("stored=%s label=%s refusal code = %q, want the apply's ENV_GROUP_MOVE_INCOMPATIBLE_SERVICES", tc.stored, tc.label, v.Errors[0].Code)
			}
			if !strings.Contains(v.Errors[0].Error, "web") {
				t.Fatalf("stored=%s label=%s refused with %q, want it to name the stranded service", tc.stored, tc.label, v.Errors[0].Error)
			}
		})
	}
}

// A stored-environment read failure fails the plan like it fails the apply,
// instead of planning on a label that may name the service's old environment.
func TestValidateBlueprintFailsWhenTheStoredEnvironmentIsUnreadable(t *testing.T) {
	svc, ctx, _ := storedMoveFixture(t, "staging", "staging", errors.New("store unavailable"))
	_, err := svc.ValidateBlueprint(ctx, "tea-a", splitPlacement("", "production"), "")
	if err == nil || !strings.Contains(err.Error(), "store unavailable") {
		t.Fatalf("validate with an unreadable store = %v, want the read error, not a plan", err)
	}
}
