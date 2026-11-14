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
	"errors"
	"net/http"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/bex-co/bex/lego/backend/internal/rendertest"
	"github.com/bex-co/bex/lego/backend/internal/resourcemeta"
)

// pgMCPCall connects an in-memory MCP client to svc as user-a and returns a
// caller that fails the test on any tool error.
func pgMCPCall(t *testing.T, svc *Service) func(name string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := ctxAs("user-a")
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cs.Close() })
	return func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		if err != nil || res.IsError {
			t.Fatalf("%s: err=%v result=%+v", name, err, res)
		}
		return res
	}
}

// TestMCPPostgresToolsAnswerRESTsObject pins w4/m378 t010: get_postgres,
// create_postgres and every list_postgres_instances item are REST's
// Render-shaped object, owner included and equal to the REST body for the
// same database. An empty workspace answers Render's sentence.
func TestMCPPostgresToolsAnswerRESTsObject(t *testing.T) {
	svc, _ := newService()
	svc.Workspace = fakeWorkspace{"user-a": "tea-a"}
	svc.Owners = fixedOwnerResolver{"tea-a": resourcemeta.Owner{ID: "tea-a", Name: "Acme", Email: "a@acme.test", Type: "team"}}
	svc.Metadata = resourcemeta.Config{Region: "fsn1", DashboardBaseURL: "https://dashboard.bex.co"}
	mcpCall := pgMCPCall(t, svc)
	call := func(name string, args map[string]any) *mcp.CallToolResult {
		t.Helper()
		res := mcpCall(name, args)
		rendertest.AssertRenderMCPResult(t, name, res, "createdAt") // the fake API server mints no creationTimestamp
		return res
	}

	if got := rendertest.MCPText(t, call("list_postgres_instances", nil)); got != "No Postgres instances found" {
		t.Errorf("empty list_postgres_instances = %q, want Render's sentence", got)
	}

	var created map[string]any
	rendertest.DecodeMCPText(t, call("create_postgres", map[string]any{"name": "pg-mcp", "plan": "free", "version": "17"}), &created)
	id, _ := created["id"].(string)

	var restGet map[string]any
	if rec := serveREST(svc, http.MethodGet, "/v1/postgres/"+id, ""); rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &restGet) != nil {
		t.Fatalf("REST get = %d %s", rec.Code, rec.Body)
	}
	var restList []map[string]any
	if err := json.Unmarshal(serveREST(svc, http.MethodGet, "/v1/postgres", "").Body.Bytes(), &restList); err != nil || len(restList) != 1 {
		t.Fatalf("REST list = %v (%v)", restList, err)
	}
	var mcpGet map[string]any
	rendertest.DecodeMCPText(t, call("get_postgres", map[string]any{"postgresId": id}), &mcpGet)
	var mcpList []map[string]any
	rendertest.DecodeMCPText(t, call("list_postgres_instances", nil), &mcpList)
	if len(mcpList) != 1 {
		t.Fatalf("list_postgres_instances = %v, want one item", mcpList)
	}
	for name, body := range map[string]map[string]any{"get_postgres": mcpGet, "list_postgres_instances[0]": mcpList[0], "create_postgres": created} {
		if owner, _ := body["owner"].(map[string]any); owner["id"] != "tea-a" || owner["email"] != "a@acme.test" || owner["type"] != "team" {
			t.Errorf("%s owner = %v, want REST's owner object", name, body["owner"])
		}
	}
	if !reflect.DeepEqual(mcpGet, restGet) {
		t.Errorf("get_postgres = %v,\nwant REST's %v", mcpGet, restGet)
	}
	if !reflect.DeepEqual(mcpList[0], restList[0]["postgres"]) {
		t.Errorf("list_postgres_instances[0] = %v,\nwant REST's item %v", mcpList[0], restList[0]["postgres"])
	}
}

// queryTool runs query_render_postgres against a fake executor returning res
// and answers the text items.
func queryTool(t *testing.T, res QueryResult) []string {
	t.Helper()
	svc, _ := newService()
	seedDatabaseAt(t, svc, "rows-db", "postgres://resolved/uri")
	var lim queryLimits
	svc.queryExecutor = func(_ context.Context, _, _ string, l queryLimits, _ bool) (QueryResult, error) {
		lim = l
		return res, nil
	}
	out := pgMCPCall(t, svc)("query_render_postgres", map[string]any{"postgresId": "rows-db", "sql": "select 1"})
	if out.StructuredContent != nil {
		t.Fatalf("structuredContent = %v, want none", out.StructuredContent)
	}
	rendertest.AssertRenderMCPResult(t, "query_render_postgres", out)
	if lim.rowCap != 0 || lim.renderJSONBytes != renderQueryResultBytes || lim.statementTimeout != queryStatementTimeout {
		t.Errorf("MCP limits = %+v, want Render's: no row cap, 5 MB of JSON, the statement timeout", lim)
	}
	var texts []string
	for _, c := range out.Content {
		texts = append(texts, c.(*mcp.TextContent).Text)
	}
	return texts
}

