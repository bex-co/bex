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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/testenv"
)

// TestLokiRequestMetricsClipToWindowOnLoki is w4/m175 t006 executed by the
// pinned Loki (3.6.7, BEX_TEST_LOKI_URL): the step-rounded evaluation keeps the
// bucket containing end, while only entries whose own timestamp lies in
// [start, end] contribute to counts and latency quantiles. It pushes request
// lines one nanosecond either side of each bound into the first and last
// buckets, which the earlier whole-bucket query counted.
func TestLokiRequestMetricsClipToWindowOnLoki(t *testing.T) {
	base := os.Getenv("BEX_TEST_LOKI_URL")
	if base == "" {
		testenv.Skip(t, "BEX_TEST_LOKI_URL not set")
	}
	base = strings.TrimRight(base, "/")
	ns := fmt.Sprintf("m175-%d", time.Now().UnixNano())

	// A recent minute boundary (inside the results cache's freshness horizon,
	// so the warm-up poll is never answered from a stale cached read), then a
	// window that starts and ends mid-bucket with nanosecond precision.
	bucket := time.Now().Add(-5 * time.Minute).Truncate(time.Minute)
	start := bucket.Add(10*time.Second + 123456789)
	end := bucket.Add(70*time.Second + 987654321)
	mid := bucket.Add(40 * time.Second)
	beforeStart := [2]time.Time{bucket.Add(time.Second), start.Add(-2)} // same bucket as start-1ns
	afterEnd := [2]time.Time{end.Add(2), end.Add(20 * time.Second)}     // same bucket as end+1ns
	const coarseStep = 720 * time.Second

	type line struct {
		at             time.Time
		status, method string
		path           string
		durationNs     int64
	}
	slow := int64(900 * time.Millisecond)
	lines := []line{
		{start.Add(-1), "200", "GET", "/a", slow},
		{start, "200", "GET", "/a", 4_000_000},
		{start.Add(1), "200", "HEAD", "/a", 4_100_000},
		{mid, "404", "GET", "/a", 4_200_000},
		{end.Add(-1), "200", "GET", "/a", 4_300_000},
		{end, "200", "GET", "/a", 4_400_000},
		{end.Add(1), "200", "GET", "/a", slow},
		{mid, "200", "GET", "/other", slow}, // filtered out by path
	}
	type stream struct {
		Stream map[string]string `json:"stream"`
		Values [][2]string       `json:"values"`
	}
	var streams []stream
	for _, l := range lines {
		msg, _ := json.Marshal(map[string]any{
			"RequestHost": "web.example.com", "RequestPath": l.path, "RequestMethod": l.method,
			"DownstreamStatus": l.status, "Duration": l.durationNs,
		})
		streams = append(streams, stream{
			Stream: map[string]string{"namespace": ns, "app": "web", "type": "request", "status": l.status, "method": l.method},
			Values: [][2]string{{strconv.FormatInt(l.at.UnixNano(), 10), string(msg)}},
		})
	}
	body, _ := json.Marshal(map[string]any{"streams": streams})
	hc := &http.Client{Timeout: 10 * time.Second}
	resp, err := hc.Post(base+"/loki/api/v1/push", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("push: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		t.Fatalf("push: HTTP %d", resp.StatusCode)
	}

	source := NewLokiRequestMetricsSource(base, hc)
	query := func(req RequestMetricsRequest) ([]MetricSeries, error) {
		req.Namespace, req.App, req.Path = ns, "web", "/a"
		if req.Resolution == 0 {
			req.Resolution = time.Minute
		}
		return source(context.Background(), req)
	}
	read := func(t *testing.T, req RequestMetricsRequest) []MetricSeries {
		t.Helper()
		series, err := query(req)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		for _, s := range series {
			for k := range s.Labels {
				if k == lokiTSSec || k == lokiTSNano {
					t.Errorf("temporary label %q leaked into a series: %v", k, s.Labels)
				}
			}
		}
		return series
	}
	total := func(series []MetricSeries) float64 {
		var n float64
		for _, s := range series {
			for _, p := range s.Points {
				n += p.Value
			}
		}
		return n
	}
	counts := func(t *testing.T, from, to time.Time, step time.Duration) float64 {
		t.Helper()
		return total(read(t, RequestMetricsRequest{Metric: MetricHTTPRequests, Start: from, End: to, Resolution: step}))
	}

	// Pushed lines are queryable once the ingester has them; wait for an
	// aligned window to see all seven /a lines.
	aligned := RequestMetricsRequest{Metric: MetricHTTPRequests, Start: bucket, End: bucket.Add(2 * time.Minute)}
	for deadline := time.Now().Add(30 * time.Second); ; time.Sleep(500 * time.Millisecond) {
		series, err := query(aligned)
		if err == nil && total(series) == 7 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("Loki never returned the 7 pushed lines: %v, %v", series, err)
		}
	}

	cases := []struct {
		name     string
		from, to time.Time
		step     time.Duration
		want     float64
	}{
		{"exact window counts both endpoints once", start, end, time.Minute, 5},
		{"exact window at a coarse 720s step", start, end, coarseStep, 5},
		{"exact window at a 15s step", start, end, 15 * time.Second, 5},
		{"same-bucket window around start", start.Add(-1), start.Add(1), time.Minute, 3},
		{"window of exactly the start instant", start, start, time.Minute, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := counts(t, tc.from, tc.to, tc.step); got != tc.want {
				t.Errorf("count = %v, want %v", got, tc.want)
			}
		})
	}

	t.Run("last point still covers end", func(t *testing.T) {
		for _, step := range []time.Duration{time.Minute, coarseStep} {
			var last time.Time
			for _, s := range read(t, RequestMetricsRequest{Metric: MetricHTTPRequests, Start: start, End: end, Resolution: step}) {
				for _, p := range s.Points {
					if at, err := time.Parse(time.RFC3339Nano, p.Timestamp); err == nil && at.After(last) {
						last = at
					}
				}
			}
			if last.Before(end) {
				t.Errorf("step %v: last point %v ends before end %v", step, last, end)
			}
		}
	})

	t.Run("excluded windows return no series", func(t *testing.T) {
		for _, w := range [][2]time.Time{beforeStart, afterEnd} {
			for _, m := range []string{MetricHTTPRequests, MetricHTTPLatency} {
				if s := read(t, RequestMetricsRequest{Metric: m, Quantile: 0.9, Start: w[0], End: w[1]}); len(s) != 0 {
					t.Errorf("%s %v–%v: %d series, want none: %+v", m, w[0], w[1], len(s), s)
				}
			}
		}
	})

	t.Run("group by status and method", func(t *testing.T) {
		byLabel := func(group, label string) map[string]float64 {
			got := map[string]float64{}
			for _, s := range read(t, RequestMetricsRequest{Metric: MetricHTTPRequests, GroupBy: group, Start: start, End: end}) {
				for _, p := range s.Points {
					got[s.Labels[label]] += p.Value
				}
				if len(s.Labels) != 1 {
					t.Errorf("group by %s: labels %v, want only %s", group, s.Labels, label)
				}
			}
			return got
		}
		if got := byLabel("status", "code"); got["200"] != 4 || got["404"] != 1 || len(got) != 2 {
			t.Errorf("by status = %v, want 200:4 404:1", got)
		}
		if got := byLabel("method", "method"); got["GET"] != 4 || got["HEAD"] != 1 || len(got) != 2 {
			t.Errorf("by method = %v, want GET:4 HEAD:1", got)
		}
	})

	t.Run("latency quantile ignores out-of-window outliers", func(t *testing.T) {
		series := read(t, RequestMetricsRequest{Metric: MetricHTTPLatency, Quantile: 0.9, Host: "web.example.com", Start: start, End: end})
		if len(series) == 0 {
			t.Fatal("no latency series for the populated window")
		}
		for _, s := range series {
			for _, p := range s.Points {
				// Every in-window duration is 4–4.4 ms; the 900 ms outliers sit
				// one nanosecond outside the window, in the same buckets.
				if p.Value <= 0 || p.Value > 0.005 {
					t.Errorf("p90 point %s = %v s, want in-window latency (<= 5ms)", p.Timestamp, p.Value)
				}
			}
			if len(s.Labels) != 0 {
				t.Errorf("ungrouped latency labels = %v, want none", s.Labels)
			}
		}
	})
}
