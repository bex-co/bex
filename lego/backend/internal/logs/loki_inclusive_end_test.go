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
	"net/http/httptest"
	"sort"
	"strconv"
	"testing"
	"time"
)

// exclusiveEndLoki emulates Loki's query_range bounds: start inclusive, end
// exclusive, `limit` kept from the newest (backward) or oldest (forward) end.
func exclusiveEndLoki(t *testing.T, stamps []time.Time) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		start, _ := strconv.ParseInt(q.Get("start"), 10, 64)
		end, _ := strconv.ParseInt(q.Get("end"), 10, 64)
		limit, _ := strconv.Atoi(q.Get("limit"))
		var hits []time.Time
		for _, s := range stamps {
			if ns := s.UnixNano(); ns >= start && ns < end {
				hits = append(hits, s)
			}
		}
		sort.Slice(hits, func(i, j int) bool { return hits[i].Before(hits[j]) })
		if len(hits) > limit {
			if q.Get("direction") == DirectionForward {
				hits = hits[:limit]
			} else {
				hits = hits[len(hits)-limit:]
			}
		}
		values := make([][2]string, 0, len(hits))
		for _, h := range hits {
			values = append(values, [2]string{strconv.FormatInt(h.UnixNano(), 10), "line-" + h.Format(time.RFC3339Nano)})
		}
		encoded, _ := json.Marshal(values)
		_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"streams","result":[{"stream":{"app":"web","pod":"web-1","container":"app"},"values":%s}]}}`, encoded)
	}))
}

func TestLokiEndIsInclusiveAndPagesStayGapFree(t *testing.T) {
	t0 := time.Date(2026, 10, 3, 11, 34, 28, 0, time.UTC) // whole second: trimmed-zero stamps must still order (w8/044)
	stamps := make([]time.Time, 150)
	for i := range stamps {
		stamps[i] = t0.Add(time.Duration(i)*7*time.Millisecond + time.Duration(i*13)) // sub-ms precision
	}
	srv := exclusiveEndLoki(t, stamps)
	defer srv.Close()
	src := NewLokiSource(srv.URL, srv.Client())
	read := func(q LogQuery) []LogEntry {
		t.Helper()
		q.App = "web"
		entries, err := src(context.Background(), "default", q)
		if err != nil {
			t.Fatal(err)
		}
		return entries
	}
	stampOf := func(e LogEntry) time.Time {
		t.Helper()
		ts, err := time.Parse(time.RFC3339Nano, e.Timestamp)
		if err != nil {
			t.Fatal(err)
		}
		return ts
	}
	since, last := stamps[0], stamps[len(stamps)-1].Add(time.Second)
	t75 := stamps[74]

	// The w8/043 repro: --end <t75> includes line #75 as the last line.
	got := read(LogQuery{Since: since, End: t75, Limit: 100, Direction: DirectionForward})
	if len(got) != 75 || !stampOf(got[len(got)-1]).Equal(t75) {
		t.Fatalf("end=t75 returned %d lines, last %v; want 75 ending at %v", len(got), got[len(got)-1].Timestamp, t75)
	}
	// A single-instant window [t75, t75] is exactly line #75.
	if got := read(LogQuery{Since: t75, End: t75, Limit: 100}); len(got) != 1 || !stampOf(got[0]).Equal(t75) {
		t.Fatalf("start=end=t75 returned %+v; want exactly line #75", got)
	}

	for _, direction := range []string{DirectionBackward, DirectionForward} {
		t.Run(direction+" paging", func(t *testing.T) {
			seen := map[string]bool{}
			pages := 0
			from, to := since, last
			for range 10 {
				page := read(LogQuery{Since: from, End: to, Limit: 50, Direction: direction})
				if len(page) == 0 {
					break
				}
				pages++
				for _, e := range page {
					if seen[e.Timestamp] {
						t.Fatalf("page %d repeats %s", pages, e.Timestamp)
					}
					seen[e.Timestamp] = true
				}
				hasMore, nextStart, nextEnd := pageCursors(page, 50, from, to, direction)
				if !hasMore {
					break
				}
				from, _ = time.Parse(time.RFC3339Nano, nextStart)
				to, _ = time.Parse(time.RFC3339Nano, nextEnd)
			}
			if len(seen) != 150 || pages != 3 {
				t.Fatalf("paged %d unique lines in %d pages; want 150 in 3", len(seen), pages)
			}
		})
	}
}

func TestLokiEndParamNudgesOnlyTheCallersEnd(t *testing.T) {
	end := time.Date(2026, 10, 3, 12, 0, 0, 0, time.UTC)
	ns := func(t time.Time) string { return strconv.FormatInt(t.UnixNano(), 10) }
	if got := lokiEndParam(LogQuery{End: end}, end); got != ns(end.Add(time.Nanosecond)) {
		t.Errorf("caller end = %s, want end+1ns", got)
	}
	if got := lokiEndParam(LogQuery{End: end}, end.Add(-time.Hour)); got != ns(end.Add(-time.Hour)) {
		t.Errorf("interior slice boundary = %s, want unchanged", got)
	}
	if got := lokiEndParam(LogQuery{}, end); got != ns(end) {
		t.Errorf("defaulted end=now = %s, want unchanged", got)
	}
}
