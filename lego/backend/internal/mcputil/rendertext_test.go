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

package mcputil_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/mcputil"
)

type exactIn struct {
	SQL string `json:"sql"`
}

type exactOut struct {
	Columns []string `json:"columns"`
	Rows    [][]any  `json:"rows"`
	Count   int      `json:"count"`
}

// exactValue carries the numbers float64 cannot hold: |n| > 2^53 and a decimal
// with more significant digits than a double. json.Number stands in for a type
// with exact custom JSON (pgtype.Numeric) without importing the driver here.
var exactValue = exactOut{
	Columns: []string{"big", "decimal", "nested", "small", "absent"},
	Rows: [][]any{{
		int64(9007199254740993),
		json.Number("12345678901234567890.123456789"),
		[]any{int64(9007199254740993), int64(-9007199254740993), json.Number("0.12345678901234567890123456789")},
		42,
		nil,
	}},
	Count: 1,
}

// exactWireRow is the row exactly as encoding/json writes exactValue.
const exactWireRow = `[[9007199254740993,12345678901234567890.123456789,[9007199254740993,-9007199254740993,0.12345678901234567890123456789],42,null]]`

// postMCP drives the SDK's streamable-HTTP transport (stateless, as bex-api
// mounts it) and returns the raw response body: the oracle is the wire bytes,
// never a client-side decode that would round the numbers itself.
func postMCP(t *testing.T, srv *mcp.Server, body string) string {
	t.Helper()
	ts := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true}))
	defer ts.Close()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodPost, ts.URL, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("POST %s => %d: %s", body, resp.StatusCode, raw)
	}
	return string(raw)
}

func callRaw(t *testing.T, srv *mcp.Server, name, args string) string {
	t.Helper()
	return postMCP(t, srv, `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"`+name+`","arguments":`+args+`}}`)
}

// wireResult extracts the JSON-RPC result object from the SSE frame while
// keeping it raw.
func wireResult(t *testing.T, raw string) map[string]json.RawMessage {
	t.Helper()
	_, data, ok := strings.Cut(raw, "data: ")
	if !ok {
		t.Fatalf("no SSE data frame in %q", raw)
	}
	var env struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &env); err != nil {
		t.Fatalf("decode envelope %q: %v", data, err)
	}
	return env.Result
}

func exactServer() *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcputil.AddTool(srv, &mcp.Tool{Name: "render_text", Description: "Render text"},
		func(context.Context, *mcp.CallToolRequest, exactIn) (*mcp.CallToolResult, any, error) {
			return mcputil.RenderText(exactValue.Rows, "a notice")
		})
	mcputil.AddTool(srv, &mcp.Tool{Name: "render_list", Description: "Render list"},
		func(_ context.Context, _ *mcp.CallToolRequest, in exactIn) (*mcp.CallToolResult, any, error) {
			if in.SQL == "empty" {
				return mcputil.RenderListWithCursor([]int{}, "")
			}
			return mcputil.RenderListWithCursor([]int{1, 2}, "dep-2")
		})
	mcputil.AddTool(srv, &mcp.Tool{Name: "typed_tool", Description: "typed"},
		func(context.Context, *mcp.CallToolRequest, exactIn) (*mcp.CallToolResult, exactOut, error) {
			return nil, exactValue, nil
		})
	return srv
}

func wireTexts(t *testing.T, res map[string]json.RawMessage) []string {
	t.Helper()
	var content []struct {
		Type, Text string
	}
	if err := json.Unmarshal(res["content"], &content); err != nil {
		t.Fatal(err)
	}
	var texts []string
	for _, c := range content {
		if c.Type != "text" {
			t.Fatalf("content %+v, want text only", content)
		}
		texts = append(texts, c.Text)
	}
	return texts
}

// TestRenderTextKeepsEveryDigit: RenderText answers as Render's server does —
// the once-marshaled JSON as the first text item, any notice after it, and no
// structuredContent — so a number float64 cannot hold keeps every digit
// (w4/m158). The typed twin pins the SDK premise that a typed output would
// round it.
func TestRenderTextKeepsEveryDigit(t *testing.T) {
	srv := exactServer()
	res := wireResult(t, callRaw(t, srv, "render_text", `{"sql":"SELECT"}`))
	if texts := wireTexts(t, res); len(texts) != 2 || texts[0] != exactWireRow || texts[1] != "a notice" {
		t.Fatalf("texts = %q, want the exact rows then the notice", texts)
	}
	if _, ok := res["structuredContent"]; ok {
		t.Fatalf("structuredContent = %s, want none", res["structuredContent"])
	}
	typed := wireResult(t, callRaw(t, srv, "typed_tool", `{"sql":"SELECT"}`))
	if strings.Contains(string(typed["structuredContent"]), "9007199254740993") {
		t.Fatal("typed SDK output kept 2^53+1; the float64 re-encode premise no longer holds")
	}
}

// TestRenderListWithCursorIsRendersText: the bare array, then
// "\n\n cursor: " and the cursor, or a quoted empty string on an empty page
// (render-mcp-server pkg/deploy/tools.go).
func TestRenderListWithCursorIsRendersText(t *testing.T) {
	srv := exactServer()
	if got := wireTexts(t, wireResult(t, callRaw(t, srv, "render_list", `{"sql":"page"}`))); len(got) != 1 || got[0] != "[1,2]\n\n cursor: dep-2" {
		t.Errorf("page = %q", got)
	}
	if got := wireTexts(t, wireResult(t, callRaw(t, srv, "render_list", `{"sql":"empty"}`))); len(got) != 1 || got[0] != "[]\n\n cursor: \"\"" {
		t.Errorf("empty page = %q", got)
	}
}
