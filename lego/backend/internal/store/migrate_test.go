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
	"regexp"
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

// TestMigrationNumbersAreUnique guards against a bug class that has bitten
// this migrations directory repeatedly: golang-migrate keys a migration off
// its leading NNNN_ number, so two files sharing one is at best a refused
// Migrate() (a duplicate-version error) and at worst — after a manual
// renumber — a SILENTLY skipped migration on a database whose
// schema_migrations version is already past it (see 0012_audit_target.up.sql's
// "HAZARD WORTH KNOWING" comment: two earlier collision-fixes renumbered
// migrations downward, and this exact class of collision has recurred on
// concurrent, independently-authored feature branches converging onto main —
// most recently 0013 (fix_ownership vs notification_settings) and 0014
// (projects vs registry_credentials), each pair authored minutes apart by
// separate work, neither aware of the other). This test is DB-less (a plain
// directory listing), so it runs on every `go test ./...`, not just the
// opt-in BEX_TEST_DB_URI path — the two prior incidents both shipped through
// review because nothing checked for the collision until a live database hit it.
func TestMigrationNumbersAreUnique(t *testing.T) {
	entries, err := migrationsFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}
	numberRE := regexp.MustCompile(`^(\d+)_`)
	seen := map[string]string{} // version -> the first filename that claimed it
	for _, e := range entries {
		// Each migration is a .up/.down PAIR sharing one version by design —
		// count the version once per migration, not once per file.
		if !strings.HasSuffix(e.Name(), ".up.sql") {
			continue
		}
		m := numberRE.FindStringSubmatch(e.Name())
		if m == nil {
			t.Errorf("migration file %q doesn't start with a NNNN_ version prefix", e.Name())
			continue
		}
		version := m[1]
		if first, dup := seen[version]; dup {
			t.Errorf("migration version %s is used by both %q and %q — renumber one (see 0012_audit_target.up.sql for why a silent skip is worse than this test failing)", version, first, e.Name())
			continue
		}
		seen[version] = e.Name()
	}
	if len(seen) == 0 {
		t.Fatal("no migration files found — the embed pattern or directory is broken")
	}
}
