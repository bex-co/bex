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

	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// enforcingBillingGate refuses every mutation for the named workspace, standing
// in for a Stripe dunning lifecycle in the enforced/recovering state.
type enforcingBillingGate struct {
	enforced string
	calls    []string
}

func (g *enforcingBillingGate) CheckBillingMutationAllowed(_ context.Context, workspaceID string) error {
	g.calls = append(g.calls, workspaceID)
	if workspaceID == g.enforced {
		return core.ErrBillingEnforced
	}
	return nil
}

// freeBlueprintStack is an all-free service + database stack: only a gate that
// ignores plans (dunning enforcement, ADR075 D7's all-plans card) can refuse it.
const freeBlueprintStack = `services:
  - name: free-web
    type: web
    runtime: image
    image: {url: nginx:alpine}
    plan: free
databases:
  - name: free-db
    plan: free
`

// TestBlueprintApplyIsRefusedUnderBillingEnforcement is the regression test for
// the security-audit run-1 finding: the Blueprint apply path once called only
// the payment-method half of the paid-intent gate (requireStackBilling ->
// RequirePaymentMethod) and never RequireBillingMutation, so a workspace under
// dunning enforcement could provision unlimited new resources via render.yaml.
//
// It uses a FREE manifest on purpose: dunning enforcement blocks ALL mutations
// regardless of plan, so a free stack is refused only when RequireBillingMutation
// is consulted. Against the pre-fix code (early return when no paid plan) the
// free stack is written and this test fails — which is exactly the proof.
func TestBlueprintApplyIsRefusedUnderBillingEnforcement(t *testing.T) {

	t.Run("deploy create refused, nothing written", func(t *testing.T) {
		svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		gate := &enforcingBillingGate{enforced: "tea-a"}
		svc.Billing = gate

		_, err := svc.DeployStack(paidGateContext(), DeployRequest{Manifest: freeBlueprintStack})
		if !errors.Is(err, core.ErrBillingEnforced) {
			t.Fatalf("enforced Blueprint deploy err = %v, want ErrBillingEnforced", err)
		}
		var apps appv1alpha1.AppList
		var databases appv1alpha1.DatabaseList
		if err := cl.List(context.Background(), &apps); err != nil {
			t.Fatal(err)
		}
		if err := cl.List(context.Background(), &databases); err != nil {
			t.Fatal(err)
		}
		if len(apps.Items) != 0 || len(databases.Items) != 0 {
			t.Fatalf("enforced Blueprint deploy wrote apps=%d databases=%d (billing bypass)", len(apps.Items), len(databases.Items))
		}
	})

	t.Run("sync refused, manifest not replaced", func(t *testing.T) {
		const originalManifest = `services:
  - name: free-web
    type: web
    runtime: image
    image: {url: nginx:alpine}
    plan: free
`
		blueprints := newFakeBlueprintStore(store.Blueprint{
			ID: "blp-enforced", TenantID: "tea-a", Manifest: originalManifest, Status: "active",
		})
		svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		svc.Billing = &enforcingBillingGate{enforced: "tea-a"}
		svc.Blueprints = blueprints

		_, err := svc.SyncBlueprint(paidGateContext(), "blp-enforced", "tea-a", freeBlueprintStack, "", nil)
		if !errors.Is(err, core.ErrBillingEnforced) {
			t.Fatalf("enforced Blueprint sync err = %v, want ErrBillingEnforced", err)
		}
		if got := blueprints.blueprints["blp-enforced"].Manifest; got != originalManifest {
			t.Fatalf("enforced Blueprint sync persisted replacement manifest %q", got)
		}
	})

	t.Run("good standing still applies", func(t *testing.T) {
		svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		// Enforcing a DIFFERENT workspace: tea-a is in good standing.
		svc.Billing = &enforcingBillingGate{enforced: "tea-other"}

		if _, err := svc.DeployStack(paidGateContext(), DeployRequest{Manifest: freeBlueprintStack}); err != nil {
			t.Fatalf("good-standing Blueprint deploy: %v", err)
		}
		var apps appv1alpha1.AppList
		if err := cl.List(context.Background(), &apps); err != nil {
			t.Fatal(err)
		}
		if len(apps.Items) == 0 {
			t.Fatal("good-standing Blueprint deploy wrote no Apps")
		}
	})
}

