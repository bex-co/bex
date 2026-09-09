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
	"strconv"
	"time"

	corev1 "k8s.io/api/core/v1"

	"github.com/bex-co/bex/lego/backend/internal/core"
	ids "github.com/bex-co/bex/lego/backend/internal/id"
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

// Render's filter/aggregate field vocabulary, shared between MetricsFilters and
// graphql.go's filter-array parsing. HOST and PATH are named so the parser can
// recognize (and Metrics can then refuse) them; INSTANCE is offered by
// MetricsFilters only.
const (
	filterFieldResource   = "RESOURCE"
	filterFieldStatusCode = "STATUS_CODE"
	filterFieldMethod     = "METHOD"
	filterFieldHost       = "HOST"
	filterFieldPath       = "PATH"
	filterFieldInstance   = "INSTANCE"
)

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

// SetLabel sets one label, allocating the map on first use. Series arrive from
// Prometheus with a nil Labels map whenever aggregation dropped every selector,
// so every caller that tags a series has to handle that — this is the one place
// that does.
func (s *MetricSeries) SetLabel(key, value string) {
	if s.Labels == nil {
		s.Labels = map[string]string{}
	}
	s.Labels[key] = value
}

// setLabelOnEach tags every series in a result set with one label.
func setLabelOnEach(series []MetricSeries, key, value string) {
	for i := range series {
		series[i].SetLabel(key, value)
	}
}

// projectInstanceLabels rewrites each series' raw pod/PVC name into the
// canonical public instance id (w5/m87). Internal Prom/cAdvisor matching still
// uses the Kubernetes name; only the public label changes. Series without an
// instance label (aggregated counts, request totals) are left untouched.
func projectInstanceLabels(resourceID string, series []MetricSeries) {
	if resourceID == "" {
		return
	}
	for i := range series {
		pod := series[i].Labels["instance"]
		if pod == "" {
			continue
		}
		series[i].SetLabel("instance", ids.ServiceInstanceID(resourceID, pod))
	}
}

// MetricQuery is the resolved request for the Metrics verb, shared by every
// adapter. Metric selects which series to return; the rest are filters/options.
type MetricQuery struct {
	App        string
	Metric     string        // one of the Metric* ids
	Start, End time.Time     // request-metric time range (zero => End=now, Start=End-1h)
	Resolution time.Duration // request-metric step (<=0 => 1m)
	Quantile   float64       // http_latency percentile 0..1 (<=0 => .95)
	// Quantiles, when non-empty, requests several http_latency percentiles in one
	// read (the dashboard's percentile "All" overlay, w5/m56) — the multi-quantile
	// sibling of Quantile. Only MetricsWithQuantiles consults it; the single-quantile
	// Metrics verb ignores it, so every existing caller is byte-identical.
	Quantiles  []float64
	Percentage bool   // cpu/memory as a fraction of the pod limit instead of absolute
	StatusCode string // request filter: "2xx" | "5xx" | "500" | ""
	// Host/Path carry Render's host/path request filters. Traefik's Prometheus
	// counters (service- and router-level) carry no host or path axis — even with
	// addRoutersLabels the router label is the router NAME, not the matched
	// Host()/Path() — so when either is set the request-metrics read is served
	// from the request-log store (Loki) instead of Prometheus (w5/m58): the
	// Traefik access log carries RequestHost/RequestPath per line. They apply
	// only to http_requests/http_latency (bandwidth has no per-request host/path
	// axis → ErrBadRequest); with the Loki source unwired they surface
	// core.ErrLogStoreUnavailable, never a silently-unfiltered Prometheus result.
	Host    string
	Path    string
	GroupBy string // request group-by: "status" | "method" | "instance" | ""
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
	Namespace string
	App       string
	AppID     string
	// Port is the App's effective backend port (app.Spec.EffectivePort()) — with
	// Namespace and App it reconstructs the exact Traefik `service` metric label
	// (codex-security round-19 #6; see traefikServiceLabel in source.go).
	Port   int32
	Direct bool
	// Routers are the exact Traefik router metric labels for bandwidth. The
	// service resolves them from the authorized App's actual Ingress; request
	// count and latency retain their existing service-level selector.
	Routers    []string
	Metric     string // http_requests | http_latency | bandwidth
	Start, End time.Time
	Resolution time.Duration
	Quantile   float64
	StatusCode string
	GroupBy    string
	// Host/Path are the per-request filters served by the request-log backend
	// (NewLokiRequestMetricsSource, w5/m58). The Prometheus source ignores them
	// (a host/path read is never routed to it); the Loki source turns them into
	// LogQL line filters over the Traefik access log's RequestHost/RequestPath.
	Host string
	Path string
}

