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
)

// mcp.go is the environments MCP fragment, mirroring internal/projects/
// mcp.go's shape (bex extension). Agents can manage environments the same way
// they manage projects.

type listEnvironmentsArgs struct {
	ProjectID string `json:"projectId" jsonschema:"the project id (prj-…) to list environments under"`
}

type environmentIDArgs struct {
	ID string `json:"id" jsonschema:"the environment id (env-…)"`
}

type createEnvironmentArgs struct {
	Name      string `json:"name" jsonschema:"the environment name (unique within the project)"`
	ProjectID string `json:"projectId" jsonschema:"the project id (prj-…) to create the environment under"`
}

type renameEnvironmentArgs struct {
	ID   string `json:"id" jsonschema:"the environment id (env-…)"`
	Name string `json:"name" jsonschema:"the new environment name"`
}

type setEnvironmentServicesArgs struct {
	ID         string   `json:"id" jsonschema:"the environment id (env-…)"`
	ServiceIDs []string `json:"serviceIds" jsonschema:"App CR names (same as the id field on a service) to assign to the environment — replaces the full list; also joins these services to the environment's project"`
}

type setEnvironmentDatabasesArgs struct {
	ID          string   `json:"id" jsonschema:"the environment id (env-…)"`
	DatabaseIDs []string `json:"databaseIds" jsonschema:"Database CR names (same as the id field on a postgres instance) to assign to the environment — replaces the full list; also joins these databases to the environment's project"`
}

type setEnvironmentKeyValuesArgs struct {
	ID          string   `json:"id" jsonschema:"the environment id (env-…)"`
	KeyValueIDs []string `json:"keyValueIds" jsonschema:"KeyValue CR names (same as the id field on a key-value instance) to assign to the environment — replaces the full list; also joins these key-value instances to the environment's project"`
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
		return nil, environmentsResult{Environments: es}, err
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
		e, err := s.Create(ctx, in.ProjectID, in.Name)
		return nil, e, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rename_environment",
		Description: "Rename an environment. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in renameEnvironmentArgs) (*mcp.CallToolResult, EnvironmentView, error) {
		e, err := s.Rename(ctx, in.ID, in.Name)
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_environment_services",
		Description: "Assign services to an environment (replaces the full list); also joins them to the environment's project. Pass serviceIds as App CR names — the same id shown by list_services. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvironmentServicesArgs) (*mcp.CallToolResult, EnvironmentView, error) {
		if in.ServiceIDs == nil {
			in.ServiceIDs = []string{}
		}
		e, err := s.SetServices(ctx, in.ID, in.ServiceIDs)
		return nil, e, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_environment_databases",
		Description: "Assign managed Postgres databases to an environment (replaces the full list); also joins them to the environment's project. Pass databaseIds as Database CR names — the same id shown by list_postgres_instances. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvironmentDatabasesArgs) (*mcp.CallToolResult, EnvironmentView, error) {
		if in.DatabaseIDs == nil {
			in.DatabaseIDs = []string{}
		}
		e, err := s.SetDatabases(ctx, in.ID, in.DatabaseIDs)
		return nil, e, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_environment_keyvalues",
		Description: "Assign managed key-value instances to an environment (replaces the full list); also joins them to the environment's project. Pass keyValueIds as KeyValue CR names — the same id shown by list_key_value_instances. bex extension.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in setEnvironmentKeyValuesArgs) (*mcp.CallToolResult, EnvironmentView, error) {
		if in.KeyValueIDs == nil {
			in.KeyValueIDs = []string{}
		}
		e, err := s.SetKeyValues(ctx, in.ID, in.KeyValueIDs)
		return nil, e, err
	})
}
