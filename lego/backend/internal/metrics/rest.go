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

package metrics

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// rest.go is the REST metrics adapter — Render metrics-API compatible. It maps
// Render's per-metric endpoints (/v1/metrics/cpu, .../memory, ...) and their
// query string onto the single Metrics verb, and renders the Render
// array-of-time-series shape.

// metricPaths maps Render's endpoint path segment to the bex metric id.
var metricPaths = map[string]string{
	"cpu":            MetricCPU,
	"memory":         MetricMemory,
	"instance-count": MetricInstanceCount,
	"http-requests":  MetricHTTPRequests,
	"http-latency":   MetricHTTPLatency,
	"bandwidth":      MetricBandwidth,
	// bex extensions (w3/m10): App-scoped, share the same Metrics verb/query shape.
	"cpu-target":    MetricCPUTarget,
	"memory-target": MetricMemoryTarget,
}

// datastoreMetricPaths maps a bex-extension path segment to the datastore
// metric id (w3/m10) — the Database/KeyValue-scoped sibling of metricPaths.
var datastoreMetricPaths = map[string]string{
	"disk":            MetricDisk,
	"disk-capacity":   MetricDiskCapacity,
	"db-connections":  MetricDBConnections,
	"replication-lag": MetricReplicationLag,
	"kv-memory":       MetricKVMemory,      // key-value only
	"kv-connections":  MetricKVConnections, // key-value only
}

// RegisterREST mounts the Render metrics endpoints plus bex's datastore-metric
// extensions.
func (s *Service) RegisterREST(mux *http.ServeMux) {
	for seg, metric := range metricPaths {
		mux.HandleFunc("GET /v1/metrics/"+seg, func(w http.ResponseWriter, r *http.Request) {
			s.metricQuery(w, r, metric)
		})
	}
	for seg, metric := range datastoreMetricPaths {
		mux.HandleFunc("GET /v1/metrics/"+seg, func(w http.ResponseWriter, r *http.Request) {
			s.datastoreMetricQuery(w, r, metric)
		})
	}
}

// metricQuery serves one Render metrics endpoint. Render's `resource` is an array
// of service ids; bex merges each App's series into one response.
func (s *Service) metricQuery(w http.ResponseWriter, r *http.Request, metric string) {
	resources, q, err := parseMetricParams(r)
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	// The window cap lives in the shared service (Metrics.checkWindow); only
	// the resource-array fan-out is adapter-shaped, so it is checked here.
	if err := checkFanOut(len(resources), 1, latencyFan(metric, q.Quantiles)); err != nil {
		core.WriteErr(w, err)
		return
	}
	q.Metric = metric

	var all []MetricSeries
	for _, res := range resources {
		q.App = res
		series, err := s.MetricsWithQuantiles(r.Context(), q)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		for _, ser := range series {
			all = append(all, ser.MetricSeries)
		}
	}
	core.WriteJSON(w, http.StatusOK, toRenderMetrics(all))
}

// datastoreMetricQuery serves one datastore-metric endpoint (disk/
// db-connections/replication-lag): a single Database/KeyValue resource per
// request (unlike Render's app metrics, a datastore metric names exactly one
// instance, mirroring /v1/postgres/{id} and /v1/key-value/{id}).
func (s *Service) datastoreMetricQuery(w http.ResponseWriter, r *http.Request, metric string) {
	q, err := parseDatastoreMetricParams(r)
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	q.Metric = metric
	series, err := s.DatastoreMetrics(r.Context(), q)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, toRenderMetrics(series))
}

// parseTimeWindow parses the startTime/endTime/resolutionSeconds query params
// shared by every metrics REST endpoint (App-scoped and datastore-scoped
// alike) into a MetricQuery's window fields.
func parseTimeWindow(v url.Values) (start, end time.Time, resolution time.Duration, err error) {
	if start, err = core.QueryTime(v, "startTime"); err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	if end, err = core.QueryTime(v, "endTime"); err != nil {
		return time.Time{}, time.Time{}, 0, err
	}
	if rs := v.Get("resolutionSeconds"); rs != "" {
		n, err := strconv.Atoi(rs)
		if err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("resolutionSeconds: %w", err)
		}
		resolution = time.Duration(n) * time.Second
	}
	return start, end, resolution, nil
}

// parseDatastoreMetricParams maps a datastore-metric query string onto a
// DatastoreMetricQuery. `kind` defaults to "database" — db-connections and
// replication-lag are Postgres-only anyway, and disk usage's other callers
// (KeyValue, and a service's attached disk) are the exception, not the common
// case.
func parseDatastoreMetricParams(r *http.Request) (DatastoreMetricQuery, error) {
	v := r.URL.Query()

	resource := v.Get("resource")
	if resource == "" {
		return DatastoreMetricQuery{}, fmt.Errorf("resource is required")
	}
	kind := v.Get("kind")
	if kind == "" {
		kind = DatastoreDatabase
	}

	start, end, resolution, err := parseTimeWindow(v)
	if err != nil {
		return DatastoreMetricQuery{}, err
	}
	return DatastoreMetricQuery{
		Kind: kind, Resource: resource,
		Start: start, End: end, Resolution: resolution,
	}, nil
}

// parseMetricParams maps Render's metrics query string onto resources + a
// MetricQuery.
func parseMetricParams(r *http.Request) ([]string, MetricQuery, error) {
	v := r.URL.Query()

	resources := v["resource"]
	if len(resources) == 0 {
		return nil, MetricQuery{}, fmt.Errorf("resource is required")
	}

	q := MetricQuery{
		StatusCode: v.Get("statusCode"),
		// host/path are parsed only so Metrics can refuse them (see MetricQuery.Host).
		Host:       v.Get("host"),
		Path:       v.Get("path"),
		GroupBy:    v.Get("groupBy"),
		Percentage: v.Get("percentage") == "true",
	}

	start, end, resolution, err := parseTimeWindow(v)
	if err != nil {
		return nil, MetricQuery{}, err
	}
	q.Start, q.End, q.Resolution = start, end, resolution

	// `quantile` may repeat: one value is the ordinary percentile pick, several
	// are the percentile "All" overlay (w5/m56) — returned as one series per
	// quantile, each carrying a `quantile` label. A lone value keeps the
	// single-quantile path byte-identical.
	for _, ql := range v["quantile"] {
		if ql == "" {
			continue
		}
		f, err := strconv.ParseFloat(ql, 64)
		if err != nil {
			return nil, MetricQuery{}, fmt.Errorf("quantile: %w", err)
		}
		q.Quantiles = append(q.Quantiles, f)
	}
	if len(q.Quantiles) == 1 {
		q.Quantile = q.Quantiles[0]
	}

	return resources, q, nil
}
