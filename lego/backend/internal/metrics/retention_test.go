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
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strconv"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w7/m427: no chart read returns a point older than the workspace plan keeps
// metrics (Render: Hobby 7 days, Pro 14, Scale and Enterprise 30). The floor
// sits under both core verbs, so REST, GraphQL and MCP apply the same one.

const (
	retentionDay  = 24 * time.Hour
	retentionStep = 6 * time.Hour // the dashboard's 30-day preset step
	retentionTea  = "tea-a"
)

// retentionNow is the clock every retention test reads.
var retentionNow = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

// planReader answers one plan for every workspace and counts its reads.
type planReader struct {
	plan  string
	err   error
	reads int
}

func (p *planReader) WorkspacePlan(context.Context, string) (string, error) {
	p.reads++
	return p.plan, p.err
}

// gridSource answers a ranged resource read as Prometheus does: one point per
// step from the request's start through its end. It records every request.
func gridSource(got *[]ResourceMetricsRangeRequest) ResourceMetricsRangeSource {
	return func(_ context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		*got = append(*got, req)
		var points []MetricPoint
		for ts := req.Start; !ts.After(req.End); ts = ts.Add(req.Resolution) {
			points = append(points, MetricPoint{Timestamp: ts.UTC().Format(time.RFC3339), Value: 1})
		}
		return []MetricSeries{{Labels: map[string]string{"instance": webInst}, Points: points}}, nil
	}
}

// retentionService is a service in workspace tea-a on plan, with a ranged cpu
// source that records what it was asked. plan "" wires no plan reader.
func retentionService(plan string, objs ...client.Object) (*Service, *planReader, *[]ResourceMetricsRangeRequest) {
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: retentionTea}
	got := &[]ResourceMetricsRangeRequest{}
	svc := rangeService(gridSource(got), nil, append([]client.Object{app, podFor("web", webInst)}, objs...)...)
	svc.Clock = func() time.Time { return retentionNow }
	reader := &planReader{plan: plan}
	if plan != "" {
		svc.Plans = reader
	}
	return svc, reader, got
}

func rfc(t time.Time) string { return t.UTC().Format(time.RFC3339) }

// firstAndCount parses a surface's point timestamps: the earliest and how many.
func firstAndCount(t *testing.T, stamps []string) (time.Time, int) {
	t.Helper()
	if len(stamps) == 0 {
		t.Fatal("no points")
	}
	first, err := time.Parse(time.RFC3339, stamps[0])
	if err != nil {
		t.Fatalf("timestamp %q: %v", stamps[0], err)
	}
	return first, len(stamps)
}

// restStamps reads one REST metrics body's point timestamps.
func restStamps(t *testing.T, svc *Service, path string) []string {
	t.Helper()
	rec := serveREST(svc, path)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", path, rec.Code, rec.Body.String())
	}
	var body []renderMetricSeries
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode %s: %v", rec.Body.String(), err)
	}
	var out []string
	for _, series := range body {
		for _, v := range series.Values {
			out = append(out, v.Timestamp)
		}
	}
	return out
}

// gqlStamps reads one GraphQL metrics field's point timestamps.
func gqlStamps(t *testing.T, svc *Service, field, query string) []string {
	t.Helper()
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: query})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	var out []string
	for _, series := range res.Data.(map[string]any)[field].([]any) {
		for _, v := range series.(map[string]any)["values"].([]any) {
			out = append(out, v.(map[string]any)["time"].(string))
		}
	}
	return out
}

