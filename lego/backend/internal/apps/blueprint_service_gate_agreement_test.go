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

// agreement/service paid-tier admission: the service plan refuses exactly what
// the service apply refuses. deployStack and the sync preflight run the action
// plan before (or without) validateBlueprintServices, and the approve dialog
// shows that plan — so a gate the plan skips is a plan that promises what the
// apply refuses (the Key Value paused-tier cell was the same bug, fixed
// earlier). The two deterministic gates the validate and apply run on every
// service write are the Pro-workspace autoscaling gate and the Scale-workspace
// inbound-rules gate; both are pure functions of (desired spec, current spec,
// workspace plan), so the plan can judge them exactly.
//
// The per-spec declared-plan layer (autoscalingSpec refuses a free declared
// tier) already runs in blueprintReadSpec, which both sides share; only the
// workspace-plan layer below was missing from the plan.
const (
	serviceGateScalingManifest = `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    plan: starter
    scaling:
      minInstances: 2
      maxInstances: 8
      targetCPUPercent: 70
`
	serviceGateIPAllowListManifest = `services:
  - name: api
    type: web
    runtime: image
    image: {url: nginx:1.27}
    plan: starter
    ipAllowList:
      - {source: 203.0.113.0/24, description: office}
`
)

// planServiceGateVerdict runs the direct current-state action plan for one
// service manifest, the verb deployStack and the sync preflight run without
// validate. It returns the plan error, or "" when the plan lists an action.
func planServiceGateVerdict(t *testing.T, svc *Service, ctx context.Context, manifest string) error {
	t.Helper()
	st, ir, err := compileStack(DeployRequest{Manifest: manifest})
	if err != nil {
		t.Fatalf("compileStack: %v", err)
	}
	plan, available, err := svc.blueprintActionPlan(ctx, ir, st, "", svc.newWorkspaceSnapshot(ctx))
	if err != nil {
		return err
	}
	if !available {
		t.Fatal("blueprintActionPlan unavailable with a client and no env groups")
	}
	if len(plan.Actions) != 1 {
		t.Fatalf("plan actions = %+v, want the one service action", plan.Actions)
	}
	return nil
}

// TestBlueprintServicePlanMirrorsPaidTierGates pins the agreement matrix: for
// each workspace plan, the plan refuses exactly when the validate boundary the
// apply runs first refuses. Hobby is below both gates, pro clears autoscaling
// only, scale clears both.
func TestBlueprintServicePlanMirrorsPaidTierGates(t *testing.T) {
	for _, tc := range []struct {
		name     string
		manifest string
		// refuseBelowPro / refuseBelowScale name the workspace plans the apply
		// refuses on; the plan must match each cell.
		refuse map[string]bool
		want   string
	}{
		{"scaling", serviceGateScalingManifest, map[string]bool{"hobby": true, "pro": false, "scale": false}, "scaling"},
		{"ipAllowList", serviceGateIPAllowListManifest, map[string]bool{"hobby": true, "pro": true, "scale": false}, "ipAllowList"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			for workspacePlan, wantRefused := range tc.refuse {
				t.Run(workspacePlan, func(t *testing.T) {
					svc, _ := newService(nil)
					withWorkspacePlan(svc, workspacePlan)
					ctx := core.WithWorkspace(context.Background(), "tea-a")

					planErr := planServiceGateVerdict(t, svc, ctx, tc.manifest)

					st, _, err := compileStack(DeployRequest{Manifest: manifestForValidate(tc.manifest)})
					if err != nil {
						t.Fatalf("compileStack: %v", err)
					}
					validateErr := svc.validateBlueprintServices(core.WithRequestMemo(ctx), st, svc.newWorkspaceSnapshot(ctx))

					if (planErr != nil) != (validateErr != nil) {
						t.Fatalf("workspace %s: plan err = %v, validate err = %v; the plan must refuse exactly what the apply refuses",
							workspacePlan, planErr, validateErr)
					}
					if wantRefused && planErr == nil {
						t.Fatalf("workspace %s: plan allowed %s the apply refuses", workspacePlan, tc.want)
					}
					if !wantRefused && planErr != nil {
						t.Fatalf("workspace %s: plan refused %s the apply allows: %v", workspacePlan, tc.want, planErr)
					}
					if planErr != nil && !strings.Contains(planErr.Error(), tc.want) {
						t.Fatalf("plan refusal names no field: %v, want it to name %q", planErr, tc.want)
					}
				})
			}
		})
	}
}

