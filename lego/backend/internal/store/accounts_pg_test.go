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
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestAccountDeletionStorePG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
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
	if _, err := pool.Exec(ctx, `TRUNCATE account_deletions, oauth_revocations, owner_ids, tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)
	assertAccountDeletionInventory(t, ctx, pool)

	solo, err := st.CreateWorkspace(ctx, "solo", PlanHobby, "identity-a")
	if err != nil {
		t.Fatal(err)
	}
	shared, err := st.CreateWorkspace(ctx, "shared", PlanPro, "identity-a")
	if err != nil {
		t.Fatal(err)
	}
	blocked, err := st.CreateWorkspace(ctx, "blocked", PlanPro, "identity-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES
		($1, 'identity-b', 'admin'), ($2, 'identity-b', 'viewer'), ($3, 'key-a', 'developer')`, shared.ID, blocked.ID, solo.ID); err != nil {
		t.Fatal(err)
	}

	machineSubjects := []string{"key-a"}
	preview, err := st.PreviewAccountDeletion(ctx, "identity-a", machineSubjects)
	if err != nil {
		t.Fatal(err)
	}
	actions := map[string]string{}
	for _, row := range preview {
		actions[row.ID] = row.Action
	}
	if actions[solo.ID] != "delete" || actions[shared.ID] != "leave" || actions[blocked.ID] != "blocked" {
		t.Fatalf("disposition=%v", actions)
	}
	if _, err := st.BeginAccountDeletion(ctx, "identity-a", "a@example.com", machineSubjects); err == nil {
		t.Fatal("blocked deletion wrote an intent")
	} else {
		var blockedErr *AccountDeletionBlockedError
		if !errors.As(err, &blockedErr) || len(blockedErr.Workspaces) != 1 {
			t.Fatalf("blocked error=%v", err)
		}
	}
	pending, err := st.AccountDeletionTombstoned(ctx, "identity-a")
	if err != nil || pending {
		t.Fatalf("pending after blocker=%v err=%v", pending, err)
	}

	if _, err := pool.Exec(ctx, `UPDATE tenant_members SET role = 'admin' WHERE tenant_id = $1 AND subject = 'identity-b'`, blocked.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO oauth_revocations (subject, client_id) VALUES ('identity-a', 'agent-client')`); err != nil {
		t.Fatal(err)
	}
	deletion, err := st.BeginAccountDeletion(ctx, "identity-a", "a@example.com", machineSubjects)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	if deletion.DeletedMarker == "" || deletion.State != AccountDeletionPending {
		t.Fatalf("deletion=%+v", deletion)
	}
	if _, err := st.CreateSSHKey(ctx, SSHKey{
		ID: "ssk-deletion-pending", Subject: "identity-a", Name: "too-late",
		PublicKey: "ssh-ed25519 AAAATEST", Fingerprint: "SHA256:deletion-pending",
	}); !errors.Is(err, ErrAccountDeletionPending) {
		t.Fatalf("SSH key creation after tombstone = %v, want ErrAccountDeletionPending", err)
	}
	var lateKeys int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM ssh_keys WHERE subject = 'identity-a'`).Scan(&lateKeys); err != nil {
		t.Fatal(err)
	}
	if lateKeys != 0 {
		t.Fatalf("SSH key survived deletion gate: %d", lateKeys)
	}
	claimed, err := st.ClaimAccountDeletions(ctx, 10)
	if err != nil || len(claimed) != 1 || claimed[0].DeletedMarker != deletion.DeletedMarker {
		t.Fatalf("claim=%+v err=%v", claimed, err)
	}
	if err := st.AdvanceAccountDeletion(ctx, "identity-a", AccountDeletionPending, AccountDeletionCleaning); err != nil {
		t.Fatalf("advance: %v", err)
	}
	var leaseRetained bool
	if err := pool.QueryRow(ctx, `SELECT claimed_until > now() FROM account_deletions WHERE subject = 'identity-a'`).Scan(&leaseRetained); err != nil {
		t.Fatal(err)
	}
	if !leaseRetained {
		t.Fatal("intermediate transition released its worker lease")
	}
	if err := st.RemoveAccountMember(ctx, shared.ID, "identity-a"); err != nil {
		t.Fatalf("leave shared: %v", err)
	}
	if err := st.CleanupAccountSubject(ctx, "identity-a", deletion.DeletedMarker); err != nil {
		t.Fatalf("cleanup: %v", err)
	}
	if err := st.CleanupAccountSubject(ctx, "identity-a", deletion.DeletedMarker); err != nil {
		t.Fatalf("idempotent cleanup: %v", err)
	}
	var markerRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM owner_ids WHERE subject = $1`, deletion.DeletedMarker).Scan(&markerRows); err != nil {
		t.Fatal(err)
	}
	if markerRows != 1 {
		t.Fatalf("deleted marker rows=%d", markerRows)
	}
	var revocationSubject string
	if err := pool.QueryRow(ctx, `SELECT subject FROM oauth_revocations WHERE client_id = 'agent-client'`).Scan(&revocationSubject); err != nil {
		t.Fatal(err)
	}
	if revocationSubject != deletion.DeletedMarker {
		t.Fatalf("OAuth revocation subject=%q want deleted marker", revocationSubject)
	}

	if err := st.FailAccountDeletion(ctx, "identity-a", strings.Repeat("x", 700)); err != nil {
		t.Fatalf("record retry: %v", err)
	}
	var errorLength int
	var claimReleased, backedOff bool
	if err := pool.QueryRow(ctx, `
		SELECT length(last_error), claimed_until IS NULL, next_attempt_at > now()
		FROM account_deletions WHERE subject = 'identity-a'`,
	).Scan(&errorLength, &claimReleased, &backedOff); err != nil {
		t.Fatal(err)
	}
	if errorLength != 500 || !claimReleased || !backedOff {
		t.Fatalf("retry metadata length=%d released=%v backedOff=%v", errorLength, claimReleased, backedOff)
	}
	if _, err := pool.Exec(ctx, `UPDATE account_deletions SET next_attempt_at = now() - interval '1 second' WHERE subject = 'identity-a'`); err != nil {
		t.Fatal(err)
	}
	reclaimed, err := st.ClaimAccountDeletions(ctx, 1)
	if err != nil || len(reclaimed) != 1 || reclaimed[0].Subject != "identity-a" {
		t.Fatalf("reclaim=%+v err=%v", reclaimed, err)
	}

	concurrent, err := st.CreateWorkspace(ctx, "concurrent-begin", PlanHobby, "identity-concurrent")
	if err != nil {
		t.Fatal(err)
	}
	const beginWorkers = 8
	beginResults := make(chan AccountDeletion, beginWorkers)
	beginErrors := make(chan error, beginWorkers)
	var beginWG sync.WaitGroup
	for range beginWorkers {
		beginWG.Add(1)
		go func() {
			deletion, beginErr := st.BeginAccountDeletion(ctx, "identity-concurrent", "concurrent@example.com", []string{})
			if beginErr != nil {
				beginErrors <- beginErr
				beginWG.Done()
				return
			}
			beginResults <- deletion
			beginWG.Done()
		}()
	}
	beginWG.Wait()
	close(beginResults)
	close(beginErrors)
	for beginErr := range beginErrors {
		t.Fatalf("concurrent begin: %v", beginErr)
	}
	markers := map[string]bool{}
	for result := range beginResults {
		markers[result.DeletedMarker] = true
	}
	if len(markers) != 1 {
		t.Fatalf("concurrent begin markers=%v", markers)
	}
	var intentRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM account_deletions WHERE subject = 'identity-concurrent'`).Scan(&intentRows); err != nil {
		t.Fatal(err)
	}
	if intentRows != 1 {
		t.Fatalf("concurrent begin wrote %d rows", intentRows)
	}
	if len(concurrent.ID) == 0 {
		t.Fatal("concurrent workspace missing")
	}

	raceWorkspace, err := st.CreateWorkspace(ctx, "last-admin-race", PlanPro, "identity-race-a")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `INSERT INTO tenant_members (tenant_id, subject, role) VALUES ($1, 'identity-race-b', 'admin')`, raceWorkspace.ID); err != nil {
		t.Fatal(err)
	}
	removeErrors := make(chan error, 2)
	var removeWG sync.WaitGroup
	for _, subject := range []string{"identity-race-a", "identity-race-b"} {
		subject := subject
		removeWG.Add(1)
		go func() {
			removeErrors <- st.RemoveAccountMember(ctx, raceWorkspace.ID, subject)
			removeWG.Done()
		}()
	}
	removeWG.Wait()
	close(removeErrors)
	var removed, refused int
	for removeErr := range removeErrors {
		switch {
		case removeErr == nil:
			removed++
		case errors.Is(removeErr, ErrLastAdmin):
			refused++
		default:
			t.Fatalf("last-admin race: %v", removeErr)
		}
	}
	var remainingAdmins int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM tenant_members WHERE tenant_id = $1 AND role = 'admin'`, raceWorkspace.ID).Scan(&remainingAdmins); err != nil {
		t.Fatal(err)
	}
	if removed != 1 || refused != 1 || remainingAdmins != 1 {
		t.Fatalf("last-admin race removed=%d refused=%d admins=%d", removed, refused, remainingAdmins)
	}
}

// assertAccountDeletionInventory turns the ADR086 disposition table into a
// schema tripwire. A future identity/provenance-shaped column must be added to
// both this list and the deletion policy instead of silently retaining data.
func assertAccountDeletionInventory(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	rows, err := pool.Query(ctx, `
		SELECT table_name || '.' || column_name
		FROM information_schema.columns
		WHERE table_schema = 'public'
		  AND column_name = ANY($1::text[])
		ORDER BY table_name, column_name`, []string{
		"subject", "owner_identity_id", "email", "invited_by", "caller", "created_by",
	})
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var got []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			t.Fatal(err)
		}
		got = append(got, column)
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"account_deletions.subject",
		"audit_events.caller",
		"device_push_subscriptions.subject",
		"github_connect_transactions.subject",
		"membership_role_reconciliations.subject",
		"notification_settings.subject",
		"oauth_revocations.subject",
		"owner_ids.subject",
		"push_deliveries.subject",
		"push_notifications.subject",
		"registry_credentials.created_by",
		"ssh_keys.subject",
		"ssh_sessions.subject",
		"tenant_invites.email",
		"tenant_invites.invited_by",
		"tenant_members.subject",
		"tenants.owner_identity_id",
		"webhook_endpoints.created_by",
		"webpush_subscriptions.subject",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("identity/provenance schema inventory changed:\n got %v\nwant %v", got, want)
	}
}
