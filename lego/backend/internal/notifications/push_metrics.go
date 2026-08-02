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

package notifications

import (
	"time"

	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/prometheus/client_golang/prometheus"
)

type PushMetrics struct {
	enabled     prometheus.Gauge
	operations  *prometheus.CounterVec
	queue       *prometheus.GaugeVec
	lastSuccess prometheus.Gauge
}

func NewPushMetrics(r prometheus.Registerer) *PushMetrics {
	m := &PushMetrics{
		enabled:     prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "bex", Subsystem: "push", Name: "enabled", Help: "Whether push transport is configured."}),
		operations:  prometheus.NewCounterVec(prometheus.CounterOpts{Namespace: "bex", Subsystem: "push", Name: "operations_total", Help: "Push operations by bounded operation and result."}, []string{"operation", "result"}),
		queue:       prometheus.NewGaugeVec(prometheus.GaugeOpts{Namespace: "bex", Subsystem: "push", Name: "queue_rows", Help: "Push queue rows by bounded state."}, []string{"state"}),
		lastSuccess: prometheus.NewGauge(prometheus.GaugeOpts{Namespace: "bex", Subsystem: "push", Name: "last_success_timestamp_seconds", Help: "Last successful provider operation."}),
	}
	r.MustRegister(m.enabled, m.operations, m.queue, m.lastSuccess)
	return m
}
func (m *PushMetrics) SetEnabled(v bool) {
	if m == nil {
		return
	}
	if v {
		m.enabled.Set(1)
	} else {
		m.enabled.Set(0)
	}
}
func (m *PushMetrics) Operation(op, result string) {
	if m != nil {
		m.operations.WithLabelValues(op, result).Inc()
	}
}
func (m *PushMetrics) SetQueue(v store.PushQueueStats) {
	if m == nil {
		return
	}
	m.queue.WithLabelValues("send_pending").Set(float64(v.Pending))
	m.queue.WithLabelValues("receipt_pending").Set(float64(v.ReceiptPending))
	m.queue.WithLabelValues("terminal").Set(float64(v.Terminal))
}
func (m *PushMetrics) Succeeded(at time.Time) {
	if m != nil {
		m.lastSuccess.Set(float64(at.Unix()))
	}
}
