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

package managedoidc

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// Metrics meters the minter. A nil *Metrics records nothing.
type Metrics struct {
	failures prometheus.Counter
	oldest   prometheus.Gauge
}

// NewMetrics registers the minter's series on registerer.
func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		failures: prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "managed_oidc", Name: "refresh_failures_total",
			Help: "Managed OIDC token refreshes that failed (a mint the issuer refused, a Secret read or write the API server refused). The service keeps its previous token until it expires.",
		}),
		oldest: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "managed_oidc", Name: "oldest_token_age_seconds",
			Help: "Age, at the end of the last minter pass, of the oldest Managed OIDC token a running service reads; 0 when none. A token lives 3600 s and is replaced at 1200 s, so a value near 3600 means services are about to lose access.",
		}),
	}
	registerer.MustRegister(m.failures, m.oldest)
	return m
}

func (m *Metrics) failed() {
	if m != nil {
		m.failures.Inc()
	}
}

func (m *Metrics) observe(oldest time.Duration) {
	if m != nil {
		m.oldest.Set(oldest.Seconds())
	}
}
