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
	"fmt"
	"maps"
	"math"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"time"

	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/client-go/kubernetes"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// source.go holds the production metrics backends — the domain depends only on
// the injected source function types, so it stays apiserver-thin and every read
// is faked in tests. The transport is thin; the tricky parts (quantity parsing,
// the PromQL builder, the Prometheus matrix parser) are pure and unit-tested.

// --- Resource metrics: metrics.k8s.io (metrics-server) ---

// NewResourceMetricsSource returns the production ResourceMetricsSource, reading
// PodMetrics from metrics-server's aggregated API (metrics.k8s.io/v1beta1).
func NewResourceMetricsSource(cs kubernetes.Interface) ResourceMetricsSource {
	return func(ctx context.Context, namespace, app string) ([]PodResourceUsage, error) {
		raw, err := cs.Discovery().RESTClient().Get().
			AbsPath("/apis/metrics.k8s.io/v1beta1/namespaces", namespace, "pods").
			Param("labelSelector", core.PodLabelApp+"="+app).
			DoRaw(ctx)
		if err != nil {
			return nil, err
		}
		return parsePodMetrics(raw)
	}
}

// podMetricsList is the subset of metrics.k8s.io/v1beta1 PodMetricsList bex reads.
type podMetricsList struct {
	Items []struct {
		Metadata struct {
			Name string `json:"name"`
		} `json:"metadata"`
		Timestamp  string `json:"timestamp"`
		Containers []struct {
			Usage struct {
				CPU    string `json:"cpu"`
				Memory string `json:"memory"`
			} `json:"usage"`
		} `json:"containers"`
	} `json:"items"`
}

// parsePodMetrics decodes a metrics-server PodMetricsList and sums each pod's
// container usage into cores + bytes. Unparseable quantities are treated as zero.
func parsePodMetrics(raw []byte) ([]PodResourceUsage, error) {
	var list podMetricsList
	if err := json.Unmarshal(raw, &list); err != nil {
		return nil, fmt.Errorf("decode pod metrics: %w", err)
	}
	out := make([]PodResourceUsage, 0, len(list.Items))
	for _, it := range list.Items {
		u := PodResourceUsage{Pod: it.Metadata.Name, Timestamp: it.Timestamp}
		for _, ctr := range it.Containers {
			if q, err := resource.ParseQuantity(ctr.Usage.CPU); err == nil {
				u.CPUCores += q.AsApproximateFloat64()
			}
			if q, err := resource.ParseQuantity(ctr.Usage.Memory); err == nil {
				u.MemoryBytes += float64(q.Value())
			}
		}
		out = append(out, u)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Pod < out[j].Pod })
	return out, nil
}

// --- Request metrics: Traefik scraped by Prometheus ---

// NewPrometheusRequestSource returns the production RequestMetricsSource, backed
// by a Prometheus range query over Traefik's metrics.
func NewPrometheusRequestSource(base string, hc *http.Client) RequestMetricsSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		query := promQueryFor(req)
		if query == "" {
			return []MetricSeries{}, nil
		}
		return promQueryRange(ctx, hc, base, query, req.Start, req.End, stepSeconds(req.Resolution))
	}
}

// promQueryRange runs one Prometheus query_range and parses the matrix reply —
// the shared transport under the request- and resource-metric sources.
func promQueryRange(ctx context.Context, hc *http.Client, base, query string, start, end time.Time, step int64) ([]MetricSeries, error) {
	u := fmt.Sprintf("%s/api/v1/query_range?%s", base, url.Values{
		"query": {query},
		"start": {strconv.FormatInt(start.Unix(), 10)},
		"end":   {strconv.FormatInt(end.Unix(), 10)},
		"step":  {strconv.FormatInt(step, 10)},
	}.Encode())

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	resp, err := hc.Do(httpReq)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("prometheus: status %d", resp.StatusCode)
	}
	var pr promRangeResponse
	if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
		return nil, fmt.Errorf("decode prometheus response: %w", err)
	}
	return parsePromMatrix(pr)
}

func stepSeconds(res time.Duration) int64 {
	if res <= 0 {
		res = defaultResolution
	}
	return int64(res / time.Second)
}

// --- Resource metrics history: cAdvisor scraped by Prometheus ---

