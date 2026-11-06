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
	"encoding/json"
	"testing"
)

// TestPGAgentTurnStructuredOutput (ADR094 D5, w21/m259): a turn stores the
// output contract it was accepted with, takes results only while it is the
// session's live current turn, and a final result is never downgraded.
func TestPGAgentTurnStructuredOutput(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "turn-output")
	ctx := context.Background()
	schema := json.RawMessage(`{"type":"object","required":["n"]}`)
	live := func(t *testing.T, output AgentTurnOutput) AgentSession {
		t.Helper()
		row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "report", InitialOutput: output})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='running' WHERE id=$1`, row.ID); err != nil {
			t.Fatal(err)
		}
		return row
	}
	turn := func(t *testing.T, id string, n int) AgentSessionTurn {
		t.Helper()
		turns, err := s.AgentSessionTurns(ctx, id)
		if err != nil || len(turns) < n {
			t.Fatalf("turns = %+v err=%v", turns, err)
		}
		return turns[n-1]
	}

	t.Run("create and a follow-up store the contract, a free-form turn stores none", func(t *testing.T) {
		row := live(t, AgentTurnOutput{Schema: schema, Required: true})
		first := turn(t, row.ID, 1)
		var stored map[string]any
		if json.Unmarshal(first.OutputSchema, &stored) != nil || stored["type"] != "object" || !first.OutputRequired || first.Result != nil || first.ResultFinal {
			t.Fatalf("turn 1 = %+v", first)
		}
		if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, row.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.BeginAgentSessionTurn(ctx, row.ID, "free", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{}, nil, nil); err != nil {
			t.Fatal(err)
		}
		if free := turn(t, row.ID, 2); free.OutputSchema != nil || free.OutputRequired {
			t.Fatalf("turn 2 = %+v, want no contract", free)
		}
	})

	t.Run("last valid wins and a final result is never downgraded", func(t *testing.T) {
		row := live(t, AgentTurnOutput{Schema: schema, Required: true})
		record := func(result string, final bool) bool {
			t.Helper()
			ok, err := s.RecordAgentSessionResult(ctx, row.ID, 1, json.RawMessage(result), final)
			if err != nil {
				t.Fatal(err)
			}
			return ok
		}
		if !record(`{"n":1}`, false) || turn(t, row.ID, 1).ResultFinal {
			t.Fatal("a partial result was refused or marked final")
		}
		if !record(`{"n":2}`, true) || !turn(t, row.ID, 1).ResultFinal {
			t.Fatal("the final result was refused")
		}
		if !record(`{"n":3}`, false) {
			t.Fatal("a late partial must be accepted (and ignored), not refused")
		}
		got := turn(t, row.ID, 1)
		var value struct{ N int }
		_ = json.Unmarshal(got.Result, &value)
		if value.N != 2 || !got.ResultFinal {
			t.Fatalf("result = %s final=%v, want the final {n:2} kept", got.Result, got.ResultFinal)
		}
		if !record(`{"n":4}`, true) {
			t.Fatal("a later final was refused")
		}
		_ = json.Unmarshal(turn(t, row.ID, 1).Result, &value)
		if value.N != 4 {
			t.Fatalf("n = %d, want the last valid final {n:4}", value.N)
		}
	})

	t.Run("refused for a free-form turn, a stale turn and a finished session", func(t *testing.T) {
		free := live(t, AgentTurnOutput{})
		if ok, err := s.RecordAgentSessionResult(ctx, free.ID, 1, json.RawMessage(`{"n":1}`), true); err != nil || ok {
			t.Fatalf("free-form turn took a result: ok=%v err=%v", ok, err)
		}
		row := live(t, AgentTurnOutput{Schema: schema, Required: true})
		if ok, _ := s.RecordAgentSessionResult(ctx, row.ID, 2, json.RawMessage(`{"n":1}`), true); ok {
			t.Fatal("a turn that does not exist took a result")
		}
		if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, row.ID); err != nil {
			t.Fatal(err)
		}
		if ok, err := s.RecordAgentSessionResult(ctx, row.ID, 1, json.RawMessage(`{"n":1}`), true); err != nil || ok {
			t.Fatalf("a finished session took a result: ok=%v err=%v", ok, err)
		}
		if got := turn(t, row.ID, 1); got.Result != nil {
			t.Fatalf("stored %s", got.Result)
		}
	})

	t.Run("an earlier turn takes no result once the session moved to the next", func(t *testing.T) {
		row := live(t, AgentTurnOutput{Schema: schema, Required: true})
		if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='completed' WHERE id=$1`, row.ID); err != nil {
			t.Fatal(err)
		}
		if _, err := s.BeginAgentSessionTurn(ctx, row.ID, "again", "redispatch", "redispatching", "redispatching", AgentAttachmentBinding{}, AgentTurnOutput{Schema: schema, Required: true}, nil, nil); err != nil {
			t.Fatal(err)
		}
		if ok, _ := s.RecordAgentSessionResult(ctx, row.ID, 1, json.RawMessage(`{"n":1}`), true); ok {
			t.Fatal("turn 1 took a result while turn 2 is current")
		}
		if ok, err := s.RecordAgentSessionResult(ctx, row.ID, 2, json.RawMessage(`{"n":1}`), true); err != nil || !ok {
			t.Fatalf("the current turn refused: ok=%v err=%v", ok, err)
		}
	})
}