func TestMetricsReachBackOnlyAsFarAsThePlan(t *testing.T) {
	// The request starts an hour off the 30-day mark, so no plan's floor falls
	// on its grid: a clamped read must begin at the next grid point, not at
	// the floor itself.
	start := retentionNow.Add(-30*retentionDay + time.Hour)
	window := url.Values{
		"resource": {"web"}, "startTime": {rfc(start)}, "endTime": {rfc(retentionNow)},
		"resolutionSeconds": {strconv.Itoa(int(retentionStep / time.Second))},
	}.Encode()
	gqlWindow := `start: "` + rfc(start) + `", end: "` + rfc(retentionNow) + `", resolution: ` + strconv.Itoa(int(retentionStep/time.Second))

	surfaces := map[string]func(*testing.T, *Service) []string{
		"REST": func(t *testing.T, svc *Service) []string {
			return restStamps(t, svc, "/v1/metrics/cpu?"+window)
		},
		"GraphQL": func(t *testing.T, svc *Service) []string {
			return gqlStamps(t, svc, "metrics", `{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: "CPU", `+gqlWindow+`}) { values { time } } }`)
		},
		"MCP": func(t *testing.T, svc *Service) []string {
			res, err := mcpSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
				"resourceId": "web", "metricTypes": []string{"cpu"},
				"startTime": rfc(start), "endTime": rfc(retentionNow), "resolution": int(retentionStep / time.Second),
			}})
			if err != nil {
				t.Fatalf("get_metrics: %v", err)
			}
			var out []string
			for _, series := range decodeGetMetrics(t, res).series() {
				for _, v := range series.Values {
					out = append(out, v.Timestamp)
				}
			}
			return out
		},
	}

	for plan, days := range map[string]int{"hobby": 7, "pro": 14, "scale": 30, "enterprise": 30} {
		floor := retentionNow.Add(-time.Duration(days) * retentionDay)
		// The first point of the request's own grid at or after the floor.
		want := start
		for want.Before(floor) {
			want = want.Add(retentionStep)
		}
		for name, read := range surfaces {
			t.Run(plan+"/"+name, func(t *testing.T) {
				svc, _, got := retentionService(plan)
				first, count := firstAndCount(t, read(t, svc))
				if !first.Equal(want) {
					t.Fatalf("first point = %s, want %s (floor %s)", rfc(first), rfc(want), rfc(floor))
				}
				if first.Before(floor) {
					t.Fatalf("first point %s is older than the %d-day floor %s", rfc(first), days, rfc(floor))
				}
				if wantCount := int(retentionNow.Sub(want)/retentionStep) + 1; count != wantCount {
					t.Fatalf("points = %d, want %d: the read must keep everything inside the window", count, wantCount)
				}
				// The store was asked for the window only, not scanned and trimmed.
				if len(*got) != 1 || !(*got)[0].Start.Equal(want) || !(*got)[0].End.Equal(retentionNow) {
					t.Fatalf("source read = %+v, want one read of [%s, %s]", *got, rfc(want), rfc(retentionNow))
				}
			})
		}
	}
}

// A clamped read returns the same points an unclamped one does at the same
// timestamps: the grid is the request's, whatever the plan.
func TestRetentionKeepsTheRequestedGrid(t *testing.T) {
	q := MetricQuery{App: "web", Metric: MetricCPU, Start: retentionNow.Add(-20*retentionDay + 37*time.Minute), End: retentionNow, Resolution: 5 * time.Hour}
	read := func(plan string) []MetricPoint {
		svc, _, _ := retentionService(plan)
		series, err := svc.Metrics(context.Background(), q)
		if err != nil || len(series) != 1 {
			t.Fatalf("%s: series %d, err %v", plan, len(series), err)
		}
		return series[0].Points
	}
	whole := read("scale")
	floor := rfc(retentionNow.Add(-7 * retentionDay))
	var want []MetricPoint
	for _, p := range whole {
		if p.Timestamp >= floor { // same-zone RFC 3339 sorts as time does
			want = append(want, p)
		}
	}
	got := read("hobby")
	if len(want) == 0 || len(want) == len(whole) {
		t.Fatalf("the fixture must straddle the floor: %d of %d points kept", len(want), len(whole))
	}
	if len(got) != len(want) || got[0] != want[0] || got[len(got)-1] != want[len(want)-1] {
		t.Fatalf("hobby = %d points from %s; want scale's %d from %s", len(got), got[0].Timestamp, len(want), want[0].Timestamp)
	}
}

func TestRangeBeforeTheFloorAnswersEmpty(t *testing.T) {
	svc, _, got := retentionService("hobby")
	from, to := retentionNow.Add(-14*retentionDay), retentionNow.Add(-8*retentionDay)

	series, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Start: from, End: to, Resolution: time.Hour})
	if err != nil || series == nil || len(series) != 0 {
		t.Fatalf("Metrics = %v, %v; want an empty, non-nil result and no error", series, err)
	}
	rec := serveREST(svc, "/v1/metrics/cpu?"+url.Values{"resource": {"web"}, "startTime": {rfc(from)}, "endTime": {rfc(to)}, "resolutionSeconds": {"3600"}}.Encode())
	if rec.Code != http.StatusOK || rec.Body.String() != "[]\n" {
		t.Fatalf("REST = %d %q, want 200 []", rec.Code, rec.Body.String())
	}
	if len(*got) != 0 {
		t.Fatalf("a range the plan no longer keeps reached the store: %+v", *got)
	}
	// The same range on Scale is inside its window and is read whole.
	scale, _, scaleGot := retentionService("scale")
	if _, err := scale.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Start: from, End: to, Resolution: time.Hour}); err != nil {
		t.Fatal(err)
	}
	if len(*scaleGot) != 1 || !(*scaleGot)[0].Start.Equal(from) {
		t.Fatalf("scale read = %+v, want the requested start %s", *scaleGot, rfc(from))
	}
}

