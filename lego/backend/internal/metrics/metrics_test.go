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
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
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

const (
	managedAppID         = "srv-c185th5c2rvvnhbfiltg"
	managedAppName       = "tea-d98210cbbpdc73dcrkvg-bex"
	managedAppPublicName = "bex"
	managedAppPod        = managedAppName + "-6d5896f9c9-abcde"
)

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

func ingressFor(app, backend string, hosts ...string) *networkingv1.Ingress {
	class := "traefik"
	pathType := networkingv1.PathTypePrefix
	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: app, Namespace: "default"},
		Spec:       networkingv1.IngressSpec{IngressClassName: &class},
	}
	for _, host := range hosts {
		ingress.Spec.Rules = append(ingress.Spec.Rules, networkingv1.IngressRule{
			Host: host,
			IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
				Paths: []networkingv1.HTTPIngressPath{{
					Path:     "/",
					PathType: &pathType,
					Backend: networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
						Name: backend,
					}},
				}},
			}},
		})
	}
	return ingress
}

// podWithLimitsFor is podFor(app, name) plus a 1-core / 1Gi limit, so
// percentage-mode metrics have a denominator.
func podWithLimitsFor(app, name string) *corev1.Pod {
	p := podFor(app, name)
	p.Spec.Containers = []corev1.Container{{
		Name: core.AppContainer,
		Resources: corev1.ResourceRequirements{Limits: corev1.ResourceList{
			corev1.ResourceCPU:    resource.MustParse("1"),
			corev1.ResourceMemory: resource.MustParse("1Gi"),
		}},
	}}
	return p
}

