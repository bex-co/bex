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
	"fmt"
	"sort"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// metrics.go is the Core metrics read — the m2 sibling of the logs verb. Like
// logs it is ONE domain implementation the REST + GraphQL adapters share, and it
// reaches its backends through injected sources so Core stays clientset-free:
//
//   - resource metrics (cpu/memory/instance_count) come from cAdvisor-over-
//     Prometheus via ResourceMetricsRange (stepped history) when it's wired,
//     else from metrics-server via ResourceMetrics (single current point) —
//     instance count in that fallback is derived from the App's pods directly;
//   - request metrics (count/latency/bandwidth) come from Traefik-over-Prometheus
//     via RequestMetrics.
//
// Shapes are Render's metrics-API time-series (see render.go: a series carries a
// unit, labels and points). The Prometheus-backed sources honor start/end/
// resolution like Render; the metrics-server fallback is a point-in-time
// snapshot, so there cpu/memory/instance_count series carry a single current
// point regardless of the requested time range — an intentional deviation
// documented in observability.md.

// Metric ids — bex's canonical names (underscored). They map 1:1 to Render's
// metrics endpoints (path segments cpu, memory, instance-count, http-requests,
// http-latency, bandwidth) in the REST adapter, and to Render's dashboard
// GraphQL `name` enum (MEMORY, MEMORY_LIMIT, CPU, CPU_LIMIT, INSTANCES,
// HTTP_REQUESTS, HTTP_LATENCY, ENRICHED_BANDWIDTH — see graphql.go's
// metricNameFromRender) verified live against a real Render dashboard session.
const (
	MetricCPU           = "cpu"
	MetricCPULimit      = "cpu_limit"
	MetricMemory        = "memory"
	MetricMemoryLimit   = "memory_limit"
	MetricInstanceCount = "instance_count"
	MetricHTTPRequests  = "http_requests"
	MetricHTTPLatency   = "http_latency"
	MetricBandwidth     = "bandwidth"
)

// Metric units (the `unit` field of a Render time-series). Resource metrics
// switch to percentage when MetricQuery.Percentage is set.
const (
	unitCores      = "cpu"        // fractional vCPU (metrics-server usage.cpu)
	unitBytes      = "bytes"      // memory / bandwidth
	unitCount      = "count"      // instance count / request count
	unitSeconds    = "seconds"    // request latency
	unitPercentage = "percentage" // cpu/memory as a fraction of the pod limit (0..100)
)

// defaults for a metrics query.
const (
	defaultResolution = time.Minute
	defaultQuantile   = 0.95 // p95, Render's default latency quantile
	defaultMetricSpan = time.Hour
)

// MetricPoint is one (timestamp, value) sample of a series.
type MetricPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// MetricSeries is one labeled time-series — Render's metrics shape. Labels
// identify the series (e.g. instance, resource, status); Unit names the value's
// dimension; Points are ordered oldest-first.
type MetricSeries struct {
	Labels map[string]string `json:"labels,omitempty"`
	Unit   string            `json:"unit"`
	Points []MetricPoint     `json:"points"`
}

// MetricQuery is the resolved request for the Metrics verb, shared by every
// adapter. Metric selects which series to return; the rest are filters/options
// (some apply only to resource or only to request metrics — see Metrics).
type MetricQuery struct {
	App        string
	Metric     string        // one of the Metric* ids
	Start, End time.Time     // request-metric time range (zero => End=now, Start=End-1h)
	Resolution time.Duration // request-metric step (<=0 => 1m)
	Quantile   float64       // http_latency percentile 0..1 (<=0 => .95)
	Percentage bool          // cpu/memory as a fraction of the pod limit instead of absolute
	StatusCode string        // request filter: "2xx" | "5xx" | "500" | ""
	Host       string        // request filter (Render vocabulary; see source for support)
	Path       string        // request filter (Render vocabulary; see source for support)
	GroupBy    string        // request group-by: "status" | "method" | "instance" | ""
	// AggregateMax collapses a per-instance series (cpu_limit/memory_limit) down
	// to one series holding the max value across instances — Render's dashboard
	// GraphQL requests this via aggregateAllMethod:"MAX" (captured live) when it
	// wants a single "Limit" figure rather than one per replica.
	AggregateMax bool
}

