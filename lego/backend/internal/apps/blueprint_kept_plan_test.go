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

// w33/006: a sync or adopt keeps an existing resource's plan when its entry
// omits plan (Render "retains the current compute plan for an existing
// service", blueprint-spec § plan, read 2026-11-16), so the estimate and the
// paid-intent gate price that resource at the plan it keeps. Until then both
// read the parse's create default, so a file that only kept a free resource
// showed a Starter line and, under BEX_REQUIRE_PAYMENT_METHOD=1, asked a
// cardless workspace for a card.

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// keptPlanKinds are the three kinds whose plan-less entry the parse gives a
// paid create default: each one's top-level key, an entry writer, the
// default the parse writes and a paid plan other than it.
var keptPlanKinds = []struct {
	kind, top, createDefault, otherPaid string
	entry                               func(name, plan string) string
}{
	{kind: "web service", top: "services", createDefault: "starter", otherPaid: "standard", entry: func(name, plan string) string {
		return "  - type: web\n    name: " + name + "\n    runtime: image\n    image: {url: nginx:alpine}\n" + planLine(plan)
	}},
	{kind: "postgres", top: "databases", createDefault: "basic-256mb", otherPaid: "basic-1gb", entry: func(name, plan string) string {
		return "  - name: " + name + "\n" + planLine(plan)
	}},
	{kind: "key value", top: "services", createDefault: "starter", otherPaid: "standard", entry: func(name, plan string) string {
		return "  - type: keyvalue\n    name: " + name + "\n    ipAllowList: []\n" + planLine(plan)
	}},
}

func planLine(plan string) string {
	if plan == "" {
		return ""
	}
	return "    plan: " + plan + "\n"
}

// An existing free resource whose entry omits plan shows no paid line and
// asks a cardless workspace for no card; a create beside it in the same file
// is still priced and gated at the create default.
func TestAnExistingFreeResourceThatOmitsPlanIsNeitherPricedNorGated(t *testing.T) {
	for _, tc := range keptPlanKinds {
		t.Run(tc.kind, func(t *testing.T) {
			svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
			ctx := paidGateContext()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.top + ":\n" + tc.entry("kept", "free")}); err != nil {
				t.Fatalf("create the free resource: %v", err)
			}
			keepsOnly := tc.top + ":\n" + tc.entry("kept", "")
			withCreate := keepsOnly + tc.entry("fresh", "")

			v, err := svc.ValidateBlueprint(ctx, "", keepsOnly, "")
			if err != nil || !v.Valid || v.EstimatedPricing == nil {
				t.Fatalf("ValidateBlueprint(kept only) = %+v, %v", v, err)
			}
			if est := v.EstimatedPricing; len(est.Lines) != 0 || len(est.Variable) != 0 || est.TotalUSD != "0.00" {
				t.Fatalf("estimate of a file that only keeps a free resource = %+v, want nothing paid", est)
			}
			v, err = svc.ValidateBlueprint(ctx, "", withCreate, "")
			if err != nil || !v.Valid || v.EstimatedPricing == nil {
				t.Fatalf("ValidateBlueprint(kept + create) = %+v, %v", v, err)
			}
			if lines := v.EstimatedPricing.Lines; len(lines) != 1 || lines[0].Name != "fresh" || lines[0].Tier != tc.createDefault {
				t.Fatalf("estimate lines = %+v, want one %s line for the create only", lines, tc.createDefault)
			}

			// BEX_REQUIRE_PAYMENT_METHOD=1: a card for paid intent only.
			gate := &rejectingPaymentGate{}
			svc.Payment = gate
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: keepsOnly}); err != nil || len(gate.calls) != 0 {
				t.Fatalf("sync that only keeps a free resource: err=%v gate=%v, want applied without the payment gate", err, gate.calls)
			}
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: withCreate}); !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 {
				t.Fatalf("sync that also creates: err=%v gate=%v, want the payment gate", err, gate.calls)
			}
		})
	}
}

