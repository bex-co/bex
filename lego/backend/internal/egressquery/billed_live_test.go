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
// Increase on every whole hour a live Prometheus still holds, read-only, for
// every source with NewSeriesFromZero (w6/m455 t003 for Traefik, w6/133 for
// the WebSocket plugin and the datastore proxies: the comparison the user
// asked for before billing switches). It reads each source's whole fleet as
// one source, so every series is compared. It fails unless, in every hour,
// Billed minus Increase is exactly the new series' last values minus their
// own increase(): the two differ only on the series newSeries selects.
//
// It also reports two things for the record. A new series that was present
// earlier in retention is checked against the value it left with, and against
// the request count it left with where the source has one: a series that
// carried its old counter on could only come back at or above both, so a
// lower value or a lower request count shows it restarted from zero, and so
// do more bytes for the same requests where every byte belongs to a finished
// request. And it sums what the guard kept out: series missing at the hour's
// start whose target was not scraped then, which a rule without the guard
// would have counted from zero.
//
// Run it with a port-forward to the production Prometheus:
//
//	BEX_TEST_EGRESS_PROM_URL=http://127.0.0.1:19090 go test ./internal/egressquery -run OnRecordedHours -v -timeout 60m
func TestBilledAgainstIncreaseOnRecordedHours(t *testing.T) {
	base := os.Getenv("BEX_TEST_EGRESS_PROM_URL")
	if base == "" {
		t.Skip("set BEX_TEST_EGRESS_PROM_URL to replay the egress billing rule on a live Prometheus's recorded hours")
	}
	// fleet reads a source's every series: id is the label that names its
	// resource; requests, when set, is a counter with the same labels and
	// lifetime that counts the resource's requests; requestBound says every
	// byte is counted with a finished request.
	fleet := map[Source]struct {
		matchers, id, requests string
		requestBound           bool
	}{
		HTTP:      {`router=~".+"`, "router", "traefik_router_requests_total", true},
		WebSocket: {`app_id=~".+"`, "app_id", "bex_http_request_starts_total", false},
		Postgres:  {`resource_id=~".+",resource_kind="postgres"`, "resource_id", "", false},
		KeyValue:  {`resource_id=~".+",resource_kind="key_value"`, "resource_id", "", false},
	}
	specs := append(App("fleet", []string{"fleet"}, true), Datastore("fleet", "postgres", true)...)
	specs = append(specs, Datastore("fleet", "key_value", true)...)
	for _, spec := range specs {
		if !spec.NewSeriesFromZero {
			continue
		}
		f, ok := fleet[spec.Source]
		if !ok {
			t.Fatalf("%s counts new series from zero but has no fleet-wide replay", spec.Source)
		}
		spec.Matchers = f.matchers
		t.Run(string(spec.Source), func(t *testing.T) {
			replayRecordedHours(t, base, spec, f.id, f.requests, f.requestBound)
		})
	}
}

func replayRecordedHours(t *testing.T, base string, spec Spec, id, requests string, requestBound bool) {
	series := fmt.Sprintf("%s{%s}", spec.Counter, spec.Matchers)
	now := time.Now().UTC()
	lowest := mustInstant(t, base, "min(prometheus_tsdb_lowest_timestamp_seconds)", now)
	retention := time.Unix(int64(lowest), 0).UTC()
	from := retention.Truncate(time.Hour).Add(2 * time.Hour) // the first hour whose guard is inside retention
	var hours, healthy, freshCount, returned, restarted, guarded int
	var oldTotal, newTotal, oldHealthy, newHealthy, firstTotal, untold, guardedBytes float64
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
		hours++
		ok := mustInstant(t, base, Health(spec, 3600), end) == 1
		if ok {
			healthy++
			oldHealthy += oldBytes
			newHealthy += newBytes
		}
		kept := liveVector(t, base, fmt.Sprintf("(last_over_time(%[1]s[3600s]) unless last_over_time(%[1]s[%[2]ds] offset 3600s)) unless %[3]s", series, newSeriesGuard, newSeries(spec, 3600)), end)
		var keptBytes float64
		for _, s := range kept {
			keptBytes += s.value
		}
		if len(kept) > 0 {
			t.Logf("%s: the guard kept %d series (%.0f bytes) on increase(); the health gate passes the hour: %t", at, len(kept), keptBytes, ok)
		}
		guarded += len(kept)
		guardedBytes += keptBytes
		freshCount += len(fresh)
		oldTotal += oldBytes
		newTotal += newBytes
		if len(fresh) == 0 {
			continue
		}
		// first: each new series' earliest value in the hour (a new series
		// with no reset in its first hour starts at its minimum); prior: its
		// last value anywhere earlier in retention.
		ids := map[string]bool{}
		for _, s := range fresh {
			ids[regexp.QuoteMeta(s.metric[id])] = true
		}
		names := make([]string, 0, len(ids))
		for r := range ids {
			names = append(names, r)
		}
		sort.Strings(names)
		matcher := fmt.Sprintf("{%s=~%q}", id, "^("+strings.Join(names, "|")+")$")
		before := end.Add(-time.Hour - newSeriesGuard*time.Second)
		span := int64(before.Sub(retention) / time.Second)
		read := func(metric, fn string, window int64, when time.Time) map[string]float64 {
			out := map[string]float64{}
			if window > 0 && metric != "" {
				for _, s := range liveVector(t, base, fmt.Sprintf("%s(%s%s[%ds])", fn, metric, matcher, window), when) {
					out[s.key()] = s.value
				}
			}
			return out
		}
		first, prior := read(spec.Counter, "min_over_time", 3600, end), read(spec.Counter, "last_over_time", span, before)
		firstReqs, priorReqs := read(requests, "min_over_time", 3600, end), read(requests, "last_over_time", span, before)
		for _, s := range fresh {
			k := s.key()
			firstTotal += first[k]
			p, seen := prior[k]
			if !seen {
				continue
			}
			returned++
			b, r, pr := first[k], firstReqs[k], priorReqs[k]
			if b < p || r < pr || (requestBound && b > p && r == pr) {
				restarted++
				continue
			}
			untold += p
			t.Logf("%s: %s came back at %.0f bytes / %.0f requests after leaving at %.0f / %.0f; the values cannot tell", at, k, b, r, p, pr)
		}
		t.Logf("%s old %.0f new %.0f (+%.0f): %d new series", at, oldBytes, newBytes, newBytes-oldBytes, len(fresh))
	}
	t.Logf("%s: %d hours ending %s..%s: old %.0f new %.0f bytes (+%.0f, +%.4f%%); %d new series, first scraped at %.0f bytes in total",
		spec.Source, hours, from.Format(time.RFC3339), now.Truncate(time.Hour).Format(time.RFC3339), oldTotal, newTotal, newTotal-oldTotal,
		100*(newTotal-oldTotal)/math.Max(oldTotal, 1), freshCount, firstTotal)
	t.Logf("%s: the %d hours the health gate passes (the hours the meter bills): old %.0f new %.0f bytes (+%.0f)",
		spec.Source, healthy, oldHealthy, newHealthy, newHealthy-oldHealthy)
	t.Logf("%s: %d of the new series were present earlier in retention: %d show a restart from zero, the values of the rest cannot tell (%.0f bytes at most)",
		spec.Source, returned, restarted, untold)
	t.Logf("%s: the guard kept %d series missing at an hour's start with no scrape of their target on increase(): %.0f bytes a rule without it would have counted from zero",
		spec.Source, guarded, guardedBytes)
}

