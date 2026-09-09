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
	"errors"
	"testing"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
)

func TestAggregateReplicasAtTimestampsMinMaxAvg(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	b := ids.ServiceInstanceID(app, "pod-b")
	ts := "2026-09-08T12:00:00Z"
	series := []MetricSeries{
		{Labels: map[string]string{"resource": app, "instance": a}, Unit: "cores", Points: []MetricPoint{{Timestamp: ts, Value: 10}}},
		{Labels: map[string]string{"resource": app, "instance": b}, Unit: "cores", Points: []MetricPoint{{Timestamp: ts, Value: 30}}},
	}
	for method, want := range map[string]float64{
		replicaAggregateMin: 10,
		replicaAggregateMax: 30,
		replicaAggregateAvg: 20,
	} {
		out := aggregateReplicasAtTimestamps(app, method, series)
		if len(out) != 1 || len(out[0].Points) != 1 {
			t.Fatalf("%s: %+v", method, out)
		}
		if out[0].Points[0].Value != want {
			t.Fatalf("%s = %v, want %v", method, out[0].Points[0].Value, want)
		}
		if out[0].Labels["aggregate"] != method {
			t.Fatalf("%s label = %q", method, out[0].Labels["aggregate"])
		}
	}
}

func TestAggregateReplicasDoesNotFillGaps(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	b := ids.ServiceInstanceID(app, "pod-b")
	series := []MetricSeries{
		{Labels: map[string]string{"instance": a}, Unit: "cores", Points: []MetricPoint{
			{Timestamp: "t1", Value: 10},
			{Timestamp: "t2", Value: 12},
		}},
		{Labels: map[string]string{"instance": b}, Unit: "cores", Points: []MetricPoint{
			{Timestamp: "t1", Value: 30},
			// t2 absent — must not become 0 or borrow from t1
		}},
	}
	out := aggregateReplicasAtTimestamps(app, replicaAggregateAvg, series)
	if len(out) != 1 || len(out[0].Points) != 2 {
		t.Fatalf("points = %+v", out)
	}
	byTS := map[string]float64{}
	for _, p := range out[0].Points {
		byTS[p.Timestamp] = p.Value
	}
	if byTS["t1"] != 20 {
		t.Fatalf("t1 avg = %v, want 20", byTS["t1"])
	}
	if byTS["t2"] != 12 {
		t.Fatalf("t2 avg = %v, want 12 (only pod-a present)", byTS["t2"])
	}
}

func TestFilterSeriesByInstancesSelectsOne(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	b := ids.ServiceInstanceID(app, "pod-b")
	series := []MetricSeries{
		{Labels: map[string]string{"instance": a}, Points: []MetricPoint{{Value: 10}}},
		{Labels: map[string]string{"instance": b}, Points: []MetricPoint{{Value: 30}}},
	}
	got := filterSeriesByInstances(app, []string{b}, series, nil)
	if len(got) != 1 || got[0].Labels["instance"] != b || got[0].Points[0].Value != 30 {
		t.Fatalf("got %+v", got)
	}
}

func TestFilterSeriesUnknownDoesNotBroaden(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	series := []MetricSeries{{Labels: map[string]string{"instance": a}, Points: []MetricPoint{{Value: 10}}}}
	got := filterSeriesByInstances(app, []string{"srv-a-zzzzzzzzzzzzzzzzzzzz"}, series, nil)
	if len(got) != 0 {
		t.Fatalf("unknown selector broadened to %+v", got)
	}
}

func TestValidateInstanceSelection(t *testing.T) {
	if err := validateInstanceSelection(nil); err != nil {
		t.Fatal(err)
	}
	if err := validateInstanceSelection([]string{""}); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("blank = %v", err)
	}
	tooMany := make([]string, maxSelectedInstances+1)
	for i := range tooMany {
		tooMany[i] = "x"
	}
	if err := validateInstanceSelection(tooMany); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("oversized = %v", err)
	}
}

func TestParseReplicaAggregate(t *testing.T) {
	r, max, err := parseReplicaAggregate("MAX")
	if err != nil || r != replicaAggregateMax || !max {
		t.Fatalf("MAX = %q %v %v", r, max, err)
	}
	r, max, err = parseReplicaAggregate("MIN")
	if err != nil || r != replicaAggregateMin || max {
		t.Fatalf("MIN = %q %v %v", r, max, err)
	}
	if _, _, err := parseReplicaAggregate("MEDIAN"); !errors.Is(err, core.ErrBadRequest) {
		t.Fatalf("unknown = %v", err)
	}
}

