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
	"log"
	"slices"
	"sort"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/savedsearch"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// project.go reads a project's Service Metrics view (w3/m508,
// docs/ADR010-observability.md § Project views): one chart for every resource
// the view's filter document keeps, read in one request. The server resolves
// the scope as a log view's (w3/m505), reads each resource through the same
// Metrics verb a service page reads (so its plan gates, retention floor and
// creation floor apply per resource), and groups the result as Render's
// metrics-view group-by does: one series per resource, per status code, or one
// for the whole scope.

// ProjectResources lists a project's resources to a caller who may view the
// project, and the project's workspace; projects.Service satisfies it, as it
// does for the log views.
type ProjectResources interface {
	ProjectLogResources(ctx context.Context, projectID string) (workspaceID string, resources []savedsearch.Resource, err error)
}

// ErrProjectMetricsUnavailable answers a project read when no project listing
// is wired (no control-plane store).
var ErrProjectMetricsUnavailable = core.Unavailable("project metrics views need the projects store")

// maxProjectMetricResources bounds the resources one view charts: a log
// view's cap (logs' maxLogResources), so a view that reads its logs charts
// its metrics too.
const maxProjectMetricResources = 20

// ProjectViewTooLargeCode is the coded refusal of a view past
// maxProjectMetricResources: params {resources, limit}, so a client names both.
const ProjectViewTooLargeCode = "PROJECT_VIEW_TOO_LARGE"

// maxConcurrentProjectReads bounds the resources read at once.
const maxConcurrentProjectReads = 8

// Render's metrics-view group-by (its dashboard bundle's enum).
const (
	ProjectGroupByNone       = "NONE"
	ProjectGroupByResource   = "RESOURCE"
	ProjectGroupByStatusCode = "STATUS_CODE"
)

// What a project read did with each resource in its scope.
const (
	// ProjectResourceCharted: read; its series (none when it had no data in
	// the window) are in the answer.
	ProjectResourceCharted = "CHARTED"
	// ProjectResourceNotCharted: the metric has no series for the resource's
	// type (a static site's CPU, a worker's requests, a datastore's instances).
	ProjectResourceNotCharted = "NOT_CHARTED"
	// ProjectResourceFailed: its read failed. It contributes nothing to the
	// series, never a zero.
	ProjectResourceFailed = "FAILED"
)

// ProjectMetricQuery is one chart of a project's Service Metrics view.
type ProjectMetricQuery struct {
	Project string
	// FilterDocument is the view's filter document as decoded JSON; nil is
	// Render's empty default (every resource of the project).
	FilterDocument any
	Metric         string
	Start, End     time.Time
	Resolution     time.Duration
	// Quantiles are the http_latency percentiles (empty: the default).
	Quantiles []float64
	// GroupBy is a ProjectGroupBy* value; "" is RESOURCE.
	GroupBy string
}

// ProjectMetricResource is one readable resource of the view (its id, shown
// name, type and environment, as savedsearch.Resource) and what the read did
// with it.
type ProjectMetricResource struct {
	ID            string
	Name          string
	Type          string
	EnvironmentID string
	Status        string
}

// ProjectMetrics is a project chart: its series, grouped, and every resource
// of the view the caller may read, in the project's order. A resource the
// caller may not read is absent, never named.
type ProjectMetrics struct {
	Series    []QuantileSeries
	Resources []ProjectMetricResource
}

// projectMetricIDs are the charts a Service Metrics view draws.
var projectMetricIDs = []string{MetricCPU, MetricMemory, MetricInstanceCount, MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth}

// chartsResource reports whether metric has series for r's type: CPU and
// memory for everything with a process (a datastore's own container
// included), instances for services that run them, requests and response
// times for what the public edge serves, bandwidth for every service.
func chartsResource(metric string, r savedsearch.Resource) bool {
	switch r.Type {
	case savedsearch.KindPostgres, savedsearch.KindKeyValue:
		return metric == MetricCPU || metric == MetricMemory
	case appv1alpha1.TypeStaticSite:
		return metric == MetricHTTPRequests || metric == MetricHTTPLatency || metric == MetricBandwidth
	case "", appv1alpha1.TypeWebService:
		return true
	default: // private services, background workers and cron jobs
		return metric != MetricHTTPRequests && metric != MetricHTTPLatency
	}
}

// AuditVerbMetrics is the verb a project chart's reads record: each is a
// Metrics read of one resource, and the project's own listing authorizes as
// projects.ProjectLogResources. bandwidthSources' hourly reads record it too.
const AuditVerbMetrics = "metrics.Metrics"

