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

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Blocked-dispatch reasons, the bounded label of a queued message that could
// not start its turn. Workspace and session ids are never labels.
const (
	blockedLiveLimit     = "live_limit"
	blockedTrustLimit    = "trust_limit"
	blockedRouteCapacity = "route_capacity"
	blockedBilling       = "billing"
	blockedOther         = "other"
)

// QueueMetrics reports the message queue (ADR047 D12) beside the
// Completer's turn metrics: how many messages wait, how long a settled
// session waits before its next message starts, and why dispatches block.
type QueueMetrics struct {
	depth           prometheus.Gauge
	dispatchLatency prometheus.Histogram
	blocked         *prometheus.CounterVec
	dropped         prometheus.Counter
}

// NewQueueMetrics registers the queue's series.
func NewQueueMetrics(reg prometheus.Registerer) *QueueMetrics {
	m := &QueueMetrics{
		depth: prometheus.NewGauge(prometheus.GaugeOpts{
			Name: "bex_agent_session_queued_messages",
			Help: "Messages waiting in agent-session queues, across every session.",
		}),
		dispatchLatency: prometheus.NewHistogram(prometheus.HistogramOpts{
			Name: "bex_agent_session_queue_dispatch_seconds",
			Help: "How long a queued message waited from the later of its session settling and its being queued until its turn started.",
			// The poll is 5 s and a blocked head is retried every 30 s; a long tail is a message blocked at dispatch.
			Buckets: []float64{1, 5, 10, 30, 60, 300, 900, 3600},
		}),
		blocked: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "bex_agent_session_queue_blocked_total",
			Help: "Queued-message dispatch attempts that did not start a turn, by bounded reason.",
		}, []string{"reason"}),
		dropped: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "bex_agent_session_queue_dropped_total",
			Help: "Queued messages dropped at dispatch because no later attempt could run them.",
		}),
	}
	reg.MustRegister(m.depth, m.dispatchLatency, m.blocked, m.dropped)
	return m
}

func (m *QueueMetrics) setDepth(n int) {
	if m != nil {
		m.depth.Set(float64(n))
	}
}

func (m *QueueMetrics) observeDispatch(wait time.Duration) {
	if m != nil && wait >= 0 {
		m.dispatchLatency.Observe(wait.Seconds())
	}
}

func (m *QueueMetrics) observeBlocked(reason string) {
	if m != nil {
		m.blocked.WithLabelValues(reason).Inc()
	}
}

func (m *QueueMetrics) observeDropped() {
	if m != nil {
		m.dropped.Inc()
	}
}
