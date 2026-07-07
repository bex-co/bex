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

package api

import (
	"fmt"
	"net/http"
	"strconv"
	"time"
)

// rest_metrics.go is the REST metrics adapter — Render metrics-API compatible. It
// maps Render's per-metric endpoints (/v1/metrics/cpu, .../memory,
// .../instance-count, .../http-requests, .../http-latency, .../bandwidth) and
// their query string (resource/startTime/endTime/resolutionSeconds/quantile +
// the status/host/path filters) onto the single Core.Metrics verb, and renders
// the Render array-of-time-series shape. Like the rest of the REST adapter it
// holds no logic beyond routing + Render (de)serialization.

// metricPaths maps Render's endpoint path segment to the bex metric id. Served,
// like the service routes, under both /v1/metrics (Render) — bex adds no second
// noun since "metrics" is already generic.
var metricPaths = map[string]string{
	"cpu":            MetricCPU,
	"memory":         MetricMemory,
	"instance-count": MetricInstanceCount,
	"http-requests":  MetricHTTPRequests,
	"http-latency":   MetricHTTPLatency,
	"bandwidth":      MetricBandwidth,
}

func (s *Server) registerMetricRoutes(mux *http.ServeMux) {
	for seg, metric := range metricPaths {
		mux.HandleFunc("GET /v1/metrics/"+seg, func(w http.ResponseWriter, r *http.Request) {
			s.metricQuery(w, r, metric)
		})
	}
}

// metricQuery serves one Render metrics endpoint. Render's `resource` is an array
// (repeatable) of service ids; bex merges each App's series into one response,
// tagging cross-resource results by their `resource` label (already set by Core).
func (s *Server) metricQuery(w http.ResponseWriter, r *http.Request, metric string) {
	resources, q, err := parseMetricParams(r)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	q.Metric = metric

	var all []MetricSeries
	for _, res := range resources {
		q.App = res
		series, err := s.Core.Metrics(r.Context(), q)
		if err != nil {
			writeErr(w, err)
			return
		}
		all = append(all, series...)
	}
	writeJSON(w, http.StatusOK, toRenderMetrics(all))
}

// parseMetricParams maps Render's metrics query string onto resources + a
// MetricQuery. resource is required (repeatable, the App id); startTime/endTime
// are RFC3339; resolutionSeconds is the step; quantile (0..1) picks the latency
// percentile; statusCode/host/path are request filters; groupBy chooses the
// series breakdown; percentage (a bex extra) switches cpu/memory to a fraction of
// the pod limit.
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

	if s := v.Get("startTime"); s != "" {
		t, err := time.Parse(time.RFC3339, s)
		if err != nil {
			return nil, MetricQuery{}, fmt.Errorf("startTime: %w", err)
		}
		q.Start = t
	}
	if e := v.Get("endTime"); e != "" {
		t, err := time.Parse(time.RFC3339, e)
		if err != nil {
			return nil, MetricQuery{}, fmt.Errorf("endTime: %w", err)
		}
		q.End = t
	}
	if rs := v.Get("resolutionSeconds"); rs != "" {
		n, err := strconv.Atoi(rs)
		if err != nil {
			return nil, MetricQuery{}, fmt.Errorf("resolutionSeconds: %w", err)
		}
		q.Resolution = time.Duration(n) * time.Second
	}
	if ql := v.Get("quantile"); ql != "" {
		f, err := strconv.ParseFloat(ql, 64)
		if err != nil {
			return nil, MetricQuery{}, fmt.Errorf("quantile: %w", err)
		}
		q.Quantile = f
	}

	return resources, q, nil
}
