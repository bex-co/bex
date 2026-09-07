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
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// opsGuardPGStore is the shared harness for the ADR087 §4 store-level guard
// tests: real Postgres (skipped without BEX_TEST_DB_URI, run in CI), migrated
// and truncated like every sibling PG test.
func opsGuardPGStore(t *testing.T) (*PGStore, context.Context) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE account_deletions, owner_ids, tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	return NewPGStore(pool), ctx
}

// TestOpsWorkspaceInviteExemptionPG: the shared redemption core
// (planAllowsJoin, behind BOTH AcceptInviteByToken and AcceptInvitesForEmail)
// exempts the pinned ops workspace from seat/plan gating — a Hobby seat cap or
// role restriction must never silently block onboarding an operator — while
// the identical invite into an ordinary workspace stays refused (ADR087 §4).
func TestOpsWorkspaceInviteExemptionPG(t *testing.T) {
	s, ctx := opsGuardPGStore(t)

	// A Hobby workspace is the harshest gate: MaxMembers 1 (the owner fills
	// it) and AllowedRoles [admin] (a developer invite fails the role check
	// too). The pin must beat both halves.
	ops, err := s.CreateWorkspace(ctx, "ops", PlanHobby, "identity-owner")
	if err != nil {
		t.Fatalf("create ops workspace: %v", err)
	}
	expires := time.Now().Add(time.Hour)
	if _, err := s.CreateInvite(ctx, ops.ID, "op2@example.com", "developer", "tok-ops-dev", "identity-owner", expires); err != nil {
		t.Fatalf("create invite: %v", err)
	}

	// Unset pin => the guard is inert: both redemption paths refuse/skip
	// exactly as before.
	if _, err := s.AcceptInviteByToken(ctx, "tok-ops-dev", "identity-op2"); !errors.Is(err, ErrInvitePlanLimit) {
		t.Fatalf("unpinned token redemption: want ErrInvitePlanLimit, got %v", err)
	}
	if accepted, err := s.AcceptInvitesForEmail(ctx, "op2@example.com", "identity-op2"); err != nil || len(accepted) != 0 {
		t.Fatalf("unpinned login redemption: accepted=%v err=%v (want silent skip)", accepted, err)
	}

	// Pin the workspace: the SAME pending invite now redeems on the login
	// path, past both the seat cap and the role-per-plan gate.
	s.OpsWorkspaceID = ops.ID
	accepted, err := s.AcceptInvitesForEmail(ctx, "op2@example.com", "identity-op2")
	if err != nil || len(accepted) != 1 || accepted[0].Role != "developer" {
		t.Fatalf("pinned login redemption: accepted=%v err=%v", accepted, err)
	}
	if m, err := s.GetTenantMember(ctx, ops.ID, "identity-op2"); err != nil || m.Role != "developer" {
		t.Fatalf("member after pinned redemption = %+v (%v)", m, err)
	}

	// The token path shares the exemption: a third seat (viewer — also not a
	// Hobby role) joins a workspace already past its cap.
	if _, err := s.CreateInvite(ctx, ops.ID, "op3@example.com", "viewer", "tok-ops-view", "identity-owner", expires); err != nil {
		t.Fatalf("create viewer invite: %v", err)
	}
	if acc, err := s.AcceptInviteByToken(ctx, "tok-ops-view", "identity-op3"); err != nil || acc.Role != "viewer" {
		t.Fatalf("pinned token redemption: %+v %v", acc, err)
	}

	// Control: the pin exempts ONLY the ops workspace — an ordinary Hobby
	// workspace's identical invite stays refused while the pin is set.
	plain, err := s.CreateWorkspace(ctx, "plain", PlanHobby, "identity-other")
	if err != nil {
		t.Fatalf("create ordinary workspace: %v", err)
	}
	if _, err := s.CreateInvite(ctx, plain.ID, "second@example.com", "admin", "tok-plain", "identity-other", expires); err != nil {
		t.Fatalf("create ordinary invite: %v", err)
	}
	if _, err := s.AcceptInviteByToken(ctx, "tok-plain", "identity-second"); !errors.Is(err, ErrInvitePlanLimit) {
		t.Fatalf("ordinary workspace must keep its seat cap: got %v", err)
	}
}

// TestAccountDeletionOpsWorkspaceBlockedPG: ADR086's sole-member `delete`
// disposition classifies the pinned ops workspace BLOCKED instead — its
// teardown would lock every operator out of the observability UI — so the
// deletion request refuses up front; unpinned, the same workspace previews as
// delete (zero behavior change).
func TestAccountDeletionOpsWorkspaceBlockedPG(t *testing.T) {
	s, ctx := opsGuardPGStore(t)

	ops, err := s.CreateWorkspace(ctx, "ops", PlanPro, "identity-op")
	if err != nil {
		t.Fatalf("create ops workspace: %v", err)
	}

	// Unpinned: a sole-member workspace previews as delete (the pre-ADR087
	// disposition, unchanged).
	preview, err := s.PreviewAccountDeletion(ctx, "identity-op", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != 1 || preview[0].Action != AccountWorkspaceDelete {
		t.Fatalf("unpinned disposition = %+v, want delete", preview)
	}

	// Pinned: the same workspace classifies blocked, and BeginAccountDeletion
	// refuses with the typed blocked error naming it.
	s.OpsWorkspaceID = ops.ID
	preview, err = s.PreviewAccountDeletion(ctx, "identity-op", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != 1 || preview[0].Action != AccountWorkspaceBlocked {
		t.Fatalf("pinned disposition = %+v, want blocked", preview)
	}
	_, err = s.BeginAccountDeletion(ctx, "identity-op", "op@example.com", nil)
	var blockedErr *AccountDeletionBlockedError
	if !errors.As(err, &blockedErr) || len(blockedErr.Workspaces) != 1 || blockedErr.Workspaces[0].ID != ops.ID {
		t.Fatalf("begin with pinned sole-member ops workspace = %v, want AccountDeletionBlockedError naming it", err)
	}
	if pending, err := s.AccountDeletionTombstoned(ctx, "identity-op"); err != nil || pending {
		t.Fatalf("blocked deletion left a tombstone: pending=%v err=%v", pending, err)
	}

	// A member of the ops workspace who is NOT its last admin still classifies
	// leave — the pin blocks teardown, not offboarding.
	if _, err := s.Pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, 'identity-op2', 'admin')`, ops.ID); err != nil {
		t.Fatal(err)
	}
	preview, err = s.PreviewAccountDeletion(ctx, "identity-op", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(preview) != 1 || preview[0].Action != AccountWorkspaceLeave {
		t.Fatalf("shared pinned workspace disposition = %+v, want leave", preview)
	}
}
