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
	"net/http"
	"net/url"
	"slices"
	"testing"
	"time"
)

func TestTextUnionPagesAcrossResourcesWithoutLosingBoundarySiblings(t *testing.T) {
	svc := newService(map[string][]string{
		"web-1": {"2026-07-05T00:00:01Z A first", "2026-07-05T00:00:02Z B web", "2026-07-05T00:00:04Z control", "2026-07-05T00:00:04Z A B both"},
		"api-1": {"2026-07-05T00:00:02Z A api", "2026-07-05T00:00:03Z B last"},
	}, sampleApp("web"), podFor("web", "web-1"), sampleApp("api"), podFor("api", "api-1"))
	for _, direction := range []string{DirectionForward, DirectionBackward} {
		for _, terms := range [][]string{{"A ", "B "}, {"B ", "A "}} {
			t.Run(direction+terms[0], func(t *testing.T) {
				params := url.Values{
					"resource": {"web", "api"}, "text": terms, "limit": {"1"}, "direction": {direction},
					"startTime": {"2026-07-05T00:00:00Z"}, "endTime": {"2026-07-05T00:00:05Z"},
				}
				var messages []string
				for pages := 0; ; pages++ {
					if pages > 4 {
						t.Fatal("cursor never reached the end of the filtered window")
					}
					page := decodeLogList(t, serveREST(svc, http.MethodGet, "/v1/logs?"+params.Encode()))
					for _, line := range page.Logs {
						messages = append(messages, line.Message)
					}
					if !page.HasMore {
						break
					}
					params.Set("startTime", page.NextStartTime)
					params.Set("endTime", page.NextEndTime)
				}
				slices.Sort(messages)
				if want := []string{"A B both", "A api", "A first", "B last", "B web"}; !slices.Equal(messages, want) {
					t.Fatalf("paged union = %q, want %q exactly once", messages, want)
				}
			})
		}
	}
}

func TestTextUnionSurvivesPartialHistoryContinuation(t *testing.T) {
	edge := scanEnd.Add(-time.Hour)
	svc := newService(nil, sampleApp("web"))
	svc.History = func(_ context.Context, _ string, q LogQuery) ([]LogEntry, error) {
		if !slices.Equal(q.Search, []string{"a", "b"}) {
			t.Fatalf("history terms = %q; continuation must retain the complete normalized set", q.Search)
		}
		if q.End.After(edge) {
			return nil, &ScanIncompleteError{Entries: []LogEntry{lineAt(scanEnd.Add(-time.Minute), "A newest")}, ScannedTo: edge}
		}
		return []LogEntry{lineAt(edge.Add(-time.Minute), "B older")}, nil
	}
	params := url.Values{"resource": {"web"}, "text": {"B", "", "A", "b"}, "startTime": {scanStart.Format(time.RFC3339Nano)}, "endTime": {scanEnd.Format(time.RFC3339Nano)}}
	var messages []string
	for i := 0; i < 2; i++ {
		page := decodeLogList(t, serveREST(svc, http.MethodGet, "/v1/logs?"+params.Encode()))
		if page.HasMore != (i == 0) {
			t.Fatalf("page %d hasMore = %v", i, page.HasMore)
		}
		if i == 0 && page.NextEndTime != edge.Add(-time.Nanosecond).Format(time.RFC3339Nano) {
			t.Fatalf("cursor skipped the uncovered window: %+v", page)
		}
		for _, line := range page.Logs {
			messages = append(messages, line.Message)
		}
		params.Set("startTime", page.NextStartTime)
		params.Set("endTime", page.NextEndTime)
	}
	if !slices.Equal(messages, []string{"A newest", "B older"}) {
		t.Fatalf("partial scan union = %q", messages)
	}
}

func TestGraphQLTextRemainsOneLiteralSubstring(t *testing.T) {
	svc := newService(map[string][]string{"web-1": {
		"2026-07-05T00:00:01Z A,B literal", "2026-07-05T00:00:02Z A alone", "2026-07-05T00:00:03Z B alone",
	}}, sampleApp("web"), podFor("web", "web-1"))
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatal(err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", text:"a,b") { logs { message } } }`)
	raw, err := json.Marshal(data["logs"])
	if err != nil {
		t.Fatal(err)
	}
	var page wirePage
	if err := json.Unmarshal(raw, &page); err != nil {
		t.Fatal(err)
	}
	if len(page.Logs) != 1 || page.Logs[0].Message != "A,B literal" {
		t.Fatalf("scalar text unexpectedly split into OR terms: %+v", page)
	}
}
