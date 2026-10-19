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
	"net/http"
	"strconv"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// w8/120: a startTime in the future with no endTime asks for lines from an
// instant that has not come yet. The omitted end is now, so the window has not
// begun: every surface answers with an empty page, where Loki was sent a start
// after its end and the read came back a 500. A deploy's log panel asks this
// way from the deploy's creation time, which a database clock running ahead
// can put a moment in the API's future.
func TestLogsFromAFutureInstantAreAnEmptyPageOnEverySurface(t *testing.T) {
	f := newFakeLoki("")
	defer f.srv.Close()
	f.status = http.StatusBadRequest // Loki's answer to a start after its end
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	svc.LabelValues = NewLokiLabelValuesSource(f.srv.URL, f.srv.Client())
	const future = "2030-01-01T00:00:00Z"

	for _, path := range []string{
		"/v1/logs?resource=web&startTime=" + future,
		"/v1/logs?resource=web&direction=forward&startTime=" + future,
	} {
		if env := decodeLogList(t, serveREST(svc, "GET", path)); len(env.Logs) != 0 || env.HasMore {
			t.Errorf("%s = %d lines, hasMore=%v; want an empty last page", path, len(env.Logs), env.HasMore)
		}
	}
	if rec := serveREST(svc, "GET", "/v1/logs/values?resource=web&label=level&startTime="+future); rec.Code != http.StatusOK {
		t.Errorf("label discovery from a future start => %d %s, want 200", rec.Code, rec.Body.String())
	}

	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	runQuery(t, schema, `{ logs(resource:"web", startTime:"`+future+`") { hasMore logs { message } } }`)
	runQuery(t, schema, `{ logLabelValues(resource:"web", label:"level", startTime:"`+future+`") }`)

	for tool, args := range map[string]map[string]any{
		"list_logs":             {"resource": []string{"web"}, "startTime": future},
		"list_log_label_values": {"resource": []string{"web"}, "label": "level", "startTime": future},
	} {
		if res, err := mcpLogsClient(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args}); err != nil || res.IsError {
			t.Errorf("%s from a future start = %+v, %v; want an empty result", tool, res, err)
		}
	}

	if f.lastPath != "" {
		t.Errorf("Loki was asked for %s, want never", f.lastPath)
	}
}

// An endTime in the future alone still reads the lines up to now: the default
// lookback counts back from now, not from the end, where it held nothing yet.
func TestLogsUpToAFutureEndLookBackFromNow(t *testing.T) {
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-9","container":"app"}`,
		"values": `[["1751673601000000000","a line from before now"]]`,
	}))
	defer f.srv.Close()
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())

	if env := decodeLogList(t, serveREST(svc, "GET", "/v1/logs?resource=web&endTime=2030-01-01T00:00:00Z")); len(env.Logs) != 1 {
		t.Fatalf("a future endTime alone = %d lines, want the one before now", len(env.Logs))
	}
	start, err := strconv.ParseInt(f.lastValues.Get("start"), 10, 64)
	if err != nil || time.Unix(0, start).After(time.Now().Add(-lokiLookback)) {
		t.Errorf("Loki window starts at %s, want the lookback from now", time.Unix(0, start).UTC())
	}

	if since, _ := resolveCursorWindow(time.Time{}, time.Now().AddDate(4, 0, 0)); since.After(time.Now()) {
		t.Errorf("the page cursors start at %s, in the future", since)
	}
}
