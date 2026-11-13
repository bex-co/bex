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

package store

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ReconcilerMetrics makes the reconciler's refused conclusions a readable
// signal instead of an invisible suppression (w6/m41): a guard that silently
// drops observations is one incident away from being blamed for a missed
// outage. The healthy shape is zero in steady state, climbing during a
// control-plane incident (informer staleness is exactly the condition that
// produces time-traveled conclusions).
//
// It also times every reconcile pass and counts what started it (w5/m369), so
// the cost of reacting to cluster status is measured, not guessed: a pass that
// runs on every App status change is only safe while one pass stays cheap.
type ReconcilerMetrics struct {
	observationRejections *prometheus.CounterVec
	passSeconds           prometheus.Histogram
	kicks                 *prometheus.CounterVec
	watchFailures         prometheus.Counter
}

// The closed vocabulary of ReconcilerMetrics.Kick's source label: an API write
// (the apps, deploys and workspaces services), a change the App status watch
// saw in the cluster, or the unhealthy debounce's confirming pass.
type kickSource string

const (
	kickSourceAPI     kickSource = "api"
	kickSourceWatch   kickSource = "watch"
	kickSourceConfirm kickSource = "confirm"
)

func NewReconcilerMetrics(registerer prometheus.Registerer) *ReconcilerMetrics {
	m := &ReconcilerMetrics{
		observationRejections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "controlplane", Name: "observed_state_rejections_total",
			Help: "Observed service-state conclusions the reconciler refused to record, by bounded reason.",
		}, []string{"reason", "subject"}),
	}
	m.passSeconds = prometheus.NewHistogram(prometheus.HistogramOpts{
		Namespace: "bex", Subsystem: "controlplane", Name: "reconcile_pass_seconds",
		Help:    "Duration of one App projection pass (ReconcileOnce), whatever started it.",
		Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2, 5, 10, 30, 60},
	})
	m.kicks = prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "bex", Subsystem: "controlplane", Name: "reconcile_kicks_total",
		Help: "Requests for an immediate App projection pass, by source (api, watch, confirm). Kicks coalesce, so passes run at most once per kick.",
	}, []string{"source"})
	m.watchFailures = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "bex", Subsystem: "controlplane", Name: "app_status_watch_failures_total",
		Help: "Times the App status watch failed (an error or an expired resource version) and was restarted after a wait. The apiserver's routine watch timeouts are not counted.",
	})
	registerer.MustRegister(m.observationRejections, m.passSeconds, m.kicks, m.watchFailures)
	return m
}

// Pass records one reconcile pass's duration.
func (m *ReconcilerMetrics) Pass(d time.Duration) {
	if m != nil {
		m.passSeconds.Observe(d.Seconds())
	}
}

// kick counts one request for an immediate pass from source.
func (m *ReconcilerMetrics) kick(source kickSource) {
	if m != nil {
		m.kicks.WithLabelValues(string(source)).Inc()
	}
}

// WatchFailure counts one failed App status watch.
func (m *ReconcilerMetrics) WatchFailure() {
	if m != nil {
		m.watchFailures.Inc()
	}
}

// Rejection counts one refused conclusion. Both labels come from closed
// vocabularies — reason from rejectReason*, subject from rejectSubject* —
// never anything tenant-derived, so label cardinality stays bounded regardless
// of fleet size. subject separates the App observation path from the managed-
// datastore one (w3/m82): they share the guard but not the incident.
func (m *ReconcilerMetrics) Rejection(reason, subject string) {
	if m != nil {
		m.observationRejections.WithLabelValues(reason, subject).Inc()
	}
}
