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

// fakeBillingReader is a stub usage.BillingReader — no Metronome needed.
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
		CurrentCost: &billing.Amount{AmountUSD: "12.34", Currency: "USD", PeriodStart: "2026-07-01T00:00:00Z", PeriodEnd: "2026-07-20T00:00:00Z"},
		Invoices: []billing.Invoice{
			{ID: "inv_1", Status: "FINALIZED", AmountUSD: "40.00", Currency: "USD", PeriodStart: "2026-06-01T00:00:00Z", PeriodEnd: "2026-07-01T00:00:00Z"},
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
	svc.Billing = &fakeBillingReader{err: errors.New("metronome down")}

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
	// Metronome once — the rest are served from the TTL cache.
	if len(reader.calls) != 1 {
		t.Fatalf("BillingFor called %d times, want 1 (cached within TTL)", len(reader.calls))
	}
}

func TestBillingErrorNotCached(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	svc.billingCache = core.NewTTLCache[*billing.Billing]()
	reader := &fakeBillingReader{err: errors.New("metronome down")}
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
	if rest.Billing.Invoices[0].Status != "FINALIZED" || rest.Billing.Invoices[0].AmountUSD != "40.00" {
		t.Fatalf("REST invoice = %+v", rest.Billing.Invoices[0])
	}

	// GraphQL: the same values under usage.billing.
	schema, err := buildTestSchema(newSvc())
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       ctx,
		RequestString: `{ usage { billing { currentCost { amountUsd currency } invoices { id status amountUsd } } } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql errors: %v", res.Errors)
	}
	gqlBilling := res.Data.(map[string]any)["usage"].(map[string]any)["billing"].(map[string]any)
	gqlCurrent := gqlBilling["currentCost"].(map[string]any)
	if gqlCurrent["amountUsd"] != rest.Billing.CurrentCost.AmountUSD {
		t.Errorf("GraphQL currentCost.amountUsd = %v, REST = %v (surfaces disagree)", gqlCurrent["amountUsd"], rest.Billing.CurrentCost.AmountUSD)
	}
	gqlInvoices := gqlBilling["invoices"].([]any)
	if len(gqlInvoices) != len(rest.Billing.Invoices) {
		t.Fatalf("GraphQL invoices %d, REST %d", len(gqlInvoices), len(rest.Billing.Invoices))
	}
	gqlInv0 := gqlInvoices[0].(map[string]any)
	if gqlInv0["status"] != rest.Billing.Invoices[0].Status || gqlInv0["amountUsd"] != rest.Billing.Invoices[0].AmountUSD {
		t.Errorf("GraphQL invoice %v vs REST %+v (surfaces disagree)", gqlInv0, rest.Billing.Invoices[0])
	}
}
