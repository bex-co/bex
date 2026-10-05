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
	"errors"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// reasonCodes0138 is the reason vocabulary migration 0138 admits. Later
// migrations widen it; this test pins 0138's own contract, not the current set.
var reasonCodes0138 = []string{
	"", EventReasonImagePullBackoff, EventReasonReadinessFailed, EventReasonRootDirectory,
	EventReasonBuildFilter, EventReasonSkipPhrase, EventReasonSuperseded,
}

func TestSupersededReasonMigrationPreservesVocabularyAndFacts(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	migration := func(path string) {
		t.Helper()
		sql, err := migrationsFS.ReadFile("migrations/" + path)
		if err != nil {
			t.Fatal(err)
		}
		exec(string(sql))
	}
	exec(`CREATE SCHEMA migration_0138_event_reason;
 SET LOCAL search_path TO migration_0138_event_reason;
 CREATE TABLE apps (id TEXT PRIMARY KEY);
 INSERT INTO apps(id) VALUES ('app');`)
	migration("0043_service_event_facts.up.sql")
	insert := func(source, reason string) error {
		_, err := tx.Exec(ctx, `INSERT INTO service_event_facts(source_key,app_id,fact_type,at,reason_code) VALUES ($1,'app','server_failed',now(),$2)`, source, reason)
		return err
	}
	rejected := func(reason string) {
		t.Helper()
		exec("SAVEPOINT rejected_reason")
		err := insert("rejected", reason)
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "service_event_facts_reason_code_check" {
			t.Fatalf("reason %q rejection=%v, want reason CHECK", reason, err)
		}
		exec("ROLLBACK TO SAVEPOINT rejected_reason")
	}
	for _, reason := range reasonCodes0138 {
		if reason == EventReasonSuperseded {
			continue
		}
		if err := insert("old-"+reason, reason); err != nil {
			t.Fatalf("old vocabulary %q: %v", reason, err)
		}
	}
	rejected(EventReasonSuperseded)
	migration("0138_service_event_superseded_reason.up.sql")
	for _, reason := range reasonCodes0138 {
		if err := insert("new-"+reason, reason); err != nil {
			t.Fatalf("migrated vocabulary %q: %v", reason, err)
		}
	}
	rejected("untyped-reason")
	migration("0138_service_event_superseded_reason.down.sql")
	var count int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM service_event_facts`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if want := 2*len(reasonCodes0138) - 1; count != want {
		t.Fatalf("rollback retained %d facts, want %d", count, want)
	}
	var reason string
	if err := tx.QueryRow(ctx, `SELECT reason_code FROM service_event_facts WHERE source_key=$1`, "new-"+EventReasonSuperseded).Scan(&reason); err != nil || reason != EventReasonSuperseded {
		t.Fatalf("rollback erased superseded attribution: %q %v", reason, err)
	}
	if err := insert("old-binary-after-down", EventReasonSuperseded); err != nil {
		t.Fatalf("rollback rejected existing producer vocabulary: %v", err)
	}
	rejected("untyped-reason")
	// Re-upgrading after the non-lossy rollback remains safe with retained rows.
	migration("0138_service_event_superseded_reason.up.sql")
}

// w4/m114: migration 0140 admits the cron-run failure reasons and adds the
// exit_code / run_id columns. Only the reason CHECK and the new columns are
// under test, so the fixture uses the 0043 fact type the 0138 test uses. Its rollback is non-lossy, so a fact written with
// a cron reason survives a downgrade and the re-upgrade is safe.
func TestCronRunReasonMigrationAdmitsFailureReasons(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := t.Context()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	exec := func(sql string) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	migration := func(path string) {
		t.Helper()
		sql, err := migrationsFS.ReadFile("migrations/" + path)
		if err != nil {
			t.Fatal(err)
		}
		exec(string(sql))
	}
	exec(`CREATE SCHEMA migration_0140_cron_reason;
 SET LOCAL search_path TO migration_0140_cron_reason;
 CREATE TABLE apps (id TEXT PRIMARY KEY);
 INSERT INTO apps(id) VALUES ('app');`)
	migration("0043_service_event_facts.up.sql")
	migration("0138_service_event_superseded_reason.up.sql")
	exec("SAVEPOINT before_0140")
	if _, err := tx.Exec(ctx, `INSERT INTO service_event_facts(source_key,app_id,fact_type,at,reason_code) VALUES ('early','app','server_failed',now(),'non_zero_exit')`); err == nil {
		t.Fatal("0138 admitted a cron failure reason; this test no longer proves 0140 widens the check")
	}
	exec("ROLLBACK TO SAVEPOINT before_0140")

	migration("0140_cron_run_failure_reason.up.sql")
	for _, reason := range []string{EventReasonNonZeroExit, EventReasonOOMKilled, EventReasonEvicted, EventReasonTimedOut} {
		if _, err := tx.Exec(ctx, `INSERT INTO service_event_facts(source_key,app_id,fact_type,at,reason_code,exit_code,run_id)
VALUES ($1,'app','server_failed',now(),$2,3,'crr-run')`, "cron-"+reason, reason); err != nil {
			t.Fatalf("0140 rejected %q: %v", reason, err)
		}
	}
	migration("0140_cron_run_failure_reason.down.sql")
	var exit int32
	var runID, reason string
	if err := tx.QueryRow(ctx, `SELECT exit_code, run_id, reason_code FROM service_event_facts WHERE source_key='cron-non_zero_exit'`).Scan(&exit, &runID, &reason); err != nil ||
		exit != 3 || runID != "crr-run" || reason != EventReasonNonZeroExit {
		t.Fatalf("rollback lost the cron failure fact: %d %q %q %v", exit, runID, reason, err)
	}
	migration("0140_cron_run_failure_reason.up.sql")
}
