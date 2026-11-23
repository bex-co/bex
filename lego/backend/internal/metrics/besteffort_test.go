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
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

// The w1/m50 split (ADR023 § Observability reads vs billing reads): the
// interactive bandwidth reads are best-effort — a source failing its health
// product (the prod failure modes: meter up-gap, in-window router-counter
// reset) yields data plus a degraded marker, never the old
// "egress source X unhealthy" error. The billing rollup's strict refusal is
// pinned separately by usage's TestQueryEgressBytesRejectsPartialSourceHealth.

// bandwidthFakeProm serves health instants per source and a fixed matrix for
// the range query. A health query reads 0 when it contains any of unhealthy:
// an `up{job="…"}` selector, or the direct meter's health metric (its query has
// no up term, w4/442).
func bandwidthFakeProm(t *testing.T, unhealthy ...string) (*httptest.Server, *[]string) {
	t.Helper()
	var queries []string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		queries = append(queries, query)
		if strings.HasSuffix(r.URL.Path, "/query_range") {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1000000,"42"]]}]}}`))
			return
		}
		value := "1"
		for _, marker := range unhealthy {
			// `up{job="traefik"}` must not match `up{job="traefik-websocket-meter"}`.
			if strings.Contains(query, marker) {
				value = "0"
			}
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"` + value + `"]}]}}`))
	}))
	return ts, &queries
}

func bandwidthReq() RequestMetricsRequest {
	return RequestMetricsRequest{
		Metric: MetricBandwidth, AppID: "srv-web", Direct: true,
		Routers: []string{"default-web-web-onbex-co@kubernetes"},
		Start:   time.Unix(1_000_000, 0), End: time.Unix(1_043_200, 0), Resolution: 5 * time.Minute,
	}
}

func TestBandwidthServesSeriesWithDegradedSourceLabel(t *testing.T) {
	ts, queries := bandwidthFakeProm(t, "bex_egress_meter_healthy")
	defer ts.Close()

	series, err := NewPrometheusRequestSource(ts.URL, ts.Client())(context.Background(), bandwidthReq())
	if err != nil {
		t.Fatalf("degraded source must not error the read: %v", err)
	}
	if len(series) != 1 || len(series[0].Points) != 1 || series[0].Points[0].Value != 42 {
		t.Fatalf("series must be served despite the degraded source: %+v", series)
	}
	if got := series[0].Labels[LabelDegradedSources]; got != "direct" {
		t.Fatalf("degraded_sources label: got %q want %q", got, "direct")
	}
	if joined := strings.Join(*queries, "\n"); !strings.Contains(joined, "traefik_router_responses_bytes_total") {
		t.Fatalf("the range query must still run: %q", joined)
	}
}

func TestBandwidthHealthyWindowCarriesNoDegradedLabel(t *testing.T) {
	ts, _ := bandwidthFakeProm(t)
	defer ts.Close()

	series, err := NewPrometheusRequestSource(ts.URL, ts.Client())(context.Background(), bandwidthReq())
	if err != nil || len(series) != 1 {
		t.Fatalf("healthy roundtrip: %v %+v", err, series)
	}
	if _, present := series[0].Labels[LabelDegradedSources]; present {
		t.Fatalf("healthy window must not carry the degraded label: %+v", series[0].Labels)
	}
}

