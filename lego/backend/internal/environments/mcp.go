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

package environments

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the environments MCP fragment, mirroring internal/projects/
// mcp.go's shape (bex extension). Agents can manage environments the same way
// they manage projects.

type listEnvironmentsArgs struct {
	ProjectID string `json:"projectId" jsonschema:"the project id (prj-…) to list environments under"`
	Cursor    string `json:"cursor,omitempty" jsonschema:"cursor from a previous list_environments call to fetch the next page; omit for the first page"`
	Limit     int    `json:"limit,omitempty" jsonschema:"max environments to return (1-100, default 20); omit for the default page size"`
}

type environmentIDArgs struct {
	ID string `json:"id" jsonschema:"the environment id (env-…)"`
}

type createEnvironmentArgs struct {
	Name                    string                  `json:"name" jsonschema:"the environment name (unique within the project)"`
	ProjectID               string                  `json:"projectId" jsonschema:"the project id (prj-…) to create the environment under"`
	ProtectedStatus         string                  `json:"protectedStatus,omitempty" jsonschema:"optional 'protected' or 'unprotected' status"`
	NetworkIsolationEnabled bool                    `json:"networkIsolationEnabled,omitempty" jsonschema:"when true, isolate member service network traffic to this environment"`
	IPAllowList             []core.IPAllowListEntry `json:"ipAllowList,omitempty" jsonschema:"optional {cidrBlock,description} entries propagated to member datastores"`
}

// updateEnvironmentArgs is update_environment's input: the environment's own
// fields plus, since w1/m71, the four membership lists and the ACL that used to
// need one set_* tool each. Every field is a pointer — absent leaves that
// setting alone, present writes exactly what is given, and a present list
// REPLACES the whole membership (pass [] to empty it).
//
// It subsumes set_environment_acl outright: the ACL triple was already here,
// and the setter's full-replace contract ("pass the current value of any field
// you don't mean to change") was the trap a patch tool removes.
type updateEnvironmentArgs struct {
	ID                      string                   `json:"id" jsonschema:"the environment id (env-…)"`
	Name                    *string                  `json:"name,omitempty" jsonschema:"new environment name; omit to leave unchanged"`
	ProtectedStatus         *string                  `json:"protectedStatus,omitempty" jsonschema:"'protected' or 'unprotected' — protected blocks unguarded delete/suspend/direct-deploy-override on member services; omit to leave unchanged"`
	NetworkIsolationEnabled *bool                    `json:"networkIsolationEnabled,omitempty" jsonschema:"when true, member services' NetworkPolicy is scoped to only other services in this environment; omit to leave unchanged"`
	IPAllowList             *[]core.IPAllowListEntry `json:"ipAllowList,omitempty" jsonschema:"replaces the full CIDR allowlist propagated to member Postgres/KeyValue resources with these {cidrBlock, description} entries; omit to leave unchanged, pass [] to deny all"`
	IPAllowListCidrs        *[]string                `json:"ipAllowListCidrs,omitempty" jsonschema:"the plain-CIDR-string form of ipAllowList, for callers with no descriptions to keep; setting both to conflicting values is rejected"`
	ServiceIDs              *[]string                `json:"serviceIds,omitempty" jsonschema:"public service ids (normally srv-...; the id field returned by list_services) assigned to the environment — REPLACES the full list and also joins them to the environment's project; omit to leave membership unchanged, pass [] to clear it"`
	DatabaseIDs             *[]string                `json:"databaseIds,omitempty" jsonschema:"immutable Postgres ids (normally dpg-...; the id field returned by list_postgres_instances) assigned to the environment — REPLACES the full list and also joins them to the environment's project; omit to leave unchanged"`
	KeyValueIDs             *[]string                `json:"keyValueIds,omitempty" jsonschema:"KeyValue CR names (same as the id field on a key-value instance) assigned to the environment — REPLACES the full list and also joins them to the environment's project; omit to leave unchanged"`
	EnvGroupIDs             *[]string                `json:"envGroupIds,omitempty" jsonschema:"environment group ids (evg-...) assigned to the environment — REPLACES the full list; every group must belong to the environment's workspace; omit to leave unchanged"`
}

type environmentsResult struct {
	Environments []EnvironmentView `json:"environments"`
}

