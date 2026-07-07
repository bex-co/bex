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

package api

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// webInst is one App instance (pod) name, reused across the metric assertions.
const webInst = "web-1"

// podWithLimits is podFor("web", name) plus a 1-core / 1Gi limit on the app
// container, so percentage-mode resource metrics have a denominator (0.5 core =>
// 50%, 512Mi => 50%).
func podWithLimits(name string) *corev1.Pod {
	p := podFor("web", name)
	p.Spec.Containers = []corev1.Container{{
		Name: appContainer,
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		}},
	}}
	return p
}

// staticResourceMetrics fakes a ResourceMetricsSource with canned per-pod usage.
func staticResourceMetrics(usage map[string]PodResourceUsage) ResourceMetricsSource {
	return func(_ context.Context, _, _ string) ([]PodResourceUsage, error) {
		out := make([]PodResourceUsage, 0, len(usage))
		for pod, u := range usage {
			u.Pod = pod
			out = append(out, u)
		}
		return out, nil
	}
}

// metricServer wires a Server whose Core has the given metric sources.
func metricServer(t *testing.T, rm ResourceMetricsSource, req RequestMetricsSource, objs ...client.Object) http.Handler {
	t.Helper()
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).Build()
	srv := &Server{
		Core: &Core{
			Client:          cl,
			Namespace:       "default",
			Now:             func() time.Time { return time.Unix(1_000_000, 0).UTC() },
			ResourceMetrics: rm,
			RequestMetrics:  req,
		},
		HydraAdminURL: fakeHydraURL(t),
	}
	h, err := srv.Handler()
	if err != nil {
		t.Fatalf("Handler: %v", err)
	}
	return h
}

// --- Core resource metrics ---

func TestCore_ResourceMetricsAbsoluteAndPercentage(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"),
		podWithLimits(webInst),
		podWithLimits("web-2"),
	).Build()
	core := &Core{
		Client: cl, Namespace: "default",
		Now: func() time.Time { return time.Unix(1_000_000, 0).UTC() },
		ResourceMetrics: staticResourceMetrics(map[string]PodResourceUsage{
			webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20}, // 0.5 core, 512Mi
			"web-2": {CPUCores: 0.25, MemoryBytes: 256 << 20},
		}),
	}

	// Absolute CPU: one series per instance, ordered by instance.
	abs, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU})
	if err != nil {
		t.Fatalf("cpu: %v", err)
	}
	if len(abs) != 2 || abs[0].Labels["instance"] != webInst || abs[0].Unit != unitCores {
		t.Fatalf("unexpected cpu series: %+v", abs)
	}
	if abs[0].Points[0].Value != 0.5 {
		t.Errorf("web-1 cpu should be 0.5 cores, got %v", abs[0].Points[0].Value)
	}

	// Percentage CPU: 0.5 core of a 1-core limit => 50%.
	pct, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Percentage: true})
	if err != nil {
		t.Fatalf("cpu%%: %v", err)
	}
	if pct[0].Unit != unitPercentage || pct[0].Points[0].Value != 50 {
		t.Errorf("web-1 cpu%% should be 50, got %+v", pct[0])
	}

	// Percentage memory: 512Mi of 1Gi => 50%.
	mem, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricMemory, Percentage: true})
	if err != nil {
		t.Fatalf("mem%%: %v", err)
	}
	if mem[0].Points[0].Value != 50 {
		t.Errorf("web-1 mem%% should be 50, got %v", mem[0].Points[0].Value)
	}
}

func TestCore_ResourceLimitMetricNeedsNoResourceMetricsSource(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podWithLimits(webInst), podWithLimits("web-2"),
	).Build()
	// No ResourceMetrics source wired — limits come from the pod spec, not
	// metrics-server, so cpu_limit/memory_limit must still work.
	core := &Core{Client: cl, Namespace: "default", Now: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}

	cpuLim, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPULimit})
	if err != nil {
		t.Fatalf("cpu_limit: %v", err)
	}
	if len(cpuLim) != 2 || cpuLim[0].Labels["instance"] != webInst || cpuLim[0].Unit != unitCores {
		t.Fatalf("unexpected cpu_limit series: %+v", cpuLim)
	}
	if cpuLim[0].Points[0].Value != 1 {
		t.Errorf("web-1 cpu_limit should be 1 core, got %v", cpuLim[0].Points[0].Value)
	}

	memLim, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricMemoryLimit})
	if err != nil {
		t.Fatalf("memory_limit: %v", err)
	}
	if memLim[0].Unit != unitBytes || memLim[0].Points[0].Value != float64(1<<30) { // 1Gi
		t.Errorf("web-1 memory_limit should be 1Gi bytes, got %+v", memLim[0])
	}
}