// NewPrometheusResourceSource returns the production ResourceMetricsRangeSource
// — stepped cpu/memory/instance-count series via query_range over the cAdvisor
// series the kubernetes-cadvisor scrape job collects
// (deploy/gitops/base/prometheus.yaml). base/hc as NewPrometheusRequestSource.
func NewPrometheusResourceSource(base string, hc *http.Client) ResourceMetricsRangeSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req ResourceMetricsRangeRequest) ([]MetricSeries, error) {
		series, err := promQueryRange(ctx, hc, base, promResourceQueryFor(req), req.Start, req.End, stepSeconds(req.Resolution))
		if err != nil {
			return nil, err
		}
		// Rewrite Prometheus's label vocabulary into Core's: pod => instance,
		// plus the resource (App) tag every series carries. instance_count has
		// no pod label (it's a scalar count), so it keeps just resource.
		for i := range series {
			labels := map[string]string{"resource": req.App}
			if pod := series[i].Labels["pod"]; pod != "" {
				labels["instance"] = pod
			}
			series[i].Labels = labels
		}
		return series, nil
	}
}

// promResourceQueryFor builds the PromQL range query for a resource metric over
// cAdvisor's per-container series:
//
//   - cpu:            per-pod rate of container_cpu_usage_seconds_total (cores)
//   - memory:         per-pod sum of container_memory_working_set_bytes (bytes)
//   - instance_count: count of pods with live cAdvisor series
//
// kubelet metrics carry pod NAMES but not pod labels (there is no
// app.bex.co/app to match on), so an App's pods are matched by the Deployment
// pod-name shape: <app>-<replicaset-hash>-<5-char-suffix>. Like the Traefik
// service selector this is a heuristic — but anchored and two-segment, so app
// "web" never matches a "web-api-…-…" pod. container!="" drops any
// pod-sandbox/aggregate rows that survive the scrape-side relabeling.
func promResourceQueryFor(req ResourceMetricsRangeRequest) string {
	matchers := fmt.Sprintf(`namespace=%q,pod=~"%s-[a-z0-9]+-[a-z0-9]{5}",container!=""`,
		req.Namespace, promEscape(req.App))
	switch req.Metric {
	case MetricMemory:
		return fmt.Sprintf(`sum by (pod) (container_memory_working_set_bytes{%s})`, matchers)
	case MetricInstanceCount:
		return fmt.Sprintf(`count(sum by (pod) (container_memory_working_set_bytes{%s}))`, matchers)
	default: // cpu
		return fmt.Sprintf(`sum by (pod) (rate(container_cpu_usage_seconds_total{%s}[%ds]))`,
			matchers, stepSeconds(req.Resolution))
	}
}

// promQueryFor builds the PromQL range query for a request metric over Traefik's
// counters. HTTP count/latency retain their service selector; bandwidth uses
// exact router identities so shared backends remain attributable. The counters
// carry no host/path labels, so those filters are rejected upstream (see
// MetricQuery.Host) and absent from RequestMetricsRequest.
func promQueryFor(req RequestMetricsRequest) string {
	selector := fmt.Sprintf(`service=~".*%s.*"`, promEscape(req.App))
	if req.Metric == MetricBandwidth {
		matcher := core.TraefikRouterMatcher(req.Routers)
		if matcher == "" {
			return ""
		}
		selector = fmt.Sprintf(`router=~%q`, matcher)
	}
	sel := []string{selector}
	if c := codeMatcher(req.StatusCode); c != "" {
		sel = append(sel, fmt.Sprintf(`code=~"%s"`, c))
	}
	matchers := strings.Join(sel, ",")
	window := fmt.Sprintf("%ds", stepSeconds(req.Resolution))

	switch req.Metric {
	case MetricHTTPLatency:
		by := "le"
		if g := groupLabel(req.GroupBy); g != "" {
			by += "," + g
		}
		return fmt.Sprintf(`histogram_quantile(%s, sum(rate(traefik_service_request_duration_seconds_bucket{%s}[%s])) by (%s))`,
			strconv.FormatFloat(req.Quantile, 'g', -1, 64), matchers, window, by)
	case MetricBandwidth:
		return sumRate("traefik_router_responses_bytes_total", matchers, window, groupLabel(req.GroupBy))
	default: // http_requests
		return sumRate("traefik_service_requests_total", matchers, window, groupLabel(req.GroupBy))
	}
}

