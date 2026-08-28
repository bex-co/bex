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
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

// mcp.go is the metrics MCP fragment. get_metrics mirrors Render's tool: a
// required `resource` array of service ids and `metricTypes` (bex metric ids),
// plus the optional time window and options.
type getMetricsArgs struct {
	Resource          []string  `json:"resource" jsonschema:"service ids (srv-...) or names to read metrics for"`
	MetricTypes       []string  `json:"metricTypes" jsonschema:"metric ids: cpu|memory|instance_count|http_requests|http_latency|bandwidth|cpu_target|memory_target (cpu_target/memory_target are bex extensions: the App's configured autoscale-target utilization, w1/m20 — omitted when autoscaling is disabled)"`
	StartTime         string    `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window (request metrics)"`
	EndTime           string    `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window (request metrics)"`
	ResolutionSeconds int64     `json:"resolutionSeconds,omitempty" jsonschema:"request-metric step in seconds"`
	Quantile          float64   `json:"quantile,omitempty" jsonschema:"http_latency percentile 0..1 (default .95)"`
	Quantiles         []float64 `json:"quantiles,omitempty" jsonschema:"http_latency percentiles 0..1 to read together — the percentile 'All' overlay (e.g. [0.5,0.9,0.99]); each returned series carries a quantile label so the overlaid percentiles stay distinct"`
	Percentage        bool      `json:"percentage,omitempty" jsonschema:"report cpu/memory as a percentage of the pod limit"`
	Host              string    `json:"host,omitempty" jsonschema:"filter http_requests/http_latency to one request Host — served from the request-log store (Loki); requires BEX_LOKI_URL or the read returns store-unavailable, and is rejected for any other metric"`
	Path              string    `json:"path,omitempty" jsonschema:"filter http_requests/http_latency to one request Path — served from the request-log store (Loki); requires BEX_LOKI_URL or the read returns store-unavailable, and is rejected for any other metric"`
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

// RegisterMCP adds the get_metrics tool to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_metrics",
		Description: "Get resource (cpu/memory/instance_count) and request (http_requests/http_latency/bandwidth) metrics for one or more services, as Render-shaped time-series.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getMetricsArgs) (*mcp.CallToolResult, getMetricsResult, error) {
		q := MetricQuery{
			Quantile:   in.Quantile,
			Quantiles:  in.Quantiles,
			Percentage: in.Percentage,
			Host:       in.Host,
			Path:       in.Path,
			Resolution: time.Duration(in.ResolutionSeconds) * time.Second,
		}
		var err error
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
		if err := checkFanOut(len(in.Resource), len(in.MetricTypes), quantileFan); err != nil {
			return nil, getMetricsResult{}, err
		}
		var all []MetricSeries
		for _, id := range in.Resource {
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
// get_metrics' multi-resource array).
type getDatastoreMetricsArgs struct {
	Resource          string   `json:"resource" jsonschema:"the Database, KeyValue, or service id (dpg-…/red-…/srv-…) — the CR name, not the display name"`
	Kind              string   `json:"kind,omitempty" jsonschema:"database|keyvalue|service (default database); service reads the disk attached to a service (ADR082)"`
	MetricTypes       []string `json:"metricTypes" jsonschema:"metric ids: disk|disk_capacity (Database, KeyValue, or a service with an attached disk) | db_connections|replication_lag (Database only; replication_lag is omitted until Postgres HA is enabled, w1/m22) | kv_memory|kv_connections (KeyValue only)"`
	StartTime         string   `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window"`
	EndTime           string   `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window"`
	ResolutionSeconds int64    `json:"resolutionSeconds,omitempty" jsonschema:"step in seconds"`
}

// RegisterDatastoreMetricsMCP adds the get_datastore_metrics tool — split from
// RegisterMCP only so datastore.go's verb and its MCP fragment read together;
// it still contributes into the same shared MCP registry.
func RegisterDatastoreMetricsMCP(s *Service, srv *mcp.Server) {
	mcputil.AddTool(srv, &mcp.Tool{
		Name:        "get_datastore_metrics",
		Description: "Get disk usage, active-connections, replication-lag (Postgres), and memory/connections (Key Value) metrics for one managed Postgres or Key Value instance — or, with kind=service, the used/capacity bytes of the persistent disk attached to a service (ADR082) — as Render-shaped time-series. bex extension (no Render equivalent).",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getDatastoreMetricsArgs) (*mcp.CallToolResult, getMetricsResult, error) {
		kind := in.Kind
		if kind == "" {
			kind = DatastoreDatabase
		}
		q := DatastoreMetricQuery{
			Kind:       kind,
			Resource:   in.Resource,
			Resolution: time.Duration(in.ResolutionSeconds) * time.Second,
		}
		var err error
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
