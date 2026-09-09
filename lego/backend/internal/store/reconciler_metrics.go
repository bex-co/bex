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

import "github.com/prometheus/client_golang/prometheus"

// ReconcilerMetrics makes the reconciler's refused conclusions a readable
// signal instead of an invisible suppression (w6/m41): a guard that silently
// drops observations is one incident away from being blamed for a missed
// outage. The healthy shape is zero in steady state, climbing during a
// control-plane incident (informer staleness is exactly the condition that
// produces time-traveled conclusions).
type ReconcilerMetrics struct {
	observationRejections *prometheus.CounterVec
}

func NewReconcilerMetrics(registerer prometheus.Registerer) *ReconcilerMetrics {
	m := &ReconcilerMetrics{
		observationRejections: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "controlplane", Name: "observed_state_rejections_total",
			Help: "Observed service-state conclusions the reconciler refused to record, by bounded reason.",
		}, []string{"reason", "subject"}),
	}
	registerer.MustRegister(m.observationRejections)
	return m
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
