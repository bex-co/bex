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
	"fmt"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/types/tiers"
)

// mcp.go is the MCP fragment for managed Postgres. Tool names track Render's
// official MCP server (render-oss/render-mcp-server): list_postgres_instances /
// get_postgres / create_postgres, keyed on Render's `postgresId`, plus Render's
// query_render_postgres (run a read-only SQL query) — MCP-only, exactly like
// Render, which exposes no REST/GraphQL equivalent (see query.go for the rails).
// Every read/create tool delegates to the same Service method REST and GraphQL
// call, so those three surfaces can't drift.

// postgresArgs is the shared single-instance argument. Render's tools key on
// `postgresId`; bex round-trips the immutable dpg-... id returned by
// list_postgres_instances.
type postgresArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the immutable postgres id, as returned by list_postgres_instances"`
}

// suspendPostgresArgs keeps the protected-environment confirmation field
// scoped to suspend_postgres instead of advertising it on unrelated tools.
type suspendPostgresArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the immutable postgres id, as returned by list_postgres_instances"`
	Confirm    string `json:"confirm,omitempty" jsonschema:"exact confirmation phrase returned when a protected environment blocks suspend"`
}

// createPostgresArgs mirrors the create body the REST/GraphQL surfaces accept
// (bex's Render subset). name is required; the rest default.
type createPostgresArgs struct {
	EnvironmentID         string   `json:"environmentId,omitempty" jsonschema:"an environment id (env-...) in the target workspace; assignment also joins its project"`
	Name                  string   `json:"name" jsonschema:"the database name"`
	DatabaseName          string   `json:"databaseName,omitempty" jsonschema:"optional physical PostgreSQL database name; lowercase letters, digits, and underscores"`
	DatabaseUser          string   `json:"databaseUser,omitempty" jsonschema:"optional physical PostgreSQL owner role; lowercase letters, digits, and underscores"`
	Plan                  string   `json:"plan,omitempty" jsonschema:"the instance plan, e.g. free, basic-256mb, basic-1gb"`
	Version               string   `json:"version,omitempty" jsonschema:"the PostgreSQL major version, e.g. 16 (omit for the default)"`
	DiskSizeGB            int32    `json:"diskSizeGB,omitempty" jsonschema:"disk size in GB (omit for the plan default)"`
	EnableDiskAutoscaling bool     `json:"enableDiskAutoscaling,omitempty" jsonschema:"automatically grow storage at 90 percent full"`
	Public                bool     `json:"public,omitempty" jsonschema:"expose an external TLS endpoint"`
	IPAllowList           []string `json:"ipAllowList,omitempty" jsonschema:"CIDR allowlist for the external endpoint; empty or omitted leaves it open to all source IPs"`
	// IPAllowListEntries is the description-carrying form (w4/m24); when
	// present it wins over ipAllowList.
	IPAllowListEntries     []core.IPAllowListEntry `json:"ipAllowListEntries,omitempty" jsonschema:"allowlist entries as {cidrBlock, description} objects; use instead of ipAllowList to keep per-entry descriptions"`
	EnableHighAvailability bool                    `json:"enableHighAvailability,omitempty" jsonschema:"provision a replicated cluster (primary + standby) for high availability"`
	DryRun                 bool                    `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
}

// listPostgresResult wraps the array — MCP tool outputs must be JSON objects.
type listPostgresResult struct {
	Postgres []PostgresView `json:"postgres"`
}

// listPostgresArgs contains only feature arguments; the composition root adds
// Render's shared optional workspaceId parameter.
type listPostgresArgs struct{}

// queryPostgresArgs mirrors Render's query_render_postgres arguments verbatim
// (postgresId, sql) — a Render-trained agent calls the tool literally, so the
// names can't drift.
type queryPostgresArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the immutable postgres id, as returned by list_postgres_instances"`
	SQL        string `json:"sql" jsonschema:"the read-only SQL query to run (SELECT/SHOW/EXPLAIN); writes, DDL and multi-statement input are rejected"`
}

