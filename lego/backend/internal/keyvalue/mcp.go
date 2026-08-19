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

package keyvalue

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the MCP fragment for managed key-value. Tool names track Render's
// official MCP server (render-oss/render-mcp-server): list_key_value /
// get_key_value / create_key_value, keyed on Render's `keyValueId`. The former
// Render's MCP server exposes no delete/suspend/resume KV tools. bex keeps
// delete absent, but exposes suspend_keyvalue as a deliberate lifecycle
// extension so agents can use the same protected-environment safety gate as
// REST, GraphQL, and the dashboard. rename_key_value is likewise a bex
// extension, the sibling of rename_postgres. Every tool delegates to the same
// Service method REST and GraphQL call, so the surfaces can't drift.

// keyValueArgs is the shared single-instance argument. Render's tools key on
// `keyValueId`; bex round-trips the immutable red-... id returned by
// list_key_value.
type keyValueArgs struct {
	KeyValueID string `json:"keyValueId" jsonschema:"the immutable key-value id (red-...), as returned by list_key_value"`
}

// suspendKeyValueArgs keeps the protected-environment confirmation field
// scoped to suspend_keyvalue instead of advertising it on get_key_value.
type suspendKeyValueArgs struct {
	KeyValueID string `json:"keyValueId" jsonschema:"the immutable key-value id (red-...), as returned by list_key_value"`
	Confirm    string `json:"confirm,omitempty" jsonschema:"exact confirmation phrase returned when a protected environment blocks suspend"`
}

