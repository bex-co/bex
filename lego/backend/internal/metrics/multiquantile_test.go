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
	"fmt"
	"strconv"
	"testing"

	"github.com/graphql-go/graphql"
)

// A request source that echoes the quantile it saw as the point value, so a test
// can map each returned series back to the percentile that produced it.
func echoQuantileSource(calls *[]float64) RequestMetricsSource {
	return func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
		*calls = append(*calls, r.Quantile)
		return []MetricSeries{{Unit: unitSeconds, Points: []MetricPoint{{Value: r.Quantile}}}}, nil
	}
}

func TestNormalizeQuantiles(t *testing.T) {
	got := normalizeQuantiles([]float64{0.99, 0.5, 0.9, 0.5, 0, 1, 1.5, -0.2})
	if fmt.Sprint(got) != fmt.Sprint([]float64{0.5, 0.9, 0.99}) {
		t.Errorf("normalizeQuantiles dedup+sort+range = %v, want [0.5 0.9 0.99]", got)
	}
	if len(normalizeQuantiles(nil)) != 0 {
		t.Error("nil quantiles should normalize to empty")
	}
}

// TestMetricsWithQuantilesFanOut is t001's core: http_latency with several
// quantiles reads each once and tags every series; a single quantile (or any
// other metric) stays byte-identical to Metrics — no fan-out, no quantile label.
func TestMetricsWithQuantilesFanOut(t *testing.T) {
	var calls []float64
	svc := newService(nil, echoQuantileSource(&calls), sampleApp("web"))

	// Several percentiles (unsorted, with a dup): one deduped, sorted series each.
	out, err := svc.MetricsWithQuantiles(context.Background(), MetricQuery{
		App: "web", Metric: MetricHTTPLatency, Quantiles: []float64{0.99, 0.5, 0.9, 0.5},
	})
	if err != nil {
		t.Fatalf("multi: %v", err)
	}
	want := []float64{0.5, 0.9, 0.99}
	if len(out) != len(want) {
		t.Fatalf("want %d overlaid series, got %d", len(want), len(out))
	}
	for i, qs := range out {
		if !qs.HasQuantile || qs.Quantile != want[i] {
			t.Errorf("series %d: HasQuantile=%v Quantile=%v, want %v", i, qs.HasQuantile, qs.Quantile, want[i])
		}
		if qs.Labels["quantile"] != strconv.FormatFloat(want[i], 'g', -1, 64) {
			t.Errorf("series %d: quantile label = %q, want %v", i, qs.Labels["quantile"], want[i])
		}
		if qs.Points[0].Value != want[i] {
			t.Errorf("series %d: source read quantile %v, series value = %v", i, want[i], qs.Points[0].Value)
		}
	}
	if len(calls) != len(want) {
		t.Errorf("source should be read once per quantile, got %d reads", len(calls))
	}

	// No Quantiles: single path — the default quantile echoed, no quantile label.
	calls = nil
	out, err = svc.MetricsWithQuantiles(context.Background(), MetricQuery{App: "web", Metric: MetricHTTPLatency})
	if err != nil || len(out) != 1 {
		t.Fatalf("single: err=%v len=%d", err, len(out))
	}
	if !out[0].HasQuantile || out[0].Quantile != defaultQuantile {
		t.Errorf("single latency should echo the default quantile, got %v", out[0].Quantile)
	}
	if _, tagged := out[0].Labels["quantile"]; tagged {
		t.Error("single-quantile series must not carry a quantile label")
	}
	if len(calls) != 1 || calls[0] != defaultQuantile {
		t.Errorf("single latency should read the default quantile once, got %v", calls)
	}

	// A lone explicit quantile is still the single path (no label, that quantile).
	calls = nil
	out, _ = svc.MetricsWithQuantiles(context.Background(), MetricQuery{App: "web", Metric: MetricHTTPLatency, Quantiles: []float64{0.9}})
	if len(out) != 1 || out[0].Quantile != 0.9 {
		t.Errorf("lone quantile: %+v", out)
	}
	if _, tagged := out[0].Labels["quantile"]; tagged {
		t.Error("lone quantile must not carry a label")
	}
	if len(calls) != 1 || calls[0] != 0.9 {
		t.Errorf("lone quantile should read 0.9 once, got %v", calls)
	}

	// A non-latency metric ignores Quantiles entirely — one read, no quantile echo.
	var reqCalls []RequestMetricsRequest
	nonLatency := newService(nil, func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
		reqCalls = append(reqCalls, r)
		return []MetricSeries{{Points: []MetricPoint{{Value: 1}}}}, nil
	}, sampleApp("web"))
	out, _ = nonLatency.MetricsWithQuantiles(context.Background(), MetricQuery{App: "web", Metric: MetricHTTPRequests, Quantiles: []float64{0.5, 0.9}})
	if len(out) != 1 || out[0].HasQuantile {
		t.Errorf("non-latency should not fan out or echo a quantile: %+v", out)
	}
	if len(reqCalls) != 1 {
		t.Errorf("non-latency should read once regardless of Quantiles, got %d reads", len(reqCalls))
	}
}

// TestRESTMultiQuantileLatency: repeated ?quantile= params overlay one labeled
// series per percentile; a single quantile stays byte-identical (no label).
func TestRESTMultiQuantileLatency(t *testing.T) {
	var calls []float64
	svc := newService(nil, echoQuantileSource(&calls), sampleApp("web"))

	var series []renderMetricSeries
	_ = json.Unmarshal(serveREST(svc, "/v1/metrics/http-latency?resource=web&quantile=0.5&quantile=0.9&quantile=0.99").Body.Bytes(), &series)
	if len(series) != 3 {
		t.Fatalf("percentile All should overlay 3 series, got %d", len(series))
	}
	for _, s := range series {
		if labelValue(s.Labels, "quantile") == "" {
			t.Errorf("overlaid series should carry a quantile label: %+v", s.Labels)
		}
	}

	series = nil
	_ = json.Unmarshal(serveREST(svc, "/v1/metrics/http-latency?resource=web&quantile=0.9").Body.Bytes(), &series)
	if len(series) != 1 {
		t.Fatalf("single quantile => 1 series, got %d", len(series))
	}
	if labelValue(series[0].Labels, "quantile") != "" {
		t.Error("single-quantile series must not carry a quantile label")
	}
}

// TestGraphQLMultiQuantileLatency: several `parameters` entries overlay a series
// per percentile, each echoing its own quantile back through `parameters`.
func TestGraphQLMultiQuantileLatency(t *testing.T) {
	var calls []float64
	svc := newService(nil, echoQuantileSource(&calls), sampleApp("web"))
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "HTTP_LATENCY", parameters: [{quantile: 0.5}, {quantile: 0.9}, {quantile: 0.99}]}) { parameters { quantile } values { value } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	list := res.Data.(map[string]any)["metrics"].([]any)
	if len(list) != 3 {
		t.Fatalf("percentile All should overlay 3 series, got %d", len(list))
	}
	seen := map[float64]bool{}
	for _, item := range list {
		params := item.(map[string]any)["parameters"].([]any)
		if len(params) != 1 {
			t.Fatalf("each series echoes exactly one quantile, got %d", len(params))
		}
		seen[params[0].(map[string]any)["quantile"].(float64)] = true
	}
	for _, q := range []float64{0.5, 0.9, 0.99} {
		if !seen[q] {
			t.Errorf("missing overlaid quantile %v (got %v)", q, seen)
		}
	}
}

func labelValue(labels []renderMetricLabel, field string) string {
	for _, l := range labels {
		if l.Field == field {
			return l.Value
		}
	}
	return ""
}
