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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// lokisource.go is the request-log metrics backend: a RequestMetricsSource that
// serves host/path-filtered request time-series from the Traefik access log in
// Loki, the one store that carries a per-request host/path axis (w5/m58).
// Prometheus's Traefik counters have no host/path label, so the request-metrics
// service routes any host/path-filtered read here instead. Loki's metric
// query_range returns the same {resultType:"matrix"} envelope as Prometheus, so
// this reuses queryRangeMatrix/parsePromMatrix; only the LogQL builder is new,
// and it is pure and unit-tested like promQueryFor.

// lokiLatencyNanoDivisor converts the Traefik access log's Duration field
// (nanoseconds) to seconds, matching the Prometheus latency series' unit
// (histogram_quantile over *_seconds_bucket).
const lokiLatencyNanoDivisor = "1000000000"

// NewLokiRequestMetricsSource returns the production RequestLogMetrics source,
// backed by a LogQL metric query_range over the App's request-log streams. base
// is BEX_LOKI_URL; hc nil => http.DefaultClient (mirrors NewLokiSource). It is
// only ever called with Host or Path set (the service guards that), so an empty
// query (an unsupported metric) yields no series rather than an error.
func NewLokiRequestMetricsSource(base string, hc *http.Client) RequestMetricsSource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		query := lokiRequestQueryFor(req)
		if query == "" {
			return []MetricSeries{}, nil
		}
		// Loki's query_range takes nanosecond bounds (like NewLokiSource) and a
		// step in seconds; the metric reply is a Prometheus-shaped matrix.
		u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", base, url.Values{
			"query": {query},
			"start": {strconv.FormatInt(req.Start.UnixNano(), 10)},
			"end":   {strconv.FormatInt(req.End.UnixNano(), 10)},
			"step":  {strconv.FormatInt(stepSeconds(req.Resolution), 10)},
		}.Encode())
		series, err := queryRangeMatrix(ctx, hc, u, "loki")
		if err != nil {
			return nil, err
		}
		// Relabel the store's `status` group label onto Prometheus's `code`
		// vocabulary so a host/path-filtered group-by series legends identically
		// to the unfiltered Prometheus one.
		for i := range series {
			if v, ok := series[i].Labels["status"]; ok {
				delete(series[i].Labels, "status")
				series[i].Labels["code"] = v
			}
		}
		return series, nil
	}
}

// lokiRequestQueryFor builds the LogQL metric query for a host/path-filtered
// request read. http_requests uses rate (matched lines/second, matching the
// Prometheus path's sum(rate(...)) req/s so a filter never rescales the chart);
// http_latency uses quantile_over_time over the unwrapped Duration field,
// nanoseconds converted to seconds. Every interpolated value is %q-escaped, so a
// service name or filter value can never break out of a matcher (the same
// label-injection guard as logs/loki.go). Returns "" for any metric that has no
// per-request host/path axis (bandwidth) — those are rejected upstream anyway.
func lokiRequestQueryFor(req RequestMetricsRequest) string {
	matchers := []string{
		fmt.Sprintf("namespace=%q", req.Namespace),
		fmt.Sprintf("app=%q", req.App),
		fmt.Sprintf("type=%q", "request"),
	}
	// StatusCode is a stream label on the request streams; codeMatcher expands a
	// class shorthand (2xx -> 2..) — a bare 3-digit code matches itself as a regex.
	if c := codeMatcher(req.StatusCode); c != "" {
		matchers = append(matchers, fmt.Sprintf("status=~%q", c))
	}
	selector := "{" + strings.Join(matchers, ", ") + "}"
	window := stepSeconds(req.Resolution)

	switch req.Metric {
	case MetricHTTPLatency:
		pipeline := lokiRequestPipeline(req, true)
		return fmt.Sprintf("quantile_over_time(%s, %s%s | unwrap latency_ns [%ds]) / %s",
			strconv.FormatFloat(lokiQuantile(req.Quantile), 'g', -1, 64), selector, pipeline, window, lokiLatencyNanoDivisor)
	case MetricHTTPRequests:
		inner := fmt.Sprintf("rate(%s%s [%ds])", selector, lokiRequestPipeline(req, false), window)
		if g := lokiGroupLabel(req.GroupBy); g != "" {
			return fmt.Sprintf("sum by (%s) (%s)", g, inner)
		}
		return fmt.Sprintf("sum(%s)", inner)
	default:
		return ""
	}
}

// lokiRequestPipeline renders the LogQL pipeline that extracts the access-log
// fields the read filters on and applies the host/path line filters. latency adds
// the numeric Duration field the quantile read unwraps.
func lokiRequestPipeline(req RequestMetricsRequest, latency bool) string {
	fields := []string{`request_host="RequestHost"`, `request_path="RequestPath"`}
	if latency {
		fields = append([]string{`latency_ns="Duration"`}, fields...)
	}
	p := " | json " + strings.Join(fields, ", ")
	if req.Host != "" {
		p += fmt.Sprintf(" | request_host=%q", req.Host)
	}
	if req.Path != "" {
		p += fmt.Sprintf(" | request_path=%q", req.Path)
	}
	return p
}

// lokiGroupLabel maps a Render groupBy onto the request-log stream label. Both
// status and method ride stream labels the shipper attaches; the status series is
// relabelled to `code` after the read to match the Prometheus grouped output.
func lokiGroupLabel(groupBy string) string {
	switch strings.ToLower(groupBy) {
	case "status", "code":
		return "status"
	case "method":
		return "method"
	default:
		return ""
	}
}

// lokiQuantile guards the pure builder's percentile (the service resolves the
// default before calling, but a direct unit-test call may not).
func lokiQuantile(q float64) float64 {
	if q <= 0 {
		return defaultQuantile
	}
	return q
}
