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
	"slices"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestWebhookAttemptMigrationBackfillsAndGuardsEvidence(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	up, err := migrationsFS.ReadFile("migrations/0084_webhook_attempts.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0084_webhook_attempts.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0084_webhook_attempts;
		SET LOCAL search_path TO migration_0084_webhook_attempts;
		CREATE TABLE webhook_endpoints (id text PRIMARY KEY);
		CREATE TABLE webhook_deliveries (
			id text PRIMARY KEY,
			endpoint_id text NOT NULL REFERENCES webhook_endpoints(id) ON DELETE CASCADE,
			event_id text NOT NULL,
			event_type text NOT NULL,
			service_id text NOT NULL DEFAULT '',
			payload text NOT NULL,
			attempt_count integer NOT NULL DEFAULT 0,
			last_status integer NOT NULL DEFAULT 0,
			last_error text NOT NULL DEFAULT '',
			response_body text NOT NULL DEFAULT '',
			next_attempt_at timestamptz NOT NULL,
			sent_at timestamptz,
			last_attempted_at timestamptz,
			delivered_at timestamptz,
			failed_at timestamptz,
			created_at timestamptz NOT NULL DEFAULT now(),
			UNIQUE (endpoint_id, event_id)
		);
		INSERT INTO webhook_endpoints VALUES ('whk-a'), ('whk-b');
	`); err != nil {
		t.Fatal(err)
	}
	longError := strings.Repeat("界", 700)
	longResponse := strings.Repeat("界", 1400)
	if _, err := tx.Exec(ctx, `
		INSERT INTO webhook_deliveries (
			id, endpoint_id, event_id, event_type, payload, attempt_count,
			last_status, last_error, response_body, next_attempt_at,
			sent_at, last_attempted_at, delivered_at, created_at
		) VALUES
			('whd-00000000000000000001', 'whk-a', 'evt-open', 'deploy_started', '{}', 1,
			 503, $1, $2, '2026-08-17T13:00:00Z',
			 '2026-08-17T12:00:00Z', '2026-08-17T12:00:00Z', NULL, '2026-08-17T11:59:00Z'),
			('whd-00000000000000000002', 'whk-a', 'evt-new', 'deploy_started', '{}', 0,
			 0, '', '', '2026-08-17T13:01:00Z',
			 NULL, NULL, NULL, '2026-08-17T12:01:00Z'),
			('whd-00000000000000000003', 'whk-a', 'evt-done', 'deploy_started', '{}', 2,
			 204, '', 'ok', '2026-08-17T12:02:00Z',
			 '2026-08-17T12:01:00Z', '2026-08-17T12:02:00Z', '2026-08-17T12:02:00Z', '2026-08-17T12:00:00Z')
	`, longError, longResponse); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply up: %v", err)
	}
	expectDBError := func(name, query, code string) {
		t.Helper()
		if _, err := tx.Exec(ctx, `SAVEPOINT expected_webhook_attempt_error`); err != nil {
			t.Fatal(err)
		}
		_, gotErr := tx.Exec(ctx, query)
		var pgErr *pgconn.PgError
		if gotErr == nil || !errors.As(gotErr, &pgErr) || pgErr.Code != code {
			t.Errorf("%s = %v, want PostgreSQL %s", name, gotErr, code)
		}
		if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT expected_webhook_attempt_error`); err != nil {
			t.Fatal(err)
		}
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT expected_webhook_attempt_error`); err != nil {
			t.Fatal(err)
		}
	}

	var attempts int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM webhook_delivery_attempts`).Scan(&attempts); err != nil || attempts != 4 {
		t.Fatalf("backfilled attempts = %d (err %v), want evidence+reservation rows = 4", attempts, err)
	}
	derived := ids.Derive(ids.WebhookDelivery, "backfill:whd-00000000000000000001:2")
	var pendingID string
	if err := tx.QueryRow(ctx, `
		SELECT id FROM webhook_delivery_attempts
		WHERE notification_id='whd-00000000000000000001' AND status='pending'
	`).Scan(&pendingID); err != nil || pendingID != derived {
		t.Fatalf("deterministic pending id = %q (err %v), want %q", pendingID, err, derived)
	}
	// The open legacy row was already due before migration and may have been
	// claimed by an old replica. Its backfilled child still receives the full
	// conservative overlap lease; the new claim predicate cannot take it yet.
	var overlapLeaseFuture bool
	if err := tx.QueryRow(ctx, `
		SELECT lease_until > clock_timestamp()
		FROM webhook_delivery_attempts WHERE id=$1
	`, pendingID).Scan(&overlapLeaseFuture); err != nil || !overlapLeaseFuture {
		t.Fatalf("rolling overlap lease future = %v, %v", overlapLeaseFuture, err)
	}
	var overlapClaims int
	if err := tx.QueryRow(ctx, `
		WITH due AS (
			SELECT id FROM webhook_delivery_attempts
			WHERE id=$1 AND available_at <= clock_timestamp()
			  AND (lease_until IS NULL OR lease_until <= clock_timestamp())
			FOR UPDATE SKIP LOCKED
		), claimed AS (
			UPDATE webhook_delivery_attempts a SET lease_until=clock_timestamp()+interval '2 minutes'
			FROM due WHERE a.id=due.id RETURNING a.id
		)
		SELECT count(*) FROM claimed
	`, pendingID).Scan(&overlapClaims); err != nil || overlapClaims != 0 {
		t.Fatalf("new claim during rolling lease = %d, %v", overlapClaims, err)
	}
	var errorBytes, responseBytes int
	if err := tx.QueryRow(ctx, `
		SELECT octet_length(transport_error), octet_length(response_body)
		FROM webhook_delivery_attempts
		WHERE id='whd-00000000000000000001'
	`).Scan(&errorBytes, &responseBytes); err != nil || errorBytes > 2048 || responseBytes > 4096 {
		t.Fatalf("bounded UTF-8 evidence = (%d,%d), err %v", errorBytes, responseBytes, err)
	}

	// A denormalized endpoint can never disagree with its parent notification.
	expectDBError("cross-endpoint child insert", `
		INSERT INTO webhook_delivery_attempts (
			id, notification_id, endpoint_id, attempt_number, available_at
		) VALUES (
			'whd-00000000000000000004', 'whd-00000000000000000002', 'whk-b', 2, now()
		)
	`, "23503")

	// Pending lease mutation is allowed; identity mutation and every terminal
	// mutation are rejected by the database, not only by Go call discipline.
	if _, err := tx.Exec(ctx, `
		UPDATE webhook_delivery_attempts SET lease_until=now()
		WHERE id='whd-00000000000000000002'
	`); err != nil {
		t.Fatalf("pending lease update: %v", err)
	}
	expectDBError("pending reservation identity update", `
		UPDATE webhook_delivery_attempts SET available_at=available_at + interval '1 second'
		WHERE id='whd-00000000000000000002'
	`, "23514")
	expectDBError("terminal evidence update", `
		UPDATE webhook_delivery_attempts SET response_body='rewritten'
		WHERE id='whd-00000000000000000003'
	`, "23514")

	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("apply lossy down: %v", err)
	}
	var childExists, compositeExists bool
	if err := tx.QueryRow(ctx, `
		SELECT to_regclass('webhook_delivery_attempts') IS NOT NULL,
		       EXISTS (
		           SELECT 1 FROM pg_constraint
		           WHERE conname='webhook_deliveries_id_endpoint_uniq'
		             AND conrelid='webhook_deliveries'::regclass
		       )
	`).Scan(&childExists, &compositeExists); err != nil || childExists || compositeExists {
		t.Fatalf("down boundary = child:%v composite:%v err:%v", childExists, compositeExists, err)
	}
}

