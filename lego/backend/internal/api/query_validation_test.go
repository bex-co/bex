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
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/logs"
	"github.com/bex-co/bex/lego/backend/internal/metrics"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Exercise the composed HTTP handler: fragment-only GraphQL tests bypass the
// sanitizer that used to turn these caller errors into "internal error".
func assertQueryError(t *testing.T, h http.Handler, query, want string) {
	t.Helper()
	body, err := json.Marshal(map[string]string{"query": query})
	if err != nil {
		t.Fatal(err)
	}
	rec := do(t, h, http.MethodPost, "/graphql", testToken, string(body))
	var result struct {
		Errors []struct{ Message string } `json:"errors"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if rec.Code != http.StatusOK || len(result.Errors) != 1 || !strings.Contains(result.Errors[0].Message, want) {
		t.Fatalf("query error = %d %s, want %q", rec.Code, rec.Body, want)
	}
}

func TestMetricsValidationMessagesSurviveGraphQLSanitization(t *testing.T) {
	h, _ := serverWith(t, &core.Base{Client: fakeClient(sampleApp("web"), ownedDB("pg", ""), ownedKV("kv", "")), Namespace: "default"}, Deps{})
	for _, tc := range []struct{ input, message string }{
		{`metrics(query: {name: "FOO_METRIC", filters: [{field: "RESOURCE", values: ["web"]}]})`, `unknown metrics name "FOO_METRIC"`},
		{`metrics(query: {name: "MEMORY", filters: []})`, "filters must include a RESOURCE entry"},
		{`datastoreMetrics(query: {resource: "pg", name: "FOO_METRIC"})`, `unknown datastore metrics name "FOO_METRIC"`},
		{`datastoreMetrics(query: {resource: "", name: "DISK"})`, "resource is required"},
		{`datastoreMetrics(query: {resource: "pg", name: "DISK", kind: "bogus"})`, `unknown datastore kind "bogus"`},
		{`datastoreMetrics(query: {resource: "pg", name: "MEMORY"})`, `metric "kv_memory" is key-value-only`},
		{`datastoreMetrics(query: {resource: "kv", name: "DB_CONNECTIONS", kind: "keyvalue"})`, `metric "db_connections" is Postgres-only`},
		{`datastoreMetrics(query: {resource: "web", name: "DB_CONNECTIONS", kind: "service"})`, `metric "db_connections" is not valid for a service resource`},
	} {
		t.Run(tc.input, func(t *testing.T) {
			assertQueryError(t, h, "{ "+tc.input+" { unit } }", "bad request: "+tc.message)
		})
	}
}

func TestInvalidQueryRangesAcrossSurfaces(t *testing.T) {
	base := &core.Base{Client: fakeClient(sampleApp("web"), ownedDB("pg", ""),
		ownedDB("dpg-c185th5c2rvvnhbfiltg", ""), ownedKV("red-c185th5c2rvvnhbfiltg", "")), Namespace: "default"}
	h, srv := serverWith(t, base, Deps{
		LogHistory: func(context.Context, string, logs.LogQuery) ([]logs.LogEntry, error) {
			t.Error("invalid range reached Loki")
			return nil, nil
		},
		ResourceMetricsRange: func(context.Context, metrics.ResourceMetricsRangeRequest) ([]metrics.MetricSeries, error) {
			t.Error("invalid range reached Prometheus")
			return nil, nil
		},
		DiskUsage: func(context.Context, metrics.DiskUsageRequest) ([]metrics.MetricSeries, error) {
			t.Error("invalid datastore range reached Prometheus")
			return nil, nil
		},
	})
	cs := mcpSession(t, srv)
	start := "2026-09-07T06:00:00Z"
	for _, end := range []string{"2026-09-07T00:00:00Z", start} {
		t.Run(end, func(t *testing.T) {
			const message = "must be before"
			window := `start: "` + start + `", end: "` + end + `"`
			assertQueryError(t, h, `{ metrics(query: {name: "MEMORY", filters: [{field: "RESOURCE", values: ["web"]}], `+window+`}) { unit } }`, message)
			assertQueryError(t, h, `{ datastoreMetrics(query: {resource: "pg", name: "DISK", `+window+`}) { unit } }`, message)
			assertQueryError(t, h, `{ logs(resource: "web", startTime: "`+start+`", endTime: "`+end+`") { timestamp } }`, message)
			for _, path := range []string{
				"/v1/metrics/memory?resource=web", "/v1/metrics/disk?resource=pg",
				"/v1/logs?resource=web", "/v1/logs/values?resource=web&label=level", "/v1/logs/subscribe?resource=web",
			} {
				rec := do(t, h, http.MethodGet, path+"&startTime="+start+"&endTime="+end, testToken, "")
				if rec.Code != http.StatusBadRequest || !strings.Contains(rec.Body.String(), message) {
					t.Errorf("%s = %d %s, want bad request with range message", path, rec.Code, rec.Body)
				}
			}
			for _, tc := range []struct {
				tool string
				args map[string]any
			}{
				{"get_metrics", map[string]any{"resource": []string{"web"}, "metricTypes": []string{"memory"}}},
				{"get_datastore_metrics", map[string]any{"resource": "pg", "metricTypes": []string{"disk"}}},
				{"list_logs", map[string]any{"resource": []string{"web"}}},
			} {
				tc.args["startTime"], tc.args["endTime"] = start, end
				result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tc.tool, Arguments: tc.args})
				if err != nil {
					t.Fatal(err)
				}
				payload, _ := json.Marshal(result.Content)
				if !result.IsError || !strings.Contains(string(payload), message) {
					t.Errorf("%s = %s, want range error", tc.tool, payload)
				}
			}
			// Direct service calls, including datastore log paths, must refuse
			// the same input even when no adapter checks the window first.
			from, _ := time.Parse(time.RFC3339, start)
			to, _ := time.Parse(time.RFC3339, end)
			for _, resource := range []string{"web", "dpg-c185th5c2rvvnhbfiltg", "red-c185th5c2rvvnhbfiltg"} {
				if _, err := srv.Logs.QueryLogs(context.Background(), logs.LogQuery{App: resource, Since: from, End: to}); !errors.Is(err, core.ErrBadRequest) {
					t.Errorf("QueryLogs = %v, want bad request", err)
				}
			}
		})
	}
}

func TestQueryUpstreamFailuresRemainRedacted(t *testing.T) {
	upstream := errors.New("upstream failed: private-host/internal-token")
	h, _ := serverWith(t, &core.Base{Client: fakeClient(sampleApp("web"), ownedDB("pg", "")), Namespace: "default"}, Deps{
		LogHistory: func(context.Context, string, logs.LogQuery) ([]logs.LogEntry, error) { return nil, upstream },
		ResourceMetricsRange: func(context.Context, metrics.ResourceMetricsRangeRequest) ([]metrics.MetricSeries, error) {
			return nil, upstream
		},
		DiskUsage: func(context.Context, metrics.DiskUsageRequest) ([]metrics.MetricSeries, error) { return nil, upstream },
	})
	for _, query := range []string{
		`{ metrics(query: {name: "MEMORY", filters: [{field: "RESOURCE", values: ["web"]}]}) { unit } }`,
		`{ datastoreMetrics(query: {resource: "pg", name: "DISK"}) { unit } }`,
		`{ logs(resource: "web") { timestamp } }`,
	} {
		assertQueryError(t, h, query, "internal error")
	}
	for _, path := range []string{"/v1/metrics/memory?resource=web", "/v1/metrics/disk?resource=pg", "/v1/logs?resource=web"} {
		rec := do(t, h, http.MethodGet, path, testToken, "")
		if rec.Code != http.StatusInternalServerError || !strings.Contains(rec.Body.String(), "internal error") || strings.Contains(rec.Body.String(), "private-host") {
			t.Errorf("%s = %d %s, want redacted server error", path, rec.Code, rec.Body)
		}
	}
}

func TestMalformedServiceIDsAcrossSurfaces(t *testing.T) {
	h, srv := serverWith(t, deadIDBase(), Deps{})
	cs := mcpSession(t, srv)
	for _, name := range []string{"srv-", "srv-!!", "srv- x", "srv-100%"} {
		t.Run(name, func(t *testing.T) {
			for _, method := range []string{http.MethodGet, http.MethodPatch, http.MethodDelete} {
				body := ""
				if method == http.MethodPatch {
					body = `{ "name": "new-name" }`
				}
				rec := do(t, h, method, "/v1/services/"+url.PathEscape(name), testToken, body)
				if rec.Code != http.StatusNotFound || !strings.Contains(rec.Body.String(), "not_found") {
					t.Errorf("%s %q = %d %s, want not found", method, name, rec.Code, rec.Body)
				}
			}
			assertQueryError(t, h, `{ service(id: "`+name+`") { id } }`, "not found")
			assertQueryError(t, h, `{ logs(resource: "`+name+`") { timestamp } }`, "not found")
			result, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: "get_service", Arguments: map[string]any{"serviceId": name}})
			if err != nil {
				t.Fatal(err)
			}
			payload, _ := json.Marshal(result.Content)
			if !result.IsError || !strings.Contains(string(payload), "not found") {
				t.Errorf("MCP get_service = %s, want not found", payload)
			}
		})
	}
}
