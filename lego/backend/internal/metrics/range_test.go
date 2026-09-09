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
	"net/http"
	"net/http/httptest"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

// rangeService builds a metrics Service with a ranged source (and optional
// snapshot source) over a fake client seeded with objs.
func rangeService(rng ResourceMetricsRangeSource, snap ResourceMetricsSource, objs ...client.Object) *Service {
	cl := fake.NewClientBuilder().WithScheme(testScheme()).WithObjects(objs...).Build()
	return &Service{
		Base:                 &core.Base{Client: cl, Namespace: "default", Clock: fixedClock},
		ResourceMetricsRange: rng,
		ResourceMetrics:      snap,
	}
}

// staticRangeSource fakes a ResourceMetricsRangeSource: it captures the request
// the service resolved (when got is non-nil) and returns canned stepped series.
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

// With a ranged source wired, cpu/memory come from it — stepped multi-point
// series, range/step resolved by the service — and the snapshot source is not
// consulted.
func TestRangedResourceMetricsPreferred(t *testing.T) {
	var got ResourceMetricsRangeRequest
	svc := rangeService(
		staticRangeSource(&got, []MetricSeries{twoPoints(webInst, 0.5, 0.25)}),
		func(_ context.Context, _, _ string) ([]PodResourceUsage, error) {
			t.Error("snapshot source consulted although a ranged source is wired")
			return nil, nil
		},
		sampleApp("web"), podWithLimits(webInst))

	start, end := time.Unix(990_000, 0).UTC(), time.Unix(1_000_000, 0).UTC()
	series, err := svc.Metrics(context.Background(), MetricQuery{
		App: "web", Metric: MetricCPU, Start: start, End: end, Resolution: 30 * time.Second,
	})
	if err != nil {
		t.Fatalf("cpu: %v", err)
	}
	if len(series) != 1 || len(series[0].Points) != 2 || series[0].Unit != unitCores || series[0].Labels["instance"] != ids.ServiceInstanceID("web", webInst) {
		t.Fatalf("want 1 stepped 2-point series: %+v", series)
	}
	if got.Metric != MetricCPU || !got.Start.Equal(start) || !got.End.Equal(end) || got.Resolution != 30*time.Second {
		t.Errorf("range request not propagated: %+v", got)
	}
	if got.Namespace != "default" || got.App != "web" {
		t.Errorf("namespace/app not propagated: %+v", got)
	}
	// Zero range still resolves defaults (end=now, start=end-1h) before the source.
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricMemory}); err != nil {
		t.Fatalf("memory: %v", err)
	}
	if got.End.IsZero() || !got.Start.Equal(got.End.Add(-defaultMetricSpan)) {
		t.Errorf("defaults not resolved for the source: %+v", got)
	}
}

// Percentage mode divides every point by the instance's current pod limit and
// omits instances with no matching pod.
func TestRangedResourceMetricsPercentage(t *testing.T) {
	svc := rangeService(staticRangeSource(nil, []MetricSeries{
		twoPoints(webInst, 0.5, 0.25),
		twoPoints("web-gone", 0.5, 0.5), // no current pod => no limit => omitted
	}), nil, sampleApp("web"), podWithLimits(webInst))

	series, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Percentage: true})
	if err != nil {
		t.Fatalf("cpu%%: %v", err)
	}
	if len(series) != 1 || series[0].Unit != unitPercentage {
		t.Fatalf("instance with no limit should be omitted, unit percentage: %+v", series)
	}
	if series[0].Points[0].Value != 50 || series[0].Points[1].Value != 25 {
		t.Errorf("every point should be divided by the 1-core limit: %+v", series[0].Points)
	}
}

// With a ranged source wired, instance_count is a stepped count-over-time series
// from it, not a single pod-count point.
func TestRangedInstanceCount(t *testing.T) {
	var got ResourceMetricsRangeRequest
	svc := rangeService(staticRangeSource(&got, []MetricSeries{{
		Labels: map[string]string{"resource": "web"},
		Points: []MetricPoint{{Timestamp: "2026-07-05T00:00:00Z", Value: 1}, {Timestamp: "2026-07-05T00:01:00Z", Value: 2}},
	}}), nil, sampleApp("web"), podFor("web", webInst))

	series, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricInstanceCount})
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

// Prometheus wired but failing at query time surfaces the error — no silent
// fallback to the snapshot source.
func TestRangedSourceErrorSurfaces(t *testing.T) {
	svc := rangeService(
		func(_ context.Context, _ ResourceMetricsRangeRequest) ([]MetricSeries, error) {
			return nil, context.DeadlineExceeded
		},
		staticResourceMetrics(map[string]PodResourceUsage{webInst: {CPUCores: 0.5}}),
		sampleApp("web"), podWithLimits(webInst))

	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU}); err != context.DeadlineExceeded {
		t.Errorf("ranged-source error should surface, got %v", err)
	}
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricInstanceCount}); err != context.DeadlineExceeded {
		t.Errorf("instance_count ranged-source error should surface, got %v", err)
	}
}

