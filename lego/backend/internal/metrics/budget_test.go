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
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// codex r7 #13 / w4/029 #14 — MaxQueryHours is enforced in the SHARED
// service, and every adapter checks the same fan-out budget before its read
// loop (the resource/metric arrays exist only adapter-side), so REST,
// GraphQL, and MCP reject the same over-budget request identically, before
// any Prometheus/Loki work.

const (
	budgetWindowStart = "2026-01-01T00:00:00Z"
	budgetWindowEnd   = "2026-01-01T02:00:01Z" // 2h1s > the 1h test cap
)

// budgetService builds a Service with a 1-hour window cap and sources that
// fail the test if any backend read happens — every case below must be
// refused before backend work.
func budgetService(t *testing.T) *Service {
	t.Helper()
	svc := newService(
		func(_ context.Context, _, _ string) ([]PodResourceUsage, error) {
			t.Error("resource source consulted for an over-budget request")
			return nil, nil
		},
		func(_ context.Context, _ RequestMetricsRequest) ([]MetricSeries, error) {
			t.Error("request source consulted for an over-budget request")
			return nil, nil
		},
		sampleApp("web"))
	svc.MaxQueryHours = 1
	return svc
}

func mcpSession(t *testing.T, svc *Service) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("MCP server connect: %v", err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("MCP client connect: %v", err)
	}
	t.Cleanup(func() { cs.Close() })
	return cs
}

func TestQueryWindowEnforcedAcrossAllAdapters(t *testing.T) {
	svc := budgetService(t)
	start, _ := time.Parse(time.RFC3339, budgetWindowStart)
	end, _ := time.Parse(time.RFC3339, budgetWindowEnd)

	// Service (the shared seam every adapter calls).
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Start: start, End: end}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("Metrics over-window error = %v, want ErrBadRequest", err)
	}
	// The datastore sibling refuses before touching the resource (no Database
	// fixture exists here, yet the window is still what rejects first).
	if _, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{Kind: DatastoreDatabase, Resource: "db", Metric: MetricDisk, Start: start, End: end}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("DatastoreMetrics over-window error = %v, want ErrBadRequest", err)
	}

	// REST.
	rec := serveREST(svc, "/v1/metrics/cpu?resource=web&startTime="+budgetWindowStart+"&endTime="+url.QueryEscape(budgetWindowEnd))
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "query range exceeds") {
		t.Errorf("REST over-window = %d %q, want 400 with a range message", rec.Code, rec.Body.String())
	}

	// GraphQL.
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU", start: "` + budgetWindowStart + `", end: "` + budgetWindowEnd + `"}) { unit } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "query range exceeds") {
		t.Errorf("GraphQL over-window errors = %v, want a range refusal", res.Errors)
	}

	// MCP.
	cs := mcpSession(t, svc)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resource": []string{"web"}, "metricTypes": []string{MetricCPU},
		"startTime": budgetWindowStart, "endTime": budgetWindowEnd,
	}})
	if err != nil || !result.IsError {
		t.Errorf("MCP over-window = err %v isError %v, want a tool error", err, result != nil && result.IsError)
	}
}

func TestMetricsFanOutBudget(t *testing.T) {
	svc := budgetService(t)

	// REST: 101 resources in one request.
	v := url.Values{}
	for i := 0; i < maxMetricsFanOut+1; i++ {
		v.Add("resource", "web")
	}
	rec := serveREST(svc, "/v1/metrics/cpu?"+v.Encode())
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "limit") {
		t.Errorf("REST fan-out = %d %q, want 400", rec.Code, rec.Body.String())
	}

	// REST: the latency-percentile multiplier counts toward the same product
	// (11 resources × 10 quantiles = 110 reads > 100).
	v = url.Values{}
	for i := 0; i < 11; i++ {
		v.Add("resource", "web")
	}
	for i := 1; i <= 10; i++ {
		v.Add("quantile", "0."+strings.Repeat("9", i))
	}
	rec = serveREST(svc, "/v1/metrics/http-latency?"+v.Encode())
	if rec.Code != 400 || !strings.Contains(rec.Body.String(), "limit") {
		t.Errorf("REST latency-quantile fan-out = %d %q, want 400", rec.Code, rec.Body.String())
	}

	// GraphQL: same budget on the RESOURCE filter array.
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	values := strings.Repeat(`"web",`, maxMetricsFanOut) + `"web"`
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: [` + values + `]}], name: "CPU"}) { unit } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "limit") {
		t.Errorf("GraphQL fan-out errors = %v, want a budget refusal", res.Errors)
	}

	// MCP: the resources × metricTypes product is what is bounded.
	cs := mcpSession(t, svc)
	resources := make([]string, 11)
	for i := range resources {
		resources[i] = "web"
	}
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resource":    resources,
		"metricTypes": []string{MetricCPU, MetricMemory, MetricInstanceCount, MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth, MetricCPUTarget, MetricMemoryTarget, MetricCPULimit, MetricMemoryLimit},
	}})
	if err != nil || !result.IsError {
		t.Errorf("MCP fan-out = err %v isError %v, want a tool error", err, result != nil && result.IsError)
	}
}

func TestLatencyQuantileBudget(t *testing.T) {
	svc := budgetService(t)
	quants := make([]float64, maxLatencyQuantiles+1)
	for i := range quants {
		quants[i] = float64(i+1) / float64(len(quants)+1)
	}
	if _, err := svc.MetricsWithQuantiles(context.Background(), MetricQuery{App: "web", Metric: MetricHTTPLatency, Quantiles: quants}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("quantile budget error = %v, want ErrBadRequest", err)
	}
}

// A malformed timestamp must surface as an error on the GraphQL and MCP
// adapters (as it always has on REST), never silently fall back to the
// default window and slip past the range cap.
func TestMalformedWindowTimestampRejected(t *testing.T) {
	svc := budgetService(t)

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU", start: "yesterday-ish"}) { unit } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "RFC3339") {
		t.Errorf("GraphQL malformed start errors = %v, want an RFC3339 refusal", res.Errors)
	}

	cs := mcpSession(t, svc)
	result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resource": []string{"web"}, "metricTypes": []string{MetricCPU}, "startTime": "yesterday-ish",
	}})
	if err != nil || !result.IsError {
		t.Errorf("MCP malformed start = err %v isError %v, want a tool error", err, result != nil && result.IsError)
	}
}
