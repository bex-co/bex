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
// get_postgres / create_postgres, keyed on Render's `postgresId`, plus Render's
// query_render_postgres (run a read-only SQL query) — MCP-only, exactly like
// Render, which exposes no REST/GraphQL equivalent (see query.go for the rails).
// Every read/create tool delegates to the same Service method REST and GraphQL
// call, so those three surfaces can't drift.

// postgresArgs is the shared single-instance argument. Render's tools key on
// `postgresId`; for bex that id is the Database name (opaque, round-tripped from
// list_postgres_instances).
type postgresArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the postgres id (bex Database name), as returned by list_postgres_instances"`
}

// createPostgresArgs mirrors the create body the REST/GraphQL surfaces accept
// (bex's Render subset). name is required; the rest default.
type createPostgresArgs struct {
	Name                   string `json:"name" jsonschema:"the database name"`
	Plan                   string `json:"plan,omitempty" jsonschema:"the instance plan, e.g. free, basic-256mb, basic-1gb"`
	Version                string `json:"version,omitempty" jsonschema:"the PostgreSQL major version, e.g. 16 (omit for the default)"`
	DiskSizeGB             int32  `json:"diskSizeGB,omitempty" jsonschema:"disk size in GB (omit for the plan default)"`
	Public                 bool   `json:"public,omitempty" jsonschema:"expose an external TLS endpoint"`
	EnableHighAvailability bool   `json:"enableHighAvailability,omitempty" jsonschema:"provision a replicated cluster (primary + standby) for high availability"`
}

// listPostgresResult wraps the array — MCP tool outputs must be JSON objects.
type listPostgresResult struct {
	Postgres []PostgresView `json:"postgres"`
}

// listPostgresArgs is list_postgres_instances' input — the ownerId scoping
// filter (w6/m2/t004), mirroring the REST/GraphQL surfaces. Empty => unscoped.
type listPostgresArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"restrict the list to this workspace id (tea-…); omit to use the session's selected workspace, if any"`
}

// queryPostgresArgs mirrors Render's query_render_postgres arguments verbatim
// (postgresId, sql) — a Render-trained agent calls the tool literally, so the
// names can't drift.
type queryPostgresArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the postgres id (bex Database name), as returned by list_postgres_instances"`
	SQL        string `json:"sql" jsonschema:"the read-only SQL query to run (SELECT/SHOW/EXPLAIN); writes, DDL and multi-statement input are rejected"`
}

// RegisterMCP adds the managed-Postgres tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_instances",
		Description: "List all managed Postgres databases in the workspace with their status. Scoped to ownerId if given, else to the session's selected workspace (select_workspace), else unscoped.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listPostgresArgs) (*mcp.CallToolResult, listPostgresResult, error) {
		list, err := s.ListPostgres(ctx, s.resolveOwnerID(req, in.OwnerID))
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
		Description: "Create a managed Postgres database. name is required; plan, version, diskSizeGB, public and enableHighAvailability are optional.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createPostgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.CreatePostgres(ctx, CreatePostgresRequest{
			Name:                   in.Name,
			Plan:                   in.Plan,
			Version:                in.Version,
			DiskSizeGB:             in.DiskSizeGB,
			Public:                 in.Public,
			EnableHighAvailability: in.EnableHighAvailability,
		})
		if err != nil {
			return nil, PostgresView{}, err
		}
		return nil, v, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "query_render_postgres",
		Description: "Run a read-only SQL query against a managed Postgres database and return the resulting columns and rows. The statement runs inside a read-only transaction with a server-side timeout; writes, DDL and long-running queries are rejected, and large result sets are truncated.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in queryPostgresArgs) (*mcp.CallToolResult, QueryResult, error) {
		res, err := s.Query(ctx, in.PostgresID, in.SQL)
		if err != nil {
			return nil, QueryResult{}, err
		}
		return nil, res, nil
	})

	s.registerLifecycleMCP(srv)
	s.registerRecoveryMCP(srv)
	s.registerAccessMCP(srv)
}

// registerLifecycleMCP adds suspend/resume/restart/failover — bex extensions
// over Render's MCP (which has no Postgres lifecycle tools), named like the
// service lifecycle tools. failover_postgres mirrors Render's REST failover.
func (s *Service) registerLifecycleMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "failover_postgres",
		Description: "Trigger a planned failover (CNPG switchover) on an HA-enabled managed Postgres database. Promotes a standby to primary. Mirrors Render's POST /postgres/{id}/failover.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, struct{ Accepted bool `json:"accepted"` }, error) {
		err := s.Failover(ctx, in.PostgresID)
		return nil, struct{ Accepted bool `json:"accepted"` }{Accepted: err == nil}, err
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "suspend_postgres",
		Description: "Suspend a managed Postgres database (hibernate: stop compute, keep the data volume). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.Suspend(ctx, in.PostgresID)
		return nil, v, err
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "resume_postgres",
		Description: "Resume a suspended managed Postgres database (un-hibernate). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.Resume(ctx, in.PostgresID)
		return nil, v, err
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "restart_postgres",
		Description: "Restart a managed Postgres database (rolling restart of the primary). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.Restart(ctx, in.PostgresID)
		return nil, v, err
	})
}