// projectMetrics reads one chart of a project's Service Metrics view. It is
// reached from the GraphQL resolver's closure, so its reads name their verb.
func (s *Service) projectMetrics(ctx context.Context, q ProjectMetricQuery) (ProjectMetrics, error) {
	ctx = core.WithRequestMemo(core.WithAuditVerb(ctx, AuditVerbMetrics))
	if s.Projects == nil {
		return ProjectMetrics{}, ErrProjectMetricsUnavailable
	}
	if !slices.Contains(projectMetricIDs, q.Metric) {
		return ProjectMetrics{}, fmt.Errorf("%w: a project metrics view charts %v, not %q", core.ErrBadRequest, projectMetricIDs, q.Metric)
	}
	groupBy, err := projectGroupBy(q.GroupBy, q.Metric)
	if err != nil {
		return ProjectMetrics{}, err
	}
	if err := s.checkWindow(q.Start, q.End); err != nil {
		return ProjectMetrics{}, err
	}
	doc := savedsearch.Default()
	if q.FilterDocument != nil {
		if doc, err = savedsearch.ParseValue(q.FilterDocument, savedsearch.TypeMetrics); err != nil {
			return ProjectMetrics{}, err
		}
	}
	workspace, all, err := s.Projects.ProjectLogResources(ctx, q.Project)
	if err != nil {
		return ProjectMetrics{}, err
	}
	scoped := doc.Scope(savedsearch.TypeMetrics, all)
	if len(scoped) > maxProjectMetricResources {
		return ProjectMetrics{}, core.NewBadRequestError(ProjectViewTooLargeCode,
			fmt.Sprintf("%s: this view charts %d resources, and one project metrics query reads at most %d; narrow it with an Environment, Resource or Resource Type filter", core.ErrBadRequest, len(scoped), maxProjectMetricResources),
			map[string]any{"resources": len(scoped), "limit": maxProjectMetricResources})
	}
	// Response times are a Pro-and-higher feature (w8/m138): the workspace's
	// refusal, once, even when no resource of the scope serves requests.
	if q.Metric == MetricHTTPLatency {
		if err := s.RequirePlanFeature(ctx, workspace, core.FeatureLatencyMetrics); err != nil {
			return ProjectMetrics{}, err
		}
	}
	charted := 0
	for _, r := range scoped {
		if chartsResource(q.Metric, r) {
			charted++
		}
	}
	if err := checkFanOut(charted, 1, latencyFan(q.Metric, q.Quantiles)); err != nil {
		return ProjectMetrics{}, err
	}
	// One window for every resource, so each one's series share the grid the
	// groups add up on.
	mq := MetricQuery{Metric: q.Metric, Start: q.Start, End: q.End, Resolution: q.Resolution, Quantiles: q.Quantiles}.normalized(s.Now())
	if groupBy == ProjectGroupByStatusCode {
		mq.GroupBy = groupByStatus
	}
	reads := make([]projectRead, len(scoped))
	var g errgroup.Group
	g.SetLimit(maxConcurrentProjectReads)
	for i, r := range scoped {
		g.Go(func() error {
			reads[i] = s.readProjectResource(ctx, mq, r)
			return nil
		})
	}
	_ = g.Wait()

	out := ProjectMetrics{Series: []QuantileSeries{}, Resources: []ProjectMetricResource{}}
	var firstFailure error
	read := make([]projectRead, 0, len(reads))
	for i, rd := range reads {
		status := ProjectResourceCharted
		switch err := rd.err; {
		case err == nil && !rd.charted:
			status = ProjectResourceNotCharted
		case err == nil:
			read = append(read, rd)
		case errors.Is(err, core.ErrNotFound) || errors.Is(err, core.ErrForbidden):
			continue // unreadable or gone: never named
		case errors.Is(err, core.ErrBadRequest) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			return ProjectMetrics{}, err // the request's own fault, the same for every resource
		default:
			status = ProjectResourceFailed
			if firstFailure == nil {
				firstFailure = err
			}
			if !errors.Is(err, core.ErrUnavailable) { // a backend not wired is no news
				log.Printf("metrics: project %s %s read of %s failed: %v", q.Project, q.Metric, scoped[i].ID, err)
			}
		}
		r := scoped[i]
		out.Resources = append(out.Resources, ProjectMetricResource{ID: r.ID, Name: r.Name, Type: r.Type, EnvironmentID: r.EnvironmentID, Status: status})
	}
	// Nothing read and something failed: the read failed, with that failure's
	// own error (no metrics backend, throttled), as a service page's would.
	if len(read) == 0 && firstFailure != nil {
		return ProjectMetrics{}, firstFailure
	}
	out.Series = groupProjectSeries(groupBy, read)
	return out, nil
}

// projectGroupBy validates a group-by against the metric: a status code
// groups requests only, and percentiles do not add up across resources.
func projectGroupBy(groupBy, metric string) (string, error) {
	switch groupBy {
	case "", ProjectGroupByResource:
		return ProjectGroupByResource, nil
	case ProjectGroupByNone:
		if metric == MetricHTTPLatency {
			return "", fmt.Errorf("%w: groupBy %s does not apply to %s: response time percentiles do not add up across resources; group by %s", core.ErrBadRequest, ProjectGroupByNone, metric, ProjectGroupByResource)
		}
		return groupBy, nil
	case ProjectGroupByStatusCode:
		if metric != MetricHTTPRequests {
			return "", fmt.Errorf("%w: groupBy %s applies only to %s", core.ErrBadRequest, ProjectGroupByStatusCode, MetricHTTPRequests)
		}
		return groupBy, nil
	}
	return "", fmt.Errorf("%w: unknown groupBy %q (want %s, %s or %s)", core.ErrBadRequest, groupBy, ProjectGroupByNone, ProjectGroupByResource, ProjectGroupByStatusCode)
}

