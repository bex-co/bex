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

package core

import (
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// WorkspaceSelectionReader is the read-only view of WorkspaceSelections — what
// apps/postgres hold. They only ever need to read a session's selection, never
// write another feature's; keeping their field typed as this interface (not
// the concrete *WorkspaceSelections) makes Set structurally unreachable from
// them, the same segregation EnvVarReader (envvars.go) gives its reader-only
// consumer.
type WorkspaceSelectionReader interface {
	Get(sessionID string) (string, bool)
}

// WorkspaceSelections is the in-memory, per-MCP-session workspace selection
// store (w6/m2/t005): select_workspace's write side (only workspaces.Service
// holds the concrete type, so only it can call Set), get_selected_workspace's
// and every workspace-scoped list tool's (list_services, list_postgres_instances)
// read side. It lives in the kernel — not the workspaces feature — so those
// other features can read a selection without importing workspaces (features
// never import each other; docs/ADR006-bex-api.md). One process-wide instance, shared
// across every MCP session; keyed by the MCP session id ("" for stdio, which
// has exactly one session, matching render-mcp-server's own convention).
// Selection is held in memory only, never persisted — a server restart clears
// it, same as the official Render MCP server's session store.
type WorkspaceSelections struct {
	mu   sync.RWMutex
	byID map[string]string
}

// NewWorkspaceSelections returns an empty selection store.
func NewWorkspaceSelections() *WorkspaceSelections {
	return &WorkspaceSelections{byID: map[string]string{}}
}

// Get returns the session's selected workspace id (ok=false if the session has
// never called select_workspace).
func (s *WorkspaceSelections) Get(sessionID string) (string, bool) {
	if s == nil {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byID[sessionID]
	return id, ok
}

// Set records the session's selection — select_workspace's write.
func (s *WorkspaceSelections) Set(sessionID, ownerID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[sessionID] = ownerID
}

// SelectedWorkspace is the ownerId precedence EVERY workspace-scoped MCP tool
// shares — list_services, list_postgres_instances, and (w6/m14) the create
// tools: an explicit ownerId argument wins; otherwise the calling session's
// select_workspace selection; with neither, "" (the caller's default
// workspace). It lives here, beside the selection store, because three features
// had grown a private copy of it (apps/postgres/keyvalue) and a fourth was
// about to — a precedence rule that drifts per feature is exactly how one
// surface silently stops honoring a selection.
//
// A nil session (stdio has exactly one session, keyed "") or an unwired
// selection store degrades to "": the caller's default workspace, never another
// caller's selection.
func SelectedWorkspace(sel WorkspaceSelectionReader, req *mcp.CallToolRequest, arg string) string {
	if arg != "" {
		return arg
	}
	if sel == nil || req == nil || req.Session == nil {
		return ""
	}
	id, _ := sel.Get(req.Session.ID())
	return id
}
