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
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Render's List logs (w8/m116): "Backward will return most recent logs first.
// Forward will start with the oldest logs in the time range." Every page of a
// cursor walk holds a line until the one that says hasMore=false.

// walkLogPages follows /v1/logs cursors over resources (a query string such
// as "resource=web") until hasMore=false and returns every page.
func walkLogPages(t *testing.T, svc *Service, resources, start, end string, limit int, direction string) []renderLogList {
	t.Helper()
	var pages []renderLogList
	for range 20 {
		path := fmt.Sprintf("/v1/logs?%s&startTime=%s&endTime=%s&limit=%d", resources, start, end, limit)
		if direction != "" {
			path += "&direction=" + direction
		}
		env := decodeLogList(t, serveREST(svc, "GET", path))
		if len(env.Logs) == 0 && len(pages) > 0 {
			t.Fatalf("direction=%q: an empty page followed hasMore=true after %d pages", direction, len(pages))
		}
		pages = append(pages, env)
		if !env.HasMore {
			return pages
		}
		if env.NextStartTime == "" || env.NextEndTime == "" {
			t.Fatalf("direction=%q page %d has more but no cursors: %+v", direction, len(pages), env)
		}
		start, end = env.NextStartTime, env.NextEndTime
	}
	t.Fatalf("direction=%q: paging never reached hasMore=false", direction)
	return nil
}

// pageMessages is each page's messages, in wire order.
func pageMessages(pages []renderLogList) [][]string {
	out := make([][]string, len(pages))
	for i, page := range pages {
		for _, line := range page.Logs {
			out[i] = append(out[i], line.Message)
		}
	}
	return out
}

// storeHistory serves entries as the durable store does: the query's lines,
// oldest-first, capped from the requested end.
func storeHistory(entries []LogEntry) func(context.Context, string, LogQuery) ([]LogEntry, error) {
	return func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		var in []LogEntry
		for _, e := range entries {
			if q.keep(e) {
				in = append(in, e)
			}
		}
		return q.capToLimit(in), nil
	}
}

// storedService serves entries from storeHistory for the App "web".
func storedService(entries ...LogEntry) *Service {
	svc := newService(nil, sampleApp("web"), podFor("web", "web-1"))
	svc.History = storeHistory(entries)
	return svc
}

// sweep465's two planted lines, through the store and the pod-log fallback,
// on every surface: backward lists second before first, forward the reverse,
// and a page that holds the last line says so.
func TestLogPagesListBackwardMostRecentFirst(t *testing.T) {
	const start, end = "2026-10-18T06:00:00Z", "2026-10-18T06:05:00Z"
	firstAt, secondAt := "2026-10-18T06:02:30.455Z", "2026-10-18T06:02:33.177Z"
	stamp := func(s string) time.Time {
		at, _ := time.Parse(time.RFC3339Nano, s)
		return at
	}
	sources := map[string]*Service{
		"pod logs": newService(map[string][]string{"web-1": {firstAt + " first", secondAt + " second"}},
			sampleApp("web"), podFor("web", "web-1")),
		"store": storedService(lineAt(stamp(firstAt), "first"), lineAt(stamp(secondAt), "second")),
	}
	for name, svc := range sources {
		t.Run(name, func(t *testing.T) {
			for _, tc := range []struct {
				direction string
				want      []string
			}{
				{"", []string{"second", "first"}},
				{DirectionBackward, []string{"second", "first"}},
				{DirectionForward, []string{"first", "second"}},
			} {
				if got := pageMessages(walkLogPages(t, svc, "resource=web", start, end, 2, tc.direction)); fmt.Sprint(got) != fmt.Sprint([][]string{tc.want}) {
					t.Errorf("direction=%q limit=2 pages = %v, want one page %v", tc.direction, got, tc.want)
				}
				if got := pageMessages(walkLogPages(t, svc, "resource=web", start, end, 1, tc.direction)); fmt.Sprint(got) != fmt.Sprint([][]string{{tc.want[0]}, {tc.want[1]}}) {
					t.Errorf("direction=%q limit=1 pages = %v, want %v then %v", tc.direction, got, tc.want[0], tc.want[1])
				}
			}

			schema, err := gqlSchema(svc)
			if err != nil {
				t.Fatal(err)
			}
			raw, _ := json.Marshal(runQuery(t, schema, `{ logs(resource:"web", startTime:"`+start+`", endTime:"`+end+`", limit:2) { hasMore logs { message } } }`)["logs"])
			var viaGraphQL wirePage
			if err := json.Unmarshal(raw, &viaGraphQL); err != nil {
				t.Fatal(err)
			}
			result, err := mcpLogsClient(t, svc).CallTool(context.Background(), &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
				"resource": []string{"web"}, "startTime": start, "endTime": end, "limit": 2}})
			if err != nil || result.IsError {
				t.Fatalf("MCP list_logs = %+v, %v", result, err)
			}
			var viaMCP wirePage
			if err := json.Unmarshal([]byte(result.Content[0].(*mcp.TextContent).Text), &viaMCP); err != nil {
				t.Fatal(err)
			}
			for surface, page := range map[string]wirePage{"GraphQL": viaGraphQL, "MCP": viaMCP} {
				var got []string
				for _, row := range page.Logs {
					got = append(got, row.Message)
				}
				if fmt.Sprint(got) != "[second first]" || page.HasMore {
					t.Errorf("%s page = %v hasMore=%v, want [second first] and no more", surface, got, page.HasMore)
				}
			}
		})
	}
}