func TestRetentionReadsNoPlanItDoesNotNeed(t *testing.T) {
	q := MetricQuery{App: "web", Metric: MetricCPU, End: retentionNow, Resolution: time.Hour}

	// Inside the shortest window no plan can clamp, so none is read.
	svc, reader, got := retentionService("hobby")
	q.Start = retentionNow.Add(-7 * retentionDay)
	if _, err := svc.Metrics(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if reader.reads != 0 || !(*got)[0].Start.Equal(q.Start) {
		t.Fatalf("a 7-day read: %d plan reads, start %s; want none and the requested start", reader.reads, rfc((*got)[0].Start))
	}

	// Without a store there is no plan and no floor.
	q.Start = retentionNow.Add(-29 * retentionDay)
	off, _, offGot := retentionService("")
	if _, err := off.Metrics(context.Background(), q); err != nil {
		t.Fatal(err)
	}
	if !(*offGot)[0].Start.Equal(q.Start) {
		t.Fatalf("store off: start %s, want the requested %s", rfc((*offGot)[0].Start), rfc(q.Start))
	}

	// An unreadable plan fails the read; it is never read as "no floor".
	broken, brokenReader, brokenGot := retentionService("hobby")
	brokenReader.err = errors.New("db down")
	if _, err := broken.Metrics(context.Background(), q); err == nil || len(*brokenGot) != 0 {
		t.Fatalf("unreadable plan: err %v, store reads %d; want an error and no read", err, len(*brokenGot))
	}
	if rec := serveREST(broken, "/v1/metrics/cpu?"+url.Values{"resource": {"web"}, "startTime": {rfc(q.Start)}, "endTime": {rfc(q.End)}, "resolutionSeconds": {"3600"}}.Encode()); rec.Code != http.StatusInternalServerError {
		t.Fatalf("unreadable plan over REST = %d, want 500", rec.Code)
	}
}

// Datastore metrics take the same floor, from the datastore's own workspace,
// on bex's datastore routes and on Render's App-metric paths for a datastore id.
func TestDatastoreMetricsTakeThePlanFloor(t *testing.T) {
	const pgID, kvID = "dpg-d0000000000000000001", "red-d0000000000000000001"
	db := sampleDatabase(pgID, false)
	db.Labels = map[string]string{core.LabelTenant: retentionTea}
	kv := sampleKeyValue(kvID)
	kv.Labels = map[string]string{core.LabelTenant: retentionTea}

	start := retentionNow.Add(-14 * retentionDay)
	floor := retentionNow.Add(-7 * retentionDay)
	window := url.Values{"startTime": {rfc(start)}, "endTime": {rfc(retentionNow)}, "resolutionSeconds": {"3600"}}
	gqlWindow := `start: "` + rfc(start) + `", end: "` + rfc(retentionNow) + `", resolution: 3600`

	var starts []time.Time
	point := []MetricSeries{{Labels: map[string]string{"instance": pgID + "-1"}, Points: []MetricPoint{{Timestamp: rfc(retentionNow), Value: 3}}}}
	svc, _, _ := retentionService("hobby", db, kv)
	svc.DBConnections = func(_ context.Context, req DBConnectionsRequest) ([]MetricSeries, error) {
		starts = append(starts, req.Start)
		return point, nil
	}
	svc.KeyValueStats = func(_ context.Context, req KeyValueStatsRequest) ([]MetricSeries, error) {
		starts = append(starts, req.Start)
		return point, nil
	}
	svc.DatastoreResource = func(_ context.Context, req DatastoreResourceRequest) ([]MetricSeries, error) {
		starts = append(starts, req.Start)
		return point, nil
	}
	svc.DatastoreActivity = func(_ context.Context, req DatastoreActivityRequest) ([]MetricSeries, error) {
		starts = append(starts, req.Start)
		return point, nil
	}
	svc.LockWaits = func(_ context.Context, req LockWaitsRequest) ([]MetricSeries, error) {
		starts = append(starts, req.Start)
		return point, nil
	}

	reads := map[string]func(){
		// Render's database-activity graphs (w7/m430), from Prometheus and
		// from the log store.
		"REST disk-activity": func() {
			restStamps(t, svc, "/v1/metrics/disk-activity?resource="+kvID+"&"+window.Encode())
		},
		"REST lock-delayed-queries": func() {
			restStamps(t, svc, "/v1/metrics/lock-delayed-queries?resource="+pgID+"&"+window.Encode())
		},
		"REST db-connections": func() {
			restStamps(t, svc, "/v1/metrics/db-connections?resource="+pgID+"&"+window.Encode())
		},
		"REST kv-connections": func() {
			restStamps(t, svc, "/v1/metrics/kv-connections?resource="+kvID+"&"+window.Encode())
		},
		"REST Render cpu path": func() {
			restStamps(t, svc, "/v1/metrics/cpu?resource="+pgID+"&"+window.Encode())
		},
		"REST Render active-connections": func() {
			restStamps(t, svc, "/v1/metrics/active-connections?resource="+kvID+"&"+window.Encode())
		},
		"GraphQL datastoreMetrics": func() {
			gqlStamps(t, svc, "datastoreMetrics", `{ datastoreMetrics(query: {kind: "DATABASE", resource: "`+pgID+`", name: "DB_CONNECTIONS", `+gqlWindow+`}) { values { time } } }`)
		},
		"MCP get_datastore_metrics": func() {
			res, err := mcpSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_datastore_metrics", Arguments: map[string]any{
				"resource": pgID, "kind": DatastoreDatabase, "metricTypes": []string{MetricDBConnections},
				"startTime": rfc(start), "endTime": rfc(retentionNow), "resolution": 3600,
			}})
			if err != nil || res.IsError {
				t.Fatalf("get_datastore_metrics: err %v, result %+v", err, res)
			}
		},
		"MCP get_metrics on a datastore": func() {
			res, err := mcpSession(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
				"resourceId": kvID, "metricTypes": []string{"active_connections"},
				"startTime": rfc(start), "endTime": rfc(retentionNow), "resolution": 3600,
			}})
			if err != nil || res.IsError {
				t.Fatalf("get_metrics: err %v, result %+v", err, res)
			}
		},
	}
	for name, read := range reads {
		starts = nil
		read()
		if len(starts) != 1 || !starts[0].Equal(floor) {
			t.Errorf("%s: store read from %v, want one read from the 7-day floor %s", name, starts, rfc(floor))
		}
	}

	// A range the plan no longer keeps is empty, and the store is not asked.
	starts = nil
	series, err := svc.DatastoreMetrics(context.Background(), DatastoreMetricQuery{
		Kind: DatastoreDatabase, Resource: pgID, Metric: MetricDBConnections,
		Start: start, End: retentionNow.Add(-8 * retentionDay), Resolution: time.Hour,
	})
	if err != nil || len(series) != 0 || len(starts) != 0 {
		t.Fatalf("before the floor: %v, %v, %d store reads; want empty, nil, none", series, err, len(starts))
	}
}

