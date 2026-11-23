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
	"net/url"
	"slices"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/bex-co/bex/lego/backend/internal/egressquery"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// promLikeBandwidth answers a bandwidth read as Prometheus does under w8/413's
// bound at now: a point every step from req.Start to the earlier of req.End
// and now, each holding the bytes sent in the step that ends there. The
// websocket and nat sources sent nothing.
func promLikeBandwidth(now time.Time, sent map[time.Time]float64) RequestMetricsSource {
	return func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		end := req.End
		if end.After(now) {
			end = now
		}
		points := []MetricPoint{}
		for at := req.Start; !at.After(end); at = at.Add(req.Resolution) {
			var bytes float64
			for when, b := range sent {
				if req.BandwidthSource != "websocket" && req.BandwidthSource != "nat" &&
					when.After(at.Add(-req.Resolution)) && !when.After(at) {
					bytes += b
				}
			}
			points = append(points, MetricPoint{Timestamp: at.Format(time.RFC3339), Value: bytes})
		}
		return []MetricSeries{{Unit: unitBytes, Points: points}}, nil
	}
}

type sourcePoints map[string][]bandwidthSourcePoint

func getBandwidthSources(t *testing.T, svc *Service, query url.Values) sourcePoints {
	t.Helper()
	rec := serveREST(svc, "/v1/metrics/bandwidth-sources?"+query.Encode())
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d: %s", rec.Code, rec.Body.String())
	}
	var body struct{ Data []bandwidthSourceSeries }
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	out := sourcePoints{}
	for _, series := range body.Data {
		if series.Labels.Resource != "web" {
			t.Fatalf("series = %+v", series)
		}
		out[series.Labels.TrafficSource] = series.Values
	}
	if len(out) != len(bandwidthTrafficSources) {
		t.Fatalf("sources = %v, want one series per traffic source", out)
	}
	return out
}

// The w8/414 repro: a free service served paced requests from 02:03:24 to
// 02:05:54, and the read came at 02:11:06.
var (
	sourcesNow  = time.Date(2026, 11, 23, 2, 11, 6, 0, time.UTC)
	sourcesSent = func() map[time.Time]float64 {
		sent := map[time.Time]float64{}
		for i := range 6 {
			sent[time.Date(2026, 11, 23, 2, 3, 24, 0, time.UTC).Add(time.Duration(i)*30*time.Second)] = 16 << 10
		}
		return sent
	}()
)

func hourOf(h int) int64 { return time.Date(2026, 11, 23, h, 0, 0, 0, time.UTC).Unix() }

func sourcesService(app *appv1alpha1.App) *Service {
	svc := newService(nil, promLikeBandwidth(sourcesNow, sourcesSent), app)
	svc.Clock = func() time.Time { return sourcesNow }
	return svc
}

