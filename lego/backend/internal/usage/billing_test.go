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
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

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
			{ID: "inv_1", Kind: billing.InvoiceKindAgentModelCharge, Status: "FINALIZED", AmountUSD: "40.00", CreditsAppliedUSD: "25.00", AmountDueUSD: "15.00", Currency: "USD", PeriodStart: "2026-06-01T00:00:00Z", PeriodEnd: "2026-07-01T00:00:00Z"},
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

// billingRoleChecker answers the relations model.fga grants one workspace role
// (w9/m620): every member holds can_view; Admin, Developer and Billing hold
// can_view_billing (Render's "View billing details"); Admin and Billing hold
// can_manage_billing ("Edit payment method").
type billingRoleChecker struct{ role string }

func (c billingRoleChecker) Check(_ context.Context, _, relation, _ string) (bool, error) {
	switch relation {
	case core.RelCanView:
		return true, nil
	case core.RelCanViewBilling:
		return c.role == "admin" || c.role == "developer" || c.role == "billing", nil
	case core.RelCanManageBilling:
		return c.role == "admin" || c.role == "billing", nil
	}
	return false, nil
}

// TestBillingDetailsPerRole is Render's "View billing details" row
// (render.com/docs/team-members, read 2026-11-23) on the usage read (w9/m620):
// Admin, Developer and Billing read usage, the estimate and the real Stripe
// projection (current cost, invoices, credit grants); a Contributor and a
// Viewer, which Render says "can't view sensitive fields (billing info, …)",
// are refused the whole read and the billing reader is never consulted.
func TestBillingDetailsPerRole(t *testing.T) {
	for _, tc := range []struct {
		role string
		read bool
	}{
		{"admin", true},
		{"developer", true},
		{"billing", true},
		{"contributor", false},
		{"viewer", false},
	} {
		t.Run(tc.role, func(t *testing.T) {
			svc := svcWithTenant(seedStore(), "tea-001")
			svc.Base.Authz = billingRoleChecker{role: tc.role}
			reader := &fakeBillingReader{result: sampleBilling()}
			svc.Billing = reader

			sum, err := svc.MonthToDate(withIdentity(context.Background()), "")
			if !tc.read {
				if !errors.Is(err, core.ErrForbidden) {
					t.Fatalf("MonthToDate = %v, want ErrForbidden for a %s", err, tc.role)
				}
				if sum.EstimatedCost.TotalUSD != "" || len(sum.Services) != 0 {
					t.Fatalf("a refused read returned figures: %+v", sum)
				}
				if len(reader.calls) != 0 {
					t.Fatalf("BillingFor called %d time(s) for a %s, want 0", len(reader.calls), tc.role)
				}
				return
			}
			if err != nil {
				t.Fatalf("MonthToDate: %v", err)
			}
			if sum.EstimatedCost.TotalUSD == "" {
				t.Error("estimate missing for a member who may view billing details")
			}
			if sum.Billing == nil || sum.Billing.CurrentCost == nil || len(sum.Billing.Invoices) == 0 {
				t.Fatalf("summary.Billing = %+v, want the real cost and invoices for a %s", sum.Billing, tc.role)
			}
		})
	}
}

// A member revoked moments ago (cached positive on another replica) must not
// receive the real Stripe projection — the reveal re-asserts can_view_billing
// on the authoritative fresh path, the round-5 finding-4 class. The usage the
// cached gate already admitted stays estimate-only.
type staleBillingChecker struct{ billingRoleChecker }

func (c staleBillingChecker) CheckFresh(_ context.Context, _, relation, _ string) (bool, error) {
	return false, nil // just-revoked on the authoritative path
}

func TestBillingRevokedMidCacheWindow(t *testing.T) {
	svc := svcWithTenant(seedStore(), "tea-001")
	svc.Base.Authz = staleBillingChecker{billingRoleChecker{role: "developer"}}
	reader := &fakeBillingReader{result: sampleBilling()}
	svc.Billing = reader

	sum, err := svc.MonthToDate(withIdentity(context.Background()), "")
	if err != nil {
		t.Fatalf("MonthToDate: %v", err)
	}
	if sum.Billing != nil || len(reader.calls) != 0 {
		t.Fatalf("summary.Billing = %+v (reader calls %d), want no projection for a just-revoked member", sum.Billing, len(reader.calls))
	}
}

