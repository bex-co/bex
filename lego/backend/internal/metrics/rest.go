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
	"errors"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
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
	// Render's database-activity and disk graphs (w7/m430). Render publishes
	// no REST path for them, so these are bex extensions named after the
	// graphs.
	"network-activity":     MetricNetworkActivity,    // Postgres only
	"transaction-volume":   MetricTransactionVolume,  // Postgres only
	"lock-delayed-queries": MetricLockDelayedQueries, // Postgres only
	"disk-activity":        MetricDiskActivity,       // Postgres, Key Value, a service's disk
	"disk-operations":      MetricDiskOperations,     // Postgres, a service's disk
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
	// Render's hourly bandwidth by traffic source (w8/190, bandwidth_sources.go).
	mux.HandleFunc("GET /v1/metrics/bandwidth-sources", s.bandwidthSources)
}

// requestedResources is Render's `resource` list plus its `service` alias.
// Empty values are dropped: a present-but-empty `resource=` names no resource,
// so the callers' count checks must answer it like a missing parameter (the
// "resource is required" 400) instead of letting an empty id flow into lookup
// and surface as a 500 (w4/370).
func requestedResources(v url.Values) []string {
	return nonBlankStrings(append(append([]string{}, v["resource"]...), v["service"]...))
}

func metricScopeFromQuery(v url.Values) savedsearch.ScopeRequest {
	return savedsearch.ScopeRequest{
		Project:     v.Get("project"),
		Environment: v.Get("environment"),
		Region:      v.Get("region"),
		Resources:   requestedResources(v),
	}
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
// than a REST route. Connection counts have no limit denominator, so
// percentage is refused exactly as DatastoreMetrics refuses it for every
// non cpu/memory metric; instance selection and replica aggregation apply to
// the per-instance series identically to a service's, with unresolved
// selectors narrowing to empty rather than broadening.
func (s *Service) activeConnections(ctx context.Context, q MetricQuery, resource string) ([]MetricSeries, error) {
	kind := datastoreKindFor(resource)
	metric, err := activeConnectionsMetric(kind, resource)
	if err != nil {
		return nil, err
	}
	if q.Percentage {
		return nil, fmt.Errorf("%w: percentage applies only to cpu and memory metrics", core.ErrBadRequest)
	}
	series, err := s.DatastoreMetrics(ctx, DatastoreMetricQuery{
		Kind: kind, Resource: resource, Metric: metric,
		Start: q.Start, End: q.End, Resolution: q.Resolution,
	})
	if err != nil {
		return nil, err
	}
	q.App, q.Metric = resource, metric
	return applyInstanceSelection(q, series, nil)
}

// datastoreAppMetric answers one of Render's App-metric paths (cpu, memory,
// *-limit, instance-count, http-*, bandwidth) for a datastore id (w8/061,
// w34/m589). Render scopes them to Postgres and Key Value too. DatastoreMetrics
// serves cpu, memory and their limits (w1/m180; Key Value memory is its
// kv_memory series), normalizes percentage requests against each instance's
// limit history from DatastoreResource before public instance-id projection,
// and refuses every other pair by name after authorizing the resource, so it
// is never mistaken for a missing id. Instance filtering and replica
// aggregation apply to its per-instance series identically to a service's.
func (s *Service) datastoreAppMetric(ctx context.Context, q MetricQuery, resource string) ([]MetricSeries, error) {
	kind := datastoreKindFor(resource)
	if err := checkCPUAggregation(q.Metric, q.CPUAggregation); err != nil {
		return nil, err
	}
	// No datastore metric has a per-request status axis, so any statusCode
	// filter is refused before any store — like the service read's refusal.
	// Validation lives here (not in DatastoreMetrics) because StatusCodes are
	// dropped when building the DatastoreMetricQuery below.
	statusCodes, err := normalizeStatusCodeFilters(q.StatusCodes)
	if err != nil {
		return nil, err
	}
	if len(statusCodes) > 0 {
		return nil, fmt.Errorf("%w: statusCode filters apply only to http_requests and http_latency", core.ErrBadRequest)
	}
	series, err := s.DatastoreMetrics(ctx, DatastoreMetricQuery{
		Kind: kind, Resource: resource, Metric: q.Metric,
		Start: q.Start, End: q.End, Resolution: q.Resolution,
		CPUAggregation: q.CPUAggregation,
		Percentage:     q.Percentage,
	})
	if err != nil {
		return nil, err
	}
	if len(q.Hosts) > 0 || len(q.Paths) > 0 {
		return nil, fmt.Errorf("%w: host/path filters apply only to http_requests and http_latency", core.ErrBadRequest)
	}
	if q.GroupBy == groupByHost && q.Metric != MetricHTTPRequests {
		return nil, fmt.Errorf("%w: aggregateBy=host applies only to http_requests", core.ErrBadRequest)
	}
	q.App = resource
	return applyInstanceSelection(q, series, nil)
}

func (s *Service) activeConnectionsQuery(w http.ResponseWriter, r *http.Request) {
	s.serveDatastoreResources(w, r, keepActiveConnectionsResource, activeConnectionsMetric)
}

// renderFilterValues is one entry of Render's filters/application and
// filters/http responses.
type renderFilterValues struct {
	Filter string   `json:"filter"`
	Values []string `json:"values"`
}

// filterValues reads the requested filter fields through the same verb GraphQL
// metricsFilters uses, so the surfaces cannot disagree about what is queryable.
func (s *Service) filterValues(w http.ResponseWriter, r *http.Request, httpOnly bool, hosts, statusCodes []string, fields ...string) ([]MetricsFilterValues, bool) {
	v := r.URL.Query()
	start, end, _, err := parseTimeWindow(v)
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
		return nil, false
	}
	scopeReq := metricScopeFromQuery(v)
	resolved, err := s.resolveMetricScope(r.Context(), scopeReq, maxMetricResources, keepFilterResource(httpOnly))
	if err != nil {
		core.WriteErr(w, err)
		return nil, false
	}
	resources := resolved.IDs
	if resolved.Scoped && len(scopeReq.Resources) == 0 {
		if resources, err = s.readableMetricResourceIDs(r.Context(), resolved.Resources); err != nil {
			core.WriteErr(w, err)
			return nil, false
		}
	}
	if len(resources) == 0 {
		if err := s.checkWindow(start, end); err != nil {
			core.WriteErr(w, err)
			return nil, false
		}
		empty := make([]MetricsFilterValues, len(fields))
		for i, f := range fields {
			empty[i] = MetricsFilterValues{Field: f, Values: []string{}}
		}
		return empty, true
	}
	values, err := s.MetricsFilters(r.Context(), MetricsFiltersQuery{
		Resources:     resources,
		Start:         start,
		End:           end,
		Hosts:         hosts,
		StatusCodes:   statusCodes,
		OutputFilters: fields,
	})
	if err != nil {
		core.WriteErr(w, err)
		return nil, false
	}
	return values, true
}

