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
	"net/http"
	"net/http/httptest"
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

// bandwidthFakeProm serves health instants per source (by job substring) and a
// fixed matrix for the range query.
func bandwidthFakeProm(t *testing.T, unhealthyJobs ...string) (*httptest.Server, *[]string) {
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
		for _, job := range unhealthyJobs {
			// `up{job="traefik"}` must not match `up{job="traefik-websocket-meter"}`.
			if strings.Contains(query, `up{job="`+job+`"}`) {
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
	ts, queries := bandwidthFakeProm(t, "bex-egress-meter")
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
		case strings.Contains(query, "increase(traefik_router_responses_bytes_total"):
			value = "1048576"
		case strings.Contains(query, "increase(bex_websocket_egress_bytes_total"):
			value = "2097152"
		case strings.Contains(query, "increase(bex_app_direct_egress_bytes_total"):
			value = "3145728"
		case strings.Contains(query, `up{job="traefik"}`): // the http health product
			value = "0"
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"value":[0,"` + value + `"]}]}}`))
	}))
	defer ts.Close()

	now := time.Now()
	value, degraded, err := NewMonthToDateBandwidthSource(ts.URL, ts.Client())(
		context.Background(), "srv-web", []string{"default-web-web-onbex-co@kubernetes"}, true, now.Add(-time.Hour), now)
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