// updatePlanArgs is update_postgres_plan's input — the postgres id and the
// desired new plan (e.g. "basic-1gb").
type updatePlanArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the immutable postgres id, as returned by list_postgres_instances"`
	Plan       string `json:"plan" jsonschema:"the target instance plan (e.g. free, basic-256mb, basic-1gb)"`
	DryRun     bool   `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
}

type updateVersionArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the immutable postgres id, as returned by list_postgres_instances"`
	Version    string `json:"version" jsonschema:"the target PostgreSQL major version (13 through 18); it must be newer than the running version"`
}

type updateDiskAutoscalingArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the immutable postgres id, as returned by list_postgres_instances"`
	Enabled    bool   `json:"enabled" jsonschema:"whether automatic grow-only disk scaling is enabled"`
}

// renamePostgresArgs is rename_postgres's input. A rename changes only the
// mutable display name; the id, connection details, and data plane stay put.
type renamePostgresArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the immutable postgres id, as returned by list_postgres_instances"`
	Name       string `json:"name" jsonschema:"the new display name (lowercase letters, digits, and hyphens; at most 30 characters)"`
	DryRun     bool   `json:"dryRun,omitempty" jsonschema:"if true, validate and preview the rename without any writes"`
}

// RegisterMCP adds the managed-Postgres tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_instances",
		Description: "List all managed Postgres databases in a workspace with their status.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listPostgresArgs) (*mcp.CallToolResult, listPostgresResult, error) {
		list, err := s.ListPostgres(ctx, core.NamedWorkspace(ctx))
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
		Description: "Create a managed Postgres database. name is required; databaseName, databaseUser, plan, version, diskSizeGB, public, ipAllowList/ipAllowListEntries and enableHighAvailability are optional. Pass dryRun:true to preview the resolved spec without any writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createPostgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.CreatePostgres(ctx, CreatePostgresRequest{
			OwnerID:                core.NamedWorkspace(ctx),
			EnvironmentID:          in.EnvironmentID,
			Name:                   in.Name,
			DatabaseName:           in.DatabaseName,
			DatabaseUser:           in.DatabaseUser,
			Plan:                   in.Plan,
			Version:                in.Version,
			DiskSizeGB:             in.DiskSizeGB,
			EnableDiskAutoscaling:  in.EnableDiskAutoscaling,
			Public:                 in.Public,
			IPAllowList:            core.AllowListOrCIDRs(in.IPAllowListEntries, in.IPAllowList),
			EnableHighAvailability: in.EnableHighAvailability,
			DryRun:                 in.DryRun,
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

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_postgres_plan",
		Description: "Change a managed Postgres database's instance plan (e.g. free → basic-1gb). The operator reconciles the new resource requests on the next sync; this is a rolling update, not a data-loss operation. Pass dryRun:true to preview the change without any writes. Valid plans: free, basic-256mb, basic-1gb.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updatePlanArgs) (*mcp.CallToolResult, PostgresView, error) {
		var (
			v   PostgresView
			err error
		)
		if in.DryRun {
			v, err = s.PreviewSetPlan(ctx, in.PostgresID, in.Plan)
		} else {
			v, err = s.SetPlan(ctx, in.PostgresID, in.Plan)
		}
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_postgres_version",
		Description: "Upgrade a managed Postgres database to a newer supported major version. The database is offline during CNPG's pg_upgrade. Durable plans require a completed physical backup first; downgrades and unknown versions are rejected.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateVersionArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.SetVersion(ctx, in.PostgresID, in.Version)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_postgres_disk_autoscaling",
		Description: fmt.Sprintf("Enable or disable automatic grow-only storage scaling for a managed Postgres database. At 90%% full, storage grows by 50%% rounded up to 5 GB, capped at %d TB with a 12-hour cooldown.", tiers.Postgres.DiskAutoscalingCapGB()/1024),
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateDiskAutoscalingArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.UpdatePostgres(ctx, in.PostgresID, PostgresPatch{EnableDiskAutoscaling: &in.Enabled})
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rename_postgres",
		Description: "Rename a managed Postgres database without changing its immutable id, connection details, project/environment membership, or data-plane objects. Pass dryRun:true to validate and preview without writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in renamePostgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		patch := PostgresPatch{Name: &in.Name}
		if in.DryRun {
			v, err := s.PreviewUpdatePostgres(ctx, in.PostgresID, patch)
			return nil, v, err
		}
		v, err := s.UpdatePostgres(ctx, in.PostgresID, patch)
		return nil, v, err
	})

	s.registerLifecycleMCP(srv)
	s.registerRecoveryMCP(srv)
	s.registerAccessMCP(srv)
	s.registerInsightsMCP(srv)
	s.registerLogsMCP(srv)
}

