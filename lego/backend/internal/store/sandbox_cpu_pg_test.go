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

// sandboxCPUTestStore is a migrated, empty store with one workspace.
func sandboxCPUTestStore(t *testing.T, name string) (*PGStore, string) {
	t.Helper()
	uri := os.Getenv("BEX_TEST_DB_URI")
	if uri == "" {
		testenv.Skip(t, "BEX_TEST_DB_URI not set")
	}
	ctx := context.Background()
	if err := Migrate(uri); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `TRUNCATE tenants CASCADE`); err != nil {
		t.Fatal(err)
	}
	s := NewPGStore(pool)
	tenant, err := s.CreateTenant(ctx, name, PlanPro)
	if err != nil {
		t.Fatal(err)
	}
	return s, tenant.ID
}

type testInterval struct {
	pod             string
	started         time.Time
	ended           *time.Time
	billed, settled bool
}

func sandboxIntervals(t *testing.T, s *PGStore, ws, id string) []testInterval {
	t.Helper()
	rows, err := s.Pool.Query(context.Background(), `
		SELECT pod_name, started_at, ended_at, usage_billed, cpu_settled
		FROM sandbox_running_intervals WHERE workspace_id = $1 AND sandbox_id = $2 ORDER BY started_at`, ws, id)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var out []testInterval
	for rows.Next() {
		var i testInterval
		if err := rows.Scan(&i.pod, &i.started, &i.ended, &i.billed, &i.settled); err != nil {
			t.Fatal(err)
		}
		out = append(out, i)
	}
	return out
}