// recoverPostgresArgs is recover_postgres' input: the source id, the new
// instance name, and an optional PITR target time.
type recoverPostgresArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the source postgres id (bex Database name) to recover from"`
	Name       string `json:"name" jsonschema:"the name of the NEW database to restore into (must differ from the source)"`
	TargetTime string `json:"targetTime,omitempty" jsonschema:"an RFC3339 point in time to recover to; omit to restore the latest available point"`
	Plan       string `json:"plan,omitempty" jsonschema:"the new instance's plan (omit to match the source)"`
	Version    string `json:"version,omitempty" jsonschema:"the new instance's PostgreSQL version (omit to match the source)"`
}

// exportsResult wraps the export list (MCP outputs must be JSON objects).
type exportsResult struct {
	Exports []BackupView `json:"exports"`
}

// registerRecoveryMCP adds recovery info, recover-to-new, and exports.
func (s *Service) registerRecoveryMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_postgres_recovery_info",
		Description: "Get the point-in-time recovery window (earliest/latest restorable time) and backup list for a managed Postgres database. Returns enabled=false for plans without backups.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, RecoveryInfoView, error) {
		v, err := s.RecoveryInfo(ctx, in.PostgresID)
		return nil, v, err
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "recover_postgres",
		Description: "Recover a managed Postgres database to a NEW instance restored to a point in time (the source is never modified).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in recoverPostgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.Recover(ctx, in.PostgresID, RecoverRequest{Name: in.Name, TargetTime: in.TargetTime, Plan: in.Plan, Version: in.Version})
		return nil, v, err
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_exports",
		Description: "List the on-demand export snapshots taken for a managed Postgres database.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, exportsResult, error) {
		list, err := s.ListExports(ctx, in.PostgresID)
		if err != nil {
			return nil, exportsResult{}, err
		}
		return nil, exportsResult{Exports: list}, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_postgres_export",
		Description: "Trigger an on-demand export (a restorable snapshot to object storage) of a managed Postgres database. Requires a plan with backups.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, BackupView, error) {
		v, err := s.CreateExport(ctx, in.PostgresID)
		return nil, v, err
	})
}

// ipAllowListArgs / userArgs are the access-tool inputs.
type ipAllowListArgs struct {
	PostgresID string   `json:"postgresId" jsonschema:"the postgres id (bex Database name)"`
	CIDRs      []string `json:"cidrs" jsonschema:"the CIDR allowlist for the external endpoint; an empty list opens it to all source IPs"`
}

type userArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the postgres id (bex Database name)"`
	Name       string `json:"name" jsonschema:"the login role name (lowercase letters, digits and underscores)"`
}

// allowListResult / usersResult wrap arrays for MCP object outputs.
type allowListResult struct {
	CIDRs []string `json:"cidrs"`
}
type usersResult struct {
	Users []PostgresUserView `json:"users"`
}

// registerAccessMCP adds the IP-allowlist and Postgres-users tools.
func (s *Service) registerAccessMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_postgres_ip_allow_list",
		Description: "Get the CIDR allowlist gating a managed Postgres database's external endpoint (empty => open to all source IPs).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, allowListResult, error) {
		list, err := s.GetIPAllowList(ctx, in.PostgresID)
		if err != nil {
			return nil, allowListResult{}, err
		}
		return nil, allowListResult{CIDRs: list}, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "set_postgres_ip_allow_list",
		Description: "Replace the CIDR allowlist gating a managed Postgres database's external endpoint. Each entry must be a valid CIDR; an empty list opens the endpoint to all source IPs.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ipAllowListArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.SetIPAllowList(ctx, in.PostgresID, in.CIDRs)
		return nil, v, err
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_users",
		Description: "List the additional managed login roles on a managed Postgres database (not the owner role).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, usersResult, error) {
		users, err := s.ListUsers(ctx, in.PostgresID)
		if err != nil {
			return nil, usersResult{}, err
		}
		return nil, usersResult{Users: users}, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_postgres_user",
		Description: "Create an additional managed login role on a managed Postgres database. Returns the generated password once.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in userArgs) (*mcp.CallToolResult, CreateUserResult, error) {
		v, err := s.CreateUser(ctx, in.PostgresID, in.Name)
		return nil, v, err
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "delete_postgres_user",
		Description: "Delete an additional managed login role from a managed Postgres database.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in userArgs) (*mcp.CallToolResult, deleteResult, error) {
		err := s.DeleteUser(ctx, in.PostgresID, in.Name)
		return nil, deleteResult{Deleted: err == nil}, err
	})
}

// deleteResult is the boolean-shaped output for delete_postgres_user.
type deleteResult struct {
	Deleted bool `json:"deleted"`
}

// resolveOwnerID is list_postgres_instances' ownerId-scoping precedence: an
// explicit argument wins; otherwise fall back to the calling MCP session's
// selected workspace (select_workspace, w6/m2/t005); with neither, ""
// (unscoped) — mirrors apps.Service.resolveOwnerID.
func (s *Service) resolveOwnerID(req *mcp.CallToolRequest, arg string) string {
	if arg != "" {
		return arg
	}
	if s.Selections == nil || req == nil || req.Session == nil {
		return ""
	}
	id, _ := s.Selections.Get(req.Session.ID())
	return id
}
