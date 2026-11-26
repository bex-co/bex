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

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// organizations_api.go is the org membership fragment of the members GraphQL
// schema and MCP registry (w7/m464, ADR115 D8): the same verbs, fields and
// codes on both.

var organizationMemberGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "OrganizationMember",
	Fields: graphql.Fields{
		"subject":        gqlutil.ReqStrField(func(m OrganizationMemberView) any { return m.Subject }),
		"userId":         gqlutil.StrField(func(m OrganizationMemberView) any { return m.UserID }),
		"email":          gqlutil.StrField(func(m OrganizationMemberView) any { return m.Email }),
		"role":           gqlutil.ReqStrField(func(m OrganizationMemberView) any { return m.Role }),
		"createdAt":      gqlutil.StrField(func(m OrganizationMemberView) any { return m.CreatedAt }),
		"workspaces":     gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(graphql.String))), func(m OrganizationMemberView) any { return m.Workspaces }),
		"isSelf":         gqlutil.ReqBoolField(func(m OrganizationMemberView) any { return m.IsSelf }),
		"avatarDownload": gqlutil.AvatarDownloadField(func(m OrganizationMemberView) core.AvatarOwner { return core.AccountAvatar(m.UserID) }),
	},
})

var organizationInviteGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "OrganizationInvite",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(i OrganizationInviteView) any { return i.ID }),
		"email":     gqlutil.ReqStrField(func(i OrganizationInviteView) any { return i.Email }),
		"role":      gqlutil.ReqStrField(func(i OrganizationInviteView) any { return i.Role }),
		"expiresAt": gqlutil.StrField(func(i OrganizationInviteView) any { return i.ExpiresAt }),
		"createdAt": gqlutil.StrField(func(i OrganizationInviteView) any { return i.CreatedAt }),
	},
})

var organizationInvitePreviewGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "OrganizationInvitePreview",
	Fields: graphql.Fields{
		"organizationId":   gqlutil.ReqStrField(func(v OrganizationInvitePreview) any { return v.OrganizationID }),
		"organizationName": gqlutil.ReqStrField(func(v OrganizationInvitePreview) any { return v.OrganizationName }),
		"role":             gqlutil.ReqStrField(func(v OrganizationInvitePreview) any { return v.Role }),
		"inviterEmail":     gqlutil.StrField(func(v OrganizationInvitePreview) any { return v.InviterEmail }),
		"alreadyMember":    gqlutil.ReqBoolField(func(v OrganizationInvitePreview) any { return v.AlreadyMember }),
	},
})

var acceptedOrganizationInviteGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AcceptedOrganizationInvite",
	Fields: graphql.Fields{
		"organizationId":   gqlutil.ReqStrField(func(a AcceptedOrganizationInviteView) any { return a.OrganizationID }),
		"organizationName": gqlutil.StrField(func(a AcceptedOrganizationInviteView) any { return a.OrganizationName }),
		"role":             gqlutil.ReqStrField(func(a AcceptedOrganizationInviteView) any { return a.Role }),
		"joined":           gqlutil.ReqBoolField(func(a AcceptedOrganizationInviteView) any { return a.Joined }),
	},
})

var joinedWorkspaceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "JoinedWorkspace",
	Fields: graphql.Fields{
		"workspaceId": gqlutil.ReqStrField(func(j JoinedWorkspaceView) any { return j.WorkspaceID }),
		"role":        gqlutil.ReqStrField(func(j JoinedWorkspaceView) any { return j.Role }),
		"joined":      gqlutil.ReqBoolField(func(j JoinedWorkspaceView) any { return j.Joined }),
	},
})

func (s *Service) organizationGraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"organizationMembers": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(organizationMemberGQLType))),
			Args: gqlutil.KeyArg("organizationId"),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListOrganizationMembers(p.Context, p.Args["organizationId"].(string))
			},
		},
		"organizationInvites": &graphql.Field{
			Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(organizationInviteGQLType))),
			Args: gqlutil.KeyArg("organizationId"),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListOrganizationInvites(p.Context, p.Args["organizationId"].(string))
			},
		},
		"organizationInvitePreview": &graphql.Field{
			Type: organizationInvitePreviewGQLType,
			Args: graphql.FieldConfigArgument{"token": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.PreviewOrganizationInvite(p.Context, p.Args["token"].(string))
			},
		},
	}
}

