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
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// TestOwnershipErrorNoMisowned confirms CheckOwnership returns nil when all
// tables are owned by the application role (the happy path).
func TestOwnershipErrorNoMisowned(t *testing.T) {
	if err := ownershipError(nil); err != nil {
		t.Errorf("ownershipError(nil) = %v, want nil", err)
	}
	if err := ownershipError([]string{}); err != nil {
		t.Errorf("ownershipError([]) = %v, want nil", err)
	}
}

// TestOwnershipErrorDetectsMisowned confirms CheckOwnership returns a
// descriptive error listing mis-owned tables — the failure mode of the
// 2026-07-12 incident where tenant_invites was owned by postgres instead of
// bex, causing "permission denied for table tenant_invites" on every
// invite-redemption call.
func TestOwnershipErrorDetectsMisowned(t *testing.T) {
	err := ownershipError([]string{
		"tenant_invites (owner: postgres)",
		"tenants (owner: postgres)",
	})
	if err == nil {
		t.Fatal("ownershipError with mis-owned tables returned nil, want error")
	}
	msg := err.Error()
	for _, want := range []string{"tenant_invites", "tenants", "0013", "OWNER TO"} {
		if !strings.Contains(msg, want) {
			t.Errorf("error message missing %q: %s", want, msg)
		}
	}
}

// TestCheckOwnershipAfterMigration is an integration test (skipped without
// BEX_TEST_DB_URI) that verifies CheckOwnership returns nil after the
// embedded migrations run — confirming every public table is owned by the
// application role and the SQL query itself is valid.
func TestCheckOwnershipAfterMigration(t *testing.T) {
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
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	if err := CheckOwnership(ctx, pool); err != nil {
		t.Errorf("CheckOwnership after clean migration: %v", err)
	}
}
