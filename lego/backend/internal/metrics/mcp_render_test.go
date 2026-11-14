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
	"encoding/json"
	"reflect"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// decodeGetMetrics reads get_metrics' Render text: one MetricsResponse.
func decodeGetMetrics(t *testing.T, res *mcp.CallToolResult) renderMetricsResponse {
	t.Helper()
	if res.IsError {
		t.Fatalf("get_metrics: tool error %+v", res.Content)
	}
	if res.StructuredContent != nil {
		t.Fatalf("get_metrics sent structuredContent %v; Render's server sends text only", res.StructuredContent)
	}
	rendertest.AssertRenderMCPResult(t, "get_metrics", res)
	var out renderMetricsResponse
	rendertest.DecodeMCPText(t, res, &out)
	return out
}

// series flattens every metric entry's REST series, in order.
func (r renderMetricsResponse) series() []renderMetricSeries {
	var out []renderMetricSeries
	for _, m := range r.Metrics {
		out = append(out, m.Data...)
	}
	return out
}

// label is a REST series' value for one label field ("" when absent).
func (s renderMetricSeries) label(field string) string {
	for _, l := range s.Labels {
		if l.Field == field {
			return l.Value
		}
	}
	return ""
}

// TestMCPGetMetricsIsRendersMetricsResponse pins get_metrics to Render's MCP
// answer (w4/m378): {resourceId, timeRange, metrics:[{type, data}]}, one entry
// per requested type in the caller's spelling, each data equal to the REST
// endpoint's body for the same read.
func TestMCPGetMetricsIsRendersMetricsResponse(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))
	cs := mcpSession(t, svc)

	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resourceId": "web", "metricTypes": []string{"cpu_usage", "memory_usage"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	var top map[string]json.RawMessage
	rendertest.DecodeMCPText(t, res, &top)
	for _, key := range []string{"resourceId", "timeRange", "metrics"} {
		if _, ok := top[key]; !ok {
			t.Errorf("get_metrics text lacks %q: %s", key, rendertest.MCPText(t, res))
		}
	}
	if _, ok := top["series"]; ok {
		t.Errorf("get_metrics text still carries bex's series: %s", rendertest.MCPText(t, res))
	}

	got := decodeGetMetrics(t, res)
	if got.ResourceID != "web" || got.TimeRange != (renderTimeRange{}) {
		t.Errorf("resourceId/timeRange = %q/%+v, want web and the empty window the call left out", got.ResourceID, got.TimeRange)
	}
	if len(got.Metrics) != 2 || got.Metrics[0].Type != "cpu_usage" || got.Metrics[1].Type != "memory_usage" {
		t.Fatalf("metrics = %+v, want cpu_usage then memory_usage", got.Metrics)
	}
	for path, entry := range map[string]renderMetricData{"/v1/metrics/cpu?resource=web": got.Metrics[0], "/v1/metrics/memory?resource=web": got.Metrics[1]} {
		rest := seriesFrom(t, serveREST(svc, path).Body.Bytes())
		if len(rest) == 0 || !reflect.DeepEqual(entry.Data, rest) {
			t.Errorf("%s data = %+v, want REST's %+v", entry.Type, entry.Data, rest)
		}
		for _, s := range entry.Data {
			if s.label(LabelMetric) != "" {
				t.Errorf("%s series carries bex's metric label: %+v", entry.Type, s.Labels)
			}
		}
	}

	windowed, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resourceId": "web", "metricTypes": []string{"cpu_usage"},
		"startTime": "2026-07-01T00:00:00Z", "endTime": "2026-07-01T01:00:00Z",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if tr := decodeGetMetrics(t, windowed).TimeRange; tr.Start != "2026-07-01T00:00:00Z" || tr.End != "2026-07-01T01:00:00Z" {
		t.Errorf("timeRange = %+v, want the caller's window echoed", tr)
	}
}

// TestMCPGetMetricsLegacyResourceArrayAnswersOnePerResource: bex's legacy
// multi-id resource array gets one MetricsResponse per resource, in order.
func TestMCPGetMetricsLegacyResourceArrayAnswersOnePerResource(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5},
	}), nil, sampleApp("web"), sampleApp("api"), podWithLimits(webInst))
	cs := mcpSession(t, svc)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resource": []string{"web", "api"}, "metricTypes": []string{"cpu_usage"},
	}})
	if err != nil || res.IsError {
		t.Fatalf("get_metrics: %v %+v", err, res)
	}
	var got []renderMetricsResponse
	rendertest.DecodeMCPText(t, res, &got)
	if len(got) != 2 || got[0].ResourceID != "web" || got[1].ResourceID != "api" {
		t.Fatalf("get_metrics = %+v, want one response for web, then api", got)
	}
}

// TestMCPGetMetricsStatusCodeBreakdownIsRESTs: aggregateHttpRequestCountsBy
// statusCode labels each series statusCode, as REST's aggregateBy does, never
// the sources' code, and the data equals REST's body.
func TestMCPGetMetricsStatusCodeBreakdownIsRESTs(t *testing.T) {
	svc := newService(nil, groupedRequestSource(t), sampleApp("web"), podFor("web", webInst))
	cs := mcpSession(t, svc)
	res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resourceId": "web", "metricTypes": []string{"http_request_count"}, "aggregateHttpRequestCountsBy": "statusCode",
	}})
	if err != nil {
		t.Fatal(err)
	}
	got := decodeGetMetrics(t, res)
	rest := seriesFrom(t, serveREST(svc, "/v1/metrics/http-requests?resource=web&aggregateBy=statusCode").Body.Bytes())
	if len(got.Metrics) != 1 || len(rest) != 3 || !reflect.DeepEqual(got.Metrics[0].Data, rest) {
		t.Fatalf("MCP data = %+v, want REST's %+v", got.Metrics, rest)
	}
	for _, s := range got.series() {
		if s.label("statusCode") == "" || s.label("code") != "" {
			t.Errorf("series labels = %+v, want statusCode and no code", s.Labels)
		}
	}
}
