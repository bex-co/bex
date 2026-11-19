//go:build lokitiming

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

// The w5/m503 timing harness: it reads a real Loki through the same
// LogHistorySource bex-api serves GET /v1/logs with, one request shape at a
// time, and prints what each shape cost (wall time, Loki round trips, lines
// Loki scanned). It never runs in CI: it needs a reachable Loki, and is run by
// hand against production's through a port-forward (read-only):
//
//	kubectl -n monitoring port-forward svc/loki 13100:3100
//	LOKI_TIMING_URL=http://localhost:13100 \
//	LOKI_TIMING_TARGETS='busy=<namespace>/<app CR name>/<srv- id>;idle=…' \
//	go test -tags lokitiming -run TestLokiTiming -v -timeout 60m ./internal/logs/
//
// LOKI_TIMING_ONLY keeps the shapes whose name holds it (`text`), and
// LOKI_TIMING_TEXT names the term a text shape searches for.
//
// The targets name tenants, so they come from the environment and are never
// written down in the repository.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// timingTransport counts the Loki round trips one shape costs and sums what
// Loki's query_range stats report: lines scanned, queue and execution time,
// and the bytes answered. LOKI_TIMING_TRACE=1 prints each read's window.
type timingTransport struct {
	mu      sync.Mutex
	calls   map[string]int
	scanned int64
	queue   time.Duration
	exec    time.Duration // Loki's own execTime, summed: the in-cluster cost
	bytes   int64         // response bytes, which a port-forward is slow to carry
}

