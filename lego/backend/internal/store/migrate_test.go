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
	"time"

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

// TestNotificationDeployStartedMigrationRepairsSkippedPrerequisite reproduces
// the production 2026-07-15 failure: an older database had advanced past the
// renumbered 0013 migration without ever creating notification_settings, so
// 0028's ALTER TABLE dirtied the migration and blocked bex-api startup. Execute
// 0028 in an isolated schema with only its tenants prerequisite and require it
// to build the complete final table from scratch.
func TestNotificationDeployStartedMigrationRepairsSkippedPrerequisite(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	sql, err := migrationsFS.ReadFile("migrations/0028_notification_deploy_started.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0028_repair;
		SET LOCAL search_path TO migration_0028_repair;
		CREATE TABLE tenants (id text PRIMARY KEY);
	`); err != nil {
		t.Fatalf("prepare skipped-prerequisite schema: %v", err)
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("migration 0028 against missing notification_settings: %v", err)
	}

	var columns int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM information_schema.columns
		WHERE table_schema = 'migration_0028_repair'
		  AND table_name = 'notification_settings'
		  AND column_name IN ('id', 'tenant_id', 'subject', 'deploy_started',
		                      'deploy_succeeded', 'deploy_failed', 'created_at', 'updated_at')
	`).Scan(&columns); err != nil {
		t.Fatalf("inspect repaired columns: %v", err)
	}
	if columns != 8 {
		t.Errorf("repaired notification_settings has %d expected columns, want 8", columns)
	}

	var indexes int
	if err := tx.QueryRow(ctx, `
		SELECT count(*)
		FROM pg_indexes
		WHERE schemaname = 'migration_0028_repair'
		  AND tablename = 'notification_settings'
		  AND indexname = 'notification_settings_member_idx'
	`).Scan(&indexes); err != nil {
		t.Fatalf("inspect repaired index: %v", err)
	}
	if indexes != 1 {
		t.Errorf("repaired notification_settings member indexes = %d, want 1", indexes)
	}
}

