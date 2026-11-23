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

package sandbox

import (
	"context"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/gqlutil"
)

// The GraphQL and MCP surfaces of sandbox groups (w22/m311). Both call the same
// Service verbs as REST, so a refusal carries the same code on all three.

// sandboxGroupGQLType is the GraphQL projection of a Group.
var sandboxGroupGQLType = graphql.NewObject(graphql.ObjectConfig{
	Name: "SandboxGroup",
	Fields: graphql.Fields{
		"id":        gqlutil.ReqStrField(func(g Group) any { return g.ID }),
		"ownerId":   gqlutil.ReqStrField(func(g Group) any { return g.OwnerID }),
		"name":      gqlutil.ReqStrField(func(g Group) any { return g.Name }),
		"region":    gqlutil.ReqStrField(func(g Group) any { return g.Region }),
		"isDefault": gqlutil.ReqBoolField(func(g Group) any { return g.IsDefault }),
		// The project environment the group was placed in (its evm- id), null
		// for a group in none (w22/m412).
		"environmentId":    gqlutil.StrField(func(g Group) any { return g.EnvironmentID }),
		"concurrencyLimit": gqlutil.Typed(graphql.NewNonNull(graphql.Int), func(g Group) any { return g.ConcurrencyLimit }),
		"createdAt":        gqlutil.TimeField(func(g Group) any { return g.CreatedAt }),
		"updatedAt":        gqlutil.TimeField(func(g Group) any { return g.UpdatedAt }),
	},
})

func (s *Service) groupQueryFields() graphql.Fields {
	return graphql.Fields{
		"sandboxGroups": &graphql.Field{
			Type: graphql.NewList(sandboxGroupGQLType),
			Args: graphql.FieldConfigArgument{
				"ownerId": gqlutil.Arg(graphql.String),
				"cursor":  gqlutil.Arg(graphql.String),
				"limit":   gqlutil.Arg(graphql.Int),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.ListGroups(gqlutil.OwnerScoped(p), gqlutil.Str(p.Args, "cursor"), gqlutil.Int(p.Args, "limit"))
			},
		},
		"sandboxGroup": &graphql.Field{
			Type: sandboxGroupGQLType,
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.GetGroup(gqlutil.OwnerScoped(p), p.Args["id"].(string))
			},
		},
	}
}

func (s *Service) groupMutationFields() graphql.Fields {
	return graphql.Fields{
		"createSandboxGroup": &graphql.Field{
			Type: sandboxGroupGQLType,
			Args: graphql.FieldConfigArgument{
				"name":          gqlutil.ReqArg(graphql.String),
				"region":        gqlutil.Arg(graphql.String),
				"ownerId":       gqlutil.Arg(graphql.String),
				"environmentId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.CreateGroup(p.Context, CreateGroupRequest{
					OwnerID:       gqlutil.Str(p.Args, "ownerId"),
					Name:          p.Args["name"].(string),
					Region:        gqlutil.Str(p.Args, "region"),
					EnvironmentID: gqlutil.Str(p.Args, "environmentId"),
				})
			},
		},
		"setDefaultSandboxGroup": gqlutil.OwnedIDVerb(sandboxGroupGQLType, s.SetDefaultGroup),
		"deleteSandboxGroup": &graphql.Field{
			Type: graphql.NewNonNull(graphql.Boolean),
			Args: graphql.FieldConfigArgument{
				"id":      gqlutil.ReqArg(graphql.String),
				"ownerId": gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				if err := s.DeleteGroup(gqlutil.OwnerScoped(p), p.Args["id"].(string)); err != nil {
					return false, err
				}
				return true, nil
			},
		},
		// Render's SDK setGroupEnvironment (w22/m603): a null environmentId
		// takes the group out of the environment it is in.
		"setSandboxGroupEnvironment": &graphql.Field{
			Type: sandboxGroupGQLType,
			Args: graphql.FieldConfigArgument{
				"id":            gqlutil.ReqArg(graphql.String),
				"environmentId": gqlutil.Arg(graphql.String),
				"ownerId":       gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.SetGroupEnvironment(gqlutil.OwnerScoped(p), p.Args["id"].(string), gqlutil.Str(p.Args, "environmentId"))
			},
		},
		"updateSandboxGroup": &graphql.Field{
			Type: sandboxGroupGQLType,
			Args: graphql.FieldConfigArgument{
				"id":               gqlutil.ReqArg(graphql.String),
				"concurrencyLimit": gqlutil.ReqArg(graphql.Int),
				"ownerId":          gqlutil.Arg(graphql.String),
			},
			Resolve: func(p graphql.ResolveParams) (any, error) {
				return s.UpdateGroup(gqlutil.OwnerScoped(p), p.Args["id"].(string), p.Args["concurrencyLimit"].(int))
			},
		},
	}
}