// Status-code discovery has no range of its own, so it is always bounded at the
// plan's floor: the dropdown offers no value the charts cannot show.
func TestFilterDiscoveryIsBoundedAtThePlanFloor(t *testing.T) {
	discover := func(plan string) time.Time {
		svc, _, _ := retentionService(plan, ingressFor("web", "web", "web.onbex.co"))
		var since time.Time
		svc.MetricsFilterValuesSource = func(_ context.Context, req MetricsFilterValuesRequest) ([]string, error) {
			since = req.Since
			return []string{"200"}, nil
		}
		if _, err := svc.MetricsFilters(context.Background(), MetricsFiltersQuery{App: "web", OutputFilters: []string{filterFieldStatusCode}}); err != nil {
			t.Fatalf("%s: %v", plan, err)
		}
		return since
	}
	if got, want := discover("pro"), retentionNow.Add(-14*retentionDay); !got.Equal(want) {
		t.Fatalf("pro discovery since %s, want %s", rfc(got), rfc(want))
	}
	if got := discover(""); !got.IsZero() {
		t.Fatalf("store off: discovery since %s, want unbounded", rfc(got))
	}

	// The Prometheus source turns the bound into the label API's start.
	var query url.Values
	prom := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query = r.URL.Query()
		_, _ = w.Write([]byte(`{"status":"success","data":["200"]}`))
	}))
	defer prom.Close()
	req := MetricsFilterValuesRequest{Namespace: "default", App: "web", Routers: []string{"default-web-web-onbex-co@kubernetes"}, Label: "code"}
	source := NewPrometheusFilterValuesSource(prom.URL, prom.Client())
	if _, err := source(context.Background(), req); err != nil || query.Has("start") {
		t.Fatalf("unbounded discovery: err %v, start %q; want no start", err, query.Get("start"))
	}
	req.Since = retentionNow.Add(-7 * retentionDay)
	if _, err := source(context.Background(), req); err != nil || query.Get("start") != strconv.FormatInt(req.Since.Unix(), 10) {
		t.Fatalf("bounded discovery: err %v, start %q; want %d", err, query.Get("start"), req.Since.Unix())
	}
}

