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

// TestPGAgentSessionFailureCode (w5/m132): only an abandoned dispatch writes a
// failure code, in the same write as its sentence. Every other write that
// clears or replaces the sentence clears the code with it.
func TestPGAgentSessionFailureCode(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "failure-code")
	ctx := context.Background()
	capacity := AgentSessionFailure{Reason: "sandbox capacity reached", Code: "SANDBOX_CAPACITY_LIMIT"}
	create := func(t *testing.T) AgentSession {
		t.Helper()
		row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "do the task"})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	stored := func(t *testing.T, id string) AgentSession {
		t.Helper()
		row, err := s.GetAgentSession(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	// refused is a session whose first dispatch was refused for capacity.
	refused := func(t *testing.T) AgentSession {
		t.Helper()
		row := create(t)
		if _, err := s.AbandonAgentDispatch(ctx, AgentDispatch{SessionID: row.ID, WorkspaceID: tenant.ID, Turn: row.Turns}, time.Now(), capacity); err != nil {
			t.Fatal(err)
		}
		return stored(t, row.ID)
	}
	setPhase := func(t *testing.T, id, phase string) {
		t.Helper()
		if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase=$2, snapshot_ref='snap-1' WHERE id=$1`, id, phase); err != nil {
			t.Fatal(err)
		}
	}
	want := func(t *testing.T, row AgentSession, reason, code string) {
		t.Helper()
		if row.FailureReason != reason || row.FailureReasonCode != code {
			t.Fatalf("failure = %q / %q, want %q / %q", row.FailureReason, row.FailureReasonCode, reason, code)
		}
	}

	t.Run("an abandoned dispatch stores the code with its sentence", func(t *testing.T) {
		row := refused(t)
		if row.Phase != "failed" {
			t.Fatalf("phase %q, want failed", row.Phase)
		}
		want(t, row, capacity.Reason, capacity.Code)
	})
	t.Run("a new turn clears it", func(t *testing.T) {
		row, err := s.BeginAgentSessionTurn(ctx, refused(t).ID, "try again", "", "redispatching", "redispatching")
		if err != nil {
			t.Fatal(err)
		}
		want(t, row, "", "")
	})
	t.Run("a recorded dispatch clears it", func(t *testing.T) {
		row := create(t)
		if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET failure_reason=$2, failure_reason_code=$3 WHERE id=$1`, row.ID, capacity.Reason, capacity.Code); err != nil {
			t.Fatal(err)
		}
		row, err := s.RecordAgentSessionDispatch(ctx, row.ID, "sbx-failure-code", "running", "running", "", row.Turns)
		if err != nil {
			t.Fatal(err)
		}
		want(t, row, "", "")
	})
	t.Run("another failure clears it", func(t *testing.T) {
		row, err := s.SetAgentSessionFailure(ctx, refused(t).ID, "", "egress phase transition failed")
		if err != nil {
			t.Fatal(err)
		}
		want(t, row, "egress phase transition failed", "")
	})
	t.Run("a finalize clears it, even with the same sentence", func(t *testing.T) {
		// Finalize's only path to a failed row is the reaper's unclaim, which
		// re-finalizes with the claimed sentence. A coded row never holds the
		// sandbox a claim needs, so the phase is set by hand.
		row := refused(t)
		setPhase(t, row.ID, "hibernating")
		row, _, err := s.FinalizeAgentSession(ctx, row.ID, "failed", "", "", 0, nil, capacity.Reason)
		if err != nil {
			t.Fatal(err)
		}
		want(t, row, capacity.Reason, "")
	})
	t.Run("a hibernation expiry clears it", func(t *testing.T) {
		row := refused(t)
		setPhase(t, row.ID, "hibernated")
		row, err := s.ExpireHibernatedAgentSession(ctx, row.ID, "snap-1")
		if err != nil {
			t.Fatal(err)
		}
		want(t, row, "hibernation retention window elapsed", "")
	})
}

// TestAgentSessionFailureReasonCodeMigration pins migration 0144 (w5/m132):
// sessions refused for sandbox capacity before it are coded, whether the
// sentence sits in failure_reason or, before w5/m80, in status. Every other
// row stays uncoded, and the down path drops the column.
func TestAgentSessionFailureReasonCodeMigration(t *testing.T) {
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
	up, err := migrationsFS.ReadFile("migrations/0144_agent_session_failure_reason_code.up.sql")
	if err != nil {
		t.Fatal(err)
	}
	down, err := migrationsFS.ReadFile("migrations/0144_agent_session_failure_reason_code.down.sql")
	if err != nil {
		t.Fatal(err)
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `
		CREATE SCHEMA migration_0144_failure_reason_code;
		SET LOCAL search_path TO migration_0144_failure_reason_code;
		CREATE TABLE agent_sessions (
			id text PRIMARY KEY,
			phase text NOT NULL,
			status text NOT NULL DEFAULT '',
			failure_reason text NOT NULL DEFAULT ''
		);
		INSERT INTO agent_sessions (id, phase, status, failure_reason) VALUES
			('ags-reason', 'failed', 'failed', 'sandbox capacity reached'),
			('ags-status', 'failed', 'sandbox capacity reached', ''),
			('ags-other', 'failed', 'failed', 'sandbox create failed'),
			('ags-running', 'running', 'running', '');
	`); err != nil {
		t.Fatalf("prepare pre-0144 sessions: %v", err)
	}
	if _, err := tx.Exec(ctx, string(up)); err != nil {
		t.Fatalf("apply migration 0144: %v", err)
	}
	for id, code := range map[string]string{
		"ags-reason": "SANDBOX_CAPACITY_LIMIT", "ags-status": "SANDBOX_CAPACITY_LIMIT", "ags-other": "", "ags-running": "",
	} {
		var got string
		if err := tx.QueryRow(ctx, `SELECT failure_reason_code FROM agent_sessions WHERE id = $1`, id).Scan(&got); err != nil {
			t.Fatal(err)
		}
		if got != code {
			t.Errorf("%s failure_reason_code = %q, want %q", id, got, code)
		}
	}
	if _, err := tx.Exec(ctx, string(down)); err != nil {
		t.Fatalf("roll back migration 0144: %v", err)
	}
	var columns int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM information_schema.columns
		WHERE table_schema = 'migration_0144_failure_reason_code' AND column_name = 'failure_reason_code'`).Scan(&columns); err != nil {
		t.Fatal(err)
	}
	if columns != 0 {
		t.Error("the down migration left failure_reason_code in place")
	}
}