func TestDeployLifecycleMigrationBackfillsOldRows(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	sql, err := migrationsFS.ReadFile("migrations/0033_deploy_lifecycle.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0033_deploy_lifecycle;
		SET LOCAL search_path TO migration_0033_deploy_lifecycle;
		CREATE TABLE deploys (
			id text PRIMARY KEY,
			app_id text NOT NULL,
			status text NOT NULL,
			created_at timestamptz NOT NULL,
			finished_at timestamptz
		);
		INSERT INTO deploys (id, app_id, status, created_at, finished_at) VALUES
			('dep-old-live', 'srv-1', 'live', '2026-07-01T00:00:00Z', '2026-07-01T00:01:00Z'),
			('dep-old-open', 'srv-2', 'update_in_progress', '2026-07-01T00:02:00Z', NULL),
			('dep-new-open', 'srv-2', 'update_in_progress', '2026-07-01T00:03:00Z', NULL);
	`); err != nil {
		t.Fatalf("prepare old deploy schema: %v", err)
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("migration 0031 against old deploy rows: %v", err)
	}

	var liveUpdated, liveFinished, openUpdated, openCreated time.Time
	if err := tx.QueryRow(ctx, `SELECT updated_at, finished_at FROM deploys WHERE id = 'dep-old-live'`).Scan(&liveUpdated, &liveFinished); err != nil {
		t.Fatal(err)
	}
	if !liveUpdated.Equal(liveFinished) {
		t.Errorf("closed old row updated_at=%s, want finished_at=%s", liveUpdated, liveFinished)
	}
	var oldStatus string
	var oldFinished *time.Time
	if err := tx.QueryRow(ctx, `SELECT status, created_at, updated_at, finished_at FROM deploys WHERE id = 'dep-old-open'`).Scan(&oldStatus, &openCreated, &openUpdated, &oldFinished); err != nil {
		t.Fatal(err)
	}
	if oldStatus != DeployCanceled || oldFinished == nil || !openUpdated.Equal(*oldFinished) {
		t.Errorf("superseded old open row = status %q updated %s finished %v, want canceled at one transition timestamp", oldStatus, openUpdated, oldFinished)
	}
	var newestStatus string
	if err := tx.QueryRow(ctx, `SELECT status FROM deploys WHERE id = 'dep-new-open'`).Scan(&newestStatus); err != nil {
		t.Fatal(err)
	}
	if newestStatus != DeployUpdateInProgress {
		t.Errorf("newest old open row status = %q, want update_in_progress", newestStatus)
	}
	if openCreated.IsZero() {
		t.Fatal("old row is no longer readable after migration")
	}
}

func TestEnvironmentAllowListMigrationNormalizesLegacyRows(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	sql, err := migrationsFS.ReadFile("migrations/0053_normalize_environment_allowlists.up.sql")
	if err != nil {
		t.Fatalf("read migration: %v", err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatalf("begin: %v", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0053_environment_allowlists;
		SET LOCAL search_path TO migration_0053_environment_allowlists;
		CREATE TABLE environments (
			id text PRIMARY KEY,
			ip_allow_list jsonb NOT NULL DEFAULT '[]'::jsonb
		);
		INSERT INTO environments (id, ip_allow_list) VALUES
			('env-legacy', '["203.0.113.0/24", {"cidrBlock":"198.51.100.0/24","description":"office"}]'),
			('env-canonical', '[{"cidrBlock":"192.0.2.0/24","description":"vpn"}]');
	`); err != nil {
		t.Fatalf("prepare legacy environment schema: %v", err)
	}
	if _, err := tx.Exec(ctx, string(sql)); err != nil {
		t.Fatalf("migration 0053 against legacy allowlists: %v", err)
	}

	var normalized, canonical string
	if err := tx.QueryRow(ctx, `SELECT ip_allow_list::text FROM environments WHERE id = 'env-legacy'`).Scan(&normalized); err != nil {
		t.Fatal(err)
	}
	if want := `[{"cidrBlock": "203.0.113.0/24", "description": ""}, {"cidrBlock": "198.51.100.0/24", "description": "office"}]`; normalized != want {
		t.Errorf("normalized legacy allowlist = %s, want %s", normalized, want)
	}
	if err := tx.QueryRow(ctx, `SELECT ip_allow_list::text FROM environments WHERE id = 'env-canonical'`).Scan(&canonical); err != nil {
		t.Fatal(err)
	}
	if want := `[{"cidrBlock": "192.0.2.0/24", "description": "vpn"}]`; canonical != want {
		t.Errorf("canonical allowlist changed = %s, want %s", canonical, want)
	}

	if _, err := tx.Exec(ctx, `SAVEPOINT reject_legacy_write`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE environments SET ip_allow_list = '["10.0.0.0/8"]' WHERE id = 'env-legacy'`); err == nil {
		t.Fatal("migration constraint accepted a legacy bare-CIDR write")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT reject_legacy_write`); err != nil {
		t.Fatal(err)
	}
}

// TestPaymentMethodBoundMigrationAppliesAndRollsBack verifies the isolated
// ADR046 schema change in both directions against real Postgres.
func TestPaymentMethodBoundMigrationAppliesAndRollsBack(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()

	up, err := migrationsFS.ReadFile("migrations/0058_payment_method_bound.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0058_payment_method_bound.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0058_payment_marker;
		SET LOCAL search_path TO migration_0058_payment_marker;
		CREATE TABLE billing_provider_mappings (workspace_id text PRIMARY KEY);
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply migration 0058: %v", err)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='migration_0058_payment_marker'
			  AND table_name='billing_provider_mappings'
			  AND column_name='payment_method_bound_at'
			  AND data_type='timestamp with time zone'
		)`).Scan(&exists); err != nil || !exists {
		t.Fatalf("column after up exists=%v err=%v", exists, err)
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("roll back migration 0058: %v", err)
	}
	if err := tx.QueryRow(ctx, `
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema='migration_0058_payment_marker'
			  AND table_name='billing_provider_mappings'
			  AND column_name='payment_method_bound_at'
		)`).Scan(&exists); err != nil || exists {
		t.Fatalf("column after down exists=%v err=%v", exists, err)
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

func TestBlueprintPathDefaultMigrationUsesCanonicalRenderFilename(t *testing.T) {
	up, err := migrationsFS.ReadFile("migrations/0067_blueprint_render_yaml_default.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(up), "ALTER COLUMN path SET DEFAULT 'render.yaml'") {
		t.Fatalf("Blueprint path migration = %q, want render.yaml default", up)
	}
	down, err := migrationsFS.ReadFile("migrations/0067_blueprint_render_yaml_default.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(down), "ALTER COLUMN path SET DEFAULT 'bex.yml'") {
		t.Fatalf("Blueprint path down migration = %q, want legacy default restoration", down)
	}
}
