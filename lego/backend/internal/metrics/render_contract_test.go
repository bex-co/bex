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
	"net/http"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// w1/m155: Render's metrics contract on REST and MCP. At filing time REST
// aggregateBy=statusCode silently returned the single ungrouped series, eight
// Render /metrics paths were bare 404s, and MCP refused Render's metric names.

// groupedRequestSource answers http_requests with one series per status code
// when the query asks for the status breakdown, and one total otherwise —
// the shape the Prometheus and Loki sources produce (label `code`).
func groupedRequestSource(t *testing.T) RequestMetricsSource {
	t.Helper()
	return func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		point := []MetricPoint{{Timestamp: "2026-09-14T19:56:00Z", Value: 1}}
		if req.GroupBy != groupByStatus {
			return []MetricSeries{{Labels: map[string]string{}, Unit: unitCount, Points: point}}, nil
		}
		var out []MetricSeries
		for _, code := range []string{"200", "404", "501"} {
			out = append(out, MetricSeries{Labels: map[string]string{"code": code}, Unit: unitCount, Points: point})
		}
		return out, nil
	}
}

func seriesFrom(t *testing.T, body []byte) []renderMetricSeries {
	t.Helper()
	var series []renderMetricSeries
	if err := json.Unmarshal(body, &series); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return series
}

func TestRESTAggregateByStatusCodeBreaksRequestsDown(t *testing.T) {
	svc := newService(nil, groupedRequestSource(t), sampleApp("web"), podFor("web", webInst))

	rec := serveREST(svc, "/v1/metrics/http-requests?resource=web&aggregateBy=statusCode")
	if rec.Code != http.StatusOK {
		t.Fatalf("aggregateBy=statusCode = %d %s", rec.Code, rec.Body)
	}
	series := seriesFrom(t, rec.Body.Bytes())
	if len(series) != 3 {
		t.Fatalf("series = %d, want one per status code: %+v", len(series), series)
	}
	var codes []string
	for _, s := range series {
		codes = append(codes, labelValue(s.Labels, "statusCode"))
		if labelValue(s.Labels, "code") != "" {
			t.Errorf("series %+v still carries the source's code label", s.Labels)
		}
	}
	if strings.Join(codes, ",") != "200,404,501" {
		t.Errorf("statusCode labels = %v, want 200,404,501", codes)
	}

	// Control: no aggregateBy keeps the single series.
	rec = serveREST(svc, "/v1/metrics/http-requests?resource=web")
	if got := seriesFrom(t, rec.Body.Bytes()); len(got) != 1 {
		t.Errorf("no aggregateBy = %d series, want 1", len(got))
	}
}

func TestRESTAggregateByHostIsACodedRefusal(t *testing.T) {
	svc := newService(nil, groupedRequestSource(t), sampleApp("web"), podFor("web", webInst))

	rec := serveREST(svc, "/v1/metrics/http-requests?resource=web&aggregateBy=host")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "aggregateBy=host is unsupported") {
		t.Fatalf("aggregateBy=host = %d %s, want a 400 naming the unsupported breakdown", rec.Code, rec.Body)
	}
}

func TestRESTCPUAggregationMethodIsHonoredOrRefused(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))

	if rec := serveREST(svc, "/v1/metrics/cpu?resource=web&aggregationMethod=AVG"); rec.Code != http.StatusOK {
		t.Errorf("aggregationMethod=AVG = %d %s, want 200", rec.Code, rec.Body)
	}
	if rec := serveREST(svc, "/v1/metrics/cpu?resource=web&aggregationMethod=MAX"); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "aggregationMethod") {
		t.Errorf("aggregationMethod=MAX = %d %s, want a coded 400", rec.Code, rec.Body)
	}
}

func TestRESTLimitPathsAreServed(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))

	for path, metric := range map[string]string{"cpu-limit": MetricCPULimit, "memory-limit": MetricMemoryLimit} {
		rec := serveREST(svc, "/v1/metrics/"+path+"?resource=web")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, rec.Code, rec.Body)
		}
		want, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: metric})
		if err != nil {
			t.Fatal(err)
		}
		got := seriesFrom(t, rec.Body.Bytes())
		if len(got) != len(want) || len(got) == 0 || got[0].Values[0].Value != want[0].Points[0].Value {
			t.Errorf("%s = %+v, want the Metrics verb's %+v", path, got, want)
		}
	}
}

func TestRESTFilterPathsUseRenderShapes(t *testing.T) {
	svc := newService(nil, nil, sampleApp("web"), podFor("web", webInst))
	svc.MetricsFilterValuesSource = func(_ context.Context, req MetricsFilterValuesRequest) ([]string, error) {
		if label := req.Label; label != "code" {
			t.Errorf("filter values label = %q, want code", label)
		}
		return []string{"200", "404"}, nil
	}

	var app []renderFilterValues
	rec := serveREST(svc, "/v1/metrics/filters/application?resource=web")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &app) != nil ||
		len(app) != 1 || app[0].Filter != "instance" || len(app[0].Values) != 1 {
		t.Fatalf("filters/application = %d %s, want [{filter: instance, values: [one instance]}]", rec.Code, rec.Body)
	}

	var httpFilters []renderFilterValues
	rec = serveREST(svc, "/v1/metrics/filters/http?resource=web")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &httpFilters) != nil ||
		len(httpFilters) != 2 || httpFilters[0].Filter != "statusCode" ||
		strings.Join(httpFilters[0].Values, ",") != "200,404" || httpFilters[1].Filter != "host" {
		t.Fatalf("filters/http = %d %s, want statusCode [200 404] and host", rec.Code, rec.Body)
	}
	if rec := serveREST(svc, "/v1/metrics/filters/http?resource=web&statusCode=404"); rec.Code != http.StatusBadRequest {
		t.Errorf("narrowed filters/http = %d, want a coded 400 rather than an ignored narrowing", rec.Code)
	}

	rec = serveREST(svc, "/v1/metrics/filters/path?resource=web")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("filters/path = %d %s, want []", rec.Code, rec.Body)
	}

	// Every filter read authorizes its resource: an unknown one is 404, never [].
	for _, path := range []string{"application", "http", "path"} {
		if rec := serveREST(svc, "/v1/metrics/filters/"+path+"?resource=nope"); rec.Code != http.StatusNotFound {
			t.Errorf("filters/%s unknown resource = %d, want 404", path, rec.Code)
		}
	}
}

