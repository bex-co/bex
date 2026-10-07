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

package api

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// TestAFailedRouterLookupAnswersWithoutItsText (w5/112): a configured metrics
// source whose router lookup fails answers an uncoded "internal error" on every
// surface, never METRICS_UNAVAILABLE or the Kubernetes text, and reads nothing.
func TestAFailedRouterLookupAnswersWithoutItsText(t *testing.T) {
	// Both Apps are Running with public hosts, but the Ingresses their routers
	// are read from are missing.
	web := sampleApp("web")
	web.Spec.Host = "web.example.com"
	site := sampleApp("site")
	site.Spec.Type = appv1alpha1.TypeStaticSite
	site.Spec.Host = "site.example.com"
	reads := 0
	h, srv := serverWith(t, &core.Base{Client: fakeClient(web, site), Namespace: "default"}, Deps{
		RequestMetrics: func(context.Context, metrics.RequestMetricsRequest) ([]metrics.MetricSeries, error) {
			reads++
			return nil, nil
		},
		MonthToDateBandwidth: func(context.Context, string, []string, bool, time.Time, time.Time) (metrics.BandwidthBytes, []string, error) {
			reads++
			return metrics.BandwidthBytes{}, nil, nil
		},
		MetricsFilterValues: func(context.Context, metrics.MetricsFilterValuesRequest) ([]string, error) {
			reads++
			return nil, nil
		},
	})
	internalError := codedRefusal{status: http.StatusInternalServerError, msg: "internal error"}

	for _, path := range []string{
		"/v1/metrics/bandwidth?resource=web",
		"/v1/metrics/http-requests?resource=site",
		"/v1/metrics/filters/http?resource=site",
	} {
		internalError.onREST(t, h, http.MethodGet, path, "")
	}
	for _, query := range []string{
		`{ metrics(query: {name: "BANDWIDTH", filters: [{field: "RESOURCE", values: ["web"]}]}) { unit } }`,
		`{ metrics(query: {name: "HTTP_REQUESTS", filters: [{field: "RESOURCE", values: ["site"]}]}) { unit } }`,
		`{ monthToDateBandwidth(resourceId: "web") { egressBandwidthMB } }`,
		`{ metricsFilters(query: {filters: [{field: "RESOURCE", values: ["site"]}], outputFilters: ["STATUS_CODE"]}) { values { field } } }`,
	} {
		internalError.onGraphQL(t, h, query)
	}
	cs := mcpSessionAs(t, srv, "dana")
	for resource, metricType := range map[string]string{"web": "bandwidth_usage", "site": "http_request_count"} {
		internalError.onMCP(t, cs, "get_metrics", map[string]any{"resourceId": resource, "metricTypes": []string{metricType}})
	}
	if reads != 0 {
		t.Fatalf("%d reads ran without the App's routers, want none", reads)
	}
}