// RequestMetricsSource returns request time-series for an App from the ingress
// metrics backend. nil => request metrics report core.ErrMetricsUnavailable.
type RequestMetricsSource func(ctx context.Context, req RequestMetricsRequest) ([]MetricSeries, error)

// BandwidthBytes is the source-level byte breakdown shared by the expanded
// total and Render-shaped month-to-date category fields.
type BandwidthBytes struct {
	HTTP, NAT, WebSocket float64
}

// MonthToDateBandwidthSource returns all applicable App egress categories in
// bytes since the given time, plus the names of sources whose health product
// failed inside the window (best-effort — ADR023 § Observability reads vs
// billing reads, w1/m50; only a transport failure rejects the result).
type MonthToDateBandwidthSource func(ctx context.Context, appID string, routers []string, direct bool, since, at time.Time) (BandwidthBytes, []string, error)

// MetricsFilterValuesSource discovers a Prometheus label's observed values (e.g.
// the `code` label backing STATUS_CODE) for an App's request metrics. nil =>
// that field's values come back empty rather than erroring.
type MetricsFilterValuesSource func(ctx context.Context, namespace, app string, port int32, label string) ([]string, error)

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
	// RequestLogMetrics reads host/path-filtered request time-series from the
	// request-log store (Traefik access log via Loki, w5/m58) — the only backend
	// that carries a per-request host/path axis. Reached only when a MetricQuery
	// sets Host or Path; nil (BEX_LOKI_URL unset) => a host/path-filtered read
	// reports core.ErrLogStoreUnavailable (the Logs-tab 503 pattern), never a
	// silently-unfiltered Prometheus result. Same signature as RequestMetrics.
	RequestLogMetrics RequestMetricsSource
	// MonthToDateBandwidthSource reads cumulative composed egress since a given time.
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
	// MaxQueryHours, when positive, caps the start–end window accepted by every
	// metrics read. Enforced in the shared service (Metrics/DatastoreMetrics),
	// so REST, GraphQL, and MCP cannot drift (codex r7 #13 — it previously
	// lived in the REST adapter alone, letting GraphQL/MCP scan the backends
	// unbounded). 0 = unlimited.
	MaxQueryHours int
}

// Fan-out budgets (codex r7 #13 / w4/029 #14): one API request multiplies
// into len(resources) × len(metricTypes) service reads, each of which is one
// or more Prometheus/Loki queries — and http_latency multiplies again per
// requested percentile. The caps bound that product far above any dashboard
// use (a handful of services × one metric × three percentiles) while keeping
// a single request from becoming an unbounded backend scan.
const (
	maxMetricsFanOut    = 100 // (resource, metric) pairs per request
	maxLatencyQuantiles = 10  // http_latency percentiles per request
	// maxPointsPerSeries bounds window/resolution so a caller cannot pin a
	// 1-second step across the full BEX_MAX_QUERY_HOURS window (round-5 finding
	// 2). checkWindow limits elapsed hours only; without this, a viewer could
	// force millions of PromQL/LogQL evaluation points per series on the shared
	// observability plane. 11,000 matches Prometheus's own query_range
	// points-per-series hard cap, so nothing that Prometheus serves today is
	// newly rejected — an over-budget request is refused here with a clear
	// "raise resolutionSeconds" message instead of an opaque upstream 400.
	maxPointsPerSeries = 11000
)

// checkWindow enforces MaxQueryHours against a query's start–end range,
// called by the shared service entry points so every adapter inherits it.
func (s *Service) checkWindow(start, end time.Time) error {
	return core.CheckQueryWindow(s.MaxQueryHours, s.Now, start, end)
}

