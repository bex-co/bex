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
	"context"
	"fmt"
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
	Get(ctx context.Context, sessionID string) (string, bool, error)
}

// MCPWorkspaceSelectionStore is the shared persistence seam used by HTTP MCP
// sessions. The control-plane PGStore satisfies it structurally. Keeping the
// interface in core lets feature packages share selection without importing
// store or one another.
type MCPWorkspaceSelectionStore interface {
	GetMCPWorkspaceSelection(ctx context.Context, sessionID, subject string) (string, bool, error)
	SetMCPWorkspaceSelection(ctx context.Context, sessionID, subject, workspaceID string) error
}

// WorkspaceSelections is the per-MCP-session workspace selection store. With
// a shared backend, selections are keyed by both the session id and the
// authenticated subject and survive replica switches. Without one, the small
// process-local map preserves storeless and stdio behavior (stdio has one
// session, keyed "").
type WorkspaceSelections struct {
	shared MCPWorkspaceSelectionStore
	mu     sync.RWMutex
	byID   map[string]string
}

// NewWorkspaceSelections returns an empty selection store.
func NewWorkspaceSelections(shared MCPWorkspaceSelectionStore) *WorkspaceSelections {
	return &WorkspaceSelections{shared: shared, byID: map[string]string{}}
}

// Get returns the session's selected workspace id (ok=false if the session has
// never called select_workspace).
func (s *WorkspaceSelections) Get(ctx context.Context, sessionID string) (string, bool, error) {
	if s == nil {
		return "", false, nil
	}
	if s.shared != nil {
		subject, err := workspaceSelectionSubject(ctx)
		if err != nil {
			return "", false, err
		}
		id, ok, err := s.shared.GetMCPWorkspaceSelection(ctx, sessionID, subject)
		if err != nil {
			return "", false, fmt.Errorf("read MCP workspace selection: %w", err)
		}
		return id, ok, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	id, ok := s.byID[sessionID]
	return id, ok, nil
}

// Set records the session's selection — select_workspace's write.
func (s *WorkspaceSelections) Set(ctx context.Context, sessionID, ownerID string) error {
	if s == nil {
		return nil
	}
	if s.shared != nil {
		subject, err := workspaceSelectionSubject(ctx)
		if err != nil {
			return err
		}
		if err := s.shared.SetMCPWorkspaceSelection(ctx, sessionID, subject, ownerID); err != nil {
			return fmt.Errorf("write MCP workspace selection: %w", err)
		}
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.byID[sessionID] = ownerID
	return nil
}

func workspaceSelectionSubject(ctx context.Context) (string, error) {
	id, ok := IdentityFrom(ctx)
	if !ok || id.Subject == "" {
		return "", ErrForbidden
	}
	return id.Subject, nil
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
func SelectedWorkspace(ctx context.Context, sel WorkspaceSelectionReader, req *mcp.CallToolRequest, arg string) (string, error) {
	if arg != "" {
		return arg, nil
	}
	if sel == nil || req == nil || req.Session == nil {
		return "", nil
	}
	id, _, err := sel.Get(ctx, req.Session.ID())
	return id, err
}
