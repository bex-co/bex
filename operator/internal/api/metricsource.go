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
)

// metricsource.go holds the production metrics backends — the metrics analogue of
// podlogs.go. Both keep their k8s/HTTP clients OUT of Core (the domain layer
// depends only on the injected ResourceMetricsSource / RequestMetricsSource
// function types), so Core stays apiserver-thin and every read is faked in tests.
// The transport is thin; the tricky parts (metrics-server quantity parsing, the
// PromQL builder, the Prometheus matrix parser) are pure functions unit-tested
// without a live backend.

// --- Resource metrics: metrics.k8s.io (metrics-server) ---

// NewResourceMetricsSource returns the production ResourceMetricsSource, reading
// PodMetrics from metrics-server's aggregated API (metrics.k8s.io/v1beta1). It
// uses the clientset's REST client with an explicit AbsPath — metrics.k8s.io is
// an aggregated API the typed clientset doesn't expose, and DoRaw avoids needing
// its scheme registered (the pure parser owns decoding).
func NewResourceMetricsSource(cs kubernetes.Interface) ResourceMetricsSource {
	return func(ctx context.Context, namespace, app string) ([]PodResourceUsage, error) {
		raw, err := cs.Discovery().RESTClient().Get().
			AbsPath("/apis/metrics.k8s.io/v1beta1/namespaces", namespace, "pods").
			Param("labelSelector", podLabelApp+"="+app).
			DoRaw(ctx)
		if err != nil {
			return nil, err
		}
		return parsePodMetrics(raw)
	}
}

// podMetricsList is the subset of metrics.k8s.io/v1beta1 PodMetricsList bex reads:
// per-pod, per-container usage as Quantity strings ("12m" CPU, "34Mi" memory).
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
// container usage into cores + bytes. Unparseable quantities are treated as zero
// (a bad sample shouldn't fail the whole read).
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
// by a Prometheus range query over Traefik's metrics. base is the Prometheus base
// URL (e.g. http://prometheus.monitoring:9090); hc defaults to http.DefaultClient.
// The Traefik service-label selector is a heuristic (see promQueryFor) that
// t005/w1 tunes to the cluster's actual ingress labels.
func NewPrometheusRequestSource(base string, hc *http.Client) RequestMetricsSource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		q := promQueryFor(req)
		u := fmt.Sprintf("%s/api/v1/query_range?%s", base, url.Values{
			"query": {q},
			"start": {strconv.FormatInt(req.Start.Unix(), 10)},
			"end":   {strconv.FormatInt(req.End.Unix(), 10)},
			"step":  {strconv.FormatInt(stepSeconds(req.Resolution), 10)},
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
		dec := json.NewDecoder(resp.Body)
		var pr promRangeResponse
		if err := dec.Decode(&pr); err != nil {
			return nil, fmt.Errorf("decode prometheus response: %w", err)
		}
		return parsePromMatrix(pr)
	}
}

func stepSeconds(res time.Duration) int64 {
	if res <= 0 {
		res = defaultResolution
	}
	return int64(res / time.Second)
}

// promQueryFor builds the PromQL range query for a request metric over Traefik's
// counters:
//
//   - http_requests: rate of traefik_service_requests_total (requests/second)
//   - bandwidth:     rate of traefik_service_responses_bytes_total (bytes/second)
//   - http_latency:  histogram_quantile over traefik_service_request_duration_seconds_bucket
//
// The App is matched by a service-label regex (Traefik names a k8s service like
// "<ns>-<ingress>-...@kubernetes"); statusCode filters the `code` label; groupBy
// breaks the result into per-label series. host/path have no Traefik service-level
// label, so they are accepted (Render vocabulary) but not applied here — a
// documented deviation, like the logs adapter's unimplemented request filters.
func promQueryFor(req RequestMetricsRequest) string {
	sel := []string{fmt.Sprintf(`service=~".*%s.*"`, promEscape(req.App))}
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
		return sumRate("traefik_service_responses_bytes_total", matchers, window, groupLabel(req.GroupBy))
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

// codeMatcher turns a Render statusCode filter into a Prometheus `code` regex:
// "2xx"/"5xx" => "2.."/"5..", a bare code ("500") stays literal, "" => no filter.
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

// groupLabel maps a Render groupBy onto a Traefik service-metric label. Only
// status/method have service-level labels; anything else groups nothing.
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

// parsePromMatrix maps a Prometheus matrix response onto Core MetricSeries: one
// series per result (labels = its metric labels), points RFC3339-stamped and
// oldest-first. NaN/parse-failures drop the point rather than fail the series.
func parsePromMatrix(pr promRangeResponse) ([]MetricSeries, error) {
	if pr.Status != "" && pr.Status != "success" {
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
