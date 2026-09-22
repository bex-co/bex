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
	"reflect"
	"strings"
	"testing"
	"time"
)

func accountCreationAttempt(t *testing.T, st *PGStore, subject, email, state string) WorkspaceCreationAttempt {
	t.Helper()
	ctx := context.Background()
	a, err := st.CreateWorkspaceCreationAttempt(ctx, subject, "account-attempt", PlanPro, email, true, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := st.Pool.Exec(ctx, `DELETE FROM workspace_creation_attempts WHERE id=$1`, a.ID); err != nil {
			t.Errorf("clean up attempt: %v", err)
		}
	})
	// Populate every correlation independently of state to prove cleanup never
	// loses a provider handle, including handles retained after an old failure.
	a, err = scanWorkspaceCreationAttempt(st.Pool.QueryRow(ctx, `
		UPDATE workspace_creation_attempts SET state=$2,
			provider_customer_id='cus_' || id, provider_setup_intent_id='seti_' || id,
			provider_payment_method_id='pm_' || id, provider_subscription_id='sub_' || id,
			provider_livemode=false,
			finalized_at=CASE WHEN $2='finalized' THEN now()-interval '10 days' END,
			cleanup_claimed_until=CASE WHEN $2='cleanup_pending' THEN now()+interval '15 minutes' END,
			updated_at=now()-interval '10 days'
		WHERE id=$1 RETURNING `+workspaceCreationColumns, a.ID, state))
	if err != nil {
		t.Fatal(err)
	}
	return a
}