func TestCore_ResourceLimitMetricOmitsPodsWithNoLimit(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podFor("web", webInst), // no container limits set
	).Build()
	core := &Core{Client: cl, Namespace: "default"}

	series, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPULimit})
	if err != nil {
		t.Fatalf("cpu_limit: %v", err)
	}
	if len(series) != 0 {
		t.Errorf("a pod with no limit should be omitted, not zeroed: %+v", series)
	}
}

func TestCore_AggregateMaxCollapsesToOneSeries(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podWithLimits(webInst), podWithLimits("web-2"),
	).Build()
	core := &Core{Client: cl, Namespace: "default", Now: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}

	series, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPULimit, AggregateMax: true})
	if err != nil {
		t.Fatalf("cpu_limit aggregateMax: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("AggregateMax should collapse to exactly one series, got %d", len(series))
	}
	if _, hasInstance := series[0].Labels["instance"]; hasInstance {
		t.Errorf("the collapsed series should drop the per-instance label: %+v", series[0].Labels)
	}
	if series[0].Points[0].Value != 1 {
		t.Errorf("both replicas have a 1-core limit, max should be 1, got %v", series[0].Points[0].Value)
	}
}

func TestCore_InstanceCountNeedsNoSource(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podFor("web", webInst), podFor("web", "web-2"),
	).Build()
	// No ResourceMetrics source wired — instance count must still work.
	core := &Core{Client: cl, Namespace: "default", Now: func() time.Time { return time.Unix(1_000_000, 0).UTC() }}

	series, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricInstanceCount})
	if err != nil {
		t.Fatalf("instance_count: %v", err)
	}
	if len(series) != 1 || series[0].Unit != unitCount || series[0].Points[0].Value != 2 {
		t.Fatalf("instance count should be 2 in one series: %+v", series)
	}
}

func TestCore_MetricsErrors(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(sampleApp("web")).Build()
	core := &Core{Client: cl, Namespace: "default"}

	// cpu/memory with no source => ErrMetricsUnavailable.
	if _, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU}); err != ErrMetricsUnavailable {
		t.Errorf("cpu without source => ErrMetricsUnavailable, got %v", err)
	}
	// request metric with no source => ErrMetricsUnavailable.
	if _, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricHTTPRequests}); err != ErrMetricsUnavailable {
		t.Errorf("http_requests without source => ErrMetricsUnavailable, got %v", err)
	}
	// unknown app => ErrNotFound, like Get.
	if _, err := core.Metrics(context.Background(), MetricQuery{App: "nope", Metric: MetricInstanceCount}); err != ErrNotFound {
		t.Errorf("unknown app => ErrNotFound, got %v", err)
	}
	// unknown metric => error.
	if _, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: "bogus"}); err == nil {
		t.Error("unknown metric should error")
	}
}

// --- REST metrics adapter (Render metrics-API shape) ---

func TestREST_ResourceMetrics(t *testing.T) {
	h := metricServer(t, staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))

	// Render metrics response is an array of {labels[], unit, values[]}.
	var series []renderMetricSeries
	decode(t, do(t, h, "GET", "/v1/metrics/cpu?resource=web", testToken, ""), &series)
	if len(series) != 1 {
		t.Fatalf("want 1 series, got %d: %+v", len(series), series)
	}
	if series[0].Unit != unitCores || len(series[0].Values) != 1 || series[0].Values[0].Value != 0.5 {
		t.Errorf("unexpected cpu series: %+v", series[0])
	}
	// labels are the {field,value} array, sorted (instance before resource).
	if len(series[0].Labels) != 2 || series[0].Labels[0].Field != "instance" || series[0].Labels[1].Field != "resource" {
		t.Errorf("labels should be sorted {field,value}: %+v", series[0].Labels)
	}

	// percentage=true switches the unit and value.
	decode(t, do(t, h, "GET", "/v1/metrics/cpu?resource=web&percentage=true", testToken, ""), &series)
	if series[0].Unit != unitPercentage || series[0].Values[0].Value != 50 {
		t.Errorf("percentage cpu should be 50%%, got %+v", series[0])
	}

	// instance-count endpoint works without a metrics source path.
	decode(t, do(t, h, "GET", "/v1/metrics/instance-count?resource=web", testToken, ""), &series)
	if len(series) != 1 || series[0].Unit != unitCount || series[0].Values[0].Value != 1 {
		t.Errorf("instance-count should be 1: %+v", series)
	}
}