// latencyFan is the percentile multiplier for the fan-out budget: only an
// http_latency read expands per requested quantile (MetricsWithQuantiles
// ignores the list for every other metric), so a non-latency request never
// pays for percentiles it will not read.
func latencyFan(metric string, quantiles []float64) int {
	if metric != MetricHTTPLatency {
		return 1
	}
	return len(normalizeQuantiles(quantiles))
}

// checkFanOut bounds the (resource, metric, latency-percentile) product one
// request may expand into, called by the REST/GraphQL/MCP adapters before
// their read loops — the array dimensions exist only adapter-side. Every
// dimension clamps to one so an absent axis counts as a single read.
func checkFanOut(resources, metrics, quantiles int) error {
	reads := max(resources, 1) * max(metrics, 1) * max(quantiles, 1)
	if reads > maxMetricsFanOut {
		return fmt.Errorf("%w: request expands into %d metric reads; the limit is %d", core.ErrBadRequest, reads, maxMetricsFanOut)
	}
	return nil
}

// checkPointBudget bounds the per-series sample count (window ÷ resolution) a
// single read may request, called by the shared service entry points after
// normalization so every adapter inherits it (round-5 finding 2). resolution is
// always positive after normalized(); a zero step is impossible here.
func checkPointBudget(start, end time.Time, resolution time.Duration) error {
	// Check again after defaults resolve an omitted end to now: a future start
	// otherwise becomes an inverted range only during normalization.
	if err := core.ValidateQueryRange(start, end); err != nil {
		return err
	}
	if resolution <= 0 {
		return nil // defensive: normalized() guarantees a positive step
	}
	if points := int64(end.Sub(start) / resolution); points > maxPointsPerSeries {
		return fmt.Errorf("%w: window ÷ resolution yields %d samples per series; the limit is %d — raise resolutionSeconds", core.ErrBadRequest, points, maxPointsPerSeries)
	}
	return nil
}

// Metrics is the single metrics read every adapter (REST, GraphQL, MCP) calls. It
// fails with core.ErrNotFound for an unknown App, dispatches on q.Metric, and
// returns Render-shaped series.
func (s *Service) Metrics(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
	// Window first, authorization second — preserves the REST adapter's
	// historical order (an over-window request 400s before an unknown app 404s).
	if err := s.checkWindow(q.Start, q.End); err != nil {
		return nil, err
	}
	app, err := s.AuthorizeApp(ctx, core.RelCanView, q.App)
	if err != nil {
		return nil, err // ErrNotFound for unknown apps, exactly like Get
	}
	statusCode, ok := normalizeStatusCodeFilter(q.StatusCode)
	if !ok {
		return nil, fmt.Errorf("%w: statusCode must be a three-digit HTTP status or a class such as 5xx", core.ErrBadRequest)
	}
	q.StatusCode = statusCode
	if q.Host != "" || q.Path != "" {
		// host/path narrow request traffic only — they have no meaning for a
		// resource metric, and bandwidth (egress L3/L7) carries no per-request
		// host/path axis. Reject anything but the two request metrics up front,
		// with a named error, before touching a store (w5/m58).
		if q.Metric != MetricHTTPRequests && q.Metric != MetricHTTPLatency {
			return nil, fmt.Errorf("%w: host/path filters apply only to http_requests and http_latency", core.ErrBadRequest)
		}
	}
	q = q.normalized(s.Now())
	if err := checkPointBudget(q.Start, q.End, q.Resolution); err != nil {
		return nil, err
	}
	var series []MetricSeries
	switch q.Metric {
	case MetricCPU, MetricMemory:
		series, err = s.resourceMetric(ctx, q, app)
	case MetricCPULimit, MetricMemoryLimit:
		series, err = s.resourceLimitMetric(ctx, q, app)
	case MetricInstanceCount:
		series, err = s.instanceCountMetric(ctx, q, app)
	case MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth:
		series, err = s.requestMetric(ctx, q, app)
	case MetricCPUTarget, MetricMemoryTarget:
		series = autoscaleTargetMetric(app, q, s.Now())
	default:
		return nil, fmt.Errorf("%w: unknown metric %q", core.ErrBadRequest, q.Metric)
	}
	if err != nil {
		return nil, err
	}
	if q.AggregateMax {
		series = aggregateMaxSeries(q.App, series)
	}
	return series, nil
}

