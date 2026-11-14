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
	"reflect"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
)

// TestMCPListLogsIsRESTsBody pins w4/m378 t002: list_logs answers the REST
// logs envelope — every line with an id and labels as the ordered
// [{name, value}] array keyed resource, never bex's internal map keyed service
// — equal to GET /v1/logs for the same window. list_log_label_values answers
// Render's bare string array.
func TestMCPListLogsIsRESTsBody(t *testing.T) {
	svc := newService(nil, sampleApp("web"))
	svc.PodLogs, svc.PodLogsFollow = nil, nil
	svc.History = func(context.Context, string, LogQuery) ([]LogEntry, error) {
		return []LogEntry{
			{Timestamp: "2026-07-05T00:00:02Z", Message: "second", Labels: map[string]string{
				"service": "web", LabelInstance: "web-abc", LabelType: LogTypeApp, LabelLevel: "info", "container": "app",
			}},
			{Timestamp: "2026-07-05T00:00:01Z", Message: "first", Labels: map[string]string{
				"service": "web", LabelInstance: "web-abc", LabelType: LogTypeApp, LabelLevel: "error",
			}},
		}, nil
	}
	const window = "startTime=2026-07-05T00:00:00Z&endTime=2026-07-05T01:00:00Z"
	rec := serveREST(svc, http.MethodGet, "/v1/logs?resource=web&"+window)
	if rec.Code != http.StatusOK {
		t.Fatalf("REST = %d %s", rec.Code, rec.Body)
	}
	var rest renderLogList
	if err := json.Unmarshal(rec.Body.Bytes(), &rest); err != nil || len(rest.Logs) != 2 {
		t.Fatalf("REST body = %s (%v)", rec.Body, err)
	}

	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	serverT, clientT := mcp.NewInMemoryTransports()
	ctx := context.Background()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_logs", Arguments: map[string]any{
		"resource": []string{"web"}, "startTime": "2026-07-05T00:00:00Z", "endTime": "2026-07-05T01:00:00Z",
	}})
	if err != nil || res.IsError {
		t.Fatalf("list_logs: %v %+v", err, res)
	}
	rendertest.AssertRenderMCPResult(t, "list_logs", res)
	var got renderLogList
	rendertest.DecodeMCPText(t, res, &got)
	if !reflect.DeepEqual(got, rest) {
		t.Errorf("MCP list_logs = %+v,\nwant REST's %+v", got, rest)
	}
	if text := rendertest.MCPText(t, res); strings.Contains(text, `"service"`) || strings.Contains(text, "container") || !strings.Contains(text, `{"name":"resource","value":"web"}`) {
		t.Errorf("MCP list_logs text = %s, want Render's resource label and no service/container", text)
	}
	if got.Logs[0].ID == "" {
		t.Error("MCP list_logs lines carry no id")
	}

	svc.LabelValues = func(context.Context, string, string, LogQuery) ([]string, error) {
		return []string{"info", "error"}, nil
	}
	values, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "list_log_label_values", Arguments: map[string]any{
		"resource": []string{"web"}, "label": "level", "startTime": "2026-07-05T00:00:00Z", "endTime": "2026-07-05T01:00:00Z",
	}})
	if err != nil || values.IsError {
		t.Fatalf("list_log_label_values: %v %+v", err, values)
	}
	rendertest.AssertRenderMCPResult(t, "list_log_label_values", values)
	if text := rendertest.MCPText(t, values); text != `["error","info"]` {
		t.Errorf("list_log_label_values = %s, want Render's sorted bare array", text)
	}
	if values.StructuredContent != nil {
		t.Errorf("list_log_label_values structuredContent = %v, want text only", values.StructuredContent)
	}
}
