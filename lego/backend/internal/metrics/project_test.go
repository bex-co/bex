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
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// fakeProjects is a project's resource listing, as projects.Service answers it.
type fakeProjects struct {
	workspace string
	resources []savedsearch.Resource
	err       error
}

func (f fakeProjects) ProjectLogResources(context.Context, string) (string, []savedsearch.Resource, error) {
	return f.workspace, f.resources, f.err
}

const projectEnv = "evm-c185th5c2rvvnhbfilu0"

// The fixture's services' ids, by App name.
var projectIDs = map[string]string{
	"web":   "srv-c185th5c2rvvnhbfilt0",
	"api":   "srv-c185th5c2rvvnhbfilug",
	"admin": "srv-c185th5c2rvvnhbfilv0",
	"jobs":  "srv-c185th5c2rvvnhbfilvg",
	"docs":  "srv-c185th5c2rvvnhbfil00",
}

func typedApp(name, typ string) *appv1alpha1.App {
	app := sampleApp(name)
	app.Spec.Type = typ
	app.Labels = map[string]string{core.LabelTenant: "tea-a", core.LabelAppID: projectIDs[name]}
	return app
}

// named rewrites a comma list of App names as their ids.
func named(names string) string {
	parts := strings.Split(names, ",")
	for i, n := range parts {
		if id, ok := projectIDs[n]; ok {
			parts[i] = id
		}
	}
	return strings.Join(parts, ",")
}

// namedStatuses rewrites "name=STATUS" pairs with the names as ids.
func namedStatuses(pairs string) string {
	parts := strings.Split(pairs, ",")
	for i, p := range parts {
		name, status, _ := strings.Cut(p, "=")
		parts[i] = named(name) + "=" + status
	}
	return strings.Join(parts, ",")
}

// projectFixture is a project of three web services (web with two instances,
// api and admin), a background worker, a static site and a Postgres no App
// backs (the caller cannot read it), each charting a fixed value per instance.
func projectFixture(t *testing.T) (*Service, *sync.Map) {
	t.Helper()
	svc := newService(nil, nil,
		typedApp("web", appv1alpha1.TypeWebService),
		typedApp("api", appv1alpha1.TypeWebService),
		typedApp("admin", appv1alpha1.TypeWebService),
		typedApp("jobs", appv1alpha1.TypeBackgroundWorker),
		typedApp("docs", appv1alpha1.TypeStaticSite),
	)
	reads := &sync.Map{}
	ts := func(i int) string { return fixedClock().Add(time.Duration(i-2) * time.Minute).Format(time.RFC3339) }
	svc.ResourceMetricsRange = func(_ context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		reads.Store(req.App+"/"+req.Metric, true)
		instances := map[string]int{"web": 2, "api": 1, "admin": 1, "jobs": 1}[req.App]
		var out []MetricSeries
		for i := range instances {
			out = append(out, MetricSeries{
				Labels: map[string]string{"instance": fmt.Sprintf("%s-%d", req.App, i)},
				Points: []MetricPoint{{Timestamp: ts(0), Value: 0.25}, {Timestamp: ts(1), Value: 0.5}},
			})
		}
		return out, nil
	}
	svc.RequestMetrics = func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		reads.Store(req.App+"/"+req.Metric, true)
		if req.GroupBy == groupByStatus {
			return []MetricSeries{
				{Labels: map[string]string{"code": "200"}, Points: []MetricPoint{{Timestamp: ts(0), Value: 3}}},
				{Labels: map[string]string{"code": "500"}, Points: []MetricPoint{{Timestamp: ts(0), Value: 1}}},
			}, nil
		}
		return []MetricSeries{{Points: []MetricPoint{{Timestamp: ts(0), Value: 4}, {Timestamp: ts(1), Value: 2}}}}, nil
	}
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "pro", nil })
	svc.Projects = fakeProjects{workspace: "tea-a", resources: []savedsearch.Resource{
		{ID: projectIDs["web"], Name: "web", Type: appv1alpha1.TypeWebService, EnvironmentID: projectEnv},
		{ID: projectIDs["api"], Name: "api", Type: appv1alpha1.TypeWebService, EnvironmentID: projectEnv},
		{ID: projectIDs["admin"], Name: "admin", Type: appv1alpha1.TypeWebService},
		{ID: projectIDs["jobs"], Name: "jobs", Type: appv1alpha1.TypeBackgroundWorker, EnvironmentID: projectEnv},
		{ID: projectIDs["docs"], Name: "docs", Type: appv1alpha1.TypeStaticSite, EnvironmentID: projectEnv},
		{ID: "dpg-c185th5c2rvvnhbfiltg", Name: "db", Type: savedsearch.KindPostgres, EnvironmentID: projectEnv},
	}}
	return svc, reads
}

