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
)

// mcp.go is the MCP fragment for managed key-value. Tool names track Render's
// official MCP server (render-oss/render-mcp-server): list_key_value_instances /
// get_key_value / create_key_value, keyed on Render's `keyValueId`. Render's MCP
// server exposes no delete/suspend/resume KV tools, so bex mirrors that exactly —
// those lifecycle verbs live on REST + GraphQL only (a deliberate match, noted in
// docs/ADR018-render-parity.md, not a gap). Every tool delegates to the same Service
// method REST and GraphQL call, so the surfaces can't drift.

// keyValueArgs is the shared single-instance argument. Render's tools key on
// `keyValueId`; for bex that id is the KeyValue name (opaque, round-tripped from
// list_key_value_instances).
type keyValueArgs struct {
	KeyValueID string `json:"keyValueId" jsonschema:"the key-value id (bex KeyValue name), as returned by list_key_value_instances"`
}

// createKeyValueArgs mirrors the create body the REST/GraphQL surfaces accept
// (bex's Render subset). name is required; the rest default.
type createKeyValueArgs struct {
	Name      string `json:"name" jsonschema:"the key-value store name"`
	Plan      string `json:"plan,omitempty" jsonschema:"the instance plan, e.g. free, starter, standard"`
	Version   string `json:"version,omitempty" jsonschema:"the major Valkey version, e.g. 8 (omit for the default)"`
	StorageGB int32  `json:"storageGB,omitempty" jsonschema:"disk size in GB (omit for the plan default)"`
	Public    bool   `json:"public,omitempty" jsonschema:"expose an external TLS endpoint"`
}

// listKeyValueResult wraps the array — MCP tool outputs must be JSON objects.
type listKeyValueResult struct {
	KeyValues []KeyValueView `json:"keyValues"`
}

// listKeyValueArgs is list_key_value_instances' input — the ownerId scoping
// filter (w6/m4/t002), mirroring the REST/GraphQL surfaces. Empty => unscoped.
type listKeyValueArgs struct {
	OwnerID string `json:"ownerId,omitempty" jsonschema:"restrict the list to this workspace id (tea-…); omit to use the session's selected workspace, if any"`
}

// RegisterMCP adds the managed key-value tools to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_key_value_instances",
		Description: "List all managed key-value (Valkey/Redis) stores in the workspace with their status. Scoped to ownerId if given, else to the session's selected workspace (select_workspace), else unscoped.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in listKeyValueArgs) (*mcp.CallToolResult, listKeyValueResult, error) {
		list, err := s.ListKeyValues(ctx, s.resolveOwnerID(req, in.OwnerID))
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
		Description: "Create a managed key-value (Valkey/Redis) store. name is required; plan, version, storageGB and public are optional.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in createKeyValueArgs) (*mcp.CallToolResult, KeyValueView, error) {
		v, err := s.CreateKeyValue(ctx, CreateKeyValueRequest{
			Name:      in.Name,
			Plan:      in.Plan,
			Version:   in.Version,
			StorageGB: in.StorageGB,
			Public:    in.Public,
		})
		if err != nil {
			return nil, KeyValueView{}, err
		}
		return nil, v, nil
	})
}

// resolveOwnerID is list_key_value_instances' ownerId-scoping precedence: an
// explicit argument wins; otherwise fall back to the calling MCP session's
// selected workspace (select_workspace, w6/m2/t005); with neither, ""
// (unscoped) — mirrors apps.Service.resolveOwnerID / postgres.Service.resolveOwnerID.
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
