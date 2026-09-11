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

package sandbox

import (
	"errors"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Create outcomes. A closed set, deliberately: these become series labels, so
// anything tenant-chosen would be unbounded cardinality (ADR088 § label
// contract).
//
// The split that matters is attempt versus refusal. succeeded/failed/capacity
// describe a create that actually reached the sandbox runtime, and only those
// three tell you whether the substrate is healthy. rejected and unavailable are
// decided before any runtime call — a bad request, a denied authorization, a
// billing gate, or sandboxes not being configured at all — so counting them in
// a failure ratio would let a client spraying invalid requests either raise a
// false alarm or, worse, dilute a real one.
const (
	outcomeSucceeded   = "succeeded"
	outcomeFailed      = "failed"
	outcomeCapacity    = "capacity"
	outcomeRejected    = "rejected"
	outcomeUnavailable = "unavailable"
)

// Metrics reports sandbox lifecycle outcomes for the first-party
// `/v1/sandboxes` surface (w5/m95).
//
// Scope is deliberately that surface alone. Agent-session sandboxes share this
// runtime but reach it through their own dispatch path, and they already have
// `bex_agent_session_provision_seconds` plus AgentSessionProvisionFailing. Were
// both counted here, one substrate incident would fire two alerts and neither
// series would answer "is the sandbox API itself healthy" — the question
// ADR088's coverage table had no signal for.
//
// A nil *Metrics is a working no-op, so a Service built without a registry
// (every test that does not care) needs no special casing.
type Metrics struct {
	creates    *prometheus.CounterVec
	createTime *prometheus.HistogramVec
	terminates *prometheus.CounterVec
}

func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		creates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bex_sandbox_create_total",
			Help: "Sandbox creations through /v1/sandboxes by bounded outcome; excludes agent-session sandboxes, which report their own provisioning.",
		}, []string{"outcome"}),
		createTime: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name: "bex_sandbox_create_seconds",
			Help: "Wall-clock of a /v1/sandboxes create call by outcome. Measures the create verb, which returns once the runtime accepts the sandbox — not time to Running, which this service never observes.",
			// Matches the agent-session provisioning buckets so the two read on
			// one axis: warm start is single-digit seconds, a cold image pull
			// ~23s (ADR059), and the platform pod-ready wait bounds the tail.
			Buckets: []float64{1, 2, 5, 10, 20, 30, 60, 120, 300},
		}, []string{"outcome"}),
		terminates: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bex_sandbox_terminate_total",
			Help: "Sandbox terminations through /v1/sandboxes by bounded outcome.",
		}, []string{"outcome"}),
	}
	reg.MustRegister(m.creates, m.createTime, m.terminates)
	return m
}

// observeCreate records one create attempt. Duration is only meaningful for a
// call that reached the runtime, so a refusal contributes to the counter but
// not the histogram — otherwise a burst of instant 400s would drag the p95 down
// and hide a genuinely slow substrate.
func (m *Metrics) observeCreate(err error, d time.Duration) {
	if m == nil {
		return
	}
	outcome := createOutcome(err)
	m.creates.WithLabelValues(outcome).Inc()
	if outcome == outcomeSucceeded || outcome == outcomeFailed || outcome == outcomeCapacity {
		m.createTime.WithLabelValues(outcome).Observe(d.Seconds())
	}
}

func (m *Metrics) observeTerminate(err error) {
	if m == nil {
		return
	}
	m.terminates.WithLabelValues(createOutcome(err)).Inc()
}

// createOutcome maps an error onto the closed label set. Order matters: the
// specific refusals are tested before the catch-all, so only a genuinely
// unexplained error reads as a substrate failure.
func createOutcome(err error) string {
	switch {
	case err == nil:
		return outcomeSucceeded
	case IsCapacityLimit(err):
		return outcomeCapacity
	case errors.Is(err, core.ErrSandboxesUnavailable):
		return outcomeUnavailable
	case errors.Is(err, core.ErrBadRequest),
		errors.Is(err, core.ErrForbidden),
		errors.Is(err, core.ErrNotFound),
		errors.Is(err, core.ErrConflict),
		errors.Is(err, core.ErrPaymentRequired),
		errors.Is(err, core.ErrBillingEnforced):
		return outcomeRejected
	default:
		return outcomeFailed
	}
}
