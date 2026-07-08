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
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

const webInst = "web-1"

func testScheme() *runtime.Scheme {
	scheme := runtime.NewScheme()
	_ = clientgoscheme.AddToScheme(scheme)
	_ = appv1alpha1.AddToScheme(scheme)
	return scheme
}

func fixedClock() time.Time { return time.Unix(1_000_000, 0).UTC() }

func sampleApp(name string) *appv1alpha1.App {
	return &appv1alpha1.App{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
		Status:     appv1alpha1.AppStatus{Phase: appv1alpha1.PhaseRunning},
	}
}

func podFor(app, name string) *corev1.Pod {
	return &corev1.Pod{ObjectMeta: metav1.ObjectMeta{
		Name: name, Namespace: "default", Labels: map[string]string{core.PodLabelApp: app},
	}}
}

// podWithLimits is podFor("web", name) plus a 1-core / 1Gi limit, so
// percentage-mode metrics have a denominator.
func podWithLimits(name string) *corev1.Pod {
	p := podFor("web", name)
	p.Spec.Containers = []corev1.Container{{
		Name: core.AppContainer,
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		}},
	}}
	return p
}

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

func newService(rm ResourceMetricsSource, req RequestMetricsSource, objs ...client.Object) *Service {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).Build()
	return &Service{
		Base:            &core.Base{Client: cl, Namespace: "default", Clock: fixedClock},
		ResourceMetrics: rm,
		RequestMetrics:  req,
	}
}

// --- Verb logic ---

func TestResourceMetricsAbsoluteAndPercentage(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
		"web-2": {CPUCores: 0.25, MemoryBytes: 256 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst), podWithLimits("web-2"))

	abs, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU})
	if err != nil || len(abs) != 2 || abs[0].Labels["instance"] != webInst || abs[0].Unit != unitCores {
		t.Fatalf("cpu: %v %+v", err, abs)
	}
	if abs[0].Points[0].Value != 0.5 {
		t.Errorf("web-1 cpu should be 0.5, got %v", abs[0].Points[0].Value)
	}
	pct, _ := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Percentage: true})
	if pct[0].Unit != unitPercentage || pct[0].Points[0].Value != 50 {
		t.Errorf("cpu%% should be 50, got %+v", pct[0])
	}
	mem, _ := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricMemory, Percentage: true})
	if mem[0].Points[0].Value != 50 {
		t.Errorf("mem%% should be 50, got %v", mem[0].Points[0].Value)
	}
}

func TestLimitAndInstanceCountNeedNoResourceSource(t *testing.T) {
	svc := newService(nil, nil, sampleApp("web"), podWithLimits(webInst), podWithLimits("web-2"))

	cpuLim, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPULimit})
	if err != nil || len(cpuLim) != 2 || cpuLim[0].Points[0].Value != 1 {
		t.Fatalf("cpu_limit: %v %+v", err, cpuLim)
	}
	memLim, _ := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricMemoryLimit})
	if memLim[0].Unit != unitBytes || memLim[0].Points[0].Value != float64(1<<30) {
		t.Errorf("memory_limit should be 1Gi bytes, got %+v", memLim[0])
	}
	// AggregateMax collapses to one label-less series.
	agg, _ := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPULimit, AggregateMax: true})
	if len(agg) != 1 || agg[0].Points[0].Value != 1 {
		t.Fatalf("aggregateMax => one series value 1, got %+v", agg)
	}
	if _, has := agg[0].Labels["instance"]; has {
		t.Errorf("aggregated series should drop instance label: %+v", agg[0].Labels)
	}
	ic, _ := newService(nil, nil, sampleApp("web"), podFor("web", webInst), podFor("web", "web-2")).
		Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricInstanceCount})
	if len(ic) != 1 || ic[0].Unit != unitCount || ic[0].Points[0].Value != 2 {
		t.Fatalf("instance_count should be 2: %+v", ic)
	}
}

func TestResourceLimitOmitsPodsWithNoLimit(t *testing.T) {
	svc := newService(nil, nil, sampleApp("web"), podFor("web", webInst))
	series, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPULimit})
	if err != nil || len(series) != 0 {
		t.Errorf("pod with no limit should be omitted: %v %+v", err, series)
	}
}

func TestMetricsErrors(t *testing.T) {
	svc := newService(nil, nil, sampleApp("web"))
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU}); err != core.ErrMetricsUnavailable {
		t.Errorf("cpu without source => ErrMetricsUnavailable, got %v", err)
	}
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricHTTPRequests}); err != core.ErrMetricsUnavailable {
		t.Errorf("http_requests without source => ErrMetricsUnavailable, got %v", err)
	}
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "nope", Metric: MetricInstanceCount}); err != core.ErrNotFound {
		t.Errorf("unknown app => ErrNotFound, got %v", err)
	}
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: "bogus"}); err == nil {
		t.Error("unknown metric should error")
	}
}

