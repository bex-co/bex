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

package billing

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Metrics contains only account-wide gauges and counters with two bounded
// labels (operation/result). Workspace, Stripe object, transaction, invoice,
// payment, request, and secret values never become labels.
type Metrics struct {
	enabled          prometheus.Gauge
	operations       *prometheus.CounterVec
	pendingRows      prometheus.Gauge
	oldestPendingAge prometheus.Gauge
	rejectedRows     prometheus.Gauge
	ambiguousRows    prometheus.Gauge
	lastWebhook      prometheus.Gauge
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		enabled: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "billing", Name: "enabled",
			Help: "Whether the external Stripe billing sink is configured (1) or estimate-only (0).",
		}),
		operations: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "billing", Name: "operations_total",
			Help: "Billing operations by bounded operation and result.",
		}, []string{"operation", "result"}),
		pendingRows: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "billing", Name: "outbox_pending_rows",
			Help: "Usage rows awaiting a final Stripe export outcome.",
		}),
		oldestPendingAge: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "billing", Name: "outbox_oldest_pending_age_seconds",
			Help: "Age of the oldest pending usage row in seconds.",
		}),
		rejectedRows: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "billing", Name: "export_rejected_rows",
			Help: "Usage rows held after a permanent provider rejection.",
		}),
		ambiguousRows: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "billing", Name: "export_ambiguous_rows",
			Help: "Usage rows held after an attempted export exceeded the provider deduplication window.",
		}),
		lastWebhook: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "billing", Name: "webhook_last_success_timestamp_seconds",
			Help: "Unix timestamp of the last signature, API-version, and mode-valid Stripe webhook.",
		}),
	}
	registerer.MustRegister(m.enabled, m.operations, m.pendingRows, m.oldestPendingAge, m.rejectedRows, m.ambiguousRows, m.lastWebhook)
	m.enabled.Set(0)
	return m
}

func (m *Metrics) SetEnabled(enabled bool) {
	if m == nil {
		return
	}
	if enabled {
		m.enabled.Set(1)
		return
	}
	m.enabled.Set(0)
}

func (m *Metrics) Operation(operation, result string) {
	if m != nil {
		m.operations.WithLabelValues(operation, result).Inc()
	}
}

func (m *Metrics) SetExportStats(stats store.BillingExportStats) {
	if m == nil {
		return
	}
	m.pendingRows.Set(float64(stats.PendingRows))
	m.oldestPendingAge.Set(stats.OldestPendingAge.Seconds())
	m.rejectedRows.Set(float64(stats.RejectedRows))
	m.ambiguousRows.Set(float64(stats.AmbiguousRows))
}

func (m *Metrics) WebhookSucceeded(at time.Time) {
	if m != nil {
		m.lastWebhook.Set(float64(at.UTC().Unix()))
		m.Operation("webhook", "success")
	}
}
