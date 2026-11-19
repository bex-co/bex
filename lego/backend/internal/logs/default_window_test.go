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

package logs

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// w5/197: Render's List logs and List log label values document startTime as
// "Defaults to `now() - 1 hour`" (render-oss/cli pkg/client/types_gen.go,
// ListLogsParams and ListLogsValuesParams), and its MCP server's list_logs and
// list_log_label_values as "Defaults to 1 hour ago". Every bex read on those
// surfaces reads the same hour, not the seven days bex read before.

// defaultWindowAges are the fixture lines' ages: three inside the last hour
// and three outside it, one of them just past its edge.
var defaultWindowAges = []time.Duration{10 * time.Minute, 30 * time.Minute, 50 * time.Minute, 70 * time.Minute, 3 * time.Hour, 3 * day}

// inTheLastHour are the ages, in minutes, of the fixture lines a read without
// a startTime returns.
var inTheLastHour = []int{10, 30, 50}

// minutesOld reads a line's age in whole minutes back from its timestamp.
func minutesOld(t *testing.T, now time.Time, stamp string) int {
	t.Helper()
	ts, err := time.Parse(time.RFC3339Nano, stamp)
	if err != nil {
		t.Fatalf("timestamp %q: %v", stamp, err)
	}
	return int(now.Sub(ts).Round(time.Minute) / time.Minute)
}

// ageLabelLoki answers Loki's label-values API with one value per fixture
// line inside the requested [start, end), named by its age in minutes, so a
// discovery shows exactly which lines the window reached.
func ageLabelLoki(t *testing.T, now time.Time) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start, _ := strconv.ParseInt(r.URL.Query().Get("start"), 10, 64)
		end, _ := strconv.ParseInt(r.URL.Query().Get("end"), 10, 64)
		values := []string{}
		for _, age := range defaultWindowAges {
			if ns := now.Add(-age).UnixNano(); ns >= start && ns < end {
				values = append(values, fmt.Sprintf("m%04d", int(age/time.Minute)))
			}
		}
		body, _ := json.Marshal(map[string]any{"status": "success", "data": values})
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// pagedRead is what one surface answered a logs read without a startTime.
type pagedRead struct {
	stamps        []string
	nextStartTime string
	hasMore       bool
}

// readWithoutAStart reads web's logs with no startTime on REST, GraphQL and
// MCP.
func readWithoutAStart(t *testing.T, svc *Service) map[string]pagedRead {
	t.Helper()
	out := map[string]pagedRead{}

	env := decodeLogList(t, serveREST(svc, http.MethodGet, "/v1/logs?resource=web&limit=100"))
	read := pagedRead{nextStartTime: env.NextStartTime, hasMore: env.HasMore}
	for _, l := range env.Logs {
		read.stamps = append(read.stamps, l.Timestamp)
	}
	out["REST /v1/logs"] = read

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", limit:100) { hasMore nextStartTime logs { timestamp } } }`)
	page := data["logs"].(map[string]any)
	read = pagedRead{nextStartTime: page["nextStartTime"].(string), hasMore: page["hasMore"].(bool)}
	for _, row := range page["logs"].([]any) {
		read.stamps = append(read.stamps, row.(map[string]any)["timestamp"].(string))
	}
	out["GraphQL logs"] = read

	res, err := mcpLogsClient(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "limit": 100,
	}})
	if err != nil || res.IsError {
		t.Fatalf("MCP list_logs = %+v, %v", res, err)
	}
	var list renderLogList
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &list); err != nil {
		t.Fatal(err)
	}
	read = pagedRead{nextStartTime: list.NextStartTime, hasMore: list.HasMore}
	for _, l := range list.Logs {
		read.stamps = append(read.stamps, l.Timestamp)
	}
	out["MCP list_logs"] = read
	return out
}

