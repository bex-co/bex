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
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
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
	"cpu-limit":      MetricCPULimit,
	"memory-limit":   MetricMemoryLimit,
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
	// Render's own datastore paths (w1/m155). Render's query has no `kind`, so
	// the resource id's prefix names it (datastoreKindFor).
	mux.HandleFunc("GET /v1/metrics/disk-usage", func(w http.ResponseWriter, r *http.Request) {
		s.datastoreMetricQuery(w, r, MetricDisk)
	})
	mux.HandleFunc("GET /v1/metrics/active-connections", s.activeConnectionsQuery)
	mux.HandleFunc("GET /v1/metrics/filters/application", s.applicationFilters)
	mux.HandleFunc("GET /v1/metrics/filters/http", s.httpFilters)
	mux.HandleFunc("GET /v1/metrics/filters/path", s.pathFilters)
	// A coded refusal, never a bare 404: the per-source time series does not
	// exist in bex (ADR018 records the divergence).
	mux.HandleFunc("GET /v1/metrics/bandwidth-sources", func(w http.ResponseWriter, _ *http.Request) {
		core.WriteErrStatus(w, http.StatusNotImplemented,
			"bandwidth-sources is not served: bex keeps its per-source bandwidth (http, nat, websocket) as month-to-date totals, not time series — read /v1/metrics/bandwidth for the all-sources series, or GraphQL monthToDateBandwidth for the per-source totals")
	})
}

// requestedResources is Render's `resource` list plus its `service` alias.
func requestedResources(v url.Values) []string {
	return append(append([]string{}, v["resource"]...), v["service"]...)
}

// datastoreKindFor infers a datastore metric's kind from the resource id's
// prefix, for Render's datastore paths, whose query has no `kind`: red- is a
// Key Value, srv- a service's attached disk, anything else (dpg-, or a bare CR
// name) the Postgres default the bex paths always had.
func datastoreKindFor(resource string) string {
	switch kind, _ := ids.KindOf(resource); kind {
	case ids.KeyValue:
		return DatastoreKeyValue
	case ids.Service:
		return DatastoreService
	default:
		return DatastoreDatabase
	}
}

// activeConnectionsMetric is Render's active-connections for a datastore kind:
// Postgres backends, or Key Value clients.
func activeConnectionsMetric(kind, resource string) (string, error) {
	switch kind {
	case DatastoreDatabase:
		return MetricDBConnections, nil
	case DatastoreKeyValue:
		return MetricKVConnections, nil
	default:
		return "", fmt.Errorf("%w: active connections need a Postgres (dpg-) or Key Value (red-) resource, not %q", core.ErrBadRequest, resource)
	}
}

// activeConnections reads Render's active_connections for one datastore
// resource — the MCP get_metrics path, which names Render's metric type rather
// than a REST route.
func (s *Service) activeConnections(ctx context.Context, resource string, start, end time.Time, resolution time.Duration) ([]MetricSeries, error) {
	kind := datastoreKindFor(resource)
	metric, err := activeConnectionsMetric(kind, resource)
	if err != nil {
		return nil, err
	}
	return s.DatastoreMetrics(ctx, DatastoreMetricQuery{
		Kind: kind, Resource: resource, Metric: metric,
		Start: start, End: end, Resolution: resolution,
	})
}

func (s *Service) activeConnectionsQuery(w http.ResponseWriter, r *http.Request) {
	q, err := parseDatastoreMetricParams(r)
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	if q.Metric, err = activeConnectionsMetric(q.Kind, q.Resource); err != nil {
		core.WriteErr(w, err)
		return
	}
	s.serveDatastoreMetric(w, r, q)
}

// renderFilterValues is one entry of Render's filters/application and
// filters/http responses.
type renderFilterValues struct {
	Filter string   `json:"filter"`
	Values []string `json:"values"`
}

// filterResource is the single resource a filters/* read names.
func filterResource(v url.Values) (string, error) {
	resources := requestedResources(v)
	if len(resources) != 1 {
		return "", fmt.Errorf("exactly one resource is required")
	}
	return resources[0], nil
}