func TestRequestMetricResolvesQueryAndUnits(t *testing.T) {
	var got RequestMetricsRequest
	req := func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
		got = r
		return []MetricSeries{{Points: []MetricPoint{{Value: 42}}}}, nil
	}
	svc := newService(nil, req, sampleApp("web"))
	series, err := svc.Metrics(context.Background(), MetricQuery{
		App: "web", Metric: MetricHTTPRequests, StatusCode: "2xx", GroupBy: "status", Resolution: 30 * time.Second,
	})
	if err != nil || series[0].Unit != unitCount || series[0].Points[0].Value != 42 {
		t.Fatalf("request metric: %v %+v", err, series)
	}
	if got.StatusCode != "2xx" || got.GroupBy != "status" || got.Resolution != 30*time.Second {
		t.Errorf("source got wrong request: %+v", got)
	}
	// latency defaults the quantile to p95.
	_, _ = svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricHTTPLatency})
	if got.Quantile != defaultQuantile {
		t.Errorf("latency should default quantile to p95, got %v", got.Quantile)
	}
}

// --- REST fragment ---

func serveREST(svc *Service, path string) *httptest.ResponseRecorder {
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("GET", path, nil))
	return rec
}

func TestRESTMetricsShapeAndErrors(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))

	var series []renderMetricSeries
	_ = json.Unmarshal(serveREST(svc, "/v1/metrics/cpu?resource=web").Body.Bytes(), &series)
	if len(series) != 1 || series[0].Unit != unitCores || series[0].Values[0].Value != 0.5 {
		t.Fatalf("cpu series: %+v", series)
	}
	// labels sorted {field,value}: instance before resource.
	if len(series[0].Labels) != 2 || series[0].Labels[0].Field != "instance" || series[0].Labels[1].Field != "resource" {
		t.Errorf("labels should be sorted: %+v", series[0].Labels)
	}
	_ = json.Unmarshal(serveREST(svc, "/v1/metrics/instance-count?resource=web").Body.Bytes(), &series)
	if series[0].Unit != unitCount || series[0].Values[0].Value != 1 {
		t.Errorf("instance-count should be 1: %+v", series)
	}

	// errors: missing resource => 400, unknown app => 404, no source => 503.
	if serveREST(svc, "/v1/metrics/cpu").Code != 400 {
		t.Error("missing resource => 400")
	}
	if serveREST(svc, "/v1/metrics/cpu?resource=nope").Code != 404 {
		t.Error("unknown app => 404")
	}
	if serveREST(newService(nil, nil, sampleApp("web"), podFor("web", webInst)), "/v1/metrics/http-requests?resource=web").Code != 503 {
		t.Error("no request source => 503")
	}
}

// --- GraphQL fragment (Render dashboard shape) ---

func gqlSchema(svc *Service) (graphql.Schema, error) {
	return graphql.NewSchema(graphql.SchemaConfig{
		Query: graphql.NewObject(graphql.ObjectConfig{Name: "Query", Fields: svc.GraphQLQuery()}),
	})
}

