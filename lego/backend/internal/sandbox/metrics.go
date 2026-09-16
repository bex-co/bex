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
// `/v1/sandboxes` surface (w5/m95) plus the inventory/orphan gauges that keep
// a sandbox from outliving every session that owned it (w5/m99).
//
// Scope for create/terminate counters is deliberately the public surface alone.
// Agent-session sandboxes share this runtime but reach it through their own
// dispatch path, and they already have `bex_agent_session_provision_seconds`
// plus AgentSessionProvisionFailing. Were both counted here, one substrate
// incident would fire two alerts and neither series would answer "is the
// sandbox API itself healthy" — the question ADR088's coverage table had no
// signal for.
//
// Inventory gauges and teardown-failure counters span every live OpenSandbox
// the reconcile sees (including agent-session sandboxes), because the cost of
// an orphan is independent of which API created it.
//
// A nil *Metrics is a working no-op, so a Service built without a registry
// (every test that does not care) needs no special casing.
type Metrics struct {
	creates          *prometheus.CounterVec
	createTime       *prometheus.HistogramVec
	terminates       *prometheus.CounterVec
	live             *prometheus.GaugeVec
	oldestAge        *prometheus.GaugeVec
	teardownFailures *prometheus.CounterVec
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
		live: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bex_sandbox_live",
			Help: "Live sandboxes by inventory class after one reconcile pass (claimed, terminal_orphan, no_row_orphan, timed, inconclusive).",
		}, []string{"class"}),
		oldestAge: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Name: "bex_sandbox_oldest_age_seconds",
			Help: "Age in seconds of the oldest live sandbox in each inventory class.",
		}, []string{"class"}),
		teardownFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bex_sandbox_teardown_failures_total",
			Help: "Durable sandbox teardown failures by bounded reason (previous_sandbox, idle_reap, inventory).",
		}, []string{"reason"}),
	}
	reg.MustRegister(m.creates, m.createTime, m.terminates, m.live, m.oldestAge, m.teardownFailures)
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

// SetInventory replaces the live/oldest gauges with one reconcile pass's
// classification. Full recount each tick keeps the gauges truthful when a
// class drops to zero.
func (m *Metrics) SetInventory(counts InventoryCounts) {
	if m == nil {
		return
	}
	classes := []struct {
		name  string
		count int
	}{
		{InventoryClassClaimed, counts.Claimed},
		{InventoryClassTerminalOrphan, counts.TerminalOrphan},
		{InventoryClassNoRowOrphan, counts.NoRowOrphan},
		{InventoryClassTimed, counts.Timed},
		{InventoryClassInconclusive, counts.Inconclusive},
	}
	for _, c := range classes {
		m.live.WithLabelValues(c.name).Set(float64(c.count))
		m.oldestAge.WithLabelValues(c.name).Set(counts.OldestAge[c.name])
	}
}

// ObserveTeardownFailure counts a durable teardown that did not confirm.
func (m *Metrics) ObserveTeardownFailure(reason string) {
	if m == nil {
		return
	}
	switch reason {
	case TeardownReasonPreviousSandbox, TeardownReasonIdleReap, TeardownReasonInventory:
	default:
		reason = "failed"
	}
	m.teardownFailures.WithLabelValues(reason).Inc()
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
