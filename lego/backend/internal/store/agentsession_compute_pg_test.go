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

	ids "github.com/bex-co/bex/lego/backend/internal/id"
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
		u, err := st.AgentSessionSandboxUsage(ctx, sessionID)
		if err != nil {
			t.Fatal(err)
		}
		if u.CPUSeconds != 0 || u.MemoryGBSeconds != 0 {
			t.Fatalf("a session with no agent switch attributed used CPU or memory: %+v", u)
		}
		return u.ComputeMilliSeconds
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

	// w22/m314: the switch of sandbox billing to used CPU leaves a session's
	// total alone. Its sandbox is not billed on use, so an interval that spans
	// the switch still counts in full, on the same weighted basis.
	st.SandboxUsageBilledFrom = t0.Add(time.Hour)
	adopt(session.ID, "os-turn3")
	observe("os-turn3", 1000, t0.Add(time.Hour-10*time.Second))
	observe("os-turn3", 1000, t0.Add(time.Hour+10*time.Second))
	if got := total(session.ID); got != 50_000 {
		t.Fatalf("across the billing switch: %d, want 50000 (30000 + 20 s x 1000)", got)
	}
	st.SandboxUsageBilledFrom = time.Time{}

	// A sandbox no session adopted stays unattributed, and another session
	// reads only its own.
	observe("os-orphan", 1000, t0)
	observe("os-orphan", 1000, t0.Add(time.Minute))
	other := newSession("second")
	if got := total(other.ID); got != 0 {
		t.Fatalf("a session with no sandbox: %d, want 0", got)
	}
	if got := total(session.ID); got != 50_000 {
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

// w6/m566 t004: from AgentSandboxUsageBilledFrom an agent session's sandbox is
// billed for the CPU it used and the memory it was allocated, and its session
// is attributed exactly those billed rows: the weighted compute of the hours
// before the switch, the CPU and memory from it, and none of the CPU and
// memory marked unbilled before it. The binding seeds all three from what the
// meters wrote before the session adopted the sandbox, and the totals equal
// the sums of the sandbox's billed usage_hourly rows of each kind. The
// session's spend ceiling still counts its model requests alone.
func TestAgentSessionUsageAcrossAgentSwitchPG(t *testing.T) {
	st, ws := sandboxCPUTestStore(t, "agent-usage-switch")
	ctx := context.Background()
	agentFrom := time.Date(2026, 11, 26, 0, 0, 0, 0, time.UTC)
	st.SandboxUsageBilledFrom = agentFrom.Add(-8 * 24 * time.Hour)
	st.AgentSandboxUsageBilledFrom = agentFrom
	ceiling := int64(1_000_000) // maxSpendUSD 1.00
	session, err := st.CreateAgentSession(ctx, AgentSession{
		WorkspaceID: ws, AgentConfig: []byte(`{"agent":"claude","task":"t"}`), InitialPrompt: "p",
		MaxSpendMicroUSD: &ceiling,
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := st.RecordAgentModelUsage(ctx, AgentModelUsage{
		RequestID: ids.New(ids.AgentModelRequest), WorkspaceID: ws, SessionID: session.ID,
		Model: "m/a", At: agentFrom.Add(5 * time.Minute), CostSource: AgentModelCostReported, PriceMicroUSD: 250_000,
	}); err != nil {
		t.Fatal(err)
	}
	observe := func(sandbox, phase string, at time.Time) {
		t.Helper()
		if err := st.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: ws, SandboxID: sandbox, Phase: phase, Tier: "agent-4c-8g", WeightMilli: 4845, MemoryMiB: 8192,
			PodName: sandbox + "-0", ObservedAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	// readCPU reads one sandbox's windows in order, one claim each, as the
	// meter hands out an interval's next window only.
	readCPU := func(windows ...float64) {
		t.Helper()
		for i, seconds := range windows {
			w, err := st.ClaimSandboxCPUWindows(ctx, agentFrom.Add(3*time.Hour), agentFrom.Add(3*time.Hour), 100)
			if err != nil || len(w) != 1 {
				t.Fatalf("window %d = %+v err=%v, want one", i, w, err)
			}
			if ok, err := st.AccrueSandboxCPUWindow(ctx, w[0], seconds); err != nil || !ok {
				t.Fatal(ok, err)
			}
		}
	}
	usage := func() AgentSessionSandboxUsage {
		t.Helper()
		u, err := st.AgentSessionSandboxUsage(ctx, session.ID)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}
	adopt := func(sandbox string) {
		t.Helper()
		if _, err := st.SetAgentSessionLifecycle(ctx, session.ID, sandbox, "running", "running", false); err != nil {
			t.Fatal(err)
		}
	}

	// The first turn's sandbox ran 30 minutes before the switch and 10 after
	// it before the session row adopted it: the binding starts from the
	// weighted half hour and the billed 10 minutes of 8 GiB.
	observe("os-a", "running", agentFrom.Add(-30*time.Minute))
	observe("os-a", "running", agentFrom.Add(10*time.Minute))
	adopt("os-a")
	if got, want := usage(), (AgentSessionSandboxUsage{ComputeMilliSeconds: 4845 * 1800, MemoryGBSeconds: 8 * 600}); got != want {
		t.Fatalf("after adoption = %+v, want %+v", got, want)
	}
	// It runs 20 more minutes and stops; the CPU meter reads 300 CPU-seconds
	// before the switch (shown, not billed, not the session's) and 450 after.
	observe("os-a", "suspended", agentFrom.Add(30*time.Minute))
	readCPU(300, 450)
	if got, want := usage(), (AgentSessionSandboxUsage{ComputeMilliSeconds: 4845 * 1800, CPUSeconds: 450, MemoryGBSeconds: 8 * 1800}); got != want {
		t.Fatalf("after the turn = %+v, want %+v", got, want)
	}

	// The next turn's sandbox runs entirely after the switch, and the CPU
	// meter reads it before the session adopts it: the seed counts its CPU.
	observe("os-b", "running", agentFrom.Add(time.Hour))
	observe("os-b", "suspended", agentFrom.Add(time.Hour+10*time.Minute))
	readCPU(60.4)
	adopt("os-b")
	want := AgentSessionSandboxUsage{ComputeMilliSeconds: 4845 * 1800, CPUSeconds: 450 + 60, MemoryGBSeconds: 8 * (1800 + 600)}
	if got := usage(); got != want {
		t.Fatalf("across two turns = %+v, want %+v", got, want)
	}

	// An hour a binary from before the switch already marked keeps its mark
	// (a deploy that lands after the switch), and the session is not charged
	// for what lands in it.
	if _, err := st.Pool.Exec(ctx, `
		INSERT INTO usage_hourly (workspace_id, service_id, kind, tier, resource_kind, window_start, quantity, unbilled)
		VALUES ($1, 'os-c', $2, 'agent-4c-8g', $3, $4, 8, true)`,
		ws, UsageKindSandboxMemoryGBSeconds, ResourceKindSandbox, agentFrom.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	adopt("os-c")
	observe("os-c", "running", agentFrom.Add(2*time.Hour+time.Second))
	observe("os-c", "terminated", agentFrom.Add(2*time.Hour+11*time.Second))
	if got := usage(); got != want {
		t.Fatalf("after an hour already marked = %+v, want %+v", got, want)
	}
	if _, err := st.Pool.Exec(ctx, `DELETE FROM usage_hourly WHERE service_id = 'os-c'`); err != nil {
		t.Fatal(err)
	}

	// The session's totals are its sandboxes' billed rows, to the unit; what
	// was marked unbilled is the CPU and memory of the half hour before.
	type line struct {
		kind     string
		unbilled bool
	}
	rows := map[line]int64{}
	rs, err := st.Pool.Query(ctx, `
		SELECT kind, unbilled, SUM(quantity)::bigint FROM usage_hourly
		WHERE workspace_id = $1 AND service_id IN ('os-a', 'os-b') GROUP BY kind, unbilled`, ws)
	if err != nil {
		t.Fatal(err)
	}
	for rs.Next() {
		var l line
		var n int64
		if err := rs.Scan(&l.kind, &l.unbilled, &n); err != nil {
			t.Fatal(err)
		}
		rows[l] = n
	}
	rs.Close()
	wantRows := map[line]int64{
		{UsageKindSandboxComputeSeconds, false}:  want.ComputeMilliSeconds,
		{UsageKindSandboxCPUSeconds, false}:      want.CPUSeconds,
		{UsageKindSandboxMemoryGBSeconds, false}: want.MemoryGBSeconds,
		{UsageKindSandboxCPUSeconds, true}:       300,
		{UsageKindSandboxMemoryGBSeconds, true}:  8 * 1800,
	}
	if len(rows) != len(wantRows) {
		t.Errorf("usage rows = %v, want %v", rows, wantRows)
	}
	for k, v := range wantRows {
		if rows[k] != v {
			t.Errorf("usage %+v = %d, want %d", k, rows[k], v)
		}
	}

	// The maxSpendUSD ceiling bounds model spend only (ADR094 D8), on both
	// sides of the switch: the session's billed CPU and memory leave its
	// headroom at the ceiling minus its one model request.
	headroom, bounded, err := st.AgentSessionCeilingHeadroom(ctx, ws, session.ID)
	if err != nil || !bounded || headroom != ceiling-250_000 {
		t.Errorf("ceiling headroom = %d bounded=%v err=%v, want %d: sandbox usage is not model spend", headroom, bounded, err, ceiling-250_000)
	}
}
