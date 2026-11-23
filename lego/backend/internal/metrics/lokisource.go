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
	"cmp"
	"context"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

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
		series, err := lokiQueryRange(ctx, hc, base, query, req.Start, req.End, req.Resolution)
		if err != nil {
			return nil, err
		}
		// Relabel the store's `status` group label onto Prometheus's `code`
		// vocabulary so a host/path-filtered group-by series legends identically
		// to the unfiltered Prometheus one, and a host breakdown's extracted
		// request host onto Render's `host`.
		for i := range series {
			if v, ok := series[i].Labels["status"]; ok {
				delete(series[i].Labels, "status")
				series[i].Labels["code"] = v
			}
			if v, ok := series[i].Labels[lokiLabelRequestHost]; ok {
				delete(series[i].Labels, lokiLabelRequestHost)
				series[i].Labels[groupByHost] = v
			}
		}
		return series, nil
	}
}

// lokiQueryRange runs one LogQL metric query_range. Loki takes nanosecond
// bounds (like NewLokiSource), rounded up to the step, and a step in seconds;
// the reply is a Prometheus-shaped matrix.
func lokiQueryRange(ctx context.Context, hc *http.Client, base, query string, start, end time.Time, resolution time.Duration) ([]MetricSeries, error) {
	step := stepSeconds(resolution)
	from, to := lokiStepBounds(start, end, step)
	u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", base, url.Values{
		"query": {query},
		"start": {strconv.FormatInt(from, 10)},
		"end":   {strconv.FormatInt(to, 10)},
		"step":  {strconv.FormatInt(step, 10)},
	}.Encode())
	return queryRangeMatrix(ctx, hc, u, "loki")
}

// lokiStepBounds rounds a read's bounds UP to the step, in nanoseconds (w4/m175).
// Loki's query frontend (align_queries_with_step) floors start and end to the
// step, and each point t counts (t-step, t], so a floored end dropped every
// request after the last boundary. Rounded up, the last point is the bucket
// that contains end and the first the bucket that contains start.
func lokiStepBounds(start, end time.Time, step int64) (int64, int64) {
	stepNs := step * int64(time.Second)
	if stepNs <= 0 {
		return start.UnixNano(), end.UnixNano()
	}
	ceil := func(t time.Time) int64 {
		n := t.UnixNano()
		if r := n % stepNs; r != 0 {
			n += stepNs - r
		}
		return n
	}
	return ceil(start), ceil(end)
}

// lokiRequestQueryFor builds the LogQL metric query for a host/path-filtered
// request read. http_requests uses count_over_time (matched lines per step
// bucket, matching the Prometheus path's sum(increase(...)) so a filter never
// rescales the chart); http_latency uses quantile_over_time over the unwrapped
// Duration field, nanoseconds converted to seconds. The [Ns] window is always
// stepSeconds(req.Resolution) — the same step the matrix is sampled at. Every
// interpolated value is %q-escaped, so a service name or filter value can never
// break out of a matcher (the same label-injection guard as logs/loki.go).
// Returns "" for any metric that has no per-request host/path axis (bandwidth)
// — those are rejected upstream anyway.
func lokiRequestSelector(namespace, app string, statusCodes []string) string {
	matchers := []string{
		fmt.Sprintf("namespace=%q", namespace),
		fmt.Sprintf("app=%q", app),
		fmt.Sprintf("type=%q", "request"),
	}
	// StatusCodes filters the `status` stream label on request streams;
	// codesMatcher expands class shorthands (`2xx` -> `2..`) and ORs multiple
	// values (`^(4..|500|502)$`).
	if c := codesMatcher(statusCodes); c != "" {
		matchers = append(matchers, fmt.Sprintf("status=~%q", c))
	}
	return "{" + strings.Join(matchers, ", ") + "}"
}