func TestREST_RequestMetrics(t *testing.T) {
	// A fake request source captures the resolved request and returns one series.
	var got RequestMetricsRequest
	req := func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
		got = r
		return []MetricSeries{{
			Labels: map[string]string{"code": "200"},
			Points: []MetricPoint{{Timestamp: "2026-07-05T00:00:00Z", Value: 42}},
		}}, nil
	}
	h := metricServer(t, nil, req, sampleApp("web"), podFor("web", webInst))

	var series []renderMetricSeries
	decode(t, do(t, h, "GET",
		"/v1/metrics/http-requests?resource=web&statusCode=2xx&groupBy=status&resolutionSeconds=30", testToken, ""), &series)
	if len(series) != 1 || series[0].Unit != unitCount || series[0].Values[0].Value != 42 {
		t.Fatalf("unexpected request series: %+v", series)
	}
	// Core resolved the query and passed the filters through to the source.
	if got.Metric != MetricHTTPRequests || got.StatusCode != "2xx" || got.GroupBy != "status" {
		t.Errorf("source got wrong request: %+v", got)
	}
	if got.Resolution != 30*time.Second {
		t.Errorf("resolutionSeconds should map to 30s, got %v", got.Resolution)
	}

	// latency defaults the quantile to p95.
	decode(t, do(t, h, "GET", "/v1/metrics/http-latency?resource=web", testToken, ""), &series)
	if got.Metric != MetricHTTPLatency || got.Quantile != defaultQuantile {
		t.Errorf("latency should default quantile to p95, got %+v", got)
	}
}

func TestREST_MetricsErrors(t *testing.T) {
	h := metricServer(t, nil, nil, sampleApp("web"), podFor("web", webInst))

	if code := do(t, h, "GET", "/v1/metrics/cpu", testToken, "").Code; code != 400 {
		t.Errorf("missing resource => 400, got %d", code)
	}
	if code := do(t, h, "GET", "/v1/metrics/cpu?resource=nope", testToken, "").Code; code != 404 {
		t.Errorf("unknown app => 404, got %d", code)
	}
	// cpu with no resource-metrics source wired => 503.
	if code := do(t, h, "GET", "/v1/metrics/cpu?resource=web", testToken, "").Code; code != 503 {
		t.Errorf("no metrics source => 503, got %d", code)
	}
	// request metric with no source wired => 503.
	if code := do(t, h, "GET", "/v1/metrics/http-requests?resource=web", testToken, "").Code; code != 503 {
		t.Errorf("no request source => 503, got %d", code)
	}
	if code := do(t, h, "GET", "/v1/metrics/cpu?resource=web&startTime=bogus", testToken, "").Code; code != 400 {
		t.Errorf("bad startTime => 400, got %d", code)
	}
	if code := do(t, h, "GET", "/v1/metrics/cpu?resource=web", "", "").Code; code != 401 {
		t.Errorf("no token => 401, got %d", code)
	}
}

// --- GraphQL metrics adapter ---