func (q MetricQuery) normalized(now time.Time) MetricQuery {
	if q.End.IsZero() {
		q.End = now
	}
	if q.Start.IsZero() {
		q.Start = q.End.Add(-defaultMetricSpan)
	}
	if q.Resolution <= 0 {
		q.Resolution = defaultResolution
	}
	if q.Quantile <= 0 {
		q.Quantile = defaultQuantile
	}
	return q
}

// PodResourceUsage is one pod's current CPU/memory usage — a metrics-server
// snapshot. CPUCores is fractional vCPU, MemoryBytes is bytes; Timestamp is the
// sample time (RFC3339, empty => Core stamps now).
type PodResourceUsage struct {
	Pod         string
	Timestamp   string
	CPUCores    float64
	MemoryBytes float64
}

// ResourceMetricsSource returns the current per-pod usage for an App's pods.
// Core depends on this narrow function instead of the metrics-server clientset so
// the domain layer stays apiserver-thin and is trivially faked in tests;
// production wires it via NewResourceMetricsSource. nil => cpu/memory metrics
// report ErrMetricsUnavailable.
type ResourceMetricsSource func(ctx context.Context, namespace, app string) ([]PodResourceUsage, error)

// ResourceMetricsRangeRequest is the backend-neutral ranged resource-metric ask
// handed to a ResourceMetricsRangeSource — the resource sibling of
// RequestMetricsRequest. The source owns the query language; Core only resolves
// defaults and delegates.
type ResourceMetricsRangeRequest struct {
	Namespace  string
	App        string
	Metric     string // cpu | memory | instance_count
	Start, End time.Time
	Resolution time.Duration
}

// ResourceMetricsRangeSource returns stepped resource time-series for an App
// over a time range (cAdvisor scraped by Prometheus) — per-instance series for
// cpu/memory, a single series for instance_count. When non-nil, Core prefers it
// over the snapshot ResourceMetrics source; nil => cpu/memory come from
// ResourceMetrics and instance_count from counting pods. Production wires it via
// NewPrometheusResourceSource.
type ResourceMetricsRangeSource func(ctx context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error)

// RequestMetricsRequest is the backend-neutral request-metric ask handed to a
// RequestMetricsSource. The source (Prometheus over Traefik) owns the query
// language; Core only resolves defaults and delegates, so swapping the ingress
// metrics backend never touches Core.
type RequestMetricsRequest struct {
	Namespace  string
	App        string
	Metric     string // http_requests | http_latency | bandwidth
	Start, End time.Time
	Resolution time.Duration
	Quantile   float64
	StatusCode string
	Host       string
	Path       string
	GroupBy    string
}

// RequestMetricsSource returns request time-series for an App from the ingress
// metrics backend (Traefik scraped by Prometheus). nil => request metrics report
// ErrMetricsUnavailable. Production wires it via NewPrometheusRequestSource.
type RequestMetricsSource func(ctx context.Context, req RequestMetricsRequest) ([]MetricSeries, error)

// Metrics is the single metrics read every adapter (REST + GraphQL) calls. It
// fails with ErrNotFound for an unknown App (like Get), dispatches on q.Metric,
// and returns Render-shaped series. resource/request metrics need their source
// wired (ErrMetricsUnavailable otherwise); instance count never does.
func (c *Core) Metrics(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	if err := c.authorize(ctx, relCanView); err != nil {
		return nil, err
	}
	if _, err := c.fetch(ctx, q.App); err != nil {
		return nil, err // ErrNotFound for unknown apps, exactly like Get
	}
	q = q.normalized(c.now())
	var series []MetricSeries
	var err error
	switch q.Metric {
	case MetricCPU, MetricMemory:
		series, err = c.resourceMetric(ctx, q)
	case MetricCPULimit, MetricMemoryLimit:
		series, err = c.resourceLimitMetric(ctx, q)
	case MetricInstanceCount:
		series, err = c.instanceCountMetric(ctx, q)
	case MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth:
		series, err = c.requestMetric(ctx, q)
	default:
		return nil, fmt.Errorf("unknown metric %q", q.Metric)
	}
	if err != nil {
		return nil, err
	}
	if q.AggregateMax {
		series = aggregateMaxSeries(q.App, series)
	}
	return series, nil
}

