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

func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_billing_readiness",
		Description: "Get Stripe Billing onboarding readiness, including test/live mode, Customer/Subscription/payment-method state, whether the paid-intent gate is on (paymentMethodRequired), and fail-closed tax configuration. Requires the billing role or workspace admin. bex extension over Render's MCP.",
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