func (t *timingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	start := time.Now()
	resp, err := http.DefaultTransport.RoundTrip(r)
	took := time.Since(start)
	endpoint := r.URL.Path[strings.LastIndex(r.URL.Path, "/")+1:]
	t.mu.Lock()
	t.calls[endpoint]++
	t.mu.Unlock()
	if err != nil || endpoint != "query_range" {
		return resp, err
	}
	body, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	var stats struct {
		Data struct {
			Stats struct {
				Summary struct {
					TotalLinesProcessed int64   `json:"totalLinesProcessed"`
					QueueTime           float64 `json:"queueTime"`
					ExecTime            float64 `json:"execTime"`
				} `json:"summary"`
			} `json:"stats"`
		} `json:"data"`
	}
	if os.Getenv("LOKI_TIMING_TRACE") != "" {
		q := r.URL.Query()
		from, _ := strconv.ParseInt(q.Get("start"), 10, 64)
		to, _ := strconv.ParseInt(q.Get("end"), 10, 64)
		fmt.Printf("  read %s .. %s (%s) limit %s: %d bytes in %s\n", time.Unix(0, from).UTC().Format("01-02T15:04:05"),
			time.Unix(0, to).UTC().Format("01-02T15:04:05"), time.Duration(to-from).Round(time.Second), q.Get("limit"), len(body), took.Round(time.Millisecond))
	}
	if json.Unmarshal(body, &stats) == nil {
		t.mu.Lock()
		t.scanned += stats.Data.Stats.Summary.TotalLinesProcessed
		t.queue += time.Duration(stats.Data.Stats.Summary.QueueTime * float64(time.Second))
		t.exec += time.Duration(stats.Data.Stats.Summary.ExecTime * float64(time.Second))
		t.bytes += int64(len(body))
		t.mu.Unlock()
	}
	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

type timingTarget struct{ name, namespace, app, appID string }

func timingTargets(t *testing.T) []timingTarget {
	var out []timingTarget
	for _, spec := range strings.Split(os.Getenv("LOKI_TIMING_TARGETS"), ";") {
		name, rest, ok := strings.Cut(spec, "=")
		parts := strings.Split(rest, "/")
		if !ok || len(parts) != 3 {
			t.Fatalf("LOKI_TIMING_TARGETS entry %q is not name=namespace/app/appID", spec)
		}
		out = append(out, timingTarget{name, parts[0], parts[1], parts[2]})
	}
	return out
}

// timingShape is one request shape as the REST adapter hands it to QueryLogs.
type timingShape struct {
	window    time.Duration // 0: no startTime, the store's default lookback
	types     []string
	filter    string // "", "text", "status", "level" or "path"
	direction string
}

func (s timingShape) String() string {
	w := "default"
	if s.window > 0 {
		w = s.window.String()
	}
	ty := "all"
	if len(s.types) > 0 {
		ty = strings.Join(s.types, "+")
	}
	f := s.filter
	if f == "" {
		f = "none"
	}
	return fmt.Sprintf("%-8s %-8s %-6s %-8s", w, ty, f, s.direction)
}

// query is the LogQuery QueryLogs hands the store for this shape: scoped to the
// App's incarnation, floored at a Hobby workspace's retention, read one line
// past Render's 100-line page (queryLogPage's probe), normalized.
func (s timingShape) query(target timingTarget, now time.Time) LogQuery {
	q := LogQuery{App: target.app, Types: s.types, Limit: maxLogLimit + 1, Direction: s.direction, probe: true}
	q.appID = target.appID
	q.retentionFloor = now.Add(-7 * 24 * time.Hour)
	if s.window > 0 {
		q.Since = now.Add(-s.window)
	}
	switch s.filter {
	case "text":
		// LOKI_TIMING_TEXT names the term: one the service never logs times the
		// prefilter's no-match read (w5/m520), a common one its full pages.
		q.Search = []string{"error"}
		if term := os.Getenv("LOKI_TIMING_TEXT"); term != "" {
			q.Search = []string{term}
		}
	case "status":
		q.StatusCode = []string{"5xx"}
	case "level":
		q.Level = []string{"error"}
	case "path":
		q.Path = []string{"/api/*"}
	}
	return q.normalized()
}

func timingShapes() []timingShape {
	var out []timingShape
	for _, window := range []time.Duration{time.Hour, 24 * time.Hour, 0, 30 * 24 * time.Hour} {
		for _, types := range [][]string{nil, {LogTypeApp}, {LogTypeRequest}, {LogTypeBuild}} {
			for _, filter := range []string{"", "text", "status", "level", "path"} {
				if (filter == "status" || filter == "path") && len(types) == 1 && types[0] != LogTypeRequest {
					continue // a request-only filter over app or build lines
				}
				if filter == "level" && len(types) == 1 && types[0] == LogTypeRequest {
					continue
				}
				out = append(out, timingShape{window: window, types: types, filter: filter, direction: DirectionBackward})
			}
		}
	}
	// Forward reads of the shapes callers page through.
	for _, window := range []time.Duration{time.Hour, 24 * time.Hour, 0} {
		for _, types := range [][]string{nil, {LogTypeRequest}} {
			for _, filter := range []string{"", "text"} {
				out = append(out, timingShape{window: window, types: types, filter: filter, direction: DirectionForward})
			}
		}
	}
	return out
}

func TestLokiTiming(t *testing.T) {
	base := os.Getenv("LOKI_TIMING_URL")
	if base == "" {
		t.Skip("LOKI_TIMING_URL is not set")
	}
	only := os.Getenv("LOKI_TIMING_ONLY") // a substring of the shape's String()
	for _, target := range timingTargets(t) {
		for _, shape := range timingShapes() {
			if only != "" && !strings.Contains(shape.String(), only) {
				continue
			}
			tr := &timingTransport{calls: map[string]int{}}
			src := NewLokiSource(base, &http.Client{Transport: tr, Timeout: 60 * time.Second})
			// The REST deadline bex-api runs a request under (w5/m286).
			ctx, cancel := context.WithTimeout(context.Background(), 28*time.Second)
			start := time.Now()
			entries, err := src(ctx, target.namespace, shape.query(target, start))
			took := time.Since(start)
			cancel()
			outcome := fmt.Sprintf("%d lines", len(entries))
			if partial, ok := err.(*ScanIncompleteError); ok {
				outcome = fmt.Sprintf("partial %d lines to %s", len(partial.Entries), start.Sub(partial.ScannedTo).Round(time.Minute))
			} else if err != nil {
				outcome = "error: " + err.Error()
			}
			t.Logf("%-5s %s %6.2fs  range=%-3d series=%-3d loki=%5.2fs MB=%5.1f scanned=%-8d %s",
				target.name, shape, took.Seconds(), tr.calls["query_range"], tr.calls["series"],
				tr.exec.Seconds(), float64(tr.bytes)/1e6, tr.scanned, outcome)
		}
	}
}

// TestLokiTimingConcurrent times one shape read by n callers at once, the way a
// scripted client's overlapping requests reach bex-api; queue is the time
// Loki held the pieces before a querier took them.
func TestLokiTimingConcurrent(t *testing.T) {
	base := os.Getenv("LOKI_TIMING_URL")
	if base == "" {
		t.Skip("LOKI_TIMING_URL is not set")
	}
	target := timingTargets(t)[0]
	for _, shape := range []timingShape{
		{types: nil, direction: DirectionBackward},
		{types: []string{LogTypeRequest}, filter: "status", direction: DirectionBackward},
		{types: nil, filter: "text", window: time.Hour, direction: DirectionBackward},
	} {
		for _, n := range []int{1, 4, 8} {
			tr := &timingTransport{calls: map[string]int{}}
			src := NewLokiSource(base, &http.Client{Transport: tr, Timeout: 60 * time.Second})
			var slowest atomic.Int64
			var wg sync.WaitGroup
			start := time.Now()
			for range n {
				wg.Add(1)
				go func() {
					defer wg.Done()
					ctx, cancel := context.WithTimeout(context.Background(), 28*time.Second)
					defer cancel()
					s := time.Now()
					_, _ = src(ctx, target.namespace, shape.query(target, s))
					took := int64(time.Since(s))
					for cur := slowest.Load(); took > cur && !slowest.CompareAndSwap(cur, took); cur = slowest.Load() {
					}
				}()
			}
			wg.Wait()
			t.Logf("%s x%d: all %5.2fs, slowest caller %5.2fs, range=%d series=%d queue=%5.2fs",
				shape, n, time.Since(start).Seconds(), time.Duration(slowest.Load()).Seconds(),
				tr.calls["query_range"], tr.calls["series"], tr.queue.Seconds())
		}
	}
}

// TestLokiTimingPrefilter times candidate LogQL line filters for a `text`
// search (w5/m520 t001): one query_range per window, as the scan's first
// slice sends it, over the request streams and over every stream the
// default read selects. LOKI_TIMING_WINDOWS overrides the windows (Go
// durations, comma-separated).
func TestLokiTimingPrefilter(t *testing.T) {
	base := os.Getenv("LOKI_TIMING_URL")
	if base == "" {
		t.Skip("LOKI_TIMING_URL is not set")
	}
	windows := []time.Duration{time.Hour, 24 * time.Hour}
	if v := os.Getenv("LOKI_TIMING_WINDOWS"); v != "" {
		windows = nil
		for _, w := range strings.Split(v, ",") {
			d, err := time.ParseDuration(w)
			if err != nil {
				t.Fatal(err)
			}
			windows = append(windows, d)
		}
	}
	// Each candidate renders the pipeline for one term. `contains` is the
	// cheapest filter Loki has (case-sensitive, not a superset); `fold` is
	// `(?i)` over the literal, which Loki answers with its own lowercase
	// compare; `classes` writes i and k as explicit classes, which keeps Loki
	// on Go's regexp; `json` renders the request line in LogQL first.
	classes := func(term string) string {
		var b strings.Builder
		b.WriteString("(?i)")
		for _, r := range term {
			switch r {
			case 'i':
				b.WriteString(`[iI\x{0130}]`)
			case 'k':
				b.WriteString(`[kK\x{212A}]`)
			default:
				b.WriteString(regexp.QuoteMeta(string(r)))
			}
		}
		return b.String()
	}
	candidates := []struct {
		name     string
		pipeline func(term string) string
	}{
		{"raw", func(string) string { return "" }},
		{"contains", func(term string) string { return fmt.Sprintf(" |= %q", term) }},
		{"fold", func(term string) string { return fmt.Sprintf(" |~ %q", "(?i)"+regexp.QuoteMeta(term)) }},
		{"classes", func(term string) string { return fmt.Sprintf(" |~ %q", classes(term)) }},
		{"json", func(term string) string {
			return ` | json ip="[\"request_True-Client-Ip\"]", rid="[\"request_Rndr-Id\"]", d="Duration", b="DownstreamContentSize", ua="[\"request_User-Agent\"]"` +
				` | line_format "clientIP=\"{{.ip}}\" requestID=\"{{.rid}}\" responseTimeMS={{.d}} responseBytes={{.b}} userAgent=\"{{.ua}}\""` +
				fmt.Sprintf(" |~ %q", "(?i)"+regexp.QuoteMeta(term))
		}},
	}
	terms := []struct{ shape, term string }{
		{"no match", "zqxtimeoutzq"},
		{"common", "mozilla"},
	}
	for _, target := range timingTargets(t) {
		for _, types := range [][]string{{LogTypeRequest}, nil} {
			for _, window := range windows {
				for _, term := range terms {
					for _, c := range candidates {
						if c.name == "raw" && term.shape != "no match" {
							continue // the same read for either term
						}
						now := time.Now()
						q := LogQuery{App: target.app, Types: types}
						q.appID = target.appID
						tr := &timingTransport{calls: map[string]int{}}
						hc := &http.Client{Transport: tr, Timeout: 120 * time.Second}
						u := fmt.Sprintf("%s/loki/api/v1/query_range?%s", strings.TrimRight(base, "/"), url.Values{
							"query":     {lokiSelectorFor(target.namespace, q) + c.pipeline(term.term)},
							"start":     {strconv.FormatInt(now.Add(-window).UnixNano(), 10)},
							"end":       {strconv.FormatInt(now.UnixNano(), 10)},
							"limit":     {strconv.Itoa(lokiTextScanPage)},
							"direction": {DirectionBackward},
						}.Encode())
						var lr lokiRangeResponse
						err := lokiGet(context.Background(), hc, u, &lr)
						took := time.Since(now)
						lines := 0
						for _, st := range lr.Data.Result {
							lines += len(st.Values)
						}
						outcome := fmt.Sprintf("%d lines", lines)
						if err != nil {
							outcome = "error: " + err.Error()
						}
						ty := "all"
						if len(types) > 0 {
							ty = types[0]
						}
						t.Logf("%-5s %-7s %-4s %-8s %-8s %6.2fs loki=%5.2fs MB=%5.1f scanned=%-8d %s",
							target.name, ty, window, term.shape, c.name, took.Seconds(), tr.exec.Seconds(),
							float64(tr.bytes)/1e6, tr.scanned, outcome)
					}
				}
			}
		}
	}
}
