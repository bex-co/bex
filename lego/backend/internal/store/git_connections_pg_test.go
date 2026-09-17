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

// TestPGGitConnectionsManyToMany exercises the ADR078 §2 N:N shape end to end
// against real Postgres: a workspace holds several connections, ONE installation
// may serve several workspaces (w2/m162 — the reversal of the old unique
// binding), the same pair is idempotent, owner resolution is exact, count backs
// the quota, the reverse lookup returns the full binding set, and
// per-installation delete stays workspace-scoped.
func TestPGGitConnectionsManyToMany(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	seedGitTenant(t, st, "tea-ws1")
	seedGitTenant(t, st, "tea-ws2")
	t.Cleanup(func() {
		_, _ = st.Pool.Exec(context.Background(), `DELETE FROM git_connections WHERE workspace_id IN ('tea-ws1','tea-ws2')`)
		_, _ = st.Pool.Exec(context.Background(), `DELETE FROM tenants WHERE id IN ('tea-ws1','tea-ws2')`)
	})

	bind := func(ws string, installation int64, login string) error {
		_, err := st.BindGitConnection(ctx, GitConnection{
			WorkspaceID: ws, InstallationID: installation, AccountLogin: login,
		}, 0, 0)
		return err
	}

	// Two installations under one workspace (the m74 shape).
	if err := bind("tea-ws1", 101, "octo"); err != nil {
		t.Fatalf("bind 101: %v", err)
	}
	if err := bind("tea-ws1", 102, "Personal"); err != nil {
		t.Fatalf("bind 102: %v", err)
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

	// THE REVERSAL (ADR078 §2): binding 101 into a SECOND workspace now succeeds.
	// Before w2/m162 this was ErrConflict, which is what made a personal GitHub
	// account unable to back more than one workspace at all.
	if err := bind("tea-ws2", 101, "octo"); err != nil {
		t.Fatalf("second workspace binding installation 101: %v, want success", err)
	}
	bindings, err := st.GitConnectionsByInstallation(ctx, 101)
	if err != nil || len(bindings) != 2 {
		t.Fatalf("GitConnectionsByInstallation(101) = %+v (err %v), want 2 bindings", bindings, err)
	}
	seen := map[string]bool{}
	for _, b := range bindings {
		seen[b.WorkspaceID] = true
	}
	if !seen["tea-ws1"] || !seen["tea-ws2"] {
		t.Fatalf("bindings = %+v, want both tea-ws1 and tea-ws2", bindings)
	}
	// ws1 gained nothing: its own set is unchanged by another workspace binding.
	if n, _ := st.CountGitConnections(ctx, "tea-ws1"); n != 2 {
		t.Fatalf("after ws2 bound 101, ws1 count = %d, want 2", n)
	}

	// The same PAIR stays idempotent — one row, refreshed login (so a GitHub
	// account rename converges) rather than a duplicate.
	if err := bind("tea-ws1", 101, "octo-updated"); err != nil {
		t.Fatalf("same-pair rebind: %v", err)
	}
	if n, _ := st.CountGitConnections(ctx, "tea-ws1"); n != 2 {
		t.Fatalf("after same-pair rebind, ws1 count = %d, want 2 (no duplicate row)", n)
	}
	refreshed, err := st.GetGitConnectionByOwner(ctx, "tea-ws1", "octo-updated")
	if err != nil || refreshed.InstallationID != 101 {
		t.Fatalf("after same-pair rebind, owner lookup = %+v (err %v), want 101", refreshed, err)
	}

	// Per-installation delete stays workspace-scoped, and removing one binding
	// leaves the other workspace's binding intact.
	if err := st.DeleteGitConnection(ctx, "tea-ws2", 102); !errors.Is(err, ErrNotFound) {
		t.Fatalf("deleting an installation ws2 never bound = %v, want ErrNotFound", err)
	}
	if err := st.DeleteGitConnection(ctx, "tea-ws2", 101); err != nil {
		t.Fatalf("ws2 disconnecting its own binding of 101: %v", err)
	}
	remaining, err := st.GitConnectionsByInstallation(ctx, 101)
	if err != nil || len(remaining) != 1 || remaining[0].WorkspaceID != "tea-ws1" {
		t.Fatalf("after ws2 disconnect, bindings = %+v (err %v), want ws1 only", remaining, err)
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
	if gone, err := st.GitConnectionsByInstallation(ctx, 101); err != nil || len(gone) != 0 {
		t.Fatalf("GitConnectionsByInstallation(101) after deletes = %+v (err %v), want empty", gone, err)
	}
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
	}, 2, 0); err != nil {
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
			}, 2, 0)
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
		}, 2, 0)
		reconnect <- err
	}()
	go func() {
		<-start
		_, err := storeB.BindGitConnection(ctx, GitConnection{
			WorkspaceID: workspaceID, InstallationID: 7004, AccountLogin: "new",
		}, 2, 0)
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

// The N:N mirror cap (ADR078 §2): how many WORKSPACES one installation may
// serve. It bounds the push webhook's fan-out (§4a), so like its sibling it must
// be admitted inside the insert's transaction — a standalone count would let two
// concurrent callbacks at limit-1 both pass.
func TestBindGitConnectionInstallationWorkspaceQuotaIsAtomicAcrossPools(t *testing.T) {
	storeA := newReplayTestStore(t)
	storeB := newReplayTestStore(t)
	ctx := context.Background()
	workspaces := []string{"tea-fanout-1", "tea-fanout-2", "tea-fanout-3"}
	const installationID = int64(7101)
	for _, ws := range workspaces {
		_, _ = storeA.Pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, ws)
		seedGitTenant(t, storeA, ws)
	}
	t.Cleanup(func() {
		for _, ws := range workspaces {
			_, _ = storeA.Pool.Exec(context.Background(), `DELETE FROM tenants WHERE id = $1`, ws)
		}
	})

	// One binding already exists; the cap is 2, so exactly one of the two racing
	// workspaces may still bind the same installation.
	if _, err := storeA.BindGitConnection(ctx, GitConnection{
		WorkspaceID: workspaces[0], InstallationID: installationID, AccountLogin: "shared",
	}, 0, 2); err != nil {
		t.Fatalf("seed binding: %v", err)
	}

	start := make(chan struct{})
	results := make(chan error, 2)
	for i, candidate := range []*PGStore{storeA, storeB} {
		ws := workspaces[i+1]
		go func() {
			<-start
			_, err := candidate.BindGitConnection(ctx, GitConnection{
				WorkspaceID: ws, InstallationID: installationID, AccountLogin: "shared",
			}, 0, 2)
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
		var limit *GitInstallationWorkspaceLimitError
		if errors.As(err, &limit) {
			limits++
			continue
		}
		t.Fatalf("unexpected bind result: %v", err)
	}
	if successes != 1 || limits != 1 {
		t.Fatalf("race results: successes=%d limits=%d", successes, limits)
	}
	bindings, err := storeA.GitConnectionsByInstallation(ctx, installationID)
	if err != nil || len(bindings) != 2 {
		t.Fatalf("bindings = %+v (err %v); want hard limit 2", bindings, err)
	}

	// A workspace that already holds the binding still refreshes at the limit —
	// the row count does not change, so neither cap applies.
	if _, err := storeA.BindGitConnection(ctx, GitConnection{
		WorkspaceID: workspaces[0], InstallationID: installationID, AccountLogin: "renamed",
	}, 0, 2); err != nil {
		t.Fatalf("same-pair refresh at the installation limit: %v", err)
	}
}

func TestDeleteTenantCascadesGitConnections(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	const workspaceID = "tea-git-cascade"
	seedGitTenant(t, st, workspaceID)
	if _, err := st.BindGitConnection(ctx, GitConnection{
		WorkspaceID: workspaceID, InstallationID: 909301, AccountLogin: "cascade-test",
	}, 0, 0); err != nil {
		t.Fatalf("BindGitConnection: %v", err)
	}

	if err := st.DeleteTenant(ctx, workspaceID); err != nil {
		t.Fatalf("DeleteTenant: %v", err)
	}
	if n, err := st.CountGitConnections(ctx, workspaceID); err != nil || n != 0 {
		t.Fatalf("CountGitConnections after tenant delete = %d, %v; want 0, nil", n, err)
	}
}
