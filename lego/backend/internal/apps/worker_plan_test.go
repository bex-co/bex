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

// worker_plan_test.go pins the paid-only Background Worker policy (w6/025):
// a worker never lands on the free tier. An omitted plan defaults to the
// cheapest paid rung, an explicit free plan is refused on create and on every
// plan-change path (all surfaces funnel through specFromCreate and
// SetPlan/PreviewSetPlan), and the Blueprint billing/pricing probes price the
// paid default rather than free. Other service types keep their free
// eligibility — pinned by TestSpecFromCreateDefaults and the instance-cap
// family.

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCreateWorkerDefaultsToPaidPlan(t *testing.T) {
	if got := defaultPaidTierID(); got != "starter" {
		t.Fatalf("defaultPaidTierID() = %q, want starter (the catalog's cheapest paid rung)", got)
	}
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "job:v1",
	}); err != nil {
		t.Fatalf("Create plan-less worker: %v", err)
	}
	if got := getApp(t, cl, "worker").Spec.Tier; got != "starter" {
		t.Errorf("plan-less worker tier = %q, want the paid default starter", got)
	}
}

func TestCreateWorkerFreePlanRefused(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "job:v1", Plan: "free",
	})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "requires a paid plan") {
		t.Fatalf("free worker create = %v, want the paid-only refusal", err)
	}
	if strings.Contains(err.Error(), "free") {
		t.Errorf("the refusal must list only plans a worker may use, got %q", err)
	}
}

// TestSetPlanWorkerFreeRefused: an existing worker cannot be downgraded to
// free through SetPlan (REST PATCH, GraphQL updateServicePlan, and MCP
// update_service all fold into it), the dry-run twin refuses identically, and
// a refused downgrade leaves spec.tier untouched. Paid-to-paid changes remain
// open.
func TestSetPlanWorkerFreeRefused(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "worker", Type: appv1alpha1.TypeBackgroundWorker, Image: "job:v1", Plan: "standard",
	}); err != nil {
		t.Fatalf("Create worker: %v", err)
	}

	_, err := svc.SetPlan(context.Background(), "worker", "free")
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "requires a paid plan") {
		t.Fatalf("SetPlan worker -> free = %v, want the paid-only refusal", err)
	}
	if got := getApp(t, cl, "worker").Spec.Tier; got != "standard" {
		t.Errorf("a refused downgrade must not change spec.tier, got %q", got)
	}

	if _, err := svc.PreviewSetPlan(context.Background(), "worker", "free"); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "requires a paid plan") {
		t.Errorf("PreviewSetPlan worker -> free = %v, want the paid-only refusal", err)
	}

	if _, err := svc.SetPlan(context.Background(), "worker", "starter"); err != nil {
		t.Fatalf("SetPlan worker -> starter must stay open, got %v", err)
	}
}

// TestBlueprintWorkerPlanResolution: the Blueprint path sees the same policy —
// a plan-less worker counts as a paid plan for the payment-method gate and is
// priced at the paid default, and a manifest that declares a free worker fails
// validation before any write.
func TestBlueprintWorkerPlanResolution(t *testing.T) {
	const planless = `services:
  - name: worker
    type: worker
    runtime: image
    image: {url: job:v1}
`
	st, err := parseStack(DeployRequest{Manifest: planless})
	if err != nil {
		t.Fatalf("parseStack: %v", err)
	}
	if !stackHasPaidPlan(st) {
		t.Error("a plan-less worker provisions at the paid default and must trip the payment-method gate")
	}
	est := blueprintEstimatedPricing(st)
	if est == nil || len(est.Lines) != 1 || est.Lines[0].Name != "worker" {
		t.Fatalf("estimate = %+v, want one worker line", est)
	}
	if est.TotalUSD == "0.00" {
		t.Error("plan-less worker estimated at 0.00/mo; it provisions at the paid default and must be priced")
	}

	const free = planless + "    plan: free\n"
	svc := &Service{Base: &core.Base{Client: fakeClient(), Namespace: "default"}}
	v, err := svc.ValidateBlueprint(context.Background(), "", free)
	if err != nil {
		t.Fatalf("ValidateBlueprint: %v", err)
	}
	if v.Valid {
		t.Fatalf("a free-worker manifest must fail validation, got %+v", v)
	}
}
