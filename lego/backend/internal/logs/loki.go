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
	"encoding/json"
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
// hc nil => http.DefaultClient (mirrors NewPrometheusRequestSource).
func NewLokiSource(base string, hc *http.Client) LogHistorySource {
	if hc == nil {
		hc = http.DefaultClient
	}
	base = strings.TrimRight(base, "/")
	return func(ctx context.Context, namespace string, q LogQuery) ([]LogEntry, error) {
		start, end := lokiRange(q, time.Now())
		u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", base, url.Values{
			"query":     {lokiQueryFor(namespace, q)},
			"start":     {strconv.FormatInt(start.UnixNano(), 10)},
			"end":       {strconv.FormatInt(end.UnixNano(), 10)},
			"limit":     {strconv.FormatInt(lokiLimit(q), 10)},
			"direction": {"backward"}, // newest-first so `limit` keeps the newest lines
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
		var lr lokiRangeResponse
		if err := json.NewDecoder(resp.Body).Decode(&lr); err != nil {
			return nil, fmt.Errorf("decode loki response: %w", err)
		}
		return parseLokiStreams(lr, q)
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

// lokiQueryFor builds the LogQL for an App's application logs: a label selector
// scoped to the App's namespace/name/container (the labels the shipper attaches,
// matching the pod-log path's app-container-only read), plus a case-insensitive
// line filter for the text search. Every interpolated value goes through %q
// (Go/LogQL double-quoted escaping), so a service name can never break out of a
// label matcher or inject a selector — label-injection guard, covered in tests.
func lokiQueryFor(namespace string, q LogQuery) string {
	sel := fmt.Sprintf(`{namespace=%q, app=%q, container=%q}`, namespace, q.App, core.AppContainer)
	if q.Search != "" {
		// (?i) + a quoted-meta literal == the same case-insensitive substring
		// match keep() applies to the pod-log path, so the two backends filter
		// text identically; QuoteMeta neutralizes any regex metacharacters.
		sel += fmt.Sprintf(" |~ %q", "(?i)"+regexp.QuoteMeta(q.Search))
	}
	return sel
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

// parseLokiStreams flattens Loki's streams into LogEntry values in the pod-log
// path's shape (service/instance/container labels, RFC3339Nano UTC timestamp),
// sorts them oldest-first, and keeps the newest q.Limit — identical ordering and
// capping to QueryLogs' pod-log branch, so the adapters can't tell the backends
// apart. Unparseable timestamps drop the line rather than fail the query.
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
				Labels: map[string]string{
					"service":   labelOr(st.Stream["app"], q.App),
					"instance":  st.Stream["pod"],
					"container": labelOr(st.Stream["container"], core.AppContainer),
				},
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Timestamp < out[j].Timestamp })
	if lim := lokiLimit(q); int64(len(out)) > lim {
		out = out[int64(len(out))-lim:] // keep the newest Limit, like collectPodLogs' caller
	}
	return out, nil
}

// labelOr returns the stream label when present, else a fallback — so an entry
// always carries the render labels even if the shipper dropped one.
func labelOr(v, fallback string) string {
	if v != "" {
		return v
	}
	return fallback
}
