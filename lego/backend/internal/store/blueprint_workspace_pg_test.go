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
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestPGBlueprintWorkspace pins w4/m169's routing read: a live Blueprint id
// resolves to its owning workspace with no tenant supplied, and a missing or
// disconnected id is ErrNotFound.
func TestPGBlueprintWorkspace(t *testing.T) {
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
	tenant, err := st.CreateWorkspace(ctx, "bpws-"+stamp, PlanHobby, "bpws-owner-"+stamp)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.DeleteTenant(context.Background(), tenant.ID) })
	bp, err := st.UpsertBlueprint(ctx, Blueprint{
		TenantID: tenant.ID, Name: "bpws", Repo: "https://github.com/acme/bpws-" + stamp,
		Branch: "main", Path: "render.yaml", Manifest: "services: []", Status: BlueprintStatusInSync,
	})
	if err != nil {
		t.Fatal(err)
	}

	if got, err := st.BlueprintWorkspace(ctx, bp.ID); err != nil || got != tenant.ID {
		t.Fatalf("BlueprintWorkspace = (%q, %v), want %q", got, err, tenant.ID)
	}
	if _, err := st.BlueprintWorkspace(ctx, "blp-missing-"+stamp); !errors.Is(err, ErrNotFound) {
		t.Fatalf("missing id = %v, want ErrNotFound", err)
	}
	if err := st.DisconnectBlueprint(ctx, bp.ID, tenant.ID); err != nil {
		t.Fatal(err)
	}
	if _, err := st.BlueprintWorkspace(ctx, bp.ID); !errors.Is(err, ErrNotFound) {
		t.Fatalf("disconnected id = %v, want ErrNotFound", err)
	}
}