func lokiRequestQueryFor(req RequestMetricsRequest) string {
	selector := lokiRequestSelector(req.Namespace, req.App, req.StatusCodes)
	window := stepSeconds(req.Resolution)

	switch req.Metric {
	case MetricHTTPLatency:
		pipeline := lokiRequestPipeline(req, true)
		// Aggregate samples inside the quantile, retaining only the requested
		// group. Otherwise every parsed path/stream gets its own percentile and
		// host-only reads can exceed Loki's series limit.
		return fmt.Sprintf("quantile_over_time(%s, %s%s | unwrap latency_ns [%ds]) by (%s) / %s",
			strconv.FormatFloat(lokiQuantile(req.Quantile), 'g', -1, 64), selector, pipeline, window, lokiGroupLabel(req.GroupBy), lokiLatencyNanoDivisor)
	case MetricHTTPRequests:
		// count_over_time: request lines in the step bucket (unit: count).
		inner := fmt.Sprintf("count_over_time(%s%s [%ds])", selector, lokiRequestPipeline(req, false), window)
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
	fields := []string{lokiLabelRequestHost + `="RequestHost"`, lokiLabelRequestPath + `="RequestPath"`}
	if latency {
		fields = append([]string{`latency_ns="Duration"`}, fields...)
	}
	p := " | json " + strings.Join(fields, ", ")
	if f := lokiOrFilter(lokiLabelRequestHost, req.Hosts); f != "" {
		p += " | " + f
	}
	if f := lokiPathFilter(lokiLabelRequestPath, req.Paths); f != "" {
		p += " | " + f
	}
	// Last, so the timestamp templates run only on lines the filters kept.
	return p + lokiEntryWindow(req.Start, req.End)
}

// lokiOrFilter renders one label filter: a single value keeps the plain
// equality form byte-for-byte; several values are ORed (Render ORs repeated
// host/path values), and distinct fields stay separate pipeline stages, so
// they AND.
func lokiOrFilter(field string, values []string) string {
	parts := make([]string, 0, len(values))
	seen := make(map[string]bool, len(values))
	for _, v := range values {
		if v == "" || seen[v] {
			continue
		}
		seen[v] = true
		parts = append(parts, fmt.Sprintf("%s=%q", field, v))
	}
	return strings.Join(parts, " or ")
}

// lokiPathFilter renders one path label filter supporting `*` glob wildcards
// (w34/m587). A path without `*` keeps exact equality `field="v"` byte-for-byte;
// a path containing `*` compiles to an anchored RE2 regex (`field=~"^...$"`)
// with every literal segment escaped via regexp.QuoteMeta and `%q`-escaped so
// regex metacharacters are literal and LogQL injection is impossible.
func lokiPathFilter(field string, paths []string) string {
	parts := make([]string, 0, len(paths))
	seen := make(map[string]bool, len(paths))
	for _, p := range paths {
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		if !strings.Contains(p, "*") {
			parts = append(parts, fmt.Sprintf("%s=%q", field, p))
			continue
		}
		parts = append(parts, fmt.Sprintf("%s=~%q", field, wildcardPathRegex(p)))
	}
	return strings.Join(parts, " or ")
}

// wildcardPathRegex turns a `*`-glob path pattern (such as `/api/*/items`) into
// an anchored RE2 regular expression (`^/api/.*/items$`). Consecutive `*` runs
// collapse into one `.*`, and every literal segment is escaped with
// regexp.QuoteMeta so `.`, `+`, `?`, `(`, `)`, etc. match literally.
func wildcardPathRegex(pattern string) string {
	var b strings.Builder
	b.WriteByte('^')
	for i := 0; i < len(pattern); {
		if pattern[i] == '*' {
			b.WriteString(".*")
			for i < len(pattern) && pattern[i] == '*' {
				i++
			}
			continue
		}
		next := strings.IndexByte(pattern[i:], '*')
		if next < 0 {
			b.WriteString(regexp.QuoteMeta(pattern[i:]))
			break
		}
		b.WriteString(regexp.QuoteMeta(pattern[i : i+next]))
		i += next
	}
	b.WriteByte('$')
	return b.String()
}

// lokiTSSec and lokiTSNano are lokiEntryWindow's temporary entry-timestamp
// labels; the bex_ prefix keeps them clear of shipped stream labels.
const (
	lokiTSSec  = "bex_ts_s"
	lokiTSNano = "bex_ts_ns"
)

// lokiEntryWindow renders the stage that keeps only entries whose own Loki
// timestamp lies in [start, end] (w4/m175). lokiStepBounds widens the evaluation
// to whole step buckets, so without it the first and last buckets also counted
// requests just before start or just after end. The comparison is on the entry
// timestamp, split into integer seconds and the nanosecond remainder: LogQL's
// numeric label filter parses float64, which cannot hold a Unix-nanosecond
// value exactly, while each half is exact. Both temporary labels are dropped
// before any aggregation, so they never become series dimensions. A zero bound leaves that side open.
func lokiEntryWindow(start, end time.Time) string {
	var conds []string
	if !start.IsZero() {
		s, ns := start.Unix(), start.Nanosecond()
		conds = append(conds, fmt.Sprintf("(%[1]s > %[3]d or (%[1]s == %[3]d and %[2]s >= %[4]d))", lokiTSSec, lokiTSNano, s, ns))
	}
	if !end.IsZero() {
		s, ns := end.Unix(), end.Nanosecond()
		conds = append(conds, fmt.Sprintf("(%[1]s < %[3]d or (%[1]s == %[3]d and %[2]s <= %[4]d))", lokiTSSec, lokiTSNano, s, ns))
	}
	if len(conds) == 0 {
		return ""
	}
	return fmt.Sprintf(` | label_format %[1]s="{{ unixEpoch (__timestamp__) }}", %[2]s="{{ (__timestamp__).Nanosecond }}" | %[3]s | drop %[1]s, %[2]s`,
		lokiTSSec, lokiTSNano, strings.Join(conds, " and "))
}

// lokiLabelRequestHost and lokiLabelRequestPath are the labels extracted from
// each Traefik access-log line's RequestHost / RequestPath fields.
const (
	lokiLabelRequestHost = "request_host"
	lokiLabelRequestPath = "request_path"

	// lokiPathSuggestionsLimit is Render's documented scan window for
	// GET /metrics/filters/path ("the most recent 5000 log lines as filtered
	// by the provided filters").
	lokiPathSuggestionsLimit = 5000
	// maxPathFilterSuggestions caps the distinct path suggestions returned per
	// request so high-cardinality routes stay bounded while surfacing the most
	// frequent paths first.
	maxPathFilterSuggestions = 100
)

// lokiStreamResponse is the subset of Loki's stream query_range reply read by
// NewLokiPathFilterSuggestionsSource.
type lokiStreamResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"`
		} `json:"result"`
	} `json:"data"`
}