// w8/190, w8/414: Render's hourly breakdown, one source-narrowed read each, in
// MiB, stamped on whole UTC hours, the hour in progress included.
func TestBandwidthSourcesStampsWholeHoursFromTheirStart(t *testing.T) {
	sentMiB := 6.0 * (16 << 10) / (1 << 20)
	for _, tc := range []struct {
		name       string
		start, end string
		created    time.Time
		want       []bandwidthSourcePoint
	}{
		{"a window ending now", "2026-11-23T00:00:00Z", "2026-11-23T02:11:06Z", time.Time{},
			[]bandwidthSourcePoint{{hourOf(0), 0}, {hourOf(1), 0}, {hourOf(2), sentMiB}}},
		{"a service created this hour", "2026-11-23T00:00:00Z", "2026-11-23T02:11:06Z", time.Date(2026, 11, 23, 2, 2, 37, 0, time.UTC),
			[]bandwidthSourcePoint{{hourOf(2), sentMiB}}},
		{"a start off the hour", "2026-11-23T01:30:00Z", "2026-11-23T02:11:06Z", time.Time{},
			[]bandwidthSourcePoint{{hourOf(1), 0}, {hourOf(2), sentMiB}}},
		{"a window ending after now", "2026-11-23T00:00:00Z", "2026-11-23T06:00:00Z", time.Time{},
			[]bandwidthSourcePoint{{hourOf(0), 0}, {hourOf(1), 0}, {hourOf(2), sentMiB}}},
		{"no window", "", "", time.Time{},
			[]bandwidthSourcePoint{{hourOf(1), 0}, {hourOf(2), sentMiB}}},
		{"a window wholly after now", "2026-11-23T04:00:00Z", "2026-11-23T06:00:00Z", time.Time{},
			[]bandwidthSourcePoint{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			app := sampleApp("web")
			app.CreationTimestamp = metav1.NewTime(tc.created)
			query := url.Values{"resource": {"web"}}
			if tc.start != "" {
				query.Set("startTime", tc.start)
				query.Set("endTime", tc.end)
			}
			got := getBandwidthSources(t, sourcesService(app), query)
			for source, want := range map[string][]bandwidthSourcePoint{"total": tc.want, "http": tc.want} {
				if !slices.Equal(got[source], want) {
					t.Errorf("%s = %v, want %v", source, got[source], want)
				}
			}
			for source, points := range got {
				for _, p := range points {
					if p.Timestamp%3600 != 0 || p.Timestamp > sourcesNow.Unix() {
						t.Errorf("%s stamps %s: not a whole UTC hour at or before now", source, time.Unix(p.Timestamp, 0).UTC())
					}
					if (source == "websocket" || source == "nat") && p.Value != 0 {
						t.Errorf("%s = %v at %d, but sent nothing", source, p.Value, p.Timestamp)
					}
				}
			}
		})
	}
}

// The total agrees with /v1/metrics/bandwidth summed over the same window
// (w8/190's acceptance), here the hour in progress.
func TestBandwidthSourcesTotalMatchesTheBandwidthSum(t *testing.T) {
	svc := sourcesService(sampleApp("web"))
	window := url.Values{"resource": {"web"}, "startTime": {"2026-11-23T02:00:00Z"}, "endTime": {"2026-11-23T02:11:06Z"}}
	rec := serveREST(svc, "/v1/metrics/bandwidth?"+window.Encode())
	if rec.Code != http.StatusOK {
		t.Fatalf("bandwidth status = %d: %s", rec.Code, rec.Body.String())
	}
	var series []struct{ Values []struct{ Value float64 } }
	if err := json.Unmarshal(rec.Body.Bytes(), &series); err != nil {
		t.Fatal(err)
	}
	var bytes float64
	for _, ser := range series {
		for _, p := range ser.Values {
			bytes += p.Value
		}
	}
	total := getBandwidthSources(t, svc, window)["total"]
	if bytes == 0 || len(total) != 1 || total[0].Timestamp != hourOf(2) || total[0].Value*(1<<20) != bytes {
		t.Fatalf("total = %v, want one point at 02:00 holding the %v bytes /v1/metrics/bandwidth sums", total, bytes)
	}
}

// The read widens the window to whole hours, and BEX_MAX_QUERY_HOURS holds for
// the window as requested: the last 30 days off the hour are served, a longer
// window is refused. The step stays within the per-series point budget.
func TestBandwidthSourcesCapAppliesToTheRequestedWindow(t *testing.T) {
	var reads []RequestMetricsRequest
	svc := newService(nil, func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		reads = append(reads, req)
		return nil, nil
	}, sampleApp("web"))
	svc.Clock = func() time.Time { return sourcesNow }
	svc.MaxQueryHours = 720
	window := func(span time.Duration) url.Values {
		return url.Values{"resource": {"web"}, "startTime": {sourcesNow.Add(-span).Format(time.RFC3339)}, "endTime": {sourcesNow.Format(time.RFC3339)}}
	}
	getBandwidthSources(t, svc, window(720*time.Hour))
	wantStart := sourcesNow.Add(-720 * time.Hour).Truncate(time.Hour)
	for _, req := range reads {
		if !req.Start.Equal(wantStart) || !req.End.Equal(sourcesNow) || req.Resolution != 4*time.Minute {
			t.Fatalf("read %s–%s every %s, want %s–%s every 4m", req.Start, req.End, req.Resolution, wantStart, sourcesNow)
		}
	}
	rec := serveREST(svc, "/v1/metrics/bandwidth-sources?"+window(721*time.Hour).Encode())
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "exceeds 720 hours") {
		t.Fatalf("721 hours = %d %s, want the cap's 400", rec.Code, rec.Body.String())
	}
}

