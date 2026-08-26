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

package usage

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/billing"
	"github.com/bex-co/bex/lego/backend/internal/core"
)

// fakeBillingReader is a stub usage.BillingReader — no Stripe call needed.
type fakeBillingReader struct {
	result *billing.Billing
	err    error
	calls  []string
}

func (f *fakeBillingReader) BillingFor(_ context.Context, customerID string, _, _ time.Time) (*billing.Billing, error) {
	f.calls = append(f.calls, customerID)
	return f.result, f.err
}

func sampleBilling() *billing.Billing {
	return &billing.Billing{
		// A period where credit absorbed part of the charge — the shape that
		// broke when one field had to be both the charge and the amount due
		// (w6/m98), so every surface test carries it.
		CurrentCost: &billing.Amount{AmountUSD: "12.34", CreditsAppliedUSD: "10.00", AmountDueUSD: "2.34", Currency: "USD", PeriodStart: "2026-07-01T00:00:00Z", PeriodEnd: "2026-07-20T00:00:00Z"},
		Invoices: []billing.Invoice{
			{ID: "inv_1", Status: "FINALIZED", AmountUSD: "40.00", CreditsAppliedUSD: "25.00", AmountDueUSD: "15.00", Currency: "USD", PeriodStart: "2026-06-01T00:00:00Z", PeriodEnd: "2026-07-01T00:00:00Z"},
		},
		Credits: &billing.Credits{
			AvailableUSD: "25.00", Currency: "USD",
			Grants: []billing.CreditGrant{{Name: "welcome", RemainingUSD: "25.00", ExpiresAt: "2026-11-15T00:00:00Z"}},
		},
	}
}

func withIdentity(ctx context.Context) context.Context {
	return core.WithIdentity(ctx, core.Identity{Subject: "user:alice"})
}

func TestBillingAttachedToSummary(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	reader := &fakeBillingReader{result: sampleBilling()}
	svc.Billing = reader

	sum, err := svc.MonthToDate(withIdentity(context.Background()), "")
	if err != nil {
		t.Fatalf("MonthToDate: %v", err)
	}
	if sum.Billing == nil || sum.Billing.CurrentCost == nil || sum.Billing.CurrentCost.AmountUSD != "12.34" {
		t.Fatalf("summary.Billing = %+v, want the sample current cost", sum.Billing)
	}
	if len(reader.calls) != 1 || reader.calls[0] != "tea-001" {
		t.Fatalf("BillingFor called with %v, want [tea-001]", reader.calls)
	}
}

func TestBillingDegradedFallsBackToEstimateOnly(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	svc.Billing = &fakeBillingReader{err: errors.New("stripe down")}

	// A degraded read must not error the usage verb — it drops to estimate-only.
	sum, err := svc.MonthToDate(withIdentity(context.Background()), "")
	if err != nil {
		t.Fatalf("MonthToDate on degraded billing = %v, want nil (estimate-only)", err)
	}
	if sum.Billing != nil {
		t.Fatalf("summary.Billing = %+v, want nil on degraded read", sum.Billing)
	}
	if sum.EstimatedCost.TotalUSD == "" {
		t.Error("estimatedCost should remain present when billing degrades")
	}
}

func TestBillingCachedWithinTTL(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	svc.billingCache = core.NewTTLCache[*billing.Billing]()
	reader := &fakeBillingReader{result: sampleBilling()}
	svc.Billing = reader

	ctx := withIdentity(context.Background())
	for i := 0; i < 3; i++ {
		if _, err := svc.MonthToDate(ctx, ""); err != nil {
			t.Fatalf("MonthToDate #%d: %v", i, err)
		}
	}
	// Three usage reads (as the three surfaces / repeated polls would do) hit
	// Stripe once — the rest are served from the TTL cache.
	if len(reader.calls) != 1 {
		t.Fatalf("BillingFor called %d times, want 1 (cached within TTL)", len(reader.calls))
	}
}

func TestBillingErrorNotCached(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	svc.billingCache = core.NewTTLCache[*billing.Billing]()
	reader := &fakeBillingReader{err: errors.New("stripe down")}
	svc.Billing = reader

	ctx := withIdentity(context.Background())
	for i := 0; i < 2; i++ {
		if _, err := svc.MonthToDate(ctx, ""); err != nil {
			t.Fatalf("MonthToDate #%d: %v", i, err)
		}
	}
	// A degraded read is never cached, so the next poll retries (poll-as-retry).
	if len(reader.calls) != 2 {
		t.Fatalf("BillingFor called %d times, want 2 (errors not cached)", len(reader.calls))
	}
}

func TestBillingNilReaderIsEstimateOnly(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001") // no Billing wired
	sum, err := svc.MonthToDate(withIdentity(context.Background()), "")
	if err != nil {
		t.Fatalf("MonthToDate: %v", err)
	}
	if sum.Billing != nil {
		t.Fatalf("summary.Billing = %+v, want nil when no reader is wired", sum.Billing)
	}
}

