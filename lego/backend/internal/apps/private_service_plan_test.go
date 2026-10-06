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

// private_service_plan_test.go pins the paid-only Private Service policy
// (w1/111, extending ADR030 §7's worker decision to the sibling type Render also
// sells only on paid instances).
//
// The defect this closes: a private service created with no plan came out `free`
// while a background worker came out `starter`, from the one shared function
// normalizeTierForType, because it special-cased only TypeBackgroundWorker. Live
// evidence is in .pm/w1/done/111.md (qa-20260926-priv on plan free vs
// qa-20260926-wrk on starter).
//
// The free-keeping types are pinned here too, so this rule can never quietly
// widen into "everything but web".

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func TestCreatePrivateServiceDefaultsToPaidPlan(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "priv", Type: appv1alpha1.TypePrivateService, Image: "nginx:1",
	}); err != nil {
		t.Fatalf("Create plan-less private service: %v", err)
	}
	if got := getApp(t, cl, "priv").Spec.Tier; got != "starter" {
		t.Errorf("plan-less private service tier = %q, want the paid default starter (was %q before w1/111)", got, "free")
	}
}

func TestCreatePrivateServiceFreePlanRefused(t *testing.T) {
	svc, _ := newService(nil)
	_, err := svc.Create(context.Background(), CreateRequest{
		Name: "priv", Type: appv1alpha1.TypePrivateService, Image: "nginx:1", Plan: "free",
	})
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "requires a paid plan") {
		t.Fatalf("Create free private service = %v, want the paid-only refusal", err)
	}
	// The refusal must name the type the caller asked for, not the sibling —
	// a message naming background_worker sends the reader to the wrong docs.
	if !strings.Contains(err.Error(), "private_service") {
		t.Errorf("refusal does not name private_service: %v", err)
	}
}

func TestSetPlanPrivateServiceFreeRefused(t *testing.T) {
	svc, cl := newService(nil)
	if _, err := svc.Create(context.Background(), CreateRequest{
		Name: "priv", Type: appv1alpha1.TypePrivateService, Image: "nginx:1", Plan: "standard",
	}); err != nil {
		t.Fatalf("Create private service: %v", err)
	}

	_, err := svc.SetPlan(context.Background(), "priv", "free")
	if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "requires a paid plan") {
		t.Fatalf("SetPlan private service -> free = %v, want the paid-only refusal", err)
	}
	if got := getApp(t, cl, "priv").Spec.Tier; got != "standard" {
		t.Errorf("a refused downgrade must not change spec.tier, got %q", got)
	}

	if _, err := svc.SetPlanDryRun(context.Background(), "priv", "free"); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "requires a paid plan") {
		t.Errorf("SetPlanDryRun private service -> free = %v, want the paid-only refusal", err)
	}

	if _, err := svc.SetPlan(context.Background(), "priv", "starter"); err != nil {
		t.Fatalf("SetPlan private service -> starter must stay open, got %v", err)
	}
}

// TestPaidOnlyServiceTypesAreExactlyTwo guards the allowlist itself. Web services
// and cron jobs are sold on Free by Render and must keep it; a static site runs no
// instance. If a future change reads the rule as "everything but web", this fails.
func TestPaidOnlyServiceTypesAreExactlyTwo(t *testing.T) {
	paidOnly := map[string]bool{
		appv1alpha1.TypeBackgroundWorker: true,
		appv1alpha1.TypePrivateService:   true,
	}
	for _, svcType := range []string{
		appv1alpha1.TypeWebService,
		appv1alpha1.TypePrivateService,
		appv1alpha1.TypeBackgroundWorker,
		appv1alpha1.TypeCronJob,
		appv1alpha1.TypeStaticSite,
	} {
		if got, want := paidOnlyServiceType(svcType), paidOnly[svcType]; got != want {
			t.Errorf("paidOnlyServiceType(%q) = %v, want %v", svcType, got, want)
		}
	}
}

// TestFreeKeepingTypesStillAcceptFree is the control: the types Render sells on
// Free still resolve an explicit free plan, so this milestone narrowed nothing it
// should not have.
func TestFreeKeepingTypesStillAcceptFree(t *testing.T) {
	for _, svcType := range []string{appv1alpha1.TypeWebService, appv1alpha1.TypeCronJob} {
		tier, err := normalizeTierForType(svcType, "free")
		if err != nil {
			t.Errorf("normalizeTierForType(%q, free) = %v, want free to be accepted", svcType, err)
			continue
		}
		if tier != "free" {
			t.Errorf("normalizeTierForType(%q, free) = %q, want free", svcType, tier)
		}
	}
}
