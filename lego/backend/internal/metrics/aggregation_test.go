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
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// w5/m140: Render's cpu interval aggregation (REST/GraphQL aggregationMethod,
// MCP cpuUsageAggregationMethod) and the per-host request-count breakdown.

func TestCPUUsageQueryAggregatesWithinEachStep(t *testing.T) {
	const m = `namespace="tea-x"`
	plain := resourceUsageQuery(MetricCPU, m, time.Minute, "")
	if plain != `sum by (pod) (max by (pod, container) (rate(container_cpu_usage_seconds_total{namespace="tea-x"}[60s])))` {
		t.Fatalf("AVG/omitted cpu query changed: %s", plain)
	}
	// At a 5-minute step the inner rate keeps a 60s lookback: the subquery
	// finds peaks inside the step, at a cost linear in the window.
	for agg, fn := range map[string]string{cpuAggregationMax: "max_over_time", cpuAggregationMin: "min_over_time"} {
		got := resourceUsageQuery(MetricCPU, m, 5*time.Minute, agg)
		want := fn + "((" + plain + ")[300s:15s])"
		if got != want {
			t.Errorf("%s = %s, want %s", agg, got, want)
		}
	}
	// A step finer than the scrape interval still spans one scrape.
	if got := resourceUsageQuery(MetricCPU, m, 10*time.Second, cpuAggregationMax); !strings.HasSuffix(got, "[15s:15s])") {
		t.Errorf("sub-scrape step = %s, want a 15s range", got)
	}
	if got := resourceUsageQuery(MetricMemory, m, time.Minute, cpuAggregationMax); strings.Contains(got, "_over_time") {
		t.Errorf("memory must never aggregate: %s", got)
	}
}

// rangeRecorder serves cpu/memory from a recording ranged source and returns
// what each read asked for.
func rangeRecorder(t *testing.T) (*Service, *[]ResourceMetricsRangeRequest) {
	t.Helper()
	var asked []ResourceMetricsRangeRequest
	svc := newService(nil, nil, sampleApp("web"), podWithLimits(webInst))
	svc.ResourceMetricsRange = func(_ context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		asked = append(asked, req)
		return []MetricSeries{{
			Labels: map[string]string{"instance": webInst},
			Points: []MetricPoint{{Timestamp: fixedClock().Format(time.RFC3339), Value: 0.5}},
		}}, nil
	}
	return svc, &asked
}

func lastCPUAggregation(t *testing.T, asked []ResourceMetricsRangeRequest, metric string) string {
	t.Helper()
	for i := len(asked) - 1; i >= 0; i-- {
		if asked[i].Metric == metric {
			return asked[i].CPUAggregation
		}
	}
	t.Fatalf("no %s read in %+v", metric, asked)
	return ""
}

func TestCPUAggregationReachesTheSourceOnEverySurface(t *testing.T) {
	for _, method := range []string{"MAX", "MIN"} {
		t.Run("REST "+method, func(t *testing.T) {
			svc, asked := rangeRecorder(t)
			rec := serveREST(svc, "/v1/metrics/cpu?resource=web&aggregationMethod="+method)
			if rec.Code != http.StatusOK {
				t.Fatalf("REST = %d %s", rec.Code, rec.Body)
			}
			if got := lastCPUAggregation(t, *asked, MetricCPU); got != method {
				t.Errorf("REST cpu read = %q, want %q", got, method)
			}
		})
		t.Run("GraphQL "+method, func(t *testing.T) {
			svc, asked := rangeRecorder(t)
			schema, err := gqlSchema(svc)
			if err != nil {
				t.Fatal(err)
			}
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
				RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU", aggregationMethod: "` + method + `"}) { unit } }`})
			if len(res.Errors) > 0 {
				t.Fatalf("GraphQL: %v", res.Errors)
			}
			if got := lastCPUAggregation(t, *asked, MetricCPU); got != method {
				t.Errorf("GraphQL cpu read = %q, want %q", got, method)
			}
		})
		t.Run("MCP "+method, func(t *testing.T) {
			svc, asked := rangeRecorder(t)
			res, err := mcpSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
				"resourceId": "web", "metricTypes": []string{MetricCPU, MetricMemory}, "cpuUsageAggregationMethod": method,
			}})
			if err != nil || res.IsError {
				raw, _ := json.Marshal(res)
				t.Fatalf("MCP: err=%v %s", err, raw)
			}
			if got := lastCPUAggregation(t, *asked, MetricCPU); got != method {
				t.Errorf("MCP cpu read = %q, want %q", got, method)
			}
			// Render's MCP passes the aggregation to its cpu read only.
			if got := lastCPUAggregation(t, *asked, MetricMemory); got != "" {
				t.Errorf("MCP memory read = %q, want unaggregated", got)
			}
		})
	}
}