// postgresLogsArgs is get_postgres_logs' input — the postgres id and the same
// filter vocabulary as Render's app list_logs.
type postgresLogsArgs struct {
	PostgresID string   `json:"postgresId" jsonschema:"the immutable postgres id, as returned by list_postgres_instances"`
	Text       string   `json:"text,omitempty" jsonschema:"case-insensitive substring to match against log lines"`
	StartTime  string   `json:"startTime,omitempty" jsonschema:"RFC3339 lower bound (inclusive)"`
	EndTime    string   `json:"endTime,omitempty" jsonschema:"RFC3339 upper bound (exclusive)"`
	Limit      int      `json:"limit,omitempty" jsonschema:"max lines to return (1–100, default 20)"`
	Direction  string   `json:"direction,omitempty" jsonschema:"backward (default, newest) or forward (oldest)"`
	Instance   []string `json:"instance,omitempty" jsonschema:"restrict to these pod names (empty = all replicas)"`
}

type postgresLogsResult struct {
	Logs []DatabaseLogEntry `json:"logs"`
}

func (s *Service) registerLogsMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_postgres_logs",
		Description: "Return recent log lines from a managed Postgres database, oldest-first and capped at limit (default 20, max 100). With BEX_LOKI_URL configured, lines survive pod restarts (standard Loki history). Without Loki, falls back to a live CNPG pod-log read: only currently running pods contribute and restarted-pod history is gone. bex extension — Render has no equivalent REST endpoint.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresLogsArgs) (*mcp.CallToolResult, postgresLogsResult, error) {
		since, end, err := core.ParseTimeWindow(in.StartTime, in.EndTime)
		if err != nil {
			return nil, postgresLogsResult{}, err
		}
		entries, err := s.QueryDatabaseLogs(ctx, in.PostgresID, DatabaseLogQuery{
			Search:    in.Text,
			Since:     since,
			End:       end,
			Limit:     int64(in.Limit),
			Direction: in.Direction,
			Instance:  in.Instance,
		})
		if err != nil {
			return nil, postgresLogsResult{}, err
		}
		return nil, postgresLogsResult{Logs: entries}, nil
	})
}

// registerLifecycleMCP adds suspend/resume/restart/failover — bex extensions
// over Render's MCP (which has no Postgres lifecycle tools), named like the
// service lifecycle tools. failover_postgres mirrors Render's REST failover.
func (s *Service) registerLifecycleMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "failover_postgres",
		Description: "Trigger a planned failover (CNPG switchover) on an HA-enabled managed Postgres database. Promotes a standby to primary. Mirrors Render's POST /postgres/{id}/failover.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, struct {
		Accepted bool `json:"accepted"`
	}, error) {
		err := s.Failover(ctx, in.PostgresID)
		return nil, struct {
			Accepted bool `json:"accepted"`
		}{Accepted: err == nil}, err
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "suspend_postgres",
		Description: "Suspend a managed Postgres database (hibernate: stop compute, keep the data volume). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in suspendPostgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		v, err := s.Suspend(core.WithConfirm(ctx, in.Confirm), in.PostgresID)
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
	PostgresID string `json:"postgresId" jsonschema:"the immutable source postgres id (dpg-...) to recover from"`
	Name       string `json:"name" jsonschema:"the name of the NEW database to restore into (must differ from the source)"`
	TargetTime string `json:"targetTime,omitempty" jsonschema:"an RFC3339 point in time to recover to; omit to restore the latest available point"`
	Plan       string `json:"plan,omitempty" jsonschema:"the new instance's plan (omit to match the source)"`
	Version    string `json:"version,omitempty" jsonschema:"the new instance's PostgreSQL version (omit to match the source)"`
}

