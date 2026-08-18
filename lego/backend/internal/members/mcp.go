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

package members

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the members MCP fragment (bex extension over Render's MCP, which
// manages members outside its tools). An agent can seat and manage teammates the
// same way a human does — the AI-native pillar applied to team management.

type listMembersResult struct {
	Members []MemberView `json:"members"`
}

type inviteMemberArgs struct {
	Email string `json:"email" jsonschema:"the invitee's email address"`
	Role  string `json:"role" jsonschema:"the role: VIEWER, CONTRIBUTOR, DEVELOPER, ADMIN, or BILLING"`
}

type changeRoleArgs struct {
	Subject string `json:"subject" jsonschema:"the member's subject (identity id)"`
	Role    string `json:"role" jsonschema:"the new role: VIEWER, CONTRIBUTOR, DEVELOPER, ADMIN, or BILLING"`
}

type removeMemberArgs struct {
	Subject string `json:"subject" jsonschema:"the member's subject (identity id)"`
}

type listInvitesResult struct {
	Invites []InviteView `json:"invites"`
}

type revokeInviteArgs struct {
	InviteID string `json:"inviteId" jsonschema:"the pending invite id, inv-…"`
}

type acceptInviteArgs struct {
	Token string `json:"token" jsonschema:"the invite token from the invitation email's accept link"`
}

type okResult struct {
	OK bool `json:"ok"`
}

// RegisterMCP adds the membership tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_workspace_members",
		Description: "List a workspace's members, their roles, opaque userId (own-…), and email (when the identity provider is configured). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listMembersResult, error) {
		ms, err := s.List(ctx, core.NamedWorkspace(ctx))
		return nil, listMembersResult{Members: ms}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "invite_workspace_member",
		Description: "Invite a teammate to a workspace by email at a role; they join on first login. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in inviteMemberArgs) (*mcp.CallToolResult, InviteView, error) {
		inv, err := s.Invite(ctx, core.NamedWorkspace(ctx), in.Email, in.Role)
		return nil, inv, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "change_workspace_member_role",
		Description: "Change a member's role in a workspace. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in changeRoleArgs) (*mcp.CallToolResult, MemberView, error) {
		m, err := s.ChangeRole(ctx, core.NamedWorkspace(ctx), in.Subject, in.Role)
		return nil, m, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "remove_workspace_member",
		Description: "Remove a member from a workspace; their access is revoked. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in removeMemberArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.Remove(ctx, core.NamedWorkspace(ctx), in.Subject)
		return nil, okResult{OK: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_workspace_invites",
		Description: "List a workspace's outstanding (pending) member invites. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listInvitesResult, error) {
		invs, err := s.ListInvites(ctx, core.NamedWorkspace(ctx))
		return nil, listInvitesResult{Invites: invs}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "revoke_workspace_invite",
		Description: "Revoke a pending workspace invite before it's accepted. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in revokeInviteArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.RevokeInvite(ctx, core.NamedWorkspace(ctx), in.InviteID)
		return nil, okResult{OK: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_workspace_seat_usage",
		Description: "A workspace's member-seat usage: used counts accepted members plus outstanding invites; limit 0 means unlimited (paid plans). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, SeatUsageView, error) {
		u, err := s.SeatUsage(ctx, core.NamedWorkspace(ctx))
		return nil, u, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resend_workspace_invite",
		Description: "Re-send a pending workspace invite's email and refresh its expiry; the invite id is unchanged but the emailed link is freshly minted and supersedes the original. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in revokeInviteArgs) (*mcp.CallToolResult, InviteView, error) {
		inv, err := s.ResendInvite(ctx, core.NamedWorkspace(ctx), in.InviteID)
		return nil, inv, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_viewer_capabilities",
		Description: "The caller's own effective permissions in the active workspace: role plus canView/canOperate/canCreate/canViewSensitive/canManage/canManageBilling booleans. Answers \"what can I do here\" before attempting a verb. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, Capabilities, error) {
		caps, err := s.Capabilities(ctx, core.NamedWorkspace(ctx))
		return nil, caps, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "accept_workspace_invite",
		Description: "Redeem a workspace invite token for the authenticated caller — joins the inviting workspace at the invited role even when the caller signed up under a different email. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in acceptInviteArgs) (*mcp.CallToolResult, AcceptedInviteView, error) {
		acc, err := s.AcceptInvite(ctx, in.Token)
		return nil, acc, err
	})
}