// updateKeyValuePlanArgs is update_key_value_plan's input.
type updateKeyValuePlanArgs struct {
	KeyValueID string `json:"keyValueId" jsonschema:"the immutable key-value id (red-...), as returned by list_key_value"`
	Plan       string `json:"plan" jsonschema:"the target instance plan (e.g. free, starter, standard)"`
	DryRun     bool   `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
}

// createKeyValueArgs mirrors the create body the REST/GraphQL surfaces accept
// (bex's Render subset). name is required; the rest default.
type createKeyValueArgs struct {
	EnvironmentID string   `json:"environmentId,omitempty" jsonschema:"an environment id (env-...) in the target workspace; assignment also joins its project"`
	Name          string   `json:"name" jsonschema:"the key-value store name"`
	Plan          string   `json:"plan,omitempty" jsonschema:"the instance plan, e.g. free, starter, standard"`
	Version       string   `json:"version,omitempty" jsonschema:"the major Valkey version, e.g. 8 (omit for the default)"`
	StorageGB     int32    `json:"storageGB,omitempty" jsonschema:"disk size in GB (omit for the plan default)"`
	Public        bool     `json:"public,omitempty" jsonschema:"expose an external TLS endpoint"`
	IPAllowList   []string `json:"ipAllowList,omitempty" jsonschema:"CIDR allowlist for the external endpoint; empty or omitted leaves it open to all source IPs"`
	// IPAllowListEntries is the description-carrying form (w4/m24); when
	// present it wins over ipAllowList.
	IPAllowListEntries []core.IPAllowListEntry `json:"ipAllowListEntries,omitempty" jsonschema:"allowlist entries as {cidrBlock, description} objects; use instead of ipAllowList to keep per-entry descriptions"`
	MaxmemoryPolicy    string                  `json:"maxmemoryPolicy,omitempty" jsonschema:"key-eviction policy at the memory budget (omit for the default allkeys-lru), e.g. noeviction, allkeys-lru, volatile-ttl"`
	PersistenceMode    string                  `json:"persistenceMode,omitempty" jsonschema:"persistence: journal-snapshot (default), snapshot (RDB only), or off"`
	DryRun             bool                    `json:"dryRun,omitempty" jsonschema:"if true, return the resolved spec preview without any writes — zero side effects (w2/m29)"`
}

// renameKeyValueArgs is rename_key_value's input. A rename changes only the
// mutable display name; the id, connection details, and data plane stay put.
type renameKeyValueArgs struct {
	KeyValueID string `json:"keyValueId" jsonschema:"the immutable key-value id (red-...), as returned by list_key_value"`
	Name       string `json:"name" jsonschema:"the new display name (lowercase letters, digits, and hyphens; at most 30 characters)"`
	DryRun     bool   `json:"dryRun,omitempty" jsonschema:"if true, validate and preview the rename without any writes"`
}

// updateKeyValueArgs is update_key_value's input: the patch-shaped fold of
// set_key_value_maxmemory_policy and set_key_value_ip_allow_list (w1/m71).
// Each field is a pointer to the value KeyValuePatch already documents as
// "nil = unchanged", and the tool mirrors update_postgres field for field so the
// two managed-datastore grammars stay symmetric.
//
// Plan and name keep their own tools (update_key_value_plan, rename_key_value).
type updateKeyValueArgs struct {
	KeyValueID       string                   `json:"keyValueId" jsonschema:"the key-value id, as returned by list_key_value"`
	MaxmemoryPolicy  *string                  `json:"maxmemoryPolicy,omitempty" jsonschema:"the eviction policy at the memory budget, e.g. noeviction (job queue) or allkeys-lru (cache); underscore or hyphen forms both accepted"`
	IPAllowList      *[]core.IPAllowListEntry `json:"ipAllowList,omitempty" jsonschema:"replaces the CIDR allowlist gating the external endpoint with these {cidrBlock, description} entries; pass [] to clear it (open to all source IPs)"`
	IPAllowListCidrs *[]string                `json:"ipAllowListCidrs,omitempty" jsonschema:"the plain-CIDR-string form of ipAllowList, for callers with no descriptions to keep; setting both to conflicting values is rejected"`
	DryRun           bool                     `json:"dryRun,omitempty" jsonschema:"if true, validate and preview without any writes"`
}

// listKeyValueResult wraps the array — MCP tool outputs must be JSON objects.
type listKeyValueResult struct {
	KeyValues []KeyValueView `json:"keyValues"`
}

// listKeyValueArgs contains only feature arguments; the composition root adds
// Render's shared optional workspaceId parameter.
type listKeyValueArgs struct{}

// RegisterMCP adds the managed key-value tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_key_value",
		Description: "List all managed key-value (Valkey/Redis) stores in a workspace with their status.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ listKeyValueArgs) (*mcp.CallToolResult, listKeyValueResult, error) {
		list, err := s.ListKeyValues(ctx, core.NamedWorkspace(ctx))
		if err != nil {
			return nil, listKeyValueResult{}, err
		}
		return nil, listKeyValueResult{KeyValues: list}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_key_value",
		Description: "Get details about a specific managed key-value store by id.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in keyValueArgs) (*mcp.CallToolResult, KeyValueView, error) {
		v, err := s.GetKeyValue(ctx, in.KeyValueID)
		if err != nil {
			return nil, KeyValueView{}, err
		}
		return nil, v, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "create_key_value",
		Description: "Create a managed key-value (Valkey/Redis) store. name is required; plan, version, storageGB, public, ipAllowList, maxmemoryPolicy and persistenceMode are optional. Pass dryRun:true to preview the resolved spec without any writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createKeyValueArgs) (*mcp.CallToolResult, KeyValueView, error) {
		allowList := core.AllowListOrCIDRs(in.IPAllowListEntries, in.IPAllowList)
		v, err := s.CreateKeyValue(ctx, CreateKeyValueRequest{
			OwnerID:         core.NamedWorkspace(ctx),
			EnvironmentID:   in.EnvironmentID,
			Name:            in.Name,
			Plan:            in.Plan,
			Version:         in.Version,
			StorageGB:       in.StorageGB,
			Public:          in.Public,
			IPAllowList:     allowList,
			MaxmemoryPolicy: in.MaxmemoryPolicy,
			PersistenceMode: in.PersistenceMode,
			DryRun:          in.DryRun,
		})
		if err != nil {
			return nil, KeyValueView{}, err
		}
		return nil, v, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "suspend_keyvalue",
		Description: "Suspend a managed key-value store (stop compute while preserving its data volume). bex extension over Render's MCP.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in suspendKeyValueArgs) (*mcp.CallToolResult, KeyValueView, error) {
		v, err := s.Suspend(core.WithConfirm(ctx, in.Confirm), in.KeyValueID)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_key_value_plan",
		Description: "Change a managed key-value store's instance plan (e.g. free → standard). The operator reconciles the new resource requests on the next sync. Pass dryRun:true to preview the change without any writes. Valid plans: free, starter, standard.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateKeyValuePlanArgs) (*mcp.CallToolResult, KeyValueView, error) {
		var (
			v   KeyValueView
			err error
		)
		if in.DryRun {
			v, err = s.PreviewSetPlan(ctx, in.KeyValueID, in.Plan)
		} else {
			v, err = s.SetPlan(ctx, in.KeyValueID, in.Plan)
		}
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "rename_key_value",
		Description: "Rename a managed key-value store without changing its immutable id, connection details, project/environment membership, or data-plane objects. Pass dryRun:true to validate and preview without writes.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in renameKeyValueArgs) (*mcp.CallToolResult, KeyValueView, error) {
		patch := KeyValuePatch{Name: &in.Name}
		if in.DryRun {
			v, err := s.PreviewUpdateKeyValue(ctx, in.KeyValueID, patch)
			return nil, v, err
		}
		v, err := s.UpdateKeyValue(ctx, in.KeyValueID, patch)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "update_key_value",
		Description: "Update a managed key-value store's settings in one call: the eviction policy (maxmemoryPolicy) and/or the external-endpoint IP allowlist (Render's Networking control). Pass only what you want to change — an omitted argument is left alone; a present ipAllowList REPLACES the whole list (pass [] to clear it, opening the endpoint to all source IPs). Pass dryRun:true to validate and preview without writes. Plan and name keep their own tools: update_key_value_plan, rename_key_value. This tool replaces the retired set_key_value_maxmemory_policy and set_key_value_ip_allow_list (w1/m71); the REST mirror is PATCH /v1/key-value/{id} plus PUT .../ip-allow-list.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in updateKeyValueArgs) (*mcp.CallToolResult, KeyValueView, error) {
		allowList, err := core.ResolveAllowListPatch(in.IPAllowList, in.IPAllowListCidrs)
		if err != nil {
			return nil, KeyValueView{}, err
		}
		patch := KeyValuePatch{MaxmemoryPolicy: in.MaxmemoryPolicy, IPAllowList: allowList}
		if in.DryRun {
			v, err := s.PreviewUpdateKeyValue(ctx, in.KeyValueID, patch)
			return nil, v, err
		}
		v, err := s.UpdateKeyValue(ctx, in.KeyValueID, patch)
		return nil, v, err
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_key_value_logs",
		Description: "Return recent log lines from a managed Valkey/Redis key-value store, oldest-first and capped at limit (default 20, max 100). With BEX_LOKI_URL configured, lines survive pod restarts (standard Loki history). Without Loki, falls back to a live Valkey pod-log read: only currently running pods contribute and restarted-pod history is gone. bex extension — Render has no equivalent endpoint.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in kvLogsArgs) (*mcp.CallToolResult, kvLogsResult, error) {
		since, end, err := core.ParseTimeWindow(in.StartTime, in.EndTime)
		if err != nil {
			return nil, kvLogsResult{}, err
		}
		entries, err := s.QueryKeyValueLogs(ctx, in.KeyValueID, KeyValueLogQuery{
			Search:    in.Text,
			Since:     since,
			End:       end,
			Limit:     int64(in.Limit),
			Direction: in.Direction,
			Instance:  in.Instance,
		})
		if err != nil {
			return nil, kvLogsResult{}, err
		}
		return nil, kvLogsResult{Logs: entries}, nil
	})
}

// kvLogsArgs is get_key_value_logs' input.
type kvLogsArgs struct {
	KeyValueID string   `json:"keyValueId" jsonschema:"the key-value id, as returned by list_key_value"`
	Text       string   `json:"text,omitempty" jsonschema:"case-insensitive substring to match against log lines"`
	StartTime  string   `json:"startTime,omitempty" jsonschema:"RFC3339 lower bound (inclusive)"`
	EndTime    string   `json:"endTime,omitempty" jsonschema:"RFC3339 upper bound (exclusive)"`
	Limit      int      `json:"limit,omitempty" jsonschema:"max lines to return (1–100, default 20)"`
	Direction  string   `json:"direction,omitempty" jsonschema:"backward (default, newest) or forward (oldest)"`
	Instance   []string `json:"instance,omitempty" jsonschema:"restrict to these pod names (empty = all replicas)"`
}

type kvLogsResult struct {
	Logs []KeyValueLogEntry `json:"logs"`
}
