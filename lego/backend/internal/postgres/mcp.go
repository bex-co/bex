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

package postgres

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// mcp.go is the MCP fragment for managed Postgres. Tool names track Render's
// official MCP server (render-oss/render-mcp-server): list_postgres_instances /
// get_postgres / create_postgres, keyed on Render's `postgresId`. Render's MCP
// also ships query_render_postgres (run a read-only SQL query) — omitted here:
// it needs live in-cluster connectivity to the tenant's Postgres from the API
// layer, a deferred capability, so it's omitted rather than faked (the same rule
// the REST/GraphQL fragments follow for HA/PITR/pooler). Every tool delegates to
// the same Service method REST and GraphQL call, so the three surfaces can't drift.

// postgresArgs is the shared single-instance argument. Render's tools key on
// `postgresId`; for bex that id is the Database name (opaque, round-tripped from
// list_postgres_instances).
type postgresArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the postgres id (bex Database name), as returned by list_postgres_instances"`
}

// createPostgresArgs mirrors the create body the REST/GraphQL surfaces accept
// (bex's Render subset). name is required; the rest default.
type createPostgresArgs struct {
	Name       string `json:"name" jsonschema:"the database name"`
	Plan       string `json:"plan,omitempty" jsonschema:"the instance plan, e.g. free, basic-256mb, basic-1gb"`
	Version    string `json:"version,omitempty" jsonschema:"the PostgreSQL major version, e.g. 16 (omit for the default)"`
	DiskSizeGB int32  `json:"diskSizeGB,omitempty" jsonschema:"disk size in GB (omit for the plan default)"`
	Public     bool   `json:"public,omitempty" jsonschema:"expose an external TLS endpoint"`
}

// listPostgresResult wraps the array — MCP tool outputs must be JSON objects.
type listPostgresResult struct {
	Postgres []PostgresView `json:"postgres"`
}

// RegisterMCP adds the managed-Postgres tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_instances",
		Description: "List all managed Postgres databases in the workspace with their status.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listPostgresResult, error) {
		list, err := s.ListPostgres(ctx)
		if err != nil {
			return nil, listPostgresResult{}, err
		}
		return nil, listPostgresResult{Postgres: list}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_postgres",
		Description: "Get details about a specific managed Postgres database by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.GetPostgres(ctx, in.PostgresID)
		if err != nil {
			return nil, PostgresView{}, err
		}
		return nil, v, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_postgres",
		Description: "Create a managed Postgres database. name is required; plan, version, diskSizeGB and public are optional.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createPostgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.CreatePostgres(ctx, CreatePostgresRequest{
			Name:       in.Name,
			Plan:       in.Plan,
			Version:    in.Version,
			DiskSizeGB: in.DiskSizeGB,
			Public:     in.Public,
		})
		if err != nil {
			return nil, PostgresView{}, err
		}
		return nil, v, nil
	})
}
