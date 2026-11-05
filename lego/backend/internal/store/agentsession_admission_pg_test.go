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
	"sync"
	"testing"
)

// ADR091 D10.1: the trust tier's concurrent-session limit is taken in the
// create's own transaction, so two creates racing at count=1 with limit 2
// admit exactly one. The count sees only active sessions of the agent, and a
// steer/resume's own session is excluded.
func TestPGAgentSessionAdmissionLimitIsAtomic(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "admission-limit")
	ctx := context.Background()
	claude := AgentSessionCount{Agent: "claude"}
	create := func(agent string, limit int) (AgentSession, error) {
		return s.CreateAgentSession(ctx, AgentSession{
			WorkspaceID: tenant.ID, AgentConfig: []byte(`{"agent":"` + agent + `","task":"x"}`), InitialPrompt: "x",
			AdmissionLimit: limit, AdmissionCount: claude,
		})
	}
	first, err := create("claude", 2)
	if err != nil {
		t.Fatal(err)
	}
	// Another profile's session and a terminal claude session never count.
	if _, err := create("codex", 0); err != nil {
		t.Fatal(err)
	}
	done, err := create("claude", 0)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase = 'completed' WHERE id = $1`, done.ID); err != nil {
		t.Fatal(err)
	}

	for round := range 5 {
		var wg sync.WaitGroup
		errs := make([]error, 2)
		ids := make([]string, 2)
		for i := range 2 {
			wg.Add(1)
			go func() {
				defer wg.Done()
				row, err := create("claude", 2)
				errs[i], ids[i] = err, row.ID
			}()
		}
		wg.Wait()
		admitted, refused := 0, 0
		for i, err := range errs {
			switch {
			case err == nil:
				admitted++
				if _, err := pool.Exec(ctx, `DELETE FROM agent_sessions WHERE id = $1`, ids[i]); err != nil {
					t.Fatal(err)
				}
			case errors.Is(err, ErrAgentSessionAdmissionLimit):
				refused++
			default:
				t.Fatalf("round %d: %v", round, err)
			}
		}
		if admitted != 1 || refused != 1 {
			t.Fatalf("round %d: admitted %d refused %d, want exactly one of each", round, admitted, refused)
		}
	}

	for _, tc := range []struct {
		c    AgentSessionCount
		want int
	}{
		{claude, 1},
		{AgentSessionCount{}, 2},
		{AgentSessionCount{Agent: "claude", ExcludeID: first.ID}, 0},
		{AgentSessionCount{WithSandbox: true}, 0},
	} {
		if n, err := s.CountActiveAgentSessions(ctx, tenant.ID, tc.c); err != nil || n != tc.want {
			t.Errorf("count %+v = %d, %v, want %d", tc.c, n, err, tc.want)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET sandbox_id = 'sbx-1' WHERE id = $1`, first.ID); err != nil {
		t.Fatal(err)
	}
	if n, err := s.CountActiveAgentSessions(ctx, tenant.ID, AgentSessionCount{WithSandbox: true}); err != nil || n != 1 {
		t.Fatalf("live sandboxes = %d, %v, want 1", n, err)
	}
}