// TestGraphQL_Metrics exercises Render's real dashboard GraphQL contract
// (docs/observability.md) — captured live, not the older flat-args shape:
// metrics(query: {filters, name, ...}) with values{time value}, not
// points{timestamp value}.
func TestGraphQL_Metrics(t *testing.T) {
	h := metricServer(t, staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))

	data := gql(t, h, `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU"}) { unit labels { field value } values { value } } }`)
	series := data["metrics"].([]any)
	if len(series) != 1 {
		t.Fatalf("want 1 series, got %d", len(series))
	}
	first := series[0].(map[string]any)
	if first["unit"] != unitCores {
		t.Errorf("unit should be cpu, got %v", first["unit"])
	}
	values := first["values"].([]any)
	if values[0].(map[string]any)["value"].(float64) != 0.5 {
		t.Errorf("cpu value should be 0.5, got %v", values[0])
	}

	// INSTANCES (Render's name for instance_count) over GraphQL, same dispatch.
	data = gql(t, h, `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "INSTANCES"}) { unit values { value } } }`)
	ic := data["metrics"].([]any)[0].(map[string]any)
	if ic["unit"] != unitCount {
		t.Errorf("instance_count unit should be count, got %v", ic["unit"])
	}

	// CPU_LIMIT with aggregateAllMethod:MAX collapses to one series with no
	// per-instance label — exactly the shape Render's dashboard requests for
	// the "Limit" figure.
	data = gql(t, h, `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU_LIMIT", aggregateAllMethod: "MAX"}) { unit labels { field value } values { value } } }`)
	limSeries := data["metrics"].([]any)
	if len(limSeries) != 1 {
		t.Fatalf("aggregateAllMethod:MAX should collapse to 1 series, got %d", len(limSeries))
	}
	lim := limSeries[0].(map[string]any)
	if lim["values"].([]any)[0].(map[string]any)["value"].(float64) != 1 {
		t.Errorf("cpu_limit should be 1 core, got %+v", lim)
	}
	for _, l := range lim["labels"].([]any) {
		if l.(map[string]any)["field"] == "instance" {
			t.Errorf("aggregated series should have no instance label: %+v", lim["labels"])
		}
	}

	// HTTP_LATENCY carries the requested quantile back in `parameters`.
	req := func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
		return []MetricSeries{{Points: []MetricPoint{{Timestamp: "2026-07-05T00:00:00Z", Value: 0.2}}}}, nil
	}
	h2 := metricServer(t, nil, req, sampleApp("web"))
	data = gql(t, h2, `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "HTTP_LATENCY", parameters: [{quantile: 0.9}]}) { parameters { quantile } } }`)
	latSeries := data["metrics"].([]any)[0].(map[string]any)
	params := latSeries["parameters"].([]any)
	if len(params) != 1 || params[0].(map[string]any)["quantile"].(float64) != 0.9 {
		t.Errorf("http_latency should echo the requested quantile in parameters, got %+v", params)
	}

	// A non-latency metric carries no parameters (Render only ever shows
	// quantile on latency responses).
	data = gql(t, h, `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "INSTANCES"}) { parameters { quantile } } }`)
	icParams := data["metrics"].([]any)[0].(map[string]any)["parameters"].([]any)
	if len(icParams) != 0 {
		t.Errorf("instance_count should carry no parameters, got %+v", icParams)
	}
}

