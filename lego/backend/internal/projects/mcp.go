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
	OwnerID string `json:"ownerId,omitempty" jsonschema:"the workspace id (tea-…); omit to use the session's selected workspace, if any"`
}

type projectIDArgs struct {
	ID string `json:"id" jsonschema:"the project id (prj-…)"`
}

type createProjectArgs struct {
	Name    string `json:"name" jsonschema:"the project name (unique within the workspace)"`
	OwnerID string `json:"ownerId,omitempty" jsonschema:"the workspace id (tea-…); omit to use the session's selected workspace, if any"`
}

type renameProjectArgs struct {
	ID   string `json:"id" jsonschema:"the project id (prj-…)"`
	Name string `json:"name" jsonschema:"the new project name"`
}

type setProjectServicesArgs struct {
	ID         string   `json:"id" jsonschema:"the project id (prj-…)"`
	ServiceIDs []string `json:"serviceIds" jsonschema:"App CR names (same as the id field on a service) to assign to the project — replaces the full list"`
}

type setProjectDatabasesArgs struct {
	ID          string   `json:"id" jsonschema:"the project id (prj-…)"`
	DatabaseIDs []string `json:"databaseIds" jsonschema:"immutable Postgres ids (normally dpg-...; the id field returned by list_postgres_instances) to assign to the project — replaces the full list"`
}

type setProjectKeyValuesArgs struct {
	ID          string   `json:"id" jsonschema:"the project id (prj-…)"`
	KeyValueIDs []string `json:"keyValueIds" jsonschema:"KeyValue CR names (same as the id field on a key-value instance) to assign to the project — replaces the full list"`
}

type projectsResult struct {
	Projects []ProjectView `json:"projects"`
}

// RegisterMCP adds the project management tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_projects",
		Description: "List all projects in a workspace. bex extension.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listProjectsArgs) (*mcp.CallToolResult, projectsResult, error) {
		ownerID := core.SelectedWorkspace(s.Selections, req, in.OwnerID)
		ps, err := s.List(ctx, ownerID)
		return nil, projectsResult{Projects: ps}, err
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
	}, func(ctx context.Context, req *mcp.CallToolRequest, in createProjectArgs) (*mcp.CallToolResult, ProjectView, error) {
		ownerID := core.SelectedWorkspace(s.Selections, req, in.OwnerID)
		p, err := s.Create(ctx, ownerID, in.Name)
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_project_services",
		Description: "Assign services to a project (replaces the full list). Pass serviceIds as App CR names — the same id shown by list_services. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setProjectServicesArgs) (*mcp.CallToolResult, ProjectView, error) {
		if in.ServiceIDs == nil {
			in.ServiceIDs = []string{}
		}
		p, err := s.SetServices(ctx, in.ID, in.ServiceIDs)
		return nil, p, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_project_databases",
		Description: "Assign managed Postgres databases to a project (replaces the full list). Pass immutable databaseIds — the id shown by list_postgres_instances. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setProjectDatabasesArgs) (*mcp.CallToolResult, ProjectView, error) {
		if in.DatabaseIDs == nil {
			in.DatabaseIDs = []string{}
		}
		p, err := s.SetDatabases(ctx, in.ID, in.DatabaseIDs)
		return nil, p, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_project_keyvalues",
		Description: "Assign managed key-value instances to a project (replaces the full list). Pass keyValueIds as KeyValue CR names — the same id shown by list_key_value. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setProjectKeyValuesArgs) (*mcp.CallToolResult, ProjectView, error) {
		if in.KeyValueIDs == nil {
			in.KeyValueIDs = []string{}
		}
		p, err := s.SetKeyValues(ctx, in.ID, in.KeyValueIDs)
		return nil, p, err
	})
}