func seriesLabels(series []QuantileSeries, key string) string {
	var out []string
	for _, s := range series {
		out = append(out, s.Labels[key])
	}
	return strings.Join(out, ",")
}

func resourceStatuses(rs []ProjectMetricResource) string {
	var out []string
	for _, r := range rs {
		out = append(out, r.ID+"="+r.Status)
	}
	return strings.Join(out, ",")
}

// A three-service project returns three CPU series labeled by resource, each
// the sum of its instances; what CPU has no series for, and what the caller
// cannot read, says so or is absent.
func TestProjectMetricsChartEveryResource(t *testing.T) {
	svc, _ := projectFixture(t)
	got, err := svc.projectMetrics(context.Background(), ProjectMetricQuery{Project: "prj-x", Metric: MetricCPU})
	if err != nil {
		t.Fatal(err)
	}
	if labels := seriesLabels(got.Series, LabelResource); labels != named("web,api,admin,jobs") {
		t.Fatalf("CPU series = %s, want one per service with a process", labels)
	}
	if web := got.Series[0]; web.Points[1].Value != 1 || web.Unit != unitCores || len(web.Labels) != 1 {
		t.Errorf("web's two instances should add up to 1 core in one series labeled by resource: %+v", web)
	}
	if s := resourceStatuses(got.Resources); s != namedStatuses("web=CHARTED,api=CHARTED,admin=CHARTED,jobs=CHARTED,docs=NOT_CHARTED") {
		t.Errorf("resources = %s; the static site is not charted and the unreadable Postgres is never named", s)
	}
	if got.Resources[0].Name != "web" || got.Resources[0].EnvironmentID != projectEnv {
		t.Errorf("a resource keeps its name and environment: %+v", got.Resources[0])
	}

	requests, err := svc.projectMetrics(context.Background(), ProjectMetricQuery{Project: "prj-x", Metric: MetricHTTPRequests})
	if err != nil {
		t.Fatal(err)
	}
	if labels := seriesLabels(requests.Series, LabelResource); labels != named("web,api,admin,docs") {
		t.Errorf("request series = %s, want the web services and the static site", labels)
	}
	if s := resourceStatuses(requests.Resources); !strings.Contains(s, named("jobs")+"=NOT_CHARTED") {
		t.Errorf("a worker serves no requests: %s", s)
	}
}

// The view's own filter document narrows the scope, as a log view's does.
func TestProjectMetricsFollowTheViewDocument(t *testing.T) {
	svc, _ := projectFixture(t)
	doc := map[string]any{"version": 1, "filters": []any{
		map[string]any{"field": "ENVIRONMENT", "operator": "IS", "values": []any{projectEnv}},
		map[string]any{"field": "RESOURCE", "operator": "IS_NOT", "values": []any{projectIDs["api"]}},
	}}
	got, err := svc.projectMetrics(context.Background(), ProjectMetricQuery{Project: "prj-x", Metric: MetricMemory, FilterDocument: doc})
	if err != nil {
		t.Fatal(err)
	}
	if labels := seriesLabels(got.Series, LabelResource); labels != named("web,jobs") {
		t.Errorf("series = %s, want the environment's services but api", labels)
	}
	bad := map[string]any{"version": 1, "filters": []any{map[string]any{"field": "LEVEL", "operator": "IS", "values": []any{"error"}}}}
	if _, err := svc.projectMetrics(context.Background(), ProjectMetricQuery{Project: "prj-x", Metric: MetricCPU, FilterDocument: bad}); !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "LEVEL") {
		t.Errorf("a log field in a metrics view's document is refused by name, got %v", err)
	}
}

