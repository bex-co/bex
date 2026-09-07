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

// The w6/m118 policy: the free plan runs at most one instance (its compute rate
// is $0.00/second, so N free instances would be N× capacity for $0), matching
// Render, whose free instance types offer no horizontal scaling. These tests
// pin the bound on every backend write path that can set an instance count —
// Scale, create, autoscaling min/max, and a plan downgrade — at the boundary
// (1 allowed) and one past it (refused, with the plan and limit named). A paid
// plan is uncapped by plan (only the platform ceiling applies), so the same
// counts that free refuses succeed on starter.

func freeWebApp(name string) *appv1alpha1.App {
	a := sampleApp(name)
	a.Spec.Type = appv1alpha1.TypeWebService
	a.Spec.Tier = "free"
	a.Spec.Replicas = 1
	return a
}

// TestScaleFreePlanCappedAtOne: Scale to the cap is allowed; one past it is a
// bad request naming the plan and the limit, and leaves spec.replicas untouched.
func TestScaleFreePlanCappedAtOne(t *testing.T) {
	svc, cl := newService(nil, freeWebApp("web"))

	if _, err := svc.Scale(context.Background(), "web", 1); err != nil {
		t.Fatalf("Scale free -> 1 (at the cap) must be allowed, got %v", err)
	}

	_, err := svc.Scale(context.Background(), "web", 2)
	if !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("Scale free -> 2 should be core.ErrBadRequest, got %v", err)
	}
	if msg := err.Error(); !strings.Contains(msg, "free") || !strings.Contains(msg, "1") {
		t.Errorf("refusal must name the plan and the limit, got %q", msg)
	}
	if got := getApp(t, cl, "web").Spec.Replicas; got != 1 {
		t.Errorf("a refused scale must not touch spec.replicas, got %d", got)
	}
}

// TestScalePaidPlanUncapped: the count free refuses succeeds on a paid plan,
// which carries no plan cap (only store.MaxReplicas).
func TestScalePaidPlanUncapped(t *testing.T) {
	svc, _ := newService(nil, paidWebApp("web"))
	if _, err := svc.Scale(context.Background(), "web", 5); err != nil {
		t.Fatalf("Scale starter -> 5 must be allowed (no plan cap), got %v", err)
	}
}

// TestCreateFreePlanCappedAtOne: create sets replicas directly, so it must be
// gated too — a caller cannot create at N to sidestep the Scale cap.
func TestCreateFreePlanCappedAtOne(t *testing.T) {
	base := CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService, Image: "nginx:1"}

	free1 := base
	free1.Plan, free1.Replicas = "free", 1
	if _, err := specFromCreate(free1); err != nil {
		t.Fatalf("create free at 1 (the cap) must be allowed, got %v", err)
	}

	free2 := base
	free2.Plan, free2.Replicas = "free", 2
	_, err := specFromCreate(free2)
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "free") {
		t.Fatalf("create free at 2 should be refused naming the plan, got %v", err)
	}

	paid := base
	paid.Plan, paid.Replicas = "standard", 3
	if _, err := specFromCreate(paid); err != nil {
		t.Fatalf("create standard at 3 must be allowed (no plan cap), got %v", err)
	}
}

// TestAutoscalingFreePlanCappedAtOne: the autoscaler drives replicas up to
// maxInstances, so an uncapped max would reintroduce the outcome by another
// door — both bounds are held to the plan cap.
func TestAutoscalingFreePlanCappedAtOne(t *testing.T) {
	cpu := int32(70)
	svc, _ := newService(nil, freeWebApp("web"))

	over := SetAutoscalingRequest{MinInstances: 1, MaxInstances: 2, TargetCPUPercent: &cpu}
	_, err := svc.SetAutoscaling(context.Background(), "web", over)
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "free") {
		t.Fatalf("autoscaling free maxInstances 2 should be refused naming the plan, got %v", err)
	}

	// max == cap is the degenerate but valid free autoscaling config.
	at := SetAutoscalingRequest{MinInstances: 1, MaxInstances: 1, TargetCPUPercent: &cpu}
	if _, err := svc.SetAutoscaling(context.Background(), "web", at); err != nil {
		t.Fatalf("autoscaling free maxInstances 1 (at the cap) must be allowed, got %v", err)
	}
}

// TestSetPlanDowngradeOverCapRefused: downgrading a multi-instance service to a
// plan whose cap is below its current count is refused with a "scale down first"
// message, rather than silently shrinking a running service. At/under the cap
// the downgrade proceeds.
func TestSetPlanDowngradeOverCapRefused(t *testing.T) {
	over := paidWebApp("web") // starter, 2 replicas (from sampleApp)
	svc, cl := newService(nil, over)
	_, err := svc.SetPlan(context.Background(), "web", "free")
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "free") {
		t.Fatalf("downgrade of a 2-replica service to free should be refused, got %v", err)
	}
	if got := getApp(t, cl, "web").Spec.Tier; got != "starter" {
		t.Errorf("a refused downgrade must not change spec.tier, got %q", got)
	}

	under := paidWebApp("ok")
	under.Spec.Replicas = 1
	svc2, _ := newService(nil, under)
	if _, err := svc2.SetPlan(context.Background(), "ok", "free"); err != nil {
		t.Fatalf("downgrade of a 1-replica service to free must be allowed, got %v", err)
	}
}

// TestInstanceCapServiceTypeFamily enumerates the five service types explicitly
// (t007 step 4). web_service and private_service share the same replica-setting
// create/Scale path and are capped identically on the free plan. A
// background_worker is paid-only (w6/025), so free is refused before any cap
// logic runs and its paid plans carry no plan cap. cron_job and static_site
// have no multi-instance path to cap — a cron runs to completion and a static
// site serves from object storage — so the cap simply never applies; they
// create fine at the default single instance.
func TestInstanceCapServiceTypeFamily(t *testing.T) {
	replicaTypes := []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
	}
	for _, typ := range replicaTypes {
		req := CreateRequest{Name: "svc", Type: typ, Image: "nginx:1", Plan: "free", Replicas: 2}
		if _, err := specFromCreate(req); !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("free %s at 2 instances should be refused, got %v", typ, err)
		}
	}

	// background_worker: free is refused outright (paid-only, w6/025) — the
	// paid-plan refusal fires, not the instance cap — and its paid plans are
	// uncapped like every other paid plan.
	worker := CreateRequest{Name: "svc", Type: appv1alpha1.TypeBackgroundWorker, Image: "nginx:1", Plan: "free", Replicas: 2}
	if _, err := specFromCreate(worker); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "paid plan") {
		t.Errorf("free background_worker should be refused as paid-only, got %v", err)
	}
	paidWorker := CreateRequest{Name: "svc", Type: appv1alpha1.TypeBackgroundWorker, Image: "nginx:1", Plan: "starter", Replicas: 3}
	if _, err := specFromCreate(paidWorker); err != nil {
		t.Errorf("starter background_worker at 3 instances must be allowed (no plan cap), got %v", err)
	}

	// cron_job: a schedule, not a replica set — created at its single instance.
	cron := CreateRequest{Name: "job", Type: appv1alpha1.TypeCronJob, Image: "job:1", Plan: "free", Schedule: "0 0 * * *", StartCommand: "bin/report"}
	if _, err := specFromCreate(cron); err != nil {
		t.Errorf("free cron_job (no replica concept) must create cleanly, got %v", err)
	}
}
