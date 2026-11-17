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
	"errors"
	"slices"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// organizations_api.go is the org fragment of the workspaces GraphQL schema
// and MCP registry (w7/m464, ADR115 D8). Render's public REST API has no org
// read besides the audit log, so REST stays Render's owner shapes. GraphQL
// and MCP carry the same fields and answer the same codes.

// organizationRefGQLType is the org a Workspace belongs to.
var organizationRefGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "WorkspaceOrganization",
	Fields: graphql.Fields{
		"id":                 gqlutil.ReqStrField(func(o store.Organization) any { return o.ID }),
		"name":               gqlutil.ReqStrField(func(o store.Organization) any { return o.Name }),
		"primaryWorkspaceId": gqlutil.ReqStrField(func(o store.Organization) any { return o.PrimaryWorkspaceID }),
	},
})

var organizationWorkspaceGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "OrganizationWorkspace",
	Fields: graphql.Fields{
		"id":             gqlutil.ReqStrField(func(w OrganizationWorkspaceView) any { return w.ID }),
		"name":           gqlutil.ReqStrField(func(w OrganizationWorkspaceView) any { return w.Name }),
		"slug":           gqlutil.ReqStrField(func(w OrganizationWorkspaceView) any { return w.Slug }),
		"plan":           gqlutil.ReqStrField(func(w OrganizationWorkspaceView) any { return w.Plan }),
		"privacy":        gqlutil.ReqStrField(func(w OrganizationWorkspaceView) any { return w.Privacy }),
		"primary":        gqlutil.ReqBoolField(func(w OrganizationWorkspaceView) any { return w.Primary }),
		"viewerIsMember": gqlutil.ReqBoolField(func(w OrganizationWorkspaceView) any { return w.ViewerIsMember }),
	},
})

var organizationGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name:        "Organization",
	Description: "A Render organization (ADR115): a Scale or Enterprise primary workspace and the workspaces it holds under its plan.",
	Fields: graphql.Fields{
		"id":                 gqlutil.ReqStrField(func(o OrganizationView) any { return o.ID }),
		"name":               gqlutil.ReqStrField(func(o OrganizationView) any { return o.Name }),
		"plan":               gqlutil.ReqStrField(func(o OrganizationView) any { return o.Plan }),
		"primaryWorkspaceId": gqlutil.ReqStrField(func(o OrganizationView) any { return o.PrimaryWorkspaceID }),
		"viewerRole":         gqlutil.ReqStrField(func(o OrganizationView) any { return o.ViewerRole }),
		"createdAt":          gqlutil.ReqStrField(func(o OrganizationView) any { return o.CreatedAt }),
		"workspaces": gqlutil.Typed(graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(organizationWorkspaceGQLType))),
			func(o OrganizationView) any { return o.Workspaces }),
	},
})

// workspaceOrganization reads the org a workspace belongs to, nil for a
// standalone one. The caller already read the workspace, and every person in
// an org workspace is in its org.
func (s *Service) workspaceOrganization(ctx context.Context, w WorkspaceView) (*store.Organization, error) {
	if w.Organization != nil {
		return w.Organization, nil
	}
	orgs, ok := s.organizations()
	if !ok || w.OrganizationID == "" {
		return nil, nil
	}
	o, err := orgs.GetOrganization(ctx, w.OrganizationID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &o, nil
}

// withOrganizations attaches to each listed workspace the org the caller
// belongs to it through, read in one query (the switcher polls this list).
func (s *Service) withOrganizations(ctx context.Context, subject string, ws []WorkspaceView) ([]WorkspaceView, error) {
	orgs, ok := s.organizations()
	if !ok || !slices.ContainsFunc(ws, func(w WorkspaceView) bool { return w.OrganizationID != "" }) {
		return ws, nil
	}
	memberships, err := orgs.ListOrganizationsForSubject(ctx, subject)
	if err != nil {
		return nil, err
	}
	byID := make(map[string]*store.Organization, len(memberships))
	for i := range memberships {
		byID[memberships[i].ID] = &memberships[i].Organization
	}
	for i := range ws {
		ws[i].Organization = byID[ws[i].OrganizationID]
	}
	return ws, nil
}

// organizationGraphQLQuery is the org reads GraphQLQuery contributes.
func (s *Service) organizationGraphQLQuery() graphql.Fields {
	return graphql.Fields{
		"organizations": &graphql.Field{
			Type:    graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(organizationGQLType))),
			Resolve: func(p graphql.ResolveParams) (any, error) { return s.ListOrganizations(p.Context) },
		},
		"organization": &graphql.Field{
			Type: organizationGQLType,
			Args: graphql.FieldConfigArgument{"id": gqlutil.ReqArg(graphql.String)},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetOrganization(p.Context, p.Args["id"].(string))
			},
		},
	}
}

