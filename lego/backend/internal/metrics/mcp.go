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
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// mcp.go is the metrics MCP fragment. get_metrics mirrors Render's tool: a
// required resourceId (legacy alias: resource[]) and metricTypes, plus the
// optional time window and Render-named options (w2/m91 alias policy: Render
// spelling wins when both a Render name and a legacy bex name are set).

type getMetricsArgs struct {
	// ResourceID is Render's single-id spelling. Resource is the legacy bex
	// multi-id array; when ResourceID is set it wins as a one-element list.
	ResourceID string   `json:"resourceId,omitempty" jsonschema:"service id (srv-...) to read metrics for — Render's MCP spelling"`
	Resource   []string `json:"resource,omitempty" jsonschema:"legacy bex alias of resourceId as a list of service ids/names; ignored when resourceId is also set"`
	MetricTypes []string `json:"metricTypes" jsonschema:"metric ids: cpu|memory|instance_count|http_requests|http_latency|bandwidth|cpu_target|memory_target (cpu_target/memory_target are bex extensions: the App's configured autoscale-target utilization, w1/m20 — omitted when autoscaling is disabled)"`
	StartTime   string   `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window (request metrics)"`
	EndTime     string   `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window (request metrics)"`
	// Resolution is Render's spelling; ResolutionSeconds is the legacy bex alias.
	Resolution        *int64 `json:"resolution,omitempty" jsonschema:"request-metric step in seconds — Render's MCP spelling"`
	ResolutionSeconds *int64 `json:"resolutionSeconds,omitempty" jsonschema:"legacy bex alias of resolution; ignored when resolution is also set"`
	// HTTPLatencyQuantile is Render's spelling; Quantile is the legacy bex alias.
	HTTPLatencyQuantile *float64  `json:"httpLatencyQuantile,omitempty" jsonschema:"http_latency percentile 0..1 (default .95) — Render's MCP spelling"`
	Quantile            *float64  `json:"quantile,omitempty" jsonschema:"legacy bex alias of httpLatencyQuantile; ignored when httpLatencyQuantile is also set"`
	Quantiles           []float64 `json:"quantiles,omitempty" jsonschema:"http_latency percentiles 0..1 to read together — the percentile 'All' overlay (e.g. [0.5,0.9,0.99]); each returned series carries a quantile label so the overlaid percentiles stay distinct"`
	Percentage          bool      `json:"percentage,omitempty" jsonschema:"report cpu/memory as a percentage of the pod limit"`
	// HTTPHost/HTTPPath are Render's spellings; Host/Path are legacy bex aliases.
	HTTPHost string `json:"httpHost,omitempty" jsonschema:"filter http_requests/http_latency to one request Host — Render's MCP spelling; served from the request-log store (Loki)"`
	Host     string `json:"host,omitempty" jsonschema:"legacy bex alias of httpHost; ignored when httpHost is also set"`
	HTTPPath string `json:"httpPath,omitempty" jsonschema:"filter http_requests/http_latency to one request Path — Render's MCP spelling; served from the request-log store (Loki)"`
	Path     string `json:"path,omitempty" jsonschema:"legacy bex alias of httpPath; ignored when httpPath is also set"`
	// CPUUsageAggregationMethod is accepted for contract parity. bex's CPU
	// series is a metrics-server snapshot (no interval aggregation), so only
	// AVG (or omit) is honored; MAX/MIN are rejected.
	CPUUsageAggregationMethod string `json:"cpuUsageAggregationMethod,omitempty" jsonschema:"AVG (default, only supported value) | MAX | MIN — bex CPU is a metrics-server snapshot, so MAX/MIN are rejected"`
	// AggregateHTTPRequestCountsBy groups http_requests. statusCode maps onto
	// GroupBy=status; host has no Traefik/Loki group-by axis and is rejected.
	AggregateHTTPRequestCountsBy string `json:"aggregateHttpRequestCountsBy,omitempty" jsonschema:"group http_requests by statusCode (wired) or host (unsupported — rejected)"`
	// Instance is a bex extension (w5/m89): public instance ids from m87. Omit
	// for all instances; unresolved ids yield an empty series, never a broaden.
	Instance []string `json:"instance,omitempty" jsonschema:"public service instance ids to keep (canonical srv-…-… ids from metricsFilters INSTANCE / serviceInstances); omit for all"`
	// AggregateAllMethod is a bex extension mirroring Render GraphQL
	// aggregateAllMethod: MIN|MAX|AVG across selected replicas at each timestamp.
	AggregateAllMethod string `json:"aggregateAllMethod,omitempty" jsonschema:"MIN|MAX|AVG across selected replicas at each timestamp (distinct from cpuUsageAggregationMethod interval aggregation)"`
}