// w22/m314 t002: every stretch a sandbox runs is one interval named after the
// pod that ran it. A resume that recreates the pod, and a pod replaced while the
// sandbox stays running, each start a new interval, and a sandbox's intervals
// neither overlap nor leave a gap between consecutive running samples.
func TestSandboxRunningIntervalsPG(t *testing.T) {
	s, ws := sandboxCPUTestStore(t, "sandbox-intervals")
	ctx := context.Background()
	t0 := time.Date(2026, 8, 1, 0, 15, 0, 0, time.UTC)
	observe := func(id, phase, pod string, billed bool, at time.Time) {
		t.Helper()
		if err := s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: ws, SandboxID: id, Phase: phase, Tier: "1c-2g", WeightMilli: 1000,
			PodName: pod, UsageBilled: billed, ObservedAt: at,
		}); err != nil {
			t.Fatalf("observe %s %s at %s: %v", id, phase, at, err)
		}
	}

	observe("sbx-a", "creating", "os-1-0", true, t0) // not running yet: no interval
	if got := sandboxIntervals(t, s, ws, "sbx-a"); len(got) != 0 {
		t.Fatalf("a sandbox that is not running has intervals: %+v", got)
	}
	observe("sbx-a", "running", "os-1-0", true, t0.Add(time.Minute))
	observe("sbx-a", "running", "os-1-0", true, t0.Add(2*time.Minute))    // same pod: same interval
	observe("sbx-a", "running", "os-1-0", true, t0.Add(90*time.Second))   // an older sample changes nothing
	observe("sbx-a", "suspended", "os-1-0", true, t0.Add(10*time.Minute)) // closes it
	observe("sbx-a", "resuming", "os-2-0", true, t0.Add(20*time.Minute))
	observe("sbx-a", "running", "os-2-0", true, t0.Add(21*time.Minute)) // the resume's pod: a new interval
	observe("sbx-a", "running", "os-3-0", true, t0.Add(30*time.Minute)) // pod replaced while running
	got := sandboxIntervals(t, s, ws, "sbx-a")
	if len(got) != 3 {
		t.Fatalf("intervals = %+v, want 3", got)
	}
	want := []struct {
		pod        string
		start, end time.Duration
		open       bool
	}{
		{"os-1-0", time.Minute, 10 * time.Minute, false},
		{"os-2-0", 21 * time.Minute, 30 * time.Minute, false},
		{"os-3-0", 30 * time.Minute, 0, true},
	}
	for i, w := range want {
		g := got[i]
		if g.pod != w.pod || !g.started.Equal(t0.Add(w.start)) || (g.ended == nil) != w.open || (!w.open && !g.ended.Equal(t0.Add(w.end))) || !g.billed {
			t.Errorf("interval %d = %+v, want pod %s from +%s to +%s (open %v)", i, g, w.pod, w.start, w.end, w.open)
		}
	}
	// The replaced pod's interval ends at the instant the next one starts.
	if !got[1].ended.Equal(got[2].started) {
		t.Errorf("a pod replaced while running left a gap or overlap: %s vs %s", got[1].ended, got[2].started)
	}

	// A sandbox that vanishes from the list closes its interval once.
	for range 2 {
		if err := s.TerminateMissingSandboxMeters(ctx, ws, []string{}, t0.Add(45*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	got = sandboxIntervals(t, s, ws, "sbx-a")
	if len(got) != 3 || got[2].ended == nil || !got[2].ended.Equal(t0.Add(45*time.Minute)) {
		t.Fatalf("a terminated sandbox's interval = %+v, want it closed at +45m", got)
	}

	// A sandbox not billed on use (an agent session's) has its interval
	// recorded and read by the CPU meter too, which records its CPU unbilled
	// (w6/m566); a sample without a pod name records none.
	observe("sbx-agent", "running", "os-9-0", false, t0)
	observe("sbx-nopod", "running", "", true, t0)
	if agent := sandboxIntervals(t, s, ws, "sbx-agent"); len(agent) != 1 || agent[0].billed || agent[0].settled {
		t.Errorf("agent sandbox interval = %+v, want one, not billed on use and still to be read", agent)
	}
	if nopod := sandboxIntervals(t, s, ws, "sbx-nopod"); len(nopod) != 0 {
		t.Errorf("a sample without a pod name opened %+v", nopod)
	}
	windows, err := s.ClaimSandboxCPUWindows(ctx, t0.Add(24*time.Hour), t0.Add(24*time.Hour), 100)
	if err != nil {
		t.Fatal(err)
	}
	agentWindows := 0
	for _, w := range windows {
		if w.SandboxID == "sbx-agent" {
			agentWindows++
			if w.PodName != "os-9-0" || !w.From.Equal(t0) {
				t.Errorf("agent sandbox window = %+v, want its pod from its start", w)
			}
		}
	}
	if agentWindows != 1 {
		t.Errorf("the CPU meter was handed %d windows of the agent sandbox, want 1: %+v", agentWindows, windows)
	}
}

// w22/m314 t003: the CPU meter walks an interval hour by hour, adds what was
// read to sandbox_cpu_seconds, carries the sub-second part, never counts a
// window twice, and leaves a window it could not read exactly where it was.
func TestSandboxCPUWindowsPG(t *testing.T) {
	s, ws := sandboxCPUTestStore(t, "sandbox-cpu-windows")
	ctx := context.Background()
	t0 := time.Date(2026, 8, 1, 0, 15, 0, 0, time.UTC)
	observe := func(phase string, at time.Time) {
		t.Helper()
		if err := s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: ws, SandboxID: "sbx-a", Phase: phase, Tier: "1c-2g", WeightMilli: 1000,
			PodName: "os-1-0", UsageBilled: true, ObservedAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	claim := func(now time.Time) []SandboxCPUWindow {
		t.Helper()
		w, err := s.ClaimSandboxCPUWindows(ctx, now, now.Add(-time.Minute), 100)
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	cpuByHour := func() map[time.Time]int64 {
		t.Helper()
		rows, err := s.Pool.Query(ctx, `
			SELECT window_start, quantity FROM usage_hourly
			WHERE workspace_id = $1 AND service_id = 'sbx-a' AND kind = $2 AND tier = '1c-2g' AND resource_kind = $3`,
			ws, UsageKindSandboxCPUSeconds, ResourceKindSandbox)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		out := map[time.Time]int64{}
		for rows.Next() {
			var at time.Time
			var q int64
			if err := rows.Scan(&at, &q); err != nil {
				t.Fatal(err)
			}
			out[at.UTC()] = q
		}
		return out
	}

	observe("running", t0)
	// The hour the sandbox is running in has not ended: nothing to read yet.
	if w := claim(t0.Add(30 * time.Minute)); len(w) != 0 {
		t.Fatalf("an open hour was claimed: %+v", w)
	}
	// Once the hour closed (and the scrape lag passed) its part is due.
	hour1 := t0.Truncate(time.Hour).Add(time.Hour)
	if w := claim(hour1.Add(30 * time.Second)); len(w) != 0 {
		t.Fatalf("a window was claimed inside the scrape lag: %+v", w)
	}
	w := claim(hour1.Add(2 * time.Minute))
	if len(w) != 1 || !w[0].From.Equal(t0) || !w[0].To.Equal(hour1) || w[0].PodName != "os-1-0" || w[0].Tier != "1c-2g" || w[0].DeferredSince != nil {
		t.Fatalf("first window = %+v, want %s..%s on os-1-0", w, t0, hour1)
	}

	// No reading: the window is deferred, marked partial, and stays put.
	deferredAt := hour1.Add(2 * time.Minute)
	if err := s.DeferSandboxCPUWindow(ctx, w[0], deferredAt); err != nil {
		t.Fatal(err)
	}
	if err := s.DeferSandboxCPUWindow(ctx, w[0], deferredAt.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got := cpuByHour(); len(got) != 0 {
		t.Fatalf("a deferred window wrote usage: %v", got)
	}
	if st, err := s.SandboxCPUStats(ctx); err != nil || st.Pending != 1 || st.Deferred != 1 {
		t.Fatalf("stats = %+v err=%v, want 1 pending, 1 deferred", st, err)
	}
	again := claim(hour1.Add(5 * time.Minute))
	if len(again) != 1 || !again[0].From.Equal(t0) || again[0].DeferredSince == nil || !again[0].DeferredSince.Equal(deferredAt) {
		t.Fatalf("the deferred window came back as %+v, want the same window deferred since the first failure", again)
	}

	// The reading arrives: 61.4 CPU-seconds. 61 are written, 0.4 carried.
	if ok, err := s.AccrueSandboxCPUWindow(ctx, again[0], 61.4); err != nil || !ok {
		t.Fatalf("accrue = %v, %v", ok, err)
	}
	// A replay of the same window (a second replica, a retry) adds nothing.
	if ok, err := s.AccrueSandboxCPUWindow(ctx, again[0], 61.4); err != nil || ok {
		t.Fatalf("replayed accrue = %v, %v, want false", ok, err)
	}
	if got := cpuByHour(); got[t0.Truncate(time.Hour)] != 61 || len(got) != 1 {
		t.Fatalf("cpu rows = %v, want 61 in the first hour", got)
	}
	if st, _ := s.SandboxCPUStats(ctx); st.Pending != 1 || st.Deferred != 0 {
		t.Fatalf("stats after the reading = %+v, want the deferral cleared", st)
	}

	// The sandbox is suspended 20 minutes into the next hour: the last window
	// ends with the interval, and 0.7 more CPU-seconds complete the carried one.
	end := hour1.Add(20 * time.Minute)
	observe("suspended", end)
	last := claim(end.Add(2 * time.Minute))
	if len(last) != 1 || !last[0].From.Equal(hour1) || !last[0].To.Equal(end) {
		t.Fatalf("last window = %+v, want %s..%s", last, hour1, end)
	}
	if ok, err := s.AccrueSandboxCPUWindow(ctx, last[0], 0.7); err != nil || !ok {
		t.Fatalf("accrue = %v, %v", ok, err)
	}
	if got := cpuByHour(); got[hour1] != 1 || got[t0.Truncate(time.Hour)] != 61 {
		t.Fatalf("cpu rows = %v, want 61 then 1 (0.4 carried + 0.7)", got)
	}
	if left := claim(end.Add(24 * time.Hour)); len(left) != 0 {
		t.Fatalf("a settled interval was claimed again: %+v", left)
	}
	if st, _ := s.SandboxCPUStats(ctx); st.Pending != 0 {
		t.Fatalf("stats = %+v, want nothing pending", st)
	}
	if got := sandboxIntervals(t, s, ws, "sbx-a"); len(got) != 1 || !got[0].settled {
		t.Fatalf("interval = %+v, want settled", got)
	}

	// An idle window is a reading of zero: it advances and writes no row.
	observe("running", end.Add(time.Hour))
	observe("suspended", end.Add(time.Hour+5*time.Minute))
	idle := claim(end.Add(2 * time.Hour))
	if len(idle) != 1 {
		t.Fatalf("idle window = %+v", idle)
	}
	if ok, err := s.AccrueSandboxCPUWindow(ctx, idle[0], 0); err != nil || !ok {
		t.Fatalf("accrue zero = %v, %v", ok, err)
	}
	if got := cpuByHour(); len(got) != 2 {
		t.Fatalf("an idle window wrote a row: %v", got)
	}
	for _, bad := range []float64{-1, 2e9} {
		if _, err := s.AccrueSandboxCPUWindow(ctx, idle[0], bad); err == nil {
			t.Errorf("a reading of %v CPU-seconds was accepted", bad)
		}
	}

	// Intervals are pruned once they ended longer ago than the retention.
	if n, err := s.PruneSandboxIntervals(ctx, end.Add(SandboxIntervalRetention+time.Second), 100); err != nil || n != 1 {
		t.Fatalf("prune = %d, %v, want the first interval only", n, err)
	}
	if n, err := s.PruneSandboxIntervals(ctx, end.Add(SandboxIntervalRetention+2*time.Hour), 100); err != nil || n != 1 {
		t.Fatalf("prune = %d, %v, want the second interval", n, err)
	}
}

// w22/m314 t004: billing switches at one instant for every workspace. Before
// it every sandbox writes the weighted meter (billed) and memory (shown). From
// it a sandbox billed on use writes memory only, and the reads mark what was
// recorded before as unbilled. An agent session's sandbox keeps the weighted
// meter on both sides, and its used CPU and allocated memory are recorded
// beside it marked unbilled (w6/m566 t001): shown, never priced, exported,
// settled or compacted, and the weighted charge is what it was.
func TestSandboxUsageSwitchPG(t *testing.T) {
	s, ws := sandboxCPUTestStore(t, "sandbox-usage-switch")
	ctx := context.Background()
	billedFrom := time.Date(2026, 8, 1, 2, 0, 0, 0, time.UTC)
	s.SandboxUsageBilledFrom = billedFrom
	observe := func(id, phase string, billed bool, at time.Time) {
		t.Helper()
		if err := s.ObserveSandboxMeter(ctx, SandboxMeterObservation{
			WorkspaceID: ws, SandboxID: id, Phase: phase, Tier: "1c-2g", WeightMilli: 1000, MemoryMiB: 2048,
			PodName: id + "-0", UsageBilled: billed, ObservedAt: at,
		}); err != nil {
			t.Fatal(err)
		}
	}
	type line struct {
		kind     string
		unbilled bool
	}
	totals := func(id string) map[line]int64 {
		t.Helper()
		rows, err := s.UsageMonthToDate(ctx, ws, billedFrom.Add(24*time.Hour))
		if err != nil {
			t.Fatal(err)
		}
		got := map[line]int64{}
		for _, r := range rows {
			if r.ServiceID == id {
				got[line{r.Kind, r.Unbilled}] += r.Total
			}
		}
		return got
	}

	// Both run from one hour before the switch to one hour after it.
	start, end := billedFrom.Add(-time.Hour), billedFrom.Add(time.Hour)
	for _, sb := range []struct {
		id     string
		billed bool
	}{{"sbx-render", true}, {"os-agent", false}} {
		observe(sb.id, "running", sb.billed, start)
		observe(sb.id, "running", sb.billed, billedFrom.Add(-30*time.Minute)) // a poll before the switch
		observe(sb.id, "suspended", sb.billed, end)                           // one interval spans the switch
	}
	// The used-CPU meter read 600 CPU-seconds in each hour of either sandbox.
	for i, at := range []time.Time{start, billedFrom} {
		windows, err := s.ClaimSandboxCPUWindows(ctx, end.Add(time.Hour), end.Add(time.Hour), 100)
		if err != nil || len(windows) != 2 {
			t.Fatalf("windows %d = %+v err=%v, want one per sandbox", i, windows, err)
		}
		for _, w := range windows {
			if !w.From.Equal(at) {
				t.Fatalf("window %d = %+v, want it from %s", i, w, at)
			}
			if ok, err := s.AccrueSandboxCPUWindow(ctx, w, 600); err != nil || !ok {
				t.Fatal(ok, err)
			}
		}
	}

	render := totals("sbx-render")
	wantRender := map[line]int64{
		{UsageKindSandboxComputeSeconds, false}:  1000 * 3600, // the hour before, billed as it was
		{UsageKindSandboxMemoryGBSeconds, true}:  2 * 3600,    // recorded beside it: shown
		{UsageKindSandboxCPUSeconds, true}:       600,
		{UsageKindSandboxMemoryGBSeconds, false}: 2 * 3600, // the hour after: billed
		{UsageKindSandboxCPUSeconds, false}:      600,
	}
	if len(render) != len(wantRender) {
		t.Errorf("Render sandbox lines = %v, want %v", render, wantRender)
	}
	for k, v := range wantRender {
		if render[k] != v {
			t.Errorf("Render sandbox %+v = %d, want %d", k, render[k], v)
		}
	}
	agent := totals("os-agent")
	wantAgent := map[line]int64{
		{UsageKindSandboxComputeSeconds, false}: 2 * 1000 * 3600, // both hours on the weighted meter
		{UsageKindSandboxMemoryGBSeconds, true}: 2 * 2 * 3600,    // shown on both sides of the switch
		{UsageKindSandboxCPUSeconds, true}:      2 * 600,
	}
	if len(agent) != len(wantAgent) {
		t.Errorf("agent sandbox lines = %v, want %v", agent, wantAgent)
	}
	for k, v := range wantAgent {
		if agent[k] != v {
			t.Errorf("agent sandbox %+v = %d, want %d", k, agent[k], v)
		}
	}

	// Without a switch instant nothing ever switches: everything keeps the
	// weighted meter and the two new kinds stay unbilled.
	never := NewPGStore(s.Pool)
	if err := never.ObserveSandboxMeter(ctx, SandboxMeterObservation{WorkspaceID: ws, SandboxID: "sbx-never", Phase: "running",
		Tier: "1c-2g", WeightMilli: 1000, MemoryMiB: 2048, UsageBilled: true, ObservedAt: start}); err != nil {
		t.Fatal(err)
	}
	if err := never.ObserveSandboxMeter(ctx, SandboxMeterObservation{WorkspaceID: ws, SandboxID: "sbx-never", Phase: "suspended",
		Tier: "1c-2g", WeightMilli: 1000, MemoryMiB: 2048, UsageBilled: true, ObservedAt: end}); err != nil {
		t.Fatal(err)
	}
	rows, err := never.UsageMonthToDate(ctx, ws, billedFrom.Add(24*time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rows {
		if r.ServiceID != "sbx-never" {
			continue
		}
		switch r.Kind {
		case UsageKindSandboxComputeSeconds:
			if r.Total != 2*1000*3600 {
				t.Errorf("without a switch the weighted meter read %d, want both hours", r.Total)
			}
		case UsageKindSandboxMemoryGBSeconds:
			if !r.Unbilled || r.Total != 2*2*3600 {
				t.Errorf("without a switch memory = %+v, want 4 GiB-hours unbilled", r)
			}
		}
	}

	// The billing outbox hands the export the agent sandbox's CPU and memory
	// marked unbilled, which it stamps emitted without sending, and nothing
	// else marked.
	pending, err := s.SelectWorkspaceUnemittedUsage(ctx, ws, start.Add(-time.Hour), UsageSeal{Before: end.Add(time.Hour)}, 100, false)
	if err != nil {
		t.Fatal(err)
	}
	marked := map[string]int64{}
	for _, r := range pending {
		agentUsage := r.ServiceID == "os-agent" && r.Kind != UsageKindSandboxComputeSeconds
		if r.Unbilled != agentUsage {
			t.Errorf("outbox row %s %s %s unbilled=%v, want %v", r.ServiceID, r.Kind, r.WindowStart, r.Unbilled, agentUsage)
		}
		if r.Unbilled {
			marked[r.Kind] += r.Quantity
		}
	}
	if marked[UsageKindSandboxCPUSeconds] != 2*600 || marked[UsageKindSandboxMemoryGBSeconds] != 2*2*3600 {
		t.Errorf("outbox rows marked unbilled = %v, want the agent sandbox's two hours of CPU and memory", marked)
	}

	// Settle before delete counts only what was exported: the hours before the
	// switch, and the agent sandbox's CPU and memory, are stamped emitted
	// without being sent.
	if _, err := s.Pool.Exec(ctx, `UPDATE usage_hourly SET billing_export_state = 'emitted', emitted_at = $2 WHERE workspace_id = $1 AND service_id IN ('sbx-render', 'os-agent')`, ws, end); err != nil {
		t.Fatal(err)
	}
	exported, err := s.ExportedUsageTotals(ctx, ws, start.Add(-time.Hour), end.Add(time.Hour), end)
	if err != nil {
		t.Fatal(err)
	}
	gotExported := map[string]int64{}
	for _, e := range exported {
		gotExported[e.Kind] += e.Quantity
	}
	if gotExported[UsageKindSandboxCPUSeconds] != 600 || gotExported[UsageKindSandboxMemoryGBSeconds] != 2*3600 ||
		gotExported[UsageKindSandboxComputeSeconds] != (1+2)*1000*3600 {
		t.Errorf("exported totals = %v, want the billed hour of CPU and memory and every weighted hour", gotExported)
	}

	// Compaction folds a month's billed usage and drops what was only shown.
	res, err := s.CompactUsage(ctx, end.Add(time.Hour))
	if err != nil || res.HourlyRows == 0 {
		t.Fatalf("compact = %+v, %v", res, err)
	}
	after := totals("sbx-render")
	if after[line{UsageKindSandboxCPUSeconds, false}] != 600 || after[line{UsageKindSandboxMemoryGBSeconds, false}] != 2*3600 ||
		after[line{UsageKindSandboxComputeSeconds, false}] != 1000*3600 {
		t.Errorf("after compaction the billed lines read %v", after)
	}
	if after[line{UsageKindSandboxCPUSeconds, true}] != 0 || after[line{UsageKindSandboxMemoryGBSeconds, true}] != 0 {
		t.Errorf("compaction kept usage that was never billed: %v", after)
	}
	if after := totals("os-agent"); len(after) != 1 || after[line{UsageKindSandboxComputeSeconds, false}] != 2*1000*3600 {
		t.Errorf("after compaction the agent sandbox reads %v, want its weighted charge alone", after)
	}
}
