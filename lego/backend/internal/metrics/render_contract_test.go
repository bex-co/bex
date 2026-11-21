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
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// w1/m155: Render's metrics contract on REST and MCP. At filing time REST
// aggregateBy=statusCode silently returned the single ungrouped series, eight
// Render /metrics paths were bare 404s, and MCP refused Render's metric names.

// groupedRequestSource answers http_requests with one series per status code
// when the query asks for the status breakdown, and one total otherwise —
// the shape the Prometheus and Loki sources produce (label `code`).
func groupedRequestSource(t *testing.T) RequestMetricsSource {
	t.Helper()
	return func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		point := []MetricPoint{{Timestamp: "2026-09-14T19:56:00Z", Value: 1}}
		if req.GroupBy != groupByStatus {
			return []MetricSeries{{Labels: map[string]string{}, Unit: unitCount, Points: point}}, nil
		}
		var out []MetricSeries
		for _, code := range []string{"200", "404", "501"} {
			out = append(out, MetricSeries{Labels: map[string]string{"code": code}, Unit: unitCount, Points: point})
		}
		return out, nil
	}
}

func seriesFrom(t *testing.T, body []byte) []renderMetricSeries {
	t.Helper()
	var series []renderMetricSeries
	if err := json.Unmarshal(body, &series); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return series
}

func TestRESTAggregateByStatusCodeBreaksRequestsDown(t *testing.T) {
	svc := newService(nil, groupedRequestSource(t), sampleApp("web"), podFor("web", webInst))

	rec := serveREST(svc, "/v1/metrics/http-requests?resource=web&aggregateBy=statusCode")
	if rec.Code != http.StatusOK {
		t.Fatalf("aggregateBy=statusCode = %d %s", rec.Code, rec.Body)
	}
	series := seriesFrom(t, rec.Body.Bytes())
	if len(series) != 3 {
		t.Fatalf("series = %d, want one per status code: %+v", len(series), series)
	}
	var codes []string
	for _, s := range series {
		codes = append(codes, labelValue(s.Labels, "statusCode"))
		if labelValue(s.Labels, "code") != "" {
			t.Errorf("series %+v still carries the source's code label", s.Labels)
		}
	}
	if strings.Join(codes, ",") != "200,404,501" {
		t.Errorf("statusCode labels = %v, want 200,404,501", codes)
	}

	// Control: no aggregateBy keeps the single series.
	rec = serveREST(svc, "/v1/metrics/http-requests?resource=web")
	if got := seriesFrom(t, rec.Body.Bytes()); len(got) != 1 {
		t.Errorf("no aggregateBy = %d series, want 1", len(got))
	}
}

