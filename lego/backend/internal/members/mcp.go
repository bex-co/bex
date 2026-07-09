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
)

// mcp.go is the members MCP fragment (bex extension over Render's MCP, which
// manages members outside its tools). An agent can seat and manage teammates the
// same way a human does — the AI-native pillar applied to team management.

type workspaceArg struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"the workspace (team) id, tea-…"`
}

type listMembersResult struct {
	Members []MemberView `json:"members"`
}

type inviteMemberArgs struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"the workspace (team) id, tea-…"`
	Email       string `json:"email" jsonschema:"the invitee's email address"`
	Role        string `json:"role" jsonschema:"the role: VIEWER, CONTRIBUTOR, DEVELOPER, ADMIN, or BILLING"`
}

type changeRoleArgs struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"the workspace (team) id, tea-…"`
	Subject     string `json:"subject" jsonschema:"the member's subject (identity id)"`
	Role        string `json:"role" jsonschema:"the new role: VIEWER, CONTRIBUTOR, DEVELOPER, ADMIN, or BILLING"`
}

type removeMemberArgs struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"the workspace (team) id, tea-…"`
	Subject     string `json:"subject" jsonschema:"the member's subject (identity id)"`
}

type listInvitesResult struct {
	Invites []InviteView `json:"invites"`
}

type revokeInviteArgs struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"the workspace (team) id, tea-…"`
	InviteID    string `json:"inviteId" jsonschema:"the pending invite id, inv-…"`
}

type okResult struct {
	OK bool `json:"ok"`
}

// RegisterMCP adds the membership tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_workspace_members",
		Description: "List a workspace's members and their roles. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in workspaceArg) (*mcp.CallToolResult, listMembersResult, error) {
		ms, err := s.List(ctx, in.WorkspaceID)
		return nil, listMembersResult{Members: ms}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "invite_workspace_member",
		Description: "Invite a teammate to a workspace by email at a role; they join on first login. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in inviteMemberArgs) (*mcp.CallToolResult, InviteView, error) {
		inv, err := s.Invite(ctx, in.WorkspaceID, in.Email, in.Role)
		return nil, inv, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "change_workspace_member_role",
		Description: "Change a member's role in a workspace. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in changeRoleArgs) (*mcp.CallToolResult, MemberView, error) {
		m, err := s.ChangeRole(ctx, in.WorkspaceID, in.Subject, in.Role)
		return nil, m, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "remove_workspace_member",
		Description: "Remove a member from a workspace; their access is revoked. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in removeMemberArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.Remove(ctx, in.WorkspaceID, in.Subject)
		return nil, okResult{OK: err == nil}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_workspace_invites",
		Description: "List a workspace's outstanding (pending) member invites. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in workspaceArg) (*mcp.CallToolResult, listInvitesResult, error) {
		invs, err := s.ListInvites(ctx, in.WorkspaceID)
		return nil, listInvitesResult{Invites: invs}, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "revoke_workspace_invite",
		Description: "Revoke a pending workspace invite before it's accepted. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in revokeInviteArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.RevokeInvite(ctx, in.WorkspaceID, in.InviteID)
		return nil, okResult{OK: err == nil}, err
	})
}