// aggregateMaxSeries collapses a per-instance series list to a single series
// holding the max of each input series' latest point — Render's
// aggregateAllMethod:"MAX" (used for *_LIMIT metrics: one "Limit" figure for
// the App, not one per replica).
func aggregateMaxSeries(app string, series []MetricSeries) []MetricSeries {
	if len(series) == 0 {
		return series
	}
	unit := series[0].Unit
	var maxVal float64
	var maxTs string
	for _, s := range series {
		if len(s.Points) == 0 {
			continue
		}
		p := s.Points[len(s.Points)-1]
		if p.Value > maxVal {
			maxVal = p.Value
			maxTs = p.Timestamp
		}
	}
	return []MetricSeries{{
		Labels: map[string]string{"resource": app},
		Unit:   unit,
		Points: []MetricPoint{{Timestamp: maxTs, Value: maxVal}},
	}}
}

// resourceMetric returns one series per replica (labels instance + resource) of
// CPU or memory usage: stepped history over the queried range when the ranged
// source (Prometheus) is wired, else a single current point (metrics-server
// snapshot). With Percentage set, each value is divided by that pod's limit
// (from the pod spec) and reported as a 0..100 percentage; a pod with no limit
// is skipped from percentage output (division is undefined).
func (c *Core) resourceMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	if c.ResourceMetricsRange != nil {
		return c.resourceMetricRange(ctx, q)
	}
	if c.ResourceMetrics == nil {
		return nil, ErrMetricsUnavailable
	}
	pods, err := c.appPods(ctx, q.App)
	if err != nil {
		return nil, err
	}
	usage, err := c.ResourceMetrics(ctx, c.Namespace, q.App)
	if err != nil {
		return nil, err
	}
	limits := podResourceLimits(pods, q.Metric) // by pod name

	unit := resourceUnit(q.Metric)
	if q.Percentage {
		unit = unitPercentage
	}
	nowStr := c.now().UTC().Format(time.RFC3339)

	out := make([]MetricSeries, 0, len(usage))
	for _, u := range usage {
		val := u.CPUCores
		if q.Metric == MetricMemory {
			val = u.MemoryBytes
		}
		if q.Percentage {
			lim, ok := limits[u.Pod]
			if !ok || lim <= 0 {
				continue // no limit => percentage is undefined; omit rather than fake
			}
			val = val / lim * 100
		}
		ts := u.Timestamp
		if ts == "" {
			ts = nowStr
		}
		out = append(out, MetricSeries{
			Labels: map[string]string{"resource": q.App, "instance": u.Pod},
			Unit:   unit,
			Points: []MetricPoint{{Timestamp: ts, Value: val}},
		})
	}
	// Stable instance order so output is deterministic across calls.
	sort.SliceStable(out, func(i, j int) bool { return out[i].Labels["instance"] < out[j].Labels["instance"] })
	return out, nil
}

// rangedResourceSeries funnels one resource-family metric through the ranged
// (Prometheus) source: it owns the MetricQuery→request mapping and the unit
// stamping, so the per-verb wrappers keep only their genuinely different
// fallbacks — the resource-family sibling of requestMetric's single funnel.
func (c *Core) rangedResourceSeries(ctx context.Context, q MetricQuery, metric, unit string) ([]MetricSeries, error) {
	series, err := c.ResourceMetricsRange(ctx, ResourceMetricsRangeRequest{
		Namespace:  c.Namespace,
		App:        q.App,
		Metric:     metric,
		Start:      q.Start,
		End:        q.End,
		Resolution: q.Resolution,
	})
	if err != nil {
		return nil, err
	}
	for i := range series {
		series[i].Unit = unit
	}
	return series, nil
}

