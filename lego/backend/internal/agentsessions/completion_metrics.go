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

import "github.com/prometheus/client_golang/prometheus"

// CompletionMetrics contains only bounded platform-controlled labels. Session,
// workspace, sandbox, repository, and error strings never become labels.
type CompletionMetrics struct {
	statusReads  *prometheus.CounterVec
	convergences *prometheus.CounterVec
}

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
	}
	reg.MustRegister(m.statusReads, m.convergences)
	return m
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