func podWithLimits(name string) *corev1.Pod {
	return podWithLimitsFor("web", name)
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

type managedMetricsCalls struct {
	ranged  []ResourceMetricsRangeRequest
	request []RequestMetricsRequest
	filters []string
}

// newManagedMetricsService reproduces the production identity shape: the
// public service name, stable srv- id, and Kubernetes App name are all
// different. Every backend source rejects the public id so a test can only pass
// when the service resolves operational selectors through the authorized App.
func newManagedMetricsService(t *testing.T) (*Service, *managedMetricsCalls) {
	t.Helper()
	app := sampleApp(managedAppName)
	app.Labels = map[string]string{
		core.LabelAppID:       managedAppID,
		core.LabelServiceName: managedAppPublicName,
	}
	app.Spec.Host = managedAppPublicName + ".onbex.co"

	calls := &managedMetricsCalls{}
	svc := newService(nil, nil,
		app,
		podWithLimitsFor(managedAppName, managedAppPod),
		ingressFor(managedAppName, managedAppName, app.Spec.Host),
	)
	svc.ResourceMetricsRange = func(_ context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		calls.ranged = append(calls.ranged, req)
		if req.App != managedAppName {
			return nil, fmt.Errorf("resource selector app = %q, want %q", req.App, managedAppName)
		}
		labels := map[string]string{"resource": managedAppName}
		value := float64(1)
		if req.Metric != MetricInstanceCount {
			labels["instance"] = managedAppPod
		}
		switch req.Metric {
		case MetricCPU:
			value = 0.5
		case MetricMemory:
			value = 512 << 20
		}
		return []MetricSeries{{
			Labels: labels,
			Points: []MetricPoint{{Timestamp: fixedClock().Format(time.RFC3339), Value: value}},
		}}, nil
	}
	svc.RequestMetrics = func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		calls.request = append(calls.request, req)
		if req.App != managedAppName {
			return nil, fmt.Errorf("request selector app = %q, want %q", req.App, managedAppName)
		}
		if req.AppID != managedAppID {
			return nil, fmt.Errorf("request app id = %q, want %q", req.AppID, managedAppID)
		}
		return []MetricSeries{{
			Points: []MetricPoint{{Timestamp: fixedClock().Format(time.RFC3339), Value: 7}},
		}}, nil
	}
	svc.MetricsFilterValuesSource = func(_ context.Context, _, app, label string) ([]string, error) {
		calls.filters = append(calls.filters, app)
		if app != managedAppName {
			return nil, fmt.Errorf("filter selector app = %q, want %q", app, managedAppName)
		}
		if label != "code" {
			return nil, fmt.Errorf("filter label = %q, want code", label)
		}
		return []string{"200", "500"}, nil
	}
	return svc, calls
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

func TestManagedServiceIDUsesResolvedAppNameForEveryMetricsLookup(t *testing.T) {
	svc, calls := newManagedMetricsService(t)
	ctx := context.Background()

	resourceCases := []struct {
		metric     string
		percentage bool
		wantValue  float64
	}{
		{metric: MetricCPU, percentage: true, wantValue: 50},
		{metric: MetricMemory, wantValue: 512 << 20},
		{metric: MetricCPULimit, wantValue: 1},
		{metric: MetricMemoryLimit, wantValue: 1 << 30},
		{metric: MetricInstanceCount, wantValue: 1},
	}
	for _, tc := range resourceCases {
		t.Run(tc.metric, func(t *testing.T) {
			series, err := svc.Metrics(ctx, MetricQuery{
				App: managedAppID, Metric: tc.metric, Percentage: tc.percentage,
			})
			if err != nil || len(series) != 1 {
				t.Fatalf("Metrics: err=%v series=%+v", err, series)
			}
			if got := series[0].Labels["resource"]; got != managedAppID {
				t.Errorf("response resource = %q, want public id %q", got, managedAppID)
			}
			if got := series[0].Points[0].Value; got != tc.wantValue {
				t.Errorf("value = %v, want %v", got, tc.wantValue)
			}
		})
	}

	for _, metric := range []string{MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth} {
		t.Run(metric, func(t *testing.T) {
			series, err := svc.Metrics(ctx, MetricQuery{App: managedAppID, Metric: metric})
			if err != nil || len(series) != 1 || series[0].Points[0].Value != 7 {
				t.Fatalf("Metrics: err=%v series=%+v", err, series)
			}
		})
	}

	if len(calls.ranged) != 3 {
		t.Fatalf("ranged source calls = %d, want CPU, memory, and instance count", len(calls.ranged))
	}
	for _, req := range calls.ranged {
		if req.App != managedAppName {
			t.Errorf("ranged source app = %q, want %q", req.App, managedAppName)
		}
	}
	if len(calls.request) != 3 {
		t.Fatalf("request source calls = %d, want request count, latency, and bandwidth", len(calls.request))
	}
	for _, req := range calls.request {
		if req.App != managedAppName || req.AppID != managedAppID {
			t.Errorf("request source identity = app %q id %q", req.App, req.AppID)
		}
		if req.Metric == MetricBandwidth && len(req.Routers) != 1 {
			t.Errorf("bandwidth routers = %v, want the resolved App ingress router", req.Routers)
		}
	}

	filters, err := svc.MetricsFilters(ctx, MetricsFiltersQuery{
		App: managedAppID, OutputFilters: []string{filterFieldResource, "INSTANCE", "STATUS_CODE"},
	})
	if err != nil {
		t.Fatalf("MetricsFilters: %v", err)
	}
	if len(filters) != 3 || len(filters[0].Values) != 1 || filters[0].Values[0] != managedAppID {
		t.Fatalf("resource filter should retain public id: %+v", filters)
	}
	if len(filters[1].Values) != 1 || filters[1].Values[0] != managedAppPod {
		t.Errorf("instance filter should come from resolved App pods: %+v", filters[1])
	}
	if len(filters[2].Values) != 2 || filters[2].Values[1] != "500" {
		t.Errorf("status filter should come from resolved Prometheus selector: %+v", filters[2])
	}
	if len(calls.filters) != 1 || calls.filters[0] != managedAppName {
		t.Errorf("filter source apps = %v, want [%s]", calls.filters, managedAppName)
	}
}

func TestManagedServiceIDMetricsAcrossRESTGraphQLAndMCP(t *testing.T) {
	svc, _ := newManagedMetricsService(t)

	var rest []renderMetricSeries
	rec := serveREST(svc, "/v1/metrics/cpu?resource="+managedAppID)
	if rec.Code != http.StatusOK {
		t.Fatalf("REST status = %d: %s", rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || len(rest) != 1 {
		t.Fatalf("REST decode: err=%v series=%+v", err, rest)
	}
	var restResource string
	for _, label := range rest[0].Labels {
		if label.Field == "resource" {
			restResource = label.Value
		}
	}
	if restResource != managedAppID {
		t.Errorf("REST resource label = %q, want %q", restResource, managedAppID)
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("GraphQL schema: %v", err)
	}
	gql := graphql.Do(graphql.Params{
		Schema: schema, Context: context.Background(),
		RequestString: fmt.Sprintf(`{ metrics(query: {filters: [{field: "RESOURCE", values: [%q]}], name: "CPU"}) { unit values { value } } }`, managedAppID),
	})
	if len(gql.Errors) > 0 {
		t.Fatalf("GraphQL errors: %v", gql.Errors)
	}
	gqlSeries := gql.Data.(map[string]any)["metrics"].([]any)
	if len(gqlSeries) != 1 || gqlSeries[0].(map[string]any)["unit"] != unitCores {
		t.Fatalf("GraphQL series = %+v", gqlSeries)
	}

	ctx := context.Background()
	server := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(server)
	serverTransport, clientTransport := mcp.NewInMemoryTransports()
	if _, err := server.Connect(ctx, serverTransport, nil); err != nil {
		t.Fatalf("MCP server connect: %v", err)
	}
	clientSession, err := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
	if err != nil {
		t.Fatalf("MCP client connect: %v", err)
	}
	defer clientSession.Close()
	result, err := clientSession.CallTool(ctx, &mcp.CallToolParams{
		Name: "get_metrics",
		Arguments: map[string]any{
			"resource": []string{managedAppID}, "metricTypes": []string{MetricCPU},
		},
	})
	if err != nil || result.IsError {
		t.Fatalf("MCP get_metrics: err=%v result=%+v", err, result)
	}
	raw, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("MCP marshal: %v", err)
	}
	var mcpMetrics getMetricsResult
	if err := json.Unmarshal(raw, &mcpMetrics); err != nil || len(mcpMetrics.Series) != 1 {
		t.Fatalf("MCP decode: err=%v series=%+v raw=%s", err, mcpMetrics.Series, raw)
	}
	if got := mcpMetrics.Series[0].Labels["resource"]; got != managedAppID {
		t.Errorf("MCP resource label = %q, want %q", got, managedAppID)
	}
}

// TestHostPathFiltersRejected: a host/path filter is refused with ErrBadRequest
// before any source is queried — Traefik's service-level counters carry no
// host/path labels, so honoring the query shape while ignoring the filter would
// return whole-service numbers dressed up as host/path-scoped ones (w3/m12).
func TestHostPathFiltersRejected(t *testing.T) {
	sourceCalled := false
	req := func(_ context.Context, _ RequestMetricsRequest) ([]MetricSeries, error) {
		sourceCalled = true
		return []MetricSeries{{Points: []MetricPoint{{Value: 1}}}}, nil
	}
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5},
	}), req, sampleApp("web"), podWithLimits(webInst))

	for _, q := range []MetricQuery{
		{App: "web", Metric: MetricHTTPRequests, Host: "web.example.com"},
		{App: "web", Metric: MetricHTTPLatency, Path: "/api"},
		{App: "web", Metric: MetricBandwidth, Host: "web.example.com", Path: "/api"},
		{App: "web", Metric: MetricCPU, Path: "/api"}, // every metric refuses, not just the request family
	} {
		_, err := svc.Metrics(context.Background(), q)
		if !errors.Is(err, core.ErrBadRequest) {
			t.Errorf("%s with host=%q path=%q: want ErrBadRequest, got %v", q.Metric, q.Host, q.Path, err)
		}
	}
	if sourceCalled {
		t.Error("a rejected query must not reach the request-metrics source")
	}
	// The same queries without host/path still answer.
	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricHTTPRequests, StatusCode: "5xx"}); err != nil {
		t.Errorf("unfiltered http_requests should succeed, got %v", err)
	}
}