// TestMCPQueryAnswersRendersRowObjects pins w4/m378 t011's text: a JSON array
// of row objects keyed by column name — a repeated name keeps its last
// column, a bigint above 2^53 keeps every digit — `[]` for no rows, and no
// row cap.
func TestMCPQueryAnswersRendersRowObjects(t *testing.T) {
	if got := queryTool(t, QueryResult{Columns: []string{"a", "b", "c"}, Rows: [][]any{{int64(1), "x", nil}}}); len(got) != 1 || got[0] != `[{"a":1,"b":"x","c":null}]` {
		t.Errorf("one row = %q", got)
	}
	if got := queryTool(t, QueryResult{Columns: []string{"a"}, Rows: [][]any{}}); len(got) != 1 || got[0] != `[]` {
		t.Errorf("no rows = %q", got)
	}
	if got := queryTool(t, QueryResult{Columns: []string{"n", "n"}, Rows: [][]any{{int64(1), int64(2)}}}); got[0] != `[{"n":2}]` {
		t.Errorf("duplicate column = %q, want the last column's value", got)
	}
	if got := queryTool(t, QueryResult{Columns: []string{"big"}, Rows: [][]any{{int64(9007199254740993)}}}); got[0] != `[{"big":9007199254740993}]` {
		t.Errorf("bigint = %q, want every digit", got)
	}
	rows := make([][]any, 600)
	for i := range rows {
		rows[i] = []any{int64(i + 1)}
	}
	got := queryTool(t, QueryResult{Columns: []string{"g"}, Rows: rows})
	var decoded []map[string]int
	if err := json.Unmarshal([]byte(got[0]), &decoded); err != nil || len(decoded) != 600 || len(got) != 1 {
		t.Errorf("600 rows => %d objects, %d text items (%v), want all 600 and no notice", len(decoded), len(got), err)
	}
}

// TestMCPQueryCutsAtFiveMBWithRendersNotice: past 5 MB of JSON the answer is
// the rows that fit plus Render's notice as a second text item, not a
// refusal; a first row over 5 MB alone answers [] and the first-row notice.
func TestMCPQueryCutsAtFiveMBWithRendersNotice(t *testing.T) {
	big := strings.Repeat("x", 100_000)
	rows := make([][]any, 60) // about 6 MB of JSON
	for i := range rows {
		rows[i] = []any{int64(i + 1), big}
	}
	got := queryTool(t, QueryResult{Columns: []string{"g", "big"}, Rows: rows})
	if len(got) != 2 {
		t.Fatalf("6 MB result = %d text items, want rows then notice", len(got))
	}
	var kept []map[string]any
	if err := json.Unmarshal([]byte(got[0]), &kept); err != nil || len(kept) == 0 || len(kept) >= 60 || len(got[0]) > renderQueryResultBytes {
		t.Fatalf("kept %d rows in %d bytes (%v), want a prefix within 5 MB", len(kept), len(got[0]), err)
	}
	if want := renderTruncatedMessage(len(kept)); got[1] != want || !strings.HasPrefix(want, "Results truncated: showing the first ") {
		t.Errorf("notice = %q, want %q", got[1], want)
	}

	huge := queryTool(t, QueryResult{Columns: []string{"big"}, Rows: [][]any{{strings.Repeat("x", renderQueryResultBytes)}}})
	if len(huge) != 2 || huge[0] != `[]` || !strings.Contains(huge[1], "the first row alone is over 5 MB") {
		t.Errorf("first row over 5 MB = %q, want [] and the first-row notice", huge)
	}

	// The collector's own cut (the executor stopped reading) keeps its rows
	// and still gets the notice.
	cut := queryTool(t, QueryResult{Columns: []string{"g"}, Rows: [][]any{{int64(1)}}, Truncated: true})
	if len(cut) != 2 || cut[0] != `[{"g":1}]` || cut[1] != renderTruncatedMessage(1) {
		t.Errorf("executor-truncated result = %q", cut)
	}
}

