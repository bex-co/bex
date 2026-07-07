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
	"net/http"
	"net/http/httptest"
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

func TestGraphQL_Metrics(t *testing.T) {
	h := metricServer(t, staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))

	data := gql(t, h, `{ metrics(resource:"web", metric:"cpu") { unit labels { field value } points { value } } }`)
	series := data["metrics"].([]any)
	if len(series) != 1 {
		t.Fatalf("want 1 series, got %d", len(series))
	}
	first := series[0].(map[string]any)
	if first["unit"] != unitCores {
		t.Errorf("unit should be cpu, got %v", first["unit"])
	}
	points := first["points"].([]any)
	if points[0].(map[string]any)["value"].(float64) != 0.5 {
		t.Errorf("cpu point should be 0.5, got %v", points[0])
	}

	// instance_count over GraphQL (same Core dispatch).
	data = gql(t, h, `{ metrics(resource:"web", metric:"instance_count") { unit points { value } } }`)
	ic := data["metrics"].([]any)[0].(map[string]any)
	if ic["unit"] != unitCount {
		t.Errorf("instance_count unit should be count, got %v", ic["unit"])
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
