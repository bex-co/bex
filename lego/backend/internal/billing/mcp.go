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

package billing

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

type billingCheckoutArgs struct {
	SuccessURL string `json:"successUrl" jsonschema:"trusted dashboard URL after successful payment setup"`
	CancelURL  string `json:"cancelUrl" jsonschema:"trusted dashboard URL after cancellation"`
}

type billingPortalArgs struct {
	ReturnURL string `json:"returnUrl" jsonschema:"trusted dashboard URL used to return from Stripe Customer Portal"`
}

// billingInfoArgs replaces a workspace's billing info, as REST's PUT does.
type billingInfoArgs struct {
	CompanyName string              `json:"companyName,omitempty" jsonschema:"company name the invoices bill; empty bills the workspace's own name"`
	Address     *billingAddressArgs `json:"address,omitempty" jsonschema:"postal address the invoices print; omitted removes it"`
	TaxID       *BillingTaxID       `json:"taxId,omitempty" jsonschema:"VAT or tax ID, typed as a Stripe customer tax ID type such as eu_vat, gb_vat or us_ein; omitted removes it"`
}

// billingAddressArgs is BillingAddress with every line but the country
// optional.
type billingAddressArgs struct {
	Line1      string `json:"line1,omitempty"`
	Line2      string `json:"line2,omitempty"`
	City       string `json:"city,omitempty"`
	State      string `json:"state,omitempty"`
	PostalCode string `json:"postalCode,omitempty"`
	Country    string `json:"country" jsonschema:"ISO 3166-1 alpha-2 country code, such as DE or US"`
}

func (a billingInfoArgs) input() BillingInfoInput {
	in := BillingInfoInput{CompanyName: a.CompanyName, TaxID: a.TaxID}
	if a.Address != nil {
		address := BillingAddress(*a.Address)
		in.Address = &address
	}
	return in
}

func (s *Service) RegisterMCP(srv *mcp.Server) {
	// get_billing_info / update_billing_info — bex extension (w6/m628):
	// Render's public API and MCP have no billing read.
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_billing_info",
		Description: "Get the workspace's billing info: the company name, address and VAT or tax ID shown on every invoice, and billTo, the name invoices bill (the company name, else the workspace's name). customerReady is false until a payment method was first added. Requires the Admin, Developer or Billing role, as Render's View billing details does. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, BillingInfo, error) {
		out, err := s.BillingInfo(ctx, core.NamedWorkspace(ctx))
		return nil, out, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_billing_info",
		Description: "Replace the workspace's billing info shown on every later invoice: companyName (empty bills the workspace's name), address and taxId; an omitted address or taxId is removed. A field Stripe or bex refuses answers BILLING_INFO_INVALID naming it. Showing a tax ID collects no tax. Requires the Admin or Billing role. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in billingInfoArgs) (*mcp.CallToolResult, BillingInfo, error) {
		out, err := s.SetBillingInfo(ctx, core.NamedWorkspace(ctx), in.input())
		return nil, out, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_billing_readiness",
		Description: "Get Stripe Billing onboarding readiness, including test/live mode, Customer/Subscription/payment-method state, the billing lifecycle (grace, enforcement, recovery) with the overdue invoice and its hosted payment page (lifecycle.overdueInvoice) while one is owed, whether the paid-intent gate is on (paymentMethodRequired), whether this workspace must bind a payment method before any resource use (paymentMethodOnboardingRequired — the sign-up wall, `all` gate mode), and fail-closed tax configuration. Requires the Admin, Developer or Billing role, as Render's View billing details does. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, Readiness, error) {
		out, err := s.Status(ctx, core.NamedWorkspace(ctx))
		return nil, out, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_billing_checkout_session",
		Description: "Create a short-lived Stripe-hosted setup-mode Checkout URL for the workspace's existing metered Subscription. Requires the billing role or workspace admin and never returns a Stripe server credential.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in billingCheckoutArgs) (*mcp.CallToolResult, HostedSession, error) {
		out, err := s.Checkout(ctx, core.NamedWorkspace(ctx), CheckoutRequest{SuccessURL: in.SuccessURL, CancelURL: in.CancelURL})
		return nil, out, err
	})

	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_billing_portal_session",
		Description: "Create a short-lived Stripe Customer Portal URL scoped to the workspace's Customer. Requires the billing role or workspace admin and a trusted dashboard return URL.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in billingPortalArgs) (*mcp.CallToolResult, HostedSession, error) {
		out, err := s.Portal(ctx, core.NamedWorkspace(ctx), PortalRequest{ReturnURL: in.ReturnURL})
		return nil, out, err
	})
}