type listGroupsArgs struct {
	Cursor string `json:"cursor,omitempty"`
	Limit  int    `json:"limit,omitempty"`
}

type listGroupsResult struct {
	SandboxGroups []Group `json:"sandboxGroups"`
}

type deleteResult struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

type setGroupEnvironmentArgs struct {
	ID            string `json:"id"`
	EnvironmentID string `json:"environmentId,omitempty" jsonschema:"the environment (evm-...) to put the group in; omit it or pass null to take the group out of its environment"`
}

type updateGroupArgs struct {
	ID               string `json:"id"`
	ConcurrencyLimit int    `json:"concurrencyLimit"`
}

func (s *Service) registerGroupMCP(server *mcp.Server) {
	addTool(s, server, &mcp.Tool{Name: "list_sandbox_groups", Description: "List the caller's workspace's sandbox groups, newest first, with each group's concurrencyLimit and whether it is the default. A sandbox is created in one group (spawn_sandbox sandboxGroupId, default: the workspace's default group); a group at its concurrencyLimit refuses a create or a resume with SANDBOX_GROUP_CONCURRENCY_LIMIT."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in listGroupsArgs) (*mcp.CallToolResult, listGroupsResult, error) {
			out, err := s.ListGroups(ctx, in.Cursor, in.Limit)
			return nil, listGroupsResult{SandboxGroups: out}, err
		})
	addTool(s, server, &mcp.Tool{Name: "get_sandbox_group", Description: "Get one sandbox group by id."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sandboxIDArgs) (*mcp.CallToolResult, Group, error) {
			g, err := s.GetGroup(ctx, in.ID)
			return nil, g, err
		})
	addTool(s, server, &mcp.Tool{Name: "create_sandbox_group", Description: "Create a sandbox group with a unique name. The workspace's first group becomes its default. A workspace may have a limited number of groups. A group and its sandboxes run in the platform's region, which the group reports; region may be omitted, oregon (Render's value) or that region. environmentId optionally places the group in one of the workspace's project environments (list_environments), as Render's clients do; the group returns it as environmentId, null when it is in none, and set_sandbox_group_environment moves it later. Placement is membership only: who can see or use the group does not change. A protected environment needs a workspace Admin, and another workspace's environment is not found."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in CreateGroupRequest) (*mcp.CallToolResult, Group, error) {
			g, err := s.CreateGroup(ctx, in)
			return nil, g, err
		})
	addTool(s, server, &mcp.Tool{Name: "set_default_sandbox_group", Description: "Make a sandbox group the workspace's default: a sandbox create or image import that names no group then uses it. The previous default stays as an ordinary group."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sandboxIDArgs) (*mcp.CallToolResult, Group, error) {
			g, err := s.SetDefaultGroup(ctx, in.ID)
			return nil, g, err
		})
	addTool(s, server, &mcp.Tool{Name: "delete_sandbox_group", Description: "Delete a sandbox group. This is irreversible: it terminates the group's sandboxes and deletes the images imported into it. The workspace's default group cannot be deleted (default_sandbox_group_not_deletable); make another group the default first with set_default_sandbox_group."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in sandboxIDArgs) (*mcp.CallToolResult, deleteResult, error) {
			if err := s.DeleteGroup(ctx, in.ID); err != nil {
				return nil, deleteResult{}, err
			}
			return nil, deleteResult{ID: in.ID, Deleted: true}, nil
		})
	addTool(s, server, &mcp.Tool{Name: "set_sandbox_group_environment", Description: "Put a sandbox group in one of the workspace's project environments (list_environments), move it to another one, or, with no environmentId, take it out of the environment it is in, as Render's SDK setGroupEnvironment and CLI `sandbox-groups environment set|remove` do. A group is in at most one environment. Moving a group into or out of a protected environment needs a workspace Admin (PROTECTED_ENVIRONMENT_ADMIN_REQUIRED). Placement is membership only: who can see or use the group does not change. Returns the group with its environmentId."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in setGroupEnvironmentArgs) (*mcp.CallToolResult, Group, error) {
			g, err := s.SetGroupEnvironment(ctx, in.ID, in.EnvironmentID)
			return nil, g, err
		})
	addTool(s, server, &mcp.Tool{Name: "update_sandbox_group", Description: "Set a sandbox group's concurrencyLimit, the most active sandboxes it allows (at least 1; a higher value is capped to the platform's maximum and the response returns the effective limit). Lowering it stops no running sandbox; the group just admits no new one until its active count is below the limit. Suspended sandboxes do not count."},
		func(ctx context.Context, _ *mcp.CallToolRequest, in updateGroupArgs) (*mcp.CallToolResult, Group, error) {
			g, err := s.UpdateGroup(ctx, in.ID, in.ConcurrencyLimit)
			return nil, g, err
		})
}
