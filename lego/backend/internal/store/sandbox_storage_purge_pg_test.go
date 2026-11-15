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
	"os"
	"slices"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

func TestWorkspaceOfSandboxNamespace(t *testing.T) {
	ws := ids.New(ids.Workspace)
	for _, c := range []struct {
		ns, want string
		ok       bool
	}{
		{SandboxNamespace(ws), ws, true},
		{ws, "", false},                                     // the hosting namespace, not the sandbox one
		{SandboxNamespace(ids.New(ids.Service)), "", false}, // not a workspace id
		{SandboxNamespace("tea-short"), "", false},          // not a well-formed id
		{"-sandbox", "", false},
		{"", "", false},
	} {
		if got, ok := WorkspaceOfSandboxNamespace(c.ns); got != c.want || ok != c.ok {
			t.Errorf("WorkspaceOfSandboxNamespace(%q) = %q, %v; want %q, %v", c.ns, got, ok, c.want, c.ok)
		}
	}
}

func openSandboxPurgePG(t *testing.T) (*PGStore, *pgxpool.Pool) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(context.Background(), uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return NewPGStore(pool), pool
}

// SandboxRootfsRefs is what the workspace purge deletes from the registry
// before the tenant row cascades the objects away: every rootfs image a
// workspace's sandboxes retained, open, closed or already queued for the reaper,
// and nothing of another workspace's or of another kind.
func TestSandboxRootfsRefsPG(t *testing.T) {
	s, pool := openSandboxPurgePG(t)
	ctx := context.Background()
	mk := func(name string) string {
		t.Helper()
		tenant, err := s.CreateTenant(ctx, name+"-"+ids.New(ids.Workspace), PlanPro)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.DeleteTenant(ctx, tenant.ID) })
		return tenant.ID
	}
	ws, other := mk("rootfs-refs"), mk("rootfs-refs-other")
	at := time.Date(2026, 8, 1, 0, 0, 0, 0, time.UTC)
	start := func(workspace, object, kind, ref string) {
		t.Helper()
		if err := s.StartSandboxStorage(ctx, workspace, object, kind, ref, at); err != nil {
			t.Fatal(err)
		}
	}
	start(ws, "sbx-open", StorageKindRootfs, "os-open")
	start(ws, "sbx-closed", StorageKindRootfs, "os-closed")
	if err := s.EndSandboxStorage(ctx, ws, "sbx-closed", at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	start(ws, "sbx-queued", StorageKindRootfs, "os-queued")
	if _, err := pool.Exec(ctx, `UPDATE sandbox_storage_objects SET ended_at = $3, reap_pending = true WHERE workspace_id = $1 AND object_id = $2`, ws, "sbx-queued", at.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	start(ws, "sbx-noref", StorageKindRootfs, "")
	start(ws, "sbx-snapshot", StorageKindSnapshot, "os-snapshot")
	start(other, "sbx-other", StorageKindRootfs, "os-other")

	refs, err := s.SandboxRootfsRefs(ctx, ws)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(refs)
	if want := []string{"os-closed", "os-open", "os-queued"}; !slices.Equal(refs, want) {
		t.Errorf("refs = %v, want %v: every rootfs image with a ref, whether open, closed or queued, and no snapshot, blank ref or other workspace's", refs, want)
	}
	if refs, err := s.SandboxRootfsRefs(ctx, "tea-unknown"); err != nil || len(refs) != 0 {
		t.Errorf("refs of an unknown workspace = %v, %v", refs, err)
	}
}

// ExistingWorkspaceIDs is the orphan sweep's only reason to keep a registry
// namespace: a workspace with a row (whatever its teardown has reached) or one
// being created. A deleted workspace, a finished or abandoned attempt that never
// made a row, and an id nothing names are not.
func TestExistingWorkspaceIDsPG(t *testing.T) {
	s, pool := openSandboxPurgePG(t)
	ctx := context.Background()
	live, err := s.CreateTenant(ctx, "existing-live-"+ids.New(ids.Workspace), PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteTenant(ctx, live.ID) })
	gone, err := s.CreateTenant(ctx, "existing-gone-"+ids.New(ids.Workspace), PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteTenant(ctx, gone.ID); err != nil {
		t.Fatal(err)
	}
	attempt := func(state string) string {
		t.Helper()
		workspace := ids.New(ids.Workspace)
		id := ids.New(ids.WorkspaceCreationAttempt)
		if _, err := pool.Exec(ctx, `
			INSERT INTO workspace_creation_attempts
				(id, workspace_id, owner_subject, name, plan, billing_email, payment_required, state, finalized_at, expires_at)
			VALUES ($1, $2, 'owner', $3, 'pro', 'a@example.com', false, $4,
				CASE WHEN $4 = 'finalized' THEN now() END, now() + interval '1 hour')`,
			id, workspace, "attempt-"+id, state); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM workspace_creation_attempts WHERE id = $1`, id) })
		return workspace
	}
	preparing, pending, succeeded := attempt("prepared"), attempt("setup_pending"), attempt("setup_succeeded")
	finalizedAway, cancelled, expired := attempt("finalized"), attempt("cleanup_pending"), attempt("expired")
	unknown := ids.New(ids.Workspace)

	got, err := s.ExistingWorkspaceIDs(ctx, []string{live.ID, gone.ID, preparing, pending, succeeded, finalizedAway, cancelled, expired, unknown, live.ID})
	if err != nil {
		t.Fatal(err)
	}
	for id, want := range map[string]bool{
		live.ID: true, preparing: true, pending: true, succeeded: true, // a workspace that is, or is being, created
		gone.ID: false, finalizedAway: false, cancelled: false, expired: false, unknown: false,
	} {
		if got[id] != want {
			t.Errorf("ExistingWorkspaceIDs[%s] = %v, want %v", id, got[id], want)
		}
	}
	if empty, err := s.ExistingWorkspaceIDs(ctx, nil); err != nil || len(empty) != 0 {
		t.Errorf("no ids = %v, %v", empty, err)
	}
}