func TestCPUAggregationOnAnotherMetricIsRefusedAlike(t *testing.T) {
	svc, _ := rangeRecorder(t)
	if rec := serveREST(svc, "/v1/metrics/memory?resource=web&aggregationMethod=MAX"); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "applies only to cpu") {
		t.Errorf("REST memory MAX = %d %s, want 400", rec.Code, rec.Body)
	}
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "MEMORY", aggregationMethod: "MAX"}) { unit } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "applies only to cpu") {
		t.Errorf("GraphQL memory MAX errors = %v, want the same refusal", res.Errors)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU", aggregationMethod: "P99"}) { unit } }`})
	if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "aggregationMethod") {
		t.Errorf("GraphQL unknown method errors = %v, want a refusal naming aggregationMethod", res.Errors)
	}
}

func TestDatastoreCPUAggregationReachesTheDatastoreSource(t *testing.T) {
	const pgID = "dpg-c185th5c2rvvnhbfiltg"
	svc := newService(nil, nil, sampleDatabase(pgID, false))
	var asked []DatastoreResourceRequest
	svc.DatastoreResource = recordingResourceSource(&asked)
	if rec := serveREST(svc, "/v1/metrics/cpu?resource="+pgID+"&aggregationMethod=MAX"); rec.Code != http.StatusOK {
		t.Fatalf("REST datastore cpu MAX = %d %s", rec.Code, rec.Body)
	}
	if len(asked) != 1 || asked[0].CPUAggregation != cpuAggregationMax {
		t.Fatalf("datastore reads = %+v, want one MAX cpu read", asked)
	}
	q := promDatastoreResourceQuery(asked[0])
	if !strings.HasPrefix(q, "max_over_time((") {
		t.Errorf("datastore cpu MAX query = %s", q)
	}
}

func TestHostBreakdownReadsTheRequestLogStoreOnly(t *testing.T) {
	counterCalled := false
	counter := func(context.Context, RequestMetricsRequest) ([]MetricSeries, error) {
		counterCalled = true
		return []MetricSeries{{Points: []MetricPoint{{Value: 9}}}}, nil
	}
	svc := newService(nil, counter, sampleApp("web"), podFor("web", webInst))
	var lokiAsked RequestMetricsRequest
	lokiErr := error(nil)
	svc.RequestLogMetrics = func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		lokiAsked = req
		if lokiErr != nil {
			return nil, lokiErr
		}
		return []MetricSeries{
			{Labels: map[string]string{"host": "a.example"}, Points: []MetricPoint{{Value: 3}}},
			{Labels: map[string]string{"host": "b.example"}, Points: []MetricPoint{{Value: 4}}},
		}, nil
	}

	rec := serveREST(svc, "/v1/metrics/http-requests?resource=web&aggregateBy=host")
	if rec.Code != http.StatusOK {
		t.Fatalf("aggregateBy=host = %d %s", rec.Code, rec.Body)
	}
	if lokiAsked.GroupBy != groupByHost {
		t.Errorf("Loki GroupBy = %q, want host", lokiAsked.GroupBy)
	}
	series := seriesFrom(t, rec.Body.Bytes())
	if len(series) != 2 || labelValue(series[0].Labels, "host") != "a.example" || labelValue(series[1].Labels, "host") != "b.example" {
		t.Errorf("series = %+v, want one per host labelled host", series)
	}

	// A Loki failure is an error, never the host-less counter.
	lokiErr = errors.New("loki down")
	if rec := serveREST(svc, "/v1/metrics/http-requests?resource=web&aggregateBy=host"); rec.Code < 500 {
		t.Errorf("host breakdown with Loki down = %d, want a server error", rec.Code)
	}
	if counterCalled {
		t.Error("host breakdown fell back to the Traefik counter")
	}
	// The ungrouped count keeps its counter fallback.
	if rec := serveREST(svc, "/v1/metrics/http-requests?resource=web"); rec.Code != http.StatusOK || !counterCalled {
		t.Errorf("ungrouped count with Loki down = %d (counter called %v), want the counter fallback", rec.Code, counterCalled)
	}

	// Render scopes the host breakdown to request counts.
	if rec := serveREST(svc, "/v1/metrics/http-latency?resource=web&aggregateBy=host"); rec.Code != http.StatusBadRequest {
		t.Errorf("latency aggregateBy=host = %d, want 400", rec.Code)
	}
}