func TestGraphQLMetrics(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU"}) { unit values { value } } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	first := res.Data.(map[string]any)["metrics"].([]any)[0].(map[string]any)
	if first["unit"] != unitCores || first["values"].([]any)[0].(map[string]any)["value"].(float64) != 0.5 {
		t.Errorf("cpu value should be 0.5: %+v", first)
	}

	// aggregateAllMethod:MAX collapses CPU_LIMIT to one label-less series.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU_LIMIT", aggregateAllMethod: "MAX"}) { values { value } } }`})
	if len(res.Errors) > 0 || len(res.Data.(map[string]any)["metrics"].([]any)) != 1 {
		t.Errorf("cpu_limit MAX should collapse to 1 series: %v %+v", res.Errors, res.Data)
	}

	// filters without a RESOURCE entry errors.
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [], name: "CPU"}) { unit } }`})
	if len(res.Errors) == 0 {
		t.Error("filters without RESOURCE should error")
	}
}

// TestGraphQLAggregateByMapsToGroupBy: aggregateBy carries Render's per-chart
// "Group by" (STATUS_CODE/METHOD) onto Core's GroupBy — the same knob REST's
// `groupBy` param sets, keeping the two surfaces parity-equal. Instance-
// flavored values stay ignored (bex always sums across instances).
func TestGraphQLAggregateByMapsToGroupBy(t *testing.T) {
	var got RequestMetricsRequest
	req := func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
		got = r
		return []MetricSeries{{Points: []MetricPoint{{Timestamp: "2026-07-05T00:00:00Z", Value: 1}}}}, nil
	}
	svc := newService(nil, req, sampleApp("web"))
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}

	ask := func(aggregateBy string) {
		t.Helper()
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
			RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}, {field: "STATUS_CODE", values: ["5xx"]}], name: "HTTP_REQUESTS", aggregateBy: [` + aggregateBy + `]}) { unit } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("gql: %v", res.Errors)
		}
	}

	ask(`"STATUS_CODE"`)
	if got.GroupBy != "status" || got.StatusCode != "5xx" {
		t.Errorf(`aggregateBy STATUS_CODE => groupBy "status" (+ filter passthrough), got %+v`, got)
	}
	ask(`"METHOD"`)
	if got.GroupBy != "method" {
		t.Errorf(`aggregateBy METHOD => groupBy "method", got %q`, got.GroupBy)
	}
	ask(`"instance"`)
	if got.GroupBy != "" {
		t.Errorf("instance-flavored aggregateBy should stay ignored, got %q", got.GroupBy)
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
	if err != nil || len(got) != 2 {
		t.Fatalf("parse: %v len=%d", err, len(got))
	}
	if got[0].Pod != webInst || got[0].CPUCores != 0.5 || got[0].MemoryBytes != 536870912 {
		t.Errorf("web-1 wrong: %+v", got[0])
	}
	if got[1].CPUCores != 0.5 { // 250m + 250m summed across containers
		t.Errorf("web-2 cpu should sum to 0.5, got %v", got[1].CPUCores)
	}
}

func TestPromQueryFor(t *testing.T) {
	q := promQueryFor(RequestMetricsRequest{App: "web", Metric: MetricHTTPRequests, Resolution: 60 * time.Second, StatusCode: "5xx", GroupBy: "status"})
	if want := `sum(rate(traefik_service_requests_total{service=~".*web.*",code=~"5.."}[60s])) by (code)`; q != want {
		t.Errorf("http_requests:\n got %q\nwant %q", q, want)
	}
	lat := promQueryFor(RequestMetricsRequest{App: "web", Metric: MetricHTTPLatency, Resolution: 60 * time.Second, Quantile: 0.9})
	if want := `histogram_quantile(0.9, sum(rate(traefik_service_request_duration_seconds_bucket{service=~".*web.*"}[60s])) by (le))`; lat != want {
		t.Errorf("latency:\n got %q\nwant %q", lat, want)
	}
}

func TestParsePromMatrixDropsNaN(t *testing.T) {
	pr := promRangeResponse{Status: "success"}
	pr.Data.Result = []struct {
		Metric map[string]string `json:"metric"`
		Values [][]any           `json:"values"`
	}{{
		Metric: map[string]string{"code": "200"},
		Values: [][]any{{float64(1_000_000), "1.5"}, {float64(1_000_060), "NaN"}, {float64(1_000_120), "2.5"}},
	}}
	got, err := parsePromMatrix(pr)
	if err != nil || len(got) != 1 || len(got[0].Points) != 2 {
		t.Fatalf("want 1 series/2 points (NaN dropped): %v %+v", err, got)
	}
	if got[0].Points[0].Value != 1.5 || got[0].Points[1].Value != 2.5 {
		t.Errorf("point values wrong: %+v", got[0].Points)
	}
}

func TestPrometheusRequestSourceRoundTrip(t *testing.T) {
	var gotQuery string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query().Get("query")
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{"code":"200"},"values":[[1000000,"7"]]}]}}`))
	}))
	defer ts.Close()
	series, err := NewPrometheusRequestSource(ts.URL, ts.Client())(context.Background(), RequestMetricsRequest{
		App: "web", Metric: MetricHTTPRequests, Start: time.Unix(1_000_000, 0), End: time.Unix(1_000_120, 0), Resolution: 60 * time.Second,
	})
	if err != nil || gotQuery == "" || len(series) != 1 || series[0].Points[0].Value != 7 {
		t.Fatalf("roundtrip: %v q=%q %+v", err, gotQuery, series)
	}
}

func TestPrometheusFilterValuesRoundTrip(t *testing.T) {
	var gotMatch string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotMatch = r.URL.Query().Get("match[]")
		_, _ = w.Write([]byte(`{"status":"success","data":["200","404","500"]}`))
	}))
	defer ts.Close()
	values, err := NewPrometheusFilterValuesSource(ts.URL, ts.Client())(context.Background(), "default", "web", "code")
	if err != nil || len(values) != 3 || values[0] != "200" {
		t.Fatalf("filter values: %v %+v", err, values)
	}
	if !strings.Contains(gotMatch, `traefik_service_requests_total{service=~".*web.*"}`) {
		t.Errorf("unexpected match[]: %q", gotMatch)
	}
}