type getMetricsResult struct {
	// Series is nil-coalesced at every success return: a nil slice marshals to
	// `null`, while REST's toRenderMetrics always emits `[]`, so a resource with
	// no series in the window would answer differently depending on the surface
	// the caller happened to use (w6/m110/t005 — the same required-shape class
	// w6/m109 fixed on the datastore views).
	Series []MetricSeries `json:"series"`
}

// metricSeriesOrEmpty keeps MCP's empty case shaped like REST's `[]`.
func metricSeriesOrEmpty(series []MetricSeries) []MetricSeries {
	if series == nil {
		return []MetricSeries{}
	}
	return series
}

// resources applies the w2/m91 alias policy for resourceId vs resource.
func (a getMetricsArgs) resources() ([]string, error) {
	return mcputil.ResourceIDs(a.ResourceID, a.Resource, "resourceId", "resource")
}

func (a getMetricsArgs) resolutionSeconds() int64 {
	return mcputil.PreferPtrOrZero(a.Resolution, a.ResolutionSeconds)
}

func (a getMetricsArgs) quantile() float64 {
	return mcputil.PreferPtrOrZero(a.HTTPLatencyQuantile, a.Quantile)
}

func (a getMetricsArgs) host() string {
	return mcputil.PreferString(a.HTTPHost, a.Host)
}

func (a getMetricsArgs) path() string {
	return mcputil.PreferString(a.HTTPPath, a.Path)
}

// applyCPUAggregation enforces the thin AVG-only wiring of
// cpuUsageAggregationMethod. Empty/AVG are no-ops; MAX/MIN are capability gaps.
func applyCPUAggregation(method string) error {
	switch strings.ToUpper(strings.TrimSpace(method)) {
	case "", "AVG":
		return nil
	case "MAX", "MIN":
		return fmt.Errorf("%w: cpuUsageAggregationMethod %q is unsupported — bex CPU is a metrics-server snapshot without interval aggregation (only AVG)", core.ErrBadRequest, method)
	default:
		return fmt.Errorf("%w: unknown cpuUsageAggregationMethod %q (want AVG|MAX|MIN)", core.ErrBadRequest, method)
	}
}

// applyHTTPRequestAggregate maps aggregateHttpRequestCountsBy onto MetricQuery.GroupBy.
func applyHTTPRequestAggregate(by string) (string, error) {
	switch strings.TrimSpace(by) {
	case "":
		return "", nil
	case "statusCode":
		return "status", nil
	case "host":
		return "", fmt.Errorf("%w: aggregateHttpRequestCountsBy=host is unsupported — neither Traefik Prometheus counters nor the Loki request-log path expose a host group-by axis (filter with httpHost instead)", core.ErrBadRequest)
	default:
		return "", fmt.Errorf("%w: unknown aggregateHttpRequestCountsBy %q (want statusCode|host)", core.ErrBadRequest, by)
	}
}

// RegisterMCP adds the get_metrics tool to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_metrics",
		Description: "Get resource (cpu/memory/instance_count) and request (http_requests/http_latency/bandwidth) metrics for one or more services, as Render-shaped time-series. Prefer Render's resourceId/resolution/httpLatencyQuantile/httpHost/httpPath; legacy resource/resolutionSeconds/quantile/host/path aliases still work (Render spelling wins when both are set).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getMetricsArgs) (*mcp.CallToolResult, getMetricsResult, error) {
		resources, err := in.resources()
		if err != nil {
			return nil, getMetricsResult{}, err
		}
		if err := applyCPUAggregation(in.CPUUsageAggregationMethod); err != nil {
			return nil, getMetricsResult{}, err
		}
		groupBy, err := applyHTTPRequestAggregate(in.AggregateHTTPRequestCountsBy)
		if err != nil {
			return nil, getMetricsResult{}, err
		}
		q := MetricQuery{
			Quantile:   in.quantile(),
			Quantiles:  in.Quantiles,
			Percentage: in.Percentage,
			Host:       in.host(),
			Path:       in.path(),
			GroupBy:    groupBy,
			Resolution: time.Duration(in.resolutionSeconds()) * time.Second,
			Instances:  in.Instance,
		}
		if in.AggregateAllMethod != "" {
			replica, aggMax, parseErr := parseReplicaAggregate(in.AggregateAllMethod)
			if parseErr != nil {
				return nil, getMetricsResult{}, parseErr
			}
			q.ReplicaAggregate, q.AggregateMax = replica, aggMax
		}
		if q.Start, err = core.ParseTime("startTime", in.StartTime); err != nil {
			return nil, getMetricsResult{}, err
		}
		if q.End, err = core.ParseTime("endTime", in.EndTime); err != nil {
			return nil, getMetricsResult{}, err
		}
		// The percentile dimension applies whenever the metric list includes
		// http_latency; a mixed list over-approximates the product, which only
		// errs toward refusing extreme requests.
		quantileFan := 1
		for _, metric := range in.MetricTypes {
			if fan := latencyFan(metric, in.Quantiles); fan > quantileFan {
				quantileFan = fan
			}
		}
		if err := checkFanOut(len(resources), len(in.MetricTypes), quantileFan); err != nil {
			return nil, getMetricsResult{}, err
		}
		var all []MetricSeries
		for _, id := range resources {
			for _, metric := range in.MetricTypes {
				q.App, q.Metric = id, metric
				// MetricsWithQuantiles fans http_latency out over q.Quantiles (the
				// percentile "All" overlay), tagging each series with its quantile.
				series, err := s.MetricsWithQuantiles(ctx, q)
				if err != nil {
					return nil, getMetricsResult{}, err
				}
				// Tag each series with its metric so multi-metric results stay distinct.
				for i := range series {
					ser := series[i].MetricSeries
					ser.SetLabel(LabelMetric, metric)
					all = append(all, ser)
				}
			}
		}
		return nil, getMetricsResult{Series: metricSeriesOrEmpty(all)}, nil
	})
	RegisterDatastoreMetricsMCP(s, srv)
}

