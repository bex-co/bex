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

package egressquery

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"
)

// TestBilledAgainstIncreaseOnRecordedHours replays the billing rule against
// Increase on every whole hour a live Prometheus still holds, read-only
// (w6/m455 t003, the comparison the user asked for before billing switched).
// It reads the whole Traefik fleet as one source, so every router series is
// compared. It fails unless, in every hour, Billed minus Increase is exactly
// the new series' last values minus their own increase(): the two differ only
// on the series newSeries selects.
//
// It also reports two things for the record. A new series that was present
// earlier in retention is checked against the value and request count it
// left with: a series that carried its old counter on could only come back
// at or above both, so a lower value, a lower request count, or more bytes
// for the same requests shows it restarted from zero. And it sums what the
// guard kept out: series missing at the hour's start whose target was not
// scraped then, which a rule without the guard would have counted from zero.
//
// Run it with a port-forward to the production Prometheus:
//
//	BEX_TEST_EGRESS_PROM_URL=http://127.0.0.1:19090 go test ./internal/egressquery -run OnRecordedHours -v -timeout 60m
func TestBilledAgainstIncreaseOnRecordedHours(t *testing.T) {
	base := os.Getenv("BEX_TEST_EGRESS_PROM_URL")
	if base == "" {
		t.Skip("set BEX_TEST_EGRESS_PROM_URL to replay the egress billing rule on a live Prometheus's recorded hours")
	}
	spec := App("fleet", []string{"fleet"}, false)[0]
	spec.Matchers = `router=~".+"`
	series := fmt.Sprintf("%s{%s}", spec.Counter, spec.Matchers)
	now := time.Now().UTC()
	lowest := mustInstant(t, base, "min(prometheus_tsdb_lowest_timestamp_seconds)", now)
	retention := time.Unix(int64(lowest), 0).UTC()
	from := retention.Truncate(time.Hour).Add(2 * time.Hour) // the first hour whose guard is inside retention
	var hours, freshCount, returned, restarted, guarded int
	var oldTotal, newTotal, firstTotal, untold, guardedBytes float64
	for end := from; !end.After(now.Truncate(time.Hour)); end = end.Add(time.Hour) {
		at := end.Format(time.RFC3339)
		oldBytes := mustInstant(t, base, Increase(spec, 3600), end)
		newBytes := mustInstant(t, base, Billed(spec, 3600), end)
		fresh := liveVector(t, base, newSeries(spec, 3600), end)
		freshIncrease := mustInstant(t, base, fmt.Sprintf("sum(increase(%s[3600s]) and %s) or vector(0)", series, newSeries(spec, 3600)), end)
		var freshLast float64
		for _, s := range fresh {
			freshLast += s.value
		}
		if got, want := newBytes-oldBytes, freshLast-freshIncrease; math.Abs(got-want) > 1e-6*math.Max(1, math.Abs(newBytes)) {
			t.Errorf("%s: Billed-Increase = %.0f, but the new series account for %.0f", at, got, want)
		}
		for _, s := range liveVector(t, base, fmt.Sprintf("(last_over_time(%[1]s[3600s]) unless last_over_time(%[1]s[%[2]ds] offset 3600s)) unless %[3]s", series, newSeriesGuard, newSeries(spec, 3600)), end) {
			guarded++
			guardedBytes += s.value
		}
		hours++
		freshCount += len(fresh)
		oldTotal += oldBytes
		newTotal += newBytes
		if len(fresh) == 0 {
			continue
		}
		// first: each new series' earliest value in the hour (a new series
		// with no reset in its first hour starts at its minimum); prior: its
		// last value anywhere earlier in retention.
		routers := map[string]bool{}
		for _, s := range fresh {
			routers[regexp.QuoteMeta(s.metric["router"])] = true
		}
		names := make([]string, 0, len(routers))
		for r := range routers {
			names = append(names, r)
		}
		sort.Strings(names)
		matcher := fmt.Sprintf("{router=~%q}", "^("+strings.Join(names, "|")+")$")
		before := end.Add(-time.Hour - newSeriesGuard*time.Second)
		span := int64(before.Sub(retention) / time.Second)
		read := func(metric, fn string, window int64, when time.Time) map[string]float64 {
			out := map[string]float64{}
			if window > 0 {
				for _, s := range liveVector(t, base, fmt.Sprintf("%s(%s%s[%ds])", fn, metric, matcher, window), when) {
					out[s.key()] = s.value
				}
			}
			return out
		}
		first, prior := read(spec.Counter, "min_over_time", 3600, end), read(spec.Counter, "last_over_time", span, before)
		firstReqs, priorReqs := read("traefik_router_requests_total", "min_over_time", 3600, end), read("traefik_router_requests_total", "last_over_time", span, before)
		for _, s := range fresh {
			k := s.key()
			firstTotal += first[k]
			p, seen := prior[k]
			if !seen {
				continue
			}
			returned++
			b, r, pr := first[k], firstReqs[k], priorReqs[k]
			if b < p || r < pr || (b > p && r == pr) {
				restarted++
				continue
			}
			untold += p
			t.Logf("%s: %s came back at %.0f bytes / %.0f requests after leaving at %.0f / %.0f; the values cannot tell", at, k, b, r, p, pr)
		}
		t.Logf("%s old %.0f new %.0f (+%.0f): %d new series", at, oldBytes, newBytes, newBytes-oldBytes, len(fresh))
	}
	t.Logf("%d hours ending %s..%s: old %.0f new %.0f bytes (+%.0f, +%.4f%%); %d new series, first scraped at %.0f bytes in total",
		hours, from.Format(time.RFC3339), now.Truncate(time.Hour).Format(time.RFC3339), oldTotal, newTotal, newTotal-oldTotal,
		100*(newTotal-oldTotal)/math.Max(oldTotal, 1), freshCount, firstTotal)
	t.Logf("%d of the new series were present earlier in retention: %d show a restart from zero, the values of the rest cannot tell (%.0f bytes at most)",
		returned, restarted, untold)
	t.Logf("the guard kept %d series missing at an hour's start with no scrape of their target on increase(): %.0f bytes a rule without it would have counted from zero",
		guarded, guardedBytes)
}