func TestMonthToDateBandwidthSourceBestEffortOnUnhealthySource(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("query")
		value := "1"
		switch {
		case strings.Contains(query, "clamp_min(traefik_router_responses_bytes_total"):
			value = "1048576"
		case strings.Contains(query, "clamp_min(bex_websocket_egress_bytes_total"):
			value = "2097152"
		case strings.Contains(query, "clamp_min(bex_app_direct_egress_bytes_total"):
			value = "3145728"
		case strings.Contains(query, `up{job="traefik"}`): // the http health product
			value = "0"
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"` + value + `"]}]}}`))
	}))
	defer ts.Close()

	// Inside one UTC hour: a single exact delta per source, whatever the clock says.
	now := time.Date(2026, 11, 4, 10, 40, 0, 0, time.UTC)
	value, degraded, err := NewMonthToDateBandwidthSource(ts.URL, ts.Client())(
		context.Background(), "srv-web", []string{"default-web-web-onbex-co@kubernetes"}, true, now.Add(-30*time.Minute), now)
	if err != nil {
		t.Fatalf("degraded source must not error month-to-date: %v", err)
	}
	// Every category still contributes what its counters recorded.
	if value.HTTP != 1048576 || value.WebSocket != 2097152 || value.NAT != 3145728 {
		t.Fatalf("month values must be served despite the degraded source: %+v", value)
	}
	if len(degraded) != 1 || degraded[0] != "http" {
		t.Fatalf("degraded sources: got %v want [http]", degraded)
	}
}

// futurePromFake answers like Prometheus does for a window running past the
// wall clock: a health instant after now reads 0 (the minutes that have not
// happened hold no `up` samples), and the range answers every step up to its
// end, 100 bytes in the past and the `or vector(0)` fallback's 0 after now.
// It records each instant's time and the range's end.
func futurePromFake(t *testing.T) (*httptest.Server, *[]int64, *int64) {
	t.Helper()
	var instants []int64
	var rangeEnd int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		now := time.Now().Unix()
		if strings.HasSuffix(r.URL.Path, "/query_range") {
			start, _ := strconv.ParseInt(q.Get("start"), 10, 64)
			rangeEnd, _ = strconv.ParseInt(q.Get("end"), 10, 64)
			step, _ := strconv.ParseInt(q.Get("step"), 10, 64)
			var values []string
			for at := start; at <= rangeEnd; at += step {
				v := "100"
				if at > now {
					v = "0"
				}
				values = append(values, fmt.Sprintf(`[%d,%q]`, at, v))
			}
			_, _ = fmt.Fprintf(w, `{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[%s]}]}}`, strings.Join(values, ","))
			return
		}
		at, _ := strconv.ParseInt(q.Get("time"), 10, 64)
		instants = append(instants, at)
		value := "1"
		if at > now {
			value = "0"
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"` + value + `"]}]}}`))
	}))
	return ts, &instants, &rangeEnd
}

// w8/413: a bandwidth read whose end is after now reads only up to now. It
// answers no zero-valued point for a minute that has not happened, and checks
// source health over the window that has, so healthy sources carry no
// degraded label. A window wholly in the future reads nothing.
func TestBandwidthWindowEndingAfterNowReadsUpToNow(t *testing.T) {
	ts, instants, rangeEnd := futurePromFake(t)
	defer ts.Close()
	src := NewPrometheusRequestSource(ts.URL, ts.Client())
	req := bandwidthReq()
	before := time.Now()
	req.Start, req.End, req.Resolution = before.Add(-5*time.Minute), before.Add(15*time.Minute), time.Minute

	series, err := src(context.Background(), req)
	if err != nil || len(series) != 1 {
		t.Fatalf("read: %v %+v", err, series)
	}
	after := time.Now()
	if *rangeEnd < before.Unix() || *rangeEnd > after.Unix() {
		t.Errorf("query_range end = %d, want now (%d..%d)", *rangeEnd, before.Unix(), after.Unix())
	}
	if len(*instants) != 3 {
		t.Fatalf("health instants = %v, want one per source", *instants)
	}
	for _, at := range *instants {
		if at > after.Unix() {
			t.Errorf("health evaluated at %d, after now (%d)", at, after.Unix())
		}
	}
	if got, ok := series[0].Labels[LabelDegradedSources]; ok {
		t.Errorf("healthy sources labeled degraded: %q", got)
	}
	var sum float64
	for _, p := range series[0].Points {
		at, _ := time.Parse(time.RFC3339, p.Timestamp)
		if at.After(after) {
			t.Errorf("point %s is after now", p.Timestamp)
		}
		sum += p.Value
	}
	if want := float64(100 * len(series[0].Points)); sum != want || len(series[0].Points) < 5 {
		t.Errorf("points = %+v, want five or six 100-byte minutes", series[0].Points)
	}

	*instants = nil
	req.Start, req.End = before.Add(time.Hour), before.Add(2*time.Hour)
	if series, err := src(context.Background(), req); err != nil || len(series) != 0 || len(*instants) != 0 {
		t.Fatalf("future window = %+v, %v after %d health reads; want nothing and no query", series, err, len(*instants))
	}
}
