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
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w4/241: a five-minute dashboard range asked for bandwidth at a 3s step. A
// byte bucket is increase(counter[step]) and the counters are scraped every
// 15s, so every 3s bucket held fewer than two samples and "or vector(0)"
// reported recorded traffic as a flat zero. A sub-minute bandwidth step (or
// window) is now a named bad request on every adapter instead.

func bandwidthGranularityService(t *testing.T) (*Service, *[]RequestMetricsRequest) {
	t.Helper()
	var got []RequestMetricsRequest
	src := func(_ context.Context, req RequestMetricsRequest) ([]MetricSeries, error) {
		got = append(got, req)
		return []MetricSeries{{Unit: unitBytes, Points: []MetricPoint{{Timestamp: "2026-10-10T16:15:00Z", Value: 245}}}}, nil
	}
	return newService(nil, src, sampleApp("web")), &got
}

func TestBandwidthRefusesStepsAndWindowsBelowOneMinute(t *testing.T) {
	svc, got := bandwidthGranularityService(t)
	end := time.Now().UTC().Truncate(time.Second)
	five := end.Add(-5 * time.Minute)

	for _, tc := range []struct {
		name       string
		start      time.Time
		resolution time.Duration
		want       string
	}{
		{"3s step over five minutes", five, 3 * time.Second, "at least 60 seconds (got 3)"},
		{"30s step", five, 30 * time.Second, "at least 60 seconds (got 30)"},
		{"59s step", five, 59 * time.Second, "at least 60 seconds (got 59)"},
		{"window shorter than one bucket", end.Add(-30 * time.Second), time.Minute, "window must span at least 60 seconds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := svc.Metrics(context.Background(), MetricQuery{
				App: "web", Metric: MetricBandwidth, Start: tc.start, End: end, Resolution: tc.resolution,
			})
			if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("err = %v, want ErrBadRequest containing %q", err, tc.want)
			}
		})
	}
	if len(*got) != 0 {
		t.Fatalf("refused bandwidth reached the source: %+v", *got)
	}

	for _, tc := range []struct {
		name       string
		resolution time.Duration
		want       time.Duration
	}{
		{"omitted resolution defaults to 60s", 0, time.Minute},
		{"60s", time.Minute, time.Minute},
		{"300s", 5 * time.Minute, 5 * time.Minute},
	} {
		t.Run(tc.name, func(t *testing.T) {
			*got = nil
			series, err := svc.Metrics(context.Background(), MetricQuery{
				App: "web", Metric: MetricBandwidth, Start: five, End: end, Resolution: tc.resolution,
			})
			if err != nil || len(series) != 1 || series[0].Unit != unitBytes || series[0].Points[0].Value != 245 {
				t.Fatalf("Metrics: err=%v series=%+v", err, series)
			}
			if len(*got) != 1 || (*got)[0].Resolution != tc.want {
				t.Fatalf("source requests = %+v, want one at step %v", *got, tc.want)
			}
		})
	}

	// The minimum is bandwidth-only: request counts keep the finer step.
	*got = nil
	if _, err := svc.Metrics(context.Background(), MetricQuery{
		App: "web", Metric: MetricHTTPRequests, Start: five, End: end, Resolution: 3 * time.Second,
	}); err != nil || len(*got) != 1 || (*got)[0].Resolution != 3*time.Second {
		t.Fatalf("http_requests at 3s: err=%v requests=%+v", err, *got)
	}
}

func TestBandwidthMinimumOnGraphQLAndMCP(t *testing.T) {
	svc, _ := bandwidthGranularityService(t)
	end := time.Now().UTC().Truncate(time.Second)
	start := end.Add(-5 * time.Minute)
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	for _, name := range []string{"BANDWIDTH", "ENRICHED_BANDWIDTH"} {
		for _, tc := range []struct {
			resolution string
			wantErr    bool
		}{{"resolution: 3", true}, {"resolution: 60", false}, {"resolution: 300", false}, {"", false}} {
			res := graphql.Do(graphql.Params{Schema: schema, Context: context.Background(), RequestString: fmt.Sprintf(
				`{ metrics(query: {filters: [{field: "RESOURCE", values: ["web"]}], name: %q, start: %q, end: %q, %s}) { unit values { value } } }`,
				name, start.Format(time.RFC3339), end.Format(time.RFC3339), tc.resolution)})
			if tc.wantErr {
				if len(res.Errors) == 0 || !strings.Contains(res.Errors[0].Message, "bandwidth resolution must be at least 60 seconds") {
					t.Errorf("%s %q: errors=%v, want the named minimum", name, tc.resolution, res.Errors)
				}
				continue
			}
			if len(res.Errors) > 0 {
				t.Errorf("%s %q: unexpected errors %v", name, tc.resolution, res.Errors)
			}
		}
	}

	cs := mcpSession(t, svc)
	for _, metricType := range []string{"bandwidth", "bandwidth_usage"} {
		for _, spelling := range []string{"resolution", "resolutionSeconds"} {
			for _, tc := range []struct {
				seconds float64
				wantErr bool
			}{{3, true}, {60, false}} {
				result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{
					Name: "get_metrics",
					Arguments: map[string]any{
						"resourceId": "web", "metricTypes": []string{metricType},
						"startTime": start.Format(time.RFC3339), "endTime": end.Format(time.RFC3339),
						spelling: tc.seconds,
					},
				})
				if err != nil {
					t.Fatalf("%s %s=%v: %v", metricType, spelling, tc.seconds, err)
				}
				if result.IsError != tc.wantErr {
					t.Errorf("%s %s=%v: IsError=%v, want %v (%+v)", metricType, spelling, tc.seconds, result.IsError, tc.wantErr, result.Content)
				}
			}
		}
	}
}

func TestRESTBandwidthKeepsDefaultStepAndRefusesSubMinuteWindow(t *testing.T) {
	svc, got := bandwidthGranularityService(t)
	end := time.Now().UTC().Truncate(time.Second)

	rec := serveREST(svc, "/v1/metrics/bandwidth?resource=web&startTime="+end.Add(-5*time.Minute).Format(time.RFC3339)+"&endTime="+end.Format(time.RFC3339))
	if rec.Code != http.StatusOK || len(*got) != 1 || (*got)[0].Resolution != time.Minute {
		t.Fatalf("five-minute REST bandwidth: status=%d body=%s requests=%+v", rec.Code, rec.Body.String(), *got)
	}

	rec = serveREST(svc, "/v1/metrics/bandwidth?resource=web&startTime="+end.Add(-30*time.Second).Format(time.RFC3339)+"&endTime="+end.Format(time.RFC3339))
	if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), "window must span at least 60 seconds") {
		t.Fatalf("30s REST bandwidth window: status=%d body=%s", rec.Code, rec.Body.String())
	}
}