// w3/080: one status breakdown, four surfaces, each naming the code as
// Render's same surface does: statusCode on REST and MCP (Render's aggregateBy
// value), STATUS_CODE on GraphQL metrics and projectMetrics (the
// MetricQueryFilter value Render's dashboard reads the label by). None
// answers the sources' code; GraphQL metrics did until w3/080.
func TestStatusCodeBreakdownLabelOnEverySurface(t *testing.T) {
	svc, _ := projectFixture(t)
	ctx := context.Background()

	// codesUnder joins each series' value for field, and fails on a series that
	// carries the source's code or the other surface's spelling.
	codesUnder := func(surface, field, other string, series []renderMetricSeries) string {
		t.Helper()
		var codes []string
		for _, s := range series {
			if labelValue(s.Labels, "code") != "" || labelValue(s.Labels, other) != "" {
				t.Errorf("%s series labels = %+v, want %s and neither code nor %s", surface, s.Labels, field, other)
			}
			codes = append(codes, labelValue(s.Labels, field))
		}
		return strings.Join(codes, ",")
	}

	rest := seriesFrom(t, serveREST(svc, "/v1/metrics/http-requests?resource=web&aggregateBy=statusCode").Body.Bytes())
	if got := codesUnder("REST", "statusCode", "STATUS_CODE", rest); got != "200,500" {
		t.Errorf("REST aggregateBy=statusCode codes = %q, want 200,500", got)
	}

	res, err := mcpSession(t, svc).CallTool(ctx, &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resourceId": "web", "metricTypes": []string{"http_request_count"}, "aggregateHttpRequestCountsBy": "statusCode",
	}})
	if err != nil {
		t.Fatal(err)
	}
	if got := codesUnder("MCP", "statusCode", "STATUS_CODE", decodeGetMetrics(t, res).series()); got != "200,500" {
		t.Errorf("MCP aggregateHttpRequestCountsBy=statusCode codes = %q, want 200,500", got)
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	gql := graphql.Do(graphql.Params{Schema: schema, Context: ctx, RequestString: `{
		metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "HTTP_REQUESTS", aggregateBy: ["STATUS_CODE"]}) { labels { field value } }
		projectMetrics(query: {project: "prj-x", name: "HTTP_REQUESTS", groupBy: "STATUS_CODE"}) { series { labels { field value } } }
	}`})
	if len(gql.Errors) > 0 {
		t.Fatalf("gql: %v", gql.Errors)
	}
	var data struct {
		Metrics        []renderMetricSeries
		ProjectMetrics struct{ Series []renderMetricSeries }
	}
	if raw, err := json.Marshal(gql.Data); err != nil || json.Unmarshal(raw, &data) != nil {
		t.Fatalf("decode %+v: %v", gql.Data, err)
	}
	if got := codesUnder("GraphQL metrics", "STATUS_CODE", "statusCode", data.Metrics); got != "200,500" {
		t.Errorf("GraphQL metrics aggregateBy STATUS_CODE codes = %q, want 200,500", got)
	}
	if got := codesUnder("GraphQL projectMetrics", "STATUS_CODE", "statusCode", data.ProjectMetrics.Series); got != "200,500" {
		t.Errorf("GraphQL projectMetrics groupBy STATUS_CODE codes = %q, want 200,500", got)
	}
}

// labelFields is a series' label fields, sorted and comma-joined.
func labelFields(series []renderMetricSeries) []string {
	out := make([]string, 0, len(series))
	for _, s := range series {
		fields := make([]string, 0, len(s.Labels))
		for _, l := range s.Labels {
			fields = append(fields, l.Field)
		}
		sort.Strings(fields)
		out = append(out, strings.Join(fields, ","))
	}
	return out
}