// resourceMetricRange serves cpu/memory from the ranged (Prometheus) source:
// one stepped series per replica over [Start, End]. Percentage mode divides
// every point by the pod's current spec limit, keyed by the series' instance
// label — an instance with no limit (including a pod that no longer exists) is
// omitted rather than faked, exactly like the snapshot path.
func (c *Core) resourceMetricRange(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	unit := resourceUnit(q.Metric)
	if q.Percentage {
		unit = unitPercentage
	}
	series, err := c.rangedResourceSeries(ctx, q, q.Metric, unit)
	if err != nil {
		return nil, err
	}
	if q.Percentage && len(series) > 0 {
		pods, err := c.appPods(ctx, q.App)
		if err != nil {
			return nil, err
		}
		limits := podResourceLimits(pods, q.Metric)
		kept := series[:0]
		for _, s := range series {
			lim, ok := limits[s.Labels["instance"]]
			if !ok || lim <= 0 {
				continue
			}
			for i := range s.Points {
				s.Points[i].Value = s.Points[i].Value / lim * 100
			}
			kept = append(kept, s)
		}
		series = kept
	}
	sort.SliceStable(series, func(i, j int) bool { return series[i].Labels["instance"] < series[j].Labels["instance"] })
	return series, nil
}

// resourceLimitMetric returns one current series per replica of the pod's
// configured CPU/memory limit (from the pod spec, not metrics-server — so it
// needs no ResourceMetrics source and works even without metrics-server wired).
// Render's dashboard queries this alongside the raw cpu/memory metric and
// computes Percentage/Total client-side from the two, rather than asking bex-api
// to compute a percentage server-side (confirmed live: MEMORY and MEMORY_LIMIT
// are fetched with the same params regardless of which tab is selected). A pod
// with no limit configured is omitted — there's no value to report, not a zero.
func (c *Core) resourceLimitMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	pods, err := c.appPods(ctx, q.App)
	if err != nil {
		return nil, err
	}
	baseMetric := MetricCPU
	if q.Metric == MetricMemoryLimit {
		baseMetric = MetricMemory
	}
	unit := resourceUnit(q.Metric)
	limits := podResourceLimits(pods, baseMetric)
	nowStr := c.now().UTC().Format(time.RFC3339)

	out := make([]MetricSeries, 0, len(limits))
	for _, pod := range pods {
		lim, ok := limits[pod.Name]
		if !ok {
			continue
		}
		out = append(out, MetricSeries{
			Labels: map[string]string{"resource": q.App, "instance": pod.Name},
			Unit:   unit,
			Points: []MetricPoint{{Timestamp: nowStr, Value: lim}},
		})
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Labels["instance"] < out[j].Labels["instance"] })
	return out, nil
}

// instanceCountMetric returns the App's replica count: a stepped
// count-over-time series when the ranged (Prometheus) source is wired, else a
// single-series current count of the App's pods — the fallback needs no metrics
// source at all, so it works even on a cluster without metrics-server.
func (c *Core) instanceCountMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	if c.ResourceMetricsRange != nil {
		return c.rangedResourceSeries(ctx, q, MetricInstanceCount, unitCount)
	}
	pods, err := c.appPods(ctx, q.App)
	if err != nil {
		return nil, err
	}
	running := 0
	for i := range pods {
		if pods[i].Status.Phase != corev1.PodSucceeded && pods[i].Status.Phase != corev1.PodFailed {
			running++
		}
	}
	return []MetricSeries{{
		Labels: map[string]string{"resource": q.App},
		Unit:   unitCount,
		Points: []MetricPoint{{Timestamp: c.now().UTC().Format(time.RFC3339), Value: float64(running)}},
	}}, nil
}

// requestMetric delegates to the injected request-metrics source, resolving the
// query into the backend-neutral RequestMetricsRequest. The unit is set here so
// the adapters don't have to know the source's dimension conventions.
func (c *Core) requestMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	if c.RequestMetrics == nil {
		return nil, ErrMetricsUnavailable
	}
	series, err := c.RequestMetrics(ctx, RequestMetricsRequest{
		Namespace:  c.Namespace,
		App:        q.App,
		Metric:     q.Metric,
		Start:      q.Start,
		End:        q.End,
		Resolution: q.Resolution,
		Quantile:   q.Quantile,
		StatusCode: q.StatusCode,
		Host:       q.Host,
		Path:       q.Path,
		GroupBy:    q.GroupBy,
	})
	if err != nil {
		return nil, err
	}
	unit := requestUnit(q.Metric)
	for i := range series {
		if series[i].Unit == "" {
			series[i].Unit = unit
		}
	}
	return series, nil
}