func TestLokiHostBreakdownQuery(t *testing.T) {
	q := lokiRequestQueryFor(RequestMetricsRequest{Namespace: "tea-x", App: "web", Metric: MetricHTTPRequests, GroupBy: groupByHost, Resolution: time.Minute})
	if !strings.HasPrefix(q, "sum by (request_host) (count_over_time(") || !strings.Contains(q, `request_host="RequestHost"`) {
		t.Errorf("host breakdown query = %s", q)
	}
}

func TestGraphQLHostBreakdownIsNotDropped(t *testing.T) {
	svc := newService(nil, nil, sampleApp("web"), podFor("web", webInst))
	var got string
	svc.RequestLogMetrics = func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		got = req.GroupBy
		return []MetricSeries{{Labels: map[string]string{"host": "a.example"}, Points: []MetricPoint{{Value: 1}}}}, nil
	}
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "HTTP_REQUESTS", aggregateBy: ["HOST"]}) { unit } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL: %v", res.Errors)
	}
	if got != groupByHost {
		t.Errorf("GraphQL HOST reached the source as %q, want host", got)
	}
}

// TestCPUAggregationQueryOnPrometheus evaluates the production MAX/MIN cpu
// expressions in Prometheus' own engine via `promtool test rules`. One pod
// runs at 0.25 cores, bursts to 1 core for ten minutes, then drops back. The
// assertions hold on both range semantics (v2.55.1 closed ranges, the CI pin;
// v3 left-open): MAX reports the burst exactly once the step window touches
// it, and MIN < AVG < MAX where the series is changing.
func TestCPUAggregationQueryOnPrometheus(t *testing.T) {
	promtool, err := exec.LookPath("promtool")
	if err != nil {
		testenv.Skip(t, "promtool not on PATH")
	}
	// A cumulative counter sampled every 15 s: 40 samples at 0.25 cores
	// (3.75 s per step), 40 at 1 core, 40 at 0.25.
	var values []string
	c := 0.0
	for i := 0; i <= 120; i++ {
		values = append(values, strconv.FormatFloat(c, 'f', -1, 64))
		if i >= 40 && i < 80 {
			c += 15
		} else {
			c += 3.75
		}
	}
	const m = `namespace="agg"`
	q := func(agg string) string { return resourceUsageQuery(MetricCPU, m, time.Minute, agg) }
	type check struct {
		expr, at string
		value    float64
	}
	checks := []check{
		// 20m45s: the step window still holds a pure-burst sample.
		{"round(" + q(cpuAggregationMax) + ", 0.001)", "1245s", 1},
		{"(" + q("") + ") < bool (" + q(cpuAggregationMax) + ")", "1245s", 1},
		// 11m: rising out of the quiet phase, so the window's minimum lags.
		{"(" + q(cpuAggregationMin) + ") < bool (" + q("") + ")", "660s", 1},
	}
	var b strings.Builder
	fmt.Fprintf(&b, "rule_files: []\nevaluation_interval: 15s\ntests:\n  - interval: 15s\n    input_series:\n")
	fmt.Fprintf(&b, "      - series: 'container_cpu_usage_seconds_total{namespace=\"agg\",pod=\"web-1\",container=\"app\"}'\n        values: '%s'\n", strings.Join(values, " "))
	b.WriteString("    promql_expr_test:\n")
	for _, ck := range checks {
		fmt.Fprintf(&b, "      - expr: '%s'\n        eval_time: %s\n        exp_samples:\n          - labels: '{pod=\"web-1\"}'\n            value: %g\n", ck.expr, ck.at, ck.value)
	}
	path := filepath.Join(t.TempDir(), "cpu_aggregation.yml")
	if err := os.WriteFile(path, []byte(b.String()), 0o600); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(promtool, "test", "rules", path).CombinedOutput(); err != nil {
		t.Fatalf("promtool: %v\n%s\n%s", err, out, b.String())
	}
}