func TestRESTDatastorePathsInferTheKindFromTheID(t *testing.T) {
	for resource, want := range map[string]string{
		"dpg-c185th5c2rvvnhbfiltg": DatastoreDatabase,
		"red-c185th5c2rvvnhbfiltg": DatastoreKeyValue,
		"srv-c185th5c2rvvnhbfiltg": DatastoreService,
		"legacy-db-name":           DatastoreDatabase,
	} {
		if got := datastoreKindFor(resource); got != want {
			t.Errorf("datastoreKindFor(%q) = %q, want %q", resource, got, want)
		}
	}

	svc := newService(nil, nil)
	rec := serveREST(svc, "/v1/metrics/active-connections?resource=srv-c185th5c2rvvnhbfiltg")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Postgres (dpg-) or Key Value (red-)") {
		t.Errorf("active-connections on a service = %d %s, want a coded 400", rec.Code, rec.Body)
	}
}

func TestRESTBandwidthSourcesIsACodedRefusal(t *testing.T) {
	rec := serveREST(newService(nil, nil), "/v1/metrics/bandwidth-sources?resource=web")
	if rec.Code != http.StatusNotImplemented || !strings.Contains(rec.Body.String(), "monthToDateBandwidth") {
		t.Errorf("bandwidth-sources = %d %s, want a 501 naming the bex reads to use", rec.Code, rec.Body)
	}
}

func TestMCPGetMetricsAcceptsRenderMetricTypes(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))
	cs := mcpSession(t, svc)

	call := func(metricTypes ...string) *mcp.CallToolResult {
		t.Helper()
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "get_metrics",
			Arguments: map[string]any{"resourceId": "web", "metricTypes": metricTypes},
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	decode := func(result *mcp.CallToolResult) getMetricsResult {
		t.Helper()
		var out getMetricsResult
		raw, _ := json.Marshal(result.StructuredContent)
		if err := json.Unmarshal(raw, &out); err != nil {
			t.Fatalf("decode %s: %v", raw, err)
		}
		return out
	}

	for render, bex := range map[string]string{"cpu_usage": MetricCPU, "memory_usage": MetricMemory, "cpu_limit": MetricCPULimit} {
		renderResult, bexResult := call(render), call(bex)
		if renderResult.IsError || bexResult.IsError {
			t.Fatalf("%s/%s: isError render=%v bex=%v", render, bex, renderResult.IsError, bexResult.IsError)
		}
		got, want := decode(renderResult).Series, decode(bexResult).Series
		if len(got) == 0 || len(got) != len(want) || got[0].Points[0].Value != want[0].Points[0].Value {
			t.Errorf("%s = %+v, want the same series as %s %+v", render, got, bex, want)
		}
		if got[0].Labels[LabelMetric] != render {
			t.Errorf("%s series labelled metric=%q, want the requested name", render, got[0].Labels[LabelMetric])
		}
	}

	refused, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_metrics",
		Arguments: map[string]any{"resourceId": "srv-c185th5c2rvvnhbfiltg", "metricTypes": []string{"active_connections"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := json.Marshal(refused.Content); !refused.IsError || !strings.Contains(string(text), "Postgres (dpg-) or Key Value (red-)") {
		t.Errorf("active_connections on a service = %s, want the coded refusal naming the datastore kinds", text)
	}
	if result := call("not_a_metric"); !result.IsError {
		t.Error("an unknown metric type must still error")
	}
}

// renderMetricsTypes pins Render's MCP get_metrics enum
// (render-oss/render-mcp-server pkg/metrics/tools.go): every name maps to a
// metric id the Metrics verb or the datastore route knows, so dropping one is a
// test failure rather than a silent regression.
func TestEveryRenderMCPMetricTypeIsKnown(t *testing.T) {
	known := map[string]bool{
		MetricCPU: true, MetricMemory: true, MetricInstanceCount: true, MetricHTTPRequests: true,
		MetricHTTPLatency: true, MetricBandwidth: true, MetricCPULimit: true, MetricMemoryLimit: true,
		MetricCPUTarget: true, MetricMemoryTarget: true,
	}
	for _, name := range []string{
		"cpu_usage", "memory_usage", "http_request_count", "active_connections", "instance_count",
		"http_latency", "cpu_limit", "cpu_target", "memory_limit", "memory_target", "bandwidth_usage",
	} {
		if name == renderActiveConnections {
			continue
		}
		if !known[metricIDFor(name)] {
			t.Errorf("Render metric type %q maps to %q, which the Metrics verb does not serve", name, metricIDFor(name))
		}
	}
}
