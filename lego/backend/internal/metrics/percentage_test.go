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

// Regression coverage for w5/m90: per-instance CPU/memory percentage
// normalization before replica aggregation, with trustworthy per-timestamp
// denominators. Every test here fails against the pre-m90 defects: dividing an
// aggregate by one latest limit (the dashboard's old client-side math), and
// letting historical samples inherit a replacement pod's or a new rollout's
// limit.

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// fakeClientForLimitE2E seeds the App plus two mixed-limit pods for the
// stub-Prometheus end-to-end test.
func fakeClientForLimitE2E(podA, podB string) client.Client {
	return fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"),
		podWithCPUAndMemoryLimits("web", podA, "500m", "512Mi"),
		podWithCPUAndMemoryLimits("web", podB, "1", "1Gi"),
	).Build()
}

const (
	mixedPodA = "web-a1b2c-d3e4f"
	mixedPodB = "web-a1b2c-d5e6f"
	mixedTS   = "2026-09-08T12:00:00Z"
	mixedTS2  = "2026-09-08T12:01:00Z"
)

// podWithCPUAndMemoryLimits is podFor plus explicit per-pod limits, so
// mixed-limit replicas (a rollout mid-stride) can be reproduced.
func podWithCPUAndMemoryLimits(app, name, cpu, mem string) *corev1.Pod {
	p := podFor(app, name)
	p.Spec.Containers = []corev1.Container{{
		Name: core.AppContainer,
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse(cpu),
			corev1.ResourceMemory: resource.MustParse(mem),
		}},
	}}
	return p
}

// staticLimitSource fakes a ResourceLimitRangeSource returning canned stepped
// limit series (raw pod names in the instance label, like the production
// kube-state-metrics source).
func staticLimitSource(series []MetricSeries) ResourceLimitRangeSource {
	return func(_ context.Context, _ ResourceLimitRangeRequest) ([]MetricSeries, error) {
		return series, nil
	}
}

func limitSeries(pod string, pts ...MetricPoint) MetricSeries {
	return MetricSeries{Labels: map[string]string{"instance": pod}, Points: pts}
}

func usageSeries(pod string, pts ...MetricPoint) MetricSeries {
	return MetricSeries{Labels: map[string]string{"instance": pod}, Points: pts}
}

// mixedLimitService builds a ranged cpu/memory service with two replicas on
// different limits (0.4 of 0.5 CPU vs 0.5 of 1 CPU; 384Mi of 512Mi vs 512Mi
// of 1Gi) and no limit history source, so the pods' current spec limits are
// the denominators.
func mixedLimitService() *Service {
	usage := func(_ context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		var vA, vB float64
		switch req.Metric {
		case MetricCPU:
			vA, vB = 0.4, 0.5
		case MetricMemory:
			vA, vB = float64(384<<20), float64(512<<20)
		default:
			return nil, nil
		}
		return []MetricSeries{
			usageSeries(mixedPodA, MetricPoint{Timestamp: mixedTS, Value: vA}),
			usageSeries(mixedPodB, MetricPoint{Timestamp: mixedTS, Value: vB}),
		}, nil
	}
	return rangeService(usage, nil,
		sampleApp("web"),
		podWithCPUAndMemoryLimits("web", mixedPodA, "500m", "512Mi"),
		podWithCPUAndMemoryLimits("web", mixedPodB, "1", "1Gi"))
}

func mustMetrics(t *testing.T, svc *Service, q MetricQuery) []MetricSeries {
	t.Helper()
	series, err := svc.Metrics(context.Background(), q)
	if err != nil {
		t.Fatalf("Metrics(%+v): %v", q, err)
	}
	return series
}

func closeTo(got, want float64) bool {
	const eps = 1e-9
	d := got - want
	return d < eps && -d < eps
}

