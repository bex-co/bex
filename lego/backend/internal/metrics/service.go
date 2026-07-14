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

// Package metrics is the metrics feature: one Service the REST + GraphQL + MCP
// adapters share, reaching its backends through injected sources so the domain
// stays clientset-free. Resource metrics (cpu/memory/instance_count) come from
// cAdvisor-over-Prometheus via ResourceMetricsRange (stepped history) when it's
// wired, else from metrics-server via ResourceMetrics (single current point) —
// instance count in that fallback is derived from the App's pods directly.
// Request metrics (count/latency/bandwidth) come from Traefik-over-Prometheus.
// Shapes are Render's metrics-API time-series.
package metrics

import (
	"context"
	"fmt"
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// Metric ids — bex's canonical names (underscored). They map 1:1 to Render's
// metrics endpoints in the REST adapter and to Render's dashboard GraphQL `name`
// enum.
const (
	MetricCPU           = "cpu"
	MetricCPULimit      = "cpu_limit"
	MetricMemory        = "memory"
	MetricMemoryLimit   = "memory_limit"
	MetricInstanceCount = "instance_count"
	MetricHTTPRequests  = "http_requests"
	MetricHTTPLatency   = "http_latency"
	MetricBandwidth     = "bandwidth"
	// MetricCPUTarget/MetricMemoryTarget are bex extensions (w3/m10, w1/m20's
	// autoscaling config): the App's configured target utilization percentage,
	// alongside its current cpu/memory usage. No Render equivalent.
	MetricCPUTarget    = "cpu_target"
	MetricMemoryTarget = "memory_target"
)

// Metric units (the `unit` field of a Render time-series).
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

// filterFieldResource is the "RESOURCE" output-filter/query-filter field name,
// shared between MetricsFilters and graphql.go's filter-array parsing.
const filterFieldResource = "RESOURCE"

// MetricPoint is one (timestamp, value) sample of a series.
type MetricPoint struct {
	Timestamp string  `json:"timestamp"`
	Value     float64 `json:"value"`
}

// MetricSeries is one labeled time-series — Render's metrics shape.
type MetricSeries struct {
	Labels map[string]string `json:"labels,omitempty"`
	Unit   string            `json:"unit"`
	Points []MetricPoint     `json:"points"`
}

// MetricQuery is the resolved request for the Metrics verb, shared by every
// adapter. Metric selects which series to return; the rest are filters/options.
type MetricQuery struct {
	App        string
	Metric     string        // one of the Metric* ids
	Start, End time.Time     // request-metric time range (zero => End=now, Start=End-1h)
	Resolution time.Duration // request-metric step (<=0 => 1m)
	Quantile   float64       // http_latency percentile 0..1 (<=0 => .95)
	Percentage bool          // cpu/memory as a fraction of the pod limit instead of absolute
	StatusCode string        // request filter: "2xx" | "5xx" | "500" | ""
	Host       string        // request filter
	Path       string        // request filter
	GroupBy    string        // request group-by: "status" | "method" | "instance" | ""
	// AggregateMax collapses a per-instance series (cpu_limit/memory_limit) down
	// to one series holding the max value across instances — Render's dashboard
	// GraphQL requests this via aggregateAllMethod:"MAX" (captured live).
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
// snapshot. CPUCores is fractional vCPU, MemoryBytes is bytes.
type PodResourceUsage struct {
	Pod         string
	Timestamp   string
	CPUCores    float64
	MemoryBytes float64
}

// ResourceMetricsSource returns the current per-pod usage for an App's pods. nil
// => cpu/memory metrics report core.ErrMetricsUnavailable (unless
// ResourceMetricsRange serves them).
type ResourceMetricsSource func(ctx context.Context, namespace, app string) ([]PodResourceUsage, error)

// ResourceMetricsRangeRequest is the backend-neutral ranged resource-metric ask
// handed to a ResourceMetricsRangeSource — the resource sibling of
// RequestMetricsRequest. The source owns the query language; the service only
// resolves defaults and delegates.
type ResourceMetricsRangeRequest struct {
	Namespace  string
	App        string
	Metric     string // cpu | memory | instance_count
	Start, End time.Time
	Resolution time.Duration
}

// ResourceMetricsRangeSource returns stepped resource time-series for an App over
// a time range (cAdvisor scraped by Prometheus) — per-instance series for
// cpu/memory, a single series for instance_count. When non-nil, the service
// prefers it over the snapshot ResourceMetrics source; nil => cpu/memory come
// from ResourceMetrics and instance_count from counting pods. Production wires it
// via NewPrometheusResourceSource.
type ResourceMetricsRangeSource func(ctx context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error)

// RequestMetricsRequest is the backend-neutral request-metric ask.
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
// metrics backend. nil => request metrics report core.ErrMetricsUnavailable.
type RequestMetricsSource func(ctx context.Context, req RequestMetricsRequest) ([]MetricSeries, error)

// MonthToDateBandwidthSource returns an App's cumulative HTTP egress in bytes
// since the given time. nil => MonthToDateBandwidth reports
// core.ErrMetricsUnavailable.
type MonthToDateBandwidthSource func(ctx context.Context, namespace, app string, since time.Time) (bytesTotal float64, err error)

// MetricsFilterValuesSource discovers a Prometheus label's observed values (e.g.
// the `code` label backing STATUS_CODE) for an App's request metrics. nil =>
// that field's values come back empty rather than erroring.
type MetricsFilterValuesSource func(ctx context.Context, namespace, app, label string) ([]string, error)

// Service is the single metrics read every adapter calls, plus the dashboard's
// month-to-date bandwidth and filter-population helpers.
type Service struct {
	*core.Base
	// ResourceMetrics reads current per-pod CPU/memory (metrics-server); nil =>
	// the cpu/memory metrics report core.ErrMetricsUnavailable (unless
	// ResourceMetricsRange serves them).
	ResourceMetrics ResourceMetricsSource
	// ResourceMetricsRange reads stepped cpu/memory/instance-count history
	// (cAdvisor via Prometheus); non-nil => preferred over ResourceMetrics and the
	// pod-count fallback.
	ResourceMetricsRange ResourceMetricsRangeSource
	// RequestMetrics reads request time-series (Traefik via Prometheus); nil =>
	// http_requests/http_latency/bandwidth report core.ErrMetricsUnavailable.
	RequestMetrics RequestMetricsSource
	// MonthToDateBandwidthSource reads cumulative HTTP egress since a given time.
	MonthToDateBandwidthSource MonthToDateBandwidthSource
	// MetricsFilterValuesSource discovers a Prometheus label's observed values.
	MetricsFilterValuesSource MetricsFilterValuesSource
	// DiskUsage reads a managed Database/KeyValue's backing-PVC usage/capacity
	// (w3/m10); nil => disk/disk_capacity report core.ErrMetricsUnavailable.
	DiskUsage DiskUsageSource
	// DBConnections reads a managed Postgres instance's active-connection
	// history (w3/m10, CNPG's exporter); nil => db_connections reports
	// core.ErrMetricsUnavailable.
	DBConnections DBConnectionsSource
	// ReplicationLag reads a managed Postgres instance's replication-lag
	// history (w3/m10, CNPG's exporter) — reached only once a Database's
	// HighAvailabilityEnabled is true (w1/m22); nil => replication_lag reports
	// core.ErrMetricsUnavailable for an HA instance instead of silently 503ing
	// on every instance regardless of HA state.
	ReplicationLag ReplicationLagSource
	// KeyValueStats reads a managed Key Value (Valkey) instance's used-memory /
	// connected-clients history (redis_exporter via Prometheus, w5/011); nil =>
	// kv_memory/kv_connections report core.ErrMetricsUnavailable.
	KeyValueStats KeyValueStatsSource
	// MaxQueryHours, when positive, caps the start–end window accepted by REST
	// metrics queries (GET /v1/metrics/*). 0 = unlimited.
	MaxQueryHours int
}

// Metrics is the single metrics read every adapter (REST + GraphQL) calls. It
// fails with core.ErrNotFound for an unknown App, dispatches on q.Metric, and
// returns Render-shaped series.
func (s *Service) Metrics(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	app, err := s.AuthorizeApp(ctx, core.RelCanView, q.App)
	if err != nil {
		return nil, err // ErrNotFound for unknown apps, exactly like Get
	}
	q = q.normalized(s.Now())
	var series []MetricSeries
	switch q.Metric {
	case MetricCPU, MetricMemory:
		series, err = s.resourceMetric(ctx, q)
	case MetricCPULimit, MetricMemoryLimit:
		series, err = s.resourceLimitMetric(ctx, q)
	case MetricInstanceCount:
		series, err = s.instanceCountMetric(ctx, q)
	case MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth:
		series, err = s.requestMetric(ctx, q)
	case MetricCPUTarget, MetricMemoryTarget:
		series = autoscaleTargetMetric(app, q, s.Now())
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
// holding the max of each input series' latest point.
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
// snapshot). With Percentage set, each value is divided by that pod's limit (from
// the pod spec) and reported as a 0..100 percentage; a pod with no limit is
// skipped from percentage output (division is undefined).
func (s *Service) resourceMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	if s.ResourceMetricsRange != nil {
		return s.resourceMetricRange(ctx, q)
	}
	if s.ResourceMetrics == nil {
		return nil, core.ErrMetricsUnavailable
	}
	pods, err := s.AppPods(ctx, q.App)
	if err != nil {
		return nil, err
	}
	usage, err := s.ResourceMetrics(ctx, s.Namespace, q.App)
	if err != nil {
		return nil, err
	}
	limits := podResourceLimits(pods, q.Metric) // by pod name

	unit := resourceUnit(q.Metric)
	if q.Percentage {
		unit = unitPercentage
	}
	nowStr := s.Now().UTC().Format(time.RFC3339)

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
	sort.SliceStable(out, func(i, j int) bool { return out[i].Labels["instance"] < out[j].Labels["instance"] })
	return out, nil
}

// rangedResourceSeries funnels one resource-family metric through the ranged
// (Prometheus) source: it owns the MetricQuery→request mapping and the unit
// stamping, so the per-verb wrappers keep only their genuinely different
// fallbacks — the resource-family sibling of requestMetric's single funnel.
func (s *Service) rangedResourceSeries(ctx context.Context, q MetricQuery, metric, unit string) ([]MetricSeries, error) {
	series, err := s.ResourceMetricsRange(ctx, ResourceMetricsRangeRequest{
		Namespace:  s.Namespace,
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

// resourceMetricRange serves cpu/memory from the ranged (Prometheus) source: one
// stepped series per replica over [Start, End]. Percentage mode divides every
// point by the pod's current spec limit, keyed by the series' instance label — an
// instance with no limit (including a pod that no longer exists) is omitted
// rather than faked, exactly like the snapshot path.
func (s *Service) resourceMetricRange(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	unit := resourceUnit(q.Metric)
	if q.Percentage {
		unit = unitPercentage
	}
	series, err := s.rangedResourceSeries(ctx, q, q.Metric, unit)
	if err != nil {
		return nil, err
	}
	if q.Percentage && len(series) > 0 {
		pods, err := s.AppPods(ctx, q.App)
		if err != nil {
			return nil, err
		}
		limits := podResourceLimits(pods, q.Metric)
		kept := series[:0]
		for _, ser := range series {
			lim, ok := limits[ser.Labels["instance"]]
			if !ok || lim <= 0 {
				continue
			}
			for i := range ser.Points {
				ser.Points[i].Value = ser.Points[i].Value / lim * 100
			}
			kept = append(kept, ser)
		}
		series = kept
	}
	sort.SliceStable(series, func(i, j int) bool { return series[i].Labels["instance"] < series[j].Labels["instance"] })
	return series, nil
}

// resourceLimitMetric returns one current series per replica of the pod's
// configured CPU/memory limit (from the pod spec, not metrics-server).
func (s *Service) resourceLimitMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	pods, err := s.AppPods(ctx, q.App)
	if err != nil {
		return nil, err
	}
	baseMetric := MetricCPU
	if q.Metric == MetricMemoryLimit {
		baseMetric = MetricMemory
	}
	unit := resourceUnit(q.Metric)
	limits := podResourceLimits(pods, baseMetric)
	nowStr := s.Now().UTC().Format(time.RFC3339)

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

// autoscaleTargetMetric returns the App's configured autoscale target
// utilization (w1/m20, App.spec.autoscaling) as a single current-value point —
// a static config value, like cpu_limit/memory_limit, not a usage sample.
// Omitted (empty, not a fake zero) when autoscaling is disabled or the specific
// target isn't configured, exactly like resourceLimitMetric's no-limit case.
// Takes the App Metrics already fetched (not a second s.GetApp round-trip).
func autoscaleTargetMetric(app *appv1alpha1.App, q MetricQuery, now time.Time) []MetricSeries {
	as := app.Spec.Autoscaling
	if as == nil || !as.Enabled {
		return nil
	}
	target := as.TargetCPUPercent
	if q.Metric == MetricMemoryTarget {
		target = as.TargetMemoryPercent
	}
	if target == nil {
		return nil
	}
	return []MetricSeries{{
		Labels: map[string]string{"resource": q.App},
		Unit:   unitPercentage,
		Points: []MetricPoint{{Timestamp: now.UTC().Format(time.RFC3339), Value: float64(*target)}},
	}}
}

// instanceCountMetric returns the App's replica count: a stepped count-over-time
// series when the ranged (Prometheus) source is wired, else a single-series
// current count of the App's pods — the fallback needs no metrics source at all,
// so it works even on a cluster without metrics-server.
func (s *Service) instanceCountMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	if s.ResourceMetricsRange != nil {
		return s.rangedResourceSeries(ctx, q, MetricInstanceCount, unitCount)
	}
	pods, err := s.AppPods(ctx, q.App)
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
		Points: []MetricPoint{{Timestamp: s.Now().UTC().Format(time.RFC3339), Value: float64(running)}},
	}}, nil
}

// requestMetric delegates to the injected request-metrics source.
func (s *Service) requestMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	if s.RequestMetrics == nil {
		return nil, core.ErrMetricsUnavailable
	}
	series, err := s.RequestMetrics(ctx, RequestMetricsRequest{
		Namespace:  s.Namespace,
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

// resourceUnit maps a resource metric to its absolute unit — the resource sibling
// of requestUnit. Percentage mode overrides it at the call sites.
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

// podResourceLimits reads each pod's CPU (cores) or memory (bytes) limit, summed
// across containers, keyed by pod name. A pod with no limit is absent.
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
// a cumulative bandwidth figure for the current calendar month. Only the HTTP
// figure is real (Traefik-scraped); the others are always 0, a documented subset.
type MonthToDateBandwidth struct {
	EgressBandwidthMB            float64
	HTTPEgressBandwidthMB        float64
	NATEgressBandwidthMB         float64
	PrivateLinkEgressBandwidthMB float64
	WebsocketEgressBandwidthMB   float64
}

// MonthToDateBandwidth returns the App's month-to-date bandwidth usage. The query
// is real (increase() over the elapsed month); a short-retention Prometheus just
// under-counts (see ADR010-observability.md).
func (s *Service) MonthToDateBandwidth(ctx context.Context, app string) (MonthToDateBandwidth, error) {
	if _, err := s.AuthorizeApp(ctx, core.RelCanView, app); err != nil {
		return MonthToDateBandwidth{}, err
	}
	if s.MonthToDateBandwidthSource == nil {
		return MonthToDateBandwidth{}, core.ErrMetricsUnavailable
	}
	now := s.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	bytesTotal, err := s.MonthToDateBandwidthSource(ctx, s.Namespace, app, monthStart)
	if err != nil {
		return MonthToDateBandwidth{}, err
	}
	mb := bytesTotal / (1 << 20)
	return MonthToDateBandwidth{EgressBandwidthMB: mb, HTTPEgressBandwidthMB: mb}, nil
}

// --- metricsFilters (Status Code/Host/Path filter-dropdown population) ---

// MetricsFiltersQuery asks which values are available for a set of output filter
// fields, for one App.
type MetricsFiltersQuery struct {
	App           string
	OutputFilters []string
}

// MetricsFilterValues is one {field, values} entry of a MetricsFilters result.
type MetricsFilterValues struct {
	Field  string
	Values []string
}

// MetricsFilters resolves available values for each requested output filter.
// RESOURCE/INSTANCE/HOST are answered from data the service already has;
// STATUS_CODE needs MetricsFilterValuesSource; BUILD/PATH always report empty.
func (s *Service) MetricsFilters(ctx context.Context, q MetricsFiltersQuery) ([]MetricsFilterValues, error) {
	a, err := s.AuthorizeApp(ctx, core.RelCanView, q.App)
	if err != nil {
		return nil, err
	}

	out := make([]MetricsFilterValues, 0, len(q.OutputFilters))
	for _, field := range q.OutputFilters {
		switch field {
		case filterFieldResource:
			out = append(out, MetricsFilterValues{Field: field, Values: []string{q.App}})
		case "INSTANCE":
			pods, err := s.AppPods(ctx, q.App)
			if err != nil {
				return nil, err
			}
			instances := make([]string, 0, len(pods))
			for _, p := range pods {
				instances = append(instances, p.Name)
			}
			out = append(out, MetricsFilterValues{Field: field, Values: instances})
		case "HOST":
			out = append(out, MetricsFilterValues{Field: field, Values: core.HostsFromURLs(a.Status.URLs)})
		case "STATUS_CODE":
			values, err := s.filterValuesOrEmpty(ctx, q.App, "code")
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

func (s *Service) filterValuesOrEmpty(ctx context.Context, app, label string) ([]string, error) {
	if s.MetricsFilterValuesSource == nil {
		return []string{}, nil
	}
	return s.MetricsFilterValuesSource(ctx, s.Namespace, app, label)
}
