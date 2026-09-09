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

package agentsessions

import (
	"context"
	"encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// Pinning a running session rewrites agent_sessions.updated_at but must not
// shorten the turn-duration sample (w5/m88) — duration is anchored on
// agent_session_turns.started_at from the bind, not session UpdatedAt.
func TestCompleterTurnDurationIgnoresPinRewrite(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewCompletionMetrics(reg)

	c, st, _, _, id := completerFixture(succeededStatus(true), nil)
	c.Metrics = metrics
	base := st.now
	started := base.Add(-90 * time.Second)
	turn := st.turns[id][1]
	turn.StartedAt = &started
	st.turns[id][1] = turn

	// Pin near the end of the turn: would collapse UpdatedAt-based duration to ~1s.
	row := st.rows[id]
	row.UpdatedAt = base.Add(-1 * time.Second)
	row.Pinned = true
	st.rows[id] = row

	c.Now = func() time.Time { return base }
	c.Reconcile(context.Background())
	if st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("phase = %s", st.rows[id].Phase)
	}
	if got := testutil.ToFloat64(metrics.turnOutcomes.WithLabelValues(string(turnOutcomeCompleted))); got != 1 {
		t.Fatalf("completed outcomes = %v, want 1", got)
	}
	// Histogram sum should reflect ~90s from started_at, not ~1s from pin.
	metricFamilies, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var sum float64
	for _, mf := range metricFamilies {
		if mf.GetName() != "bex_agent_session_turn_duration_seconds" {
			continue
		}
		for _, m := range mf.GetMetric() {
			sum = m.GetHistogram().GetSampleSum()
		}
	}
	if sum < 80 || sum > 100 {
		t.Fatalf("turn duration sum = %v, want ~90s (pin must not shorten)", sum)
	}
}

// Two Completers racing Finalize yield exactly one outcome observation.
func TestCompleterDoubleFinalizeObservesOnce(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewCompletionMetrics(reg)
	raw, _ := json.Marshal(statusReport{State: "failed", Error: "race"})
	c, st, _, _, id := completerFixture(string(raw), nil)
	c.Metrics = metrics
	base := st.now
	c.Now = func() time.Time { return base.Add(5 * time.Second) }

	var wg sync.WaitGroup
	wg.Add(2)
	for range 2 {
		go func() {
			defer wg.Done()
			c.Reconcile(context.Background())
		}()
	}
	wg.Wait()
	if st.rows[id].Phase != PhaseFailed {
		t.Fatalf("phase = %s", st.rows[id].Phase)
	}
	if got := testutil.ToFloat64(metrics.turnOutcomes.WithLabelValues(string(turnOutcomeFailed))); got != 1 {
		t.Fatalf("failed outcomes = %v, want 1 (no double-count)", got)
	}
}

// A dispatch abandoned before bind has no started_at — outcome only, no duration.
func TestDispatchFailedNeverRunningOmitsDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewCompletionMetrics(reg)
	svc, _, _, lc := fixture()
	svc.Metrics = metrics
	lc.createErr = errors.New("pod schedule timeout")
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(metrics.turnOutcomes.WithLabelValues(string(turnOutcomeDispatchFailed))); got != 1 {
		t.Fatalf("dispatch_failed outcomes = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(metrics.turnDuration); n != 0 {
		t.Fatalf("running-duration series = %d, want 0 for never-running failure", n)
	}
}

// Cancel of a bound running turn records canceled with a duration sample.
func TestCancelObservesCanceledOutcome(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewCompletionMetrics(reg)
	svc, st, _, _ := fixture()
	svc.Metrics = metrics
	created, err := svc.Create(caller("alice"), createInput())
	if err != nil {
		t.Fatal(err)
	}
	id := created.ID
	// Bind so started_at exists (async create may already have bound).
	row := st.rows[id]
	if row.SandboxID == "" {
		bound, err := st.RecordAgentSessionDispatch(context.Background(), id, "sandbox-1", PhaseRunning, "running", "", row.Turns)
		if err != nil {
			t.Fatal(err)
		}
		row = bound
	}
	started := st.now.Add(-30 * time.Second)
	turn := st.turns[id][row.Turns]
	turn.StartedAt = &started
	st.turns[id][row.Turns] = turn

	if _, err := svc.Cancel(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(metrics.turnOutcomes.WithLabelValues(string(turnOutcomeCanceled))); got != 1 {
		t.Fatalf("canceled outcomes = %v, want 1", got)
	}
	if n := testutil.CollectAndCount(metrics.turnDuration); n != 1 {
		t.Fatalf("canceled running-duration series = %d, want 1", n)
	}
	// Idempotent cancel does not double-count.
	if _, err := svc.Cancel(caller("alice"), id); err != nil {
		t.Fatal(err)
	}
	if got := testutil.ToFloat64(metrics.turnOutcomes.WithLabelValues(string(turnOutcomeCanceled))); got != 1 {
		t.Fatalf("canceled outcomes after idempotent cancel = %v, want 1", got)
	}
}
