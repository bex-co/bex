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
	"errors"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestGetMetricsArgsAliasPolicy(t *testing.T) {
	res := int64(30)
	legacy := int64(60)
	qRender := 0.9
	qLegacy := 0.5

	args := getMetricsArgs{
		ResourceID:          "srv-render",
		Resource:            []string{"srv-legacy", "srv-other"},
		Resolution:          &res,
		ResolutionSeconds:   &legacy,
		HTTPLatencyQuantile: &qRender,
		Quantile:            &qLegacy,
		HTTPHost:            "render.example",
		Host:                "legacy.example",
		HTTPPath:            "/render",
		Path:                "/legacy",
	}
	resources, err := args.resources()
	if err != nil || len(resources) != 1 || resources[0] != "srv-render" {
		t.Fatalf("resources() = %v, %v; want [srv-render]", resources, err)
	}
	if args.resolutionSeconds() != 30 {
		t.Errorf("resolutionSeconds() = %d, want 30", args.resolutionSeconds())
	}
	if args.quantile() != 0.9 {
		t.Errorf("quantile() = %v, want 0.9", args.quantile())
	}
	if args.host() != "render.example" || args.path() != "/render" {
		t.Errorf("host/path = %q/%q, want render spellings", args.host(), args.path())
	}

	legacyOnly := getMetricsArgs{Resource: []string{"a", "b"}, ResolutionSeconds: &legacy, Quantile: &qLegacy, Host: "h", Path: "/p"}
	resources, err = legacyOnly.resources()
	if err != nil || len(resources) != 2 {
		t.Fatalf("legacy resources() = %v, %v", resources, err)
	}
	if legacyOnly.resolutionSeconds() != 60 || legacyOnly.quantile() != 0.5 {
		t.Errorf("legacy-only merge failed: res=%d q=%v", legacyOnly.resolutionSeconds(), legacyOnly.quantile())
	}
}

func TestApplyCPUAggregation(t *testing.T) {
	if err := applyCPUAggregation(""); err != nil {
		t.Errorf("empty: %v", err)
	}
	if err := applyCPUAggregation("AVG"); err != nil {
		t.Errorf("AVG: %v", err)
	}
	for _, bad := range []string{"MAX", "MIN", "p99"} {
		err := applyCPUAggregation(bad)
		if !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s: %v, want ErrBadRequest", bad, err)
		}
	}
}

func TestApplyHTTPRequestAggregate(t *testing.T) {
	got, err := applyHTTPRequestAggregate("statusCode")
	if err != nil || got != "status" {
		t.Fatalf("statusCode = %q, %v; want status", got, err)
	}
	if _, err := applyHTTPRequestAggregate("host"); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("host: %v, want ErrBadRequest", err)
	}
}

func TestMCPGetMetricsRenderSpellings(t *testing.T) {
	var gotApp string
	var gotRes int64
	var gotHost, gotPath, gotGroupBy string
	var gotQuantile float64

	resourceSrc := func(_ context.Context, _, app string) ([]PodResourceUsage, error) {
		gotApp = app
		return []PodResourceUsage{{Pod: app + "-0", CPUCores: 0.1, MemoryBytes: 1024}}, nil
	}
	requestSrc := func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		gotQuantile = req.Quantile
		gotRes = int64(req.Resolution.Seconds())
		gotHost, gotPath, gotGroupBy = req.Host, req.Path, req.GroupBy
		return []MetricSeries{{Unit: "requests", Points: []MetricPoint{{Value: 1}}}}, nil
	}
	svc := newService(resourceSrc, requestSrc, sampleApp("web"))
	svc.RequestLogMetrics = requestSrc // host/path-filtered path (w5/m58)
	cs := mcpSession(t, svc)
	ctx := context.Background()

	result, err := cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_metrics",
		Arguments: map[string]any{
			"resourceId":                "web",
			"resource":                  []string{"ignored-legacy"},
			"metricTypes":               []string{MetricCPU},
			"resolution":                float64(30),
			"resolutionSeconds":         float64(60),
			"cpuUsageAggregationMethod": "AVG",
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("get_metrics resourceId: err=%v result=%+v", err, result)
	}
	if gotApp != "web" {
		t.Errorf("resourceId preference: app=%q, want web", gotApp)
	}

	result, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_metrics",
		Arguments: map[string]any{
			"resourceId":                   "web",
			"metricTypes":                  []string{MetricHTTPRequests},
			"resolution":                   float64(30),
			"resolutionSeconds":            float64(60),
			"httpHost":                     "render.example",
			"host":                         "legacy.example",
			"httpPath":                     "/api",
			"path":                         "/legacy",
			"aggregateHttpRequestCountsBy": "statusCode",
		},
	})
	if err != nil || result.IsError {
		raw, _ := json.Marshal(result)
		t.Fatalf("get_metrics http args: err=%v result=%s", err, raw)
	}
	if gotRes != 30 || gotHost != "render.example" || gotPath != "/api" || gotGroupBy != "status" {
		t.Errorf("request merge: res=%d host=%q path=%q groupBy=%q", gotRes, gotHost, gotPath, gotGroupBy)
	}

	result, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_metrics",
		Arguments: map[string]any{
			"resourceId":            "web",
			"metricTypes":           []string{MetricHTTPLatency},
			"httpLatencyQuantile":   0.9,
			"quantile":              0.5,
			"resolution":            float64(15),
		},
	})
	if err != nil || result.IsError {
		raw, _ := json.Marshal(result)
		t.Fatalf("get_metrics latency quantile: err=%v result=%s", err, raw)
	}
	if gotQuantile != 0.9 {
		t.Errorf("httpLatencyQuantile preference = %v, want 0.9", gotQuantile)
	}

	result, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_metrics",
		Arguments: map[string]any{
			"resourceId":                "web",
			"metricTypes":               []string{MetricCPU},
			"cpuUsageAggregationMethod": "MAX",
		},
	})
	if err == nil && !result.IsError {
		t.Fatal("cpuUsageAggregationMethod=MAX should fail")
	}
	if result != nil && result.IsError {
		msg, _ := json.Marshal(result.Content)
		if !strings.Contains(string(msg), "cpuUsageAggregationMethod") {
			t.Errorf("MAX error should name the arg, got %s", msg)
		}
	}

	result, err = cs.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_metrics",
		Arguments: map[string]any{
			"resourceId":                   "web",
			"metricTypes":                  []string{MetricHTTPRequests},
			"aggregateHttpRequestCountsBy": "host",
		},
	})
	if err == nil && (result == nil || !result.IsError) {
		t.Fatal("aggregateHttpRequestCountsBy=host should fail")
	}
}