// TestMetricsFiltersOfferNoHostPathValues: the filter-discovery verb reports
// empty HOST/PATH values even when the App has live hosts — a client is never
// handed a filter value the Metrics verb would refuse (w3/m12).
func TestMetricsFiltersOfferNoHostPathValues(t *testing.T) {
	app := sampleApp("web")
	app.Status.URLs = []string{"https://web.example.com"}
	svc := newService(nil, nil, app)

	vals, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{
		App: "web", OutputFilters: []string{"HOST", "PATH", "RESOURCE"},
	})
	if err != nil {
		t.Fatalf("filters: %v", err)
	}
	for _, v := range vals {
		if v.Field == filterFieldResource {
			continue // RESOURCE stays answerable
		}
		if len(v.Values) != 0 {
			t.Errorf("%s should offer no values, got %v", v.Field, v.Values)
		}
	}
}

func TestBandwidthMetricResolvesExactIngressRouters(t *testing.T) {
	var got RequestMetricsRequest
	req := func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
		got = r
		return []MetricSeries{}, nil
	}
	app := sampleApp("static")
	app.Labels = map[string]string{core.LabelAppID: "srv-static"}
	app.Spec.Type = appv1alpha1.TypeStaticSite
	app.Spec.Host = "site.onbex.co"
	app.Spec.Hosts = []string{"www.example.com"}
	svc := newService(nil, req, app, ingressFor("static", "shared-static-server", "site.onbex.co", "www.example.com"))

	if _, err := svc.Metrics(context.Background(), MetricQuery{App: "static", Metric: MetricBandwidth}); err != nil {
		t.Fatalf("bandwidth metric: %v", err)
	}
	want := []string{
		"default-static-site-onbex-co@kubernetes",
		"default-static-www-example-com@kubernetes",
	}
	if len(got.Routers) != len(want) || got.Routers[0] != want[0] || got.Routers[1] != want[1] {
		t.Fatalf("resolved routers: got %v, want %v", got.Routers, want)
	}
	if got.AppID != "srv-static" || got.Direct {
		t.Fatalf("bandwidth attribution/applicability: appID=%q direct=%v", got.AppID, got.Direct)
	}
}