func TestAccountDeletionAnonymizesWorkspaceCreationAttemptsPG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	subject, other := "creation-owner-"+run, "creation-other-"+run
	email, alternate := "owner-"+run+"@example.test", "billing-"+run+"@example.test"
	states := []string{
		WorkspaceCreationPrepared, WorkspaceCreationSetupPending, WorkspaceCreationSetupSucceeded,
		WorkspaceCreationCleanupPending, WorkspaceCreationFinalized, WorkspaceCreationExpired,
	}
	before := make(map[string]WorkspaceCreationAttempt)
	for i, state := range states {
		billingEmail := email
		if i%2 != 0 {
			billingEmail = alternate
		}
		before[state] = accountCreationAttempt(t, st, subject, billingEmail, state)
	}
	sameEmail := accountCreationAttempt(t, st, other, email, WorkspaceCreationPrepared)
	unrelated := accountCreationAttempt(t, st, other, alternate, WorkspaceCreationPrepared)

	deletion, err := st.BeginAccountDeletion(ctx, subject, "  "+strings.ToUpper(email)+"  ", nil)
	if err != nil {
		t.Fatalf("begin deletion: %v", err)
	}
	anonymousEmail := strings.ReplaceAll(deletion.DeletedMarker, ":", "-") + "@invalid"
	afterBegin := make(map[string]WorkspaceCreationAttempt)
	for state, previous := range before {
		a, err := st.GetWorkspaceCreationAttempt(ctx, previous.ID, subject)
		if err != nil {
			t.Fatal(err)
		}
		wantState := state
		if state == WorkspaceCreationPrepared || state == WorkspaceCreationSetupPending || state == WorkspaceCreationSetupSucceeded {
			wantState = WorkspaceCreationCleanupPending
			if a.ExpiresAt.After(time.Now()) || a.CleanupClaimedUntil != nil {
				t.Errorf("%s was not made immediately eligible for cleanup: %+v", state, a)
			}
		} else if !a.UpdatedAt.Equal(previous.UpdatedAt) || !reflect.DeepEqual(a.CleanupClaimedUntil, previous.CleanupClaimedUntil) {
			t.Errorf("%s retention clock or existing cleanup lease changed", state)
		}
		wantEmail := previous.BillingEmail
		if wantEmail == email {
			wantEmail = anonymousEmail
		}
		if a.State != wantState || a.BillingEmail != wantEmail {
			t.Errorf("after intent %s: state=%s email=%s, want %s/%s", state, a.State, a.BillingEmail, wantState, wantEmail)
		}
		afterBegin[state] = a
	}
	for range 2 {
		if err := st.CleanupAccountSubject(ctx, subject, deletion.DeletedMarker); err != nil {
			t.Fatalf("subject cleanup: %v", err)
		}
	}
	for state, previous := range afterBegin {
		if _, err := st.GetWorkspaceCreationAttempt(ctx, previous.ID, subject); !errors.Is(err, ErrNotFound) {
			t.Errorf("deleted owner can still resolve %s attempt: %v", state, err)
		}
		a, err := st.GetWorkspaceCreationAttempt(ctx, previous.ID, deletion.DeletedMarker)
		if err != nil {
			t.Fatal(err)
		}
		want := previous
		want.OwnerSubject, want.BillingEmail = deletion.DeletedMarker, anonymousEmail
		if !reflect.DeepEqual(a, want) {
			t.Errorf("%s cleanup changed retained state: got %+v, want %+v", state, a, want)
		}
	}
	for _, expected := range []WorkspaceCreationAttempt{sameEmail, unrelated} {
		a, err := st.GetWorkspaceCreationAttempt(ctx, expected.ID, other)
		if err != nil {
			t.Fatal(err)
		}
		if expected.BillingEmail == email {
			expected.BillingEmail = anonymousEmail
		}
		if !reflect.DeepEqual(a, expected) {
			t.Errorf("other owner's attempt changed beyond matching email: got %+v, want %+v", a, expected)
		}
	}

	// Financial cleanup is keyed by the attempt, not its anonymized owner. A
	// failed provider cleanup must retain the handles and remain retryable.
	target := before[WorkspaceCreationSetupSucceeded]
	claimed, err := st.ExpireWorkspaceCreationAttempts(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, a := range claimed {
		if a.ID == before[WorkspaceCreationFinalized].ID || a.ID == before[WorkspaceCreationExpired].ID || a.ID == before[WorkspaceCreationCleanupPending].ID {
			t.Errorf("claimed a terminal attempt or an existing live lease: %s", a.ID)
		}
		if a.ID == target.ID {
			found = true
			if a.OwnerSubject != deletion.DeletedMarker || a.BillingEmail != anonymousEmail || a.ProviderCustomerID != target.ProviderCustomerID || a.ProviderSetupIntentID != target.ProviderSetupIntentID {
				t.Fatalf("cleanup lost provider correlations: %+v", a)
			}
		}
	}
	if !found {
		t.Fatal("anonymized attempt was not claimable for provider cleanup")
	}
	if err := st.FinishWorkspaceCreationCleanup(ctx, target.ID, false); err != nil {
		t.Fatal(err)
	}
	failed, err := st.GetWorkspaceCreationAttempt(ctx, target.ID, deletion.DeletedMarker)
	if err != nil || failed.State != WorkspaceCreationCleanupPending || failed.CleanupClaimedUntil == nil || !failed.CleanupClaimedUntil.After(time.Now()) {
		t.Fatalf("failed cleanup is not retryable: %+v err=%v", failed, err)
	}
	if _, err := st.Pool.Exec(ctx, `UPDATE workspace_creation_attempts SET cleanup_claimed_until=now()-interval '1 second' WHERE id=$1`, target.ID); err != nil {
		t.Fatal(err)
	}
	retried, err := st.ExpireWorkspaceCreationAttempts(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	found = false
	for _, a := range retried {
		if a.ID == target.ID {
			found = true
			if err := st.FinishWorkspaceCreationCleanup(ctx, a.ID, true); err != nil {
				t.Fatal(err)
			}
		}
	}
	if !found {
		t.Fatal("anonymized attempt was not reclaimed after provider cleanup failure")
	}
	finished, err := st.GetWorkspaceCreationAttempt(ctx, target.ID, deletion.DeletedMarker)
	if err != nil || finished.State != WorkspaceCreationExpired {
		t.Fatalf("cleanup completion: %+v err=%v", finished, err)
	}
}

func TestAccountDeletionAnonymizesWorkspaceBillingEmailPG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	subject, other := "billing-owner-"+run, "billing-admin-"+run
	email, alternate := "owner-"+run+"@example.test", "team-"+run+"@example.test"
	var workspaces []Tenant
	for _, billingEmail := range []string{email, alternate} {
		workspace, err := st.CreateWorkspace(ctx, "billing-"+uniqueMachineRun(), PlanPro, subject)
		if err != nil {
			t.Fatal(err)
		}
		if err := st.AddMember(ctx, other, workspace.ID, "admin"); err != nil {
			t.Fatal(err)
		}
		if _, err := st.Pool.Exec(ctx, `UPDATE tenants SET billing_email=$2 WHERE id=$1`, workspace.ID, billingEmail); err != nil {
			t.Fatal(err)
		}
		workspaces = append(workspaces, workspace)
	}
	deletion, err := st.BeginAccountDeletion(ctx, subject, email, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, workspace := range workspaces {
		if err := st.RemoveAccountMember(ctx, workspace.ID, subject); err != nil {
			t.Fatal(err)
		}
	}
	if err := st.CleanupAccountSubject(ctx, subject, deletion.DeletedMarker); err != nil {
		t.Fatal(err)
	}
	for i, workspace := range workspaces {
		var got string
		if err := st.Pool.QueryRow(ctx, `SELECT billing_email FROM tenants WHERE id=$1`, workspace.ID).Scan(&got); err != nil {
			t.Fatal(err)
		}
		want := alternate
		if i == 0 {
			want = strings.ReplaceAll(deletion.DeletedMarker, ":", "-") + "@invalid"
		}
		if got != want {
			t.Errorf("surviving workspace billing email=%q, want %q", got, want)
		}
	}
}

func TestAccountDeletionBlocksLateWorkspaceCreationPG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	subject := "late-creator-" + uniqueMachineRun()
	// Even if the account's current email is unavailable, a previously supplied
	// billing address on its own attempt must not survive subject cleanup.
	a := accountCreationAttempt(t, st, subject, subject+"@example.test", WorkspaceCreationExpired)
	deletion, err := st.BeginAccountDeletion(ctx, subject, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.CleanupAccountSubject(ctx, subject, deletion.DeletedMarker); err != nil {
		t.Fatal(err)
	}
	cleaned, err := st.GetWorkspaceCreationAttempt(ctx, a.ID, deletion.DeletedMarker)
	if err != nil || cleaned.BillingEmail != strings.ReplaceAll(deletion.DeletedMarker, ":", "-")+"@invalid" {
		t.Fatalf("cleanup without account email: %+v err=%v", cleaned, err)
	}
	// This models a request authenticated before deletion that reaches the
	// store after the cleanup pass. Only the durable store gate can reject it.
	if _, err := st.CreateWorkspaceCreationAttempt(ctx, subject, "too-late", PlanPro, subject+"@example.test", true, time.Now().Add(time.Hour)); !errors.Is(err, ErrAccountDeletionPending) {
		t.Fatalf("late workspace creation: %v, want ErrAccountDeletionPending", err)
	}
	var count int
	if err := st.Pool.QueryRow(ctx, `SELECT count(*) FROM workspace_creation_attempts WHERE owner_subject=$1 OR billing_email=$2`, subject, subject+"@example.test").Scan(&count); err != nil || count != 0 {
		t.Fatalf("late attempt restored deleted identity: count=%d err=%v", count, err)
	}
}

func TestAccountDeletionAnonymizesDeployProvenancePG(t *testing.T) {
	st := newReplayTestStore(t)
	ctx := context.Background()
	run := uniqueMachineRun()
	subject, other := "deploy-owner-"+run, "deploy-admin-"+run
	tenant, err := st.CreateWorkspace(ctx, "deploy-history-"+run, PlanPro, subject)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.AddMember(ctx, other, tenant.ID, "admin"); err != nil {
		t.Fatal(err)
	}
	app, err := st.CreateApp(ctx, App{TenantID: tenant.ID, Name: "web", Image: "nginx:1", Port: 80, Replicas: 1, Tier: "starter"})
	if err != nil {
		t.Fatal(err)
	}
	var deployments []Deploy
	for i, actor := range []string{subject, other} {
		deployment, err := st.CreateDeploy(ctx, app.ID, TriggerAPI, "nginx:1", int64(i+2), CommitInfo{Hash: "abc123", Message: "retained deploy"}, actor)
		if err != nil {
			t.Fatal(err)
		}
		deployments = append(deployments, deployment)
	}
	for i, deployment := range deployments {
		deployments[i], err = st.GetDeploy(ctx, app.ID, deployment.ID)
		if err != nil {
			t.Fatal(err)
		}
	}
	deletion, err := st.BeginAccountDeletion(ctx, subject, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := st.RemoveAccountMember(ctx, tenant.ID, subject); err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if err := st.CleanupAccountSubject(ctx, subject, deletion.DeletedMarker); err != nil {
			t.Fatal(err)
		}
	}
	for _, expected := range deployments {
		got, err := st.GetDeploy(ctx, app.ID, expected.ID)
		if err != nil {
			t.Fatal(err)
		}
		if expected.TriggeredBy == subject {
			expected.TriggeredBy = deletion.DeletedMarker
		}
		if !reflect.DeepEqual(got, expected) {
			t.Errorf("deploy history changed beyond departing actor: got %+v, want %+v", got, expected)
		}
	}
}