// TestDiskEventTypeRenameMigration rewrites live webhook subscription filters
// from the pre-w8/m34 bex spellings to Render's disk_created/disk_deleted, leaves
// already-canonical filters alone, and is idempotent on re-run. Immutable
// delivery payloads are out of scope for this migration (and for this fixture).
func TestDiskEventTypeRenameMigration(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		t.Skip("BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()

	up, err := migrationsFS.ReadFile("migrations/0106_disk_event_type_rename.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0106_disk_event_rename;
		SET LOCAL search_path TO migration_0106_disk_event_rename;
		CREATE TABLE webhook_endpoints (
			id text PRIMARY KEY,
			event_types text[] NOT NULL
		);
		INSERT INTO webhook_endpoints (id, event_types) VALUES
			('whk-legacy', ARRAY['disk_attached','deploy_started','disk_detached']),
			('whk-mixed', ARRAY['disk_created','disk_attached']),
			('whk-clean', ARRAY['disk_created','disk_deleted']),
			('whk-other', ARRAY['deploy_ended']);
	`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply up: %v", err)
	}
	// Re-run: must be a no-op (idempotent).
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("re-apply up: %v", err)
	}

	rows, err := tx.Query(ctx, `SELECT id, event_types FROM webhook_endpoints ORDER BY id`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	got := map[string][]string{}
	for rows.Next() {
		var id string
		var types []string
		if err := rows.Scan(&id, &types); err != nil {
			t.Fatal(err)
		}
		got[id] = types
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	want := map[string][]string{
		"whk-clean":  {"disk_created", "disk_deleted"},
		"whk-legacy": {"disk_created", "deploy_started", "disk_deleted"},
		"whk-mixed":  {"disk_created"},
		"whk-other":  {"deploy_ended"},
	}
	for id, wantTypes := range want {
		if !slices.Equal(got[id], wantTypes) {
			t.Errorf("%s event_types = %v, want %v", id, got[id], wantTypes)
		}
	}
}