// applicationFilters is Render's "List queryable instance values".
func (s *Service) applicationFilters(w http.ResponseWriter, r *http.Request) {
	values, ok := s.filterValues(w, r, false, nil, nil, filterFieldInstance)
	if !ok {
		return
	}
	core.WriteJSON(w, http.StatusOK, []renderFilterValues{{Filter: "instance", Values: values[0].Values}})
}

// httpFilters is Render's "List queryable status codes and host values",
// honoring repeatable resource/service, project/environment/region,
// startTime/endTime, and host/statusCode cross-filtering (w34/m588, w4/m596).
func (s *Service) httpFilters(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	values, ok := s.filterValues(w, r, true, nonEmptyValues(v["host"]), nonEmptyValues(v["statusCode"]), filterFieldStatusCode, filterFieldHost)
	if !ok {
		return
	}
	core.WriteJSON(w, http.StatusOK, []renderFilterValues{
		{Filter: "statusCode", Values: values[0].Values},
		{Filter: "host", Values: values[1].Values},
	})
}

// pathFilters is Render's "List queryable paths" (GET /v1/metrics/filters/path):
// returns deduplicated, bounded path suggestions derived from the most recent
// 5,000 matching Traefik access-log lines in Loki across one or more
// authorized resources or a project/environment scope (w34/m587, w4/m596).
// Empty results always serialize as [].
func (s *Service) pathFilters(w http.ResponseWriter, r *http.Request) {
	v := r.URL.Query()
	start, end, _, err := parseTimeWindow(v)
	if err != nil {
		core.WriteErrStatus(w, http.StatusBadRequest, err.Error())
		return
	}
	scopeReq := metricScopeFromQuery(v)
	resolved, err := s.resolveMetricScope(r.Context(), scopeReq, maxMetricResources, keepFilterResource(true))
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	resources := resolved.IDs
	if resolved.Scoped && len(scopeReq.Resources) == 0 {
		if resources, err = s.readableMetricResourceIDs(r.Context(), resolved.Resources); err != nil {
			core.WriteErr(w, err)
			return
		}
	}
	if len(resources) == 0 {
		if err := s.checkWindow(start, end); err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, []string{})
		return
	}
	paths, err := s.PathFilterSuggestions(r.Context(), PathFilterSuggestionsQuery{
		Apps:        resources,
		Start:       start,
		End:         end,
		Hosts:       nonEmptyValues(v["host"]),
		StatusCodes: nonEmptyValues(v["statusCode"]),
		Paths:       nonEmptyValues(v["path"]),
	})
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	core.WriteJSON(w, http.StatusOK, paths)
}

