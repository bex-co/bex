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

package logs

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// loki.go is the durable-history backend: a LogHistorySource backed by Loki's
// query_range API (docs/ADR010-observability.md). The domain depends only on the
// injected LogHistorySource; the transport here is thin and the tricky parts —
// the LogQL builder and the stream-response parser — are pure and unit-tested,
// exactly like metrics' Prometheus source. Loki owns metrics? no: Prometheus
// owns metrics, Loki owns logs — this is the logs sibling of NewPrometheus*.

// lokiLookback is the default lower time bound for a query that carries none:
// the point of durable history is a real window, so an unbounded "recent logs"
// read still reaches back across restarts. Matches the shipped retention
// (docs/ADR010-observability.md) so a bare query can surface anything still stored.
const lokiLookback = 7 * 24 * time.Hour

// NewLokiSource returns the production LogHistorySource, backed by a Loki
// query_range over the streams the log-shipper DaemonSet labels with
// namespace/app/container (deploy/gitops/base/loki.yaml). base is BEX_LOKI_URL;
// hc nil => core.UpstreamClient (bounded, codex round-8 #10 — mirrors
// NewPrometheusRequestSource).
func NewLokiSource(base string, hc *http.Client) LogHistorySource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, namespace string, q LogQuery) ([]LogEntry, error) {
		start, end := lokiRange(q, time.Now())
		u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", base, url.Values{
			"query": {lokiQueryFor(namespace, q)},
			"start": {strconv.FormatInt(start.UnixNano(), 10)},
			"end":   {strconv.FormatInt(end.UnixNano(), 10)},
			"limit": {strconv.FormatInt(lokiLimit(q), 10)},
			// Render's direction decides which end of the window `limit` keeps:
			// backward (default) the newest lines, forward the oldest.
			"direction": {lokiDirection(q)},
		}.Encode())

		var lr lokiRangeResponse
		if err := lokiGet(ctx, hc, u, &lr); err != nil {
			return nil, err
		}
		return parseLokiStreams(lr, q)
	}
}

// NewLokiLabelValuesSource returns the production LogLabelValuesSource, backed by
// Loki's /label/<name>/values API scoped to the query's stream selector — so the
// values a caller discovers are only ever those of the App they asked about
// (tenancy: an unscoped label-values call would enumerate every tenant's pods).
func NewLokiLabelValuesSource(base string, hc *http.Client) LogLabelValuesSource {
	if hc == nil {
		hc = core.UpstreamClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, namespace, label string, q LogQuery) ([]string, error) {
		start, end := lokiRange(q, time.Now())
		// Selector only: Loki's label-values API takes a stream selector, not a
		// line pipeline, so the line filters (text/path/host) don't apply here.
		u := fmt.Sprintf("%s/loki/api/v1/label/%s/values?%s", base, url.PathEscape(lokiLabelFor(label)), url.Values{
			"query": {lokiSelectorFor(namespace, q)},
			"start": {strconv.FormatInt(start.UnixNano(), 10)},
			"end":   {strconv.FormatInt(end.UnixNano(), 10)},
		}.Encode())

		var lv lokiLabelValuesResponse
		if err := lokiGet(ctx, hc, u, &lv); err != nil {
			return nil, err
		}
		if lv.Status != "" && lv.Status != "success" {
			return nil, fmt.Errorf("loki status %q", lv.Status)
		}
		return lv.Data, nil
	}
}

// lokiGet performs a Loki GET and decodes the JSON body into out. Shared by the
// query_range and label-values sources so both surface an unreachable or unhappy
// Loki as an error instead of an empty page.
func lokiGet(ctx context.Context, hc *http.Client, u string, out any) error {
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	resp, err := hc.Do(httpReq)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("loki: status %d", resp.StatusCode)
	}
	if err := core.DecodeUpstreamJSON(resp.Body, out); err != nil {
		return fmt.Errorf("decode loki response: %w", err)
	}
	return nil
}

