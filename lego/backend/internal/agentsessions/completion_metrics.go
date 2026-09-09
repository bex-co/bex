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

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// CompletionMetrics contains only bounded platform-controlled labels. Session,
// workspace, sandbox, repository, and error strings never become labels.
type CompletionMetrics struct {
	statusReads  *prometheus.CounterVec
	convergences *prometheus.CounterVec
	// turnOutcomes counts successful terminal CASes by bounded outcome
	// (w5/m88). Includes never-running failures that must not fabricate a
	// running-duration sample.
	turnOutcomes *prometheus.CounterVec
	// turnDuration is the wall-clock a turn spent in its running phase before
	// terminalization (w5/m81, corrected w5/m88), labelled by terminal outcome.
	// Anchored on agent_session_turns.started_at (set at bind), not
	// agent_sessions.updated_at — pin/archive must not shorten the sample. A
	// turn that never reached running (started_at NULL) does not emit a
	// running-duration sample.
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

// turnOutcome is the bounded terminal-outcome label on turn metrics. It never
// carries a session id, reason, or any tenant string.
type turnOutcome string

const (
	turnOutcomeCompleted          turnOutcome = "completed"
	turnOutcomeFailed             turnOutcome = "failed"
	turnOutcomeLost               turnOutcome = "lost"
	turnOutcomeCanceled           turnOutcome = "canceled"
	turnOutcomeVendorAuthRejected turnOutcome = "vendor_auth_rejected"
	turnOutcomeDispatchFailed     turnOutcome = "dispatch_failed"
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
		turnOutcomes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bex_agent_session_turn_outcomes_total",
			Help: "Agent-session turns terminalized by bounded outcome (successful CAS only).",
		}, []string{"outcome"}),
		turnDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "bex_agent_session_turn_duration_seconds",
			Help: "Agent-session turn running wall-clock (started_at → terminal CAS) by outcome; omitted when the turn never reached running.",
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
	reg.MustRegister(m.statusReads, m.convergences, m.turnOutcomes, m.turnDuration, m.provisionLatency)
	return m
}

// observeProvision records how long a session's sandbox took to reach Running
// (or to fail), under its outcome label. A non-positive duration is dropped.
func (m *CompletionMetrics) observeProvision(outcome provisionOutcome, d time.Duration) {
	if m != nil && d > 0 {
		m.provisionLatency.WithLabelValues(string(outcome)).Observe(d.Seconds())
	}
}

// observeTerminalTurn records a successful terminal CAS: always the outcome
// counter, and a running-duration sample only when started_at was set. Callers
// must invoke this only when the store reported the CAS won (err == nil).
func (m *CompletionMetrics) observeTerminalTurn(outcome turnOutcome, fact store.TerminalTurnFact) {
	if m == nil {
		return
	}
	m.turnOutcomes.WithLabelValues(string(outcome)).Inc()
	if fact.StartedAt == nil {
		return
	}
	d := fact.TerminalAt.Sub(*fact.StartedAt)
	if d > 0 {
		m.turnDuration.WithLabelValues(string(outcome)).Observe(d.Seconds())
	}
}

// ObserveVendorAuthRejected records a successful model-auth terminal CAS from
// the :8091 auth-failure verb (w5/m88). Exported so main can wire ModelAuthFailer
// without importing agentsessions' unexported outcome constants into agentsession.
func (m *CompletionMetrics) ObserveVendorAuthRejected(fact store.TerminalTurnFact) {
	m.observeTerminalTurn(turnOutcomeVendorAuthRejected, fact)
}

// observeDispatchFailed records a successful AbandonAgentDispatch session
// terminalization. A zero Turn means tombstone-only (no session phase change).
func (m *CompletionMetrics) observeDispatchFailed(fact store.TerminalTurnFact) {
	if fact.Turn > 0 {
		m.observeTerminalTurn(turnOutcomeDispatchFailed, fact)
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
