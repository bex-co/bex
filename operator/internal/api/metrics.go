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
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// metrics.go is the Core metrics read — the m2 sibling of the logs verb. Like
// logs it is ONE domain implementation the REST + GraphQL adapters share, and it
// reaches its backends through injected sources so Core stays clientset-free:
//
//   - resource metrics (cpu/memory) come from metrics-server via ResourceMetrics;
//   - instance count is derived from the App's pods (no source needed);
//   - request metrics (count/latency/bandwidth) come from Traefik-over-Prometheus
//     via RequestMetrics.
//
// Shapes are Render's metrics-API time-series (see render.go: a series carries a
// unit, labels and points). metrics-server is a point-in-time snapshot, so bex's
// cpu/memory/instance_count series carry a single current point regardless of the
// requested time range; the Prometheus-backed request metrics honor start/end/
// resolution. Both intentional deviations are documented in observability.md.

// Metric ids — bex's canonical names (underscored). They map 1:1 to Render's
// metrics endpoints (path segments cpu, memory, instance-count, http-requests,
// http-latency, bandwidth) in the REST adapter and are the GraphQL `metric` enum.
const (
	MetricCPU           = "cpu"
	MetricMemory        = "memory"
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
	if _, err := c.fetch(ctx, q.App); err != nil {
		return nil, err // ErrNotFound for unknown apps, exactly like Get
	}
	q = q.normalized(c.now())
	switch q.Metric {
	case MetricCPU, MetricMemory:
		return c.resourceMetric(ctx, q)
	case MetricInstanceCount:
		return c.instanceCountMetric(ctx, q)
	case MetricHTTPRequests, MetricHTTPLatency, MetricBandwidth:
		return c.requestMetric(ctx, q)
	default:
		return nil, fmt.Errorf("unknown metric %q", q.Metric)
	}
}

// resourceMetric returns one current series per replica (labels instance +
// resource) of CPU or memory usage. With Percentage set, each value is divided by
// that pod's limit (from the pod spec) and reported as a 0..100 percentage; a pod
// with no limit is skipped from percentage output (division is undefined).
func (c *Core) resourceMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
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

	unit := unitCores
	if q.Metric == MetricMemory {
		unit = unitBytes
	}
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

// instanceCountMetric returns a single-series current replica count (running
// pods). It needs no metrics source — it counts the App's pods directly — so it
// works even on a cluster without metrics-server.
func (c *Core) instanceCountMetric(ctx context.Context, q MetricQuery) ([]MetricSeries, error) {
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
