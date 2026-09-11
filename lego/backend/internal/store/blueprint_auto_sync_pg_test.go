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
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPGStoreBlueprintAutoSyncIntents covers w8/m38 durable enqueue, UNIQUE
// dedupe, claim, and complete/fail against real Postgres when available.
func TestPGStoreBlueprintAutoSyncIntents(t *testing.T) {
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
	tenant, err := st.CreateWorkspace(ctx, "asi-test-"+stamp, PlanHobby, "asi-owner-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })

	bp, err := st.UpsertBlueprint(ctx, Blueprint{
		TenantID: tenant.ID, Name: "asi", Repo: "https://github.com/acme/asi",
		Branch: "main", Path: "render.yaml", AutoSync: true,
		Manifest: "services: []", Status: BlueprintStatusInSync,
	})
	if err != nil {
		t.Fatal(err)
	}

	listed, err := st.ListAutoSyncBlueprints(ctx, "main", tenant.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 || listed[0].ID != bp.ID {
		t.Fatalf("ListAutoSyncBlueprints = %+v, want [%s]", listed, bp.ID)
	}
	foreign, err := st.ListAutoSyncBlueprints(ctx, "main", "tea-missing")
	if err != nil {
		t.Fatal(err)
	}
	if len(foreign) != 0 {
		t.Fatalf("scoped list returned %d rows", len(foreign))
	}

	intent := BlueprintAutoSyncIntent{
		TenantID: tenant.ID, BlueprintID: bp.ID,
		DeliveryDigest: "digest-" + stamp,
		CommitSHA:      strings.Repeat("a", 40),
		Path:           "render.yaml",
	}
	inserted, err := st.EnqueueBlueprintAutoSyncIntent(ctx, intent)
	if err != nil || !inserted {
		t.Fatalf("enqueue: inserted=%v err=%v", inserted, err)
	}
	inserted, err = st.EnqueueBlueprintAutoSyncIntent(ctx, intent)
	if err != nil || inserted {
		t.Fatalf("dedupe: inserted=%v err=%v, want false", inserted, err)
	}

	claimed, err := st.ClaimBlueprintAutoSyncIntents(ctx, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(claimed) != 1 || claimed[0].BlueprintID != bp.ID {
		t.Fatalf("claim = %+v", claimed)
	}
	if err := st.CompleteBlueprintAutoSyncIntent(ctx, claimed[0].ID); err != nil {
		t.Fatal(err)
	}
	again, err := st.ClaimBlueprintAutoSyncIntents(ctx, 10, time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Fatalf("claimed completed intent: %+v", again)
	}
}
