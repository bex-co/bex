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
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// CompletionMetrics contains only bounded platform-controlled labels. Session,
// workspace, sandbox, repository, and error strings never become labels.
type CompletionMetrics struct {
	statusReads  *prometheus.CounterVec
	convergences *prometheus.CounterVec
	// turnDuration is the wall-clock a turn spent in its running phase before the
	// Completer terminalized it, labelled by terminal outcome (w5/m81 t001). It is
	// measured from the running-transition (record.UpdatedAt) to terminalization,
	// so it includes up to one poll interval of detection lag — an upper bound on
	// the turn's true duration, which is exactly the signal an operator wants: it
	// is what surfaces a slow/hung turn (e.g. the ~192s bad-key case w5/m80 t003
	// eliminates) as a metric rather than a manual pod-level investigation.
	turnDuration *prometheus.HistogramVec
	// provisionLatency is the wall-clock to stand a session's sandbox up: the
	// CreateAgentSessionSandbox call, which blocks until the pod is Running with an
	// IP (or fails), labelled running/failed (w5/m81 t002). It is the "accept fast,
	// provision async" (ADR047) gap the user waits through before the agent boots;
	// agent boot itself measured <1s on dev-5, so this closely tracks time-to-first
	// -model-call for a turn-1 session (ADR047 records that approximation).
	provisionLatency *prometheus.HistogramVec
}

// provisionOutcome is the bounded outcome label on the provisioning histogram.
type provisionOutcome string

const (
	provisionRunning provisionOutcome = "running"
	provisionFailed  provisionOutcome = "failed"
)

// turnOutcome is the bounded terminal-outcome label on the turn-duration
// histogram. It never carries a session id, reason, or any tenant string.
type turnOutcome string

const (
	turnOutcomeCompleted turnOutcome = "completed"
	turnOutcomeFailed    turnOutcome = "failed"
	// turnOutcomeLost is a driver/sandbox that died before reporting — the
	// Completer's terminal backstop, distinct from a driver-reported failure.
	turnOutcomeLost turnOutcome = "lost"
)

type statusReadOutcome string

const (
	statusReadOK             statusReadOutcome = "ok"
	statusReadTerminal       statusReadOutcome = "terminal"
	statusReadTransientError statusReadOutcome = "transient_error"
)

type terminalConvergenceReason string

const (
	convergenceTargetTerminated  terminalConvergenceReason = "target_terminated"
	convergenceStatusUnavailable terminalConvergenceReason = "status_unavailable"
)

func NewCompletionMetrics(reg prometheus.Registerer) *CompletionMetrics {
	m := &CompletionMetrics{
		statusReads: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bex_agent_session_status_reads_total",
			Help: "Agent-session Completer status reads by bounded outcome.",
		}, []string{"outcome"}),
		convergences: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bex_agent_session_terminal_convergences_total",
			Help: "Agent sessions terminalized by the lifecycle backstop.",
		}, []string{"reason"}),
		turnDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "bex_agent_session_turn_duration_seconds",
			Help: "Agent-session turn wall-clock (running-transition to terminalization) by outcome.",
			// Sub-second through ~30m: brackets a fast happy-path turn, a normal
			// coding turn, and a hung turn approaching the BEX_AGENT_TURN_TIMEOUT bound.
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1200, 1800, 3600},
		}, []string{"outcome"}),
		provisionLatency: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "bex_agent_session_provision_seconds",
			Help: "Agent-session sandbox provisioning wall-clock (create → Running) by outcome.",
			// Warm 3–10s; a cold image pull is ~23s (ADR059); the platform pod-ready
			// wait is ~300s, so the tail bracket captures a stuck provision.
			Buckets: []float64{1, 2, 5, 10, 20, 30, 60, 120, 300},
		}, []string{"outcome"}),
	}
	reg.MustRegister(m.statusReads, m.convergences, m.turnDuration, m.provisionLatency)
	return m
}

// observeProvision records how long a session's sandbox took to reach Running
// (or to fail), under its outcome label. A non-positive duration is dropped.
func (m *CompletionMetrics) observeProvision(outcome provisionOutcome, d time.Duration) {
	if m != nil && d > 0 {
		m.provisionLatency.WithLabelValues(string(outcome)).Observe(d.Seconds())
	}
}

// observeTurn records a terminated turn's duration under its outcome label. A
// non-positive duration (a clock skew or a row whose updated_at is in the future)
// is dropped rather than recorded as a spurious zero.
func (m *CompletionMetrics) observeTurn(outcome turnOutcome, d time.Duration) {
	if m != nil && d > 0 {
		m.turnDuration.WithLabelValues(string(outcome)).Observe(d.Seconds())
	}
}

func (m *CompletionMetrics) read(outcome statusReadOutcome) {
	if m != nil {
		m.statusReads.WithLabelValues(string(outcome)).Inc()
	}
}

func (m *CompletionMetrics) converged(reason terminalConvergenceReason) {
	if m != nil {
		m.convergences.WithLabelValues(string(reason)).Inc()
	}
}