// exportsResult wraps the export list (MCP outputs must be JSON objects).
type exportsResult struct {
	Exports []ExportView `json:"exports"`
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
		Description: "List logical pg_dump exports for a managed Postgres database. Available exports include a short-lived authenticated download URL.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, exportsResult, error) {
		list, err := s.ListExports(ctx, in.PostgresID)
		if err != nil {
			return nil, exportsResult{}, err
		}
		return nil, exportsResult{Exports: list}, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_postgres_export",
		Description: "Trigger a logical pg_dump directory-format export of a managed Postgres database. The artifact is retained for seven days; only one export may run at once.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, ExportView, error) {
		v, err := s.CreateExport(ctx, in.PostgresID)
		return nil, v, err
	})
}

// ipAllowListArgs / userArgs are the access-tool inputs.
// updatePostgresArgs is update_postgres's input: the patch-shaped fold of
// set_postgres_ip_allow_list and set_postgres_parameter_overrides (w1/m71).
// Each field is a pointer to the value the underlying PostgresPatch already
// documents as "nil = unchanged", so an omitted argument writes nothing while a
// present one replaces that whole list or map — the same semantics REST PATCH
// has, because both go through UpdatePostgres.
//
// It carries exactly the two folded settings. Plan, version, disk autoscaling,
// and name keep their own tools (update_postgres_plan, update_postgres_version,
// update_postgres_disk_autoscaling, rename_postgres).
type updatePostgresArgs struct {
	PostgresID         string                   `json:"postgresId" jsonschema:"the immutable postgres id (dpg-...)"`
	IPAllowList        *[]core.IPAllowListEntry `json:"ipAllowList,omitempty" jsonschema:"replaces the CIDR allowlist gating the external endpoint with these {cidrBlock, description} entries; pass [] to open the endpoint to all source IPs"`
	IPAllowListCidrs   *[]string                `json:"ipAllowListCidrs,omitempty" jsonschema:"the plain-CIDR-string form of ipAllowList, for callers with no descriptions to keep; setting both to conflicting values is rejected"`
	ParameterOverrides *map[string]string       `json:"parameterOverrides,omitempty" jsonschema:"replaces the postgresql.conf parameter overrides (key = parameter name, value = setting string); the operator projects them to the CNPG Cluster and rolls it if needed. Pass {} to clear every override. shared_preload_libraries cannot be overridden"`
	DryRun             bool                     `json:"dryRun,omitempty" jsonschema:"if true, validate and return the resolved preview without any writes"`
}

type userArgs struct {
	PostgresID string `json:"postgresId" jsonschema:"the immutable postgres id (dpg-...)"`
	Name       string `json:"name" jsonschema:"the login role name (lowercase letters, digits and underscores)"`
}

