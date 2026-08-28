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
	"github.com/bex-co/bex/lego/backend/internal/egressquery"
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
		// Bound the read like every other upstream (codex round-8 #10): the
		// shared clientset carries no request deadline of its own, and a wedged
		// aggregated API would otherwise hold this handler indefinitely. (The
		// buffered DoRaw body has no size cap — an in-cluster platform service,
		// documented residual.)
		ctx, cancel := context.WithTimeout(ctx, core.UpstreamTimeout)
		defer cancel()
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

// LabelDegradedSources is the reserved series label carrying the comma-joined
// egress sources whose health product failed inside the queried window
// (ADR023 § Observability reads vs billing reads). It rides the existing
// labels array so REST/GraphQL/MCP expose it without a schema change; absent
// means every applicable source was healthy for the whole window.
const LabelDegradedSources = "degraded_sources"

// LabelResource carries the public id or name the caller asked about, restored
// onto every series because aggregation commonly drops the selector labels.
const LabelResource = "resource"

// LabelMetric names which metric a series came from, so a multi-metric result
// stays distinguishable after the per-metric results are concatenated.
const LabelMetric = "metric"

// NewPrometheusRequestSource returns the production RequestMetricsSource, backed
// by a Prometheus range query over Traefik's metrics.
func NewPrometheusRequestSource(base string, hc *http.Client) RequestMetricsSource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		var degraded []string
		if req.Metric == MetricBandwidth {
			// Interactive reads are best-effort (ADR023 § Observability reads
			// vs billing reads, w1/m50): a source failing its health product
			// (up gap, in-window counter reset, counter loss) no longer
			// rejects the whole window — the series is served and the failing
			// sources travel as the degraded_sources label so clients can
			// annotate. The usage rollup keeps the strict gate.
			seconds := int64(req.End.Sub(req.Start) / time.Second)
			for _, spec := range egressquery.App(req.AppID, req.Routers, req.Direct) {
				health, err := egressquery.Instant(ctx, hc, base, egressquery.Health(spec, seconds), req.End)
				if err != nil {
					return nil, err
				}
				if health < 1 {
					degraded = append(degraded, string(spec.Source))
				}
			}
		}
		query := promQueryFor(req)
		if query == "" {
			return []MetricSeries{}, nil
		}
		series, err := promQueryRange(ctx, hc, base, query, req.Start, req.End, stepSeconds(req.Resolution))
		if err != nil {
			return nil, err
		}
		if len(degraded) > 0 {
			setLabelOnEach(series, LabelDegradedSources, strings.Join(degraded, ","))
		}
		return series, nil
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
	return queryRangeMatrix(ctx, hc, u, "prometheus")
}

