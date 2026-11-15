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

package store

import (
	"context"
	"strings"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// What the workspace purge and the registry orphan sweep read about sandbox
// images (w22/017, docs/ADR023-usage-metering.md § Sandbox snapshot storage).
// `sandbox_storage_objects` cascades away with the tenant row, so a workspace
// deleted while a sandbox of it was suspended left the image in the registry
// with nothing naming it: the purge lists the refs before the cascade, and the
// sweep finds what is left by asking which workspaces still exist.

// SandboxRootfsRefs lists the registry refs (substrate sandbox ids) of every
// rootfs image the workspace's sandboxes retained: open, closed, or queued for
// the reaper. A resumed sandbox is included, because its older tags stay in the
// registry until the reaper deletes them, and so is an object that already
// waits for the reaper, whose row would otherwise vanish with the workspace.
// Snapshot objects (kind snapshot) and objects without a ref name no repository
// of this layout.
func (s *PGStore) SandboxRootfsRefs(ctx context.Context, workspaceID string) ([]string, error) {
	rows, err := s.Pool.Query(ctx, `
		SELECT DISTINCT ref FROM sandbox_storage_objects
		WHERE workspace_id = $1 AND kind = $2 AND ref <> ''`, workspaceID, StorageKindRootfs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []string
	for rows.Next() {
		var ref string
		if err := rows.Scan(&ref); err != nil {
			return nil, err
		}
		refs = append(refs, ref)
	}
	return refs, rows.Err()
}

// ExistingWorkspaceIDs reports which of the given workspace ids still name a
// workspace: one with a tenant row, whatever stage of deletion it has reached
// (a delete runs its purgers while the row stands), or one being created (an
// attempt that has not finalized, cancelled or expired). The orphan sweep
// deletes a namespace's images only for an id missing from the answer.
func (s *PGStore) ExistingWorkspaceIDs(ctx context.Context, workspaceIDs []string) (map[string]bool, error) {
	exist := map[string]bool{}
	if len(workspaceIDs) == 0 {
		return exist, nil
	}
	rows, err := s.Pool.Query(ctx, `
		SELECT id FROM tenants WHERE id = ANY($1)
		UNION
		SELECT workspace_id FROM workspace_creation_attempts
		WHERE workspace_id = ANY($1) AND state IN ($2, $3, $4)`,
		workspaceIDs, WorkspaceCreationPrepared, WorkspaceCreationSetupPending, WorkspaceCreationSetupSucceeded)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		exist[id] = true
	}
	return exist, rows.Err()
}

// WorkspaceOfSandboxNamespace is the workspace id a `<ws>-sandbox` namespace
// belongs to; ok is false for any name that is not a workspace's sandbox
// namespace, so a sweep never attributes a repository it cannot name an owner
// for.
func WorkspaceOfSandboxNamespace(namespace string) (string, bool) {
	workspaceID, found := strings.CutSuffix(namespace, SandboxNamespace(""))
	if !found {
		return "", false
	}
	if kind, ok := ids.KindOf(workspaceID); !ok || kind != ids.Workspace {
		return "", false
	}
	return workspaceID, true
}
