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
		"subject":    &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m MemberView) any { return m.Subject })},
		"userId":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m MemberView) any { return m.UserID })},
		"email":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m MemberView) any { return m.Email })},
		"role":       &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m MemberView) any { return m.Role })},
		"createdAt":  &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(m MemberView) any { return m.CreatedAt })},
		"mfaEnabled": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(m MemberView) any { return m.MFAEnabled })},
	},
})

// seatUsageGQLType mirrors Render's owner.usage.users {used, limit}
// (docs/render-artifacts/team-members.graphql); limit 0 = unlimited.
var seatUsageGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceSeatUsage",
	Fields: graphql.Fields{
		"used":  &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(u SeatUsageView) any { return u.Used })},
		"limit": &graphql.Field{Type: graphql.Int, Resolve: gqlutil.Field(func(u SeatUsageView) any { return u.Limit })},
	},
})

var acceptedInviteGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "AcceptedWorkspaceInvite",
	Fields: graphql.Fields{
		"workspaceId":          &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AcceptedInviteView) any { return a.WorkspaceID })},
		"workspaceName":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AcceptedInviteView) any { return a.WorkspaceName })},
		"role":                 &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(a AcceptedInviteView) any { return a.Role })},
		"authorizationPending": &graphql.Field{Type: graphql.Boolean, Resolve: gqlutil.Field(func(a AcceptedInviteView) any { return a.AuthorizationPending })},
	},
})

var inviteGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceInvite",
	Fields: graphql.Fields{
		"id":        &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i InviteView) any { return i.ID })},
		"email":     &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i InviteView) any { return i.Email })},
		"role":      &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i InviteView) any { return i.Role })},
		"expiresAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i InviteView) any { return i.ExpiresAt })},
		"createdAt": &graphql.Field{Type: graphql.String, Resolve: gqlutil.Field(func(i InviteView) any { return i.CreatedAt })},
	},
})

func workspaceIDArg() graphql.FieldConfigArgument {
	return graphql.FieldConfigArgument{
		"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
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
	}
}

// GraphQLMutation contributes the invite / role-change / remove mutations.
func (s *Service) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"inviteWorkspaceMember": &graphql.Field{
			Type: inviteGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"email":       &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"role":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.Invite(p.Context, p.Args["workspaceId"].(string), p.Args["email"].(string), p.Args["role"].(string))
			},
		},
		"changeWorkspaceMemberRole": &graphql.Field{
			Type: memberGQLType,
			Args: graphql.FieldConfigArgument{
				"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"subject":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"role":        &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
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
				"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"subject":     &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
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
				"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"inviteId":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
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
				"workspaceId": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
				"inviteId":    &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ResendInvite(p.Context, p.Args["workspaceId"].(string), p.Args["inviteId"].(string))
			},
		},
		"acceptWorkspaceInvite": &graphql.Field{
			Type: acceptedInviteGQLType,
			Args: graphql.FieldConfigArgument{
				"token": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.String)},
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.AcceptInvite(p.Context, p.Args["token"].(string))
			},
		},
	}
}