// TestBillingDetailsOAuthCapability: can_view_billing maps to bex.read
// (w9/m620), the capability every read takes, so a third-party human token a
// Developer delegated bex.read to reads the usage and the projection their
// role reads; a token without bex.read is refused the read with the coded
// insufficient_token_scope, never served figures. OpenFGA still decides the
// role: a Contributor's bex.read token is refused (TestBillingDetailsPerRole's
// checker, through the same gate).
func TestBillingDetailsOAuthCapability(t *testing.T) {
	oauth := func(scopes string) context.Context {
		return core.WithIdentity(context.Background(), core.Identity{
			Subject: "user:alice", Method: "oauth2", Human: true, CanonicalScopes: scopes,
		})
	}
	for _, tc := range []struct {
		name   string
		role   string
		scopes string
		want   error
	}{
		{"developer, bex.read", "developer", core.ScopeRead, nil},
		{"developer, bex.write only", "developer", core.ScopeWrite, core.ErrForbidden},
		{"contributor, bex.read", "contributor", core.ScopeRead, core.ErrForbidden},
	} {
		t.Run(tc.name, func(t *testing.T) {
			svc := svcWithTenant(seedStore(), "tea-001")
			svc.Base.Authz = billingRoleChecker{role: tc.role}
			reader := &fakeBillingReader{result: sampleBilling()}
			svc.Billing = reader

			sum, err := svc.MonthToDate(oauth(tc.scopes), "")
			if tc.want != nil {
				if !errors.Is(err, tc.want) {
					t.Fatalf("MonthToDate = %v, want %v", err, tc.want)
				}
				if len(reader.calls) != 0 {
					t.Fatalf("BillingFor called %d time(s) for a refused token, want 0", len(reader.calls))
				}
				return
			}
			if err != nil {
				t.Fatalf("MonthToDate: %v", err)
			}
			if sum.Billing == nil || sum.Billing.CurrentCost == nil {
				t.Fatalf("summary.Billing = %+v, want the projection for a Developer's bex.read token", sum.Billing)
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
		RequestString: `{ usage { billing { currentCost { amountUsd creditsAppliedUsd amountDueUsd currency } invoices { id status kind amountUsd creditsAppliedUsd amountDueUsd } credits { availableUsd currency grants { name remainingUsd expiresAt } } } } }`,
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
	if gqlInv0["status"] != rest.Billing.Invoices[0].Status || gqlInv0["kind"] != rest.Billing.Invoices[0].Kind ||
		gqlInv0["amountUsd"] != rest.Billing.Invoices[0].AmountUSD ||
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

// invoiceLinksBilling is a history with one open invoice Stripe gave both
// links and one void invoice it gave none (w6/m625).
func invoiceLinksBilling() *billing.Billing {
	return &billing.Billing{
		Invoices: []billing.Invoice{
			{ID: "in_open", Kind: billing.InvoiceKindCycle, Status: "OPEN", AmountUSD: "40.00", CreditsAppliedUSD: "0.00", AmountDueUSD: "40.00", Currency: "USD",
				HostedInvoiceURL: "https://invoice.stripe.com/i/acct_1/test_open", InvoicePDFURL: "https://pay.stripe.com/invoice/acct_1/test_open/pdf"},
			{ID: "in_void", Kind: billing.InvoiceKindCycle, Status: "VOID", AmountUSD: "5.00", CreditsAppliedUSD: "0.00", AmountDueUSD: "5.00", Currency: "USD"},
		},
	}
}

// usageOverMCP calls get_usage on svc over an in-memory MCP transport.
func usageOverMCP(t *testing.T, ctx context.Context, svc *Service) *mcp.CallToolResult {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "usage-test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatalf("MCP server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "usage-test-client", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatalf("MCP client connect: %v", err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	result, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "get_usage"})
	if err != nil {
		t.Fatalf("MCP get_usage: %v", err)
	}
	return result
}

// rawInvoices is the billing.invoices array of a usage response body, each
// invoice as the JSON object a client decodes.
func rawInvoices(t *testing.T, body []byte) []map[string]any {
	t.Helper()
	var resp struct {
		Billing *struct {
			Invoices []map[string]any `json:"invoices"`
		} `json:"billing"`
	}
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	if resp.Billing == nil {
		t.Fatalf("no billing object in %s", body)
	}
	return resp.Billing.Invoices
}

// TestInvoiceLinksAgreeAcrossSurfaces is w6/m625's adapter contract: REST
// GET /v1/usage, GraphQL BillingInvoice and MCP get_usage carry the same
// hostedInvoiceUrl and invoicePdfUrl for the same invoice, and a link Stripe
// did not give is absent on REST and MCP and null on GraphQL, never "".
func TestInvoiceLinksAgreeAcrossSurfaces(t *testing.T) {
	newSvc := func() *Service {
		s := svcWithTenant(seedStore(), "tea-001")
		s.Billing = &fakeBillingReader{result: invoiceLinksBilling()}
		return s
	}
	ctx := withIdentity(context.Background())
	want := invoiceLinksBilling().Invoices

	checkJSON := func(surface string, invoices []map[string]any) {
		t.Helper()
		if len(invoices) != 2 {
			t.Fatalf("%s invoices = %v, want 2", surface, invoices)
		}
		if invoices[0]["hostedInvoiceUrl"] != want[0].HostedInvoiceURL || invoices[0]["invoicePdfUrl"] != want[0].InvoicePDFURL {
			t.Errorf("%s open invoice = %v, want Stripe's two links", surface, invoices[0])
		}
		for _, key := range []string{"hostedInvoiceUrl", "invoicePdfUrl"} {
			if v, ok := invoices[1][key]; ok {
				t.Errorf("%s void invoice carries %s = %q, want it omitted", surface, key, v)
			}
		}
	}

	mux := http.NewServeMux()
	newSvc().RegisterREST(mux)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, httptest.NewRequest("GET", "/v1/usage", nil).WithContext(ctx))
	if w.Code != http.StatusOK {
		t.Fatalf("REST %d: %s", w.Code, w.Body.String())
	}
	checkJSON("REST", rawInvoices(t, w.Body.Bytes()))

	result := usageOverMCP(t, ctx, newSvc())
	if result.IsError {
		t.Fatalf("MCP get_usage error: %+v", result.Content)
	}
	rawMCP, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("MCP marshal: %v", err)
	}
	checkJSON("MCP", rawInvoices(t, rawMCP))

	schema, err := buildTestSchema(newSvc())
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{
		Schema:        schema,
		Context:       ctx,
		RequestString: `{ usage { billing { invoices { id hostedInvoiceUrl invoicePdfUrl } } } }`,
	})
	if len(res.Errors) > 0 {
		t.Fatalf("graphql errors: %v", res.Errors)
	}
	gql := res.Data.(map[string]any)["usage"].(map[string]any)["billing"].(map[string]any)["invoices"].([]any)
	open, void := gql[0].(map[string]any), gql[1].(map[string]any)
	if open["hostedInvoiceUrl"] != want[0].HostedInvoiceURL || open["invoicePdfUrl"] != want[0].InvoicePDFURL {
		t.Errorf("GraphQL open invoice = %v, want Stripe's two links", open)
	}
	if void["hostedInvoiceUrl"] != nil || void["invoicePdfUrl"] != nil {
		t.Errorf("GraphQL void invoice = %v, want both links null", void)
	}
}

// TestInvoiceLinksWithheldFromMembersWhoCannotViewBilling: a Contributor and a
// Viewer, who Render says "can't view sensitive fields (billing info, …)",
// read neither invoice link on REST, GraphQL or MCP (w9/m620, w6/m625).
func TestInvoiceLinksWithheldFromMembersWhoCannotViewBilling(t *testing.T) {
	links := invoiceLinksBilling().Invoices[0]
	for _, role := range []string{"contributor", "viewer"} {
		t.Run(role, func(t *testing.T) {
			newSvc := func() *Service {
				s := svcWithTenant(seedStore(), "tea-001")
				s.Base.Authz = billingRoleChecker{role: role}
				s.Billing = &fakeBillingReader{result: invoiceLinksBilling()}
				return s
			}
			ctx := withIdentity(context.Background())
			leaks := func(surface, out string) {
				t.Helper()
				if strings.Contains(out, links.HostedInvoiceURL) || strings.Contains(out, links.InvoicePDFURL) {
					t.Errorf("%s gave a %s an invoice link: %s", surface, role, out)
				}
			}

			mux := http.NewServeMux()
			newSvc().RegisterREST(mux)
			w := httptest.NewRecorder()
			mux.ServeHTTP(w, httptest.NewRequest("GET", "/v1/usage", nil).WithContext(ctx))
			if w.Code != http.StatusForbidden {
				t.Errorf("REST %d for a %s, want 403", w.Code, role)
			}
			leaks("REST", w.Body.String())

			schema, err := buildTestSchema(newSvc())
			if err != nil {
				t.Fatalf("schema: %v", err)
			}
			res := graphql.Do(graphql.Params{
				Schema:        schema,
				Context:       ctx,
				RequestString: `{ usage { billing { invoices { hostedInvoiceUrl invoicePdfUrl } } } }`,
			})
			if len(res.Errors) == 0 {
				t.Errorf("GraphQL gave a %s no error", role)
			}
			gqlOut, _ := json.Marshal(res)
			leaks("GraphQL", string(gqlOut))

			result := usageOverMCP(t, ctx, newSvc())
			if !result.IsError {
				t.Errorf("MCP get_usage succeeded for a %s", role)
			}
			mcpOut, _ := json.Marshal(result)
			leaks("MCP", string(mcpOut))
		})
	}
}
