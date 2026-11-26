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

package workspaces

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// mcp.go exposes workspace discovery plus bex's resource-limit extension.
// Resource tools take Render's optional per-call workspaceId at the API
// composition root; there is deliberately no transport-session selection.

// RegisterMCP adds workspace discovery and limits to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_workspaces",
		Description: "List the workspaces the caller has access to, as Render's owner objects (id, name, email, type), each with its inbound IP rules (ipAllowList; the seeded allow-all when none were set). Pass a returned id as workspaceId on each resource-tool call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, any, error) {
		owners, err := s.ListOwners(ctx, OwnerFilter{})
		if err != nil {
			return nil, nil, err
		}
		// Render's text (pkg/owner/tools.go): the bare array of REST owner
		// objects, as GET /v1/owners carries them (w4/m378).
		out := make([]renderOwner, 0, len(owners))
		for _, o := range owners {
			out = append(out, toRenderOwner(o))
		}
		return mcputil.RenderText(out)
	})

	// get_workspace_limits — bex extension (w7/m9): surfaces per-workspace
	// resource usage vs. cap for agents that want to check headroom before
	// attempting a create. Authorizes can_view on the workspace.
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_workspace_limits",
		Description: "Get the resource usage and limits for a workspace (services, Postgres, key-value). Used is the current count and gates creates (it includes any resource still finishing deletion, which keeps holding quota); terminating is how many of used are mid-deletion; limit 0 means unlimited. pipelineMinutesExceeded is true while the workspace's builds are stopped for the month: it used its plan's included pipeline minutes and cannot buy more (no payment method, or its pipeline spend limit reached), so a new build or pre-deploy command is refused until then.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ResourceLimitsView, error) {
		limits, err := s.ResourceLimits(ctx, core.NamedWorkspace(ctx))
		if err != nil {
			return nil, ResourceLimitsView{}, err
		}
		return nil, limits, nil
	})

	// get_billing_email / update_billing_email — bex extension (w6/m627):
	// the workspace's billing email, as the Billing page reads and sets it.
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_billing_email",
		Description: "Get where the workspace's invoices and billing notices go: email is its owner's account email on Hobby, and on Pro or higher until the workspace sets its own; configurable says whether its plan lets it set one. Requires the Admin, Developer or Billing role, as Render's View billing details does. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, BillingEmailView, error) {
		out, err := s.BillingEmail(ctx, core.NamedWorkspace(ctx))
		return nil, out, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "update_billing_email",
		Description: "Set the workspace's billing email, which its Stripe customer and bex's billing notices then use. Pro plan or higher (PLAN_FEATURE_REQUIRED on Hobby, whose billing email is its owner's account email); requires the Admin or Billing role. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in billingEmailArgs) (*mcp.CallToolResult, BillingEmailView, error) {
		out, err := s.SetBillingEmail(ctx, core.NamedWorkspace(ctx), in.Email)
		return nil, out, err
	})

	s.registerOrganizationMCP(srv)
}

type billingEmailArgs struct {
	Email string `json:"email" jsonschema:"the billing email address"`
}
