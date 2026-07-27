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
)

type billingWorkspaceArgs struct {
	WorkspaceID string `json:"workspaceId,omitempty" jsonschema:"workspace id; defaults to the selected or caller's default workspace"`
}

type billingCheckoutArgs struct {
	WorkspaceID string `json:"workspaceId,omitempty" jsonschema:"workspace id; defaults to the selected or caller's default workspace"`
	SuccessURL  string `json:"successUrl" jsonschema:"trusted dashboard URL after successful payment setup"`
	CancelURL   string `json:"cancelUrl" jsonschema:"trusted dashboard URL after cancellation"`
}

type billingPortalArgs struct {
	WorkspaceID string `json:"workspaceId,omitempty" jsonschema:"workspace id; defaults to the selected or caller's default workspace"`
	ReturnURL   string `json:"returnUrl" jsonschema:"trusted dashboard URL used to return from Stripe Customer Portal"`
}

func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_billing_readiness",
		Description: "Get workspace-admin Stripe Billing onboarding readiness, including test/live mode, Customer/Subscription/payment-method state, and fail-closed tax configuration. bex extension over Render's MCP.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in billingWorkspaceArgs) (*mcp.CallToolResult, Readiness, error) {
		workspaceID, err := core.SelectedWorkspace(ctx, s.Selections, req, in.WorkspaceID)
		if err != nil {
			return nil, Readiness{}, err
		}
		out, err := s.Status(ctx, workspaceID)
		return nil, out, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_billing_checkout_session",
		Description: "Create a short-lived Stripe-hosted setup-mode Checkout URL for the workspace's existing metered Subscription. Requires workspace admin and never returns a Stripe server credential.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in billingCheckoutArgs) (*mcp.CallToolResult, HostedSession, error) {
		workspaceID, err := core.SelectedWorkspace(ctx, s.Selections, req, in.WorkspaceID)
		if err != nil {
			return nil, HostedSession{}, err
		}
		out, err := s.Checkout(ctx, workspaceID, CheckoutRequest{SuccessURL: in.SuccessURL, CancelURL: in.CancelURL})
		return nil, out, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_billing_portal_session",
		Description: "Create a short-lived Stripe Customer Portal URL scoped to the workspace's Customer. Requires workspace admin and a trusted dashboard return URL.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in billingPortalArgs) (*mcp.CallToolResult, HostedSession, error) {
		workspaceID, err := core.SelectedWorkspace(ctx, s.Selections, req, in.WorkspaceID)
		if err != nil {
			return nil, HostedSession{}, err
		}
		out, err := s.Portal(ctx, workspaceID, PortalRequest{ReturnURL: in.ReturnURL})
		return nil, out, err
	})
}