// filterValues reads the requested filter fields through the same verb GraphQL
// metricsFilters uses, so the surfaces cannot disagree about what is queryable.
func (s *Service) filterValues(w http.ResponseWriter, r *http.Request, fields ...string) ([]MetricsFilterValues, bool) {
	resource, err := filterResource(r.URL.Query())
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
		return nil, false
	}
	values, err := s.MetricsFilters(r.Context(), MetricsFiltersQuery{App: resource, OutputFilters: fields})
	if err != nil {
		core.WriteErr(w, err)
		return nil, false
	}
	return values, true
}

// applicationFilters is Render's "List queryable instance values".
func (s *Service) applicationFilters(w http.ResponseWriter, r *http.Request) {
	values, ok := s.filterValues(w, r, filterFieldInstance)
	if !ok {
		return
	}
	core.WriteJSON(w, http.StatusOK, []renderFilterValues{{Filter: "instance", Values: values[0].Values}})
}

// httpFilters is Render's "List queryable status codes and host values". bex
// cannot narrow one filter's values by another's, so a narrowing parameter is
// refused rather than ignored; host values are discovered from the logs label
// read (the App's URLs), not here, exactly as in GraphQL metricsFilters.
func (s *Service) httpFilters(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	if v.Get("statusCode") != "" || v.Get("host") != "" {
		core.WriteErrStatus(w, http.StatusBadRequest,
			"narrowing filter values by statusCode or host is unsupported: omit them to list every queryable value")
		return
	}
	values, ok := s.filterValues(w, r, filterFieldStatusCode, filterFieldHost)
	if !ok {
		return
	}
	core.WriteJSON(w, http.StatusOK, []renderFilterValues{
		{Filter: "statusCode", Values: values[0].Values},
		{Filter: "host", Values: values[1].Values},
	})
}

// pathFilters is Render's "List queryable paths". bex has no path suggestions:
// the request path is a log-line field, not a discoverable label, and GraphQL
// metricsPathFilterSuggestions answers the same empty list. The resource is
// still authorized, so the route is no existence oracle.
func (s *Service) pathFilters(w http.ResponseWriter, r *http.Request) {
	if _, ok := s.filterValues(w, r, filterFieldResource); !ok {
		return
	}
	core.WriteJSON(w, http.StatusOK, []string{})
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
			// Render's REST vocabulary for the breakdown is statusCode (its
			// aggregateBy value and filters/http field); the sources label it code.
			if q.GroupBy == groupByStatus {
				if code, ok := ser.Labels["code"]; ok {
					delete(ser.Labels, "code")
					ser.SetLabel("statusCode", code)
				}
			}
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
	s.serveDatastoreMetric(w, r, q)
}

func (s *Service) serveDatastoreMetric(w http.ResponseWriter, r *http.Request, q DatastoreMetricQuery) {
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
// DatastoreMetricQuery. `kind` is a bex extension; without it the resource id's
// prefix decides (datastoreKindFor), which is what Render's own datastore
// paths need, since their query has no `kind`.
func parseDatastoreMetricParams(r *http.Request) (DatastoreMetricQuery, error) {
	v := r.URL.Query()

	resources := requestedResources(v)
	if len(resources) != 1 {
		return DatastoreMetricQuery{}, fmt.Errorf("exactly one resource is required")
	}
	resource := resources[0]
	kind := v.Get("kind")
	if kind == "" {
		kind = datastoreKindFor(resource)
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

	resources := requestedResources(v)
	if len(resources) == 0 {
		return nil, MetricQuery{}, fmt.Errorf("resource is required")
	}
	groupBy, err := requestGroupBy("aggregateBy", v.Get("aggregateBy"))
	if err != nil {
		return nil, MetricQuery{}, err
	}
	if err := cpuAggregation("aggregationMethod", v.Get("aggregationMethod")); err != nil {
		return nil, MetricQuery{}, err
	}

	q := MetricQuery{
		StatusCode: v.Get("statusCode"),
		// host/path are parsed only so Metrics can refuse them (see MetricQuery.Host).
		Host:       v.Get("host"),
		Path:       v.Get("path"),
		GroupBy:    groupBy,
		Percentage: v.Get("percentage") == "true",
		Instances:  v["instance"],
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
	if method := v.Get("aggregateAllMethod"); method != "" {
		replica, aggMax, parseErr := parseReplicaAggregate(method)
		if parseErr != nil {
			return nil, MetricQuery{}, parseErr
		}
		q.ReplicaAggregate, q.AggregateMax = replica, aggMax
	}

	return resources, q, nil
}
