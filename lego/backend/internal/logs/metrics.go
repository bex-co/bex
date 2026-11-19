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

package logs

import (
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// Metrics times every history page read (REST GET /v1/logs, GraphQL logs, MCP
// list_logs, project views) by the shape of its query, so the shape that is
// slow can be told from the rest (w5/m503). Every label is a closed set the
// query is folded into: no resource, workspace or filter value becomes one.
type Metrics struct {
	pages *prometheus.HistogramVec
}

// NewMetrics registers the logs families on reg.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		pages: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Namespace: "bex", Subsystem: "api", Name: "logs_page_duration_seconds",
			Help: "Duration of one logs history page read (REST, GraphQL, MCP, project views) by query shape: " +
				"window (1h, 1d, 7d, 30d: the span read, up to), type (default, app, request, build, predeploy, mixed, other), " +
				"filter (none, label, line: path or host, text), direction, and result (ok, partial: the scan budget cut it, error).",
			Buckets: core.RequestDurationBuckets,
		}, []string{"window", "type", "filter", "direction", "result"}),
	}
	reg.MustRegister(m.pages)
	return m
}

// observePage records one page read of q that took took and ended in err (nil
// when it answered), partial when a scan budget cut its window.
func (m *Metrics) observePage(q LogQuery, partial bool, err error, took time.Duration) {
	if m == nil {
		return
	}
	result := "ok"
	switch {
	case err != nil:
		result = "error"
	case partial:
		result = "partial"
	}
	m.pages.WithLabelValues(windowClass(q), typeClass(q.Types), filterClass(q), lokiDirection(q), result).Observe(took.Seconds())
}

// windowClass is the span q reads, as the smallest of four classes that holds
// it. An absent start reads back lokiLookback, as the store does.
func windowClass(q LogQuery) string {
	since, end := resolveCursorWindow(q.Since, q.End)
	switch span := end.Sub(since); {
	case span <= time.Hour:
		return "1h"
	case span <= 24*time.Hour:
		return "1d"
	case span <= 7*24*time.Hour:
		return "7d"
	}
	return "30d"
}

func typeClass(types []string) string {
	switch len(types) {
	case 0:
		return "default"
	case 1:
		switch t := types[0]; t {
		case LogTypeApp, LogTypeRequest, LogTypeBuild, LogTypePreDeploy:
			return t
		}
		return "other"
	}
	return "mixed"
}

// filterClass is the costliest filter q carries: a text search (read raw and
// matched in bex-api), a line filter Loki applies (path, host), a label filter
// (a stream selector), or none.
func filterClass(q LogQuery) string {
	switch {
	case len(q.Search) > 0:
		return "text"
	case len(q.Path) > 0 || len(q.Host) > 0:
		return "line"
	case len(q.Level) > 0 || len(q.Instance) > 0 || len(q.StatusCode) > 0 || len(q.Method) > 0 || len(q.Build) > 0:
		return "label"
	}
	return "none"
}