// TestLokiHostBreakdownOnLoki runs the per-host request count in the pinned
// Loki (BEX_TEST_LOKI_URL): lines for two request hosts come back as two
// series labelled with Render's `host`, each counting only its own lines.
func TestLokiHostBreakdownOnLoki(t *testing.T) {
	base := os.Getenv("BEX_TEST_LOKI_URL")
	if base == "" {
		testenv.Skip(t, "BEX_TEST_LOKI_URL not set")
	}
	base = strings.TrimRight(base, "/")
	ns := fmt.Sprintf("m140-%d", time.Now().UnixNano())
	bucket := time.Now().Add(-5 * time.Minute).Truncate(time.Minute)

	type stream struct {
		Stream map[string]string `json:"stream"`
		Values [][2]string       `json:"values"`
	}
	var streams []stream
	for i, host := range []string{"a.example.com", "a.example.com", "a.example.com", "b.example.com"} {
		msg, _ := json.Marshal(map[string]any{
			"RequestHost": host, "RequestPath": "/", "RequestMethod": "GET", "DownstreamStatus": "200", "Duration": 1_000_000,
		})
		streams = append(streams, stream{
			Stream: map[string]string{"namespace": ns, "app": "web", "type": "request", "status": "200", "method": "GET"},
			Values: [][2]string{{strconv.FormatInt(bucket.Add(time.Duration(10+i)*time.Second).UnixNano(), 10), string(msg)}},
		})
	}
	body, _ := json.Marshal(map[string]any{"streams": streams})
	hc := &http.Client{Timeout: 10 * time.Second}
	resp, err := hc.Post(base+"/loki/api/v1/push", "application/json", strings.NewReader(string(body)))
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("push: HTTP %d", resp.StatusCode)
	}

	source := NewLokiRequestMetricsSource(base, hc)
	totals := map[string]float64{}
	// Loki ingests asynchronously; poll briefly until both hosts are visible.
	for deadline := time.Now().Add(20 * time.Second); time.Now().Before(deadline); time.Sleep(time.Second) {
		series, err := source(context.Background(), RequestMetricsRequest{
			Namespace: ns, App: "web", Metric: MetricHTTPRequests, GroupBy: groupByHost,
			Start: bucket, End: bucket.Add(2 * time.Minute), Resolution: time.Minute,
		})
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		totals = map[string]float64{}
		for _, s := range series {
			if _, leaked := s.Labels["request_host"]; leaked {
				t.Fatalf("series kept the store's request_host label: %v", s.Labels)
			}
			for _, p := range s.Points {
				totals[s.Labels["host"]] += p.Value
			}
		}
		if totals["a.example.com"] == 3 && totals["b.example.com"] == 1 {
			return
		}
	}
	t.Fatalf("per-host totals = %v, want a.example.com=3 b.example.com=1", totals)
}

// TestNoMetricsArgumentIsAcceptedWithoutEffect is w5/m140's guard against the
// class GraphQL's aggregationMethod and aggregateBy HOST belonged to: every
// field of the GraphQL MetricsQueryInput, and every query parameter Render's
// pinned OpenAPI declares on /metrics/cpu and /metrics/http-requests, must
// either change the parsed query or be refused. A new argument without a probe
// fails here until someone decides what it does.
// TestRepeatedHostPathValuesAreKept is w4/401: Render ORs repeated host/path
// values; both adapters must keep every value, in either order, and GraphQL
// must refuse several STATUS_CODE values instead of silently using the first.
func TestRepeatedHostPathValuesAreKept(t *testing.T) {
	for _, url := range []string{
		"/v1/metrics/http-requests?resource=web&path=/nf&path=/err&host=a.example.com",
		"/v1/metrics/http-requests?resource=web&path=/err&path=/nf&host=a.example.com",
	} {
		_, q, err := parseMetricParams(httptest.NewRequest(http.MethodGet, url, nil))
		if err != nil {
			t.Fatalf("%s: %v", url, err)
		}
		if len(q.Paths) != 2 || len(q.Hosts) != 1 {
			t.Errorf("%s: paths=%v hosts=%v, want both paths and the host", url, q.Paths, q.Hosts)
		}
	}
	gql := func(filters ...any) map[string]any {
		return map[string]any{"name": "HTTP_REQUESTS", "filters": filters}
	}
	_, q, err := metricsQueryInputFromArgs(gql(
		map[string]any{"field": "RESOURCE", "values": []any{"web"}},
		map[string]any{"field": "PATH", "values": []any{"/nf", "/err"}},
		map[string]any{"field": "HOST", "values": []any{"a.example.com", "b.example.com"}},
	))
	if err != nil {
		t.Fatalf("GraphQL multi host/path: %v", err)
	}
	if len(q.Paths) != 2 || len(q.Hosts) != 2 {
		t.Errorf("GraphQL paths=%v hosts=%v, want both of each", q.Paths, q.Hosts)
	}
	if _, _, err := metricsQueryInputFromArgs(gql(
		map[string]any{"field": "RESOURCE", "values": []any{"web"}},
		map[string]any{"field": "STATUS_CODE", "values": []any{"200", "500"}},
	)); err == nil {
		t.Error("GraphQL STATUS_CODE with two values: want a refusal, got none")
	}
}

