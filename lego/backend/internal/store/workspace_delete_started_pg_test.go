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

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// workspace_delete_started_pg_test.go is w9/121's store half against REAL
// Postgres. Once a workspace's delete has started, DeletingTenantAdmin names
// exactly the people recorded as its admins, and no path seats anyone in it or
// re-roles anyone there: not an invite by email or by link, not an org join,
// not a role change, not the role outbox.
//
// Additive: subjects are unique per run, and it never truncates.
func TestWorkspaceDeleteStartedPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)

	run := uniqueMachineRun()
	admin, dev, removed, key := "admin-"+run, "dev-"+run, "removed-"+run, "key-"+run
	doomed, err := st.CreateWorkspace(ctx, "doomed-"+run, PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	kept, err := st.CreateWorkspace(ctx, "kept-"+run, PlanPro, admin)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(ctx, dev, doomed.ID, "developer"); err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(ctx, removed, doomed.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveMember(ctx, doomed.ID, removed); err != nil {
		t.Fatal(err)
	}
	// An API key the admin minted there is bound at the admin's role.
	if role, err := st.BindClientForCreator(ctx, key, doomed.ID, admin); err != nil || role != "admin" {
		t.Fatalf("bind the admin's key: %q, %v", role, err)
	}
	// A role change whose OpenFGA write is still owed when the delete starts.
	if err := st.UpdateMemberRole(ctx, doomed.ID, dev, "viewer"); err != nil {
		t.Fatal(err)
	}
	expires := time.Now().Add(time.Hour)
	email := "late-" + run + "@example.com"
	for _, w := range []Tenant{doomed, kept} {
		if _, err := st.CreateInvite(ctx, w.ID, email, "developer", "tok-"+w.ID, admin, expires); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := st.CreateInvite(ctx, doomed.ID, "link-"+run+"@example.com", "developer", "tok-link-"+run, admin, expires); err != nil {
		t.Fatal(err)
	}

	deletingAdmin := func(workspace, subject string) bool {
		t.Helper()
		ok, err := st.DeletingTenantAdmin(ctx, workspace, subject)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	started := func(workspace string) bool {
		t.Helper()
		ok, err := st.TenantDeleteStarted(ctx, workspace)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if started(doomed.ID) || deletingAdmin(doomed.ID, admin) {
		t.Fatal("precondition: no delete has started, so nobody is a deleting admin")
	}

	if err := st.StartTenantDelete(ctx, doomed.ID); err != nil {
		t.Fatal(err)
	}
	var first time.Time
	if err := pool.QueryRow(ctx, `SELECT delete_started_at FROM tenants WHERE id = $1`, doomed.ID).Scan(&first); err != nil {
		t.Fatal(err)
	}
	if err := st.StartTenantDelete(ctx, doomed.ID); err != nil {
		t.Fatalf("a retry's start: %v", err)
	}
	var again time.Time
	if err := pool.QueryRow(ctx, `SELECT delete_started_at FROM tenants WHERE id = $1`, doomed.ID).Scan(&again); err != nil || !again.Equal(first) {
		t.Fatalf("a retry moved the start from %v to %v (%v)", first, again, err)
	}
	if err := st.StartTenantDelete(ctx, "tea-"+run+"missing"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("starting a missing workspace's delete: %v, want not found", err)
	}
	if !started(doomed.ID) || started(kept.ID) {
		t.Fatalf("started: doomed %v, kept %v", started(doomed.ID), started(kept.ID))
	}

	// Only the recorded admin of the workspace being deleted.
	for _, c := range []struct {
		name, workspace, subject string
		want                     bool
	}{
		{"its admin", doomed.ID, admin, true},
		{"a member at another role", doomed.ID, dev, false},
		{"an admin removed before the delete", doomed.ID, removed, false},
		{"an API key bound as admin", doomed.ID, key, false},
		{"a stranger", doomed.ID, "stranger-" + run, false},
		{"its admin, on a workspace no delete has reached", kept.ID, admin, false},
	} {
		if got := deletingAdmin(c.workspace, c.subject); got != c.want {
			t.Errorf("%s: DeletingTenantAdmin = %v, want %v", c.name, got, c.want)
		}
	}

	// The outbox holds the developer's owed role change, and hands it to no
	// worker while the delete runs.
	claimed, err := st.ClaimRoleReconciliations(ctx, 500)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range claimed {
		if row.TenantID == doomed.ID {
			t.Fatalf("the outbox handed out %+v for a workspace being deleted", row)
		}
	}
	var owed int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM membership_role_reconciliations WHERE tenant_id = $1 AND subject = $2`, doomed.ID, dev).Scan(&owed); err != nil || owed != 1 {
		t.Fatalf("precondition: the developer's owed role change: %d rows (%v)", owed, err)
	}

	// The same email's invites: the workspace being deleted is skipped, and
	// skipping it does not cost the other workspace its new member.
	newcomer := "newcomer-" + run
	accepted, err := st.AcceptInvitesForEmail(ctx, email, newcomer)
	if err != nil {
		t.Fatal(err)
	}
	if len(accepted) != 1 || accepted[0].TenantID != kept.ID {
		t.Fatalf("redeemed %+v, want only the invite to %s", accepted, kept.ID)
	}
	member := func(workspace, subject string) bool {
		t.Helper()
		ok, err := st.IsMember(ctx, subject, workspace)
		if err != nil {
			t.Fatal(err)
		}
		return ok
	}
	if member(doomed.ID, newcomer) {
		t.Fatal("an invite seated the newcomer in a workspace being deleted")
	}
	if _, err := st.AcceptInviteByToken(ctx, "tok-link-"+run, "linked-"+run); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an invite link into a workspace being deleted: %v, want not found", err)
	}
	if member(doomed.ID, "linked-"+run) {
		t.Fatal("an invite link seated someone in a workspace being deleted")
	}
	if _, err := st.JoinOrganizationWorkspace(ctx, doomed.ID, "owner-"+run, "admin"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("an org join into a workspace being deleted: %v, want not found", err)
	}
	if member(doomed.ID, "owner-"+run) {
		t.Fatal("an org join seated someone in a workspace being deleted")
	}
	if err := st.UpdateMemberRole(ctx, doomed.ID, dev, "admin"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("a role change in a workspace being deleted: %v, want not found", err)
	}
	if m, err := st.GetTenantMember(ctx, doomed.ID, dev); err != nil || m.Role != "viewer" {
		t.Fatalf("the developer's role after a refused change: %+v (%v), want viewer", m, err)
	}

	// The cascade ends it: nobody is a deleting admin of a workspace that is gone.
	if err := st.DeleteTenant(ctx, doomed.ID); err != nil {
		t.Fatal(err)
	}
	if deletingAdmin(doomed.ID, admin) || started(doomed.ID) {
		t.Fatal("a deleted workspace still answers as being deleted")
	}
}