// TestBlueprintApplyHonorsAllPlansCardGate pins w5/072: with PaymentAllPlans
// (BEX_REQUIRE_PAYMENT_METHOD=all, production), ADR075 D7 requires a bound card
// for every create and every Blueprint apply, free plans included. The stack
// gate once asked for the card only when a resource was paid, so a cardless
// workspace created free services and databases through a Blueprint.
func TestBlueprintApplyHonorsAllPlansCardGate(t *testing.T) {
	written := func(t *testing.T, cl client.Client) int {
		t.Helper()
		var apps appv1alpha1.AppList
		var databases appv1alpha1.DatabaseList
		if err := cl.List(context.Background(), &apps); err != nil {
			t.Fatal(err)
		}
		if err := cl.List(context.Background(), &databases); err != nil {
			t.Fatal(err)
		}
		return len(apps.Items) + len(databases.Items)
	}

	t.Run("cardless deploy refused, nothing written", func(t *testing.T) {
		svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		gate := &rejectingPaymentGate{}
		svc.Payment, svc.PaymentAllPlans = gate, true
		_, err := svc.DeployStack(paidGateContext(), DeployRequest{Manifest: freeBlueprintStack})
		if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) != 1 || gate.calls[0] != "tea-a" {
			t.Fatalf("all-mode free Blueprint deploy err=%v gate=%v, want ErrPaymentRequired for tea-a", err, gate.calls)
		}
		if n := written(t, cl); n != 0 {
			t.Fatalf("cardless Blueprint deploy wrote %d resources", n)
		}
	})

	t.Run("cardless sync refused, manifest not replaced", func(t *testing.T) {
		const original = "services:\n  - name: free-web\n    type: web\n    runtime: image\n    image: {url: nginx:alpine}\n    plan: free\n"
		blueprints := newFakeBlueprintStore(store.Blueprint{ID: "blp-cardless", TenantID: "tea-a", Manifest: original, Status: "active"})
		svc, _ := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		svc.Payment, svc.PaymentAllPlans = &rejectingPaymentGate{}, true
		svc.Blueprints = blueprints
		if _, err := svc.SyncBlueprint(paidGateContext(), "blp-cardless", "tea-a", freeBlueprintStack, "", nil); !errors.Is(err, core.ErrPaymentRequired) {
			t.Fatalf("all-mode free Blueprint sync err = %v, want ErrPaymentRequired", err)
		}
		if got := blueprints.blueprints["blp-cardless"].Manifest; got != original {
			t.Fatalf("cardless Blueprint sync persisted replacement manifest %q", got)
		}
	})

	t.Run("cardless Blueprint connect refused", func(t *testing.T) {
		svc, _ := connectionService(t)
		gate := &rejectingPaymentGate{}
		svc.Payment, svc.PaymentAllPlans = gate, true
		_, err := svc.CreateBlueprint(ownershipCtx(), connOwner, CreateBlueprintRequest{Repo: repoBex, Branch: "main", Path: pathSite, Name: "bpA"})
		if !errors.Is(err, core.ErrPaymentRequired) || len(gate.calls) == 0 {
			t.Fatalf("all-mode Blueprint connect err=%v gate=%v, want ErrPaymentRequired", err, gate.calls)
		}
	})

	t.Run("bound workspace applies", func(t *testing.T) {
		svc, cl := newTenantService(fakeWorkspace{"identity-a": "tea-a"})
		gate := &acceptingPaymentGate{}
		svc.Payment, svc.PaymentAllPlans = gate, true
		if _, err := svc.DeployStack(paidGateContext(), DeployRequest{Manifest: freeBlueprintStack}); err != nil {
			t.Fatalf("bound all-mode Blueprint deploy: %v", err)
		}
		if n := written(t, cl); gate.calls == 0 || n == 0 {
			t.Fatalf("bound all-mode deploy consulted gate %d times and wrote %d resources", gate.calls, n)
		}
	})

}