// The milestone's headline counterexample: raw 80%/50% per instance, then
// MIN 50%, MAX 80%, AVG 65% — never the single-denominator answers (40%/50%
// raw, or an aggregate divided once by the max limit).
func TestMixedLimitsNormalizeBeforeAggregation(t *testing.T) {
	svc := mixedLimitService()

	byInst := map[string]MetricSeries{}
	for _, s := range mustMetrics(t, svc, MetricQuery{App: "web", Metric: MetricCPU, Percentage: true}) {
		if s.Unit != unitPercentage {
			t.Fatalf("unit = %q, want percentage", s.Unit)
		}
		byInst[s.Labels["instance"]] = s
	}
	if len(byInst) != 2 {
		t.Fatalf("want 2 normalized series, got %+v", byInst)
	}
	idA, idB := ids.ServiceInstanceID("web", mixedPodA), ids.ServiceInstanceID("web", mixedPodB)
	if got := byInst[idA].Points[0].Value; !closeTo(got, 80) {
		t.Errorf("0.4/0.5 CPU = %v, want 80", got)
	}
	if got := byInst[idB].Points[0].Value; !closeTo(got, 50) {
		t.Errorf("0.5/1 CPU = %v, want 50", got)
	}

	for method, want := range map[string]float64{"MIN": 50, "MAX": 80, "AVG": 65} {
		agg := mustMetrics(t, svc, MetricQuery{App: "web", Metric: MetricCPU, Percentage: true, ReplicaAggregate: method})
		if len(agg) != 1 || len(agg[0].Points) != 1 {
			t.Fatalf("%s: %+v", method, agg)
		}
		if got := agg[0].Points[0].Value; !closeTo(got, want) {
			t.Errorf("%s = %v, want %v", method, got, want)
		}
		if agg[0].Unit != unitPercentage {
			t.Errorf("%s unit = %q, want percentage", method, agg[0].Unit)
		}
	}
}

// Equivalent memory case: 75% of 512Mi vs 50% of 1Gi aggregate to 62.5% AVG.
func TestMixedLimitsMemoryNormalizeBeforeAggregation(t *testing.T) {
	svc := mixedLimitService()
	agg := mustMetrics(t, svc, MetricQuery{App: "web", Metric: MetricMemory, Percentage: true, ReplicaAggregate: "AVG"})
	if len(agg) != 1 || len(agg[0].Points) != 1 {
		t.Fatalf("AVG: %+v", agg)
	}
	if got := agg[0].Points[0].Value; !closeTo(got, 62.5) {
		t.Errorf("memory AVG = %v, want 62.5", got)
	}
}

// Total mode preserves absolute samples: no division, no unit change, and the
// replica aggregate averages absolutes (0.45), not percentages.
func TestTotalModePreservesAbsolute(t *testing.T) {
	svc := mixedLimitService()
	raw := mustMetrics(t, svc, MetricQuery{App: "web", Metric: MetricCPU})
	vals := map[string]float64{}
	for _, s := range raw {
		if s.Unit != unitCores {
			t.Fatalf("total unit = %q, want cores", s.Unit)
		}
		vals[s.Labels["instance"]] = s.Points[0].Value
	}
	idA, idB := ids.ServiceInstanceID("web", mixedPodA), ids.ServiceInstanceID("web", mixedPodB)
	if !closeTo(vals[idA], 0.4) || !closeTo(vals[idB], 0.5) {
		t.Fatalf("total raw = %v, want 0.4/0.5", vals)
	}
	agg := mustMetrics(t, svc, MetricQuery{App: "web", Metric: MetricCPU, ReplicaAggregate: "AVG"})
	if len(agg) != 1 || !closeTo(agg[0].Points[0].Value, 0.45) {
		t.Fatalf("total AVG = %+v, want 0.45", agg)
	}
	if agg[0].Unit != unitCores {
		t.Errorf("total AVG unit = %q, want cores", agg[0].Unit)
	}
}