// getDatastoreMetricsArgs is get_datastore_metrics' input — the Database/
// KeyValue-scoped sibling of getMetricsArgs. One resource per call (a
// datastore metric always names exactly one instance, mirroring the
// list_postgres_instances/get_postgres_instance MCP shape rather than
// get_metrics' multi-resource array). resourceId/resolution aliases match
// get_metrics for consistency (w2/m91); this tool remains a bex Extension.
type getDatastoreMetricsArgs struct {
	ResourceID        string   `json:"resourceId,omitempty" jsonschema:"the Database, KeyValue, or service id (dpg-…/red-…/srv-…) — Render-shaped alias of resource"`
	Resource          string   `json:"resource,omitempty" jsonschema:"legacy spelling of resourceId — the CR name, not the display name; ignored when resourceId is also set"`
	Kind              string   `json:"kind,omitempty" jsonschema:"database|keyvalue|service (default database); service reads the disk attached to a service (ADR082)"`
	MetricTypes       []string `json:"metricTypes" jsonschema:"metric ids: disk|disk_capacity (Database, KeyValue, or a service with an attached disk) | db_connections|replication_lag (Database only; replication_lag is omitted until Postgres HA is enabled, w1/m22) | kv_memory|kv_connections (KeyValue only)"`
	StartTime         string   `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window"`
	EndTime           string   `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window"`
	Resolution        *int64   `json:"resolution,omitempty" jsonschema:"step in seconds — Render-shaped alias of resolutionSeconds"`
	ResolutionSeconds *int64   `json:"resolutionSeconds,omitempty" jsonschema:"legacy bex alias of resolution; ignored when resolution is also set"`
}

func (a getDatastoreMetricsArgs) resource() (string, error) {
	return mcputil.RequireAliasString(a.ResourceID, a.Resource, "resourceId", "resource")
}

func (a getDatastoreMetricsArgs) resolutionSeconds() int64 {
	return mcputil.PreferPtrOrZero(a.Resolution, a.ResolutionSeconds)
}

// RegisterDatastoreMetricsMCP adds the get_datastore_metrics tool — split from
// RegisterMCP only so datastore.go's verb and its MCP fragment read together;
// it still contributes into the same shared MCP registry.
func RegisterDatastoreMetricsMCP(s *Service, srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_datastore_metrics",
		Description: "Get disk usage, active-connections, replication-lag (Postgres), and memory/connections (Key Value) metrics for one managed Postgres or Key Value instance — or, with kind=service, the used/capacity bytes of the persistent disk attached to a service (ADR082) — as Render-shaped time-series. bex extension (no Render equivalent).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getDatastoreMetricsArgs) (*mcp.CallToolResult, getMetricsResult, error) {
		resource, err := in.resource()
		if err != nil {
			return nil, getMetricsResult{}, err
		}
		kind := in.Kind
		if kind == "" {
			kind = DatastoreDatabase
		}
		q := DatastoreMetricQuery{
			Kind:       kind,
			Resource:   resource,
			Resolution: time.Duration(in.resolutionSeconds()) * time.Second,
		}
		if q.Start, err = core.ParseTime("startTime", in.StartTime); err != nil {
			return nil, getMetricsResult{}, err
		}
		if q.End, err = core.ParseTime("endTime", in.EndTime); err != nil {
			return nil, getMetricsResult{}, err
		}
		if err := checkFanOut(1, len(in.MetricTypes), 1); err != nil {
			return nil, getMetricsResult{}, err
		}
		var all []MetricSeries
		for _, metric := range in.MetricTypes {
			q.Metric = metric
			series, err := s.DatastoreMetrics(ctx, q)
			if err != nil {
				return nil, getMetricsResult{}, err
			}
			setLabelOnEach(series, LabelMetric, metric)
			all = append(all, series...)
		}
		return nil, getMetricsResult{Series: metricSeriesOrEmpty(all)}, nil
	})
}