// valuesWithoutAStart reads web's `level` values with no startTime on REST,
// GraphQL and MCP.
func valuesWithoutAStart(t *testing.T, svc *Service) map[string][]string {
	t.Helper()
	out := map[string][]string{}

	rec := serveREST(svc, http.MethodGet, "/v1/logs/values?resource=web&label=level&limit=100")
	if rec.Code != http.StatusOK {
		t.Fatalf("REST /v1/logs/values = %d %s", rec.Code, rec.Body.String())
	}
	var values []string
	if err := json.Unmarshal(rec.Body.Bytes(), &values); err != nil {
		t.Fatal(err)
	}
	out["REST /v1/logs/values"] = values

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	values = nil
	for _, v := range runQuery(t, schema, `{ logLabelValues(resource:"web", label:"level") }`)["logLabelValues"].([]any) {
		values = append(values, v.(string))
	}
	out["GraphQL logLabelValues"] = values

	res, err := mcpLogsClient(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "list_log_label_values", Arguments: map[string]any{
		"resource": []string{"web"}, "label": "level",
	}})
	if err != nil || res.IsError {
		t.Fatalf("MCP list_log_label_values = %+v, %v", res, err)
	}
	values = nil
	if err := json.Unmarshal([]byte(res.Content[0].(*mcp.TextContent).Text), &values); err != nil {
		t.Fatal(err)
	}
	out["MCP list_log_label_values"] = values
	return out
}

// checkLastHour fails unless every surface returned exactly the last hour's
// lines, as one complete page whose nextStartTime is that hour's start.
func checkLastHour(t *testing.T, before time.Time, reads map[string]pagedRead) {
	t.Helper()
	after := time.Now()
	for surface, read := range reads {
		var ages []int
		for _, s := range read.stamps {
			ages = append(ages, minutesOld(t, before, s))
		}
		slices.Sort(ages)
		if !slices.Equal(ages, inTheLastHour) {
			t.Errorf("%s without a startTime read lines %v minutes old, want the last hour's %v", surface, ages, inTheLastHour)
		}
		if read.hasMore {
			t.Errorf("%s: hasMore = true, want the hour read whole", surface)
		}
		start, err := time.Parse(time.RFC3339Nano, read.nextStartTime)
		if err != nil || start.Before(before.Add(-renderLookback)) || start.After(after.Add(-renderLookback)) {
			t.Errorf("%s: nextStartTime = %q, want the hour before now", surface, read.nextStartTime)
		}
	}
}

func TestEveryRenderSurfaceReadsTheLastHourWithoutAStart(t *testing.T) {
	now := time.Now()
	stamps := make([]time.Time, len(defaultWindowAges))
	for i, age := range defaultWindowAges {
		stamps[i] = now.Add(-age)
	}
	store := exclusiveEndLoki(t, stamps)
	t.Cleanup(store.Close)
	labels := ageLabelLoki(t, now)
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(store.URL, store.Client())
	svc.LabelValues = NewLokiLabelValuesSource(labels.URL, labels.Client())

	checkLastHour(t, now, readWithoutAStart(t, svc))

	want := []string{"m0010", "m0030", "m0050"}
	for surface, got := range valuesWithoutAStart(t, svc) {
		slices.Sort(got)
		if !slices.Equal(got, want) {
			t.Errorf("%s without a startTime = %v, want the last hour's %v", surface, got, want)
		}
	}

	// An explicit startTime still reaches as far back as it names.
	env := decodeLogList(t, serveREST(svc, http.MethodGet, "/v1/logs?resource=web&limit=100&startTime="+now.Add(-4*day).UTC().Format(time.RFC3339)))
	if len(env.Logs) != len(defaultWindowAges) {
		t.Fatalf("a four-day startTime read %d lines, want all %d", len(env.Logs), len(defaultWindowAges))
	}
}

// The pod-log fallback (no store) reads the container's whole log, so the
// window holds there only because the adapters set the start before the read.
func TestThePodLogFallbackReadsTheLastHourWithoutAStart(t *testing.T) {
	now := time.Now()
	lines := make([]string, 0, len(defaultWindowAges))
	for _, age := range defaultWindowAges {
		lines = append(lines, now.Add(-age).UTC().Format(time.RFC3339Nano)+" line "+age.String())
	}
	svc := newService(map[string][]string{"web-1": lines}, sampleApp("web"), podFor("web", "web-1"))

	checkLastHour(t, now, readWithoutAStart(t, svc))
}