// w3/087: Render's GraphQL MetricResultLabel.field is its MetricQueryFilter
// enum, and its dashboard finds a series' labels by it, so GraphQL names a
// series' resource, instance, host, method and status code RESOURCE, INSTANCE,
// HOST, METHOD and STATUS_CODE on metrics, datastoreMetrics and projectMetrics
// alike. The labels the enum has no value for (quantile, aggregate, direction,
// degraded_sources) are Bex extensions and keep their names. REST and MCP keep
// the lowercase names Render's REST uses.
func TestGraphQLMetricLabelNames(t *testing.T) {
	svc, _ := projectFixture(t)
	point := []MetricPoint{{Timestamp: fixedClock().Format(time.RFC3339), Value: 1}}
	statusOrTotal := svc.RequestMetrics
	requests := func(ctx context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		switch {
		case req.Metric == MetricBandwidth:
			return []MetricSeries{{Labels: map[string]string{LabelDegradedSources: "direct"}, Unit: unitBytes, Points: point}}, nil
		case req.GroupBy == groupByMethod:
			return []MetricSeries{{Labels: map[string]string{groupByMethod: "GET"}, Unit: unitCount, Points: point}}, nil
		case req.GroupBy == groupByHost:
			return []MetricSeries{{Labels: map[string]string{groupByHost: "a.example"}, Unit: unitCount, Points: point}}, nil
		}
		return statusOrTotal(ctx, req)
	}
	svc.RequestMetrics, svc.RequestLogMetrics = requests, requests

	stores := newService(nil, nil, sampleDatabase("pg", false))
	stores.DatastoreActivity = func(context.Context, DatastoreActivityRequest) ([]MetricSeries, error) {
		return []MetricSeries{
			{Labels: map[string]string{LabelResource: "pg", "instance": "pg-1", LabelDirection: directionRead}, Unit: unitBytes, Points: point},
			{Labels: map[string]string{LabelResource: "pg", "instance": "pg-1", LabelDirection: directionWrite}, Unit: unitBytes, Points: point},
		}, nil
	}

	web := `filters: [{field: "RESOURCE", values: ["web"]}]`
	for _, tc := range []struct {
		name     string
		svc      *Service
		graphql  string // a query whose one field answers [MetricSeries] or {series}
		rest     string // the same read on REST, "" where REST has none
		gql, raw string // each series' label fields on GraphQL and REST
	}{
		{"per-instance CPU", svc, `metrics(query: {` + web + `, name: "CPU"})`, "/v1/metrics/cpu?resource=web", "INSTANCE,RESOURCE", "instance,resource"},
		{"replica aggregate", svc, `metrics(query: {` + web + `, name: "CPU", aggregateAllMethod: "AVG"})`, "/v1/metrics/cpu?resource=web&aggregateAllMethod=AVG", "RESOURCE,aggregate", "aggregate,resource"},
		{"status code breakdown", svc, `metrics(query: {` + web + `, name: "HTTP_REQUESTS", aggregateBy: ["STATUS_CODE"]})`, "/v1/metrics/http-requests?resource=web&aggregateBy=statusCode", "RESOURCE,STATUS_CODE", "resource,statusCode"},
		{"host breakdown", svc, `metrics(query: {` + web + `, name: "HTTP_REQUESTS", aggregateBy: ["HOST"]})`, "/v1/metrics/http-requests?resource=web&aggregateBy=host", "HOST,RESOURCE", "host,resource"},
		{"method breakdown", svc, `metrics(query: {` + web + `, name: "HTTP_REQUESTS", aggregateBy: ["METHOD"]})`, "", "METHOD,RESOURCE", ""},
		{"percentile overlay", svc, `metrics(query: {` + web + `, name: "HTTP_LATENCY", parameters: [{quantile: 0.5}, {quantile: 0.9}]})`, "/v1/metrics/http-latency?resource=web&quantile=0.5&quantile=0.9", "RESOURCE,quantile", "quantile,resource"},
		{"degraded bandwidth", svc, `metrics(query: {` + web + `, name: "BANDWIDTH"})`, "/v1/metrics/bandwidth?resource=web", "RESOURCE,degraded_sources", "degraded_sources,resource"},
		{"datastore read and write", stores, `datastoreMetrics(query: {resource: "pg", name: "NETWORK_ACTIVITY"})`, "/v1/metrics/network-activity?resource=pg&kind=database", "INSTANCE,RESOURCE,direction", "direction,instance,resource"},
		{"project by resource", svc, `projectMetrics(query: {project: "prj-x", name: "CPU"}) { series`, "", "RESOURCE", ""},
		{"project by status code", svc, `projectMetrics(query: {project: "prj-x", name: "HTTP_REQUESTS", groupBy: "STATUS_CODE"}) { series`, "", "STATUS_CODE", ""},
		{"project percentiles", svc, `projectMetrics(query: {project: "prj-x", name: "HTTP_LATENCY", parameters: [{quantile: 0.5}, {quantile: 0.9}]}) { series`, "", "RESOURCE,quantile", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var series []renderMetricSeries
			decodeGraphQLSeries(t, tc.svc, tc.graphql, "labels { field value }", &series)
			assertEveryFields(t, "GraphQL", labelFields(series), tc.gql)

			if tc.rest == "" {
				return
			}
			rec := serveREST(tc.svc, tc.rest)
			if rec.Code != http.StatusOK {
				t.Fatalf("REST %s = %d %s", tc.rest, rec.Code, rec.Body)
			}
			assertEveryFields(t, "REST", labelFields(seriesFrom(t, rec.Body.Bytes())), tc.raw)
		})
	}

	// MCP keeps REST's names, as Render's MCP server forwards REST's series.
	for _, tc := range []struct {
		args map[string]any
		want string
	}{
		{map[string]any{"resourceId": "web", "metricTypes": []string{"cpu_usage"}}, "instance,resource instance,resource"},
		{map[string]any{"resourceId": "web", "metricTypes": []string{"http_request_count"}, "aggregateHttpRequestCountsBy": "host"}, "host,resource"},
	} {
		res, err := mcpSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: tc.args})
		if err != nil || res.IsError {
			t.Fatalf("MCP get_metrics %v: %v %s", tc.args, err, mustJSON(res.Content))
		}
		if got := strings.Join(labelFields(decodeGetMetrics(t, res).series()), " "); got != tc.want {
			t.Errorf("MCP get_metrics %v label fields = %q, want %q", tc.args, got, tc.want)
		}
	}
}