// billingRoleChecker allows the workspace read relation and, when billing is
// true, the billing relation — modeling a viewer/contributor versus a billing
// member (or admin) of the same workspace.
type billingRoleChecker struct{ billing bool }

func (c billingRoleChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	if relation == core.RelCanView {
		return true, nil
	}
	return c.billing && relation == core.RelCanManageBilling, nil
}

// TestBillingHiddenFromNonBillingRoles (round-13 #5): usage + the advisory
// estimate stay readable by every can_view member, but the real Stripe
// projection (current cost, invoices, credit grants) is a can_manage_billing
// capability — a viewer/contributor/developer gets a null billing object and
// the billing reader is never even consulted.
func TestBillingHiddenFromNonBillingRoles(t *testing.T) {
	for _, tc := range []struct {
		name    string
		billing bool
	}{
		{"viewer/contributor/developer", false},
		{"billing member", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := svcWithTenant(seedStore(), "tea-001")
			svc.Base.Authz = billingRoleChecker{billing: tc.billing}
			reader := &fakeBillingReader{result: sampleBilling()}
			svc.Billing = reader

			sum, err := svc.MonthToDate(withIdentity(context.Background()), "")
			if err != nil {
				t.Fatalf("MonthToDate: %v", err)
			}
			if sum.EstimatedCost.TotalUSD == "" {
				t.Error("estimate must stay present for every can_view member")
			}
			if !tc.billing && sum.Billing != nil {
				t.Fatalf("summary.Billing = %+v, want nil for a non-billing role", sum.Billing)
			}
			if !tc.billing && len(reader.calls) != 0 {
				t.Fatalf("BillingFor called %d time(s) for a non-billing role, want 0", len(reader.calls))
			}
			if tc.billing && (sum.Billing == nil || sum.Billing.CurrentCost == nil) {
				t.Fatalf("summary.Billing = %+v, want the sample for a billing member", sum.Billing)
			}
		})
	}
}

// A billing member revoked moments ago (cached positive on another replica)
// must not receive the projection — the reveal uses the authoritative fresh
// decision, the round-5 finding-4 class.
type staleBillingChecker struct{ billingRoleChecker }

func (c staleBillingChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	return false, nil // just-revoked on the authoritative path
}

func TestBillingRevokedMidCacheWindow(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	svc.Base.Authz = staleBillingChecker{billingRoleChecker{billing: true}}
	reader := &fakeBillingReader{result: sampleBilling()}
	svc.Billing = reader

	sum, err := svc.MonthToDate(withIdentity(context.Background()), "")
	if err != nil {
		t.Fatalf("MonthToDate: %v", err)
	}
	if sum.Billing != nil {
		t.Fatalf("summary.Billing = %+v, want nil for a just-revoked billing member", sum.Billing)
	}
}

// TestBillingHiddenFromReadOnlyOAuthBillingToken (capability composition on the
// audit-silent projection): mayManageBilling composes can_manage_billing's
// mapped OAuth capability, so a third-party human token delegated only
// bex.read by a billing-role member must NOT receive the real Stripe
// projection (cost, invoices, credit grants) even though OpenFGA still grants
// the billing relation. bex.write restores it.
func TestBillingHiddenFromReadOnlyOAuthBillingToken(t *testing.T) {
	oauthBilling := func(scopes string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{
			Subject: "user:alice", Method: "oauth2", Human: true, CanonicalScopes: scopes,
		})
	}
	for _, tc := range []struct {
		name   string
		scopes string
		want   bool
	}{
		{"bex.read only", core.ScopeRead, false},
		{"bex.read + bex.write", core.ScopeRead + " " + core.ScopeWrite, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := svcWithTenant(seedStore(), "tea-001")
			svc.Base.Authz = billingRoleChecker{billing: true}
			reader := &fakeBillingReader{result: sampleBilling()}
			svc.Billing = reader

			sum, err := svc.MonthToDate(oauthBilling(tc.scopes), "")
			if err != nil {
				t.Fatalf("MonthToDate: %v", err)
			}
			if !tc.want {
				if sum.Billing != nil {
					t.Fatalf("summary.Billing = %+v, want nil for a read-only OAuth billing token", sum.Billing)
				}
				if len(reader.calls) != 0 {
					t.Fatalf("BillingFor called %d time(s), want 0 for a read-only OAuth billing token", len(reader.calls))
				}
			}
			if tc.want && (sum.Billing == nil || sum.Billing.CurrentCost == nil) {
				t.Fatalf("summary.Billing = %+v, want the sample for a write-scoped billing token", sum.Billing)
			}
		})
	}
}