func (s *Service) organizationGraphQLMutation() graphql.Fields {
	ok := func(err error) (any, error) {
		if err != nil {
			return false, err
		}
		return true, nil
	}
	return graphql.Fields{
		"inviteOrganizationMember": &graphql.Field{
			Type: organizationInviteGQLType,
			Args: graphql.FieldConfigArgument{
				"organizationId": gqlutil.ReqArg(graphql.String),
				"email":          gqlutil.ReqArg(graphql.String),
				"role":           gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.InviteToOrganization(p.Context, p.Args["organizationId"].(string), p.Args["email"].(string), p.Args["role"].(string))
			},
		},
		"revokeOrganizationInvite": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"organizationId": gqlutil.ReqArg(graphql.String),
				"inviteId":       gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return ok(s.RevokeOrganizationInvite(p.Context, p.Args["organizationId"].(string), p.Args["inviteId"].(string)))
			},
		},
		"acceptOrganizationInvite": &graphql.Field{
			Type: acceptedOrganizationInviteGQLType,
			Args: graphql.FieldConfigArgument{"token": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AcceptOrganizationInvite(p.Context, p.Args["token"].(string))
			},
		},
		"changeOrganizationMemberRole": &graphql.Field{
			Type: organizationMemberGQLType,
			Args: graphql.FieldConfigArgument{
				"organizationId": gqlutil.ReqArg(graphql.String),
				"subject":        gqlutil.ReqArg(graphql.String),
				"role":           gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ChangeOrganizationRole(p.Context, p.Args["organizationId"].(string), p.Args["subject"].(string), p.Args["role"].(string))
			},
		},
		"removeOrganizationMember": &graphql.Field{
			Type: graphql.Boolean,
			Args: graphql.FieldConfigArgument{
				"organizationId": gqlutil.ReqArg(graphql.String),
				"subject":        gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return ok(s.RemoveFromOrganization(p.Context, p.Args["organizationId"].(string), p.Args["subject"].(string)))
			},
		},
		"joinWorkspace": &graphql.Field{
			Type: joinedWorkspaceGQLType,
			Args: graphql.FieldConfigArgument{"workspaceId": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.JoinWorkspace(p.Context, p.Args["workspaceId"].(string))
			},
		},
		"joinWorkspaceAsAdmin": &graphql.Field{
			Type: joinedWorkspaceGQLType,
			Args: graphql.FieldConfigArgument{"workspaceId": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.JoinWorkspaceAsAdmin(p.Context, p.Args["workspaceId"].(string))
			},
		},
	}
}

type orgArgs struct {
	OrganizationID string `json:"organizationId" jsonschema:"the organization id, org-…"`
}

type orgMembersResult struct {
	Members []OrganizationMemberView `json:"members"`
}

type orgInvitesResult struct {
	Invites []OrganizationInviteView `json:"invites"`
}

type orgInviteArgs struct {
	OrganizationID string `json:"organizationId" jsonschema:"the organization id, org-…"`
	Email          string `json:"email" jsonschema:"the invitee's email address"`
	Role           string `json:"role" jsonschema:"the org role: OWNER or MEMBER"`
}

type orgInviteRefArgs struct {
	OrganizationID string `json:"organizationId" jsonschema:"the organization id, org-…"`
	InviteID       string `json:"inviteId" jsonschema:"the pending invite id, inv-…"`
}

type orgRoleArgs struct {
	OrganizationID string `json:"organizationId" jsonschema:"the organization id, org-…"`
	Subject        string `json:"subject" jsonschema:"the person's subject (identity id)"`
	Role           string `json:"role" jsonschema:"the org role: OWNER, MEMBER or GUEST"`
}

