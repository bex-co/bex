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

package api

import (
	"context"
	"net/http"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// cliWorkspaceHeader carries the launcher's active workspace (id or name).
// This is a bex extension: Render's service paths accept ids, not names.
const cliWorkspaceHeader = "X-Bex-Workspace"

// withCLIWorkspace scopes service-name paths only. Typed service ids remain
// global addresses and authorize against the resource's own workspace.
func (s *Server) withCLIWorkspace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		selected := strings.TrimSpace(r.Header.Get(cliWorkspaceHeader))
		target, _, _ := strings.Cut(strings.TrimPrefix(r.URL.Path, "/v1/services/"), "/")
		kind, _ := ids.KindOf(target)
		if selected == "" || !strings.HasPrefix(r.URL.Path, "/v1/services/") || target == "" || kind == ids.Service {
			next.ServeHTTP(w, r)
			return
		}
		workspaceID, err := s.cliWorkspaceID(r.Context(), selected)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		ctx := core.WithWorkspace(r.Context(), workspaceID)
		if err := s.scopeBase().ValidateNamedWorkspace(ctx); err != nil {
			core.WriteErr(w, err)
			return
		}
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *Server) cliWorkspaceID(ctx context.Context, selected string) (string, error) {
	if kind, _ := ids.KindOf(selected); kind == ids.Workspace {
		return selected, nil
	}
	if s.Workspaces == nil {
		return "", core.ErrWorkspacesUnavailable
	}
	visible, err := s.Workspaces.List(ctx)
	if err != nil {
		return "", err
	}
	var matched string
	for _, workspace := range visible {
		if workspace.Name != selected {
			continue
		}
		if matched != "" {
			return "", core.NewConflictError("WORKSPACE_NAME_AMBIGUOUS", "several workspaces have that name; select a workspace by id", nil)
		}
		matched = workspace.ID
	}
	if matched == "" {
		return "", core.ErrNotFound
	}
	return matched, nil
}