// TestGraphQLSeriesParametersRenderShape is w3/095: Render's GraphQL answers a
// series' parameters as one MetricResultParameters object, {quantile}, with
// quantile null where the series has none. Its dashboard names a latency line
// `p${(e.parameters.quantile?.toFixed(2) ?? "").slice(2)}`; renderLatencyName is
// that expression, so a list-shaped parameters fails to decode here.
func TestGraphQLSeriesParametersRenderShape(t *testing.T) {
	svc, _ := projectFixture(t)
	stores := newService(nil, nil, sampleDatabase("pg", false))
	stores.DatastoreActivity = func(context.Context, DatastoreActivityRequest) ([]MetricSeries, error) {
		point := []MetricPoint{{Timestamp: fixedClock().Format(time.RFC3339), Value: 1}}
		return []MetricSeries{{Labels: map[string]string{LabelResource: "pg", LabelDirection: directionRead}, Unit: unitBytes, Points: point}}, nil
	}

	web := `filters: [{field: "RESOURCE", values: ["web"]}]`
	for _, tc := range []struct {
		name    string
		svc     *Service
		graphql string // a query whose one field answers [MetricSeries] or {series}
		want    string // the distinct Render line names, sorted; "null" for no quantile
	}{
		{"default percentile", svc, `metrics(query: {` + web + `, name: "HTTP_LATENCY"})`, "p95"},
		{"picked percentile", svc, `metrics(query: {` + web + `, name: "HTTP_LATENCY", parameters: [{quantile: 0.9}]})`, "p90"},
		{"percentile overlay", svc, `metrics(query: {` + web + `, name: "HTTP_LATENCY", parameters: [{quantile: 0.5}, {quantile: 0.99}]})`, "p50,p99"},
		{"no percentile", svc, `metrics(query: {` + web + `, name: "CPU"})`, "null"},
		{"datastore", stores, `datastoreMetrics(query: {resource: "pg", name: "NETWORK_ACTIVITY"})`, "null"},
		{"project percentiles", svc, `projectMetrics(query: {project: "prj-x", name: "HTTP_LATENCY", parameters: [{quantile: 0.5}, {quantile: 0.9}]}) { series`, "p50,p90"},
		{"project without percentile", svc, `projectMetrics(query: {project: "prj-x", name: "CPU"}) { series`, "null"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var series []struct {
				Parameters *struct {
					Typename string   `json:"__typename"`
					Quantile *float64 `json:"quantile"`
				} `json:"parameters"`
			}
			decodeGraphQLSeries(t, tc.svc, tc.graphql, "parameters { __typename quantile }", &series)
			if len(series) == 0 {
				t.Fatalf("%s answered no series", tc.graphql)
			}
			names := map[string]bool{}
			for _, s := range series {
				if s.Parameters == nil || s.Parameters.Typename != "MetricResultParameters" {
					t.Fatalf("each series carries one MetricResultParameters object, got %s", mustJSON(series))
				}
				names[renderLatencyName(s.Parameters.Quantile)] = true
			}
			got := make([]string, 0, len(names))
			for name := range names {
				got = append(got, name)
			}
			sort.Strings(got)
			if strings.Join(got, ",") != tc.want {
				t.Errorf("Render line names = %v, want %s", got, tc.want)
			}
		})
	}
}

