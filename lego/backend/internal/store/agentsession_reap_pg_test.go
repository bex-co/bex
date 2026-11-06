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
	"testing"
	"time"
)

// A failed teardown schedules the next reap with a growing backoff (w5/m99).
// The 076/077 live run found the UPDATE refused on real Postgres: `$2 +
// make_interval(...)` made Postgres type $2 as interval (SQLSTATE 42804), so
// no retry was ever scheduled. Only a fake store had exercised it.
func TestPGScheduleAgentSessionSandboxReapBacksOff(t *testing.T) {
	s, pool, tenant := agentSessionPGStore(t, "sandbox-reap")
	ctx := context.Background()
	row, err := s.CreateAgentSession(ctx, AgentSession{WorkspaceID: tenant.ID, InitialPrompt: "x"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `UPDATE agent_sessions SET phase='failed', sandbox_id='sbx-reap' WHERE id=$1`, row.ID); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 6, 6, 0, 0, 0, time.UTC)
	due := func(at time.Time) bool {
		t.Helper()
		rows, err := s.ListTerminalAgentSessionsWithSandbox(ctx, at)
		if err != nil {
			t.Fatal(err)
		}
		for _, r := range rows {
			if r.ID == row.ID {
				return true
			}
		}
		return false
	}
	if !due(now) {
		t.Fatal("a never-attempted terminal session is not due")
	}
	// Attempts 0, 1, 2 back off 1m, 2m, 4m from the failure time.
	for i, wait := range []time.Duration{time.Minute, 2 * time.Minute, 4 * time.Minute} {
		if err := s.ScheduleAgentSessionSandboxReap(ctx, row.ID, now); err != nil {
			t.Fatalf("schedule attempt %d: %v", i, err)
		}
		var after time.Time
		var attempts int
		if err := pool.QueryRow(ctx, `SELECT sandbox_reap_after, sandbox_reap_attempts FROM agent_sessions WHERE id=$1`, row.ID).Scan(&after, &attempts); err != nil {
			t.Fatal(err)
		}
		if !after.Equal(now.Add(wait)) || attempts != i+1 {
			t.Fatalf("attempt %d: reap_after=%v attempts=%d, want %v and %d", i, after, attempts, now.Add(wait), i+1)
		}
		if due(now.Add(wait-time.Second)) || !due(now.Add(wait)) {
			t.Fatalf("attempt %d: due-ness does not follow the %v backoff", i, wait)
		}
	}
}
