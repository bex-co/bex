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
)

// mcp.go is the metrics MCP fragment. get_metrics mirrors Render's tool: a
// required `resource` array of service ids and `metricTypes` (bex metric ids),
// plus the optional time window and options.
type getMetricsArgs struct {
	Resource          []string `json:"resource" jsonschema:"service ids (bex App names) to read metrics for"`
	MetricTypes       []string `json:"metricTypes" jsonschema:"metric ids: cpu|memory|instance_count|http_requests|http_latency|bandwidth|cpu_target|memory_target (cpu_target/memory_target are bex extensions: the App's configured autoscale-target utilization, w1/m20 — omitted when autoscaling is disabled)"`
	StartTime         string   `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window (request metrics)"`
	EndTime           string   `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window (request metrics)"`
	ResolutionSeconds int64    `json:"resolutionSeconds,omitempty" jsonschema:"request-metric step in seconds"`
	Quantile          float64  `json:"quantile,omitempty" jsonschema:"http_latency percentile 0..1 (default .95)"`
	Percentage        bool     `json:"percentage,omitempty" jsonschema:"report cpu/memory as a percentage of the pod limit"`
}

type getMetricsResult struct {
	Series []MetricSeries `json:"series"`
}

// RegisterMCP adds the get_metrics tool to the shared MCP server.
func (s *Service) RegisterMCP(srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_metrics",
		Description: "Get resource (cpu/memory/instance_count) and request (http_requests/http_latency/bandwidth) metrics for one or more services, as Render-shaped time-series.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in getMetricsArgs) (*mcp.CallToolResult, getMetricsResult, error) {
		q := MetricQuery{
			Quantile:   in.Quantile,
			Percentage: in.Percentage,
			Resolution: time.Duration(in.ResolutionSeconds) * time.Second,
		}
		if in.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, in.StartTime); err == nil {
				q.Start = t
			}
		}
		if in.EndTime != "" {
			if t, err := time.Parse(time.RFC3339, in.EndTime); err == nil {
				q.End = t
			}
		}
		var all []MetricSeries
		for _, id := range in.Resource {
			for _, metric := range in.MetricTypes {
				q.App, q.Metric = id, metric
				series, err := s.Metrics(ctx, q)
				if err != nil {
					return nil, getMetricsResult{}, err
				}
				// Tag each series with its metric so multi-metric results stay distinct.
				for i := range series {
					if series[i].Labels == nil {
						series[i].Labels = map[string]string{}
					}
					series[i].Labels["metric"] = metric
				}
				all = append(all, series...)
			}
		}
		return nil, getMetricsResult{Series: all}, nil
	})
	RegisterDatastoreMetricsMCP(s, srv)
}

// getDatastoreMetricsArgs is get_datastore_metrics' input — the Database/
// KeyValue-scoped sibling of getMetricsArgs. One resource per call (a
// datastore metric always names exactly one instance, mirroring the
// list_postgres_instances/get_postgres_instance MCP shape rather than
// get_metrics' multi-resource array).
type getDatastoreMetricsArgs struct {
	Resource          string   `json:"resource" jsonschema:"the Database or KeyValue name to read metrics for"`
	Kind              string   `json:"kind,omitempty" jsonschema:"database|keyvalue (default database)"`
	MetricTypes       []string `json:"metricTypes" jsonschema:"metric ids: disk|disk_capacity (Database or KeyValue) | db_connections|replication_lag (Database only; replication_lag is omitted until Postgres HA is enabled, w1/m22) | kv_memory|kv_connections (KeyValue only)"`
	StartTime         string   `json:"startTime,omitempty" jsonschema:"RFC3339 start of the window"`
	EndTime           string   `json:"endTime,omitempty" jsonschema:"RFC3339 end of the window"`
	ResolutionSeconds int64    `json:"resolutionSeconds,omitempty" jsonschema:"step in seconds"`
}

// RegisterDatastoreMetricsMCP adds the get_datastore_metrics tool — split from
// RegisterMCP only so datastore.go's verb and its MCP fragment read together;
// it still contributes into the same shared MCP registry.
func RegisterDatastoreMetricsMCP(s *Service, srv *mcp.Server) {
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_datastore_metrics",
		Description: "Get disk usage, active-connections, replication-lag (Postgres), and memory/connections (Key Value) metrics for one managed Postgres or Key Value instance, as Render-shaped time-series. bex extension (no Render equivalent).",
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
		if in.StartTime != "" {
			if t, err := time.Parse(time.RFC3339, in.StartTime); err == nil {
				q.Start = t
			}
		}
		if in.EndTime != "" {
			if t, err := time.Parse(time.RFC3339, in.EndTime); err == nil {
				q.End = t
			}
		}
		var all []MetricSeries
		for _, metric := range in.MetricTypes {
			q.Metric = metric
			series, err := s.DatastoreMetrics(ctx, q)
			if err != nil {
				return nil, getMetricsResult{}, err
			}
			for i := range series {
				if series[i].Labels == nil {
					series[i].Labels = map[string]string{}
				}
				series[i].Labels["metric"] = metric
			}
			all = append(all, series...)
		}
		return nil, getMetricsResult{Series: all}, nil
	})
}