// decodeGraphQLSeries runs field, which answers [MetricSeries] or, ending in
// "{ series", {series}, selecting selection on each series, and decodes the
// series into out.
func decodeGraphQLSeries(t *testing.T, svc *Service, field, selection string, out any) {
	t.Helper()
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	project := strings.HasSuffix(field, "{ series")
	query := "{ " + field + " { " + selection + " } }"
	if project {
		query += " }"
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("gql %s: %v", query, res.Errors)
	}
	var data map[string]json.RawMessage
	if err := json.Unmarshal([]byte(mustJSON(res.Data)), &data); err != nil {
		t.Fatalf("decode %+v: %v", res.Data, err)
	}
	for _, raw := range data {
		if project {
			var answer struct{ Series json.RawMessage }
			if err := json.Unmarshal(raw, &answer); err != nil {
				t.Fatalf("decode %s: %v", raw, err)
			}
			raw = answer.Series
		}
		if err := json.Unmarshal(raw, out); err != nil {
			t.Fatalf("decode the series of %s: %v in %s", query, err, raw)
		}
	}
}

// renderLatencyName is Render's `p${(quantile?.toFixed(2) ?? "").slice(2)}`,
// "null" where the series has no quantile.
func renderLatencyName(quantile *float64) string {
	if quantile == nil {
		return "null"
	}
	return "p" + strconv.FormatFloat(*quantile, 'f', 2, 64)[2:]
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

// assertEveryFields fails unless there is a series and each one's label fields are want.
func assertEveryFields(t *testing.T, surface string, got []string, want string) {
	t.Helper()
	if len(got) == 0 {
		t.Fatalf("%s answered no series, want each labeled %s", surface, want)
	}
	for _, fields := range got {
		if fields != want {
			t.Errorf("%s series label fields = %v, want each %s", surface, got, want)
			return
		}
	}
}

// The Traefik counter has no host, so a host breakdown never falls back to it:
// with the request-log store unwired it is the host filter's 503.
func TestRESTAggregateByHostNeedsTheRequestLogStore(t *testing.T) {
	svc := newService(nil, groupedRequestSource(t), sampleApp("web"), podFor("web", webInst))

	rec := serveREST(svc, "/v1/metrics/http-requests?resource=web&aggregateBy=host")
	if rec.Code != http.StatusServiceUnavailable || !strings.Contains(rec.Body.String(), "LOG_STORE_UNAVAILABLE") {
		t.Fatalf("aggregateBy=host without Loki = %d %s, want 503 LOG_STORE_UNAVAILABLE", rec.Code, rec.Body)
	}
}

func TestRESTCPUAggregationOnTheSnapshotSourceIsRefused(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))

	if rec := serveREST(svc, "/v1/metrics/cpu?resource=web&aggregationMethod=AVG"); rec.Code != http.StatusOK {
		t.Errorf("aggregationMethod=AVG = %d %s, want 200", rec.Code, rec.Body)
	}
	if rec := serveREST(svc, "/v1/metrics/cpu?resource=web&aggregationMethod=MAX"); rec.Code != http.StatusBadRequest ||
		!strings.Contains(rec.Body.String(), "metrics-server snapshot") {
		t.Errorf("aggregationMethod=MAX on the snapshot source = %d %s, want a coded 400", rec.Code, rec.Body)
	}
}

