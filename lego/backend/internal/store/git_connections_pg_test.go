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
	"errors"
	"testing"
)

// seedGitTenant inserts the workspace row required by git_connections' tenant
// FK; a bare insert keeps the test self-contained.
func seedGitTenant(t *testing.T, st *PGStore, id string) {
	t.Helper()
	_, err := st.Pool.Exec(context.Background(),
		`INSERT INTO tenants (id, name) VALUES ($1, $1) ON CONFLICT (id) DO NOTHING`, id)
	if err != nil {
		t.Fatalf("seed tenant %s: %v", id, err)
	}
}

// TestPGGitConnectionsMultiPerWorkspace exercises the ADR075 N-per-workspace
// shape end to end against real Postgres: a workspace holds several connections,
// an installation belongs to at most one workspace, owner resolution is exact,
// count backs the quota, and per-installation delete is scoped.
func TestPGGitConnectionsMultiPerWorkspace(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	seedGitTenant(t, st, "tea-ws1")
	seedGitTenant(t, st, "tea-ws2")

	// Two installations under one workspace — the shape the old PK forbade.
	if _, err := st.UpsertGitConnection(ctx, GitConnection{WorkspaceID: "tea-ws1", InstallationID: 101, AccountLogin: "octo"}); err != nil {
		t.Fatalf("upsert 101: %v", err)
	}
	if _, err := st.UpsertGitConnection(ctx, GitConnection{WorkspaceID: "tea-ws1", InstallationID: 102, AccountLogin: "Personal"}); err != nil {
		t.Fatalf("upsert 102: %v", err)
	}

	list, err := st.ListGitConnections(ctx, "tea-ws1")
	if err != nil || len(list) != 2 {
		t.Fatalf("ListGitConnections = %v (err %v), want 2", list, err)
	}
	if n, _ := st.CountGitConnections(ctx, "tea-ws1"); n != 2 {
		t.Fatalf("CountGitConnections = %d, want 2", n)
	}

	// Owner resolution is exact and case-insensitive.
	got, err := st.GetGitConnectionByOwner(ctx, "tea-ws1", "personal")
	if err != nil || got.InstallationID != 102 {
		t.Fatalf("GetGitConnectionByOwner(personal) = %+v (err %v), want installation 102", got, err)
	}
	if _, err := st.GetGitConnectionByOwner(ctx, "tea-ws1", "stranger"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("unknown owner err = %v, want ErrNotFound", err)
	}

	// One-workspace-per-installation: re-binding 101 to another workspace moves it
	// (PK on installation_id), it never duplicates.
	if _, err := st.UpsertGitConnection(ctx, GitConnection{WorkspaceID: "tea-ws2", InstallationID: 101, AccountLogin: "octo"}); err != nil {
		t.Fatalf("rebind 101: %v", err)
	}
	if n, _ := st.CountGitConnections(ctx, "tea-ws1"); n != 1 {
		t.Fatalf("after rebind, ws1 count = %d, want 1", n)
	}
	owner, err := st.GitConnectionByInstallation(ctx, 101)
	if err != nil || owner.WorkspaceID != "tea-ws2" {
		t.Fatalf("GitConnectionByInstallation(101) = %+v (err %v), want ws2", owner, err)
	}

	// Per-installation delete is workspace-scoped: ws1 cannot delete ws2's 101.
	if err := st.DeleteGitConnection(ctx, "tea-ws1", 101); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace delete err = %v, want ErrNotFound", err)
	}
	if err := st.DeleteGitConnection(ctx, "tea-ws1", 102); err != nil {
		t.Fatalf("delete own connection: %v", err)
	}
	if n, _ := st.CountGitConnections(ctx, "tea-ws1"); n != 0 {
		t.Fatalf("after delete, ws1 count = %d, want 0", n)
	}

	// Cleanup so a rerun on the same DB starts clean.
	_, _ = st.Pool.Exec(ctx, `DELETE FROM git_connections WHERE workspace_id IN ('tea-ws1','tea-ws2')`)
	_, _ = st.Pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ('tea-ws1','tea-ws2')`)
}

func TestDeleteTenantCascadesGitConnections(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	const workspaceID = "tea-git-cascade"
	seedGitTenant(t, st, workspaceID)
	if _, err := st.UpsertGitConnection(ctx, GitConnection{
		WorkspaceID: workspaceID, InstallationID: 909301, AccountLogin: "cascade-test",
	}); err != nil {
		t.Fatalf("UpsertGitConnection: %v", err)
	}

	if err := st.DeleteTenant(ctx, workspaceID); err != nil {
		t.Fatalf("DeleteTenant: %v", err)
	}
	if n, err := st.CountGitConnections(ctx, workspaceID); err != nil || n != 0 {
		t.Fatalf("CountGitConnections after tenant delete = %d, %v; want 0, nil", n, err)
	}
}
