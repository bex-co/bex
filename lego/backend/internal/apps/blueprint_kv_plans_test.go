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
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/keyvalue"
	"github.com/bex-co/bex/lego/types/tiers"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func kvPlanManifest(plan string) string {
	return `
services:
  - name: cache
    type: keyvalue
    ipAllowList: []
    plan: ` + plan + `
`
}

// A Blueprint names Render's larger Key Value plans by size or legacy name; a
// paused one is refused with the API's code unless the store already runs it,
// so re-syncing an existing store's own plan keeps working.
func TestBlueprintKeyValueLargePlans(t *testing.T) {
	if !tiers.Valkey.Paused("pro_ultra") {
		t.Skip("pro_ultra is no longer paused")
	}
	svc, cl := newService(nil)
	ctx := context.Background()
	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: kvPlanManifest("10g")})
	if err != nil {
		t.Fatalf("deploy plan 10g: %v", err)
	}
	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, key(res.KeyValues[0].ID), &kv); err != nil {
		t.Fatal(err)
	}
	if kv.Spec.Plan != "pro_plus" {
		t.Errorf("10g stored as %q, want pro_plus", kv.Spec.Plan)
	}

	_, err = svc.DeployStack(ctx, DeployRequest{Manifest: kvPlanManifest("40g")})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != keyvalue.InstanceTypeUnavailableCode {
		t.Fatalf("upgrade to paused 40g => %v, want 400 %s", err, keyvalue.InstanceTypeUnavailableCode)
	}
	if err := cl.Get(ctx, key(kv.Name), &kv); err != nil {
		t.Fatal(err)
	}
	if kv.Spec.Plan != "pro_plus" {
		t.Errorf("refused upgrade changed the plan to %q", kv.Spec.Plan)
	}

	// A store already on the paused tier may re-declare it, here in the
	// Blueprint schema's spaced spelling.
	kv.Spec.Plan = "pro_ultra"
	if err := cl.Update(ctx, &kv); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: kvPlanManifest("pro ultra")}); err != nil {
		t.Errorf("re-sync of the store's own paused plan => %v, want nil", err)
	}
}

// A paused Key Value tier is refused at validation as the apply refuses it:
// the create review must not say valid and price a store the sync then
// rejects. A plan change onto it is refused the same way, while a store
// already running it may re-declare its own plan.
func TestValidateBlueprintRefusesPausedKeyValuePlan(t *testing.T) {
	if !tiers.Valkey.Paused("pro_ultra") {
		t.Skip("pro_ultra is no longer paused")
	}
	svc, cl := newService(nil)
	ctx := context.Background()

	v, err := svc.ValidateBlueprint(ctx, "", kvPlanManifest("40g"), "")
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid || len(v.Errors) == 0 || v.Errors[0].Code != keyvalue.InstanceTypeUnavailableCode {
		t.Fatalf("paused create = valid %v errors %+v, want %s", v.Valid, v.Errors, keyvalue.InstanceTypeUnavailableCode)
	}
	if v.EstimatedPricing != nil {
		t.Errorf("an invalid manifest must carry no estimate, got %+v", v.EstimatedPricing)
	}

	res, err := svc.DeployStack(ctx, DeployRequest{Manifest: kvPlanManifest("standard")})
	if err != nil {
		t.Fatalf("deploy standard: %v", err)
	}
	v, err = svc.ValidateBlueprint(ctx, "", kvPlanManifest("40g"), "")
	if err != nil {
		t.Fatalf("ValidateBlueprint upgrade: %v", err)
	}
	if v.Valid || len(v.Errors) == 0 || v.Errors[0].Code != keyvalue.InstanceTypeUnavailableCode {
		t.Fatalf("paused upgrade = valid %v errors %+v, want %s", v.Valid, v.Errors, keyvalue.InstanceTypeUnavailableCode)
	}

	var kv appv1alpha1.KeyValue
	if err := cl.Get(ctx, key(res.KeyValues[0].ID), &kv); err != nil {
		t.Fatal(err)
	}
	kv.Spec.Plan = "pro_ultra"
	if err := cl.Update(ctx, &kv); err != nil {
		t.Fatal(err)
	}
	if v, err := svc.ValidateBlueprint(ctx, "", kvPlanManifest("pro ultra"), ""); err != nil || !v.Valid {
		t.Fatalf("re-declaring the store's own paused plan = valid %v errors %+v err %v", v.Valid, v.Errors, err)
	}
}

// agreement/keyvalue create paused-plan: the action plan refuses a Key Value
// create on a paused tier exactly as the apply refuses it. The preview's
// create path and the plan's update path already run keyvalue.ErrPausedPlan
// (#154); the plan's create path showed a bare Create, so deployStack's plan
// gate — which runs before validate and before any write — let the sync
// through to record the grouping and env-group rows before applyKeyValue
// refused it. The paused check is a pure catalog judgment, so the plan judges
// the apply's exact input, blueprintKeyValueGatePlan on a create (the declared
// plan, or the create default a plan-less entry carries). The second-free
// store limit stays out: it is workspace runtime state the plan cannot know,
// judged by the preview's live CheckFreeLimit and enforced by the apply, the
// same rule as the env-group count cap.
func TestBlueprintKeyValuePlanRefusesPausedCreate(t *testing.T) {
	if !tiers.Valkey.Paused("pro_ultra") {
		t.Skip("pro_ultra is no longer paused")
	}
	for _, tc := range []struct {
		name string
		plan string
		// wantRefused is the apply's verdict: applyKeyValue refuses a create
		// naming a paused tier with INSTANCE_TYPE_UNAVAILABLE, and a store
		// already on it may re-declare it. The plan must match each cell.
		wantRefused bool
	}{
		{"paused create refused", "pro ultra", true},
		{"unpaused create plans", "starter", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc, _ := newService(nil)
			ctx := core.WithWorkspace(context.Background(), "tea-a")
			st, ir, err := compileStack(DeployRequest{Manifest: kvPlanManifest(tc.plan)})
			if err != nil {
				t.Fatalf("compileStack: %v", err)
			}
			plan, available, planErr := svc.blueprintActionPlan(ctx, ir, st, "", svc.newWorkspaceSnapshot(ctx))
			if planErr == nil && !available {
				t.Fatal("blueprintActionPlan unavailable with a client and no env groups")
			}
			// The apply-side oracle on the identical judgment: a create has
			// no store to keep, so the declared plan is the new selection.
			applyErr := keyvalue.ErrPausedPlan(blueprintKeyValueGatePlan(st.keyValues[0], ""), "")
			if (planErr != nil) != (applyErr != nil) {
				t.Fatalf("plan err = %v, apply err = %v; the plan must refuse exactly what the apply refuses",
					planErr, applyErr)
			}
			if !tc.wantRefused {
				if planErr != nil {
					t.Fatalf("plan refused a create the apply allows: %v", planErr)
				}
				if len(plan.Actions) != 1 || plan.Actions[0].Operation != BlueprintPlanCreate {
					t.Fatalf("plan actions = %+v, want the one key-value create", plan.Actions)
				}
				return
			}
			if planErr == nil {
				t.Fatalf("plan showed a create the apply refuses with %v", applyErr)
			}
			var coded *core.CodedError
			if !errors.As(planErr, &coded) || coded.Code != keyvalue.InstanceTypeUnavailableCode {
				t.Fatalf("plan refusal = %v, want the coded %s the apply answers", planErr, keyvalue.InstanceTypeUnavailableCode)
			}
		})
	}
}