// organizationGraphQLMutation is the org workspace mutations GraphQLMutation
// contributes.
func (s *Service) organizationGraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"createOrganizationWorkspace": &graphql.Field{
			Type:        s.workspaceType(),
			Description: "Create a workspace in an organization (Owner only): it takes the org's plan, with no plan fee or payment step. privacy is public or invite_only (the default).",
			Args: graphql.FieldConfigArgument{
				"organizationId": gqlutil.ReqArg(graphql.String),
				"name":           gqlutil.ReqArg(graphql.String),
				"privacy":        gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateOrganizationWorkspace(p.Context, p.Args["organizationId"].(string), p.Args["name"].(string), gqlutil.Str(p.Args, "privacy"))
			},
		},
		"setWorkspacePrivacy": &graphql.Field{
			Type:        s.workspaceType(),
			Description: "Set an organization workspace's privacy, public or invite_only (workspace admins).",
			Args: graphql.FieldConfigArgument{
				"workspaceId": gqlutil.ReqArg(graphql.String),
				"privacy":     gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetWorkspacePrivacy(p.Context, p.Args["workspaceId"].(string), p.Args["privacy"].(string))
			},
		},
		"renameOrganization": &graphql.Field{
			Type: organizationGQLType,
			Args: graphql.FieldConfigArgument{
				"organizationId": gqlutil.ReqArg(graphql.String),
				"name":           gqlutil.ReqArg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.RenameOrganization(p.Context, p.Args["organizationId"].(string), p.Args["name"].(string))
			},
		},
	}
}

type organizationArgs struct {
	OrganizationID string `json:"organizationId" jsonschema:"the organization id, org-…"`
}

type listOrganizationsResult struct {
	Organizations []OrganizationView `json:"organizations"`
}

type createOrganizationWorkspaceArgs struct {
	OrganizationID string `json:"organizationId" jsonschema:"the organization id, org-…"`
	Name           string `json:"name" jsonschema:"the new workspace's name"`
	Privacy        string `json:"privacy,omitempty" jsonschema:"public or invite_only (the default)"`
}

type setWorkspacePrivacyArgs struct {
	WorkspaceID string `json:"workspaceId" jsonschema:"the organization workspace's id, tea-…"`
	Privacy     string `json:"privacy" jsonschema:"public or invite_only"`
}

// registerOrganizationMCP adds the org tools RegisterMCP registers: the same
// reads and writes GraphQL offers.
func (s *Service) registerOrganizationMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "list_organizations",
		Description: "List the organizations (Scale and Enterprise) the caller belongs to, with the caller's org role (OWNER, MEMBER or GUEST) and the org's workspaces: primary, privacy (public or invite_only) and whether the caller is a member. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listOrganizationsResult, error) {
		orgs, err := s.ListOrganizations(ctx)
		if orgs == nil {
			orgs = []OrganizationView{}
		}
		return nil, listOrganizationsResult{Organizations: orgs}, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_organization",
		Description: "Get one organization the caller belongs to: its plan, primary workspace, the caller's role and the workspaces the caller may see. bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in organizationArgs) (*mcp.CallToolResult, OrganizationView, error) {
		o, err := s.GetOrganization(ctx, in.OrganizationID)
		return nil, o, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "create_organization_workspace",
		Description: "Create a workspace in an organization (Owner only). It takes the org's plan with no plan fee; privacy is public or invite_only (the default). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createOrganizationWorkspaceArgs) (*mcp.CallToolResult, WorkspaceView, error) {
		w, err := s.CreateOrganizationWorkspace(ctx, in.OrganizationID, in.Name, in.Privacy)
		return nil, w, err
	})
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "set_workspace_privacy",
		Description: "Set an organization workspace's privacy to public (any org Member may join it) or invite_only (workspace admins only). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setWorkspacePrivacyArgs) (*mcp.CallToolResult, WorkspaceView, error) {
		w, err := s.SetWorkspacePrivacy(ctx, in.WorkspaceID, in.Privacy)
		return nil, w, err
	})
}