// allowListResult / usersResult wrap arrays for MCP object outputs.
type allowListResult struct {
	// CIDRs is kept for compatibility with agents that parsed this tool's
	// pre-m24 {cidrs} result; it is always AllowListCIDRs(Entries).
	CIDRs []string `json:"cidrs"`
	// Entries carries each entry's description alongside its CIDR (w4/m24).
	Entries []core.IPAllowListEntry `json:"entries"`
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
		return nil, allowListResult{CIDRs: core.AllowListCIDRs(list), Entries: list}, nil
	})
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_postgres",
		Description: "Update a managed Postgres database's settings in one call: the external-endpoint IP allowlist and/or the postgresql.conf parameter overrides. Pass only what you want to change — an omitted argument is left alone; a present one REPLACES that whole list or map (pass an empty one to clear it). Pass dryRun:true to validate and preview without writes. Plan, major version, disk autoscaling, and name keep their own tools: update_postgres_plan, update_postgres_version, update_postgres_disk_autoscaling, rename_postgres. This tool replaces the retired set_postgres_ip_allow_list and set_postgres_parameter_overrides (w1/m71).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updatePostgresArgs) (*mcp.CallToolResult, PostgresView, error) {
		allowList, err := core.ResolveAllowListPatch(in.IPAllowList, in.IPAllowListCidrs)
		if err != nil {
			return nil, PostgresView{}, err
		}
		patch := PostgresPatch{ParameterOverrides: in.ParameterOverrides, IPAllowList: allowList}
		if in.DryRun {
			v, err := s.PreviewUpdatePostgres(ctx, in.PostgresID, patch)
			return nil, v, err
		}
		v, err := s.UpdatePostgres(ctx, in.PostgresID, patch)
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

// --- MCP results for insights ---

type processesResult struct {
	Processes []ProcessView `json:"processes"`
}

type topQueriesResult struct {
	Queries []TopQueryView `json:"queries"`
}

type tableScansResult struct {
	TableScans []TableScanView `json:"tableScans"`
}

type parameterOverridesResult struct {
	Overrides []ParameterOverrideView `json:"overrides"`
}

// registerInsightsMCP adds the five observability tools (processes, top-queries,
// sizes, table-scans, parameter-overrides) to the shared MCP server.
func (s *Service) registerInsightsMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_processes",
		Description: "List active backend processes for a managed Postgres database (pg_stat_activity snapshot). Includes each process's pid, user, application name, state, current query, wait event, and how long it has been running.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, processesResult, error) {
		out, err := s.Processes(ctx, in.PostgresID)
		if err != nil {
			return nil, processesResult{}, err
		}
		return nil, processesResult{Processes: out}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_top_queries",
		Description: "List the top 25 queries by total execution time for a managed Postgres database (pg_stat_statements). Returns query text, call count, total/mean time in milliseconds, row count, and block hit/read stats. Returns an empty list when pg_stat_statements is not yet available.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, topQueriesResult, error) {
		out, err := s.TopQueries(ctx, in.PostgresID)
		if err != nil {
			return nil, topQueriesResult{}, err
		}
		return nil, topQueriesResult{Queries: out}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_postgres_sizes",
		Description: "Get the total database size and per-table sizes for a managed Postgres database. Returns the overall database size (bytes + human-readable) and up to 50 tables ordered by size descending.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, SizesView, error) {
		v, err := s.Sizes(ctx, in.PostgresID)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_table_scans",
		Description: "List sequential vs index scan stats per table for a managed Postgres database (pg_stat_user_tables). High sequential scan counts on large tables indicate missing indexes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, tableScansResult, error) {
		out, err := s.TableScans(ctx, in.PostgresID)
		if err != nil {
			return nil, tableScansResult{}, err
		}
		return nil, tableScansResult{TableScans: out}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_postgres_parameter_overrides",
		Description: "List non-default postgresql.conf parameters for a managed Postgres database (pg_settings where source is not 'default'). Shows name, current setting, unit, and source of each override.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in postgresArgs) (*mcp.CallToolResult, parameterOverridesResult, error) {
		out, err := s.ParameterOverrides(ctx, in.PostgresID)
		if err != nil {
			return nil, parameterOverridesResult{}, err
		}
		return nil, parameterOverridesResult{Overrides: out}, nil
	})

}