func TestRESTLimitPathsAreServed(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))

	for path, metric := range map[string]string{"cpu-limit": MetricCPULimit, "memory-limit": MetricMemoryLimit} {
		rec := serveREST(svc, "/v1/metrics/"+path+"?resource=web")
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d %s", path, rec.Code, rec.Body)
		}
		want, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: metric})
		if err != nil {
			t.Fatal(err)
		}
		got := seriesFrom(t, rec.Body.Bytes())
		if len(got) != len(want) || len(got) == 0 || got[0].Values[0].Value != want[0].Points[0].Value {
			t.Errorf("%s = %+v, want the Metrics verb's %+v", path, got, want)
		}
	}
}

func TestRESTFilterPathsUseRenderShapes(t *testing.T) {
	svc := newService(nil, nil, sampleApp("web"), podFor("web", webInst))
	svc.MetricsFilterValuesSource = func(_ context.Context, req MetricsFilterValuesRequest) ([]string, error) {
		if label := req.Label; label != "code" {
			t.Errorf("filter values label = %q, want code", label)
		}
		return []string{"200", "404"}, nil
	}

	var app []renderFilterValues
	rec := serveREST(svc, "/v1/metrics/filters/application?resource=web")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &app) != nil ||
		len(app) != 1 || app[0].Filter != "instance" || len(app[0].Values) != 1 {
		t.Fatalf("filters/application = %d %s, want [{filter: instance, values: [one instance]}]", rec.Code, rec.Body)
	}

	var httpFilters []renderFilterValues
	rec = serveREST(svc, "/v1/metrics/filters/http?resource=web")
	if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &httpFilters) != nil ||
		len(httpFilters) != 2 || httpFilters[0].Filter != "statusCode" ||
		strings.Join(httpFilters[0].Values, ",") != "200,404" || httpFilters[1].Filter != "host" {
		t.Fatalf("filters/http = %d %s, want statusCode [200 404] and host", rec.Code, rec.Body)
	}
	if rec := serveREST(svc, "/v1/metrics/filters/http?resource=web&statusCode=404"); rec.Code != http.StatusBadRequest {
		t.Errorf("narrowed filters/http = %d, want a coded 400 rather than an ignored narrowing", rec.Code)
	}

	rec = serveREST(svc, "/v1/metrics/filters/path?resource=web")
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != "[]" {
		t.Errorf("filters/path = %d %s, want []", rec.Code, rec.Body)
	}

	// Every filter read authorizes its resource: an unknown one is 404, never [].
	for _, path := range []string{"application", "http", "path"} {
		if rec := serveREST(svc, "/v1/metrics/filters/"+path+"?resource=nope"); rec.Code != http.StatusNotFound {
			t.Errorf("filters/%s unknown resource = %d, want 404", path, rec.Code)
		}
	}
}

func TestRESTDatastorePathsInferTheKindFromTheID(t *testing.T) {
	for resource, want := range map[string]string{
		"dpg-c185th5c2rvvnhbfiltg": DatastoreDatabase,
		"red-c185th5c2rvvnhbfiltg": DatastoreKeyValue,
		"srv-c185th5c2rvvnhbfiltg": DatastoreService,
		"legacy-db-name":           DatastoreDatabase,
	} {
		if got := datastoreKindFor(resource); got != want {
			t.Errorf("datastoreKindFor(%q) = %q, want %q", resource, got, want)
		}
	}

	svc := newService(nil, nil)
	rec := serveREST(svc, "/v1/metrics/active-connections?resource=srv-c185th5c2rvvnhbfiltg")
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "Postgres (dpg-) or Key Value (red-)") {
		t.Errorf("active-connections on a service = %d %s, want a coded 400", rec.Code, rec.Body)
	}
}