// TestRenderRowArrayMatchesRendersBound pins the size check to Render's
// collectRows: a row fits when the array so far, the row, a comma and the
// closing bracket stay within the limit, counting the comma for the first
// row too.
func TestRenderRowArrayMatchesRendersBound(t *testing.T) {
	a := newRenderRowArray(10)
	if !a.add(7) { // 1 + 7 + 2 = 10
		t.Fatal("a 7-byte first row must fit a 10-byte limit")
	}
	if a.add(1) { // 8 + 1 + 2 > 10
		t.Fatal("no second row fits once the array is full")
	}
	if b := newRenderRowArray(10); b.add(8) {
		t.Fatal("an 8-byte first row must not fit: Render counts a comma for it")
	}
}

// TestMCPQueryIntegrationUnderRendersLimits replays the milestone's probes
// against a real Postgres: Render's row objects, no 500-row cap, a 300 KB
// cell answered rather than refused, a 6 MB result cut to 5 MB with the
// notice, a first row over 5 MB answering [] (the wire cap ends it as
// truncated), and the read-only and timeout rails still refusing.
func TestMCPQueryIntegrationUnderRendersLimits(t *testing.T) {
	uri := testDBURI(t)
	svc, _ := newService()
	seedDatabaseAt(t, svc, "render-db", uri)
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	svc.RegisterMCP(srv)
	ctx := ctxAs("user-a")
	serverT, clientT := mcp.NewInMemoryTransports()
	if _, err := srv.Connect(ctx, serverT, nil); err != nil {
		t.Fatal(err)
	}
	cs, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientT, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer cs.Close()
	query := func(sql string) (*mcp.CallToolResult, []string) {
		t.Helper()
		res, err := cs.CallTool(ctx, &mcp.CallToolParams{Name: "query_render_postgres", Arguments: map[string]any{"postgresId": "render-db", "sql": sql}})
		if err != nil {
			t.Fatal(err)
		}
		var texts []string
		for _, c := range res.Content {
			texts = append(texts, c.(*mcp.TextContent).Text)
		}
		return res, texts
	}
	rows := func(sql string) ([]map[string]any, []string) {
		t.Helper()
		res, texts := query(sql)
		if res.IsError {
			t.Fatalf("%s: tool error %q", sql, texts)
		}
		var out []map[string]any
		if err := json.Unmarshal([]byte(texts[0]), &out); err != nil {
			t.Fatalf("%s: decode: %v", sql, err)
		}
		return out, texts
	}

	if _, texts := query("select 1 as a, 'x' as b, null::int as c"); texts[0] != `[{"a":1,"b":"x","c":null}]` {
		t.Errorf("one row = %q", texts)
	}
	if _, texts := query("select 1 as a where false"); len(texts) != 1 || texts[0] != `[]` {
		t.Errorf("no rows = %q", texts)
	}
	if got, texts := rows("select g from generate_series(1,600) g"); len(got) != 600 || len(texts) != 1 {
		t.Errorf("600 rows => %d objects, %d text items", len(got), len(texts))
	}
	if got, texts := rows("select repeat('x', 300000) as big"); len(got) != 1 || len(texts) != 1 {
		t.Errorf("300 KB cell => %d rows, %d text items", len(got), len(texts))
	}
	got, texts := rows("select g, repeat('x', 100000) as big from generate_series(1,60) g")
	if len(got) == 0 || len(got) >= 60 || len(texts) != 2 || texts[1] != renderTruncatedMessage(len(got)) {
		t.Errorf("6 MB result => %d rows, notice %q", len(got), texts[1:])
	}
	if got, texts := rows("select repeat('x', 6000000) as big"); len(got) != 0 || len(texts) != 2 || texts[1] != renderTruncatedMessage(0) {
		t.Errorf("6 MB first row => %d rows, notice %q, want [] and the first-row notice", len(got), texts[1:])
	}
	if res, texts := query("create table render_evil(i int)"); !res.IsError || !strings.Contains(texts[0], "read-only") {
		t.Errorf("write => %q (isError %v), want the read-only refusal", texts, res.IsError)
	}
	short := renderQueryLimits
	short.statementTimeout = 500 * time.Millisecond
	if _, err := runReadOnlyQuery(context.Background(), uri, "select pg_sleep(3)", short); !errors.Is(err, errQueryTimeout) {
		t.Errorf("pg_sleep under Render's limits => %v, want errQueryTimeout", err)
	}
}
