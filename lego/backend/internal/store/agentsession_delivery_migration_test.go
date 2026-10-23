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
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestAgentSessionDeliveryMigrationsRoundTrip pins 0159 (delivery_note) and
// 0160 (drop evidence) in both directions on a scratch copy of the 0066 shape:
// a pre-existing row survives, reads an empty note, loses the digest, and the
// down path restores a column an older binary can read and write again.
func TestAgentSessionDeliveryMigrationsRoundTrip(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	defer pool.Close()
	read := func(name string) string {
		t.Helper()
		raw, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			t.Fatal(err)
		}
		return string(raw)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0160_agent_evidence;
		SET LOCAL search_path TO migration_0160_agent_evidence;
		CREATE TABLE agent_sessions (
			id text PRIMARY KEY,
			evidence jsonb NOT NULL DEFAULT '{}'::jsonb
		);
		INSERT INTO agent_sessions (id, evidence) VALUES ('ags-pre', '{"commandLog":["go test"]}');
	`); err != nil {
		t.Fatal(err)
	}
	hasColumn := func(column string) bool {
		t.Helper()
		var exists bool
		if err := tx.QueryRow(ctx, `
			SELECT EXISTS (SELECT 1 FROM information_schema.columns
			WHERE table_schema='migration_0160_agent_evidence' AND table_name='agent_sessions' AND column_name=$1)`, column).Scan(&exists); err != nil {
			t.Fatal(err)
		}
		return exists
	}
	exec := func(tx pgx.Tx, label, sql string) {
		t.Helper()
		if _, err := tx.Exec(ctx, sql); err != nil {
			t.Fatalf("%s: %v", label, err)
		}
	}

	exec(tx, "apply 0159", read("0159_agent_session_delivery_note.up.sql"))
	exec(tx, "apply 0160", read("0160_drop_agent_session_evidence.up.sql"))
	var note string
	if err := tx.QueryRow(ctx, `SELECT delivery_note FROM agent_sessions WHERE id='ags-pre'`).Scan(&note); err != nil || note != "" {
		t.Fatalf("pre-existing row delivery_note = %q err=%v, want the empty default", note, err)
	}
	if hasColumn("evidence") {
		t.Fatal("evidence survived 0160")
	}

	exec(tx, "roll back 0160", read("0160_drop_agent_session_evidence.down.sql"))
	exec(tx, "roll back 0159", read("0159_agent_session_delivery_note.down.sql"))
	if hasColumn("delivery_note") {
		t.Fatal("delivery_note survived the 0159 down migration")
	}
	if !hasColumn("evidence") {
		t.Fatal("0160 down did not restore evidence")
	}
	// An older binary inserts without naming evidence and reads it back as the
	// empty object it treats as "no evidence"; the column is nullable now.
	exec(tx, "old-binary insert", `INSERT INTO agent_sessions (id) VALUES ('ags-old')`)
	var evidence string
	if err := tx.QueryRow(ctx, `SELECT evidence::text FROM agent_sessions WHERE id='ags-old'`).Scan(&evidence); err != nil || evidence != "{}" {
		t.Fatalf("restored evidence default = %q err=%v, want {}", evidence, err)
	}
	var nullable string
	if err := tx.QueryRow(ctx, `
		SELECT is_nullable FROM information_schema.columns
		WHERE table_schema='migration_0160_agent_evidence' AND table_name='agent_sessions' AND column_name='evidence'`).Scan(&nullable); err != nil || nullable != "YES" {
		t.Fatalf("restored evidence is_nullable = %q err=%v, want YES", nullable, err)
	}
}