// TestRenderAppMetricPathsAnswerForDatastores is w8/061 and w1/m180: Render
// scopes its cpu/memory/... paths to Postgres and Key Value too. cpu, memory
// and their limits answer from the datastore's own container (a Key Value's
// memory is its kv_memory series); every other datastore pair is a coded
// refusal naming the metric, and only a missing id is 404.
func TestRenderAppMetricPathsAnswerForDatastores(t *testing.T) {
	const kvID, pgID = "red-c185th5c2rvvnhbfiltg", "dpg-c185th5c2rvvnhbfiltg"
	svc := newService(nil, nil, sampleKeyValue(kvID), sampleDatabase(pgID, false))
	var asked KeyValueStatsRequest
	svc.KeyValueStats = func(_ context.Context, req KeyValueStatsRequest) ([]MetricSeries, error) {
		asked = req
		return []MetricSeries{{Labels: map[string]string{}, Unit: unitBytes, Points: []MetricPoint{{Value: 4096}}}}, nil
	}
	var resources []DatastoreResourceRequest
	svc.DatastoreResource = recordingResourceSource(&resources)

	rec := serveREST(svc, "/v1/metrics/memory?resource="+kvID)
	got := seriesFrom(t, rec.Body.Bytes())
	if rec.Code != http.StatusOK || len(got) != 1 || got[0].Unit != unitBytes || got[0].Values[0].Value != 4096 || asked.Dimension != "memory" {
		t.Fatalf("Key Value memory = %d %s (asked %+v), want the kv_memory series", rec.Code, rec.Body, asked)
	}

	served := map[string]bool{"cpu": true, "memory": true, "cpu-limit": true, "memory-limit": true}
	for seg := range metricPaths {
		for _, id := range []string{kvID, pgID} {
			if seg == "memory" && id == kvID {
				continue
			}
			rec := serveREST(svc, "/v1/metrics/"+seg+"?resource="+id)
			if served[seg] {
				if rec.Code != http.StatusOK || len(seriesFrom(t, rec.Body.Bytes())) != 1 {
					t.Errorf("%s on %s = %d %s, want the datastore container's series", seg, id, rec.Code, rec.Body)
				}
				continue
			}
			if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "is not served for") || !strings.Contains(rec.Body.String(), id) {
				t.Errorf("%s on %s = %d %s, want a coded 400 naming the pair", seg, id, rec.Code, rec.Body)
			}
		}
	}

	for _, id := range []string{"red-zzzzzzzzzzzzzzzzzzzz", "dpg-zzzzzzzzzzzzzzzzzzzz"} {
		if rec := serveREST(svc, "/v1/metrics/memory?resource="+id); rec.Code != http.StatusNotFound {
			t.Errorf("memory on nonexistent %s = %d %s, want 404", id, rec.Code, rec.Body)
		}
		if rec := serveREST(svc, "/v1/metrics/cpu?resource="+id); rec.Code != http.StatusNotFound {
			t.Errorf("cpu on nonexistent %s = %d %s, want 404", id, rec.Code, rec.Body)
		}
	}
}