func TestPromResourceQueryFor(t *testing.T) {
	req := ResourceMetricsRangeRequest{Namespace: "default", App: "web", Resolution: 60 * time.Second}
	// Two alternatives: the untruncated two-segment pod name, and the
	// single-segment name Kubernetes leaves when generateName truncates
	// (w6/m110). 59 = 63 - len("web") - 1, the exact length a truncated pod
	// name always has.
	matchers := `namespace="default",pod=~"web-[a-z0-9]+-[a-z0-9]{5}|web-[a-z0-9]{59}",container!=""`

	// Memory and CPU dedupe cAdvisor's duplicate per-container series across a
	// restart (`max by (pod, container)` before the sum) so an OOM-restarting
	// container never double-counts above the pod's real usage (w4/050).
	req.Metric = MetricCPU
	if got, want := promResourceQueryFor(req), `sum by (pod) (max by (pod, container) (rate(container_cpu_usage_seconds_total{`+matchers+`}[60s])))`; got != want {
		t.Errorf("cpu query:\n got %q\nwant %q", got, want)
	}
	req.Metric = MetricMemory
	if got, want := promResourceQueryFor(req), `sum by (pod) (max by (pod, container) (container_memory_working_set_bytes{`+matchers+`}))`; got != want {
		t.Errorf("memory query:\n got %q\nwant %q", got, want)
	}
	// Instance count is untouched: the inner `sum by (pod)` already collapses a
	// pod's duplicated container series to one, so counting pods is correct.
	req.Metric = MetricInstanceCount
	if got, want := promResourceQueryFor(req), `count(sum by (pod) (container_memory_working_set_bytes{`+matchers+`}))`; got != want {
		t.Errorf("instance_count query:\n got %q\nwant %q", got, want)
	}
}

// TestPromResourceQuerySelectsTruncatedPodNames is w6/m110 Defect B at the
// metrics surface: the Metrics page's Memory/CPU/Total Instances cards read
// "No data in range" for every service whose Kubernetes object name pushed the
// generated pod name past the 58-char truncation point, while the Network cards
// beside them — which select by the Traefik service label, not the pod name —
// had data over the same window for the same service. The pod names below are
// the live ones for qa-20260826-webhook-svc (23 characters).
func TestPromResourceQuerySelectsTruncatedPodNames(t *testing.T) {
	req := ResourceMetricsRangeRequest{
		Namespace:  "tea-d98210cbbpdc73dcrkvg",
		App:        "tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc",
		Metric:     MetricMemory,
		Resolution: 60 * time.Second,
	}
	matcher := regexp.MustCompile(`pod=~"([^"]+)"`).FindStringSubmatch(promResourceQueryFor(req))
	if matcher == nil {
		t.Fatalf("no pod matcher in query: %s", promResourceQueryFor(req))
	}
	re := regexp.MustCompile(`^(?:` + matcher[1] + `)$`) // Prometheus anchors as ^(?:re)$
	for _, pod := range []string{
		"tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-55d855bcb7sxgd",
		"tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-6d6cfb74ddh5dr",
		"tea-d98210cbbpdc73dcrkvg-qa-20260826-webhook-svc-7c964c469k4w25",
	} {
		if !re.MatchString(pod) {
			t.Errorf("resource query does not select live pod %q\nmatcher: %s", pod, matcher[1])
		}
	}
	// A short-named service in the same namespace must not be swept in.
	if re.MatchString("tea-d98210cbbpdc73dcrkvg-block-eden-mono-5f557c45fd-ll7x9") {
		t.Errorf("resource query selects another App's pods\nmatcher: %s", matcher[1])
	}
}

// TestNewPrometheusResourceSourceRoundTrip exercises the whole resource-history
// path (URL build + HTTP + matrix parse + label rewrite) against a stand-in.
func TestNewPrometheusResourceSourceRoundTrip(t *testing.T) {
	var gotValues url.Values
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotValues = r.URL.Query()
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[
		  {"metric":{"pod":"web-abc123-x2f4z"},"values":[[1000000,"0.5"],[1000060,"0.25"]]}]}}`))
	}))
	defer ts.Close()

	series, err := NewPrometheusResourceSource(ts.URL, ts.Client())(context.Background(), ResourceMetricsRangeRequest{
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
	// Prometheus's pod label is rewritten into the instance/resource vocabulary.
	if series[0].Labels["instance"] != "web-abc123-x2f4z" || series[0].Labels["resource"] != "web" {
		t.Errorf("labels not rewritten: %+v", series[0].Labels)
	}
	if _, still := series[0].Labels["pod"]; still {
		t.Errorf("raw pod label should not leak out: %+v", series[0].Labels)
	}
}

func TestNewPrometheusResourceSourceErrorStatus(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer ts.Close()

	if _, err := NewPrometheusResourceSource(ts.URL, ts.Client())(context.Background(),
		ResourceMetricsRangeRequest{Namespace: "default", App: "web", Metric: MetricCPU}); err == nil {
		t.Error("non-200 from prometheus should error")
	}
	down := NewPrometheusResourceSource("http://127.0.0.1:1", http.DefaultClient)
	if _, err := down(context.Background(), ResourceMetricsRangeRequest{Namespace: "default", App: "web", Metric: MetricMemory}); err == nil {
		t.Error("unreachable prometheus should error")
	}
}