// lokiLabelFor translates a domain label name (DiscoverableLabels — Render's
// vocabulary) into the stream label this store actually carries. Only the two that
// differ need a case; the rest are the same word. Validation is the service's
// (LogLabelValues rejects an unknown label), and `host` never reaches here — it is
// answered from the App's own URLs, not from a stream label. Keeping the mapping
// here is what lets a different store back the same domain verb.
func lokiLabelFor(label string) string {
	switch label {
	case LabelStatusCode:
		return "status"
	case LabelInstance:
		return "pod"
	default:
		return label
	}
}

// lokiRange resolves the [start, end] window for a query: an explicit bound wins,
// an absent one defaults (end=now, start=end-lokiLookback). now is passed in so
// the source stays testable.
func lokiRange(q LogQuery, now time.Time) (start, end time.Time) {
	end = q.End
	if end.IsZero() {
		end = now
	}
	start = q.Since
	if start.IsZero() {
		start = end.Add(-lokiLookback)
	}
	return start, end
}

// lokiLimit is the query_range line cap. q.Limit is already clamped to Render's
// paging range by normalized(); Logs passes an unnormalized tail, so guard the
// zero/negative case to a sane default rather than asking Loki for everything.
func lokiLimit(q LogQuery) int64 {
	if q.Limit <= 0 {
		return defaultLogTail
	}
	return q.Limit
}

// lokiDirection maps the query's direction onto Loki's. Backward (Render's
// default) has Loki return the newest lines in the window, forward the oldest;
// parseLokiStreams re-sorts either way, so the slice is always oldest-first.
func lokiDirection(q LogQuery) string {
	if q.Direction == DirectionForward {
		return DirectionForward
	}
	return DirectionBackward
}

// lokiSelectorFor builds the LogQL *stream selector* — the `{...}` half — for a
// query: the App's namespace/name, the streams its `type` asks for, and every
// structured filter that is a stream label (level, instance, method, statusCode).
// This is what scopes a read (and a label-values discovery) to one App: a caller
// can never widen it to another tenant's streams, because App and namespace are
// always equality matchers on values the service resolved, not the caller.
//
// Every interpolated value goes through %q (Go/LogQL double-quoted escaping), so
// a service name or filter value can never break out of a matcher and inject a
// selector or a line filter — the label-injection guard, covered in tests.
func lokiSelectorFor(namespace string, q LogQuery) string {
	matchers := []string{fmt.Sprintf("namespace=%q", namespace)}
	switch {
	case q.Database != "":
		matchers = append(matchers, fmt.Sprintf("database=%q", q.Database))
	case q.KeyValue != "":
		matchers = append(matchers, fmt.Sprintf("keyvalue=%q", q.KeyValue))
	default:
		matchers = append(matchers, fmt.Sprintf("app=%q", q.App))
	}
	if q.Database == "" && q.KeyValue == "" {
		if m := lokiTypeMatcher(q); m != "" {
			matchers = append(matchers, m)
		}
	}
	// The labels the shipper attaches; `path`/`host` are deliberately absent —
	// they are line-only (see the cardinality budget in log-shipper.yaml) and are
	// applied as line filters by lokiQueryFor instead.
	add := func(label string, values []string) {
		if m := labelMatcher(label, values); m != "" {
			matchers = append(matchers, m)
		}
	}
	add(lokiLabelFor(LabelLevel), q.Level)
	add(lokiLabelFor(LabelInstance), q.Instance)
	add(lokiLabelFor(LabelMethod), q.Method)
	// statusCode is the one filter with a class shorthand (`4xx`), expanded here —
	// at the filter that owns the vocabulary, not inside labelMatcher, which would
	// silently rewrite a `path` or `host` value that happened to look like one.
	add(lokiLabelFor(LabelStatusCode), statusClasses(q.StatusCode))
	return "{" + strings.Join(matchers, ", ") + "}"
}