type liveSample struct {
	metric map[string]string
	value  float64
}

// key identifies a series by its labels without the metric name, the way
// PromQL matches binary operands.
func (s liveSample) key() string {
	names := make([]string, 0, len(s.metric))
	for name := range s.metric {
		if name != "__name__" {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	parts := make([]string, len(names))
	for i, name := range names {
		parts[i] = fmt.Sprintf("%s=%q", name, s.metric[name])
	}
	return "{" + strings.Join(parts, ",") + "}"
}

func mustInstant(t *testing.T, base, query string, at time.Time) float64 {
	t.Helper()
	v, err := Instant(context.Background(), nil, base, query, at)
	if err != nil {
		t.Fatalf("%s at %s: %v", query, at.Format(time.RFC3339), err)
	}
	return v
}

func liveVector(t *testing.T, base, query string, at time.Time) []liveSample {
	t.Helper()
	u := fmt.Sprintf("%s/api/v1/query?%s", strings.TrimRight(base, "/"), url.Values{
		"query": {query}, "time": {strconv.FormatInt(at.Unix(), 10)},
	}.Encode())
	req, err := http.NewRequestWithContext(context.Background(), http.MethodGet, u, nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	var body struct {
		Status string `json:"status"`
		Data   struct {
			Result []struct {
				Metric map[string]string `json:"metric"`
				Value  [2]any            `json:"value"`
			} `json:"result"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil || body.Status != "success" {
		t.Fatalf("%s: status %q, %v", query, body.Status, err)
	}
	out := make([]liveSample, 0, len(body.Data.Result))
	for _, r := range body.Data.Result {
		raw, _ := r.Value[1].(string)
		v, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			t.Fatalf("%s: value %q: %v", query, raw, err)
		}
		out = append(out, liveSample{metric: r.Metric, value: v})
	}
	return out
}