func TestMonthToDateBandwidthReportsRealCategoriesAndExpandedTotal(t *testing.T) {
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelAppID: "srv-web"}
	app.Spec.Host = "web.onbex.co"
	svc := newService(nil, nil, app, ingressFor("web", "web", "web.onbex.co"))
	svc.MonthToDateBandwidthSource = func(_ context.Context, appID string, routers []string, direct bool, _, _ time.Time) (BandwidthBytes, error) {
		if appID != "srv-web" || len(routers) != 1 || !direct {
			t.Fatalf("source identity/applicability: id=%q routers=%v direct=%v", appID, routers, direct)
		}
		return BandwidthBytes{HTTP: 1 << 20, NAT: 2 << 20, WebSocket: 3 << 20}, nil
	}
	got, err := svc.MonthToDateBandwidth(context.Background(), "web")
	if err != nil {
		t.Fatal(err)
	}
	if got.HTTPEgressBandwidthMB != 1 || got.NATEgressBandwidthMB != 2 || got.WebsocketEgressBandwidthMB != 3 || got.EgressBandwidthMB != 6 || got.PrivateLinkEgressBandwidthMB != 0 {
		t.Fatalf("month bandwidth categories: %+v", got)
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

// TestRESTHostPathFiltersReturn400: REST maps the host/path rejection to a 400
// naming the filter, on every request-metric endpoint — never a silently
// unfiltered 200 (w3/m12).
func TestRESTHostPathFiltersReturn400(t *testing.T) {
	req := func(_ context.Context, _ RequestMetricsRequest) ([]MetricSeries, error) {
		return []MetricSeries{{Points: []MetricPoint{{Value: 1}}}}, nil
	}
	svc := newService(nil, req, sampleApp("web"), podFor("web", webInst))

	for _, seg := range []string{"http-requests", "http-latency", "bandwidth"} {
		for _, filter := range []string{"host=web.example.com", "path=%2Fapi"} {
			rec := serveREST(svc, "/v1/metrics/"+seg+"?resource=web&"+filter)
			if rec.Code != 400 {
				t.Errorf("%s?%s: want 400, got %d (%s)", seg, filter, rec.Code, rec.Body.String())
			} else if !strings.Contains(rec.Body.String(), "host/path") {
				t.Errorf("%s?%s: 400 body should name the filter, got %s", seg, filter, rec.Body.String())
			}
		}
		// The same endpoint without host/path still answers.
		if rec := serveREST(svc, "/v1/metrics/"+seg+"?resource=web&statusCode=5xx"); rec.Code != 200 {
			t.Errorf("%s unfiltered: want 200, got %d (%s)", seg, rec.Code, rec.Body.String())
		}
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

	// A HOST/PATH filter is refused, mirroring REST's 400 (w3/m12).
	for _, field := range []string{"HOST", "PATH"} {
		res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
			RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}, {field: "` + field + `", values: ["x"]}], name: "HTTP_REQUESTS"}) { unit } }`})
		if len(res.Errors) == 0 {
			t.Errorf("%s filter should error", field)
		} else if !strings.Contains(res.Errors[0].Message, "host/path") {
			t.Errorf("%s filter error should name the filter, got %q", field, res.Errors[0].Message)
		}
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
	bandwidth := promQueryFor(RequestMetricsRequest{
		Metric: MetricBandwidth, Resolution: 60 * time.Second, AppID: "srv-web", Direct: true,
		Routers: []string{"default-web-web.onbex.co@kubernetes", "default-web-api-web.onbex.co@kubernetes"},
	})
	for _, metric := range []string{"traefik_router_responses_bytes_total", "bex_websocket_egress_bytes_total", "bex_app_direct_egress_bytes_total"} {
		if strings.Count(bandwidth, metric) != 1 {
			t.Errorf("bandwidth query should contain %s exactly once: %q", metric, bandwidth)
		}
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

func TestPrometheusRequestSourceSkipsPrivateBandwidthQuery(t *testing.T) {
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { called = true }))
	defer ts.Close()
	series, err := NewPrometheusRequestSource(ts.URL, ts.Client())(context.Background(), RequestMetricsRequest{
		Metric: MetricBandwidth, Start: time.Now().Add(-time.Minute), End: time.Now(), Resolution: time.Minute,
	})
	if err != nil || len(series) != 0 || called {
		t.Fatalf("private bandwidth: series=%v err=%v called=%v, want empty/no query", series, err, called)
	}
}

func TestMonthToDateBandwidthSourceUsesExactRouterCounter(t *testing.T) {
	var mu sync.Mutex
	var gotQueries []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		mu.Lock()
		gotQueries = append(gotQueries, query)
		mu.Unlock()
		value := "1"
		switch {
		case strings.Contains(query, "traefik_router_responses_bytes_total"):
			value = "1048576"
		case strings.Contains(query, "bex_websocket_egress_bytes_total"):
			value = "2097152"
		case strings.Contains(query, "bex_app_direct_egress_bytes_total"):
			value = "3145728"
		}
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"vector","result":[{"value":[0,%q]}]}}`, value)
	}))
	defer ts.Close()
	now := time.Now()
	value, err := NewMonthToDateBandwidthSource(ts.URL, ts.Client())(
		context.Background(), "srv-web", []string{"default-web-web-onbex-co@kubernetes"}, true, now.Add(-time.Hour), now)
	if err != nil || value.HTTP != 1048576 || value.WebSocket != 2097152 || value.NAT != 3145728 {
		t.Fatalf("month source: value=%v err=%v", value, err)
	}
	mu.Lock()
	gotQuery := strings.Join(gotQueries, "\n")
	mu.Unlock()
	if !strings.Contains(gotQuery, "traefik_router_responses_bytes_total") ||
		!strings.Contains(gotQuery, `router=~"^(default-web-web-onbex-co@kubernetes)$"`) ||
		strings.Contains(gotQuery, "traefik_service_responses_bytes_total") {
		t.Fatalf("month source query is not exact-router based: %q", gotQuery)
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