// lokiTypeMatcher selects the streams a query's `type` asks for. App logs are
// identified by the container they came from rather than by the `type` label:
// container="app" holds for streams shipped before the label existed too, so a
// query still finds history the shipper labelled the old way. Request streams —
// Traefik's access log, attributed to the App — carry no container label at all,
// which is what `container=~"app|"` (match "app" OR absent) unions in when the
// caller asks for both. Build streams carry type="build" (w7/m28 — shipped by
// the build_pods pipeline in log-shipper.yaml). Returns "" when all three types
// are wanted (no restriction — all streams for this App).
func lokiTypeMatcher(q LogQuery) string {
	if len(q.Types) == 0 {
		// No explicit type filter: backward-compat union of app + request
		// (the default Render clients see). Build logs are only included when
		// the caller explicitly requests them — `type=build` or `type=all`.
		return fmt.Sprintf("container=~%q", core.AppContainer+"|")
	}
	wantApp, wantRequest, wantBuild := q.wants(LogTypeApp), q.wants(LogTypeRequest), q.wants(LogTypeBuild)
	switch {
	case wantApp && wantRequest && wantBuild:
		return "" // explicit all-three: no type restriction
	case wantApp && wantRequest:
		return fmt.Sprintf("container=~%q", core.AppContainer+"|")
	case wantApp && wantBuild:
		return fmt.Sprintf("type=~%q", "app|build")
	case wantRequest && wantBuild:
		return fmt.Sprintf("type=~%q", "request|build")
	case wantBuild:
		return fmt.Sprintf("type=%q", LogTypeBuild)
	case wantRequest:
		return fmt.Sprintf("type=%q", LogTypeRequest)
	default: // app only
		return fmt.Sprintf("container=%q", core.AppContainer)
	}
}

// lokiQueryFor builds the full LogQL for a query: the stream selector, the
// case-insensitive text line filter, and the request-only line filters (`path`,
// `host`) parsed out of the JSON access line with LogQL's `json` stage — the
// query-time half of the cardinality budget's "unbounded fields stay in the line".
// An app log line has no RequestPath/RequestHost (and typically isn't JSON at all),
// so either filter also narrows the read to request logs, which is exactly Render's
// "filter request logs by their path/host".
func lokiQueryFor(namespace string, q LogQuery) string {
	query := lokiSelectorFor(namespace, q)
	if q.Search != "" {
		// (?i) + a quoted-meta literal == the same case-insensitive substring
		// match keep() applies to the pod-log path, so the two backends filter
		// text identically; QuoteMeta neutralizes any regex metacharacters.
		query += fmt.Sprintf(" |~ %q", "(?i)"+regexp.QuoteMeta(q.Search))
	}
	if len(q.Path) == 0 && len(q.Host) == 0 {
		return query
	}
	// One `json` stage extracting only the two fields we filter on (not the whole
	// line — the access log has ~20 fields), then a label filter per field.
	query += ` | json request_path="RequestPath", request_host="RequestHost"`
	if m := labelMatcher("request_path", q.Path); m != "" {
		query += " | " + m
	}
	if m := labelMatcher("request_host", q.Host); m != "" {
		query += " | " + m
	}
	return query
}

// labelMatcher renders one LogQL matcher for a Render filter's value set. Render's
// filters are arrays (OR within a filter) and accept `*` wildcards, so:
//
//	one plain value       => name="v"            (equality — the cheapest matcher)
//	several, or wildcards => name=~"^(a|b.*)$"   (anchored alternation)
//
// Every value is regexp.QuoteMeta'd before `*` is restored as `.*`, so a `.` stays
// a literal dot and no caller-supplied metacharacter (or quote) can escape the
// matcher. Render also documents full regex support; bex honors the wildcard
// subset and treats the rest as literals — a documented divergence, not a silent
// one (docs/ADR010-observability.md § Log filters). An empty set matches nothing
// and renders as "" (no matcher at all).
func labelMatcher(name string, values []string) string {
	if len(values) == 0 {
		return ""
	}
	if len(values) == 1 && !strings.Contains(values[0], "*") {
		return fmt.Sprintf("%s=%q", name, values[0])
	}
	alts := make([]string, 0, len(values))
	for _, v := range values {
		alts = append(alts, strings.ReplaceAll(regexp.QuoteMeta(v), `\*`, ".*"))
	}
	return fmt.Sprintf("%s=~%q", name, "^("+strings.Join(alts, "|")+")$")
}