func TestMCPGetMetricsAnswersForDatastores(t *testing.T) {
	const kvID = "red-c185th5c2rvvnhbfiltg"
	svc := newService(nil, nil, sampleKeyValue(kvID))
	svc.KeyValueStats = func(context.Context, KeyValueStatsRequest) ([]MetricSeries, error) {
		return []MetricSeries{{Labels: map[string]string{}, Unit: unitBytes, Points: []MetricPoint{{Value: 4096}}}}, nil
	}
	var resources []DatastoreResourceRequest
	svc.DatastoreResource = recordingResourceSource(&resources)
	cs := mcpSession(t, svc)
	call := func(metricType string) *mcp.CallToolResult {
		t.Helper()
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "get_metrics",
			Arguments: map[string]any{"resourceId": kvID, "metricTypes": []string{metricType}},
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	for _, metricType := range []string{"memory_usage", "cpu_usage", "cpu_limit", "memory_limit"} {
		if result := call(metricType); result.IsError {
			t.Fatalf("%s on a Key Value: %+v", metricType, result.Content)
		}
	}
	if len(resources) != 3 || resources[0].Container != keyValueContainer {
		t.Errorf("container reads = %+v, want cpu and both limits from the valkey container", resources)
	}
	result := call("instance_count")
	raw, _ := json.Marshal(result.Content)
	if !result.IsError || !strings.Contains(string(raw), "is not served for Key Value") {
		t.Errorf("instance_count on a Key Value = %s, want a refusal naming the pair", raw)
	}
}

func TestMCPGetMetricsAcceptsRenderMetricTypes(t *testing.T) {
	svc := newService(staticResourceMetrics(map[string]PodResourceUsage{
		webInst: {CPUCores: 0.5, MemoryBytes: 512 << 20},
	}), nil, sampleApp("web"), podWithLimits(webInst))
	cs := mcpSession(t, svc)

	call := func(metricTypes ...string) *mcp.CallToolResult {
		t.Helper()
		result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
			Name:      "get_metrics",
			Arguments: map[string]any{"resourceId": "web", "metricTypes": metricTypes},
		})
		if err != nil {
			t.Fatal(err)
		}
		return result
	}
	for render, bex := range map[string]string{"cpu_usage": MetricCPU, "memory_usage": MetricMemory, "cpu_limit": MetricCPULimit} {
		renderResult, bexResult := call(render), call(bex)
		if renderResult.IsError || bexResult.IsError {
			t.Fatalf("%s/%s: isError render=%v bex=%v", render, bex, renderResult.IsError, bexResult.IsError)
		}
		gotResp := decodeGetMetrics(t, renderResult)
		got, want := gotResp.series(), decodeGetMetrics(t, bexResult).series()
		if len(got) == 0 || len(got) != len(want) || got[0].Values[0].Value != want[0].Values[0].Value {
			t.Errorf("%s = %+v, want the same series as %s %+v", render, got, bex, want)
		}
		if gotResp.Metrics[0].Type != render {
			t.Errorf("%s answered as type %q, want the requested name", render, gotResp.Metrics[0].Type)
		}
	}

	refused, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "get_metrics",
		Arguments: map[string]any{"resourceId": "srv-c185th5c2rvvnhbfiltg", "metricTypes": []string{"active_connections"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if text, _ := json.Marshal(refused.Content); !refused.IsError || !strings.Contains(string(text), "Postgres (dpg-) or Key Value (red-)") {
		t.Errorf("active_connections on a service = %s, want the coded refusal naming the datastore kinds", text)
	}
	if result := call("not_a_metric"); !result.IsError {
		t.Error("an unknown metric type must still error")
	}
}

// renderMetricsTypes pins Render's MCP get_metrics enum
// (render-oss/render-mcp-server pkg/metrics/tools.go): every name maps to a
// metric id the Metrics verb or the datastore route knows, so dropping one is a
// test failure rather than a silent regression.
func TestEveryRenderMCPMetricTypeIsKnown(t *testing.T) {
	known := map[string]bool{
		MetricCPU: true, MetricMemory: true, MetricInstanceCount: true, MetricHTTPRequests: true,
		MetricHTTPLatency: true, MetricBandwidth: true, MetricCPULimit: true, MetricMemoryLimit: true,
		MetricCPUTarget: true, MetricMemoryTarget: true,
	}
	for _, name := range []string{
		"cpu_usage", "memory_usage", "http_request_count", "active_connections", "instance_count",
		"http_latency", "cpu_limit", "cpu_target", "memory_limit", "memory_target", "bandwidth_usage",
	} {
		if name == renderActiveConnections {
			continue
		}
		if !known[metricIDFor(name)] {
			t.Errorf("Render metric type %q maps to %q, which the Metrics verb does not serve", name, metricIDFor(name))
		}
	}
}
