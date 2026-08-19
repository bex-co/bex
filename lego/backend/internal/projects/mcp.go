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

package projects

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the projects MCP fragment (bex extension). Agents can group
// services into projects the same way a human does via the dashboard.

type listProjectsArgs struct {
	Cursor string `json:"cursor,omitempty" jsonschema:"resume after the id of the last project from the previous page"`
	Limit  int    `json:"limit,omitempty" jsonschema:"page size, 1-100; omit cursor and limit together to preserve the complete-list compatibility response"`
}

type projectIDArgs struct {
	ID string `json:"id" jsonschema:"the project id (prj-…)"`
}

type createProjectArgs struct {
	Name string `json:"name" jsonschema:"the project name (unique within the workspace)"`
}

type renameProjectArgs struct {
	ID   string `json:"id" jsonschema:"the project id (prj-…)"`
	Name string `json:"name" jsonschema:"the new project name"`
}

// updateProjectArgs is update_project's input: the patch-shaped fold of
// set_project_services / set_project_databases / set_project_keyvalues
// (w1/m71), mirroring update_environment's grammar so the two grouping
// resources read the same way. Every field is a pointer — absent leaves that
// setting alone, and a present membership list REPLACES that whole membership
// (pass [] to empty it). rename_project remains the dedicated rename verb, the
// same way rename_environment sits beside update_environment.
type updateProjectArgs struct {
	ID          string    `json:"id" jsonschema:"the project id (prj-…)"`
	Name        *string   `json:"name,omitempty" jsonschema:"new project name; omit to leave unchanged"`
	ServiceIDs  *[]string `json:"serviceIds,omitempty" jsonschema:"public service ids (normally srv-...; the id field returned by list_services) assigned to the project — REPLACES the full list; omit to leave membership unchanged, pass [] to clear it"`
	DatabaseIDs *[]string `json:"databaseIds,omitempty" jsonschema:"immutable Postgres ids (normally dpg-...; the id field returned by list_postgres_instances) assigned to the project — REPLACES the full list; omit to leave unchanged"`
	KeyValueIDs *[]string `json:"keyValueIds,omitempty" jsonschema:"KeyValue CR names (same as the id field on a key-value instance) assigned to the project — REPLACES the full list; omit to leave unchanged"`
}

type projectsResult struct {
	Projects []ProjectView `json:"projects"`
	Cursor   string        `json:"cursor,omitempty"`
}

// RegisterMCP adds the project management tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_projects",
		Description: "List projects in a workspace. Optional cursor/limit select stable id-ordered pages; omitting both returns the complete list for compatibility. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in listProjectsArgs) (*mcp.CallToolResult, projectsResult, error) {
		ps, err := s.List(ctx, core.NamedWorkspace(ctx))
		if err != nil {
			return nil, projectsResult{}, err
		}
		requested := in.Cursor != "" || in.Limit != 0
		limit := core.PageLimitOrDefault(in.Limit)
		ps = core.StablePage(ps, in.Cursor, limit, requested, func(project ProjectView) string { return project.ID })
		result := projectsResult{Projects: ps}
		if requested && len(ps) > 0 {
			result.Cursor = ps[len(ps)-1].ID
		}
		return nil, result, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_project",
		Description: "Get a single project by id. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in projectIDArgs) (*mcp.CallToolResult, ProjectView, error) {
		p, err := s.Get(ctx, in.ID)
		return nil, p, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_project",
		Description: "Create a named project in a workspace to group services. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createProjectArgs) (*mcp.CallToolResult, ProjectView, error) {
		p, err := s.Create(ctx, core.NamedWorkspace(ctx), in.Name)
		return nil, p, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rename_project",
		Description: "Rename a project. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in renameProjectArgs) (*mcp.CallToolResult, ProjectView, error) {
		p, err := s.Rename(ctx, in.ID, in.Name)
		return nil, p, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_project",
		Description: "Update a project in one call: its name and/or which services, databases, and key-value instances belong to it. Only the fields you pass change — an omitted field is left alone, and a present membership list REPLACES that whole membership (pass [] to empty it). This tool replaces the retired set_project_services / set_project_databases / set_project_keyvalues (w1/m71). bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateProjectArgs) (*mcp.CallToolResult, ProjectView, error) {
		p, err := s.applyProjectPatch(ctx, in)
		return nil, p, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_project",
		Description: "Delete a project; its services become unassigned. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in projectIDArgs) (*mcp.CallToolResult, struct {
		ID string `json:"id"`
	}, error) {
		err := s.Delete(ctx, in.ID)
		return nil, struct {
			ID string `json:"id"`
		}{ID: in.ID}, err
	})

}

// applyProjectPatch runs update_project's present arguments as the same Service
// verbs the retired setters called, in argument order. Absent arguments produce
// no call at all, so a rename never rewrites membership and vice versa.
func (s *Service) applyProjectPatch(ctx context.Context, in updateProjectArgs) (ProjectView, error) {
	var ops core.PatchOps[ProjectView]
	ops.Add(in.Name != nil, func() (ProjectView, error) { return s.Rename(ctx, in.ID, *in.Name) })
	ops.Add(in.ServiceIDs != nil, func() (ProjectView, error) {
		return s.SetServices(ctx, in.ID, core.IDList(in.ServiceIDs))
	})
	ops.Add(in.DatabaseIDs != nil, func() (ProjectView, error) {
		return s.SetDatabases(ctx, in.ID, core.IDList(in.DatabaseIDs))
	})
	ops.Add(in.KeyValueIDs != nil, func() (ProjectView, error) {
		return s.SetKeyValues(ctx, in.ID, core.IDList(in.KeyValueIDs))
	})

	return ops.Run(func() (ProjectView, error) { return s.Get(ctx, in.ID) })
}
