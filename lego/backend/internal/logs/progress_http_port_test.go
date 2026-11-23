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
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

func rendersDetectedPortLine(port int32) string {
	return fmt.Sprintf("\x1b[0;32m\x1b[1m==> \x1b[0m\x1b[1mDetected service running on port %d\x1b[0m", port)
}

func TestHTTPPortNarrationUsesTheClosedDeployRecord(t *testing.T) {
	finished := time.Date(2026, 10, 28, 16, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name, status string
		port         int32
		finished     time.Time
		want         bool
	}{
		{"live detected", "live", 15672, finished, true},
		{"historical detected", "deactivated", 80, finished, true},
		{"older unrecorded deploy", "deactivated", 0, finished, false},
		{"failed candidate", "update_failed", 15672, finished, false},
		{"pending candidate", "update_in_progress", 15672, time.Time{}, false},
		{"invalid port", "live", 65536, finished, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := DeployProgress{ID: "dep-record", Status: tc.status, CreatedAt: finished.Add(-time.Minute), FinishedAt: tc.finished, DetectedHTTPPort: tc.port}
			entries := progressLines(row, progressContext{})
			wantDetected := rendersDetectedPortLine(tc.port)
			found := -1
			for i, entry := range entries {
				if entry.Message != wantDetected && !strings.Contains(entry.Message, "Detected service running on port") && !strings.Contains(entry.Message, "Detected HTTP") {
					continue
				}
				found = i
				if entry.Message != wantDetected {
					t.Fatalf("detected port message = %q, want Render's %q", entry.Message, wantDetected)
				}
				if entry.Timestamp != finished.Format(time.RFC3339Nano) || entry.Labels[LabelInstance] != row.ID || entry.Labels[LabelType] != LogTypeBuild || entry.Labels["container"] != progressContainer {
					t.Fatalf("wrong narration identity: %+v", entry)
				}
			}
			if (found >= 0) != tc.want {
				t.Fatalf("narration=%+v, wantedHTTP=%v", entries, tc.want)
			}
			if !tc.want {
				for _, entry := range entries {
					if entry.Message == rendersDetectedPortDocs {
						t.Fatalf("unexpected green port docs line: %+v", entries)
					}
				}
				return
			}
			if found+2 >= len(entries) || entries[found+1].Message != rendersDetectedPortDocs || entries[found+2].Message != rendersLive {
				t.Fatalf("detected port + green docs must immediately precede live: %+v", entries)
			}
			docs := entries[found+1]
			if docs.Timestamp != finished.Format(time.RFC3339Nano) || docs.Labels[LabelInstance] != row.ID || docs.Labels[LabelType] != LogTypeBuild || docs.Labels["container"] != progressContainer {
				t.Fatalf("wrong docs narration identity: %+v", docs)
			}
		})
	}
}

func TestHTTPPortProgressFollowerEmitsOnce(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	at := time.Date(2026, 10, 28, 16, 0, 0, 0, time.UTC)
	row := DeployProgress{ID: "dep-http", Status: "live", CreatedAt: at.Add(-time.Minute), FinishedAt: at, DetectedHTTPPort: 15672}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) { return []DeployProgress{row}, nil }
	follower := svc.newProgressFollower(LogQuery{App: "web", Types: []string{LogTypeBuild}}, "web", progressContext{})
	var entries []LogEntry
	for range 3 {
		if err := follower.emitReached(t.Context(), func(e LogEntry) error { entries = append(entries, e); return nil }); err != nil {
			t.Fatal(err)
		}
	}
	wantDetected := rendersDetectedPortLine(15672)
	var detectedCount, docsCount int
	for _, entry := range entries {
		switch entry.Message {
		case wantDetected:
			detectedCount++
		case rendersDetectedPortDocs:
			docsCount++
		}
	}
	if detectedCount != 1 || docsCount != 1 {
		t.Fatalf("detected port emitted (%d, %d) times: %+v", detectedCount, docsCount, entries)
	}
}