func TestNoMetricsArgumentIsAcceptedWithoutEffect(t *testing.T) {

	t.Run("GraphQL MetricsQueryInput", func(t *testing.T) {
		probes := map[string]any{
			"start":              "2026-10-01T00:00:00Z",
			"end":                "2026-10-01T01:00:00Z",
			"name":               "MEMORY",
			"resolution":         300,
			"parameters":         []any{map[string]any{"quantile": 0.9}},
			"aggregateBy":        []any{"HOST"},
			"aggregationMethod":  "MAX",
			"aggregateAllMethod": "MAX",
			"percentage":         true,
			"filters":            []any{map[string]any{"field": "RESOURCE", "values": []any{"other"}}},
		}
		baseline := func() map[string]any {
			return map[string]any{
				"name":    "CPU",
				"filters": []any{map[string]any{"field": "RESOURCE", "values": []any{"web"}}},
			}
		}
		baseRes, baseQ, err := metricsQueryInputFromArgs(baseline())
		if err != nil {
			t.Fatal(err)
		}
		for field := range metricsQueryInputType.Fields() {
			probe, ok := probes[field]
			if !ok {
				t.Errorf("MetricsQueryInput.%s has no probe: decide what it does and add one", field)
				continue
			}
			in := baseline()
			in[field] = probe
			res, q, err := metricsQueryInputFromArgs(in)
			if err == nil && reflect.DeepEqual(res, baseRes) && reflect.DeepEqual(q, baseQ) {
				t.Errorf("MetricsQueryInput.%s=%v is accepted and changes nothing", field, probe)
			}
		}
	})

	t.Run("REST query parameters Render declares", func(t *testing.T) {
		raw, err := os.ReadFile("../api/openapi/render-public-api-1.json")
		if err != nil {
			t.Fatal(err)
		}
		var spec map[string]any
		if err := json.Unmarshal(raw, &spec); err != nil {
			t.Fatal(err)
		}
		// resolve follows a local JSON pointer ($ref "#/a/b~1c/0").
		resolve := func(ref string) any {
			var node any = spec
			for _, tok := range strings.Split(strings.TrimPrefix(ref, "#/"), "/") {
				tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
				switch n := node.(type) {
				case map[string]any:
					node = n[tok]
				case []any:
					i, _ := strconv.Atoi(tok)
					node = n[i]
				}
			}
			return node
		}
		paramNames := func(path string) []string {
			item, _ := spec["paths"].(map[string]any)[path].(map[string]any)
			params, _ := item["parameters"].([]any)
			var names []string
			for _, p := range params {
				m, _ := p.(map[string]any)
				for ref, ok := m["$ref"].(string); ok; ref, ok = m["$ref"].(string) {
					m, _ = resolve(ref).(map[string]any)
				}
				names = append(names, m["name"].(string))
			}
			return names
		}
		probes := map[string]string{
			"startTime":         "2026-10-01T00:00:00Z",
			"endTime":           "2026-10-01T01:00:00Z",
			"resolutionSeconds": "300",
			"resource":          "other",
			"service":           "other",
			"instance":          "web-abc-1",
			"aggregationMethod": "MAX",
			"aggregateBy":       "host",
			"host":              "a.example.com",
			"path":              "/x",
		}
		for _, path := range []string{"/metrics/cpu", "/metrics/http-requests"} {
			names := paramNames(path)
			if len(names) == 0 {
				t.Fatalf("pinned spec declares no parameters on %s", path)
			}
			for _, name := range names {
				probe, ok := probes[name]
				if !ok {
					t.Errorf("%s ?%s has no probe: decide what it does and add one", path, name)
					continue
				}
				parse := func(query string) ([]string, MetricQuery, error) {
					return parseMetricParams(httptest.NewRequest(http.MethodGet, "/v1"+path+"?"+query, nil))
				}
				baseRes, baseQ, err := parse("resource=web")
				if err != nil {
					t.Fatal(err)
				}
				query := "resource=web&" + name + "=" + probe
				if name == "resource" || name == "service" {
					query = name + "=" + probe
				}
				res, q, err := parse(query)
				if err == nil && reflect.DeepEqual(res, baseRes) && reflect.DeepEqual(q, baseQ) {
					t.Errorf("%s ?%s=%s is accepted and changes nothing", path, name, probe)
				}
			}
		}
	})
}
