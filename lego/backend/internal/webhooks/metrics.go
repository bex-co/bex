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

package webhooks

import (
	"errors"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Metrics records process-wide webhook outcomes using closed, low-cardinality
// labels. Endpoint, attempt, event, actor, URL, payload, and idempotency values
// never become metric labels.
type Metrics struct {
	attempts    *prometheus.CounterVec
	resends     *prometheus.CounterVec
	admissions  *prometheus.CounterVec
	cappedBatch prometheus.Histogram
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		attempts: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "webhooks", Name: "delivery_attempts_total",
			Help: "Completed webhook delivery attempts by automatic/manual origin and terminal result.",
		}, []string{"origin", "result"}),
		resends: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "webhooks", Name: "resend_requests_total",
			Help: "Manual webhook Resend requests by bounded service result.",
		}, []string{"result"}),
		admissions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "webhooks", Name: "delivery_admissions_total",
			Help: "Webhook logical-delivery queue admission outcomes.",
		}, []string{"result"}),
		cappedBatch: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "webhooks", Name: "delivery_capped_batch_size",
			Help:    "Number of webhook deliveries capped by one committed dispatcher page.",
			Buckets: []float64{1, 10, 50, 100, 500, 1000, 5000},
		}),
	}
	registerer.MustRegister(m.attempts, m.resends, m.admissions, m.cappedBatch)
	return m
}

// ObserveWebhookAdmission implements AdmissionObserver. The result has only
// three fixed counters and carries no tenant-controlled dimensions.
func (m *Metrics) ObserveWebhookAdmission(result store.WebhookEnqueueResult) {
	if m == nil {
		return
	}
	m.observeAdmissionCount("admitted", result.Admitted)
	m.observeAdmissionCount("capped", result.Capped)
	m.observeAdmissionCount("deduplicated", result.Deduplicated)
	if result.Capped > 0 {
		m.cappedBatch.Observe(float64(result.Capped))
	}
}

func (m *Metrics) observeAdmissionCount(label string, count int) {
	if count > 0 {
		m.admissions.WithLabelValues(label).Add(float64(count))
	}
}

// ObserveWebhookAttempt implements AttemptObserver. Unknown values collapse to
// one fixed bucket so a future store value cannot create an unbounded label.
func (m *Metrics) ObserveWebhookAttempt(origin, result string) {
	if m == nil {
		return
	}
	switch origin {
	case store.WebhookAttemptAutomatic, store.WebhookAttemptManual:
	default:
		origin = "unknown"
	}
	switch result {
	case store.WebhookAttemptDelivered, store.WebhookAttemptFailed:
	default:
		result = "unknown"
	}
	m.attempts.WithLabelValues(origin, result).Inc()
}

func (m *Metrics) observeResend(err error) {
	if m == nil {
		return
	}
	result := "queued"
	if err != nil {
		result = "error"
		var coded *core.CodedError
		switch {
		case errors.As(err, &coded):
			switch coded.Code {
			case WebhookResendIdempotencyKeyInvalidCode:
				result = "invalid"
			case WebhookEndpointNotFoundCode, WebhookDeliveryNotFoundCode:
				result = "not_found"
			case WebhookEndpointDisabledCode:
				result = "disabled"
			case WebhookDeliveryPendingCode:
				result = "pending"
			}
		case errors.Is(err, core.ErrForbidden):
			result = "denied"
		case errors.Is(err, core.ErrUnavailable):
			result = "unavailable"
		}
	}
	m.resends.WithLabelValues(result).Inc()
}
