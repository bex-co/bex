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

// w9/m178: the sandbox compute meter attributes a session's sandboxes to it.
// A sandbox the session row adopts is bound by migration 0206's trigger,
// starting from what the meter already accrued for it; later accruals,
// including the final interval of a sandbox that disappeared, add to the
// session's total; the total sums across turns' sandboxes; a sandbox no
// session adopted is never counted.
func TestAgentSessionComputeAttributionPG(t *testing.T) {
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
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
	st := NewPGStore(pool)
	tenant, err := st.CreateTenant(ctx, "agent-compute", PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pool.Exec(ctx, `DELETE FROM tenants WHERE id = $1`, tenant.ID) })
	newSession := func(prompt string) AgentSession {
		t.Helper()
		row, err := st.CreateAgentSession(ctx, AgentSession{
			WorkspaceID: tenant.ID, AgentConfig: []byte(`{"agent":"claude","task":"t"}`), InitialPrompt: prompt,
		})
		if err != nil {
			t.Fatal(err)
		}
		return row
	}
	observe := func(sandbox string, weight int64, at time.Time) {
		t.Helper()
		if err := st.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: tenant.ID, SandboxID: sandbox, Phase: "running",
			Tier: "agent", WeightMilli: weight, ObservedAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	adopt := func(sessionID, sandbox string) {
		t.Helper()
		if _, err := st.SetAgentSessionLifecycle(ctx, sessionID, sandbox, "running", "running", false); err != nil {
			t.Fatal(err)
		}
	}
	total := func(sessionID string) int64 {
		t.Helper()
		n, err := st.AgentSessionComputeMilliSeconds(ctx, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		return n
	}

	session := newSession("first")
	t0 := time.Date(2026, 11, 6, 10, 0, 0, 0, time.UTC)

	// The meter saw the sandbox run 10 s (1 vCPU) before the session row
	// adopted it: the binding starts from that accrual.
	observe("os-turn1", 1000, t0)
	observe("os-turn1", 1000, t0.Add(10*time.Second))
	adopt(session.ID, "os-turn1")
	if got := total(session.ID); got != 10_000 {
		t.Fatalf("after adopting a sandbox with 10 vCPU-seconds metered: %d, want 10000", got)
	}
	// Re-recording the same sandbox does not seed it twice.
	adopt(session.ID, "os-turn1")
	observe("os-turn1", 1000, t0.Add(20*time.Second))
	if got := total(session.ID); got != 20_000 {
		t.Fatalf("after 10 more seconds: %d, want 20000", got)
	}

	// The next turn runs in a fresh 2-vCPU sandbox; the total spans both. Its
	// final interval accrues when the meter finds it gone.
	adopt(session.ID, "os-turn2")
	observe("os-turn2", 2000, t0.Add(30*time.Second))
	if err := st.TerminateMissingSandboxMeters(ctx, tenant.ID, []string{"os-turn1"}, t0.Add(35*time.Second)); err != nil {
		t.Fatal(err)
	}
	if got := total(session.ID); got != 30_000 {
		t.Fatalf("across two turns' sandboxes: %d, want 30000 (20000 + 5 s x 2000)", got)
	}

	// A sandbox no session adopted stays unattributed, and another session
	// reads only its own.
	observe("os-orphan", 1000, t0)
	observe("os-orphan", 1000, t0.Add(time.Minute))
	other := newSession("second")
	if got := total(other.ID); got != 0 {
		t.Fatalf("a session with no sandbox: %d, want 0", got)
	}
	if got := total(session.ID); got != 30_000 {
		t.Fatalf("an orphan sandbox leaked into the session: %d", got)
	}
	var orphanRows int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM agent_session_sandboxes WHERE sandbox_id = 'os-orphan'`).Scan(&orphanRows); err != nil || orphanRows != 0 {
		t.Fatalf("orphan bindings = %d / %v, want 0", orphanRows, err)
	}

	// The binding goes with the session.
	if _, err := pool.Exec(ctx, `DELETE FROM agent_sessions WHERE id = $1`, session.ID); err != nil {
		t.Fatal(err)
	}
	if got := total(session.ID); got != 0 {
		t.Fatalf("after deleting the session: %d, want 0", got)
	}
}
