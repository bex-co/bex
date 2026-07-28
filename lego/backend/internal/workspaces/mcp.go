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

package workspaces

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go exposes workspace discovery plus bex's resource-limit extension.
// Resource tools take Render's optional per-call workspaceId at the API
// composition root; there is deliberately no transport-session selection.

// mcpWorkspace is the workspace shape an agent needs to discover a workspaceId.
type mcpWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Plan string `json:"plan,omitempty"`
	Role string `json:"role,omitempty"`
}

func toMCPWorkspace(w WorkspaceView) mcpWorkspace {
	return mcpWorkspace{ID: w.ID, Name: w.Name, Plan: w.Plan, Role: w.Role}
}

type listWorkspacesResult struct {
	Workspaces []mcpWorkspace `json:"workspaces"`
}

// RegisterMCP adds workspace discovery and limits to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_workspaces",
		Description: "List the workspaces the caller has access to. Pass a returned id as workspaceId on each resource-tool call.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listWorkspacesResult, error) {
		ws, err := s.List(ctx)
		if err != nil {
			return nil, listWorkspacesResult{}, err
		}
		out := make([]mcpWorkspace, 0, len(ws))
		for _, w := range ws {
			out = append(out, toMCPWorkspace(w))
		}
		return nil, listWorkspacesResult{Workspaces: out}, nil
	})

	// get_workspace_limits — bex extension (w7/m9): surfaces per-workspace
	// resource usage vs. cap for agents that want to check headroom before
	// attempting a create. Authorizes can_view on the workspace.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_workspace_limits",
		Description: "Get the resource usage and limits for a workspace (services, Postgres, key-value). Used is the current count; limit 0 means unlimited.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, ResourceLimitsView, error) {
		limits, err := s.ResourceLimits(ctx, core.NamedWorkspace(ctx))
		if err != nil {
			return nil, ResourceLimitsView{}, err
		}
		return nil, limits, nil
	})
}