// A rollout mid-window keeps each sample's own denominator: the limit history
// moves 0.5 -> 1.0 at the second timestamp, so identical 0.4 usage reads 80%
// then 40% — history never inherits the new limit.
func TestRolloutKeepsHistoricalDenominator(t *testing.T) {
	usage := staticRangeSource(nil, []MetricSeries{
		usageSeries(mixedPodA,
			MetricPoint{Timestamp: mixedTS, Value: 0.4},
			MetricPoint{Timestamp: mixedTS2, Value: 0.4}),
	})
	svc := rangeService(usage, nil, sampleApp("web"),
		podWithCPUAndMemoryLimits("web", mixedPodA, "1", "1Gi"))
	svc.ResourceLimitRange = staticLimitSource([]MetricSeries{
		limitSeries(mixedPodA,
			MetricPoint{Timestamp: mixedTS, Value: 0.5},
			MetricPoint{Timestamp: mixedTS2, Value: 1}),
	})

	series := mustMetrics(t, svc, MetricQuery{App: "web", Metric: MetricCPU, Percentage: true})
	if len(series) != 1 || len(series[0].Points) != 2 {
		t.Fatalf("want 1 two-point series, got %+v", series)
	}
	if got := series[0].Points[0].Value; !closeTo(got, 80) {
		t.Errorf("pre-rollout point = %v, want 80 (old 0.5 limit)", got)
	}
	if got := series[0].Points[1].Value; !closeTo(got, 40) {
		t.Errorf("post-rollout point = %v, want 40 (new 1.0 limit)", got)
	}
}

// Untrustworthy denominators omit the affected samples, never fabricated:
// a deleted pod (usage, no limit timeline) drops entirely; a sample predating
// the limit history drops while later samples survive; a zero limit drops.
func TestUntrustworthyDenominatorsOmit(t *testing.T) {
	usage := staticRangeSource(nil, []MetricSeries{
		usageSeries("web-gone",
			MetricPoint{Timestamp: mixedTS, Value: 0.4}),
		usageSeries(mixedPodA,
			MetricPoint{Timestamp: "2026-09-08T11:59:00Z", Value: 0.4},
			MetricPoint{Timestamp: mixedTS, Value: 0.4},
			MetricPoint{Timestamp: mixedTS2, Value: 0.4}),
	})
	svc := rangeService(usage, nil, sampleApp("web"),
		podWithCPUAndMemoryLimits("web", mixedPodA, "500m", "512Mi"))
	svc.ResourceLimitRange = staticLimitSource([]MetricSeries{
		limitSeries(mixedPodA,
			MetricPoint{Timestamp: mixedTS, Value: 0.5},
			MetricPoint{Timestamp: mixedTS2, Value: 0}),
	})

	series := mustMetrics(t, svc, MetricQuery{App: "web", Metric: MetricCPU, Percentage: true})
	if len(series) != 1 {
		t.Fatalf("deleted pod must be omitted, zero-limit point dropped: %+v", series)
	}
	if len(series[0].Points) != 1 || series[0].Points[0].Timestamp != mixedTS {
		t.Fatalf("only the trustworthy sample survives: %+v", series[0].Points)
	}
	if got := series[0].Points[0].Value; !closeTo(got, 80) {
		t.Errorf("survivor = %v, want 80", got)
	}
}

// A failing limit history rejects the read — a half-joined percentage would
// be worse than none.
func TestLimitSourceErrorSurfaces(t *testing.T) {
	svc := mixedLimitService()
	svc.ResourceLimitRange = func(_ context.Context, _ ResourceLimitRangeRequest) ([]MetricSeries, error) {
		return nil, context.DeadlineExceeded
	}
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Percentage: true}); err != context.DeadlineExceeded {
		t.Errorf("limit-source error should surface, got %v", err)
	}
	// Absolute reads never touch the limit history.
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU}); err != nil {
		t.Errorf("absolute read should not need limits, got %v", err)
	}
}

