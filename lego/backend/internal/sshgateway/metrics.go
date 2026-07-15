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

package sshgateway

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics exposes only bounded operational labels. Subjects, service ids,
// instance ids, remote addresses, commands, and terminal content are
// deliberately absent so observability cannot become a second audit stream.
type Metrics struct {
	authentications *prometheus.CounterVec
	activeSessions  prometheus.Gauge
	sessions        *prometheus.CounterVec
	durations       *prometheus.HistogramVec
	limitRejections *prometheus.CounterVec
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		authentications: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "authentications_total",
			Help: "Completed SSH public-key authentication attempts by bounded result.",
		}, []string{"result"}),
		activeSessions: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "active_sessions",
			Help: "Authenticated SSH connections currently held by the gateway.",
		}),
		sessions: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "sessions_total",
			Help: "SSH connections completed by bounded result.",
		}, []string{"result"}),
		durations: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "session_duration_seconds",
			Help:    "Duration of authenticated SSH connections by bounded result.",
			Buckets: prometheus.ExponentialBuckets(1, 4, 8),
		}, []string{"result"}),
		limitRejections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "limit_rejections_total",
			Help: "Authenticated SSH connections rejected by the configured session limit.",
		}, []string{"scope"}),
	}
	registerer.MustRegister(m.authentications, m.activeSessions, m.sessions, m.durations, m.limitRejections)
	return m
}

func (m *Metrics) authentication(result string) {
	if m != nil {
		m.authentications.WithLabelValues(result).Inc()
	}
}

func (m *Metrics) sessionStarted() {
	if m != nil {
		m.activeSessions.Inc()
	}
}

func (m *Metrics) sessionEnded(result string, elapsed time.Duration) {
	if m != nil {
		m.activeSessions.Dec()
		m.sessions.WithLabelValues(result).Inc()
		m.durations.WithLabelValues(result).Observe(elapsed.Seconds())
	}
}

func (m *Metrics) limitRejected(scope string) {
	if m != nil {
		m.limitRejections.WithLabelValues(scope).Inc()
	}
}
