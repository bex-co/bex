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
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func TestPGAgentDispatchRecovery(t *testing.T) {
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
	t.Cleanup(pool.Close)
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, "dispatch-recovery-"+time.Now().Format("150405.000000000"), PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM agent_session_dispatches WHERE workspace_id=$1`, tenant.ID)
		_, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id=$1`, tenant.ID)
	})
	create := func() AgentSession {
		t.Helper()
		row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "do the task"})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	future := time.Now().Add(16 * time.Minute)

	t.Run("acceptance survives restart and failed cleanup without losing prompt", func(t *testing.T) {
		row := create()
		restarted := NewPGStore(pool)
		due, err := restarted.ListAgentDispatchesDue(ctx, future)
		if err != nil {
			t.Fatal(err)
		}
		var intent AgentDispatch
		for _, d := range due {
			if d.SessionID == row.ID {
				intent = d
			}
		}
		if intent.SessionID == "" {
			t.Fatal("accepted turn has no durable dispatch intent")
		}
		if err := restarted.AbandonAgentDispatch(ctx, intent, future, "provisioning interrupted; retry"); err != nil {
			t.Fatal(err)
		}
		// Crash before cleanup/schedule leaves the tombstone immediately recoverable.
		due, err = restarted.ListAgentDispatchesDue(ctx, future)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, d := range due {
			found = found || d.SessionID == row.ID
		}
		if !found {
			t.Fatal("cleanup intent lost after terminalization")
		}
		got, _ := s.GetAgentSession(ctx, row.ID)
		turns, _ := s.AgentSessionTurns(ctx, row.ID)
		if got.Phase != "failed" || got.FailureReason == "" || len(turns) != 1 || turns[0].Prompt != "do the task" || turns[0].CompletedAt == nil || turns[0].TranscriptComplete {
			t.Fatalf("row=%+v turns=%+v", got, turns)
		}
		if _, err := s.RecordAgentSessionDispatch(ctx, row.ID, "late-old", "running", "running", "", row.Turns); !errors.Is(err, ErrNotFound) {
			t.Fatalf("abandoned bind: %v", err)
		}
		retry, err := s.BeginAgentSessionTurn(ctx, row.ID, "retry", "redispatch", "redispatching", "redispatching")
		if err != nil {
			t.Fatal(err)
		}
		if err := s.AbandonAgentDispatch(ctx, intent, future, "stale sweep"); err != nil {
			t.Fatal(err)
		}
		got, _ = s.GetAgentSession(ctx, row.ID)
		if got.Turns != retry.Turns || got.Phase != "redispatching" {
			t.Fatalf("stale sweep damaged retry: %+v", got)
		}
		if _, err := s.RecordAgentSessionDispatch(ctx, row.ID, "late-old", "running", "running", "", row.Turns); !errors.Is(err, ErrNotFound) {
			t.Fatalf("old turn bound over retry: %v", err)
		}
		if _, err := s.RecordAgentSessionDispatch(ctx, row.ID, "new-"+row.ID, "running", "running", "redispatch", retry.Turns); err != nil {
			t.Fatal(err)
		}
		if err := s.DeferAgentDispatchCleanup(ctx, intent, future.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		due, err = s.ListAgentDispatchesDue(ctx, future.Add(2*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		found = false
		for _, d := range due {
			found = found || d.SessionID == row.ID && d.Turn == row.Turns
		}
		if !found {
			t.Fatal("late-create cleanup tombstone forgotten")
		}
	})
	t.Run("predecessor persisted before steer clears binding", func(t *testing.T) {
		row := create()
		row, err = s.RecordAgentSessionDispatch(ctx, row.ID, "previous-"+row.ID, "running", "running", "", row.Turns)
		if err != nil {
			t.Fatal(err)
		}
		row, err = s.FinalizeAgentSession(ctx, row.ID, "completed", "", "", 0, nil, "")
		if err != nil {
			t.Fatal(err)
		}
		row, err = s.BeginAgentSessionTurn(ctx, row.ID, "followup", "redispatch", "redispatching", "redispatching")
		if err != nil {
			t.Fatal(err)
		}
		due, err := s.ListAgentDispatchesDue(ctx, future)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range due {
			if d.SessionID == row.ID {
				if d.PreviousSandboxID != "previous-"+row.ID {
					t.Fatalf("predecessor=%q", d.PreviousSandboxID)
				}
				return
			}
		}
		t.Fatal("missing follow-up intent")
	})
	t.Run("old replica acceptance after migration and binding win", func(t *testing.T) {
		row := create()
		if _, err := pool.Exec(ctx, `DELETE FROM agent_session_dispatches WHERE session_id=$1`, row.ID); err != nil {
			t.Fatal(err)
		}
		// This row was accepted by an old process after the schema migration.
		due, err := s.ListAgentDispatchesDue(ctx, future)
		if err != nil {
			t.Fatal(err)
		}
		var intent AgentDispatch
		for _, d := range due {
			if d.SessionID == row.ID {
				intent = d
			}
		}
		if !intent.Legacy {
			t.Fatal("post-migration old acceptance was not discovered")
		}
		if _, err := s.SetAgentSessionLifecycle(ctx, row.ID, "legacy-"+row.ID, "running", "running", false); err != nil {
			t.Fatal(err)
		}
		if err := s.AbandonAgentDispatch(ctx, intent, future, "interrupted"); !errors.Is(err, ErrNotFound) {
			t.Fatalf("old binding must skip orphan deletion: %v", err)
		}
		got, err := s.GetAgentSession(ctx, row.ID)
		if err != nil || got.Phase != "running" || got.SandboxID != "legacy-"+row.ID {
			t.Fatalf("legacy binding damaged: %+v %v", got, err)
		}
	})
	t.Run("cancel remains terminal", func(t *testing.T) {
		row := create()
		d := AgentDispatch{SessionID: row.ID, Turn: row.Turns, WorkspaceID: row.WorkspaceID}
		if _, err := s.SetAgentSessionLifecycle(ctx, row.ID, "", "canceled", "canceled", false); err != nil {
			t.Fatal(err)
		}
		if err := s.AbandonAgentDispatch(ctx, d, future, "interrupted"); err != nil {
			t.Fatal(err)
		}
		got, _ := s.GetAgentSession(ctx, row.ID)
		if got.Phase != "canceled" {
			t.Fatalf("cancel resurrected: %+v", got)
		}
		if _, err := s.RecordAgentSessionDispatch(ctx, row.ID, "late-"+row.ID, "running", "running", "", row.Turns); !errors.Is(err, ErrNotFound) {
			t.Fatalf("late bind after cancel: %v", err)
		}
	})
	t.Run("deleted session retains policy cleanup obligation", func(t *testing.T) {
		row := create()
		if err := s.DeleteAgentSession(ctx, row.ID); err != nil {
			t.Fatal(err)
		}
		due, err := s.ListAgentDispatchesDue(ctx, future)
		if err != nil {
			t.Fatal(err)
		}
		for _, d := range due {
			if d.SessionID == row.ID {
				if !d.SessionDeleted {
					t.Fatal("deleted session was not distinguished for policy cleanup")
				}
				return
			}
		}
		t.Fatal("session deletion lost orphan cleanup intent")
	})
	t.Run("bind and abandonment serialize across replicas", func(t *testing.T) {
		for range 12 {
			row := create()
			d := AgentDispatch{SessionID: row.ID, Turn: row.Turns, WorkspaceID: row.WorkspaceID}
			var bindErr, abandonErr error
			var wg sync.WaitGroup
			wg.Add(2)
			go func() {
				defer wg.Done()
				_, bindErr = s.RecordAgentSessionDispatch(ctx, row.ID, "candidate-"+row.ID, "running", "running", "", row.Turns)
			}()
			go func() { defer wg.Done(); abandonErr = s.AbandonAgentDispatch(ctx, d, time.Now(), "interrupted") }()
			wg.Wait()
			got, _ := s.GetAgentSession(ctx, row.ID)
			if bindErr == nil {
				if !errors.Is(abandonErr, ErrNotFound) || got.Phase != "running" || got.SandboxID != "candidate-"+row.ID {
					t.Fatalf("binding winner: row=%+v abandon=%v", got, abandonErr)
				}
			} else if abandonErr != nil || !errors.Is(bindErr, ErrNotFound) || got.Phase != "failed" || got.SandboxID != "" {
				t.Fatalf("abandonment winner: row=%+v bind=%v abandon=%v", got, bindErr, abandonErr)
			}
		}
	})
}