// NewLokiPathFilterSuggestionsSource returns the production
// PathFilterSuggestionsSource backed by Loki's /loki/api/v1/query_range over
// the App's Traefik request-log streams (direction=backward, limit<=5000).
// Each matched line is projected to its RequestPath so the transport transfers
// only paths, then deduplicated and ordered by frequency descending then
// lexicographically ascending, bounded to maxPathFilterSuggestions.
func NewLokiPathFilterSuggestionsSource(base string, hc *http.Client) PathFilterSuggestionsSource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, req PathFilterSuggestionsRequest) ([]string, error) {
		if req.Start.After(req.End) {
			return []string{}, nil
		}
		limit := req.Limit
		if limit <= 0 || limit > lokiPathSuggestionsLimit {
			limit = lokiPathSuggestionsLimit
		}
		end := req.End
		if !end.IsZero() {
			// Loki's query_range end is exclusive; bump by 1 ns so entries
			// stamped exactly at req.End are included.
			end = end.Add(time.Nanosecond)
		}
		u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", base, url.Values{
			"query":     {lokiPathSuggestionsQueryFor(req)},
			"start":     {strconv.FormatInt(req.Start.UnixNano(), 10)},
			"end":       {strconv.FormatInt(end.UnixNano(), 10)},
			"limit":     {strconv.Itoa(limit)},
			"direction": {"backward"},
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
			return nil, fmt.Errorf("loki: status %d", resp.StatusCode)
		}
		var lr lokiStreamResponse
		if err := core.DecodeUpstreamJSON(resp.Body, &lr); err != nil {
			return nil, fmt.Errorf("decode loki response: %w", err)
		}
		if lr.Status != "" && lr.Status != promStatusSuccess {
			return nil, fmt.Errorf("loki status %q", lr.Status)
		}
		counts := map[string]int{}
		for _, st := range lr.Data.Result {
			for _, pair := range st.Values {
				if len(pair) == 2 && pair[1] != "" {
					counts[pair[1]]++
				}
			}
		}
		return topPathSuggestions(counts, maxPathFilterSuggestions), nil
	}
}

// lokiPathSuggestionsQueryFor builds the LogQL stream query for HTTP path
// filter suggestions: scoped to the authorized App's request-log streams,
// applying optional statusCode, host, and path (exact or `*` wildcard) filters,
// and projecting each matching line to its RequestPath.
func lokiPathSuggestionsQueryFor(req PathFilterSuggestionsRequest) string {
	q := lokiRequestSelector(req.Namespace, req.App, req.StatusCodes)
	q += ` | json ` + lokiLabelRequestHost + `="RequestHost", ` + lokiLabelRequestPath + `="RequestPath"`
	if f := lokiOrFilter(lokiLabelRequestHost, req.Hosts); f != "" {
		q += " | " + f
	}
	if f := lokiPathFilter(lokiLabelRequestPath, req.Paths); f != "" {
		q += " | " + f
	}
	q += fmt.Sprintf(` | %s!="" | line_format "{{.%s}}"`, lokiLabelRequestPath, lokiLabelRequestPath)
	return q
}

// topPathSuggestions returns up to limit distinct paths ordered by frequency
// descending, breaking ties lexicographically ascending.
func topPathSuggestions(counts map[string]int, limit int) []string {
	if len(counts) == 0 || limit <= 0 {
		return []string{}
	}
	paths := slices.Collect(maps.Keys(counts))
	slices.SortFunc(paths, func(a, b string) int {
		if c := cmp.Compare(counts[b], counts[a]); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	})
	if len(paths) > limit {
		paths = paths[:limit]
	}
	return paths
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
	case groupByHost:
		// Not a stream label: lokiRequestPipeline extracts it from each line.
		return lokiLabelRequestHost
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
