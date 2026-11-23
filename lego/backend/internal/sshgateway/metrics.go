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
	handshakes                 *prometheus.CounterVec
	authentications            *prometheus.CounterVec
	activeSessions             prometheus.Gauge
	sessions                   *prometheus.CounterVec
	durations                  *prometheus.HistogramVec
	limitRejections            *prometheus.CounterVec
	channels                   prometheus.Counter
	activeChannels             prometheus.Gauge
	reauths                    *prometheus.CounterVec
	sessionWakes               *prometheus.CounterVec
	twoFactor                  *prometheus.CounterVec
	gitUpstream                *prometheus.CounterVec
	modelUsage                 *prometheus.CounterVec
	egressConns                *prometheus.CounterVec
	egressBytes                *prometheus.CounterVec
	sandboxBrokerRequests      *prometheus.CounterVec
	sandboxBrokerOverlaps      prometheus.Counter
	sandboxBrokerLookupSeconds prometheus.Histogram
	sandboxBrokerCache         *prometheus.CounterVec
	drainBuffered              prometheus.Gauge
	drainFallbacks             prometheus.Counter
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
			Help: "Native SSH reassertions of the transport-auth-time key + target authorization (codex round-8 #5), at each channel open and revalidation tick, by bounded result: accepted, rejected (a definite no), or unavailable (no answer; a live session retries it, w6/147).",
		}, []string{"result"}),
		sessionWakes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "session_wakes_total",
			Help: "Native SSH logins into an agent session whose sandbox was closed or starting (w10/m597), once each, by bounded result: refused (the caller may not start it), refused_start (bex-api refused the start: admission, or the session's state), unavailable (no answer from bex-api; the login got the banner), opened (the sandbox came up), no_sandbox (the start ended without one), timed_out (the start's ceiling passed), abandoned (the connection, its authorization or the gateway ended first). No session, workspace or subject label.",
		}, []string{"result"}),
		twoFactor: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "two_factor_refusals_total",
			Help: "Gateway authorizations (native SSH, web shell, agent attach, sandbox exec and files) refused by a workspace's login requirements, by bounded result: not_enrolled (two-factor), not_google (Google login, w7/m465), or unavailable (bex-api could not answer, failed closed). No workspace or subject label.",
		}, []string{"result"}),
		gitUpstream: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "git_proxy_upstream_failures_total",
			Help: "Agent-session Git smart-HTTP exchanges that were admitted and authorized but failed on the gateway->forge hop, by bounded cause.",
		}, []string{"cause"}),
		modelUsage: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "model_proxy_usage_reports_total",
			Help: "Platform model route token-usage reports (w10/m14) by bounded result: reported, retried, unparsed, dropped_queue_full, dropped_failed.",
		}, []string{"result"}),
		egressConns: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "egress_proxy_connections_total",
			Help: "Agent-session egress proxy requests (w10/m19) by bounded result: allowed, or the refusal reason (provider, private, port, tier, sni, dns, dial, budget, ...). No host, URL or session label.",
		}, []string{"result"}),
		egressBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "egress_proxy_bytes_total",
			Help: "Bytes relayed by the agent-session egress proxy, by direction (upstream = sandbox to internet, downstream = internet to sandbox).",
		}, []string{"direction"}),
		sandboxBrokerRequests: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "sandbox_broker_requests_total",
			Help: "Sandbox credential broker requests (w2/m534) by closed outcome: injected, passthrough, denied, or error. Zero host, path, secret, or sandbox labels.",
		}, []string{"outcome"}),
		sandboxBrokerOverlaps: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "sandbox_broker_overlaps_total",
			Help: "Sandbox credential broker requests where multiple enabled credentials matched and the deterministic lexicographical-ID winner was injected.",
		}),
		sandboxBrokerLookupSeconds: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "sandbox_broker_lookup_duration_seconds",
			Help:    "Latency of gateway->bex-api internal sandbox credential broker lookups.",
			Buckets: prometheus.DefBuckets,
		}),
		sandboxBrokerCache: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "sandbox_broker_cache_total",
			Help: "Sandbox credential broker lookup cache accesses by result: hit or miss.",
		}, []string{"result"}),
		// w6/101: the exec drain buffer (w6/099) holding output read ahead of
		// a slow SSH client, and the writes that went straight through because
		// the process-wide budget was spent, which is when kube-apiserver's
		// slow-reader tail loss can return.
		drainBuffered: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "drain_buffered_bytes",
			Help: "Exec output bytes the gateway currently holds ahead of slow SSH clients, process-wide.",
		}),
		drainFallbacks: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "ssh_gateway", Name: "drain_fallback_writes_total",
			Help: "Exec output writes forwarded unbuffered because the process-wide drain budget was spent.",
		}),
	}
	registerer.MustRegister(m.handshakes, m.authentications, m.activeSessions, m.sessions, m.durations, m.limitRejections, m.channels, m.activeChannels, m.reauths, m.sessionWakes, m.twoFactor, m.gitUpstream, m.modelUsage, m.egressConns, m.egressBytes, m.sandboxBrokerRequests, m.sandboxBrokerOverlaps, m.sandboxBrokerLookupSeconds, m.sandboxBrokerCache, m.drainBuffered, m.drainFallbacks)
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