func TestPromLimitQueryFor(t *testing.T) {
	req := ResourceLimitRangeRequest{Namespace: "default", App: "web", Resolution: 60 * time.Second}
	matchers := `namespace="default",pod=~"web-[a-z0-9]+-[a-z0-9]{5}|web-[a-z0-9]{59}",container!=""`
	req.Metric = MetricCPU
	if got, want := promLimitQueryFor(req), `sum by (pod) (kube_pod_container_resource_limits{`+matchers+`,resource="cpu"})`; got != want {
		t.Errorf("cpu limit query:\n got %q\nwant %q", got, want)
	}
	req.Metric = MetricMemory
	if got, want := promLimitQueryFor(req), `sum by (pod) (kube_pod_container_resource_limits{`+matchers+`,resource="memory"})`; got != want {
		t.Errorf("memory limit query:\n got %q\nwant %q", got, want)
	}
	if got := promLimitQueryFor(ResourceLimitRangeRequest{Metric: MetricInstanceCount}); got != "" {
		t.Errorf("non-resource metric should yield no limit query, got %q", got)
	}
}

// REST, GraphQL, and MCP agree on the normalized contract: the same mixed
// replicas read 65% AVG on every surface.
func TestPercentageCrossSurfaceParity(t *testing.T) {
	rec := serveREST(mixedLimitService(), "/v1/metrics/cpu?resource=web&percentage=true&aggregateAllMethod=AVG")
	if rec.Code != 200 {
		t.Fatalf("REST: %d %s", rec.Code, rec.Body.String())
	}
	restVals := restValueList(t, rec.Body.Bytes())

	schema, err := gqlSchema(mixedLimitService())
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field:"RESOURCE",values:["web"]}], name:"CPU", percentage:true, aggregateAllMethod:"AVG"}) { values { value } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	gqlVals := gqlValueList(t, res.Data)

	cs := mcpSession(t, mixedLimitService())
	mcpRes, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resourceId": "web", "metricTypes": []string{MetricCPU}, "percentage": true, "aggregateAllMethod": "AVG",
	}})
	if err != nil || mcpRes.IsError {
		t.Fatalf("MCP get_metrics: err=%v result=%+v", err, mcpRes)
	}
	raw, err := json.Marshal(mcpRes.StructuredContent)
	if err != nil {
		t.Fatalf("MCP marshal: %v", err)
	}
	var mcpMetrics getMetricsResult
	if err := json.Unmarshal(raw, &mcpMetrics); err != nil {
		t.Fatalf("MCP decode: %v raw=%s", err, raw)
	}
	var mcpVals []float64
	for _, s := range mcpMetrics.Series {
		for _, p := range s.Points {
			mcpVals = append(mcpVals, p.Value)
		}
	}

	for name, vals := range map[string][]float64{"REST": restVals, "GraphQL": gqlVals, "MCP": mcpVals} {
		if len(vals) != 1 || !closeTo(vals[0], 65) {
			t.Errorf("%s AVG = %v, want [65]", name, vals)
		}
	}
}