// manifestForValidate is the identity: the validate boundary judges the same
// manifest the plan does.
func manifestForValidate(manifest string) string { return manifest }

// TestBlueprintServicePlanGateRefusalsAreCoded pins the refusal shape the plan
// surfaces: exactly what the apply answers with, naming the service, so the
// dialog can locate it rather than failing the sync at apply. The autoscaling
// gate answers the coded PLAN_FEATURE_REQUIRED; the inbound gate answers a
// plain bad request with the shared one-sentence message.
func TestBlueprintServicePlanGateRefusalsAreCoded(t *testing.T) {
	svc, _ := newService(nil)
	withWorkspacePlan(svc, "hobby")
	ctx := core.WithWorkspace(context.Background(), "tea-a")

	planErr := planServiceGateVerdict(t, svc, ctx, serviceGateScalingManifest)
	if planErr == nil {
		t.Fatal("plan allowed autoscaling on a hobby workspace")
	}
	var coded *core.CodedError
	if !errors.As(planErr, &coded) || coded.Code != core.PlanFeatureRequiredCode {
		t.Fatalf("plan refusal = %v, want the coded PLAN_FEATURE_REQUIRED the apply answers", planErr)
	}

	withWorkspacePlan(svc, "pro")
	planErr = planServiceGateVerdict(t, svc, ctx, serviceGateIPAllowListManifest)
	if planErr == nil {
		t.Fatal("plan allowed inbound rules on a pro workspace")
	}
	if !errors.Is(planErr, core.ErrBadRequest) || !strings.Contains(planErr.Error(), core.InboundIPRulesMessage) {
		t.Fatalf("plan refusal = %v, want the shared inbound-rules refusal the apply answers", planErr)
	}
}

// TestBlueprintServicePlanGateUpdateSide pins the update cell: turning
// autoscaling on for an existing service on a hobby workspace is a refusal,
// not an update naming scaling; re-sending an already-on policy still plans an
// update-free no-op path (the gate's unchanged-policy exemption).
//
// The service is store-on (a workspace resolver plus a member identity), so
// the snapshot and the gates resolve the same tenant, as in production;
// store-off the snapshot would miss the service and the gates would degrade
// open, testing neither side.
func TestBlueprintServicePlanGateUpdateSide(t *testing.T) {
	newTenantService := func(app *appv1alpha1.App) (*Service, context.Context) {
		svc, _ := newService(nil, app)
		svc.Base.Workspace = fakeWorkspace{"id-a": "tea-a"}
		ctx := core.WithWorkspace(
			core.WithIdentity(context.Background(), core.Identity{Subject: "id-a", Method: "session"}),
			"tea-a",
		)
		return svc, ctx
	}

	existing := tenantWebApp("api")
	existing.Spec.Tier = "starter"
	svc, ctx := newTenantService(existing)
	withWorkspacePlan(svc, "hobby")

	if err := planServiceGateVerdict(t, svc, ctx, serviceGateScalingManifest); err == nil {
		t.Fatal("plan allowed turning autoscaling on where the apply refuses")
	}

	withWorkspacePlan(svc, "scale")
	if err := planServiceGateVerdict(t, svc, ctx, serviceGateScalingManifest); err != nil {
		t.Fatalf("plan refused autoscaling a scale workspace allows: %v", err)
	}

	// The unchanged-policy exemption agrees too: a service already running the
	// declared policy is not refused on a hobby workspace.
	cpu := int32(70)
	running := tenantWebApp("api")
	running.Spec.Tier = "starter"
	running.Spec.Autoscaling = &appv1alpha1.AutoscalingSpec{Enabled: true, MinReplicas: 2, MaxReplicas: 8, TargetCPUPercent: &cpu}
	svc, ctx = newTenantService(running)
	withWorkspacePlan(svc, "hobby")
	if err := planServiceGateVerdict(t, svc, ctx, serviceGateScalingManifest); err != nil {
		t.Fatalf("plan refused re-sending the running policy: %v", err)
	}
}