// queryRangeMatrix performs a query_range GET and parses the matrix reply. Both
// Prometheus and Loki return the identical {status, data:{resultType:"matrix",
// result:[{metric, values:[[unixSeconds, "value"]]}]}} envelope, so the Loki
// request-metrics source (w5/m58) reuses this parser after building its own URL.
// store names the backend for the transport error message only.
func queryRangeMatrix(ctx context.Context, hc *http.Client, u, store string) ([]MetricSeries, error) {
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
		return nil, fmt.Errorf("%s: status %d", store, resp.StatusCode)
	}
	var pr promRangeResponse
	if err := core.DecodeUpstreamJSON(resp.Body, &pr); err != nil {
		return nil, fmt.Errorf("decode %s response: %w", store, err)
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
		hc = core.UpstreamClient
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
// pod-name shape — see egressquery.PodNameRegex, which owns both the
// untruncated <app>-<replicaset-hash>-<5-char-suffix> form and the single-
// segment name Kubernetes leaves once generateName truncates past 58 chars
// (w6/m110: the two-segment-only selector blanked Memory/CPU/Total Instances
// for every service whose object name crossed that threshold). Like the Traefik
// service selector this is a reconstruction from names, but fully anchored, so
// app "web" never matches a "web-api-…-…" pod.
func promResourceQueryFor(req ResourceMetricsRangeRequest) string {
	matchers := egressquery.PodNameMatcher(req.Namespace, req.App)
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

// traefikServiceLabel returns the exact Prometheus `service` label Traefik's
// Kubernetes Ingress provider assigns to an App's backend:
// "<namespace>-<app>-<port>@kubernetes" (docs/ADR010-observability.md,
// ground-truthed 2026-07 against production; the identical regex is load-bearing
// in the request-log pipeline, deploy/gitops/base/log-shipper.yaml's
// `stage.regex` over the access log's ServiceName field). The operator names the
// App's k8s Service after the App and gives it exactly app.Spec.EffectivePort()
// (app_controller.go), so this is a reconstruction of a known identity, not a
// heuristic — codex-security round-19 #6: the prior unanchored `.*<app>.*`
// substring selector dropped namespace entirely and could match another
// tenant's Traefik service whose name happened to contain this App's name.
func traefikServiceLabel(namespace, app string, port int32) string {
	return fmt.Sprintf("%s-%s-%d@kubernetes", namespace, app, port)
}

// promQueryFor builds the PromQL range query for a request metric over Traefik's
// counters. HTTP count/latency retain their exact service selector; bandwidth uses
// exact router identities so shared backends remain attributable. Traefik's
// Prometheus counters carry no host or path labels (the router label is a name,
// not a matched Host()/PathPrefix() value), so a host/path-filtered read is never
// routed here — it goes to the Loki request-metrics source instead (w5/m58); this
// builder ignores RequestMetricsRequest.Host/Path.
func promQueryFor(req RequestMetricsRequest) string {
	selector := fmt.Sprintf(`service=%q`, traefikServiceLabel(req.Namespace, req.App, req.Port))
	if req.Metric == MetricBandwidth {
		return egressquery.SumRates(egressquery.App(req.AppID, req.Routers, req.Direct), stepSeconds(req.Resolution))
	}
	sel := []string{selector}
	if c := codeMatcher(req.StatusCode); c != "" {
		sel = append(sel, fmt.Sprintf(`code=~%q`, c))
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
	s, ok := normalizeStatusCodeFilter(statusCode)
	if !ok || s == "" {
		return ""
	}
	if s[1] == 'x' {
		return string(s[0]) + ".."
	}
	return s
}

// normalizeStatusCodeFilter accepts only the closed Render status-filter
// grammar. Builders also call it defensively so an adapter cannot accidentally
// turn an unvalidated string into PromQL or LogQL syntax.
func normalizeStatusCodeFilter(statusCode string) (string, bool) {
	s := strings.ToLower(strings.TrimSpace(statusCode))
	if s == "" {
		return "", true
	}
	if len(s) != 3 || s[0] < '1' || s[0] > '5' {
		return "", false
	}
	if s[1] == 'x' && s[2] == 'x' {
		return s, true
	}
	if s[1] < '0' || s[1] > '9' || s[2] < '0' || s[2] > '9' {
		return "", false
	}
	return s, true
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

// NewMonthToDateBandwidthSource returns the production composed App egress
// source. Best-effort (ADR023 § Observability reads vs billing reads, w1/m50):
// a source failing its health product still contributes whatever its counters
// recorded — chart-grade, not billing-grade — and is named in the returned
// degraded list instead of rejecting the whole month.
func NewMonthToDateBandwidthSource(base string, hc *http.Client) MonthToDateBandwidthSource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, appID string, routers []string, direct bool, since, at time.Time) (BandwidthBytes, []string, error) {
		elapsed := at.Sub(since)
		if elapsed <= 0 {
			return BandwidthBytes{}, nil, nil
		}
		var out BandwidthBytes
		var degraded []string
		for _, spec := range egressquery.App(appID, routers, direct) {
			health, err := egressquery.Instant(ctx, hc, base, egressquery.Health(spec, int64(elapsed/time.Second)), at)
			if err != nil {
				return BandwidthBytes{}, nil, err
			}
			if health < 1 {
				degraded = append(degraded, string(spec.Source))
			}
			value, err := egressquery.Instant(ctx, hc, base, egressquery.Increase(spec, int64(elapsed/time.Second)), at)
			if err != nil {
				return BandwidthBytes{}, nil, err
			}
			switch spec.Source {
			case egressquery.HTTP:
				out.HTTP += value
			case egressquery.Direct:
				out.NAT += value
			case egressquery.WebSocket:
				out.WebSocket += value
			}
		}
		return out, degraded, nil
	}
}

// --- Datastore metrics: PVC stats + CNPG's postgres_exporter over Prometheus ---

// NewPrometheusDiskUsageSource returns the production DiskUsageSource — PVC
// used/capacity bytes via query_range over kubelet's volume-stats series
// (deploy/gitops/base/prometheus.yaml's kubernetes-kubelet job, already scraped
// cluster-wide for the PersistentVolumeFillingUp alert, so no new scrape config
// is needed for this series).
func NewPrometheusDiskUsageSource(base string, hc *http.Client) DiskUsageSource {
	if hc == nil {
		hc = core.UpstreamClient
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
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req KeyValueStatsRequest) ([]MetricSeries, error) {
		metric, unit := "redis_memory_used_bytes", unitBytes
		if req.Dimension == "connections" {
			metric, unit = "redis_connected_clients", unitCount
		}
		q := fmt.Sprintf(`sum by (pod) (%s{namespace=%q,pod=~%q})`,
			metric, req.Namespace, fmt.Sprintf(`%s-[0-9]+`, egressquery.RegexEscape(req.Resource)))
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
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req DBConnectionsRequest) ([]MetricSeries, error) {
		q := fmt.Sprintf(`sum by (pod) (cnpg_backends_total{namespace=%q,cnpg_io_cluster=%q})`,
			req.Namespace, egressquery.RegexEscape(req.Cluster))
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
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req ReplicationLagRequest) ([]MetricSeries, error) {
		q := fmt.Sprintf(`max by (pod) (cnpg_pg_replication_lag{namespace=%q,cnpg_io_cluster=%q})`,
			req.Namespace, egressquery.RegexEscape(req.Cluster))
		return cnpgInstanceQuery(ctx, hc, base, q, req.Cluster, unitSeconds, req.Start, req.End, stepSeconds(req.Resolution))
	}
}

// --- Filter-value discovery: Prometheus's label-values API ---

// NewPrometheusFilterValuesSource returns the production MetricsFilterValuesSource,
// backed by Prometheus's /api/v1/label/<name>/values endpoint.
func NewPrometheusFilterValuesSource(base string, hc *http.Client) MetricsFilterValuesSource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, namespace, app string, port int32, label string) ([]string, error) {
		match := fmt.Sprintf(`traefik_service_requests_total{service=%q}`, traefikServiceLabel(namespace, app, port))
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
		if err := core.DecodeUpstreamJSON(resp.Body, &lr); err != nil {
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