// statusClasses rewrites Render's status-code class shorthand (`2xx`) as the
// wildcard labelMatcher already understands (`2*`) — status codes are three digits,
// so the two describe the same set. It mirrors the metrics feature's `2xx` -> `2..`
// mapping so logs and metrics speak one status vocabulary. Any other value passes
// through untouched.
func statusClasses(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	out := make([]string, 0, len(values))
	for _, v := range values {
		if s := strings.TrimSpace(v); len(s) == 3 && s[0] >= '1' && s[0] <= '5' && strings.EqualFold(s[1:], "xx") {
			out = append(out, string(s[0])+"*")
			continue
		}
		out = append(out, v)
	}
	return out
}

// lokiRangeResponse is the subset of Loki's query_range result bex reads: a
// streams matrix, each stream a label set + [ns-timestamp, line] value pairs.
type lokiRangeResponse struct {
	Status string `json:"status"`
	Data   struct {
		ResultType string `json:"resultType"`
		Result     []struct {
			Stream map[string]string `json:"stream"`
			Values [][]string        `json:"values"` // [ ["<unixNanos>", "<line>"], ... ]
		} `json:"result"`
	} `json:"data"`
}

// lokiLabelValuesResponse is Loki's /label/<name>/values shape — the same
// {status, data:[…]} envelope Prometheus uses for its label values.
type lokiLabelValuesResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"`
}

// parseLokiStreams flattens Loki's streams into LogEntry values in the pod-log
// path's shape (service/instance/container labels, RFC3339Nano UTC timestamp),
// sorts them oldest-first, and keeps q.Limit from the end the direction asks for —
// identical ordering and capping to QueryLogs' pod-log branch, so the adapters
// can't tell the backends apart. A request line additionally carries the type /
// method / statusCode labels the shipper attached, which is what lets the REST
// adapter render a truthful `type` per line instead of assuming `app`.
// Unparseable timestamps drop the line rather than fail the query.
func parseLokiStreams(lr lokiRangeResponse, q LogQuery) ([]LogEntry, error) {
	if lr.Status != "" && lr.Status != "success" {
		return nil, fmt.Errorf("loki status %q", lr.Status)
	}
	var out []LogEntry
	for _, st := range lr.Data.Result {
		for _, pair := range st.Values {
			if len(pair) != 2 {
				continue
			}
			ns, err := strconv.ParseInt(pair[0], 10, 64)
			if err != nil {
				continue
			}
			out = append(out, LogEntry{
				Timestamp: time.Unix(0, ns).UTC().Format(time.RFC3339Nano),
				Message:   pair[1],
				Labels:    entryLabels(st.Stream, q),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	return q.capToLimit(out), nil
}

// entryLabels renders a Loki stream's label set in the shape the adapters expect:
// Render's names (service/instance/type/level/method/statusCode) rather than the
// store's (app/pod/status). Labels a stream doesn't carry — no `pod` on a request
// stream, no `method` on an app stream — are simply absent, never faked.
func entryLabels(stream map[string]string, q LogQuery) map[string]string {
	labels := map[string]string{
		"service": labelOr(stream["app"], q.App),
		LabelType: labelOr(stream[LabelType], LogTypeApp),
	}
	// Absent labels are left absent, never faked: a request line has no pod or
	// container (it came from the edge), an app line no method or statusCode.
	for _, l := range optionalEntryLabels {
		if v := stream[l.loki]; v != "" {
			labels[l.render] = v
		}
	}
	return labels
}

// optionalEntryLabels maps the store's label names onto Render's, for the labels a
// stream may or may not carry. Package-level: parseLokiStreams walks it per line.
var optionalEntryLabels = []struct{ render, loki string }{
	{LabelInstance, "pod"},
	{"container", "container"},
	{LabelLevel, "level"},
	{LabelMethod, "method"},
	{LabelStatusCode, "status"},
}

// labelOr returns the stream label when present, else a fallback — so an entry
// always carries the render labels even if the shipper dropped one.
func labelOr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
