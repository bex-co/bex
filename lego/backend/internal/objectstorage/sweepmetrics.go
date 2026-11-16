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

package objectstorage

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// How a pass ended, the result label of the pass duration histogram.
const (
	passComplete = "complete" // every workspace was reached and none failed
	passFailures = "failures" // every workspace was reached, some failed
	passCutOff   = "cut_off"  // the deadline came first (PassTimeout or the interval)
)

// SweepMetrics is what the hourly sweep reports about itself (ADR112 D8), so how
// long a pass takes against its hour, and what a workspace costs, are read from
// Prometheus instead of guessed. Only the replica that holds the sweep's lock
// records; the others' series stay empty.
//
// The names are bex_usage_object_storage_sweep_*, beside the egress gauge, because
// the bex-api scrape job keeps only the families its keep-list names
// (deploy/gitops/base/prometheus.yaml) and bex_usage_ is one: a bex_object_storage_
// family was exported by the pods and stored by nothing.
type SweepMetrics struct {
	passDuration      *prometheus.HistogramVec
	workspaceDuration prometheus.Histogram
	workspaces        *prometheus.CounterVec
	lastComplete      prometheus.Gauge
	listedBytes       *prometheus.CounterVec
}

// NewSweepMetrics registers the sweep's series on reg.
func NewSweepMetrics(reg prometheus.Registerer) *SweepMetrics {
	m := &SweepMetrics{
		passDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "usage", Name: "object_storage_sweep_pass_duration_seconds",
			Help:    "Wall time of one hourly object storage sweep pass by how it ended: complete, failures (every workspace reached, some failed) or cut_off (the deadline came first; the next pass starts with the workspaces it did not reach). A pass has fifty minutes.",
			Buckets: []float64{1, 5, 15, 30, 60, 120, 300, 600, 1200, 1800, 2400, 3000},
		}, []string{"result"}),
		workspaceDuration: prometheus.NewHistogram(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "usage", Name: "object_storage_sweep_workspace_duration_seconds",
			Help:    "Wall time to sweep or purge one workspace: its listing in each region, its lifecycle deletes and the usage write. A workspace left fresh by another replica's pass is not observed.",
			Buckets: []float64{.1, .25, .5, 1, 2.5, 5, 15, 60, 180, 600, 1800},
		}),
		workspaces: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "usage", Name: "object_storage_sweep_workspaces_total",
			Help: "Workspaces by what their turn in a pass came to: swept (listed or purged), fresh (observed within the minimum gap, left alone), failed, or not_reached (the pass ended first).",
		}, []string{"result"}),
		lastComplete: prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: "bex", Subsystem: "usage", Name: "object_storage_sweep_last_complete_timestamp_seconds",
			Help: "Unix time the last pass that reached every workspace without a failure ended; 0 until one has.",
		}),
		listedBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Namespace: "bex", Subsystem: "usage", Name: "object_storage_sweep_listed_bytes_total",
			Help: "Estimated bytes of ListObjectsV2 responses the sweep has read from Wasabi, by region. Wasabi counts what a listing reads as egress against its free-egress ratio on the shared account (ADR112 D8), so this is the sweep's share of it; an estimate from the keys listed (about 250 bytes each), not a measured count.",
		}, []string{"region"}),
	}
	reg.MustRegister(m.passDuration, m.workspaceDuration, m.workspaces, m.lastComplete, m.listedBytes)
	return m
}

// observePass records one finished pass. A nil receiver records nothing.
func (m *SweepMetrics) observePass(took time.Duration, t passTally) {
	if m == nil {
		return
	}
	result := passComplete
	switch {
	case t.notReached > 0:
		result = passCutOff
	case t.failed > 0:
		result = passFailures
	}
	m.passDuration.WithLabelValues(result).Observe(took.Seconds())
	m.workspaces.WithLabelValues("swept").Add(float64(t.swept))
	m.workspaces.WithLabelValues("fresh").Add(float64(t.fresh))
	m.workspaces.WithLabelValues("failed").Add(float64(t.failed))
	m.workspaces.WithLabelValues("not_reached").Add(float64(t.notReached))
	if result == passComplete {
		m.lastComplete.SetToCurrentTime()
	}
}

// observeListed records the estimated bytes of one listing page read in region.
func (m *SweepMetrics) observeListed(region Region, bytes int64) {
	if m != nil {
		m.listedBytes.WithLabelValues(string(region)).Add(float64(bytes))
	}
}

// observeWorkspace records what sweeping one workspace cost.
func (m *SweepMetrics) observeWorkspace(took time.Duration) {
	if m != nil {
		m.workspaceDuration.Observe(took.Seconds())
	}
}
