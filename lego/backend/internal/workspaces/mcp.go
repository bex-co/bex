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
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcp.go is the MCP fragment cloning render-oss/render-mcp-server's official
// workspace trio under its exact names/params (pinned in
// docs/render-artifacts/owners-api.md, w6/m2/t005): list_workspaces (no args),
// select_workspace (required ownerID), get_selected_workspace (no args).
// Selection is held in core.WorkspaceSelections, keyed by the MCP session id, so
// the apps/postgres list tools can read it without importing this feature.
// render-mcp-server renders its results as marshaled text; bex's convention
// (apps/mcp.go, postgres/mcp.go) is typed structured-result objects instead —
// MCP tool outputs must be JSON objects, not bare arrays/strings.

// mcpWorkspace is the workspace shape the three tools return — the fields an
// agent needs to list, choose, and confirm a selection.
type mcpWorkspace struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Plan string `json:"plan,omitempty"`
	Role string `json:"role,omitempty"`
}

func toMCPWorkspace(w WorkspaceView) mcpWorkspace {
	return mcpWorkspace{ID: w.ID, Name: w.Name, Plan: w.Plan, Role: w.Role}
}

// listWorkspacesResult is list_workspaces' output. Note carries the
// auto-selection message render-mcp-server sends as tool-result text ("Only one
// workspace found, automatically selected it") when Selected is set.
type listWorkspacesResult struct {
	Workspaces []mcpWorkspace `json:"workspaces"`
	Selected   *mcpWorkspace  `json:"selected,omitempty"`
	Note       string         `json:"note,omitempty"`
}

// selectWorkspaceArgs is select_workspace's input — Render's param name
// (ownerID, not ownerId) verbatim from render-mcp-server/pkg/owner/tools.go.
type selectWorkspaceArgs struct {
	OwnerID string `json:"ownerID" jsonschema:"the workspace id (tea-…) to select for all subsequent tool calls, as returned by list_workspaces"`
}

// selectedWorkspaceResult is select_workspace's and get_selected_workspace's
// output. Selected is nil only from get_selected_workspace when the session
// has no selection AND the caller has no default workspace to fall back to
// (e.g. the control-plane store is unavailable).
type selectedWorkspaceResult struct {
	Selected *mcpWorkspace `json:"selected,omitempty"`
}

// RegisterMCP adds the workspace trio to the shared MCP server. A nil
// Selections (composition-root wiring bug, not a runtime condition) skips
// registration rather than panicking on every tool call.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	if s.Selections == nil {
		return
	}

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_workspaces",
		Description: "List the workspaces the caller has access to. If exactly one exists, it is automatically selected.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, listWorkspacesResult, error) {
		ws, err := s.List(ctx)
		if err != nil {
			return nil, listWorkspacesResult{}, err
		}
		out := make([]mcpWorkspace, 0, len(ws))
		for _, w := range ws {
			out = append(out, toMCPWorkspace(w))
		}
		res := listWorkspacesResult{Workspaces: out}
		if len(ws) == 1 && req.Session != nil {
			if err := s.Selections.Set(ctx, req.Session.ID(), ws[0].ID); err != nil {
				return nil, listWorkspacesResult{}, err
			}
			sel := toMCPWorkspace(ws[0])
			res.Selected = &sel
			res.Note = "Only one workspace found, automatically selected it"
		}
		return nil, res, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "select_workspace",
		Description: "Select a workspace to use for all actions in this session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, in selectWorkspaceArgs) (*mcp.CallToolResult, selectedWorkspaceResult, error) {
		// GetWorkspace both validates the id (foreign/unknown => ErrForbidden/
		// ErrNotFound, selection left untouched) and resolves an own- id to the
		// caller's default workspace, so select_workspace("own-…") is a valid way
		// to reselect "my" workspace.
		w, err := s.GetWorkspace(ctx, in.OwnerID)
		if err != nil {
			return nil, selectedWorkspaceResult{}, err
		}
		if req.Session != nil {
			if err := s.Selections.Set(ctx, req.Session.ID(), w.ID); err != nil {
				return nil, selectedWorkspaceResult{}, err
			}
		}
		sel := toMCPWorkspace(w)
		return nil, selectedWorkspaceResult{Selected: &sel}, nil
	})

	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_selected_workspace",
		Description: "Get the currently selected workspace for this session.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ struct{}) (*mcp.CallToolResult, selectedWorkspaceResult, error) {
		if req.Session != nil {
			id, ok, err := s.Selections.Get(ctx, req.Session.ID())
			if err != nil {
				return nil, selectedWorkspaceResult{}, err
			}
			if ok {
				w, err := s.GetWorkspace(ctx, id)
				if err != nil {
					return nil, selectedWorkspaceResult{}, err
				}
				sel := toMCPWorkspace(w)
				return nil, selectedWorkspaceResult{Selected: &sel}, nil
			}
		}
		// No explicit selection yet: fall back to the caller's default workspace
		// (matching bex's pre-m1 single-default-workspace behavior) rather than
		// erroring the way render-mcp-server's ErrNoWorkspace does.
		w, err := s.defaultWorkspace(ctx)
		switch {
		case errors.Is(err, core.ErrForbidden), errors.Is(err, core.ErrNotFound):
			// Nothing to fall back to (no identity, or a caller with no workspaces
			// yet): an empty selection, not a tool error.
			return nil, selectedWorkspaceResult{}, nil
		case err != nil:
			return nil, selectedWorkspaceResult{}, err
		}
		sel := toMCPWorkspace(w)
		return nil, selectedWorkspaceResult{Selected: &sel}, nil
	})

	// get_workspace_limits — bex extension (w7/m9): surfaces per-workspace
	// resource usage vs. cap for agents that want to check headroom before
	// attempting a create. Authorizes can_view on the workspace.
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_workspace_limits",
		Description: "Get the resource usage and limits for a workspace (services, Postgres, key-value). Used is the current count; limit 0 means unlimited.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in selectWorkspaceArgs) (*mcp.CallToolResult, ResourceLimitsView, error) {
		limits, err := s.ResourceLimits(ctx, in.OwnerID)
		if err != nil {
			return nil, ResourceLimitsView{}, err
		}
		return nil, limits, nil
	})
}