// QuantileSeries is a MetricSeries paired with the http_latency percentile that
// produced it — MetricsWithQuantiles' unit so an adapter can echo the quantile
// per series (GraphQL's `parameters { quantile }`, or the `quantile` label the
// REST/MCP surfaces already carry alongside `resource`/`metric`). HasQuantile is
// false for every non-latency metric (quantile carries no meaning there).
type QuantileSeries struct {
	MetricSeries
	Quantile    float64
	HasQuantile bool
}

// MetricsWithQuantiles is Metrics plus http_latency multi-quantile fan-out: when
// q.Metric is http_latency and q.Quantiles names several percentiles, it reads
// each quantile in one pass and returns every series tagged (labels + the paired
// Quantile) with the percentile that produced it — feeding the dashboard's
// percentile "All" overlay (w5/m56). For every other metric, and for a single
// quantile, it is Metrics with the resolved quantile echoed on latency and the
// series left byte-identical (no added label). All of Metrics' auth / normalize /
// store-less / over-window behavior is preserved (it delegates per quantile).
func (s *Service) MetricsWithQuantiles(ctx context.Context, q MetricQuery) ([]QuantileSeries, error) {
	isLatency := q.Metric == MetricHTTPLatency
	// Non-latency, or latency with at most one requested quantile: one pass, no
	// added label — identical to calling Metrics directly.
	quants := normalizeQuantiles(q.Quantiles)
	if len(quants) > maxLatencyQuantiles {
		return nil, fmt.Errorf("%w: at most %d latency percentiles per request", core.ErrBadRequest, maxLatencyQuantiles)
	}
	if !isLatency || len(quants) <= 1 {
		single := q.Quantile
		if len(quants) == 1 {
			single = quants[0]
		}
		qq := q
		qq.Quantiles = nil
		qq.Quantile = single
		series, err := s.Metrics(ctx, qq)
		if err != nil {
			return nil, err
		}
		echoed := 0.0
		if isLatency {
			echoed = single
			if echoed <= 0 {
				echoed = defaultQuantile
			}
		}
		out := make([]QuantileSeries, 0, len(series))
		for _, ser := range series {
			out = append(out, QuantileSeries{MetricSeries: ser, Quantile: echoed, HasQuantile: isLatency})
		}
		return out, nil
	}
	// Latency, several quantiles: one Metrics read per percentile, each series
	// tagged so the overlaid p50/p90/p99 stay distinguishable on every surface.
	var out []QuantileSeries
	for _, quant := range quants {
		qq := q
		qq.Quantiles = nil
		qq.Quantile = quant
		series, err := s.Metrics(ctx, qq)
		if err != nil {
			return nil, err
		}
		for _, ser := range series {
			out = append(out, QuantileSeries{MetricSeries: tagQuantile(ser, quant), Quantile: quant, HasQuantile: true})
		}
	}
	return out, nil
}

// normalizeQuantiles keeps only valid histogram percentiles (0 < q < 1), drops
// duplicates, and sorts ascending — so "All" always reads p50/p90/p99 in a
// stable order regardless of how the caller listed them.
func normalizeQuantiles(qs []float64) []float64 {
	seen := make(map[float64]bool, len(qs))
	out := make([]float64, 0, len(qs))
	for _, q := range qs {
		if q <= 0 || q >= 1 || seen[q] {
			continue
		}
		seen[q] = true
		out = append(out, q)
	}
	sort.Float64s(out)
	return out
}