// SessionWake records how one login into an agent session with no sandbox to
// open ended (w10/m597): a refusal at login, or its wait's outcome.
func (m *Metrics) SessionWake(result string) {
	if m != nil {
		m.sessionWakes.WithLabelValues(result).Inc()
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

// Reauthorization records one reassertion outcome ("accepted", "rejected" or
// "unavailable") — the transport may live for hours, so each channel and each
// revalidation tick re-checks the key and the authorization that admitted the
// transport (codex round-8 #5).
func (m *Metrics) Reauthorization(result string) {
	if m != nil {
		m.reauths.WithLabelValues(result).Inc()
	}
}

// The refusal results of the gateway's workspace login-requirements check, the
// whole vocabulary of the two_factor_refusals_total result label.
const (
	twoFactorNotEnrolled = "not_enrolled"
	twoFactorNotGoogle   = "not_google"
	twoFactorUnavailable = "unavailable"
)

// TwoFactorRefusal records one authorization the gateway refused because of a
// workspace's login requirements: twoFactorNotEnrolled for a member with no
// second factor, twoFactorNotGoogle for one not signed in with Google (or a
// key not created so), twoFactorUnavailable when bex-api could not answer
// (failed closed).
func (m *Metrics) TwoFactorRefusal(result string) {
	if m != nil {
		m.twoFactor.WithLabelValues(result).Inc()
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

// ModelUsageReport records one platform model route usage-report outcome
// (w10/m14 t003). Any dropped_* or unparsed count is unmetered platform spend
// and should alert.
func (m *Metrics) ModelUsageReport(result string) {
	if m != nil {
		m.modelUsage.WithLabelValues(result).Inc()
	}
}

// EgressProxyConnection records one egress proxy request outcome (w10/m19):
// "allowed" or a bounded refusal reason. Hosts and URLs never become labels.
func (m *Metrics) EgressProxyConnection(result string) {
	if m != nil {
		m.egressConns.WithLabelValues(result).Inc()
	}
}

// EgressProxyBytes adds n relayed bytes in direction ("upstream" sandbox to
// internet, "downstream" internet to sandbox).
func (m *Metrics) EgressProxyBytes(direction string, n int64) {
	if m != nil && n > 0 {
		m.egressBytes.WithLabelValues(direction).Add(float64(n))
	}
}

// SandboxBrokerRequest records one sandbox credential broker outcome
// ("injected", "passthrough", "denied", or "error"). Closed labels only.
func (m *Metrics) SandboxBrokerRequest(outcome string) {
	if m != nil {
		m.sandboxBrokerRequests.WithLabelValues(outcome).Inc()
	}
}

// SandboxBrokerOverlap counts one request where multiple enabled group
// credentials matched and deterministic tie-breaking selected one winner.
func (m *Metrics) SandboxBrokerOverlap() {
	if m != nil {
		m.sandboxBrokerOverlaps.Inc()
	}
}

// SandboxBrokerLookupDuration observes one bex-api internal broker lookup.
func (m *Metrics) SandboxBrokerLookupDuration(d time.Duration) {
	if m != nil {
		m.sandboxBrokerLookupSeconds.Observe(d.Seconds())
	}
}

// SandboxBrokerCache records one lookup cache "hit" or "miss".
func (m *Metrics) SandboxBrokerCache(hit bool) {
	if m != nil {
		if hit {
			m.sandboxBrokerCache.WithLabelValues("hit").Inc()
		} else {
			m.sandboxBrokerCache.WithLabelValues("miss").Inc()
		}
	}
}

// DrainBuffered moves the process-wide drain buffer gauge by delta bytes.
func (m *Metrics) DrainBuffered(delta int64) {
	if m != nil {
		m.drainBuffered.Add(float64(delta))
	}
}

// DrainFallback counts one write forwarded unbuffered because the drain
// budget was spent.
func (m *Metrics) DrainFallback() {
	if m != nil {
		m.drainFallbacks.Inc()
	}
}