// metricQuery serves one Render metrics endpoint. Render's `resource` (or
// `project`/`environment`/`region`) resolves to a bounded set of resources;
// bex merges each resource's series into one response.
func (s *Service) metricQuery(w http.ResponseWriter, r *http.Request, metric string) {
	scopeReq, q, err := parseMetricParams(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	resolved, err := s.resolveMetricScope(r.Context(), scopeReq, maxMetricResources, keepMetricResource(metric))
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	// The window cap lives in the shared service (Metrics.checkWindow); only
	// the resource-array fan-out is adapter-shaped, so it is checked here.
	if err := checkFanOut(max(len(scopeReq.Resources), len(resolved.IDs)), 1, latencyFan(metric, q.Quantiles)); err != nil {
		core.WriteErr(w, err)
		return
	}
	q.Metric = metric
	if len(resolved.IDs) == 0 {
		if err := s.checkWindow(q.Start, q.End); err != nil {
			core.WriteErr(w, err)
			return
		}
		if metric == MetricHTTPLatency && resolved.WorkspaceID != "" {
			if err := s.RequirePlanFeature(r.Context(), resolved.WorkspaceID, core.FeatureLatencyMetrics); err != nil {
				core.WriteErr(w, err)
				return
			}
		}
		core.WriteJSON(w, http.StatusOK, toRenderMetrics(nil))
		return
	}

	skipUnreadable := resolved.Scoped && len(scopeReq.Resources) == 0
	var all []MetricSeries
	for _, res := range resolved.IDs {
		if core.IsDatastoreResource(res) {
			series, err := s.datastoreAppMetric(r.Context(), q, res)
			if err != nil {
				if skipUnreadable && (errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrForbidden)) {
					continue
				}
				core.WriteErr(w, err)
				return
			}
			all = append(all, series...)
			continue
		}
		q.App = res
		series, err := s.MetricsWithQuantiles(r.Context(), q)
		if err != nil {
			if skipUnreadable && (errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrForbidden)) {
				continue
			}
			core.WriteErr(w, err)
			return
		}
		for _, ser := range series {
			all = append(all, restStatusCode(ser.MetricSeries, q.GroupBy))
		}
	}
	core.WriteJSON(w, http.StatusOK, toRenderMetrics(all))
}

// datastoreMetricQuery serves one datastore-metric endpoint (disk-usage/
// disk-capacity/replication-lag/db-connections/...), honoring repeatable
// `resource` (and `service`) and Render's `project`/`environment`/`region`
// query parameters (w34/m589, w4/m596).
func (s *Service) datastoreMetricQuery(w http.ResponseWriter, r *http.Request, metric string) {
	s.serveDatastoreResources(w, r, keepMetricResource(metric), func(_, _ string) (string, error) { return metric, nil })
}

func (s *Service) serveDatastoreResources(w http.ResponseWriter, r *http.Request, keep func(savedsearch.Resource) bool, metricFor func(kind, resource string) (string, error)) {
	scopeReq, base, err := parseDatastoreMetricParams(r)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	resolved, err := s.resolveMetricScope(r.Context(), scopeReq, maxMetricResources, keep)
	if err != nil {
		core.WriteErr(w, err)
		return
	}
	if err := checkFanOut(len(resolved.IDs), 1, 1); err != nil {
		core.WriteErr(w, err)
		return
	}
	if len(resolved.IDs) == 0 {
		if err := s.checkWindow(base.Start, base.End); err != nil {
			core.WriteErr(w, err)
			return
		}
		core.WriteJSON(w, http.StatusOK, toRenderMetrics(nil))
		return
	}
	skipUnreadable := resolved.Scoped && len(scopeReq.Resources) == 0
	var all []MetricSeries
	for _, res := range resolved.IDs {
		q := base
		q.Resource = res
		if q.Kind == "" {
			q.Kind = datastoreKindFor(res)
		}
		if q.Metric, err = metricFor(q.Kind, res); err != nil {
			core.WriteErr(w, err)
			return
		}
		series, err := s.DatastoreMetrics(r.Context(), q)
		if err != nil {
			if skipUnreadable && (errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrForbidden)) {
				continue
			}
			core.WriteErr(w, err)
			return
		}
		all = append(all, series...)
	}
	core.WriteJSON(w, http.StatusOK, toRenderMetrics(all))
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
		f, err := strconv.ParseFloat(rs, 64)
		// !(f >= min) also refuses NaN, which compares false with everything.
		if err != nil || !(f >= minResolutionSeconds) || math.IsInf(f, 0) {
			return time.Time{}, time.Time{}, 0, fmt.Errorf("%w: resolutionSeconds must be a number of at least %d", core.ErrBadRequest, minResolutionSeconds)
		}
		resolution = time.Duration(math.Round(min(f, maxResolutionSeconds))) * time.Second
	}
	return start, end, resolution, nil
}