// The month-to-date bandwidth figure is the included-bandwidth accounting, a
// billing read: it covers the whole month on every plan.
func TestMonthToDateBandwidthIsNotFloored(t *testing.T) {
	svc, reader, _ := retentionService("hobby", ingressFor("web", "web", "web.onbex.co"))
	var since time.Time
	svc.MonthToDateBandwidthSource = func(_ context.Context, _ string, _ []string, _ bool, from, _ time.Time) (BandwidthBytes, []string, error) {
		since = from
		return BandwidthBytes{HTTP: 1 << 20}, nil, nil
	}
	if _, err := svc.MonthToDateBandwidth(context.Background(), "web"); err != nil {
		t.Fatal(err)
	}
	// 2026-11-16 is fifteen days into the month, past Hobby's seven.
	if want := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC); !since.Equal(want) || reader.reads != 0 {
		t.Fatalf("month-to-date read from %s with %d plan reads; want %s and none", rfc(since), reader.reads, rfc(want))
	}
}

// Render's hourly traffic-source breakdown is a chart read: it takes the floor.
func TestBandwidthSourcesTakeThePlanFloor(t *testing.T) {
	svc, _, _ := retentionService("hobby", ingressFor("web", "web", "web.onbex.co"))
	var starts []time.Time
	svc.RequestMetrics = func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		starts = append(starts, req.Start)
		return []MetricSeries{{Unit: unitBytes, Points: []MetricPoint{{Timestamp: rfc(req.Start), Value: 1 << 20}}}}, nil
	}
	start := retentionNow.Add(-14 * retentionDay)
	rec := serveREST(svc, "/v1/metrics/bandwidth-sources?"+url.Values{"resource": {"web"}, "startTime": {rfc(start)}, "endTime": {rfc(retentionNow)}}.Encode())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	floor := retentionNow.Add(-7 * retentionDay)
	if len(starts) != len(bandwidthTrafficSources) {
		t.Fatalf("source reads = %d, want one per traffic source", len(starts))
	}
	for _, got := range starts {
		if !got.Equal(floor) {
			t.Fatalf("a traffic source was read from %s, want the floor %s", rfc(got), rfc(floor))
		}
	}
}

// metricsRetention tells the dashboard the window to bound its picker by: the
// plan, its days and the cheapest plan that keeps more.
func TestGraphQLMetricsRetention(t *testing.T) {
	const pgID = "dpg-d0000000000000000001"
	db := sampleDatabase(pgID, false)
	db.Labels = map[string]string{core.LabelTenant: retentionTea}
	query := func(svc *Service, resource string) any {
		schema, err := gqlSchema(svc)
		if err != nil {
			t.Fatalf("schema: %v", err)
		}
		res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
			RequestString: `{ metricsRetention(resource: "` + resource + `") { plan days upgrades { plan days } } }`})
		if len(res.Errors) > 0 {
			t.Fatalf("gql: %v", res.Errors)
		}
		return res.Data.(map[string]any)["metricsRetention"]
	}
	pro, scale := map[string]any{"plan": "pro", "days": 14}, map[string]any{"plan": "scale", "days": 30}
	for _, tc := range []struct {
		plan, resource string
		days           int
		upgrades       []any
	}{
		{"hobby", "web", 7, []any{pro, scale}},
		{"pro", "web", 14, []any{scale}},
		{"scale", "web", 30, []any{}},
		{"enterprise", "web", 30, []any{}},
		{"hobby", pgID, 7, []any{pro, scale}},
	} {
		svc, _, _ := retentionService(tc.plan, db)
		got, _ := query(svc, tc.resource).(map[string]any)
		if got["plan"] != tc.plan || got["days"] != tc.days || !reflect.DeepEqual(got["upgrades"], tc.upgrades) {
			t.Errorf("%s on %s: metricsRetention = %+v, want %d days and upgrades %v", tc.resource, tc.plan, got, tc.days, tc.upgrades)
		}
	}
	// No plan applies without a store: null, and the picker stays unbounded.
	off, _, _ := retentionService("")
	if got := query(off, "web"); got != nil {
		t.Fatalf("store off: metricsRetention = %+v, want null", got)
	}
	// An unknown resource is not found, as on every by-id read.
	svc, _, _ := retentionService("hobby")
	if _, err := svc.MetricsRetention(context.Background(), "missing"); !errors.Is(err, core.ErrNotFound) {
		t.Fatalf("unknown resource: %v, want not found", err)
	}
}