// TestBillingCrossSurfaceParity asserts REST, GraphQL, and MCP present the same
// billing fields with the same values — the ADR006 one-core/thin-adapters
// invariant for the m48 surface.
func TestBillingCrossSurfaceParity(t *testing.T) {
	newSvc := func() *Service {
		s := svcWithTenant(seedStore(), "tea-001")
		s.Billing = &fakeBillingReader{result: sampleBilling()}
		return s
	}
	ctx := withIdentity(context.Background())

	// REST (also the MCP shape — get_usage returns the same toUsageResponse).
	mux := http.NewServeMux()
	newSvc().RegisterREST(mux)
	req := httptest.NewRequest("GET", "/v1/usage", nil).WithContext(ctx)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("REST %d: %s", w.Code, w.Body.String())
	}
	var rest usageResponse
	if err := json.NewDecoder(w.Body).Decode(&rest); err != nil {
		t.Fatalf("decode REST: %v", err)
	}
	if rest.Billing == nil || rest.Billing.CurrentCost.AmountUSD != "12.34" || len(rest.Billing.Invoices) != 1 {
		t.Fatalf("REST billing = %+v", rest.Billing)
	}
	// Value correctness, not just agreement: three surfaces reporting the same
	// wrong number is exactly how the netted-to-zero total went unnoticed. The
	// gross charge less the credit applied must be what is actually due.
	if cur := rest.Billing.CurrentCost; cur.CreditsAppliedUSD != "10.00" || cur.AmountDueUSD != "2.34" {
		t.Fatalf("REST currentCost = %+v, want the credit and due figures carried separately", cur)
	}
	if rest.Billing.Invoices[0].Status != "FINALIZED" || rest.Billing.Invoices[0].AmountUSD != "40.00" {
		t.Fatalf("REST invoice = %+v", rest.Billing.Invoices[0])
	}
	if inv := rest.Billing.Invoices[0]; inv.CreditsAppliedUSD != "25.00" || inv.AmountDueUSD != "15.00" {
		t.Fatalf("REST invoice = %+v, want the credit and due figures carried separately", inv)
	}
	if rest.Billing.Credits == nil || rest.Billing.Credits.AvailableUSD != "25.00" || len(rest.Billing.Credits.Grants) != 1 {
		t.Fatalf("REST credits = %+v", rest.Billing.Credits)
	}

	// GraphQL: the same values under usage.billing.
	schema, err := buildTestSchema(newSvc())
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       ctx,
		RequestString: `{ usage { billing { currentCost { amountUsd creditsAppliedUsd amountDueUsd currency } invoices { id status amountUsd creditsAppliedUsd amountDueUsd } credits { availableUsd currency grants { name remainingUsd expiresAt } } } } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql errors: %v", res.Errors)
	}
	gqlBilling := res.Data.(map[string]any)["usage"].(map[string]any)["billing"].(map[string]any)
	gqlCurrent := gqlBilling["currentCost"].(map[string]any)
	if gqlCurrent["amountUsd"] != rest.Billing.CurrentCost.AmountUSD ||
		gqlCurrent["creditsAppliedUsd"] != rest.Billing.CurrentCost.CreditsAppliedUSD ||
		gqlCurrent["amountDueUsd"] != rest.Billing.CurrentCost.AmountDueUSD {
		t.Errorf("GraphQL currentCost = %v, REST = %+v (surfaces disagree)", gqlCurrent, rest.Billing.CurrentCost)
	}
	gqlInvoices := gqlBilling["invoices"].([]any)
	if len(gqlInvoices) != len(rest.Billing.Invoices) {
		t.Fatalf("GraphQL invoices %d, REST %d", len(gqlInvoices), len(rest.Billing.Invoices))
	}
	gqlInv0 := gqlInvoices[0].(map[string]any)
	if gqlInv0["status"] != rest.Billing.Invoices[0].Status || gqlInv0["amountUsd"] != rest.Billing.Invoices[0].AmountUSD ||
		gqlInv0["creditsAppliedUsd"] != rest.Billing.Invoices[0].CreditsAppliedUSD ||
		gqlInv0["amountDueUsd"] != rest.Billing.Invoices[0].AmountDueUSD {
		t.Errorf("GraphQL invoice %v vs REST %+v (surfaces disagree)", gqlInv0, rest.Billing.Invoices[0])
	}
	gqlCredits := gqlBilling["credits"].(map[string]any)
	if gqlCredits["availableUsd"] != rest.Billing.Credits.AvailableUSD || gqlCredits["currency"] != rest.Billing.Credits.Currency {
		t.Errorf("GraphQL credits %v vs REST %+v (surfaces disagree)", gqlCredits, rest.Billing.Credits)
	}
	gqlGrant0 := gqlCredits["grants"].([]any)[0].(map[string]any)
	restGrant0 := rest.Billing.Credits.Grants[0]
	if gqlGrant0["name"] != restGrant0.Name || gqlGrant0["remainingUsd"] != restGrant0.RemainingUSD || gqlGrant0["expiresAt"] != restGrant0.ExpiresAt {
		t.Errorf("GraphQL grant %v vs REST %+v (surfaces disagree)", gqlGrant0, restGrant0)
	}
}