func TestHourlyReadSpansTheWholeHoursUpToNow(t *testing.T) {
	at := func(h, m int) time.Time { return time.Date(2026, 11, 23, h, m, 0, 0, time.UTC) }
	for _, tc := range []struct {
		name            string
		start, end, now time.Time
		from, to        time.Time
		step            time.Duration
	}{
		{"past hours", at(0, 10), at(1, 50), at(5, 0), at(0, 0), at(2, 0), time.Minute},
		{"an end on the hour", at(0, 0), at(2, 0), at(5, 0), at(0, 0), at(2, 0), time.Minute},
		{"the hour in progress", at(1, 30), at(2, 11), at(2, 11), at(1, 0), at(2, 11), time.Minute},
		{"its first minute", at(2, 0), at(2, 0).Add(30 * time.Second), at(2, 0).Add(30 * time.Second), at(2, 0), at(2, 1), time.Minute},
		{"after now", at(4, 0), at(6, 0), at(2, 11), at(4, 0), at(4, 1), time.Minute},
		{"ten days", at(2, 0).Add(-240 * time.Hour), at(2, 0), at(2, 0), at(2, 0).Add(-240 * time.Hour), at(2, 0), 2 * time.Minute},
	} {
		from, to, step := hourlyRead(tc.start, tc.end, tc.now)
		if !from.Equal(tc.from) || !to.Equal(tc.to) || step != tc.step {
			t.Errorf("%s: %s–%s every %s, want %s–%s every %s", tc.name, from, to, step, tc.from, tc.to, tc.step)
		}
	}
}

func TestBandwidthSourcesRefusals(t *testing.T) {
	svc := newService(nil, func(context.Context, RequestMetricsRequest) ([]MetricSeries, error) { return nil, nil }, sampleApp("web"))
	mux := http.NewServeMux()
	svc.RegisterREST(mux)
	limit := make([]string, maxBandwidthSourceResources)
	for i := range limit {
		limit[i] = "resource=web"
	}
	rrOK := httptest.NewRecorder()
	mux.ServeHTTP(rrOK, httptest.NewRequest(http.MethodGet, "/v1/metrics/bandwidth-sources?"+strings.Join(limit, "&"), nil))
	if rrOK.Code != http.StatusOK {
		t.Fatalf("at limit (%d resources): %d %s, want 200", maxBandwidthSourceResources, rrOK.Code, rrOK.Body.String())
	}
	many := append(limit, "resource=web")
	for _, tc := range []struct{ name, query, want, wantCode string }{
		{"before the data begins", "resource=web&startTime=2025-03-08T00:00:00Z&endTime=2025-03-10T00:00:00Z", "only available after 2025-03-09", ""},
		{"too many resources", strings.Join(many, "&"), "at most 30 resources", TooManyResourcesCode},
		{"no resource", "startTime=2026-10-10T15:00:00Z", "resource is required", ""},
	} {
		rr := httptest.NewRecorder()
		mux.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/v1/metrics/bandwidth-sources?"+tc.query, nil))
		if rr.Code != http.StatusBadRequest || !strings.Contains(rr.Body.String(), tc.want) {
			t.Errorf("%s: %d %s, want 400 containing %q", tc.name, rr.Code, rr.Body.String(), tc.want)
			continue
		}
		var body map[string]any
		if err := json.Unmarshal(rr.Body.Bytes(), &body); err != nil {
			t.Errorf("%s: invalid JSON %q: %v", tc.name, rr.Body.String(), err)
			continue
		}
		gotCode, _ := body["code"].(string)
		if gotCode != tc.wantCode {
			t.Errorf("%s: code = %q, want %q (body: %s)", tc.name, gotCode, tc.wantCode, rr.Body.String())
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
