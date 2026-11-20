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
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/351: Render types resolutionSeconds as a number of at least 30 and its
// pinned client sends a float32, so a decimal or exponent spelling is a valid
// step, rounded to whole seconds; anything else is a named 400.
func TestParseTimeWindowResolutionIsRendersNumber(t *testing.T) {
	for raw, want := range map[string]time.Duration{
		"":     0, // omitted: the service's default applies later
		"60":   time.Minute,
		"60.0": time.Minute,
		"1e2":  100 * time.Second,
		"90.5": 91 * time.Second,
		"30":   30 * time.Second,
		"3e10": maxResolutionSeconds * time.Second, // beyond time.Duration: bounded
	} {
		_, _, got, err := parseTimeWindow(url.Values{"resolutionSeconds": {raw}})
		if err != nil || got != want {
			t.Errorf("resolutionSeconds=%q = %v, %v; want %v", raw, got, err, want)
		}
	}
	for _, raw := range []string{"abc", "29", "29.9", "0", "-60", "NaN", "Inf", "-Inf", "1e400", "60s"} {
		_, _, _, err := parseTimeWindow(url.Values{"resolutionSeconds": {raw}})
		if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "resolutionSeconds must be a number of at least 30") {
			t.Errorf("resolutionSeconds=%q = %v; want the named 400", raw, err)
		}
	}
}

// Every REST metrics path shares parseTimeWindow: a decimal step answers what
// its whole-second spelling answers, and no refusal leaks a Go parser message.
func TestRESTMetricsResolutionSecondsAcceptsRenderNumbers(t *testing.T) {
	var got RequestMetricsRequest
	req := func(_ context.Context, r RequestMetricsRequest) ([]MetricSeries, error) {
		got = r
		return []MetricSeries{{Points: []MetricPoint{{Value: 42}}}}, nil
	}
	svc := newService(nil, req, sampleApp("web"), podWithLimits(webInst), sampleDatabase("pg", false))

	rec := serveREST(svc, "/v1/metrics/http-requests?resource=web&resolutionSeconds=60.0")
	if rec.Code != http.StatusOK || got.Resolution != time.Minute {
		t.Fatalf("http-requests resolutionSeconds=60.0 = %d %s, step %v; want 200 at 1m", rec.Code, rec.Body, got.Resolution)
	}

	paths := map[string]string{"disk-usage": "pg", "active-connections": "pg"}
	for seg := range metricPaths {
		paths[seg] = "web"
	}
	for seg := range datastoreMetricPaths {
		paths[seg] = "pg"
	}
	for seg, resource := range paths {
		base := "/v1/metrics/" + seg + "?resource=" + resource + "&resolutionSeconds="
		for spelling, whole := range map[string]string{"60.0": "60", "1e2": "100", "90.5": "91"} {
			want, rec := serveREST(svc, base+whole), serveREST(svc, base+spelling)
			if rec.Code != want.Code || rec.Body.String() != want.Body.String() {
				t.Errorf("%s resolutionSeconds=%s = %d %s; want %s's %d %s", seg, spelling, rec.Code, rec.Body, whole, want.Code, want.Body)
			}
		}
		for _, bad := range []string{"abc", "29", "NaN"} {
			rec := serveREST(svc, base+bad)
			if body := rec.Body.String(); rec.Code != http.StatusBadRequest || !strings.Contains(body, "resolutionSeconds must be a number of at least 30") || strings.Contains(body, "strconv") {
				t.Errorf("%s resolutionSeconds=%s = %d %s; want the named 400", seg, bad, rec.Code, body)
			}
		}
	}

	for _, bad := range []string{"abc", "NaN", "1.5"} {
		rec := serveREST(svc, "/v1/metrics/http-latency?resource=web&quantile="+bad)
		if body := rec.Body.String(); rec.Code != http.StatusBadRequest || !strings.Contains(body, "quantile must be a number between 0 and 1") || strings.Contains(body, "strconv") {
			t.Errorf("quantile=%s = %d %s; want the named 400", bad, rec.Code, body)
		}
	}
}
