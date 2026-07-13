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
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := checkQueryRange(s.MaxQueryHours, q.Start, q.End); err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	q.Metric = metric

	var all []MetricSeries
	for _, res := range resources {
		q.App = res
		series, err := s.Metrics(r.Context(), q)
		if err != nil {
			core.WriteErr(w, err)
			return
		}
		all = append(all, series...)
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
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	if err := checkQueryRange(s.MaxQueryHours, q.Start, q.End); err != nil {
		core.WriteJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
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

// checkQueryRange enforces MaxQueryHours (0 = unlimited) against a resolved
// start–end window — shared by metricQuery and datastoreMetricQuery so the
// REST cap can't drift between the two endpoint families.
func checkQueryRange(maxQueryHours int, start, end time.Time) error {
	if maxQueryHours <= 0 || start.IsZero() {
		return nil
	}
	if end.IsZero() {
		end = time.Now()
	}
	if end.Sub(start) > time.Duration(maxQueryHours)*time.Hour {
		return fmt.Errorf("query range exceeds %d hours", maxQueryHours)
	}
	return nil
}

// parseTimeWindow parses the startTime/endTime/resolutionSeconds query params
// shared by every metrics REST endpoint (App-scoped and datastore-scoped
// alike) into a MetricQuery's window fields.
func parseTimeWindow(v url.Values) (start, end time.Time, resolution time.Duration, err error) {
	if s := v.Get("startTime"); s != "" {
		if start, err = time.Parse(time.RFC3339, s); err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("startTime: %w", err)
		}
	}
	if e := v.Get("endTime"); e != "" {
		if end, err = time.Parse(time.RFC3339, e); err != nil {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("endTime: %w", err)
		}
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
// replication-lag are Postgres-only anyway, and disk usage's other caller
// (KeyValue) is the exception, not the common case.
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

	if ql := v.Get("quantile"); ql != "" {
		f, err := strconv.ParseFloat(ql, 64)
		if err != nil {
			return nil, MetricQuery{}, fmt.Errorf("quantile: %w", err)
		}
		q.Quantile = f
	}

	return resources, q, nil
}