// TestTargetHealthOnRecordedHours replays the SNI sources' per-pod health
// (targetHealth, w6/155) on every whole hour a live Prometheus still holds,
// read-only, each source's whole fleet read as one store. It logs every hour
// the rule fails, with each failing pod and the terms it meets, and the hours
// it passes. It fails if a pod the rule calls new had a process sample in
// the hour before: that pod was a restart, whose earlier process may have
// counted bytes no scrape read.
//
// Run it with a port-forward to the production Prometheus:
//
//	BEX_TEST_EGRESS_PROM_URL=http://127.0.0.1:19090 go test ./internal/egressquery -run TargetHealthOnRecordedHours -v -timeout 60m
func TestTargetHealthOnRecordedHours(t *testing.T) {
	base := os.Getenv("BEX_TEST_EGRESS_PROM_URL")
	if base == "" {
		t.Skip("set BEX_TEST_EGRESS_PROM_URL to replay the SNI sources' per-pod health on a live Prometheus's recorded hours")
	}
	specs := append(Datastore("fleet", "postgres", true), Datastore("fleet", "key_value", true)...)
	for _, spec := range specs {
		spec.Matchers = fmt.Sprintf(`resource_id=~".+",resource_kind=%q`, map[Source]string{Postgres: "postgres", KeyValue: "key_value"}[spec.Source])
		t.Run(string(spec.Source), func(t *testing.T) {
			terms := newTargetTerms(spec, 3600, 192)
			named := []struct{ name, expr string }{
				{"healthy", terms.healthy}, {"old", terms.old}, {"born", terms.born}, {"full", terms.full},
				{"covered", terms.covered}, {"alive", terms.alive}, {"drained", terms.drained},
			}
			now := time.Now().UTC()
			lowest := mustInstant(t, base, "min(prometheus_tsdb_lowest_timestamp_seconds)", now)
			from := time.Unix(int64(lowest), 0).UTC().Truncate(time.Hour).Add(2 * time.Hour)
			var hours, healthy int
			for end := from; !end.After(now.Truncate(time.Hour)); end = end.Add(time.Hour) {
				hours++
				at := end.Format(time.RFC3339)
				for _, s := range liveVector(t, base, fmt.Sprintf("%s and on(pod) count_over_time(%s[3600s] offset 3600s)", terms.born, spec.ProcessMetric), end) {
					t.Errorf("%s: %s is new to the rule, but its pod had a process in the hour before", at, s.metric["pod"])
				}
				if mustInstant(t, base, Health(spec, 3600), end) == 1 {
					healthy++
					continue
				}
				met := map[string][]string{}
				for _, term := range named {
					for _, s := range liveVector(t, base, term.expr, end) {
						met[s.metric["pod"]] = append(met[s.metric["pod"]], term.name)
					}
				}
				for _, s := range liveVector(t, base, fmt.Sprintf("%s unless on(instance, pod) %s", terms.targets, terms.good()), end) {
					t.Logf("%s: %s fails, meeting only %v", at, s.metric["pod"], met[s.metric["pod"]])
				}
				if resets := mustInstant(t, base, fmt.Sprintf("sum(resets(%s{%s}[3600s])) or vector(0)", spec.Counter, spec.Matchers), end); resets > 0 {
					t.Logf("%s: the counter reset %.0f times", at, resets)
				}
			}
			t.Logf("%s: %d hours ending %s..%s, %d pass the per-pod health", spec.Source, hours, from.Format(time.RFC3339), now.Truncate(time.Hour).Format(time.RFC3339), healthy)
		})
	}
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