func TestGraphQL_MetricsRequiresResourceFilter(t *testing.T) {
	h := metricServer(t, nil, nil, sampleApp("web"))
	body, _ := json.Marshal(map[string]string{
		"query": `{ metrics(query: {filters: [], name: "CPU"}) { unit } }`,
	})
	w := do(t, h, "POST", "/graphql", testToken, string(body))
	var out struct {
		Errors []any `json:"errors"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(out.Errors) == 0 {
		t.Error("filters without a RESOURCE entry should error, got none")
	}
}

// --- Production source parsers (no live backend) ---

func TestParsePodMetrics(t *testing.T) {
	raw := []byte(`{"items":[
	  {"metadata":{"name":"web-1"},"timestamp":"2026-07-05T00:00:00Z","containers":[
	    {"name":"app","usage":{"cpu":"500m","memory":"536870912"}}]},
	  {"metadata":{"name":"web-2"},"timestamp":"2026-07-05T00:00:00Z","containers":[
	    {"name":"app","usage":{"cpu":"250m","memory":"268435456"}},
	    {"name":"sidecar","usage":{"cpu":"250m","memory":"0"}}]}
	]}`)
	got, err := parsePodMetrics(raw)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 pods, got %d", len(got))
	}
	// sorted by pod name; container usage summed.
	if got[0].Pod != webInst || got[0].CPUCores != 0.5 || got[0].MemoryBytes != 536870912 {
		t.Errorf("web-1 wrong: %+v", got[0])
	}
	if got[1].CPUCores != 0.5 { // 250m + 250m across two containers
		t.Errorf("web-2 cpu should sum containers to 0.5, got %v", got[1].CPUCores)
	}
}

func TestPromQueryFor(t *testing.T) {
	// http_requests with a status filter and group-by.
	q := promQueryFor(RequestMetricsRequest{
		App: "web", Metric: MetricHTTPRequests, Resolution: 60 * time.Second,
		StatusCode: "5xx", GroupBy: "status",
	})
	want := `sum(rate(traefik_service_requests_total{service=~".*web.*",code=~"5.."}[60s])) by (code)`
	if q != want {
		t.Errorf("http_requests query:\n got %q\nwant %q", q, want)
	}

	// latency wraps histogram_quantile over the bucket metric, grouped by le.
	lat := promQueryFor(RequestMetricsRequest{App: "web", Metric: MetricHTTPLatency, Resolution: 60 * time.Second, Quantile: 0.9})
	wantLat := `histogram_quantile(0.9, sum(rate(traefik_service_request_duration_seconds_bucket{service=~".*web.*"}[60s])) by (le))`
	if lat != wantLat {
		t.Errorf("latency query:\n got %q\nwant %q", lat, wantLat)
	}

	// bandwidth uses the responses-bytes counter.
	bw := promQueryFor(RequestMetricsRequest{App: "web", Metric: MetricBandwidth, Resolution: 60 * time.Second})
	wantBw := `sum(rate(traefik_service_responses_bytes_total{service=~".*web.*"}[60s]))`
	if bw != wantBw {
		t.Errorf("bandwidth query:\n got %q\nwant %q", bw, wantBw)
	}
}

func TestParsePromMatrix(t *testing.T) {
	pr := promRangeResponse{Status: "success"}
	pr.Data.ResultType = "matrix"
	pr.Data.Result = []struct {
		Metric map[string]string `json:"metric"`
		Values [][]any           `json:"values"`
	}{
		{
			Metric: map[string]string{"code": "200"},
			Values: [][]any{
				{float64(1_000_000), "1.5"},
				{float64(1_000_060), "NaN"}, // empty bucket -> dropped
				{float64(1_000_120), "2.5"},
			},
		},
	}
	got, err := parsePromMatrix(pr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(got) != 1 || got[0].Labels["code"] != "200" {
		t.Fatalf("unexpected series: %+v", got)
	}
	if len(got[0].Points) != 2 { // NaN dropped
		t.Fatalf("want 2 points (NaN dropped), got %d: %+v", len(got[0].Points), got[0].Points)
	}
	if got[0].Points[0].Value != 1.5 || got[0].Points[1].Value != 2.5 {
		t.Errorf("point values wrong: %+v", got[0].Points)
	}
}

// TestNewPrometheusRequestSource_RoundTrip exercises the whole request path (URL
// build + HTTP + matrix parse) against an httptest Prometheus stand-in.
func TestNewPrometheusRequestSource_RoundTrip(t *testing.T) {
	var gotQuery string
	ts := newPromStub(t, &gotQuery)
	defer ts.Close()

	src := NewPrometheusRequestSource(ts.URL, ts.Client())
	series, err := src(context.Background(), RequestMetricsRequest{
		App: "web", Metric: MetricHTTPRequests,
		Start: time.Unix(1_000_000, 0), End: time.Unix(1_000_120, 0), Resolution: 60 * time.Second,
	})
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if gotQuery == "" {
		t.Error("prometheus stub saw no query param")
	}
	if len(series) != 1 || len(series[0].Points) != 1 || series[0].Points[0].Value != 7 {
		t.Fatalf("unexpected series from stub: %+v", series)
	}
}

// newPromStub is an httptest Prometheus that records the `query` param and
// returns a one-series matrix (a single sample of value 7).
func newPromStub(t *testing.T, gotQuery *string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		*gotQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[
		  {"metric":{"code":"200"},"values":[[1000000,"7"]]}]}}`))
	}))
}