// Group by NONE adds every resource up; STATUS_CODE adds each code up across
// resources; percentiles never add up, and a status code groups requests only.
func TestProjectMetricsGroupBy(t *testing.T) {
	svc, _ := projectFixture(t)
	ctx := context.Background()
	none, err := svc.projectMetrics(ctx, ProjectMetricQuery{Project: "prj-x", Metric: MetricHTTPRequests, GroupBy: ProjectGroupByNone})
	if err != nil {
		t.Fatal(err)
	}
	if len(none.Series) != 1 || none.Series[0].Points[0].Value != 16 || len(none.Series[0].Labels) != 0 {
		t.Errorf("NONE should be one unlabeled series of 4 requests from each of four resources: %+v", none.Series)
	}
	byCode, err := svc.projectMetrics(ctx, ProjectMetricQuery{Project: "prj-x", Metric: MetricHTTPRequests, GroupBy: ProjectGroupByStatusCode})
	if err != nil {
		t.Fatal(err)
	}
	if labels := seriesLabels(byCode.Series, "STATUS_CODE"); labels != "200,500" || byCode.Series[0].Points[0].Value != 12 || byCode.Series[1].Points[0].Value != 4 {
		t.Errorf("STATUS_CODE should add each code across the four resources: %+v", byCode.Series)
	}
	for _, tc := range []struct{ metric, groupBy, want string }{
		{MetricHTTPLatency, ProjectGroupByNone, "do not add up"},
		{MetricCPU, ProjectGroupByStatusCode, "applies only to http_requests"},
		{MetricCPU, "SERVICE", "unknown groupBy"},
	} {
		_, err := svc.projectMetrics(ctx, ProjectMetricQuery{Project: "prj-x", Metric: tc.metric, GroupBy: tc.groupBy})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s by %s: got %v, want a bad request naming %q", tc.metric, tc.groupBy, err, tc.want)
		}
	}
}

// A Hobby workspace gets the response-time chart's Pro refusal before any
// resource is read; request counts stay open.
func TestProjectMetricsLatencyFollowsThePlan(t *testing.T) {
	svc, reads := projectFixture(t)
	svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return "hobby", nil })
	_, err := svc.projectMetrics(context.Background(), ProjectMetricQuery{Project: "prj-x", Metric: MetricHTTPLatency})
	var coded *core.CodedError
	if !errors.As(err, &coded) || coded.Code != core.PlanFeatureRequiredCode || coded.Params["feature"] != "latencyMetrics" {
		t.Fatalf("hobby latency: got %v, want PLAN_FEATURE_REQUIRED for latencyMetrics", err)
	}
	if _, read := reads.Load("web/" + MetricHTTPLatency); read {
		t.Error("a refused chart reached the source")
	}
	if _, err := svc.projectMetrics(context.Background(), ProjectMetricQuery{Project: "prj-x", Metric: MetricHTTPRequests}); err != nil {
		t.Errorf("request counts stay open on Hobby: %v", err)
	}
}

// A resource whose read fails is named FAILED and adds nothing, never a zero;
// when nothing could be read the chart fails with that failure's own error.
func TestProjectMetricsPartialAndFailedReads(t *testing.T) {
	svc, _ := projectFixture(t)
	ranged := svc.ResourceMetricsRange
	svc.ResourceMetricsRange = func(ctx context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		if req.App == "api" {
			return nil, errors.New("prometheus: query timed out")
		}
		return ranged(ctx, req)
	}
	got, err := svc.projectMetrics(context.Background(), ProjectMetricQuery{Project: "prj-x", Metric: MetricCPU, GroupBy: ProjectGroupByNone})
	if err != nil {
		t.Fatal(err)
	}
	if s := resourceStatuses(got.Resources); !strings.Contains(s, named("api")+"=FAILED") || !strings.Contains(s, named("web")+"=CHARTED") {
		t.Errorf("statuses = %s, want api FAILED beside the charted ones", s)
	}
	if v := got.Series[0].Points[1].Value; v != 2 {
		t.Errorf("the total adds web (2 instances), admin and jobs only: %v, want 2", v)
	}

	svc.ResourceMetricsRange = nil
	svc.ResourceMetrics = nil
	if _, err := svc.projectMetrics(context.Background(), ProjectMetricQuery{Project: "prj-x", Metric: MetricCPU}); !errors.Is(err, core.ErrMetricsUnavailable) {
		t.Errorf("no metrics backend at all should answer METRICS_UNAVAILABLE, got %v", err)
	}
}

