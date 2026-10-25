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
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestRetireAgentSessionHibernationMigration pins 0162 (w10/m15) on a scratch
// copy of the 0073 shape: every legacy hibernation row lands in the steerable
// completed/failed phase with no sandbox, a rehydrate orphaned in `resuming`
// (the ags-dbdh96fsf6rc738q771g shape) is failed with its never-run turn
// settled, a live plain Resume and ordinary rows are untouched, updated_at is
// kept, and the down path restores columns and phases an older binary reads.
func TestRetireAgentSessionHibernationMigration(t *testing.T) {
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
	const schema = "migration_0162_retire_hibernation"
	updatedAt := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA `+schema+`;
		SET LOCAL search_path TO `+schema+`;
		CREATE TABLE agent_sessions (
			id                  text PRIMARY KEY,
			sandbox_id          text NOT NULL DEFAULT '',
			phase               text NOT NULL,
			status              text NOT NULL DEFAULT '',
			turns               int  NOT NULL DEFAULT 1,
			failure_reason      text NOT NULL DEFAULT '',
			failure_reason_code text NOT NULL DEFAULT '',
			updated_at          timestamptz NOT NULL,
			pinned              boolean NOT NULL DEFAULT false,
			snapshot_ref        text NOT NULL DEFAULT '',
			snapshot_bytes      bigint NOT NULL DEFAULT 0,
			snapshot_sha        text NOT NULL DEFAULT '',
			hibernated_at       timestamptz,
			retain_until        timestamptz,
			CONSTRAINT agent_sessions_phase_check CHECK (phase IN ('creating', 'running', 'resuming', 'redispatching',
				'hibernating', 'hibernated', 'completed', 'failed', 'canceling', 'canceled'))
		);
		CREATE INDEX agent_sessions_hibernated_idx ON agent_sessions (retain_until)
			WHERE phase = 'hibernated' AND snapshot_ref <> '';
		CREATE TABLE agent_session_turns (
			session_id   text NOT NULL REFERENCES agent_sessions(id),
			turn         int  NOT NULL,
			completed_at timestamptz,
			PRIMARY KEY (session_id, turn)
		);
	`); err != nil {
		t.Fatalf("prepare pre-0162 schema: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO agent_sessions (id, sandbox_id, phase, status, turns, failure_reason, updated_at, pinned, snapshot_ref, snapshot_bytes, hibernated_at, retain_until) VALUES
			('ags-hib',      '',      'hibernated',  'hibernated',  1, '',                 $1, true,  'agent-snapshots/a.tgz', 4096, $1, NULL),
			('ags-hib-fail', '',      'hibernated',  'hibernated',  1, 'agent turn failed', $1, false, 'agent-snapshots/b.tgz', 10,   $1, $1),
			('ags-claiming', 'sbx-c', 'hibernating', 'hibernating', 1, '',                 $1, false, '',                      0,    NULL, NULL),
			('ags-orphan',   '',      'resuming',    'rehydrating', 2, '',                 $1, false, 'agent-snapshots/d.tgz', 99,   $1, $1),
			('ags-resume',   'sbx-e', 'resuming',    'resuming',    1, '',                 $1, false, '',                      0,    NULL, NULL),
			('ags-done',     'sbx-f', 'completed',   'completed',   1, '',                 $1, false, '',                      0,    NULL, NULL)`, updatedAt); err != nil {
		t.Fatalf("prepare pre-0162 sessions: %v", err)
	}
	if _, err := tx.Exec(ctx, `
		INSERT INTO agent_session_turns (session_id, turn, completed_at) VALUES
			('ags-hib', 1, $1), ('ags-hib-fail', 1, $1), ('ags-claiming', 1, $1),
			('ags-orphan', 1, $1), ('ags-orphan', 2, NULL),
			('ags-resume', 1, $1), ('ags-done', 1, $1)`, updatedAt); err != nil {
		t.Fatalf("prepare pre-0162 turns: %v", err)
	}
	if _, err := tx.Exec(ctx, read("0162_retire_agent_session_hibernation.up.sql")); err != nil {
		t.Fatalf("apply migration 0162: %v", err)
	}

	want := map[string]struct{ phase, sandbox, reason string }{
		"ags-hib":      {"completed", "", ""},
		"ags-hib-fail": {"failed", "", "agent turn failed"},
		"ags-claiming": {"completed", "", ""},
		"ags-orphan":   {"failed", "", "the sandbox restore was interrupted; send a message to continue in a fresh sandbox"},
		"ags-resume":   {"resuming", "sbx-e", ""},
		"ags-done":     {"completed", "sbx-f", ""},
	}
	for id, w := range want {
		var phase, status, sandbox, reason string
		var updated time.Time
		if err := tx.QueryRow(ctx, `SELECT phase, status, sandbox_id, failure_reason, updated_at FROM agent_sessions WHERE id = $1`, id).
			Scan(&phase, &status, &sandbox, &reason, &updated); err != nil {
			t.Fatal(err)
		}
		if phase != w.phase || sandbox != w.sandbox || reason != w.reason {
			t.Errorf("%s = phase %q sandbox %q reason %q, want %+v", id, phase, sandbox, reason, w)
		}
		if id != "ags-resume" && status != phase {
			t.Errorf("%s status = %q, want it to match phase %q", id, status, phase)
		}
		if !updated.Equal(updatedAt) {
			t.Errorf("%s updated_at moved to %v; the push worker would re-announce it", id, updated)
		}
	}
	var unsettled int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM agent_session_turns WHERE completed_at IS NULL`).Scan(&unsettled); err != nil {
		t.Fatal(err)
	}
	if unsettled != 0 {
		t.Errorf("%d turns left unsettled; the orphan's never-run turn must be settled", unsettled)
	}
	countColumns := func() int {
		t.Helper()
		var n int
		if err := tx.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
			WHERE table_schema = $1 AND table_name = 'agent_sessions'
			  AND column_name IN ('pinned','snapshot_ref','snapshot_bytes','snapshot_sha','hibernated_at','retain_until')`, schema).Scan(&n); err != nil {
			t.Fatal(err)
		}
		return n
	}
	if n := countColumns(); n != 0 {
		t.Errorf("%d hibernation columns survived the up migration", n)
	}
	if _, err := tx.Exec(ctx, `SAVEPOINT phase_check`); err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET phase = 'hibernated' WHERE id = 'ags-done'`); err == nil {
		t.Error("the phase check still admits 'hibernated'")
	}
	if _, err := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT phase_check`); err != nil {
		t.Fatal(err)
	}

	if _, err := tx.Exec(ctx, read("0162_retire_agent_session_hibernation.down.sql")); err != nil {
		t.Fatalf("roll back migration 0162: %v", err)
	}
	if n := countColumns(); n != 6 {
		t.Errorf("down migration restored %d of 6 hibernation columns", n)
	}
	if _, err := tx.Exec(ctx, `UPDATE agent_sessions SET phase = 'hibernated', snapshot_ref = 'x', pinned = true WHERE id = 'ags-done'`); err != nil {
		t.Errorf("an older binary cannot write its hibernation state after the down migration: %v", err)
	}
}
