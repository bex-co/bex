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
	channels        prometheus.Counter
	activeChannels  prometheus.Gauge
	reauths         *prometheus.CounterVec
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
		channels: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "channels_total",
			Help: "Session channels accepted on multi-channel (agent-session sandbox) connections.",
		}),
		activeChannels: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "active_channels",
			Help: "Session channels (pods/exec streams) currently held open by the gateway, single- and multi-channel paths alike.",
		}),
		reauths: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "channel_reauthorizations_total",
			Help: "Per-channel reassertions of the transport-auth-time key + target authorization (codex round-8 #5) by bounded result.",
		}, []string{"result"}),
	}
	registerer.MustRegister(m.authentications, m.activeSessions, m.sessions, m.durations, m.limitRejections, m.channels, m.activeChannels, m.reauths)
	return m
}

func (m *Metrics) Authentication(result string) {
	if m != nil {
		m.authentications.WithLabelValues(result).Inc()
	}
}

func (m *Metrics) SessionStarted() {
	if m != nil {
		m.activeSessions.Inc()
	}
}

func (m *Metrics) SessionEnded(result string, elapsed time.Duration) {
	if m != nil {
		m.activeSessions.Dec()
		m.sessions.WithLabelValues(result).Inc()
		m.durations.WithLabelValues(result).Observe(elapsed.Seconds())
	}
}

func (m *Metrics) LimitRejected(scope string) {
	if m != nil {
		m.limitRejections.WithLabelValues(scope).Inc()
	}
}

func (m *Metrics) ChannelOpened() {
	if m != nil {
		m.channels.Inc()
		m.activeChannels.Inc()
	}
}

// ChannelClosed pairs with ChannelOpened once the channel's exec stream ends.
func (m *Metrics) ChannelClosed() {
	if m != nil {
		m.activeChannels.Dec()
	}
}

// Reauthorization records one per-channel reassertion outcome ("accepted" /
// "rejected") — the transport may live for hours, so each channel re-checks the
// key and the authorization that admitted the transport (codex round-8 #5).
func (m *Metrics) Reauthorization(result string) {
	if m != nil {
		m.reauths.WithLabelValues(result).Inc()
	}
}
