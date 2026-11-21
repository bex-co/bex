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
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"testing"
	"time"

	"github.com/graphql-go/graphql"
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
// hour counts back from now, not from the end, where it held nothing yet.
func TestLogsUpToAFutureEndLookBackFromNow(t *testing.T) {
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-9","container":"app"}`,
		"values": `[["1751673601000000000","a line from before now"]]`,
	}))
	defer f.srv.Close()
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())

	before := time.Now()
	if env := decodeLogList(t, serveREST(svc, "GET", "/v1/logs?resource=web&endTime=2030-01-01T00:00:00Z")); len(env.Logs) != 1 {
		t.Fatalf("a future endTime alone = %d lines, want the one before now", len(env.Logs))
	}
	start, err := strconv.ParseInt(f.lastValues.Get("start"), 10, 64)
	if from := time.Unix(0, start); err != nil || from.Before(before.Add(-renderLookback)) || from.After(time.Now().Add(-renderLookback)) {
		t.Errorf("Loki window starts at %s, want the hour before now (w5/197)", from.UTC())
	}

	if since, _ := resolveCursorWindow(time.Time{}, time.Now().AddDate(4, 0, 0)); since.After(time.Now()) {
		t.Errorf("the page cursors start at %s, in the future", since)
	}
}

// w8/123: no line is stamped after now, so a far-future endTime reads the
// window up to now. Loki was sent the caller's end, and once that lay more
// than its 720-hour maximum past the defaulted start it refused the query (a
// 500); just under it, the scan walked weeks of empty future and timed out.
func TestLogsUpToAFarFutureEndQueryLokiOnlyUpToNow(t *testing.T) {
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-9","container":"app"}`,
		"values": `[["1751673601000000000","a line from before now"]]`,
	}))
	defer f.srv.Close()
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())

	for _, path := range []string{
		"/v1/logs?resource=web&endTime=2030-01-01T00:00:00Z",
		"/v1/logs?resource=web&endTime=2030-01-01T00:00:00Z&direction=forward",
	} {
		before := time.Now()
		if env := decodeLogList(t, serveREST(svc, "GET", path)); len(env.Logs) != 1 {
			t.Fatalf("%s = %d lines, want the one before now", path, len(env.Logs))
		}
		start, _ := strconv.ParseInt(f.lastValues.Get("start"), 10, 64)
		end, err := strconv.ParseInt(f.lastValues.Get("end"), 10, 64)
		if err != nil || time.Unix(0, end).After(time.Now().Add(lokiFutureGrace)) || time.Unix(0, end).Before(before) {
			t.Errorf("%s: Loki end = %s, want now plus at most the clock grace", path, time.Unix(0, end).UTC())
		}
		if span := time.Duration(end - start); span > renderLookback+lokiFutureGrace+time.Second {
			t.Errorf("%s: Loki window spans %s, want at most the %s default plus the clock grace", path, span, renderLookback)
		}
	}
}

// The query-hours cap measures the window that can hold lines: up to now,
// not up to a future end (w8/123). A start past the cap is clamped to it, not
// refused (w8/374): Render's List logs documents no range limit, so a start of
// any age reads what is kept. Only an inverted window is a 400.
func TestLogQueryCapClampsAStartPastIt(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	svc.MaxQueryHours = 720
	limit := 720 * time.Hour
	before := time.Now()
	past := before.Add(-300 * 24 * time.Hour)
	for _, tc := range []struct {
		name       string
		start, end time.Time
		// wantSince is the clamped start; zero means "now minus the cap",
		// which lies between the test's own clock readings.
		wantSince time.Time
	}{
		{"inside the cap, future end", before.Add(-24 * time.Hour), before.Add(40 * 24 * time.Hour), before.Add(-24 * time.Hour)},
		{"31 days back, future end", before.Add(-31 * 24 * time.Hour), before.Add(24 * time.Hour), time.Time{}},
		{"30 days back, no end", before.Add(-limit), time.Time{}, time.Time{}},
		{"400 days back, no end", before.Add(-400 * 24 * time.Hour), time.Time{}, time.Time{}},
		{"a year back, past end", past.Add(-365 * 24 * time.Hour), past, past.Add(-limit)},
		{"not begun", before.Add(24 * time.Hour), before.Add(40 * 24 * time.Hour), before.Add(24 * time.Hour)},
	} {
		q := LogQuery{Since: tc.start, End: tc.end}
		if err := svc.renderWindow(&q); err != nil {
			t.Errorf("%s: err = %v, want the window clamped", tc.name, err)
			continue
		}
		if !q.End.Equal(tc.end) {
			t.Errorf("%s: end = %s, want the caller's %s", tc.name, q.End.UTC(), tc.end.UTC())
		}
		if tc.wantSince.IsZero() {
			if q.Since.Before(before.Add(-limit)) || q.Since.After(time.Now().Add(-limit)) {
				t.Errorf("%s: start = %s, want now minus %s", tc.name, q.Since.UTC(), limit)
			}
		} else if !q.Since.Equal(tc.wantSince) {
			t.Errorf("%s: start = %s, want %s", tc.name, q.Since.UTC(), tc.wantSince.UTC())
		}
	}

	if err := svc.renderWindow(&LogQuery{Since: past, End: past.Add(-time.Minute)}); err == nil {
		t.Error("an inverted window => nil, want a 400")
	}
}

