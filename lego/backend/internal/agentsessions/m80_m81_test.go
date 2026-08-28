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
	"strconv"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
)

// --- w5/m80 t002: turn timeout is injected, never the driver's 4h fallback ---

func TestDriverEnvInjectsTurnTimeout(t *testing.T) {
	rec := store.AgentSession{ID: "ags-x", WorkspaceID: "tea-a", Turns: 1}

	// Default: the Service injects the 30m default so an unset knob is never 4h.
	def := (&Service{}).driverEnv(AgentConfig{Agent: "claude"}, rec)
	if got := def["BEX_AGENT_TURN_TIMEOUT_MS"]; got != strconv.Itoa(int((30*time.Minute).Milliseconds())) {
		t.Fatalf("default turn timeout = %q, want 1800000ms", got)
	}

	// A configured value flows through verbatim (in ms).
	set := (&Service{TurnTimeout: 12 * time.Minute}).driverEnv(AgentConfig{Agent: "claude"}, rec)
	if got := set["BEX_AGENT_TURN_TIMEOUT_MS"]; got != strconv.Itoa(int((12*time.Minute).Milliseconds())) {
		t.Fatalf("configured turn timeout = %q, want 720000ms", got)
	}
	if want := int64(4 * 60 * 60 * 1000); set["BEX_AGENT_TURN_TIMEOUT_MS"] == strconv.FormatInt(want, 10) {
		t.Fatal("turn timeout must never be the driver's 4h fallback")
	}
}

// --- w5/m80 t004: a failed session skips the completed-result idle grace ------

func TestCompleterFailedSessionSkipsIdleGrace(t *testing.T) {
	raw, _ := json.Marshal(statusReport{State: "failed", Error: "agent crashed"})
	c, st, lc, _, id := completerFixture(string(raw), nil)
	c.IdleTTL = 30 * time.Minute // a live grace that a COMPLETED session would defer under
	base := st.now
	c.Now = func() time.Time { return base }

	c.Reconcile(context.Background())

	if st.rows[id].Phase != PhaseFailed {
		t.Fatalf("phase = %s, want failed", st.rows[id].Phase)
	}
	// Despite the 30m grace, the failed sandbox is reclaimed at this very tick.
	if lc.canceled != 1 || st.rows[id].SandboxID != "" {
		t.Fatalf("failed sandbox not reclaimed under grace (canceled=%d sandbox=%q)", lc.canceled, st.rows[id].SandboxID)
	}
}

// An open editor SSH still pins even a FAILED session — never kill a live edit.
func TestCompleterFailedSessionPinnedByOpenSSH(t *testing.T) {
	raw, _ := json.Marshal(statusReport{State: "failed", Error: "agent crashed"})
	c, st, lc, _, id := completerFixture(string(raw), nil)
	c.IdleTTL = 30 * time.Minute
	base := st.now
	c.Now = func() time.Time { return base }
	st.openSSH[id] = base // an editor is connected right now

	c.Reconcile(context.Background())

	if st.rows[id].Phase != PhaseFailed {
		t.Fatalf("phase = %s, want failed", st.rows[id].Phase)
	}
	if lc.canceled != 0 || st.rows[id].SandboxID == "" {
		t.Fatalf("failed sandbox reclaimed while an editor SSH is open (canceled=%d sandbox=%q)", lc.canceled, st.rows[id].SandboxID)
	}
}

// --- w5/m81 t001/t002: lifecycle latency histograms observe on terminalize ---

func TestCompleterObservesTurnAndProvisionMetrics(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewCompletionMetrics(reg)

	// A completed turn observes the turn-duration histogram under outcome=completed.
	c, st, _, _, id := completerFixture(succeededStatus(true), nil)
	c.Metrics = metrics
	base := st.now
	c.Now = func() time.Time { return base.Add(42 * time.Second) } // 42s after the running transition
	c.Reconcile(context.Background())
	if st.rows[id].Phase != PhaseCompleted {
		t.Fatalf("phase = %s", st.rows[id].Phase)
	}
	// One outcome=completed series now exists (a series is created on first Observe).
	if n := testutil.CollectAndCount(metrics.turnDuration); n != 1 {
		t.Fatalf("turn-duration series after completed = %d, want 1", n)
	}

	// A failed turn observes under outcome=failed — a distinct second series.
	rawFail, _ := json.Marshal(statusReport{State: "failed", Error: "boom"})
	cf, _, _, _, _ := completerFixture(string(rawFail), nil)
	cf.Metrics = metrics
	cf.Now = func() time.Time { return base.Add(9 * time.Second) }
	cf.Reconcile(context.Background())
	if n := testutil.CollectAndCount(metrics.turnDuration); n != 2 {
		t.Fatalf("turn-duration series after completed+failed = %d, want 2", n)
	}

	// The Service records provisioning latency on a create failure.
	svc, _, _, lc := fixture()
	svc.Metrics = metrics
	lc.createErr = errors.New("pod schedule timeout")
	if _, err := svc.Create(caller("alice"), createInput()); err != nil {
		t.Fatal(err)
	}
	if n := testutil.CollectAndCount(metrics.provisionLatency); n != 1 {
		t.Fatalf("provision-latency series after a create failure = %d, want 1", n)
	}
}
