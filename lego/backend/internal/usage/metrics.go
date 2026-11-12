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

package usage

import (
	"context"
	"log"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/store"
)

// Metrics publishes the rollup's own health: when the oldest hour a meter can
// still write but has not began, per coverage source (w6/m334). It is a start
// time, not an age, so an alert on time() minus it keeps ageing when bex-api
// stops publishing.
type Metrics struct {
	oldestUnmetered *prometheus.GaugeVec
}

func NewMetrics(registerer prometheus.Registerer) *Metrics {
	m := &Metrics{
		oldestUnmetered: prometheus.NewGaugeVec(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "usage", Name: "oldest_unmetered_hour_timestamp_seconds",
			Help: "Start, in Unix seconds, of the oldest hour per coverage source that the rollup can still meter but has not; 0 when every hour inside the 48 h catch-up window is metered. An hour past that window is skipped for good.",
		}, []string{"source"}),
	}
	registerer.MustRegister(m.oldestUnmetered)
	return m
}

// setOldestUnmetered sets every source, so a source whose hole was metered
// reads 0 again rather than keeping its last start.
func (m *Metrics) setOldestUnmetered(oldest map[string]time.Time) {
	for _, source := range store.UsageStreamSources() {
		start := 0.0
		if hole, ok := oldest[source]; ok {
			start = float64(hole.Unix())
		}
		m.oldestUnmetered.WithLabelValues(source).Set(start)
	}
}

// unmeteredReadTimeout bounds the fleet-wide read so a wedged query cannot
// hold the rollup and compaction ticks behind it.
const unmeteredReadTimeout = 30 * time.Second

// publishUnmeteredHours refreshes the unmetered-hour starts. A failed read
// keeps the last values and logs: the alert ages them from time(), so a hole
// already published still pages.
func (s *Service) publishUnmeteredHours(ctx context.Context) {
	if s.Metrics == nil {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, unmeteredReadTimeout)
	defer cancel()
	oldest, err := s.Store.OldestUnmeteredUsageHours(ctx, s.Now().UTC())
	if err != nil {
		log.Printf("usage: read oldest unmetered hours: %v", err)
		return
	}
	s.Metrics.setOldestUnmetered(oldest)
}