// RegisterMCP adds the environment management tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_environments",
		Description: "List all environments under a project. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listEnvironmentsArgs) (*mcp.CallToolResult, environmentsResult, error) {
		es, err := s.List(ctx, in.ProjectID)
		if err != nil {
			return nil, environmentsResult{}, err
		}
		limit := core.PageLimitOrDefault(in.Limit)
		paged := core.StablePage(es, in.Cursor, limit, in.Cursor != "" || in.Limit != 0, func(e EnvironmentView) string { return e.ID })
		return nil, environmentsResult{Environments: paged}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_environment",
		Description: "Get a single environment by id. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in environmentIDArgs) (*mcp.CallToolResult, EnvironmentView, error) {
		e, err := s.Get(ctx, in.ID)
		return nil, e, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_environment",
		Description: "Create a named environment under a project (e.g. staging/production) to group a subset of its services. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createEnvironmentArgs) (*mcp.CallToolResult, EnvironmentView, error) {
		e, err := s.CreateWithACL(ctx, CreateEnvironmentRequest{
			Name: in.Name, ProjectID: in.ProjectID, ProtectedStatus: in.ProtectedStatus,
			NetworkIsolationEnabled: in.NetworkIsolationEnabled, IPAllowList: in.IPAllowList,
		})
		return nil, e, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_environment",
		Description: "Update an environment in one call: its name, the protected-environment ACL (protectedStatus, networkIsolationEnabled, ipAllowList), and/or which services, databases, key-value instances, and env groups belong to it. Only the fields you pass change — an omitted field is left alone, and a present membership list REPLACES that whole membership (pass [] to empty it). Assigning a service, database, or key-value instance also joins it to the environment's project. This tool replaces the retired set_environment_acl / set_environment_services / set_environment_databases / set_environment_keyvalues / set_environment_env_groups (w1/m71) and rename_environment (w1/m74 — pass name here instead). bex extension (Render parity: PATCH /environments/{id}).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateEnvironmentArgs) (*mcp.CallToolResult, EnvironmentView, error) {
		e, err := s.applyEnvironmentPatch(ctx, in)
		return nil, e, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_environment",
		Description: "Delete an environment; its services stay in the project but become unassigned from the environment. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in environmentIDArgs) (*mcp.CallToolResult, struct {
		ID string `json:"id"`
	}, error) {
		err := s.Delete(ctx, in.ID)
		return nil, struct {
			ID string `json:"id"`
		}{ID: in.ID}, err
	})

}

// applyEnvironmentPatch runs update_environment's present arguments as the same
// Service verbs the retired setters called: one Update for the environment's own
// fields, then each membership list that was actually passed. Absent arguments
// produce no call at all, so a name change never touches membership and a
// membership change never rewrites the ACL.
func (s *Service) applyEnvironmentPatch(ctx context.Context, in updateEnvironmentArgs) (EnvironmentView, error) {
	allowList, err := core.ResolveAllowListPatch(in.IPAllowList, in.IPAllowListCidrs)
	if err != nil {
		return EnvironmentView{}, err
	}
	patch := EnvironmentPatch{
		Name:                    in.Name,
		ProtectedStatus:         in.ProtectedStatus,
		NetworkIsolationEnabled: in.NetworkIsolationEnabled,
		IPAllowList:             allowList,
	}

	var ops core.PatchOps[EnvironmentView]
	ops.Add(patch.Name != nil || patch.ProtectedStatus != nil || patch.NetworkIsolationEnabled != nil || patch.IPAllowList != nil,
		func() (EnvironmentView, error) { return s.Update(ctx, in.ID, patch) })
	ops.Add(in.ServiceIDs != nil, func() (EnvironmentView, error) {
		return s.SetServices(ctx, in.ID, core.IDList(in.ServiceIDs))
	})
	ops.Add(in.DatabaseIDs != nil, func() (EnvironmentView, error) {
		return s.SetDatabases(ctx, in.ID, core.IDList(in.DatabaseIDs))
	})
	ops.Add(in.KeyValueIDs != nil, func() (EnvironmentView, error) {
		return s.SetKeyValues(ctx, in.ID, core.IDList(in.KeyValueIDs))
	})
	ops.Add(in.EnvGroupIDs != nil, func() (EnvironmentView, error) {
		return s.SetEnvGroups(ctx, in.ID, core.IDList(in.EnvGroupIDs))
	})

	return ops.Run(func() (EnvironmentView, error) { return s.Get(ctx, in.ID) })
}
