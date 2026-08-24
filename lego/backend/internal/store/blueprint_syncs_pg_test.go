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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// w6/m50 t001/t007: a failed sync used to compute a real applyErr and then
// discard it before UpdateBlueprintSync, leaving every error-state row with
// no forensic trail. This proves the store round-trips a non-empty
// error_message for a failed run and leaves it null for a successful one.
func TestPGStoreBlueprintSyncErrorMessage(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	if err := Migrate(uri); err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	st := NewPGStore(pool)
	stamp := fmt.Sprintf("%d", time.Now().UnixNano())
	owner := "bpsync-owner-" + stamp
	tenant, err := st.CreateWorkspace(ctx, "bpsync-test-"+stamp, PlanHobby, owner)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })

	bp, err := st.UpsertBlueprint(ctx, Blueprint{
		TenantID: tenant.ID, Name: "bpsync", Repo: "example/repo", Branch: "main",
		Manifest: "services: []", Status: BlueprintStatusCreated,
	})
	if err != nil {
		t.Fatal(err)
	}

	// A failed run persists a non-empty error_message.
	failed, err := st.InsertBlueprintSync(ctx, BlueprintSync{
		BlueprintID: bp.ID, CommitID: "deadbeef", State: BlueprintSyncStateRunning, StartedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	completedAt := time.Now().UTC()
	errMsg := "quota exceeded: workspace at service limit"
	updated, err := st.UpdateBlueprintSync(ctx, failed.ID, BlueprintSyncStateError, &completedAt, &errMsg)
	if err != nil {
		t.Fatal(err)
	}
	if updated.ErrorMessage == nil || *updated.ErrorMessage != errMsg {
		t.Fatalf("UpdateBlueprintSync(error) ErrorMessage = %v, want %q", updated.ErrorMessage, errMsg)
	}

	// A successful run leaves error_message null.
	success, err := st.InsertBlueprintSync(ctx, BlueprintSync{
		BlueprintID: bp.ID, CommitID: "cafef00d", State: BlueprintSyncStateRunning, StartedAt: time.Now().UTC(),
	})
	if err != nil {
		t.Fatal(err)
	}
	updatedOK, err := st.UpdateBlueprintSync(ctx, success.ID, BlueprintSyncStateSuccess, &completedAt, nil)
	if err != nil {
		t.Fatal(err)
	}
	if updatedOK.ErrorMessage != nil {
		t.Fatalf("UpdateBlueprintSync(success) ErrorMessage = %v, want nil", *updatedOK.ErrorMessage)
	}

	// The list read path (what REST/GraphQL/MCP consume) carries the same field.
	runs, err := st.ListBlueprintSyncs(ctx, bp.ID, "", 0)
	if err != nil {
		t.Fatal(err)
	}
	var sawFailed, sawSuccess bool
	for _, r := range runs {
		switch r.ID {
		case failed.ID:
			sawFailed = true
			if r.ErrorMessage == nil || *r.ErrorMessage != errMsg {
				t.Errorf("ListBlueprintSyncs failed row ErrorMessage = %v, want %q", r.ErrorMessage, errMsg)
			}
		case success.ID:
			sawSuccess = true
			if r.ErrorMessage != nil {
				t.Errorf("ListBlueprintSyncs success row ErrorMessage = %v, want nil", *r.ErrorMessage)
			}
		}
	}
	if !sawFailed || !sawSuccess {
		t.Fatalf("ListBlueprintSyncs missing rows: sawFailed=%v sawSuccess=%v (%d runs)", sawFailed, sawSuccess, len(runs))
	}
}