func TestApplyInstanceSelectionEndToEnd(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	b := ids.ServiceInstanceID(app, "pod-b")
	ts := "2026-09-08T12:00:00Z"
	series := []MetricSeries{
		{Labels: map[string]string{"resource": app, "instance": a}, Unit: "cores", Points: []MetricPoint{{Timestamp: ts, Value: 10}}},
		{Labels: map[string]string{"resource": app, "instance": b}, Unit: "cores", Points: []MetricPoint{{Timestamp: ts, Value: 30}}},
	}
	out, err := applyInstanceSelection(MetricQuery{
		App: app, Instances: []string{b}, ReplicaAggregate: replicaAggregateMax,
	}, series, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Points[0].Value != 30 {
		t.Fatalf("select b + MAX = %+v", out)
	}
	out, err = applyInstanceSelection(MetricQuery{
		App: app, Instances: []string{a, b}, ReplicaAggregate: replicaAggregateAvg,
	}, series, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 || out[0].Points[0].Value != 20 {
		t.Fatalf("select both + AVG = %+v", out)
	}
}

func TestInstanceFilterSupportedCoversLimits(t *testing.T) {
	for _, m := range []string{MetricCPU, MetricMemory, MetricCPULimit, MetricMemoryLimit} {
		if !instanceFilterSupported(m) {
			t.Fatalf("%q should support INSTANCE", m)
		}
	}
	for _, m := range []string{MetricInstanceCount, MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth, MetricCPUTarget, MetricMemoryTarget} {
		if instanceFilterSupported(m) {
			t.Fatalf("%q should reject INSTANCE", m)
		}
	}
}

func TestApplyInstanceSelectionEmptySupportedSucceeds(t *testing.T) {
	app := "srv-a"
	sel := ids.ServiceInstanceID(app, "pod-a")
	for _, m := range []string{MetricCPU, MetricMemory, MetricCPULimit, MetricMemoryLimit} {
		out, err := applyInstanceSelection(MetricQuery{App: app, Metric: m, Instances: []string{sel}}, nil, nil)
		if err != nil {
			t.Fatalf("%s empty = %v", m, err)
		}
		if len(out) != 0 {
			t.Fatalf("%s empty = %+v, want empty success", m, out)
		}
	}
}

func TestApplyInstanceSelectionUnsupportedRejectsWhenEmpty(t *testing.T) {
	app := "srv-a"
	sel := ids.ServiceInstanceID(app, "pod-a")
	for _, m := range []string{MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth, MetricInstanceCount, MetricCPUTarget} {
		if _, err := applyInstanceSelection(MetricQuery{App: app, Metric: m, Instances: []string{sel}}, nil, nil); !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("%s empty = %v, want ErrBadRequest", m, err)
		}
	}
}

func TestMetricsEmptyInstanceFilteredQuerySucceeds(t *testing.T) {
	svc := rangeService(staticRangeSource(nil, nil), nil, sampleApp("web"), podWithLimits(webInst))
	sel := ids.ServiceInstanceID("web", webInst)
	series, err := svc.Metrics(context.Background(), MetricQuery{App: "web", Metric: MetricCPU, Instances: []string{sel}})
	if err != nil {
		t.Fatalf("empty filtered cpu = %v", err)
	}
	if len(series) != 0 {
		t.Fatalf("empty filtered cpu = %+v, want no series", series)
	}
}

func TestInstanceFilterEmptyCrossSurfaceParity(t *testing.T) {
	emptySvc := func() *Service {
		svc := rangeService(staticRangeSource(nil, nil), nil, sampleApp("web"), podWithLimits(webInst))
		svc.RequestMetrics = func(_ context.Context, _ RequestMetricsRequest) ([]MetricSeries, error) {
			return nil, nil
		}
		return svc
	}
	sel := ids.ServiceInstanceID("web", webInst)

	if rec := serveREST(emptySvc(), "/v1/metrics/cpu?resource=web&instance="+sel); rec.Code != 200 {
		t.Fatalf("REST empty filtered cpu = %d %s, want 200", rec.Code, rec.Body.String())
	}
	schema, err := gqlSchema(emptySvc())
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field:"RESOURCE",values:["web"]},{field:"INSTANCE",values:["` + sel + `"]}], name:"CPU"}) { unit } }`})
	if len(res.Errors) > 0 {
		t.Fatalf("GraphQL empty filtered cpu = %v", res.Errors)
	}
	cs := mcpSession(t, emptySvc())
	mcpRes, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resource": []string{"web"}, "metricTypes": []string{MetricCPU}, "instance": []string{sel},
	}})
	if err != nil || mcpRes.IsError {
		t.Fatalf("MCP empty filtered cpu: err=%v isError=%v", err, mcpRes != nil && mcpRes.IsError)
	}

	if rec := serveREST(emptySvc(), "/v1/metrics/http-requests?resource=web&instance="+sel); rec.Code != 400 {
		t.Fatalf("REST instance on http_requests = %d, want 400", rec.Code)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ metrics(query: {filters: [{field:"RESOURCE",values:["web"]},{field:"INSTANCE",values:["` + sel + `"]}], name:"HTTP_REQUESTS"}) { unit } }`})
	if len(res.Errors) == 0 {
		t.Fatal("GraphQL instance on http_requests should error")
	}
	mcpRes, err = cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_metrics", Arguments: map[string]any{
		"resource": []string{"web"}, "metricTypes": []string{MetricHTTPRequests}, "instance": []string{sel},
	}})
	if err == nil && !mcpRes.IsError {
		t.Fatal("MCP instance on http_requests should error")
	}
}

func TestApplyInstanceSelectionForeignDoesNotBroaden(t *testing.T) {
	app := "srv-a"
	a := ids.ServiceInstanceID(app, "pod-a")
	series := []MetricSeries{{Labels: map[string]string{"instance": a}, Points: []MetricPoint{{Value: 10}}}}
	foreign := ids.ServiceInstanceID("srv-other", "pod-x")
	out, err := applyInstanceSelection(MetricQuery{App: app, Metric: MetricCPU, Instances: []string{foreign}}, series, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("foreign selector broadened to %+v", out)
	}
}