func TestParsePromScalarSum(t *testing.T) {
	pr := promInstantResponse{Status: "success"}
	pr.Data.Result = []struct {
		Value []any `json:"value"`
	}{
		{Value: []any{float64(1_000_000), "12.5"}},
		{Value: []any{float64(1_000_000), "7.5"}},
	}
	got, err := parsePromScalarSum(pr)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != 20 {
		t.Errorf("want sum 20, got %v", got)
	}

	if _, err := parsePromScalarSum(promInstantResponse{Status: "error"}); err == nil {
		t.Error("want error on non-success status")
	}
}

// TestNewMonthToDateBandwidthSource_RoundTrip exercises the whole instant-query
// path (URL build + HTTP + scalar-sum parse) against an httptest stand-in.
func TestNewMonthToDateBandwidthSource_RoundTrip(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[
		  {"metric":{},"value":[1000000,"104857600"]}]}}`))
	}))
	defer ts.Close()

	src := NewMonthToDateBandwidthSource(ts.URL, ts.Client())
	bytesTotal, err := src(context.Background(), "default", "web", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if !strings.Contains(gotQuery, "increase(traefik_service_responses_bytes_total") || !strings.Contains(gotQuery, `service=~".*web.*"`) {
		t.Errorf("unexpected query sent to prometheus: %q", gotQuery)
	}
	if bytesTotal != 104857600 {
		t.Errorf("want 104857600 bytes, got %v", bytesTotal)
	}
}

func TestNewMonthToDateBandwidthSource_FutureSinceIsZero(t *testing.T) {
	src := NewMonthToDateBandwidthSource("http://unused.invalid", http.DefaultClient)
	bytesTotal, err := src(context.Background(), "default", "web", time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if bytesTotal != 0 {
		t.Errorf("want 0 for a since in the future, got %v", bytesTotal)
	}
}

// --- Ranged resource metrics (cAdvisor via Prometheus) ---

// staticRangeSource fakes a ResourceMetricsRangeSource: it captures the request
// Core resolved (when got is non-nil) and returns canned stepped series.
func staticRangeSource(got *ResourceMetricsRangeRequest, series []MetricSeries) ResourceMetricsRangeSource {
	return func(_ context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		if got != nil {
			*got = req
		}
		return series, nil
	}
}

// twoPoints is a stepped two-sample series for one instance — what a ranged
// source returns and a snapshot source never can.
func twoPoints(instance string, v1, v2 float64) MetricSeries {
	return MetricSeries{
		Labels: map[string]string{"resource": "web", "instance": instance},
		Points: []MetricPoint{
			{Timestamp: "2026-07-05T00:00:00Z", Value: v1},
			{Timestamp: "2026-07-05T00:01:00Z", Value: v2},
		},
	}
}

// TestCore_RangedResourceMetricsPreferred: with a ranged source wired, cpu/memory
// come from it — stepped multi-point series, range/step resolved by Core — and
// the metrics-server snapshot source is not consulted.
func TestCore_RangedResourceMetricsPreferred(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podWithLimits(webInst),
	).Build()
	var got ResourceMetricsRangeRequest
	core := &Core{
		Client: cl, Namespace: "default",
		Now:                  func() time.Time { return time.Unix(1_000_000, 0).UTC() },
		ResourceMetricsRange: staticRangeSource(&got, []MetricSeries{twoPoints(webInst, 0.5, 0.25)}),
		ResourceMetrics: func(_ context.Context, _, _ string) ([]PodResourceUsage, error) {
			t.Error("snapshot source consulted although a ranged source is wired")
			return nil, nil
		},
	}

	start, end := time.Unix(990_000, 0).UTC(), time.Unix(1_000_000, 0).UTC()
	series, err := core.Metrics(context.Background(), MetricQuery{
		App: "web", Metric: MetricCPU, Start: start, End: end, Resolution: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("cpu: %v", err)
	}
	if len(series) != 1 || len(series[0].Points) != 2 {
		t.Fatalf("want 1 stepped series with 2 points, got %+v", series)
	}
	if series[0].Unit != unitCores || series[0].Labels["instance"] != webInst {
		t.Errorf("unit/labels wrong: %+v", series[0])
	}
	if got.Metric != MetricCPU || !got.Start.Equal(start) || !got.End.Equal(end) || got.Resolution != 30*time.Second {
		t.Errorf("range request not propagated: %+v", got)
	}
	if got.Namespace != "default" || got.App != "web" {
		t.Errorf("namespace/app not propagated: %+v", got)
	}

	// Zero range still resolves defaults (end=now, start=end-1h) before the source.
	if _, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricMemory}); err != nil {
		t.Fatalf("memory: %v", err)
	}
	if got.End.IsZero() || !got.Start.Equal(got.End.Add(-defaultMetricSpan)) {
		t.Errorf("defaults not resolved for the source: %+v", got)
	}
}

// TestCore_RangedResourceMetricsPercentage: percentage mode divides every point
// by the instance's current pod limit and omits instances with no matching pod
// (e.g. one that no longer exists) — same omit-don't-fake rule as the snapshot.
func TestCore_RangedResourceMetricsPercentage(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podWithLimits(webInst), // 1 core / 1Gi limits
	).Build()
	core := &Core{
		Client: cl, Namespace: "default",
		Now: func() time.Time { return time.Unix(1_000_000, 0).UTC() },
		ResourceMetricsRange: staticRangeSource(nil, []MetricSeries{
			twoPoints(webInst, 0.5, 0.25),
			twoPoints("web-gone", 0.5, 0.5), // no current pod => no limit => omitted
		}),
	}

	series, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Percentage: true})
	if err != nil {
		t.Fatalf("cpu%%: %v", err)
	}
	if len(series) != 1 {
		t.Fatalf("instance with no limit should be omitted, got %+v", series)
	}
	if series[0].Unit != unitPercentage {
		t.Errorf("unit should be percentage, got %q", series[0].Unit)
	}
	if series[0].Points[0].Value != 50 || series[0].Points[1].Value != 25 {
		t.Errorf("every point should be divided by the 1-core limit: %+v", series[0].Points)
	}
}

// TestCore_RangedInstanceCount: with a ranged source wired, instance_count is a
// stepped count-over-time series from it, not a single pod-count point.
func TestCore_RangedInstanceCount(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podFor("web", webInst),
	).Build()
	var got ResourceMetricsRangeRequest
	core := &Core{
		Client: cl, Namespace: "default",
		Now: func() time.Time { return time.Unix(1_000_000, 0).UTC() },
		ResourceMetricsRange: staticRangeSource(&got, []MetricSeries{{
			Labels: map[string]string{"resource": "web"},
			Points: []MetricPoint{
				{Timestamp: "2026-07-05T00:00:00Z", Value: 1},
				{Timestamp: "2026-07-05T00:01:00Z", Value: 2},
			},
		}}),
	}

	series, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricInstanceCount})
	if err != nil {
		t.Fatalf("instance_count: %v", err)
	}
	if got.Metric != MetricInstanceCount {
		t.Errorf("source should be asked for instance_count, got %q", got.Metric)
	}
	if len(series) != 1 || series[0].Unit != unitCount || len(series[0].Points) != 2 {
		t.Fatalf("want one stepped count series, got %+v", series)
	}
}

// TestCore_RangedSourceErrorSurfaces: Prometheus wired but failing at query time
// surfaces the error — no silent fallback to the snapshot source.
func TestCore_RangedSourceErrorSurfaces(t *testing.T) {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(
		sampleApp("web"), podWithLimits(webInst),
	).Build()
	core := &Core{
		Client: cl, Namespace: "default",
		ResourceMetricsRange: func(_ context.Context, _ ResourceMetricsRangeRequest) ([]MetricSeries, error) {
			return nil, context.DeadlineExceeded
		},
		ResourceMetrics: staticResourceMetrics(map[string]PodResourceUsage{webInst: {CPUCores: 0.5}}),
	}

	if _, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU}); err != context.DeadlineExceeded {
		t.Errorf("ranged-source error should surface, got %v", err)
	}
	if _, err := core.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricInstanceCount}); err != context.DeadlineExceeded {
		t.Errorf("instance_count ranged-source error should surface, got %v", err)
	}
}

func TestPromResourceQueryFor(t *testing.T) {
	req := ResourceMetricsRangeRequest{Namespace: "default", App: "web", Resolution: 60 * time.Second}
	matchers := `namespace="default",pod=~"web-[a-z0-9]+-[a-z0-9]{5}",container!=""`

	req.Metric = MetricCPU
	if got, want := promResourceQueryFor(req),
		`sum by (pod) (rate(container_cpu_usage_seconds_total{`+matchers+`}[60s]))`; got != want {
		t.Errorf("cpu query:\n got %q\nwant %q", got, want)
	}
	req.Metric = MetricMemory
	if got, want := promResourceQueryFor(req),
		`sum by (pod) (container_memory_working_set_bytes{`+matchers+`})`; got != want {
		t.Errorf("memory query:\n got %q\nwant %q", got, want)
	}
	req.Metric = MetricInstanceCount
	if got, want := promResourceQueryFor(req),
		`count(sum by (pod) (container_memory_working_set_bytes{`+matchers+`}))`; got != want {
		t.Errorf("instance_count query:\n got %q\nwant %q", got, want)
	}
}

// TestNewPrometheusResourceSource_RoundTrip exercises the whole resource-history
// path (URL build + HTTP + matrix parse + label rewrite) against an httptest
// Prometheus stand-in.
func TestNewPrometheusResourceSource_RoundTrip(t *testing.T) {
	var gotValues url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotValues = r.URL.Query()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[
		  {"metric":{"pod":"web-abc123-x2f4z"},"values":[[1000000,"0.5"],[1000060,"0.25"]]}]}}`))
	}))
	defer ts.Close()

	src := NewPrometheusResourceSource(ts.URL, ts.Client())
	series, err := src(context.Background(), ResourceMetricsRangeRequest{
		Namespace: "default", App: "web", Metric: MetricCPU,
		Start: time.Unix(1_000_000, 0), End: time.Unix(1_003_600, 0), Resolution: 60 * time.Second,
	})
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if gotValues.Get("start") != "1000000" || gotValues.Get("end") != "1003600" || gotValues.Get("step") != "60" {
		t.Errorf("range params not propagated: %v", gotValues)
	}
	if !strings.Contains(gotValues.Get("query"), "container_cpu_usage_seconds_total") {
		t.Errorf("unexpected query: %q", gotValues.Get("query"))
	}
	if len(series) != 1 || len(series[0].Points) != 2 {
		t.Fatalf("want one two-point series, got %+v", series)
	}
	// Prometheus's pod label is rewritten into Core's instance/resource vocabulary.
	if series[0].Labels["instance"] != "web-abc123-x2f4z" || series[0].Labels["resource"] != "web" {
		t.Errorf("labels not rewritten: %+v", series[0].Labels)
	}
	if _, still := series[0].Labels["pod"]; still {
		t.Errorf("raw pod label should not leak out: %+v", series[0].Labels)
	}
}