// resourceUnit maps a resource metric to its absolute unit — the resource
// sibling of requestUnit. Percentage mode overrides it at the call sites.
func resourceUnit(metric string) string {
	if metric == MetricMemory || metric == MetricMemoryLimit {
		return unitBytes
	}
	return unitCores
}

func requestUnit(metric string) string {
	switch metric {
	case MetricHTTPLatency:
		return unitSeconds
	case MetricBandwidth:
		return unitBytes
	default: // http_requests
		return unitCount
	}
}

// appPods lists an App's replica pods (the controller's app.bex.co/app label) —
// the same selection the logs verb uses.
func (c *Core) appPods(ctx context.Context, app string) ([]corev1.Pod, error) {
	var pods corev1.PodList
	if err := c.Client.List(ctx, &pods,
		client.InNamespace(c.Namespace),
		client.MatchingLabels{podLabelApp: app}); err != nil {
		return nil, err
	}
	return pods.Items, nil
}

// podResourceLimits reads each pod's CPU (cores) or memory (bytes) limit, summed
// across containers, keyed by pod name. A pod with no limit on the metric is
// absent from the map (percentage mode skips it).
func podResourceLimits(pods []corev1.Pod, metric string) map[string]float64 {
	out := map[string]float64{}
	for i := range pods {
		var total float64
		var found bool
		for _, ctr := range pods[i].Spec.Containers {
			if metric == MetricMemory {
				if v, has := ctr.Resources.Limits[corev1.ResourceMemory]; has {
					total += float64(v.Value())
					found = true
				}
			} else {
				if v, has := ctr.Resources.Limits[corev1.ResourceCPU]; has {
					total += v.AsApproximateFloat64()
					found = true
				}
			}
		}
		if found {
			out[pods[i].Name] = total
		}
	}
	return out
}

// --- monthToDateBandwidth (Render's dashboard GraphQL, captured live) ---

// MonthToDateBandwidth is bex's answer to Render's monthToDateBandwidth query —
// a cumulative bandwidth figure for the current calendar month (the "Usage
// this month" footer under Outbound Bandwidth). Render tracks several egress
// paths (HTTP, NAT gateway, private link, websocket); bex only has
// Traefik-scraped HTTP egress, so only the HTTP figure is real — the others
// are always 0, a documented subset rather than a fabricated total.
type MonthToDateBandwidth struct {
	EgressBandwidthMB            float64
	HTTPEgressBandwidthMB        float64
	NATEgressBandwidthMB         float64
	PrivateLinkEgressBandwidthMB float64
	WebsocketEgressBandwidthMB   float64
}

// MonthToDateBandwidthSource returns an App's cumulative HTTP egress in bytes
// since the given time. nil => MonthToDateBandwidth reports
// ErrMetricsUnavailable. Production wires it via NewMonthToDateBandwidthSource
// (a Prometheus increase() query); tests fake it.
type MonthToDateBandwidthSource func(ctx context.Context, namespace, app string, since time.Time) (bytesTotal float64, err error)

// MonthToDateBandwidth returns the App's month-to-date bandwidth usage.
// ACCURACY NOTE: bex's Prometheus retains only a few hours of history (see
// deploy/gitops/base/prometheus.yaml) — a real calendar-month figure needs
// retention to match, which this PoC-scale deployment doesn't have. The query
// itself is real (a genuine increase() over the elapsed month), it just
// under-counts on a short-retention Prometheus; extending retention is a
// deploy-config change, not a code change.
func (c *Core) MonthToDateBandwidth(ctx context.Context, app string) (MonthToDateBandwidth, error) {
	if err := c.authorize(ctx, relCanView); err != nil {
		return MonthToDateBandwidth{}, err
	}
	if _, err := c.fetch(ctx, app); err != nil {
		return MonthToDateBandwidth{}, err
	}
	if c.MonthToDateBandwidthSource == nil {
		return MonthToDateBandwidth{}, ErrMetricsUnavailable
	}
	now := c.now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	bytesTotal, err := c.MonthToDateBandwidthSource(ctx, c.Namespace, app, monthStart)
	if err != nil {
		return MonthToDateBandwidth{}, err
	}
	mb := bytesTotal / (1 << 20)
	return MonthToDateBandwidth{EgressBandwidthMB: mb, HTTPEgressBandwidthMB: mb}, nil
}