// Two resources interleaved in time, one with two instances sharing a
// timestamp: each direction is one global order, and walking it in pages of
// three visits every line once, in that order, with the shared-timestamp
// pair on one page.
func TestLogPagesKeepOneGlobalOrderAcrossResourcesAndInstances(t *testing.T) {
	const start, end = "2026-07-05T00:00:00Z", "2026-07-05T00:01:00Z"
	svc := newService(map[string][]string{
		"web-1": {"2026-07-05T00:00:01Z w1", "2026-07-05T00:00:04Z twin-a", "2026-07-05T00:00:06Z w6"},
		"web-2": {"2026-07-05T00:00:04Z twin-b"},
		"api-1": {"2026-07-05T00:00:02Z a2", "2026-07-05T00:00:03Z a3", "2026-07-05T00:00:05Z a5", "2026-07-05T00:00:07Z a7"},
	}, sampleApp("web"), podFor("web", "web-1"), podFor("web", "web-2"), sampleApp("api"), podFor("api", "api-1"))
	oldestFirst := []string{"w1", "a2", "a3", "twin", "twin", "a5", "w6", "a7"}

	for _, direction := range []string{DirectionBackward, DirectionForward} {
		want := slices.Clone(oldestFirst)
		if direction == DirectionBackward {
			slices.Reverse(want)
		}
		var walked []string
		for _, page := range pageMessages(walkLogPages(t, svc, "resource=web&resource=api", start, end, 3, direction)) {
			twins := 0
			for _, msg := range page {
				if strings.HasPrefix(msg, "twin-") {
					msg, twins = "twin", twins+1
				}
				walked = append(walked, msg)
			}
			if twins == 1 {
				t.Errorf("%s page %v splits the shared-timestamp pair", direction, page)
			}
		}
		if !slices.Equal(walked, want) {
			t.Errorf("%s walk = %v, want %v", direction, walked, want)
		}
	}
}

// hasMore holds at the edges a one-line probe meets: a page of the maximum
// size over one line more than it, and a shared-timestamp group kept whole
// that absorbs the probe line.
func TestLogPagesSayExactlyWhetherMoreRemain(t *testing.T) {
	base := time.Date(2026, 7, 5, 0, 0, 0, 0, time.UTC)
	const start, end = "2026-07-05T00:00:00Z", "2026-07-05T01:00:00Z"

	var lines []LogEntry
	for i := range maxLogLimit + 1 {
		lines = append(lines, lineAt(base.Add(time.Duration(i+1)*time.Second), fmt.Sprintf("l%03d", i)))
	}
	pages := walkLogPages(t, storedService(lines...), "resource=web", start, end, maxLogLimit, "")
	if len(pages) != 2 || len(pages[0].Logs) != maxLogLimit || len(pages[1].Logs) != 1 || pages[1].Logs[0].Message != "l000" {
		t.Fatalf("limit=%d over %d lines = %d pages %v", maxLogLimit, len(lines), len(pages), pageMessages(pages))
	}

	twinA, twinB := lineAt(base.Add(time.Second), "twin-a"), lineAt(base.Add(time.Second), "twin-b")
	twinB.Labels = map[string]string{LabelInstance: "web-2"}
	svc := storedService(twinA, twinB, lineAt(base.Add(time.Minute), "c"))
	for direction, want := range map[string]string{
		DirectionBackward: "[[c twin-b twin-a]]",
		DirectionForward:  "[[twin-a twin-b] [c]]",
	} {
		if got := fmt.Sprint(pageMessages(walkLogPages(t, svc, "resource=web", start, end, 2, direction))); got != want {
			t.Errorf("%s twin walk = %s, want %s", direction, got, want)
		}
	}
}

// A page the scan edge empties resumes at the edge: its own cursors would name
// the window just read, and a client would re-read it forever.
func TestLogPageEmptiedByTheScanEdgeResumesAtTheEdge(t *testing.T) {
	edge := scanEnd.Add(-time.Hour)
	svc := newService(nil, sampleApp("web"), sampleApp("api"))
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		if q.App == "web" {
			return nil, &ScanIncompleteError{ScannedTo: edge}
		}
		return []LogEntry{lineAt(edge.Add(-2*time.Hour), "older"), lineAt(edge.Add(-90*time.Minute), "old")}, nil
	}
	page, err := svc.queryLogPage(context.Background(), []string{"web", "api"},
		LogQuery{Search: []string{"x"}, Since: scanStart, End: scanEnd, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if len(page.Entries) != 0 || !page.HasMore || page.NextEndTime != edge.Add(-time.Nanosecond).Format(time.RFC3339Nano) {
		t.Fatalf("page = %+v, want no lines, more, and the next page ending just before the edge %s", page, edge)
	}
}
