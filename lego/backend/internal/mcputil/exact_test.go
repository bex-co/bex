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
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
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

func exactServer(handler func(context.Context, *mcp.CallToolRequest, exactIn) (exactOut, error)) *mcp.Server {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0.0.1"}, nil)
	mcputil.AddExactJSONTool(srv, &mcp.Tool{Name: "exact_tool", Description: "exact"}, handler)
	mcputil.AddTool(srv, &mcp.Tool{Name: "typed_tool", Description: "typed"},
		func(context.Context, *mcp.CallToolRequest, exactIn) (*mcp.CallToolResult, exactOut, error) {
			return nil, exactValue, nil
		})
	return srv
}

func returnExact(context.Context, *mcp.CallToolRequest, exactIn) (exactOut, error) {
	return exactValue, nil
}

// TestAddExactJSONToolKeepsEveryDigit is the w4/m158 regression: both the
// structuredContent and the text block carry the once-marshaled bytes. The
// typed twin pins the SDK premise; if it ever stops rounding, the exact seam
// could be retired.
func TestAddExactJSONToolKeepsEveryDigit(t *testing.T) {
	srv := exactServer(returnExact)

	res := wireResult(t, callRaw(t, srv, "exact_tool", `{"sql":"SELECT"}`))
	if !strings.Contains(string(res["structuredContent"]), `"rows":`+exactWireRow) {
		t.Fatalf("structuredContent lost digits: %s", res["structuredContent"])
	}
	var content []struct {
		Type, Text string
	}
	if err := json.Unmarshal(res["content"], &content); err != nil {
		t.Fatal(err)
	}
	if len(content) != 1 || content[0].Type != "text" || content[0].Text != string(res["structuredContent"]) {
		t.Fatalf("text block %+v differs from structuredContent %s", content, res["structuredContent"])
	}
	if _, isErr := res["isError"]; isErr {
		t.Fatalf("exact result flagged as an error: %v", res)
	}

	typed := wireResult(t, callRaw(t, srv, "typed_tool", `{"sql":"SELECT"}`))
	if strings.Contains(string(typed["structuredContent"]), "9007199254740993") {
		t.Fatal("typed SDK output kept 2^53+1; the float64 re-encode premise no longer holds")
	}
}

// TestAddExactJSONToolAdvertisesTheTypedSchema: the opt-in must not change
// what tools/list tells a client — same output schema a typed registration of
// the same Out derives, and the same validated input schema.
func TestAddExactJSONToolAdvertisesTheTypedSchema(t *testing.T) {
	raw := postMCP(t, exactServer(returnExact), `{"jsonrpc":"2.0","id":1,"method":"tools/list"}`)
	var tools struct {
		Tools []struct {
			Name         string          `json:"name"`
			InputSchema  json.RawMessage `json:"inputSchema"`
			OutputSchema json.RawMessage `json:"outputSchema"`
		} `json:"tools"`
	}
	_, data, _ := strings.Cut(raw, "data: ")
	var env struct {
		Result json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &env); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(env.Result, &tools); err != nil {
		t.Fatal(err)
	}
	byName := map[string]int{}
	for i, tl := range tools.Tools {
		byName[tl.Name] = i
	}
	exact, typed := tools.Tools[byName["exact_tool"]], tools.Tools[byName["typed_tool"]]
	if len(exact.OutputSchema) == 0 || string(exact.OutputSchema) != string(typed.OutputSchema) {
		t.Fatalf("output schema drifted:\nexact %s\ntyped %s", exact.OutputSchema, typed.OutputSchema)
	}
	if string(exact.InputSchema) != string(typed.InputSchema) {
		t.Fatalf("input schema drifted:\nexact %s\ntyped %s", exact.InputSchema, typed.InputSchema)
	}
	for _, field := range []string{`"columns"`, `"rows"`, `"count"`} {
		if !strings.Contains(string(exact.OutputSchema), field) {
			t.Errorf("output schema missing %s: %s", field, exact.OutputSchema)
		}
	}
}

// TestAddExactJSONToolKeepsTheAddToolContract: typed input validation, coded
// and redacted errors, and panic isolation all still apply on the exact path.
func TestAddExactJSONToolKeepsTheAddToolContract(t *testing.T) {
	var fail error
	srv := exactServer(func(context.Context, *mcp.CallToolRequest, exactIn) (exactOut, error) {
		if fail != nil {
			return exactOut{}, fail
		}
		panic(`boom: host=10.0.0.5`)
	})
	errorText := func(res map[string]json.RawMessage) string {
		t.Helper()
		if string(res["isError"]) != "true" {
			t.Fatalf("want an error result, got %v", res)
		}
		if _, ok := res["structuredContent"]; ok {
			t.Fatalf("error result carried structuredContent: %s", res["structuredContent"])
		}
		var content []struct{ Text string }
		if err := json.Unmarshal(res["content"], &content); err != nil || len(content) != 1 {
			t.Fatalf("error content %s: %v", res["content"], err)
		}
		return content[0].Text
	}

	if got := errorText(wireResult(t, callRaw(t, srv, "exact_tool", `{"sql":12}`))); !strings.Contains(got, `validating "arguments"`) {
		t.Errorf("non-string sql => %q, want an input validation error", got)
	}
	if got := errorText(wireResult(t, callRaw(t, srv, "exact_tool", `{"sql":"SELECT"}`))); got != "internal error" {
		t.Errorf("panic => %q, want redacted internal error", got)
	}
	fail = core.NewBadRequestError("PAYMENT_REQUIRED", "a payment method is required", nil)
	if got := errorText(wireResult(t, callRaw(t, srv, "exact_tool", `{"sql":"SELECT"}`))); !strings.Contains(got, "PAYMENT_REQUIRED") {
		t.Errorf("coded error => %q, want its code", got)
	}
	fail = errors.New(`pq: constraint "x" host=10.0.0.5`)
	if got := errorText(wireResult(t, callRaw(t, srv, "exact_tool", `{"sql":"SELECT"}`))); got != "internal error" {
		t.Errorf("unclassified error => %q, want redacted internal error", got)
	}
	fail = core.ErrNotFound
	if got := errorText(wireResult(t, callRaw(t, srv, "exact_tool", `{"sql":"SELECT"}`))); !strings.Contains(got, "not found") {
		t.Errorf("not found => %q", got)
	}
}
