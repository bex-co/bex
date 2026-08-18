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
	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// graphql.go is the members GraphQL fragment the dashboard's Team page consumes.
// Field names follow Render's captured team-members contract
// (docs/render-artifacts/team-members.graphql): members[].role (UPPERCASE enum),
// pendingInvites{ id, email, role, expiresAt }. bex flattens Render's
// owner.team.members nesting (bex has no polymorphic `owner`) into
// workspace-scoped queries — recorded as parity shape drift (docs/ADR018-render-parity.md).

var memberGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceMember",
	Fields: graphql.Fields{
		"subject":    gqlutil.StrField(func(m MemberView) any { return m.Subject }),
		"userId":     gqlutil.StrField(func(m MemberView) any { return m.UserID }),
		"email":      gqlutil.StrField(func(m MemberView) any { return m.Email }),
		"role":       gqlutil.StrField(func(m MemberView) any { return m.Role }),
		"createdAt":  gqlutil.StrField(func(m MemberView) any { return m.CreatedAt }),
		"mfaEnabled": gqlutil.BoolField(func(m MemberView) any { return m.MFAEnabled }),
	},
})

// seatUsageGQLType mirrors Render's owner.usage.users {used, limit}
// (docs/render-artifacts/team-members.graphql); limit 0 = unlimited.
var seatUsageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceSeatUsage",
	Fields: graphql.Fields{
		"used":  gqlutil.IntField(func(u SeatUsageView) any { return u.Used }),
		"limit": gqlutil.IntField(func(u SeatUsageView) any { return u.Limit }),
	},
})

var acceptedInviteGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AcceptedWorkspaceInvite",
	Fields: graphql.Fields{
		"workspaceId":          gqlutil.StrField(func(a AcceptedInviteView) any { return a.WorkspaceID }),
		"workspaceName":        gqlutil.StrField(func(a AcceptedInviteView) any { return a.WorkspaceName }),
		"role":                 gqlutil.StrField(func(a AcceptedInviteView) any { return a.Role }),
		"authorizationPending": gqlutil.BoolField(func(a AcceptedInviteView) any { return a.AuthorizationPending }),
	},
})

var inviteGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceInvite",
	Fields: graphql.Fields{
		"id":        gqlutil.StrField(func(i InviteView) any { return i.ID }),
		"email":     gqlutil.StrField(func(i InviteView) any { return i.Email }),
		"role":      gqlutil.StrField(func(i InviteView) any { return i.Role }),
		"expiresAt": gqlutil.StrField(func(i InviteView) any { return i.ExpiresAt }),
		"createdAt": gqlutil.StrField(func(i InviteView) any { return i.CreatedAt }),
	},
})

// viewerCapabilitiesGQLType is the caller's effective authorization in one
// workspace — what the dashboard reads to disable controls the server would
// refuse (w9/m84). role is UPPERCASE (or "" when unresolved); every can* is the
// authoritative Can-probe of the matching relation.
var viewerCapabilitiesGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "ViewerCapabilities",
	Fields: graphql.Fields{
		"role":             gqlutil.StrField(func(c Capabilities) any { return c.Role }),
		"canView":          gqlutil.ReqBoolField(func(c Capabilities) any { return c.CanView }),
		"canViewLogs":      gqlutil.ReqBoolField(func(c Capabilities) any { return c.CanViewLogs }),
		"canOperate":       gqlutil.ReqBoolField(func(c Capabilities) any { return c.CanOperate }),
		"canCreate":        gqlutil.ReqBoolField(func(c Capabilities) any { return c.CanCreate }),
		"canViewSensitive": gqlutil.ReqBoolField(func(c Capabilities) any { return c.CanViewSensitive }),
		"canManageKeys":    gqlutil.ReqBoolField(func(c Capabilities) any { return c.CanManageKeys }),
		"canManage":        gqlutil.ReqBoolField(func(c Capabilities) any { return c.CanManage }),
		"canManageBilling": gqlutil.ReqBoolField(func(c Capabilities) any { return c.CanManageBilling }),
	},
})

func workspaceIDArg() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"workspaceId": gqlutil.ReqArg(graphql.String),
	}
}

// GraphQLQuery contributes the members + pending-invites reads to the root Query.
func (s *Service) GraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"workspaceMembers": &graphql.Field{
			Type: graphql.NewList(memberGQLType),
			Args: workspaceIDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.List(p.Context, p.Args["workspaceId"].(string))
			},
		},
		"workspaceInvites": &graphql.Field{
			Type: graphql.NewList(inviteGQLType),
			Args: workspaceIDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListInvites(p.Context, p.Args["workspaceId"].(string))
			},
		},
		"workspaceSeatUsage": &graphql.Field{
			Type: seatUsageGQLType,
			Args: workspaceIDArg(),
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SeatUsage(p.Context, p.Args["workspaceId"].(string))
			},
		},
		// viewerCapabilities is the caller's own effective permissions in a
		// workspace — ownerId optional (absent => the caller's default), the
		// dashboard passes its active workspace. Distinct from workspaceMembers:
		// that lists everyone; this answers "what can *I* do here".
		"viewerCapabilities": &graphql.Field{
			Type: viewerCapabilitiesGQLType,
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Capabilities(p.Context, gqlutil.Str(p.Args, "ownerId"))
			},
		},
	}
}

// GraphQLMutation contributes the invite / role-change / remove mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"inviteWorkspaceMember": &graphql.Field{
			Type: inviteGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"email":       gqlutil.ReqArg(graphql.String),
				"role":        gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Invite(p.Context, p.Args["workspaceId"].(string), p.Args["email"].(string), p.Args["role"].(string))
			},
		},
		"changeWorkspaceMemberRole": &graphql.Field{
			Type: memberGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"subject":     gqlutil.ReqArg(graphql.String),
				"role":        gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ChangeRole(p.Context, p.Args["workspaceId"].(string), p.Args["subject"].(string), p.Args["role"].(string))
			},
		},
		"removeWorkspaceMember": &graphql.Field{
			// Echoes the removed subject on success (Render's delete mutations return
			// the affected id); a boolean would lose it.
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"subject":     gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				subject := p.Args["subject"].(string)
				if err := s.Remove(p.Context, p.Args["workspaceId"].(string), subject); err != nil {
					return nil, err
				}
				return subject, nil
			},
		},
		"revokeWorkspaceInvite": &graphql.Field{
			Type: graphql.String,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"inviteId":    gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				inviteID := p.Args["inviteId"].(string)
				if err := s.RevokeInvite(p.Context, p.Args["workspaceId"].(string), inviteID); err != nil {
					return nil, err
				}
				return inviteID, nil
			},
		},
		"resendWorkspaceInvite": &graphql.Field{
			Type: inviteGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"inviteId":    gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ResendInvite(p.Context, p.Args["workspaceId"].(string), p.Args["inviteId"].(string))
			},
		},
		"acceptWorkspaceInvite": &graphql.Field{
			Type: acceptedInviteGQLType,
			Args: graphql.FieldConfigArgument{
				"token": gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AcceptInvite(p.Context, p.Args["token"].(string))
			},
		},
	}
}