func TestNewPrometheusResourceSource_ErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer ts.Close()

	src := NewPrometheusResourceSource(ts.URL, ts.Client())
	if _, err := src(context.Background(), ResourceMetricsRangeRequest{Namespace: "default", App: "web", Metric: MetricCPU}); err == nil {
		t.Error("non-200 from prometheus should error")
	}

	// Unreachable endpoint errors too (no swallow-into-empty).
	down := NewPrometheusResourceSource("http://127.0.0.1:1", http.DefaultClient)
	if _, err := down(context.Background(), ResourceMetricsRangeRequest{Namespace: "default", App: "web", Metric: MetricMemory}); err == nil {
		t.Error("unreachable prometheus should error")
	}
}

// TestNewPrometheusFilterValuesSource_RoundTrip exercises the label-values path
// against an httptest stand-in.
func TestNewPrometheusFilterValuesSource_RoundTrip(t *testing.T) {
	var gotPath, gotMatch string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotMatch = r.URL.Query().Get("match[]")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","data":["200","404","500"]}`))
	}))
	defer ts.Close()

	src := NewPrometheusFilterValuesSource(ts.URL, ts.Client())
	values, err := src(context.Background(), "default", "web", "code")
	if err != nil {
		t.Fatalf("source: %v", err)
	}
	if gotPath != "/api/v1/label/code/values" {
		t.Errorf("unexpected path: %q", gotPath)
	}
	if !strings.Contains(gotMatch, `traefik_service_requests_total{service=~".*web.*"}`) {
		t.Errorf("unexpected match[] selector: %q", gotMatch)
	}
	if len(values) != 3 || values[0] != "200" {
		t.Errorf("unexpected values: %+v", values)
	}
}
