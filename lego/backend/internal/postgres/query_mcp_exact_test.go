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

package postgres

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// mcpQueryWire calls query_render_postgres through svc's real MCP registration
// over the SDK's stateless streamable-HTTP transport and returns the JSON-RPC
// result object raw. Numbers are read from these bytes, never from a decoded
// float64, so the oracle cannot round them itself (w4/m158).
func mcpQueryWire(t *testing.T, svc *Service, args string) map[string]json.RawMessage {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ts := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server { return srv },
		&mcp.StreamableHTTPOptions{Stateless: true}))
	defer ts.Close()
	body := `{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"query_render_postgres","arguments":` + args + `}}`
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
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("tools/call => %d %s (%v)", resp.StatusCode, raw, err)
	}
	_, data, ok := strings.Cut(string(raw), "data: ")
	if !ok {
		t.Fatalf("no SSE data frame: %s", raw)
	}
	var env struct {
		Result map[string]json.RawMessage `json:"result"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(data)), &env); err != nil {
		t.Fatalf("decode %s: %v", data, err)
	}
	return env.Result
}

// mcpQueryText returns the single text block and asserts it is byte-identical
// to structuredContent on success.
func mcpQueryText(t *testing.T, res map[string]json.RawMessage) string {
	t.Helper()
	var content []struct{ Text string }
	if err := json.Unmarshal(res["content"], &content); err != nil || len(content) != 1 {
		t.Fatalf("content %s: %v", res["content"], err)
	}
	if string(res["isError"]) != "true" && content[0].Text != string(res["structuredContent"]) {
		t.Fatalf("text %s differs from structuredContent %s", content[0].Text, res["structuredContent"])
	}
	return content[0].Text
}

func exactNumeric(t *testing.T, s string) pgtype.Numeric {
	t.Helper()
	var n pgtype.Numeric
	if err := n.Scan(s); err != nil {
		t.Fatal(err)
	}
	return n
}

// TestMCPQueryKeepsExactNumbers is the w4/m158 regression for the composed
// tool: the captured live values — 2^53+1, an exact numeric, a signed bigint
// array and a long fraction — reach both MCP result forms with every digit,
// and the small/null/bool/bytea/text controls keep their existing encoding.
// Under the previous typed registration both forms carried 9007199254740992.
func TestMCPQueryKeepsExactNumbers(t *testing.T) {
	svc, _ := newService()
	seedDatabaseAt(t, svc, "exact-db", "postgres://resolved/uri")
	svc.queryExecutor = func(_ context.Context, _, _ string, _ queryLimits, readOnly bool) (QueryResult, error) {
		if !readOnly {
			t.Fatal("MCP query ran outside the read-only envelope")
		}
		return QueryResult{
			Columns: []string{"exact_int", "exact_decimal", "exact_array", "fraction", "small_int", "simple_decimal", "absent", "flag", "binary_control", "exact_text"},
			Rows: [][]any{{
				int64(9007199254740993),
				exactNumeric(t, "12345678901234567890.123456789"),
				[]any{int64(9007199254740993), int64(-9007199254740993)},
				exactNumeric(t, "0.12345678901234567890123456789"),
				int64(42),
				exactNumeric(t, "1.25"),
				nil,
				true,
				[]byte{0x00, 0x01, 0xff},
				"9007199254740993",
			}},
			RowCount: 1,
		}, nil
	}

	res := mcpQueryWire(t, svc, `{"postgresId":"exact-db","sql":"SELECT 1"}`)
	want := `{"columns":["exact_int","exact_decimal","exact_array","fraction","small_int","simple_decimal","absent","flag","binary_control","exact_text"],` +
		`"rows":[[9007199254740993,12345678901234567890.123456789,[9007199254740993,-9007199254740993],0.12345678901234567890123456789,42,1.25,null,true,"AAH/","9007199254740993"]],` +
		`"rowCount":1,"truncated":false}`
	if got := string(res["structuredContent"]); got != want {
		t.Fatalf("structuredContent\n got %s\nwant %s", got, want)
	}
	if got := mcpQueryText(t, res); got != want {
		t.Fatalf("text block\n got %s\nwant %s", got, want)
	}
}

// TestMCPQueryErrorsStayToolErrors: the exact registration keeps the typed
// input check, the scoped not-found / forbidden answers and the safe query
// error mapping — none of them reach the executor or grow structuredContent.
func TestMCPQueryErrorsStayToolErrors(t *testing.T) {
	svc, _ := newService()
	seedDatabaseAt(t, svc, "guarded-db", "postgres://resolved/uri")
	ran := false
	svc.queryExecutor = func(context.Context, string, string, queryLimits, bool) (QueryResult, error) {
		ran = true
		return QueryResult{}, errQueryReadOnly
	}
	toolError := func(args string) string {
		t.Helper()
		res := mcpQueryWire(t, svc, args)
		if string(res["isError"]) != "true" || len(res["structuredContent"]) != 0 {
			t.Fatalf("%s => %v, want a tool error without structuredContent", args, res)
		}
		return mcpQueryText(t, res)
	}

	if got := toolError(`{"postgresId":"guarded-db","sql":7}`); !strings.Contains(got, `validating "arguments"`) || ran {
		t.Fatalf("non-string sql => %q (executor ran=%v)", got, ran)
	}
	if got := toolError(`{"postgresId":"no-such-db","sql":"SELECT 1"}`); !strings.Contains(got, "not found") || ran {
		t.Fatalf("missing db => %q (executor ran=%v)", got, ran)
	}
	svc.Authz = &queryAuthzChecker{allow: false}
	if got := toolError(`{"postgresId":"guarded-db","sql":"SELECT 1"}`); !strings.Contains(got, "forbidden") || ran {
		t.Fatalf("denied db => %q (executor ran=%v)", got, ran)
	}
	svc.Authz = nil
	if got := toolError(`{"postgresId":"guarded-db","sql":"DELETE FROM t"}`); got != core.MCPError(errQueryReadOnly).Error() || !ran {
		t.Fatalf("read-only refusal => %q (executor ran=%v)", got, ran)
	}
}
