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

	// One-workspace-per-installation: re-binding 101 to another workspace is a
	// conflict (finding-4); the store must not silently transfer the installation.
	if _, err := st.UpsertGitConnection(ctx, GitConnection{WorkspaceID: "tea-ws2", InstallationID: 101, AccountLogin: "octo"}); !errors.Is(err, ErrConflict) {
		t.Fatalf("cross-workspace rebind 101 err = %v, want ErrConflict", err)
	}
	if n, _ := st.CountGitConnections(ctx, "tea-ws1"); n != 2 {
		t.Fatalf("after rejected rebind, ws1 count = %d, want 2", n)
	}
	owner, err := st.GitConnectionByInstallation(ctx, 101)
	if err != nil || owner.WorkspaceID != "tea-ws1" {
		t.Fatalf("GitConnectionByInstallation(101) = %+v (err %v), want ws1", owner, err)
	}
	// Same-workspace reconnect remains idempotent and updates the login.
	if _, err := st.UpsertGitConnection(ctx, GitConnection{WorkspaceID: "tea-ws1", InstallationID: 101, AccountLogin: "octo-updated"}); err != nil {
		t.Fatalf("same-workspace re-upsert 101: %v", err)
	}
	owner, err = st.GitConnectionByInstallation(ctx, 101)
	if err != nil || owner.AccountLogin != "octo-updated" {
		t.Fatalf("after same-workspace update, account = %q (err %v), want octo-updated", owner.AccountLogin, err)
	}

	// Per-installation delete is workspace-scoped: ws2 cannot delete ws1's 101
	// (101 remained in ws1 after the rejected cross-workspace rebind).
	if err := st.DeleteGitConnection(ctx, "tea-ws2", 101); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-workspace delete err = %v, want ErrNotFound", err)
	}
	if err := st.DeleteGitConnection(ctx, "tea-ws1", 102); err != nil {
		t.Fatalf("delete own connection 102: %v", err)
	}
	if err := st.DeleteGitConnection(ctx, "tea-ws1", 101); err != nil {
		t.Fatalf("delete own connection 101: %v", err)
	}
	if n, _ := st.CountGitConnections(ctx, "tea-ws1"); n != 0 {
		t.Fatalf("after delete, ws1 count = %d, want 0", n)
	}

	// Cleanup so a rerun on the same DB starts clean.
	_, _ = st.Pool.Exec(ctx, `DELETE FROM git_connections WHERE workspace_id IN ('tea-ws1','tea-ws2')`)
	_, _ = st.Pool.Exec(ctx, `DELETE FROM tenants WHERE id IN ('tea-ws1','tea-ws2')`)
}

// Two stores model callbacks landing on different API replicas. The workspace
// advisory lock must make the count+insert decision serial even across pools.
func TestBindGitConnectionQuotaIsAtomicAcrossPools(t *testing.T) {
	storeA := newReplayTestStore(t)
	storeB := newReplayTestStore(t)
	ctx := context.Background()
	const workspaceID = "tea-git-quota-race"
	_, _ = storeA.Pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, workspaceID)
	seedGitTenant(t, storeA, workspaceID)
	t.Cleanup(func() {
		_, _ = storeA.Pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, workspaceID)
	})
	if _, err := storeA.BindGitConnection(ctx, GitConnection{
		WorkspaceID: workspaceID, InstallationID: 7001, AccountLogin: "first",
	}, 2); err != nil {
		t.Fatalf("seed connection: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for i, candidate := range []*PGStore{storeA, storeB} {
		installationID := int64(7002 + i)
		go func() {
			<-start
			_, err := candidate.BindGitConnection(ctx, GitConnection{
				WorkspaceID: workspaceID, InstallationID: installationID, AccountLogin: "candidate",
			}, 2)
			results <- err
		}()
	}
	close(start)
	successes, limits := 0, 0
	for range 2 {
		err := <-results
		if err == nil {
			successes++
			continue
		}
		var limit *GitConnectionLimitError
		if errors.As(err, &limit) {
			limits++
			continue
		}
		t.Fatalf("unexpected bind result: %v", err)
	}
	if successes != 1 || limits != 1 {
		t.Fatalf("race results: successes=%d limits=%d", successes, limits)
	}
	if count, err := storeA.CountGitConnections(ctx, workspaceID); err != nil || count != 2 {
		t.Fatalf("connection count = %d, %v; want hard limit 2", count, err)
	}

	// At the limit, refreshing an existing installation remains exempt while a
	// genuinely new binding racing it is still refused.
	reconnect := make(chan error, 1)
	newBinding := make(chan error, 1)
	start = make(chan struct{})
	go func() {
		<-start
		_, err := storeA.BindGitConnection(ctx, GitConnection{
			WorkspaceID: workspaceID, InstallationID: 7001, AccountLogin: "refreshed",
		}, 2)
		reconnect <- err
	}()
	go func() {
		<-start
		_, err := storeB.BindGitConnection(ctx, GitConnection{
			WorkspaceID: workspaceID, InstallationID: 7004, AccountLogin: "new",
		}, 2)
		newBinding <- err
	}()
	close(start)
	if err := <-reconnect; err != nil {
		t.Fatalf("same-workspace reconnect: %v", err)
	}
	var limit *GitConnectionLimitError
	if err := <-newBinding; !errors.As(err, &limit) {
		t.Fatalf("new binding at limit = %v, want GitConnectionLimitError", err)
	}
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
