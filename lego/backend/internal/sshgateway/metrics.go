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
	handshakes      *prometheus.CounterVec
	authentications *prometheus.CounterVec
	activeSessions  prometheus.Gauge
	sessions        *prometheus.CounterVec
	durations       *prometheus.HistogramVec
	limitRejections *prometheus.CounterVec
	channels        prometheus.Counter
	activeChannels  prometheus.Gauge
	reauths         *prometheus.CounterVec
	gitUpstream     *prometheus.CounterVec
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		// Pre-authentication transport outcome. "established" means the SSH
		// version exchange + key exchange completed and the connection reached
		// authentication; "failed" means it never did (a malformed handshake, an
		// un-stripped PROXY header, or the peer hanging up before KEXINIT). An
		// infrastructure fault that kills the handshake for everyone — the w6/m132
		// regression — turns "failed" into ~100% of this counter while
		// authentications_total stays flat, so a dead edge is loud and is
		// distinguishable from an authorization refusal (which fails AFTER the
		// handshake, in authentications_total). It carries no subject, address, or
		// cause, so ADR035:106's non-disclosure of AUTHENTICATION causes is intact.
		handshakes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "handshakes_total",
			Help: "Pre-authentication SSH handshakes by bounded transport result.",
		}, []string{"result"}),
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
		gitUpstream: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "git_proxy_upstream_failures_total",
			Help: "Agent-session Git smart-HTTP exchanges that were admitted and authorized but failed on the gateway->forge hop, by bounded cause.",
		}, []string{"cause"}),
	}
	registerer.MustRegister(m.handshakes, m.authentications, m.activeSessions, m.sessions, m.durations, m.limitRejections, m.channels, m.activeChannels, m.reauths, m.gitUpstream)
	return m
}

// Handshake records one pre-authentication transport outcome ("established" /
// "failed"). "failed" is the honest, content-free signal that a connection
// never reached authentication — the loud, distinguishable pre-auth failure
// w6/m132 needed and that ADR035:106 does not govern (it covers only causes
// AFTER authentication begins).
func (m *Metrics) Handshake(result string) {
	if m != nil {
		m.handshakes.WithLabelValues(result).Inc()
	}
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

// GitProxyUpstreamFailure records one agent-session Git smart-HTTP exchange
// that passed admission and authorization but failed on the gateway→forge hop,
// by bounded cause ("mint" / "request" / "network" / "refused" /
// "response_cap" / "stream"). The sandbox
// deliberately sees only an undifferentiated 502 (upstream error bodies are
// never reflected), so this counter plus the paired gateway log line is where
// a broken upstream becomes loud (w5/m82).
func (m *Metrics) GitProxyUpstreamFailure(cause string) {
	if m != nil {
		m.gitUpstream.WithLabelValues(cause).Inc()
	}
}
