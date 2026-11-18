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
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/logs"
)

// TestPinnedCLIScrollRequestPages replays the request the pinned Render CLI's
// log viewer sends to load an older page, byte for byte as its own client
// builds it (lego/cli/logs_scroll_contract_test.go records it from
// logs.PaginationLogLimit, so a pin bump that changes the page size moves the
// golden and this replay with it). It runs through the composed handler,
// Render contract gate included, which refused this exact limit=1000 with a
// 400 until w3/m475. The answer is a Render log page of at most 100 lines whose
// cursors the viewer can follow to the oldest line.
func TestPinnedCLIScrollRequestPages(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "cli", "testdata", "logs-scroll-request.json"))
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		PaginationLogLimit int    `json:"paginationLogLimit"`
		Path               string `json:"path"`
		Query              string `json:"query"`
	}
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}
	query, err := url.ParseQuery(golden.Query)
	if err != nil {
		t.Fatal(err)
	}
	if query.Get("limit") != fmt.Sprint(golden.PaginationLogLimit) {
		t.Fatalf("the golden's limit %q is not the CLI's PaginationLogLimit %d", query.Get("limit"), golden.PaginationLogLimit)
	}
	resource := query.Get("resource")
	end, err := time.Parse(time.RFC3339Nano, query.Get("endTime"))
	if err != nil {
		t.Fatal(err)
	}

	// 250 lines, one a second, up to the scroll window's end.
	const total = 250
	var lines []logs.LogEntry
	for i := range total {
		lines = append(lines, logs.LogEntry{
			Timestamp: end.Add(time.Duration(i-total+1) * time.Second).Format(time.RFC3339Nano),
			Message:   fmt.Sprintf("l%03d", i),
			Labels:    map[string]string{logs.LabelInstance: resource + "-abcde"},
		})
	}
	var largest int64
	h, _ := serverWith(t, &core.Base{Client: fakeClient(sampleApp(resource)), Namespace: "default"}, Deps{
		// The durable store: the window's lines oldest-first, the newest
		// q.Limit of them for a backward read.
		LogHistory: func(_ context.Context, _ string, q logs.LogQuery) ([]logs.LogEntry, error) {
			largest = max(largest, q.Limit)
			var in []logs.LogEntry
			for _, line := range lines {
				at, _ := time.Parse(time.RFC3339Nano, line.Timestamp)
				if !at.Before(q.Since) && !at.After(q.End) {
					in = append(in, line)
				}
			}
			return in[max(0, len(in)-int(q.Limit)):], nil
		},
		LogLabelValues: func(context.Context, string, string, logs.LogQuery) ([]string, error) { return nil, nil },
	})

	var sizes []int
	target := golden.Path + "?" + golden.Query // the first scroll exactly as the CLI sends it
	for page := 0; ; page++ {
		rec := do(t, h, http.MethodGet, target, testToken, "")
		if rec.Code != http.StatusOK {
			t.Fatalf("page %d of the pinned CLI's scroll = %d %s", page, rec.Code, rec.Body)
		}
		var list struct {
			HasMore       bool   `json:"hasMore"`
			NextStartTime string `json:"nextStartTime"`
			NextEndTime   string `json:"nextEndTime"`
			Logs          []struct {
				Message string `json:"message"`
			} `json:"logs"`
		}
		if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
			t.Fatalf("page %d does not decode as a Render log list: %v (%s)", page, err, rec.Body)
		}
		sizes = append(sizes, len(list.Logs))
		if page == 0 && (len(list.Logs) == 0 || list.Logs[0].Message != "l249") {
			t.Errorf("the first page does not start at the newest line: %s", rec.Body)
		}
		if !list.HasMore {
			if len(list.Logs) == 0 || list.Logs[len(list.Logs)-1].Message != "l000" {
				t.Errorf("the walk does not end at the oldest line l000: %s", rec.Body)
			}
			break
		}
		if page == 5 {
			t.Fatalf("still paging after %v", sizes)
		}
		// The viewer feeds the cursors back with the same limit.
		query.Set("startTime", list.NextStartTime)
		query.Set("endTime", list.NextEndTime)
		target = golden.Path + "?" + query.Encode()
	}
	if fmt.Sprint(sizes) != "[100 100 50]" {
		t.Errorf("limit=%d over %d lines = pages %v, want [100 100 50]", golden.PaginationLogLimit, total, sizes)
	}
	if largest > 101 {
		t.Errorf("the store was asked for %d lines; a page reads at most 100 and one past it", largest)
	}
}