// TestLimitHistoryEndToEndOverHTTP drives the real production sources
// (NewPrometheusResourceSource + NewPrometheusResourceLimitSource) against a
// stub Prometheus serving canned cAdvisor + kube-state-metrics matrices, then
// through Metrics with selection + aggregation: the full percentage pipeline
// minus a live cluster. It pins the PromQL family each source queries, the
// pod=>instance relabeling, the per-timestamp join, and the select→
// normalize→aggregate order in one pass.
func TestLimitHistoryEndToEndOverHTTP(t *testing.T) {
	const (
		podA = "web-abc12-def34"
		podB = "web-abc12-def35"
	)
	base := time.Date(2026, 9, 8, 12, 0, 0, 0, time.UTC).Unix()
	var queries []string
	stub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query().Get("query")
		queries = append(queries, q)
		var result string
		switch {
		case strings.Contains(q, "container_cpu_usage_seconds_total"):
			result = fmt.Sprintf(`[
			  {"metric":{"pod":%q},"values":[[%d,"0.4"]]},
			  {"metric":{"pod":%q},"values":[[%d,"0.5"]]}]`, podA, base, podB, base)
		case strings.Contains(q, "kube_pod_container_resource_limits"):
			result = fmt.Sprintf(`[
			  {"metric":{"pod":%q},"values":[[%d,"0.5"]]},
			  {"metric":{"pod":%q},"values":[[%d,"1"]]}]`, podA, base, podB, base)
		default:
			t.Errorf("unexpected PromQL: %q", q)
			result = `[]`
		}
		fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":%s}}`, result)
	}))
	defer stub.Close()

	cl := fakeClientForLimitE2E(podA, podB)
	svc := &Service{
		Base:                 &core.Base{Client: cl, Namespace: "default", Clock: fixedClock},
		ResourceMetricsRange: NewPrometheusResourceSource(stub.URL, stub.Client()),
		ResourceLimitRange:   NewPrometheusResourceLimitSource(stub.URL, stub.Client()),
	}
	start := time.Unix(base, 0).UTC()
	end := start.Add(5 * time.Minute)

	raw := mustMetrics(t, svc, MetricQuery{
		App: "web", Metric: MetricCPU, Percentage: true,
		Start: start, End: end, Resolution: time.Minute,
	})
	byInst := map[string]float64{}
	for _, s := range raw {
		if s.Unit != unitPercentage || len(s.Points) != 1 {
			t.Fatalf("raw: %+v", raw)
		}
		byInst[s.Labels["instance"]] = s.Points[0].Value
	}
	idA, idB := ids.ServiceInstanceID("web", podA), ids.ServiceInstanceID("web", podB)
	if !closeTo(byInst[idA], 80) || !closeTo(byInst[idB], 50) {
		t.Fatalf("raw percentages = %v, want 80/50", byInst)
	}

	// Selection still precedes aggregation: one replica + AVG is that
	// replica's own normalized value, not a blend.
	sel := mustMetrics(t, svc, MetricQuery{
		App: "web", Metric: MetricCPU, Percentage: true,
		Start: start, End: end, Resolution: time.Minute,
		Instances: []string{idA}, ReplicaAggregate: "AVG",
	})
	if len(sel) != 1 || !closeTo(sel[0].Points[0].Value, 80) {
		t.Fatalf("select A + AVG = %+v, want 80", sel)
	}
	avg := mustMetrics(t, svc, MetricQuery{
		App: "web", Metric: MetricCPU, Percentage: true,
		Start: start, End: end, Resolution: time.Minute,
		ReplicaAggregate: "AVG",
	})
	if len(avg) != 1 || !closeTo(avg[0].Points[0].Value, 65) {
		t.Fatalf("AVG = %+v, want 65", avg)
	}

	sawUsage, sawLimits := false, false
	for _, q := range queries {
		sawUsage = sawUsage || strings.Contains(q, "container_cpu_usage_seconds_total")
		sawLimits = sawLimits || strings.Contains(q, "kube_pod_container_resource_limits")
	}
	if !sawUsage || !sawLimits {
		t.Errorf("expected both usage and limit PromQL families, got %q", queries)
	}
}

func restValueList(t *testing.T, body []byte) []float64 {
	t.Helper()
	var series []struct {
		Values []struct {
			Value float64 `json:"value"`
		} `json:"values"`
	}
	if err := json.Unmarshal(body, &series); err != nil {
		t.Fatalf("decode REST body: %v", err)
	}
	var out []float64
	for _, s := range series {
		for _, v := range s.Values {
			out = append(out, v.Value)
		}
	}
	return out
}

func gqlValueList(t *testing.T, data any) []float64 {
	t.Helper()
	raw, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}
	var parsed struct {
		Metrics []struct {
			Values []struct {
				Value float64 `json:"value"`
			} `json:"values"`
		} `json:"metrics"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatalf("decode GraphQL data: %v", err)
	}
	var out []float64
	for _, s := range parsed.Metrics {
		for _, v := range s.Values {
			out = append(out, v.Value)
		}
	}
	return out
}