func sumRate(metric, matchers, window, by string) string {
	if by != "" {
		return fmt.Sprintf(`sum(rate(%s{%s}[%s])) by (%s)`, metric, matchers, window, by)
	}
	return fmt.Sprintf(`sum(rate(%s{%s}[%s]))`, metric, matchers, window)
}

// codeMatcher turns a Render statusCode filter into a Prometheus `code` regex.
func codeMatcher(statusCode string) string {
	s := strings.ToLower(strings.TrimSpace(statusCode))
	if s == "" {
		return ""
	}
	if len(s) == 3 && s[1] == 'x' && s[2] == 'x' {
		return string(s[0]) + ".."
	}
	return s
}

// groupLabel maps a Render groupBy onto a Traefik service-metric label.
func groupLabel(groupBy string) string {
	switch strings.ToLower(groupBy) {
	case "status", "code":
		return "code"
	case "method":
		return "method"
	default:
		return ""
	}
}

// promEscape escapes regex metacharacters in an App name used inside a =~ matcher.
func promEscape(s string) string {
	var b strings.Builder
	for _, r := range s {
		if strings.ContainsRune(`.+*?()|[]{}^$\`, r) {
			b.WriteByte('\\')
		}
		b.WriteRune(r)
	}
	return b.String()
}

// promStatusSuccess is Prometheus's "status" field on a successful response.
const promStatusSuccess = "success"

// promRangeResponse is the subset of Prometheus's query_range result bex reads.
type promRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Metric map[string]string `json:"metric"`
			Values [][]any           `json:"values"` // [ [unixSeconds(float), "value"(string)], ... ]
		} `json:"result"`
	} `json:"data"`
}

// --- Month-to-date bandwidth: a Prometheus increase() instant query ---

// NewMonthToDateBandwidthSource returns the production MonthToDateBandwidthSource
// — a single Prometheus instant query for cumulative HTTP egress since a time.
func NewMonthToDateBandwidthSource(base string, hc *http.Client) MonthToDateBandwidthSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, routers []string, since time.Time) (float64, error) {
		matcher := core.TraefikRouterMatcher(routers)
		if matcher == "" {
			return 0, nil
		}
		elapsed := time.Since(since)
		if elapsed <= 0 {
			return 0, nil
		}
		q := fmt.Sprintf(`sum(increase(traefik_router_responses_bytes_total{router=~%q}[%ds]))`,
			matcher, int64(elapsed/time.Second))
		u := fmt.Sprintf("%s/api/v1/query?%s", base, url.Values{"query": {q}}.Encode())

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return 0, err
		}
		resp, err := hc.Do(httpReq)
		if err != nil {
			return 0, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return 0, fmt.Errorf("prometheus: status %d", resp.StatusCode)
		}
		var pr promInstantResponse
		if err := json.NewDecoder(resp.Body).Decode(&pr); err != nil {
			return 0, fmt.Errorf("decode prometheus response: %w", err)
		}
		return parsePromScalarSum(pr)
	}
}

// promInstantResponse is the subset of Prometheus's instant /api/v1/query result.
type promInstantResponse struct {
	Status string `json:"status"`
	Data   struct {
		Result []struct {
			Value []any `json:"value"` // [unixSeconds(float), "value"(string)]
		} `json:"result"`
	} `json:"data"`
}

// parsePromScalarSum sums every returned vector element's value.
func parsePromScalarSum(pr promInstantResponse) (float64, error) {
	if pr.Status != "" && pr.Status != promStatusSuccess {
		return 0, fmt.Errorf("prometheus status %q", pr.Status)
	}
	var total float64
	for _, res := range pr.Data.Result {
		if len(res.Value) != 2 {
			continue
		}
		str, ok := res.Value[1].(string)
		if !ok {
			continue
		}
		val, err := strconv.ParseFloat(str, 64)
		if err != nil || math.IsNaN(val) {
			continue
		}
		total += val
	}
	return total, nil
}

// --- Datastore metrics: PVC stats + CNPG's postgres_exporter over Prometheus ---

// NewPrometheusDiskUsageSource returns the production DiskUsageSource — PVC
// used/capacity bytes via query_range over kubelet's volume-stats series
// (deploy/gitops/base/prometheus.yaml's kubernetes-kubelet job, already scraped
// cluster-wide for the PersistentVolumeFillingUp alert, so no new scrape config
// is needed for this series).
func NewPrometheusDiskUsageSource(base string, hc *http.Client) DiskUsageSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req DiskUsageRequest) ([]MetricSeries, error) {
		metric := "kubelet_volume_stats_used_bytes"
		if req.Metric == MetricDiskCapacity {
			metric = "kubelet_volume_stats_capacity_bytes"
		}
		q := fmt.Sprintf(`%s{namespace=%q,persistentvolumeclaim=~%q}`, metric, req.Namespace, req.PVCPattern)
		series, err := promQueryRange(ctx, hc, base, q, req.Start, req.End, stepSeconds(req.Resolution))
		if err != nil {
			return nil, err
		}
		unit := unitBytes
		for i := range series {
			labels := map[string]string{"resource": req.Resource}
			if pvc := series[i].Labels["persistentvolumeclaim"]; pvc != "" {
				labels["instance"] = pvc
			}
			series[i].Labels = labels
			series[i].Unit = unit
		}
		return series, nil
	}
}

// NewPrometheusKeyValueStatsSource returns the production KeyValueStatsSource —
// stepped redis_exporter series (used-memory bytes, connected-clients count)
// via query_range, the KeyValue sibling of NewPrometheusDiskUsageSource. A
// managed KeyValue is a single-instance StatefulSet, so its pods are named
// <name>-<ordinal>; the anchored, two-segment matcher means store "cache" never
// matches a "cache-api-…" pod. The valkey-instances scrape job
// (deploy/gitops/base/prometheus.yaml) labels each series with pod + namespace.
func NewPrometheusKeyValueStatsSource(base string, hc *http.Client) KeyValueStatsSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req KeyValueStatsRequest) ([]MetricSeries, error) {
		metric, unit := "redis_memory_used_bytes", unitBytes
		if req.Dimension == "connections" {
			metric, unit = "redis_connected_clients", unitCount
		}
		q := fmt.Sprintf(`sum by (pod) (%s{namespace=%q,pod=~%q})`,
			metric, req.Namespace, fmt.Sprintf(`%s-[0-9]+`, promEscape(req.Resource)))
		series, err := promQueryRange(ctx, hc, base, q, req.Start, req.End, stepSeconds(req.Resolution))
		if err != nil {
			return nil, err
		}
		for i := range series {
			labels := map[string]string{"resource": req.Resource}
			if pod := series[i].Labels["pod"]; pod != "" {
				labels["instance"] = pod
			}
			series[i].Labels = labels
			series[i].Unit = unit
		}
		return series, nil
	}
}

// cnpgInstanceQuery runs a query_range for a CNPG-cluster-scoped metric and
// relabels each series onto Core's vocabulary (pod => instance, plus the
// cluster's resource tag) with the given unit — the shared scaffold behind
// NewPrometheusDBConnectionsSource and NewPrometheusReplicationLagSource,
// which differ only in their PromQL and unit.
func cnpgInstanceQuery(ctx context.Context, hc *http.Client, base, query, cluster, unit string, start, end time.Time, step int64) ([]MetricSeries, error) {
	series, err := promQueryRange(ctx, hc, base, query, start, end, step)
	if err != nil {
		return nil, err
	}
	for i := range series {
		labels := map[string]string{"resource": cluster}
		if pod := series[i].Labels["pod"]; pod != "" {
			labels["instance"] = pod
		}
		series[i].Labels = labels
		series[i].Unit = unit
	}
	return series, nil
}

// NewPrometheusDBConnectionsSource returns the production DBConnectionsSource —
// a managed Postgres instance's active-connection count via query_range over
// CNPG's postgres_exporter cnpg_backends_total (verified against CNPG's
// monitoring reference: the "backends" default query's `total` column, summed
// across every datname/usename/state combination for one cluster).
func NewPrometheusDBConnectionsSource(base string, hc *http.Client) DBConnectionsSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req DBConnectionsRequest) ([]MetricSeries, error) {
		q := fmt.Sprintf(`sum by (pod) (cnpg_backends_total{namespace=%q,cnpg_io_cluster=%q})`,
			req.Namespace, promEscape(req.Cluster))
		return cnpgInstanceQuery(ctx, hc, base, q, req.Cluster, unitCount, req.Start, req.End, stepSeconds(req.Resolution))
	}
}

// NewPrometheusReplicationLagSource returns the production ReplicationLagSource
// — a managed Postgres instance's replication lag (seconds) via query_range
// over CNPG's postgres_exporter cnpg_pg_replication_lag (verified against
// CNPG's monitoring reference: the "pg_replication" default query's `lag`
// column). Only called once a Database's HighAvailabilityEnabled is true
// (datastore.go) — pre-HA there is no standby, and this metric reports 0 from
// the lone primary rather than absence, which is exactly the fake-zero the
// caller gates against.
func NewPrometheusReplicationLagSource(base string, hc *http.Client) ReplicationLagSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req ReplicationLagRequest) ([]MetricSeries, error) {
		q := fmt.Sprintf(`max by (pod) (cnpg_pg_replication_lag{namespace=%q,cnpg_io_cluster=%q})`,
			req.Namespace, promEscape(req.Cluster))
		return cnpgInstanceQuery(ctx, hc, base, q, req.Cluster, unitSeconds, req.Start, req.End, stepSeconds(req.Resolution))
	}
}

// --- Filter-value discovery: Prometheus's label-values API ---

// NewPrometheusFilterValuesSource returns the production MetricsFilterValuesSource,
// backed by Prometheus's /api/v1/label/<name>/values endpoint.
func NewPrometheusFilterValuesSource(base string, hc *http.Client) MetricsFilterValuesSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, _, app, label string) ([]string, error) {
		match := fmt.Sprintf(`traefik_service_requests_total{service=~".*%s.*"}`, promEscape(app))
		u := fmt.Sprintf("%s/api/v1/label/%s/values?%s", base, url.PathEscape(label), url.Values{"match[]": {match}}.Encode())

		httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
		if err != nil {
			return nil, err
		}
		resp, err := hc.Do(httpReq)
		if err != nil {
			return nil, err
		}
		defer func() { _ = resp.Body.Close() }()
		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("prometheus: status %d", resp.StatusCode)
		}
		var lr promLabelValuesResponse
		if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
			return nil, fmt.Errorf("decode prometheus response: %w", err)
		}
		if lr.Status != "" && lr.Status != promStatusSuccess {
			return nil, fmt.Errorf("prometheus status %q", lr.Status)
		}
		return lr.Data, nil
	}
}

// promLabelValuesResponse is Prometheus's /api/v1/label/<name>/values shape.
type promLabelValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// parsePromMatrix maps a Prometheus matrix response onto MetricSeries: one series
// per result, points RFC3339-stamped and oldest-first. NaN/parse-failures drop
// the point rather than fail the series.
func parsePromMatrix(pr promRangeResponse) ([]MetricSeries, error) {
	if pr.Status != "" && pr.Status != promStatusSuccess {
		return nil, fmt.Errorf("prometheus status %q", pr.Status)
	}
	out := make([]MetricSeries, 0, len(pr.Data.Result))
	for _, res := range pr.Data.Result {
		labels := make(map[string]string, len(res.Metric))
		maps.Copy(labels, res.Metric)
		points := make([]MetricPoint, 0, len(res.Values))
		for _, pair := range res.Values {
			if len(pair) != 2 {
				continue
			}
			ts, ok := pair[0].(float64)
			if !ok {
				continue
			}
			str, ok := pair[1].(string)
			if !ok {
				continue
			}
			val, err := strconv.ParseFloat(str, 64)
			if err != nil || math.IsNaN(val) {
				continue // Prometheus emits "NaN" for empty buckets; skip it
			}
			points = append(points, MetricPoint{
				Timestamp: time.Unix(int64(ts), 0).UTC().Format(time.RFC3339),
				Value:     val,
			})
		}
		out = append(out, MetricSeries{Labels: labels, Points: points})
	}
	return out, nil
}
