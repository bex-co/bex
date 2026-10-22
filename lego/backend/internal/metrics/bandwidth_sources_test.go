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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/egressquery"
)

// w8/190: GET /metrics/bandwidth-sources answers Render's hourly breakdown, one
// source-narrowed read each, in MiB.
func TestBandwidthSourcesServesHourlyPerSourceSeries(t *testing.T) {
	var got []RequestMetricsRequest
	src := func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		got = append(got, req)
		// 2 MiB for the total, 1 MiB for each narrowed source.
		bytes := float64(1 << 20)
		if req.BandwidthSource == "" {
			bytes *= 2
		}
		return []MetricSeries{{Unit: unitBytes, Points: []MetricPoint{{Timestamp: "2026-10-10T16:00:00Z", Value: bytes}}}}, nil
	}
	svc := newService(nil, src, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)

	rr := httptest.NewRecorder()
	mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet,
		"/v1/metrics/bandwidth-sources?resource=web&startTime=2026-10-10T15:00:00Z&endTime=2026-10-10T17:00:00Z", nil))
	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rr.Code, rr.Body.String())
	}
	var body struct {
		Data []struct {
			Labels struct{ Resource, TrafficSource string }
			Values []struct {
				Timestamp int64
				Value     float64
			}
		}
	}
	if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	sources := map[string]float64{}
	for _, series := range body.Data {
		if series.Labels.Resource != "web" || len(series.Values) != 1 || series.Values[0].Timestamp != time.Date(2026, 10, 10, 16, 0, 0, 0, time.UTC).Unix() {
			t.Fatalf("series = %+v", series)
		}
		sources[series.Labels.TrafficSource] = series.Values[0].Value
	}
	want := map[string]float64{"total": 2, "http": 1, "websocket": 1, "nat": 1}
	if len(sources) != len(want) {
		t.Fatalf("sources = %v, want %v", sources, want)
	}
	for k, v := range want {
		if sources[k] != v {
			t.Errorf("%s = %v MiB, want %v", k, sources[k], v)
		}
	}
	for _, req := range got {
		if req.Resolution.Hours() != 1 {
			t.Errorf("resolution = %v, want hourly", req.Resolution)
		}
	}
}

func TestBandwidthSourcesRefusals(t *testing.T) {
	svc := newService(nil, func(context.Context, RequestMetricsRequest) ([]MetricSeries, error) { return nil, nil }, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	many := make([]string, 31)
	for i := range many {
		many[i] = "resource=web"
	}
	for _, tc := range []struct{ name, query, want string }{
		{"before the data begins", "resource=web&startTime=2025-03-08T00:00:00Z&endTime=2025-03-10T00:00:00Z", "only available after 2025-03-09"},
		{"too many resources", strings.Join(many, "&"), "too_many_resources"},
		{"no resource", "startTime=2026-10-10T15:00:00Z", "resource is required"},
	} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/metrics/bandwidth-sources?"+tc.query, nil))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), tc.want) {
			t.Errorf("%s: %d %s, want 400 containing %q", tc.name, rr.Code, rr.Body.String(), tc.want)
		}
	}
}

// A source-narrowed read selects only that source's counters; the all-sources
// read is unchanged.
func TestBandwidthSpecsNarrowToTheRequestedSource(t *testing.T) {
	base := RequestMetricsRequest{AppID: "srv-1", Routers: []string{"web"}, Direct: true}
	sourcesOf := func(source string) []egressquery.Source {
		req := base
		req.BandwidthSource = source
		var out []egressquery.Source
		for _, spec := range bandwidthSpecs(req) {
			out = append(out, spec.Source)
		}
		return out
	}
	if got := sourcesOf(""); !slices.Equal(got, []egressquery.Source{egressquery.HTTP, egressquery.WebSocket, egressquery.Direct}) {
		t.Errorf("all = %v", got)
	}
	for source, want := range map[string]egressquery.Source{"http": egressquery.HTTP, "websocket": egressquery.WebSocket, "nat": egressquery.Direct} {
		if got := sourcesOf(source); !slices.Equal(got, []egressquery.Source{want}) {
			t.Errorf("%s = %v, want [%s]", source, got, want)
		}
	}
}