// w32/099: REST /v1/logs, SSE and WebSocket /v1/logs/subscribe, GraphQL logs
// and MCP list_logs all return Render's two green port-detection lines in exact
// byte order before `==> Your service is live 🎉`, with identical IDs.
func TestDetectedPortLinesReadTheSameAcrossSurfaces(t *testing.T) {
	app := sampleApp("web")
	app.Spec.Repo = rolloutRepo
	opened := time.Now()
	soon := func(d time.Duration) time.Time { return opened.Add(d).UTC() }
	created, started, finished := soon(30*time.Minute), soon(31*time.Minute), soon(72*time.Minute)
	wantTriple := []string{rendersDetectedPortLine(8000), rendersDetectedPortDocs, rendersLive}

	svc := newService(map[string][]string{
		"bld-web-gen-4-a": {soon(time.Hour).Format(time.RFC3339Nano) + " ==> Build successful 🎉"},
	}, app)
	svc.BuildNamespace = "builds"
	svc.BuildPodWaitInterval = 10 * time.Millisecond
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) { return nil, nil }
	var mu sync.Mutex
	row := DeployProgress{ID: "dep-port", Generation: 4, Status: "live", Built: true, Commit: "abc1234def5678",
		DetectedHTTPPort: 8000, CreatedAt: created, StartedAt: started, FinishedAt: finished}
	svc.DeployProgress = func(context.Context, string, time.Time) ([]DeployProgress, error) {
		mu.Lock()
		defer mu.Unlock()
		return []DeployProgress{row}, nil
	}

	start, end := finished.Add(-time.Second).Format(time.RFC3339), finished.Add(time.Second).Format(time.RFC3339)
	extractTriple := func(surface string, messages, ids []string) []string {
		t.Helper()
		if di, li := slices.Index(messages, wantTriple[0]), slices.Index(messages, rendersLive); di >= 0 && li >= 0 && di > li {
			slices.Reverse(messages)
			slices.Reverse(ids)
		}
		at := slices.Index(messages, wantTriple[0])
		if at < 0 || at+len(wantTriple) > len(messages) || !slices.Equal(messages[at:at+len(wantTriple)], wantTriple) {
			t.Fatalf("%s reads %q, want %q in order", surface, messages, wantTriple)
		}
		return ids[at : at+len(wantTriple)]
	}

	// REST GET /v1/logs
	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type=build&startTime="+start+"&endTime="+end)
	var env renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &env); err != nil || rec.Code != http.StatusOK {
		t.Fatalf("REST = %d %s (%v)", rec.Code, rec.Body, err)
	}
	var messages, ids []string
	for _, l := range env.Logs {
		messages, ids = append(messages, l.Message), append(ids, l.ID)
	}
	restIDs := extractTriple("REST", messages, ids)

	// REST display-mode search matches the plain words Render shows.
	phrase := url.QueryEscape("==> Detected service running on port 8000")
	recSearch := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&type=build&startTime="+start+"&endTime="+end+"&textMode=display&text="+phrase)
	var searchEnv renderLogList
	if err := json.Unmarshal(recSearch.Body.Bytes(), &searchEnv); err != nil || recSearch.Code != http.StatusOK {
		t.Fatalf("REST search = %d %s (%v)", recSearch.Code, recSearch.Body, err)
	}
	if len(searchEnv.Logs) != 1 || searchEnv.Logs[0].Message != wantTriple[0] || searchEnv.Logs[0].ID != restIDs[0] {
		t.Fatalf("REST display search = %+v, want [%q / %s]", searchEnv.Logs, wantTriple[0], restIDs[0])
	}

	// GraphQL logs
	schema, err := gqlSchema(svc)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	data := runQuery(t, schema, `{ logs(resource:"web", type:"build", startTime:"`+start+`", endTime:"`+end+`") { logs { id message } } }`)
	messages, ids = nil, nil
	for _, r := range data["logs"].(map[string]any)["logs"].([]any) {
		m := r.(map[string]any)
		messages, ids = append(messages, m["message"].(string)), append(ids, m["id"].(string))
	}
	if gqlIDs := extractTriple("GraphQL", messages, ids); !slices.Equal(gqlIDs, restIDs) {
		t.Fatalf("GraphQL ids %q, REST %q", gqlIDs, restIDs)
	}

	// MCP list_logs
	mcpSrv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(mcpSrv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := mcpSrv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "type": []string{LogTypeBuild}, "startTime": start, "endTime": end,
	}})
	if err != nil || res.IsError {
		t.Fatalf("list_logs: %v %+v", err, res)
	}
	var mcpGot renderLogList
	rendertest.DecodeMCPText(t, res, &mcpGot)
	messages, ids = nil, nil
	for _, l := range mcpGot.Logs {
		messages, ids = append(messages, l.Message), append(ids, l.ID)
	}
	if mcpIDs := extractTriple("MCP", messages, ids); !slices.Equal(mcpIDs, restIDs) {
		t.Fatalf("MCP ids %q, REST %q", mcpIDs, restIDs)
	}

	// SSE GET /v1/logs/subscribe
	if err := svc.Client.Create(ctx, buildPodFor("web", "bld-web-gen-4-a", "builds", "buildkit", opened)); err != nil {
		t.Fatal(err)
	}
	httpSrv := revalidationTestServer(svc)
	defer httpSrv.Close()
	sseReq, err := http.NewRequestWithContext(ctx, http.MethodGet, httpSrv.URL+"/v1/logs/subscribe?resource=web&type=build", nil)
	if err != nil {
		t.Fatal(err)
	}
	sseReq.Header.Set("Accept", "text/event-stream")
	sseResp, err := http.DefaultClient.Do(sseReq)
	if err != nil {
		t.Fatalf("SSE get: %v", err)
	}
	defer sseResp.Body.Close()
	if sseResp.StatusCode != http.StatusOK || sseResp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("SSE = %d %q", sseResp.StatusCode, sseResp.Header.Get("Content-Type"))
	}
	messages, ids = nil, nil
	scanner := bufio.NewScanner(sseResp.Body)
	for scanner.Scan() {
		line, ok := strings.CutPrefix(scanner.Text(), "data: ")
		if !ok {
			continue
		}
		var item renderLog
		if err := json.Unmarshal([]byte(line), &item); err != nil || item.ID == "" {
			continue
		}
		messages, ids = append(messages, item.Message), append(ids, item.ID)
	}
	if sseIDs := extractTriple("SSE", messages, ids); !slices.Equal(sseIDs, restIDs) {
		t.Fatalf("SSE ids %q, REST %q", sseIDs, restIDs)
	}

	// WebSocket GET /v1/logs/subscribe (CLI --tail)
	mu.Lock()
	row.Status, row.FinishedAt, row.DetectedHTTPPort = deployStatusBuildInProgress, time.Time{}, 0
	mu.Unlock()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(httpSrv.URL, "http")+"/v1/logs/subscribe?resource=web&type=build", nil)
	if err != nil {
		t.Fatalf("WS dial: %v", err)
	}
	defer conn.Close()
	messages, ids = nil, nil
	readWS := func(until func() bool, wait time.Duration) {
		t.Helper()
		_ = conn.SetReadDeadline(time.Now().Add(wait))
		for !until() {
			_, payload, err := conn.ReadMessage()
			if err != nil {
				if wait < time.Second {
					return
				}
				t.Fatalf("WS waiting after %q: %v", messages, err)
			}
			var item renderLog
			if err := json.Unmarshal(payload, &item); err != nil {
				t.Fatalf("WS frame %s: %v", payload, err)
			}
			messages, ids = append(messages, item.Message), append(ids, item.ID)
		}
	}
	readWS(func() bool { return slices.Contains(messages, "==> Build successful 🎉") }, 5*time.Second)
	mu.Lock()
	row.Status, row.FinishedAt, row.DetectedHTTPPort = "live", finished, 8000
	mu.Unlock()
	readWS(func() bool { return slices.Contains(messages, rendersLive) }, 5*time.Second)
	readWS(func() bool { return false }, 100*time.Millisecond)
	if wsIDs := extractTriple("WebSocket", messages, ids); !slices.Equal(wsIDs, restIDs) {
		t.Fatalf("WebSocket ids %q, REST %q", wsIDs, restIDs)
	}
}