// A view past the bound is refused by name; so is a read with no projects
// store, a metric the view does not chart, and a project the caller cannot see.
func TestProjectMetricsRefusals(t *testing.T) {
	svc, _ := projectFixture(t)
	ctx := context.Background()
	many := make([]savedsearch.Resource, maxProjectMetricResources+1)
	for i := range many {
		many[i] = savedsearch.Resource{ID: fmt.Sprintf("svc-%d", i), Type: appv1alpha1.TypeWebService}
	}
	svc.Projects = fakeProjects{workspace: "tea-a", resources: many}
	_, err := svc.projectMetrics(ctx, ProjectMetricQuery{Project: "prj-x", Metric: MetricCPU})
	var coded *core.CodedError
	if !errors.Is(err, core.ErrBadRequest) || !errors.As(err, &coded) || coded.Code != ProjectViewTooLargeCode ||
		coded.Params["resources"] != 21 || coded.Params["limit"] != 20 || !strings.Contains(err.Error(), "charts 21 resources") {
		t.Errorf("a view past the bound is refused by name, with both counts: got %v", err)
	}
	svc.Projects = fakeProjects{err: core.ErrNotFound}
	if _, err := svc.projectMetrics(ctx, ProjectMetricQuery{Project: "prj-x", Metric: MetricCPU}); !errors.Is(err, core.ErrNotFound) {
		t.Errorf("an unreadable project: got %v", err)
	}
	if _, err := svc.projectMetrics(ctx, ProjectMetricQuery{Project: "prj-x", Metric: MetricCPULimit}); !errors.Is(err, core.ErrBadRequest) {
		t.Errorf("a limit is not a view chart: got %v", err)
	}
	svc.Projects = nil
	if _, err := svc.projectMetrics(ctx, ProjectMetricQuery{Project: "prj-x", Metric: MetricCPU}); !errors.Is(err, ErrProjectMetricsUnavailable) {
		t.Errorf("no projects store: got %v", err)
	}
}

// GraphQL's projectMetrics carries the series, labels and resources.
func TestGraphQLProjectMetrics(t *testing.T) {
	svc, _ := projectFixture(t)
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: `{
		projectMetrics(query: {project: "prj-x", name: "HTTP_LATENCY", parameters: [{quantile: 0.5}, {quantile: 0.9}],
			filterDocument: {version: 1, filters: [{field: "RESOURCE", operator: "IS", values: ["srv-c185th5c2rvvnhbfilt0", "srv-c185th5c2rvvnhbfil00"]}]}}) {
			series { unit labels { field value } parameters { quantile } values { value } }
			resources { id name type environmentId status }
		}
	}`})
	if len(res.Errors) > 0 {
		t.Fatalf("gql: %v", res.Errors)
	}
	data := res.Data.(map[string]any)["projectMetrics"].(map[string]any)
	series := data["series"].([]any)
	if len(series) != 0 {
		// The fixture's request source answers latency with the count shape;
		// what matters is the labels: resource and quantile per series.
		first := series[0].(map[string]any)
		labels := fmt.Sprint(first["labels"])
		if !strings.Contains(labels, "resource") || !strings.Contains(labels, "quantile") {
			t.Errorf("a latency series names its resource and percentile: %s", labels)
		}
	}
	if got := len(series); got != 4 {
		t.Errorf("two resources × two percentiles = 4 series, got %d", got)
	}
	resources := data["resources"].([]any)
	if len(resources) != 2 || resources[1].(map[string]any)["status"] != ProjectResourceCharted || resources[1].(map[string]any)["type"] != appv1alpha1.TypeStaticSite {
		t.Errorf("resources = %v", resources)
	}
	res = graphql.Do(graphql.Params{Schema: schema, Context: context.Background(),
		RequestString: `{ projectMetrics(query: {project: "prj-x", name: "DISK"}) { resources { id } } }`})
	if len(res.Errors) == 0 {
		t.Error("an unknown chart name is refused")
	}
}