// --- metricsFilters (Status Code/Host/Path filter-dropdown population) ---

// MetricsFiltersQuery asks which values are available for a set of output
// filter fields (Render's outputFilters — e.g. RESOURCE, INSTANCE, BUILD for
// application-type filters; HOST, STATUS_CODE for HTTP-type filters), for one
// App.
// filterFieldResource is the "RESOURCE" output-filter/query-filter field name,
// shared between Core.MetricsFilters and graphql.go's filter-array parsing.
const filterFieldResource = "RESOURCE"

type MetricsFiltersQuery struct {
	App           string
	OutputFilters []string
}

// MetricsFilterValues is one {field, values} entry of a MetricsFilters result.
type MetricsFilterValues struct {
	Field  string
	Values []string
}

// MetricsFilterValuesSource discovers a Prometheus label's observed values
// (e.g. the `code` label backing STATUS_CODE) for an App's request metrics.
// nil => that field's values come back empty rather than erroring — unlike
// ResourceMetrics/RequestMetrics, a missing source here isn't fatal (the UI
// just shows no suggestions for that one field, matching bex's declared
// filter capability rather than reporting an outage). Production wires it via
// NewPrometheusFilterValuesSource.
type MetricsFilterValuesSource func(ctx context.Context, namespace, app, label string) ([]string, error)

// MetricsFilters resolves available values for each requested output filter.
// RESOURCE/INSTANCE/HOST are answered from data Core already has (no source
// needed); STATUS_CODE needs MetricsFilterValuesSource; BUILD and PATH have no
// bex equivalent and always report empty — an honest gap, not a guess.
func (c *Core) MetricsFilters(ctx context.Context, q MetricsFiltersQuery) ([]MetricsFilterValues, error) {
	if err := c.authorize(ctx, relCanView); err != nil {
		return nil, err
	}
	a, err := c.fetch(ctx, q.App)
	if err != nil {
		return nil, err
	}

	out := make([]MetricsFilterValues, 0, len(q.OutputFilters))
	for _, field := range q.OutputFilters {
		switch field {
		case filterFieldResource:
			out = append(out, MetricsFilterValues{Field: field, Values: []string{q.App}})
		case "INSTANCE":
			pods, err := c.appPods(ctx, q.App)
			if err != nil {
				return nil, err
			}
			instances := make([]string, 0, len(pods))
			for _, p := range pods {
				instances = append(instances, p.Name)
			}
			out = append(out, MetricsFilterValues{Field: field, Values: instances})
		case "HOST":
			out = append(out, MetricsFilterValues{Field: field, Values: hostsFromURLs(a.Status.URLs)})
		case "STATUS_CODE":
			values, err := c.filterValuesOrEmpty(ctx, q.App, "code")
			if err != nil {
				return nil, err
			}
			out = append(out, MetricsFilterValues{Field: field, Values: values})
		default: // BUILD, PATH — no bex equivalent
			out = append(out, MetricsFilterValues{Field: field, Values: []string{}})
		}
	}
	return out, nil
}

func (c *Core) filterValuesOrEmpty(ctx context.Context, app, label string) ([]string, error) {
	if c.MetricsFilterValuesSource == nil {
		return []string{}, nil
	}
	return c.MetricsFilterValuesSource(ctx, c.Namespace, app, label)
}

// hostsFromURLs strips the scheme (and any trailing slash) from an App's
// status URLs, matching Render's HOST filter's bare-hostname vocabulary.
func hostsFromURLs(urls []string) []string {
	out := make([]string, 0, len(urls))
	for _, u := range urls {
		host := u
		if _, after, ok := strings.Cut(u, "://"); ok {
			host = after
		}
		out = append(out, strings.TrimSuffix(host, "/"))
	}
	return out
}