// The Blueprint sync's own preflight (prepareSyncManifest) runs the same
// gate before it stages the file, so a cardless workspace's sync of a file
// that only keeps its free resource is applied, not refused.
func TestABlueprintSyncThatOnlyKeepsAFreeResourceAsksForNoCard(t *testing.T) {
	for _, tc := range keptPlanKinds {
		t.Run(tc.kind, func(t *testing.T) {
			svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
			ctx := paidGateContext()
			created := tc.top + ":\n" + tc.entry("kept", "free")
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: created}); err != nil {
				t.Fatalf("create the free resource: %v", err)
			}
			blueprints := newFakeBlueprintStore(store.Blueprint{ID: "blp-kept", TenantID: "tea-a", Manifest: created, Status: "active"})
			svc.Blueprints = blueprints
			gate := &rejectingPaymentGate{}
			svc.Payment = gate
			keepsOnly := tc.top + ":\n" + tc.entry("kept", "")
			if _, err := svc.SyncBlueprint(ctx, "blp-kept", "tea-a", keepsOnly, "", nil); err != nil || len(gate.calls) != 0 {
				t.Fatalf("Blueprint sync that only keeps a free resource: err=%v gate=%v, want applied without the payment gate", err, gate.calls)
			}
			if got := blueprints.blueprints["blp-kept"].Manifest; got != keepsOnly {
				t.Fatalf("stored manifest = %q, want the synced file", got)
			}
		})
	}
}

// The kept plan is the resource's own, not merely "not the default": an
// existing resource on another paid plan is priced on that plan.
func TestAnExistingPaidResourceThatOmitsPlanIsPricedAtItsOwnPlan(t *testing.T) {
	for _, tc := range keptPlanKinds {
		t.Run(tc.kind, func(t *testing.T) {
			svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
			ctx := paidGateContext()
			if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.top + ":\n" + tc.entry("kept", tc.otherPaid)}); err != nil {
				t.Fatalf("create the %s resource: %v", tc.otherPaid, err)
			}
			v, err := svc.ValidateBlueprint(ctx, "", tc.top+":\n"+tc.entry("kept", ""), "")
			if err != nil || !v.Valid || v.EstimatedPricing == nil {
				t.Fatalf("ValidateBlueprint = %+v, %v", v, err)
			}
			if lines := v.EstimatedPricing.Lines; len(lines) != 1 || lines[0].Tier != tc.otherPaid {
				t.Fatalf("estimate lines = %+v, want one %s line", lines, tc.otherPaid)
			}
		})
	}
}

// The gate and the estimate read one kept-plan stack, so they still agree
// (blueprint_pricing_agreement_test.go) once live plans are read: paid intent
// exactly when the estimate shows something paid.
func TestKeptPlansKeepTheGateAndTheEstimateInAgreement(t *testing.T) {
	for _, tc := range keptPlanKinds {
		for _, live := range []string{"free", tc.otherPaid} {
			t.Run(tc.kind+" on "+live, func(t *testing.T) {
				svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
				ctx := paidGateContext()
				if _, err := svc.DeployStack(ctx, DeployRequest{Manifest: tc.top + ":\n" + tc.entry("kept", live)}); err != nil {
					t.Fatalf("create: %v", err)
				}
				st := parseBlueprintStackForTest(t, tc.top+":\n"+tc.entry("kept", ""))
				kept, err := svc.stackAtKeptPlans(ctx, st, svc.newWorkspaceSnapshot(ctx))
				if err != nil {
					t.Fatal(err)
				}
				est := blueprintEstimatedPricing(kept)
				paidContent := len(est.Lines) > 0 || len(est.Variable) > 0
				if paid := stackHasPaidPlan(kept); paid != paidContent || paid != (live != "free") {
					t.Fatalf("gate=%v estimate=%+v for a resource on %s", paid, est, live)
				}
				// The parsed stack the apply writes from is left as parsed.
				if !stackHasPaidPlan(st) {
					t.Fatalf("stackAtKeptPlans changed the parsed stack: %+v", st)
				}
			})
		}
	}
}

// A service with no field record is re-applied whole, plan included
// (applyBlueprintServiceSpec), so it keeps nothing and prices at the parse.
func TestAServiceWithNoFieldRecordIsPricedAsParsed(t *testing.T) {
	existing := tenantApp("web", "tea-a")
	existing.Spec.Tier = "free"
	svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"}, existing)
	ctx := paidGateContext()
	st := parsedStack{services: []parsedService{{req: CreateRequest{Name: "web", Type: appv1alpha1.TypeWebService}}}}
	kept, err := svc.stackAtKeptPlans(ctx, st, svc.newWorkspaceSnapshot(ctx))
	if err != nil || !stackHasPaidPlan(kept) {
		t.Fatalf("stackAtKeptPlans = %+v, %v; want the parse's paid default", kept, err)
	}
}