// tagQuantile returns a shallow copy of the series with a `quantile` label added
// (the REST/MCP per-series echo, alongside the existing `resource`/`metric`
// labels) — copying the map so the source series is never mutated.
func tagQuantile(ser MetricSeries, quant float64) MetricSeries {
	labels := make(map[string]string, len(ser.Labels)+1)
	for k, v := range ser.Labels {
		labels[k] = v
	}
	labels["quantile"] = strconv.FormatFloat(quant, 'g', -1, 64)
	ser.Labels = labels
	return ser
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
func (s *Service) resourceMetric(ctx context.Context, q MetricQuery, app *appv1alpha1.App) ([]MetricSeries, error) {
	if s.ResourceMetricsRange != nil {
		return s.resourceMetricRange(ctx, q, app)
	}
	if s.ResourceMetrics == nil {
		return nil, core.ErrMetricsUnavailable
	}
	pods, err := s.AppPodsIn(ctx, app.Namespace, app.Name)
	if err != nil {
		return nil, err
	}
	// The snapshot source reads pod metrics from the App's per-tenant `<ws>`
	// namespace (ADR043), same as the ranged source in rangedResourceSeries —
	// the shared s.Namespace was emptied of pods by the migration.
	usage, err := s.ResourceMetrics(ctx, app.Namespace, app.Name)
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
			Labels: map[string]string{"resource": q.App, "instance": ids.ServiceInstanceID(q.App, u.Pod)},
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
func (s *Service) rangedResourceSeries(ctx context.Context, q MetricQuery, app *appv1alpha1.App, metric, unit string) ([]MetricSeries, error) {
	series, err := s.ResourceMetricsRange(ctx, ResourceMetricsRangeRequest{
		// cAdvisor series carry the pod's namespace; under ADR043 that is the
		// App's per-tenant `<ws>` namespace, so resolve it from the app name rather
		// than pinning the shared namespace (which the migration emptied of pods).
		Namespace:  app.Namespace,
		App:        app.Name,
		Metric:     metric,
		Start:      q.Start,
		End:        q.End,
		Resolution: q.Resolution,
	})
	if err != nil {
		return nil, err
	}
	for i := range series {
		series[i].SetLabel(LabelResource, q.App)
		series[i].Unit = unit
	}
	return series, nil
}

// resourceMetricRange serves cpu/memory from the ranged (Prometheus) source: one
// stepped series per replica over [Start, End]. Percentage mode divides every
// point by the pod's current spec limit, keyed by the series' instance label — an
// instance with no limit (including a pod that no longer exists) is omitted
// rather than faked, exactly like the snapshot path.
func (s *Service) resourceMetricRange(ctx context.Context, q MetricQuery, app *appv1alpha1.App) ([]MetricSeries, error) {
	unit := resourceUnit(q.Metric)
	if q.Percentage {
		unit = unitPercentage
	}
	series, err := s.rangedResourceSeries(ctx, q, app, q.Metric, unit)
	if err != nil {
		return nil, err
	}
	if q.Percentage && len(series) > 0 {
		pods, err := s.AppPodsIn(ctx, app.Namespace, app.Name)
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
	projectInstanceLabels(q.App, series)
	sort.SliceStable(series, func(i, j int) bool { return series[i].Labels["instance"] < series[j].Labels["instance"] })
	return series, nil
}

// resourceLimitMetric returns one current series per replica of the pod's
// configured CPU/memory limit (from the pod spec, not metrics-server).
func (s *Service) resourceLimitMetric(ctx context.Context, q MetricQuery, app *appv1alpha1.App) ([]MetricSeries, error) {
	pods, err := s.AppPodsIn(ctx, app.Namespace, app.Name)
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
			Labels: map[string]string{"resource": q.App, "instance": ids.ServiceInstanceID(q.App, pod.Name)},
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
func (s *Service) instanceCountMetric(ctx context.Context, q MetricQuery, app *appv1alpha1.App) ([]MetricSeries, error) {
	if s.ResourceMetricsRange != nil {
		return s.rangedResourceSeries(ctx, q, app, MetricInstanceCount, unitCount)
	}
	pods, err := s.AppPodsIn(ctx, app.Namespace, app.Name)
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
func (s *Service) requestMetric(ctx context.Context, q MetricQuery, app *appv1alpha1.App) ([]MetricSeries, error) {
	// A host/path filter is served from the request-log store (Loki) — Traefik's
	// Prometheus counters carry no per-request host/path axis (w5/m58). Metrics()
	// has already rejected host/path on any metric but the two request metrics, so
	// this branch only ever sees http_requests/http_latency.
	if q.Host != "" || q.Path != "" {
		if s.RequestLogMetrics == nil {
			return nil, core.ErrLogStoreUnavailable
		}
		// w6/m131/t009: this guard covers "source unwired", not "source wired but
		// the access-log stream is not being produced" — the state that made a
		// host filter zero a real request graph while the unfiltered read showed
		// traffic. That second state is deliberately NOT represented here: it is
		// indistinguishable from a genuinely quiet host at this vantage point,
		// exactly as for the logs read (w6/m131/t002 — the reasoning is recorded
		// in docs/ADR018-render-parity.md row 182). A pipeline that has stopped
		// producing is caught out-of-band and loudly by the scheduled
		// request-logs-liveness probe, not by manufacturing an error here that a
		// quiet host would also trigger.
		return s.readRequestSeries(ctx, s.RequestLogMetrics, q, app, nil)
	}
	if s.RequestMetrics == nil {
		return nil, core.ErrMetricsUnavailable
	}
	var routers []string
	if q.Metric == MetricBandwidth {
		var err error
		routers, err = s.TraefikRouterNames(ctx, app)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", core.ErrMetricsUnavailable, err)
		}
	}
	return s.readRequestSeries(ctx, s.RequestMetrics, q, app, routers)
}

// readRequestSeries issues one request-metric read against the chosen backend
// (Prometheus for the unfiltered path, Loki for a host/path-filtered one) and
// restores the caller-facing resource identity + unit on every returned series —
// the post-processing both backends share.
func (s *Service) readRequestSeries(ctx context.Context, source RequestMetricsSource, q MetricQuery, app *appv1alpha1.App, routers []string) ([]MetricSeries, error) {
	series, err := source(ctx, RequestMetricsRequest{
		// The App CR is in hand — its namespace is the per-tenant `<ws>` namespace
		// under ADR043; use it directly rather than the shared s.Namespace.
		Namespace:  app.Namespace,
		App:        app.Name,
		AppID:      appResourceID(app, q.App),
		Port:       app.Spec.EffectivePort(),
		Direct:     app.Spec.Type != appv1alpha1.TypeStaticSite,
		Routers:    routers,
		Metric:     q.Metric,
		Start:      q.Start,
		End:        q.End,
		Resolution: q.Resolution,
		Quantile:   q.Quantile,
		StatusCode: q.StatusCode,
		GroupBy:    q.GroupBy,
		Host:       q.Host,
		Path:       q.Path,
	})
	if err != nil {
		return nil, err
	}
	unit := requestUnit(q.Metric)
	for i := range series {
		// Aggregation commonly removes every selector label. Restore the
		// caller-facing resource identity here: operational selectors use the
		// resolved App name, but multi-resource REST/GraphQL/MCP responses must
		// remain attributable to the public id or name the caller requested.
		series[i].SetLabel(LabelResource, q.App)
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
// a cumulative bandwidth figure for the current calendar month. HTTP, direct
// public (Render's NAT-shaped category), and WebSocket bytes are all real.
type MonthToDateBandwidth struct {
	EgressBandwidthMB            float64
	HTTPEgressBandwidthMB        float64
	NATEgressBandwidthMB         float64
	PrivateLinkEgressBandwidthMB float64
	WebsocketEgressBandwidthMB   float64
	// DegradedSources names the egress sources whose health product failed
	// inside the month window (bex extension; empty = fully healthy). The
	// figures above still include whatever those sources recorded.
	DegradedSources []string
}

// MonthToDateBandwidth returns the App's month-to-date bandwidth usage. The query
// is real (increase() over the elapsed month); a short-retention Prometheus just
// under-counts (see ADR010-observability.md).
func (s *Service) MonthToDateBandwidth(ctx context.Context, app string) (MonthToDateBandwidth, error) {
	resolved, err := s.AuthorizeApp(ctx, core.RelCanView, app)
	if err != nil {
		return MonthToDateBandwidth{}, err
	}
	if s.MonthToDateBandwidthSource == nil {
		return MonthToDateBandwidth{}, core.ErrMetricsUnavailable
	}
	routers, err := s.TraefikRouterNames(ctx, resolved)
	if err != nil {
		return MonthToDateBandwidth{}, fmt.Errorf("%w: %v", core.ErrMetricsUnavailable, err)
	}
	now := s.Now().UTC()
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, time.UTC)
	bytesByCategory, degraded, err := s.MonthToDateBandwidthSource(
		ctx, appResourceID(resolved, app), routers,
		resolved.Spec.Type != appv1alpha1.TypeStaticSite, monthStart, now,
	)
	if err != nil {
		return MonthToDateBandwidth{}, err
	}
	const bytesPerMiB = 1 << 20
	httpMB := bytesByCategory.HTTP / bytesPerMiB
	natMB := bytesByCategory.NAT / bytesPerMiB
	wsMB := bytesByCategory.WebSocket / bytesPerMiB
	return MonthToDateBandwidth{
		EgressBandwidthMB:          httpMB + natMB + wsMB,
		HTTPEgressBandwidthMB:      httpMB,
		NATEgressBandwidthMB:       natMB,
		WebsocketEgressBandwidthMB: wsMB,
		DegradedSources:            degraded,
	}, nil
}

func appResourceID(app *appv1alpha1.App, fallback string) string {
	if app != nil && app.Labels[core.LabelAppID] != "" {
		return app.Labels[core.LabelAppID]
	}
	return fallback
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

// maxMetricsOutputFilters bounds the GraphQL dropdown-discovery fan-out. A
// legitimate query names only Render's small filter vocabulary; eight leaves
// room for extensions while preventing repeated Kubernetes/Prometheus scans.
const maxMetricsOutputFilters = 8

// MetricsFilters resolves available values for each requested output filter.
// RESOURCE/INSTANCE are answered from data the service already has;
// STATUS_CODE needs MetricsFilterValuesSource; BUILD/HOST/PATH always report
// empty here — this Prometheus-backed discovery has no build/host/path axis.
// Since w5/m58 the Metrics verb DOES accept host/path (for http_requests/
// http_latency, served from the request-log store), but their values are
// discovered from the logs label-values read instead (host from the App's URLs,
// path is a free-text filter), never from this verb.
func (s *Service) MetricsFilters(ctx context.Context, q MetricsFiltersQuery) ([]MetricsFilterValues, error) {
	if len(q.OutputFilters) > maxMetricsOutputFilters {
		return nil, fmt.Errorf("%w: outputFilters accepts at most %d entries", core.ErrBadRequest, maxMetricsOutputFilters)
	}
	seen := make(map[string]struct{}, len(q.OutputFilters))
	for _, field := range q.OutputFilters {
		if _, ok := seen[field]; ok {
			return nil, fmt.Errorf("%w: outputFilters contains duplicate %q", core.ErrBadRequest, field)
		}
		seen[field] = struct{}{}
	}
	app, err := s.AuthorizeApp(ctx, core.RelCanView, q.App)
	if err != nil {
		return nil, err
	}

	out := make([]MetricsFilterValues, 0, len(q.OutputFilters))
	for _, field := range q.OutputFilters {
		switch field {
		case filterFieldResource:
			out = append(out, MetricsFilterValues{Field: field, Values: []string{q.App}})
		case filterFieldInstance:
			pods, err := s.AppPodsIn(ctx, app.Namespace, app.Name)
			if err != nil {
				return nil, err
			}
			instances := make([]string, 0, len(pods))
			for _, p := range pods {
				instances = append(instances, ids.ServiceInstanceID(q.App, p.Name))
			}
			out = append(out, MetricsFilterValues{Field: field, Values: instances})
		case filterFieldStatusCode:
			values, err := s.filterValuesOrEmpty(ctx, app, "code")
			if err != nil {
				return nil, err
			}
			out = append(out, MetricsFilterValues{Field: field, Values: values})
		default: // BUILD, HOST, PATH — filters Metrics can't honor stay unoffered
			out = append(out, MetricsFilterValues{Field: field, Values: []string{}})
		}
	}
	return out, nil
}

func (s *Service) filterValuesOrEmpty(ctx context.Context, app *appv1alpha1.App, label string) ([]string, error) {
	if s.MetricsFilterValuesSource == nil {
		return []string{}, nil
	}
	// app.Namespace is the App's per-tenant `<ws>` namespace (ADR043), where
	// its series live — never the shared s.Namespace.
	return s.MetricsFilterValuesSource(ctx, app.Namespace, app.Name, app.Spec.EffectivePort(), label)
}