// BEX_MAX_QUERY_HOURS bounds every log read by clamping (w8/374): REST's query
// and label discovery, GraphQL's logs and logLabelValues, and MCP's list_logs
// and list_log_label_values all answer a start past the cap, and the store is
// asked only for the cap's span before the window's end. This is the scan
// bound w9/004 and w9/m1/t008 added, without the 400 Render never sends.
func TestMaxQueryHoursClampsEveryLogRead(t *testing.T) {
	f := newFakeLoki("")
	defer f.srv.Close()
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	svc.LabelValues = NewLokiLabelValuesSource(f.srv.URL, f.srv.Client())
	svc.MaxQueryHours = 24
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	lines := lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-9","container":"app"}`,
		"values": `[["1751673601000000000","a line"]]`,
	})
	const values = `{"status":"success","data":["info"]}`
	gql := func(q string) error {
		if res := graphql.Do(graphql.Params{Schema: schema, RequestString: q, Context: context.Background()}); len(res.Errors) > 0 {
			return res.Errors[0]
		}
		return nil
	}
	cs := mcpLogsClient(t, svc)
	mcpCall := func(tool string, args map[string]any) error {
		res, err := cs.CallTool(context.Background(), &mcp.CallToolParams{Name: tool, Arguments: args})
		if err == nil && res.IsError {
			err = fmt.Errorf("tool error: %+v", res.Content)
		}
		return err
	}
	rest := func(path string) error {
		if rec := serveREST(svc, "GET", path); rec.Code != http.StatusOK {
			return fmt.Errorf("%d %s", rec.Code, rec.Body.String())
		}
		return nil
	}
	reads := []struct {
		name string
		body string
		run  func(start, end string) error
	}{
		{"REST /v1/logs", lines, func(start, end string) error {
			return rest("/v1/logs?resource=web&" + windowParams(start, end).Encode())
		}},
		{"REST /v1/logs/values", values, func(start, end string) error {
			return rest("/v1/logs/values?resource=web&label=level&" + windowParams(start, end).Encode())
		}},
		{"GraphQL logs", lines, func(start, end string) error {
			return gql(`{ logs(resource:"web", ` + windowArgs(start, end) + `) { logs { message } } }`)
		}},
		{"GraphQL logLabelValues", values, func(start, end string) error {
			return gql(`{ logLabelValues(resource:"web", label:"level", ` + windowArgs(start, end) + `) }`)
		}},
		{"MCP list_logs", lines, func(start, end string) error {
			return mcpCall("list_logs", windowMap(map[string]any{"resource": []string{"web"}}, start, end))
		}},
		{"MCP list_log_label_values", values, func(start, end string) error {
			return mcpCall("list_log_label_values", windowMap(map[string]any{"resource": []string{"web"}, "label": "level"}, start, end))
		}},
	}

	end := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	for _, r := range reads {
		f.body, f.firstValues = r.body, nil
		if err := r.run("2020-01-01T00:00:00Z", end.Format(time.RFC3339)); err != nil {
			t.Errorf("%s over six years to %s: %v, want the window clamped", r.name, end, err)
			continue
		}
		from, to := lokiWindow(t, f)
		if !from.Equal(end.Add(-24*time.Hour)) || !to.Equal(end.Add(time.Nanosecond)) {
			t.Errorf("%s: store read %s .. %s, want the 24 hours before %s", r.name, from.UTC(), to.UTC(), end.UTC())
		}

		f.body, f.firstValues = r.body, nil
		before := time.Now()
		if err := r.run(before.AddDate(0, 0, -400).UTC().Format(time.RFC3339), ""); err != nil {
			t.Errorf("%s from 400 days back: %v, want the window clamped", r.name, err)
			continue
		}
		if from, _ := lokiWindow(t, f); from.Before(before.Add(-24*time.Hour)) || from.After(time.Now().Add(-24*time.Hour)) {
			t.Errorf("%s from 400 days back: store read from %s, want the 24 hours before now", r.name, from.UTC())
		}
	}
}