type orgMemberArgs struct {
	OrganizationID string `json:"organizationId" jsonschema:"the organization id, org-…"`
	Subject        string `json:"subject" jsonschema:"the person's subject (identity id)"`
}

type joinArgs struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"the organization workspace's id, tea-…"`
}

func (s *Service) registerOrganizationMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_organization_members",
		Description: "List an organization's people with their org role (OWNER, MEMBER or GUEST) and the org workspaces each belongs to. Owners and Members only. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orgArgs) (*mcp.CallToolResult, orgMembersResult, error) {
		ms, err := s.ListOrganizationMembers(ctx, in.OrganizationID)
		return nil, orgMembersResult{Members: ms}, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_organization_invites",
		Description: "List an organization's pending invites (Owner only). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orgArgs) (*mcp.CallToolResult, orgInvitesResult, error) {
		invs, err := s.ListOrganizationInvites(ctx, in.OrganizationID)
		return nil, orgInvitesResult{Invites: invs}, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "invite_organization_member",
		Description: "Invite someone to an organization by email as OWNER or MEMBER (Owner only); the emailed link joins whoever opens it, and signing in with that email joins too. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orgInviteArgs) (*mcp.CallToolResult, OrganizationInviteView, error) {
		inv, err := s.InviteToOrganization(ctx, in.OrganizationID, in.Email, in.Role)
		return nil, inv, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "preview_organization_invite",
		Description: "Review an organization invitation's organization, role, inviter and your membership without accepting it. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in acceptInviteArgs) (*mcp.CallToolResult, OrganizationInvitePreview, error) {
		preview, err := s.PreviewOrganizationInvite(ctx, in.Token)
		return nil, preview, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "accept_organization_invite",
		Description: "Redeem an organization invite token for the authenticated caller: joins the organization at the invited role even when the caller signed up under a different email. bex extension over Render's MCP.",
		Annotations: mcputil.NonDestructiveAnnotations(false, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in acceptInviteArgs) (*mcp.CallToolResult, AcceptedOrganizationInviteView, error) {
		acc, err := s.AcceptOrganizationInvite(ctx, in.Token)
		return nil, acc, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "revoke_organization_invite",
		Description: "Revoke a pending organization invite (Owner only). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orgInviteRefArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.RevokeOrganizationInvite(ctx, in.OrganizationID, in.InviteID)
		return nil, okResult{OK: err == nil}, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "change_organization_member_role",
		Description: "Change a person's org role to OWNER, MEMBER or GUEST (Owner only). The last Owner cannot be demoted, and a Guest belongs to at most one org workspace. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orgRoleArgs) (*mcp.CallToolResult, OrganizationMemberView, error) {
		m, err := s.ChangeOrganizationRole(ctx, in.OrganizationID, in.Subject, in.Role)
		return nil, m, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "remove_organization_member",
		Description: "Remove a person from an organization and from every one of its workspaces (Owner only). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in orgMemberArgs) (*mcp.CallToolResult, okResult, error) {
		err := s.RemoveFromOrganization(ctx, in.OrganizationID, in.Subject)
		return nil, okResult{OK: err == nil}, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "join_workspace",
		Description: "Join a Public workspace of your organization as a Developer. bex extension over Render's MCP.",
		Annotations: mcputil.NonDestructiveAnnotations(true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in joinArgs) (*mcp.CallToolResult, JoinedWorkspaceView, error) {
		j, err := s.JoinWorkspace(ctx, in.WorkspaceID)
		return nil, j, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "join_workspace_as_admin",
		Description: "As an Owner of its organization, join any of the org's workspaces as admin. bex extension over Render's MCP.",
		Annotations: mcputil.NonDestructiveAnnotations(true, false),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in joinArgs) (*mcp.CallToolResult, JoinedWorkspaceView, error) {
		j, err := s.JoinWorkspaceAsAdmin(ctx, in.WorkspaceID)
		return nil, j, err
	})
}