// projectRead is one resource's part of a project read.
type projectRead struct {
	id      string
	charted bool
	series  []QuantileSeries
	err     error
}

// readProjectResource reads r's series through the verb its own page reads,
// or, for a type the metric does not chart, only checks the caller may read
// it, so the answer names it without reading anything.
func (s *Service) readProjectResource(ctx context.Context, q MetricQuery, r savedsearch.Resource) projectRead {
	out := projectRead{id: r.ID, charted: chartsResource(q.Metric, r)}
	if !out.charted {
		out.err = s.authorizeProjectResource(ctx, r)
		return out
	}
	q.App = r.ID
	if core.IsDatastoreResource(r.ID) {
		series, err := s.datastoreAppMetric(ctx, q, r.ID)
		for _, ser := range series {
			out.series = append(out.series, QuantileSeries{MetricSeries: ser})
		}
		out.err = err
		return out
	}
	out.series, out.err = s.MetricsWithQuantiles(ctx, q)
	return out
}

// authorizeProjectResource is the view check a read of r would make.
func (s *Service) authorizeProjectResource(ctx context.Context, r savedsearch.Resource) error {
	switch r.Type {
	case savedsearch.KindPostgres:
		db, err := s.AuthorizeDatabase(ctx, core.RelCanView, r.ID)
		if err != nil {
			return err
		}
		return core.NotFoundIfDeleting(db)
	case savedsearch.KindKeyValue:
		kv, err := s.AuthorizeKeyValue(ctx, core.RelCanView, r.ID)
		if err != nil {
			return err
		}
		return core.NotFoundIfDeleting(kv)
	default:
		_, err := s.AuthorizeApp(ctx, core.RelCanView, r.ID)
		return err
	}
}

// groupProjectSeries groups the resources' series as groupBy asks. RESOURCE
// is one series per resource (its instances added up; response times one per
// percentile), STATUS_CODE one per status code across every resource, NONE
// one for the whole scope. A resource with no data adds nothing.
func groupProjectSeries(groupBy string, reads []projectRead) []QuantileSeries {
	out := []QuantileSeries{}
	switch groupBy {
	case ProjectGroupByNone:
		var all []MetricSeries
		for _, rd := range reads {
			for _, ser := range rd.series {
				all = append(all, ser.MetricSeries)
			}
		}
		if sum, ok := sumSeries(map[string]string{}, all); ok {
			out = append(out, QuantileSeries{MetricSeries: sum})
		}
	case ProjectGroupByStatusCode:
		byCode := map[string][]MetricSeries{}
		for _, rd := range reads {
			for _, ser := range rd.series {
				code := ser.Labels[sourceStatusCodeLabel]
				byCode[code] = append(byCode[code], ser.MetricSeries)
			}
		}
		codes := make([]string, 0, len(byCode))
		for code := range byCode {
			codes = append(codes, code)
		}
		sort.Strings(codes)
		for _, code := range codes {
			if sum, ok := sumSeries(map[string]string{sourceStatusCodeLabel: code}, byCode[code]); ok {
				out = append(out, QuantileSeries{MetricSeries: sum})
			}
		}
	default: // RESOURCE
		for _, rd := range reads {
			if len(rd.series) > 0 && rd.series[0].HasQuantile {
				// A percentile is the resource's own: one series per quantile.
				for _, ser := range rd.series {
					labels := map[string]string{LabelResource: rd.id}
					if quant, ok := ser.Labels["quantile"]; ok {
						labels["quantile"] = quant
					}
					ser.MetricSeries = MetricSeries{Labels: labels, Unit: ser.Unit, Points: ser.Points}
					out = append(out, ser)
				}
				continue
			}
			series := make([]MetricSeries, 0, len(rd.series))
			for _, ser := range rd.series {
				series = append(series, ser.MetricSeries)
			}
			if sum, ok := sumSeries(map[string]string{LabelResource: rd.id}, series); ok {
				out = append(out, QuantileSeries{MetricSeries: sum})
			}
		}
	}
	return out
}

// sumSeries adds series point by point at each timestamp into one series
// labeled labels: a resource's instances, or several resources. A timestamp
// only some series have adds only theirs. false when there is no point.
func sumSeries(labels map[string]string, series []MetricSeries) (MetricSeries, bool) {
	byTS := map[string]float64{}
	unit := ""
	for _, ser := range series {
		if unit == "" {
			unit = ser.Unit
		}
		for _, p := range ser.Points {
			byTS[p.Timestamp] += p.Value
		}
	}
	if len(byTS) == 0 {
		return MetricSeries{}, false
	}
	timestamps := make([]string, 0, len(byTS))
	for ts := range byTS {
		timestamps = append(timestamps, ts)
	}
	sort.Strings(timestamps)
	points := make([]MetricPoint, 0, len(timestamps))
	for _, ts := range timestamps {
		points = append(points, MetricPoint{Timestamp: ts, Value: byTS[ts]})
	}
	return MetricSeries{Labels: labels, Unit: unit, Points: points}, true
}