// Render types resolutionSeconds as a number of at least 30 on every metrics
// path, and its pinned client sends a float32, so 60.0, 1e2 and 90.5 are all
// valid (w8/351). Prometheus and Loki take the step in whole seconds
// (stepSeconds), so a fractional value is rounded to the nearest second once,
// here. Render's spec sets no maximum; maxResolutionSeconds (about 31 years,
// longer than any retention, so a longer step selects no other samples) only
// keeps the conversion to time.Duration from overflowing.
const (
	minResolutionSeconds = 30
	maxResolutionSeconds = 1e9
)

// parseDatastoreMetricParams maps a datastore-metric query string onto a
// ScopeRequest plus a base DatastoreMetricQuery. `kind` is a bex extension;
// when empty, each resource's id prefix decides (datastoreKindFor) so mixed
// resource lists work on Render's disk-usage, disk-capacity, and
// active-connections endpoints (w34/m589, w4/m596).
func parseDatastoreMetricParams(r *http.Request) (savedsearch.ScopeRequest, DatastoreMetricQuery, error) {
	v := r.URL.Query()
	start, end, resolution, err := parseTimeWindow(v)
	if err != nil {
		return savedsearch.ScopeRequest{}, DatastoreMetricQuery{}, err
	}
	return metricScopeFromQuery(v), DatastoreMetricQuery{
		Kind:  v.Get("kind"),
		Start: start, End: end, Resolution: resolution,
	}, nil
}

// nonEmptyValues drops empty repeated-parameter values, preserving order.
func nonEmptyValues(values []string) []string {
	out := values[:0]
	for _, v := range values {
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}

// parseMetricParams maps Render's metrics query string onto a ScopeRequest + a
// MetricQuery.
func parseMetricParams(r *http.Request) (savedsearch.ScopeRequest, MetricQuery, error) {
	v := r.URL.Query()
	groupBy, err := requestGroupBy("aggregateBy", v.Get("aggregateBy"))
	if err != nil {
		return savedsearch.ScopeRequest{}, MetricQuery{}, err
	}
	cpuAgg, err := parseCPUAggregation("aggregationMethod", v.Get("aggregationMethod"))
	if err != nil {
		return savedsearch.ScopeRequest{}, MetricQuery{}, err
	}

	q := MetricQuery{
		CPUAggregation: cpuAgg,
		StatusCodes:    nonEmptyValues(v["statusCode"]),
		// host/path are parsed only so Metrics can refuse them (see MetricQuery.Hosts).
		Hosts:      nonEmptyValues(v["host"]),
		Paths:      nonEmptyValues(v["path"]),
		GroupBy:    groupBy,
		Percentage: v.Get("percentage") == "true",
		Instances:  v["instance"],
	}

	start, end, resolution, err := parseTimeWindow(v)
	if err != nil {
		return savedsearch.ScopeRequest{}, MetricQuery{}, err
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
		// The negated range also refuses NaN.
		if err != nil || !(f >= 0 && f <= 1) {
			return savedsearch.ScopeRequest{}, MetricQuery{}, fmt.Errorf("%w: quantile must be a number between 0 and 1", core.ErrBadRequest)
		}
		q.Quantiles = append(q.Quantiles, f)
	}
	if len(q.Quantiles) == 1 {
		q.Quantile = q.Quantiles[0]
		q.QuantileSet = true
	}
	if method := v.Get("aggregateAllMethod"); method != "" {
		replica, aggMax, parseErr := parseReplicaAggregate(method)
		if parseErr != nil {
			return savedsearch.ScopeRequest{}, MetricQuery{}, parseErr
		}
		q.ReplicaAggregate, q.AggregateMax = replica, aggMax
	}

	return metricScopeFromQuery(v), q, nil
}
