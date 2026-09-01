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

func TestWorkspaceCreationPG(t *testing.T) {
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
	if _, err := pool.Exec(ctx, `TRUNCATE workspace_creation_attempts, account_deletions, tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	st := NewPGStore(pool)

	attempt, err := st.CreateWorkspaceCreationAttempt(
		ctx, "identity-a", "new-workspace", PlanPro, "billing@example.com", true, time.Now().Add(time.Hour),
	)
	if err != nil {
		t.Fatal(err)
	}
	visible, err := st.ListTenantsForSubject(ctx, "identity-a")
	if err != nil || len(visible) != 0 {
		t.Fatalf("provisional attempt visible as tenant: tenants=%v err=%v", visible, err)
	}
	if _, err := st.GetWorkspaceCreationAttempt(ctx, attempt.ID, "identity-b"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("foreign attempt read = %v, want not found", err)
	}

	attempt, err = st.SetWorkspaceCreationSetup(ctx, attempt.ID, "identity-a", "cus_test", "seti_test", false)
	if err != nil {
		t.Fatal(err)
	}
	attempt, err = st.MarkWorkspaceCreationSetupSucceeded(ctx, attempt.ID, "identity-a", "pm_test")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SetWorkspaceCreationSubscription(ctx, attempt.ID, "identity-a", "sub_test"); err != nil {
		t.Fatal(err)
	}
	boundAt := time.Now().UTC().Truncate(time.Microsecond)
	tenant, err := st.FinalizeWorkspaceCreation(ctx, attempt.ID, "identity-a", boundAt)
	if err != nil {
		t.Fatal(err)
	}
	replayed, err := st.FinalizeWorkspaceCreation(ctx, attempt.ID, "identity-a", boundAt)
	if err != nil || replayed.ID != tenant.ID {
		t.Fatalf("finalize replay = %+v err=%v", replayed, err)
	}

	var email, customer, subscription string
	var marker time.Time
	if err := pool.QueryRow(ctx, `
		SELECT t.billing_email, m.customer_id, m.subscription_id, m.payment_method_bound_at
		FROM tenants t JOIN billing_provider_mappings m ON m.workspace_id=t.id
		WHERE t.id=$1`, tenant.ID).Scan(&email, &customer, &subscription, &marker); err != nil {
		t.Fatal(err)
	}
	if email != "billing@example.com" || customer != "cus_test" || subscription != "sub_test" || !marker.Equal(boundAt) {
		t.Fatalf("final billing state email=%q customer=%q subscription=%q marker=%v", email, customer, subscription, marker)
	}
	var role string
	if err := pool.QueryRow(ctx, `SELECT role FROM tenant_members WHERE tenant_id=$1 AND subject='identity-a'`, tenant.ID).Scan(&role); err != nil {
		t.Fatal(err)
	}
	if role != "admin" {
		t.Fatalf("owner role = %q", role)
	}

	abandoned, err := st.CreateWorkspaceCreationAttempt(
		ctx, "identity-a", "abandoned", PlanHobby, "billing@example.com", false, time.Now().Add(-time.Minute),
	)
	if err != nil {
		t.Fatal(err)
	}
	claimed, err := st.ExpireWorkspaceCreationAttempts(ctx, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].ID != abandoned.ID || claimed[0].State != WorkspaceCreationCleanupPending {
		t.Fatalf("cleanup claim = %+v", claimed)
	}
	if err := st.FinishWorkspaceCreationCleanup(ctx, abandoned.ID, true); err != nil {
		t.Fatal(err)
	}
}