// A clamped window's page cursors name the clamped start (w8/374), so a client
// paging on from them reads the same window and is never refused.
func TestClampedLogWindowCursorsNameTheClampedStart(t *testing.T) {
	now := time.Now()
	older, newer := now.Add(-48*time.Hour), now.Add(-47*time.Hour)
	f := newFakeLoki(lokiResp(map[string]any{
		"stream": `{"app":"web","pod":"web-9","container":"app"}`,
		"values": fmt.Sprintf(`[["%d","newer"],["%d","older"]]`, newer.UnixNano(), older.UnixNano()),
	}))
	defer f.srv.Close()
	svc := newService(nil, sampleApp("web"))
	svc.History = NewLokiSource(f.srv.URL, f.srv.Client())
	svc.MaxQueryHours = 720
	path := "/v1/logs?resource=web&limit=1&startTime=" + now.AddDate(0, 0, -400).UTC().Format(time.RFC3339)

	before := time.Now()
	env := decodeLogList(t, serveREST(svc, "GET", path))
	if len(env.Logs) != 1 || env.Logs[0].Message != "newer" || !env.HasMore {
		t.Fatalf("a 400-day start with limit 1 = %+v, want the newest line and hasMore", env)
	}
	nextStart, err := time.Parse(time.RFC3339Nano, env.NextStartTime)
	if err != nil || nextStart.Before(before.Add(-720*time.Hour)) || nextStart.After(time.Now().Add(-720*time.Hour)) {
		t.Errorf("nextStartTime = %q, want the clamped start, 720 hours before now", env.NextStartTime)
	}
	if want := newer.Add(-time.Nanosecond).UTC().Format(time.RFC3339Nano); env.NextEndTime != want {
		t.Errorf("nextEndTime = %q, want %q, just before the page's oldest line", env.NextEndTime, want)
	}

	f.body = lokiResp()
	before = time.Now()
	env = decodeLogList(t, serveREST(svc, "GET", path))
	nextStart, err = time.Parse(time.RFC3339, env.NextStartTime)
	if len(env.Logs) != 0 || env.HasMore || err != nil ||
		nextStart.Before(before.Add(-720*time.Hour).Truncate(time.Second)) || nextStart.After(time.Now().Add(-720*time.Hour)) {
		t.Errorf("an empty 400-day read = %+v, want an empty last page whose cursor starts at the clamped start", env)
	}
}

// lokiWindow is the [start, end) the store's first request covered.
func lokiWindow(t *testing.T, f *fakeLoki) (time.Time, time.Time) {
	t.Helper()
	if f.firstValues == nil {
		t.Fatal("the store was never asked")
	}
	start, errStart := strconv.ParseInt(f.firstValues.Get("start"), 10, 64)
	end, errEnd := strconv.ParseInt(f.firstValues.Get("end"), 10, 64)
	if errStart != nil || errEnd != nil {
		t.Fatalf("store window %v: %v %v", f.firstValues, errStart, errEnd)
	}
	return time.Unix(0, start), time.Unix(0, end)
}

func windowParams(start, end string) url.Values {
	v := url.Values{"startTime": {start}}
	if end != "" {
		v.Set("endTime", end)
	}
	return v
}

func windowArgs(start, end string) string {
	args := `startTime:"` + start + `"`
	if end != "" {
		args += `, endTime:"` + end + `"`
	}
	return args
}

func windowMap(args map[string]any, start, end string) map[string]any {
	args["startTime"] = start
	if end != "" {
		args["endTime"] = end
	}
	return args
}
