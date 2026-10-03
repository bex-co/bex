# MCP silently rounds exact SQL results

- **Severity:** major — successful reads return changed identifiers and decimal values to agents. Stored database values are unchanged.
- **Repro:** QA sweep 40, 2026-10-03 UTC / 2026-10-02 local. Owned Free Postgres 18 `qa-20261002-parity-r40`, id `dpg-db0891itm2ss7389ql90`, public off. Run the exact SELECT below in dashboard SQL console, REST and MCP. Reload dashboard, repeat the console query and MCP request. Both rounds reproduce the discrepancy.
- **Expected:** retain the core's exact JSON numbers in MCP structuredContent and its text block. **Actual:** `9007199254740993` becomes `9007199254740992`; `12345678901234567890.123456789` becomes `12345678901234567000`. Positive/negative bigint array entries also round, and a long numeric fraction becomes `0.12345678901234568`. REST bytes and GraphQL string cells are exact. These are raw response strings, not numbers parsed into JavaScript.

## Root and consumer path

1. `lego/backend/internal/postgres/query.go:87-92` exposes QueryResult.Rows as `[][]any`; collectQueryRows at :258-295 preserves finite native values and serializes them accurately for its budget. REST `rest.go:248` writes that result. GraphQL `graphql.go:353-389` produces exact scalar strings. These are strong positive controls, not a caller workaround: the live wire retains every numeric digit before MCP's distinct adapter.
2. `postgres/mcp.go:153-162` registers query_render_postgres with typed QueryResult output and returns `nil, res, nil`. `mcputil/mcputil.go:44-65` delegates typed registration to the SDK while preserving safe errors and handler panic isolation.
3. Actual pinned **modelcontextprotocol/go-sdk v1.6.1** `mcp/server.go:308-315` derives an output schema for this typed Out. At :365-386 it first marshals the output exactly, then calls applySchema, then assigns the returned raw JSON to structuredContent. The later RawMessage does not recover precision already lost by validation.
4. SDK `mcp/tool.go:69-107` applySchema decodes into `map[string]any`, applies defaults, validates and marshals the map again. SDK `internal/json/json.go:20-31` sets case handling, but does not enable UseNumber. The pinned **segmentio/encoding v0.5.4**, `json/decode.go:1334-1394`, takes its default float64 branch for dynamic numbers. The re-encoded map therefore contains rounded values.
5. SDK server.go:389-395 generates the default text content from that same lossy JSON. Both MCP result forms are wrong for the same reason. `mcp/protocol.go:71-96` CallToolResult can hold a json.RawMessage in StructuredContent and exact text in Content, so the desired shape is expressible.
6. `lego/backend/internal/api/mcp_workspace.go:89-117` returns next's result directly; it strips/validates workspace input, not query rows. The SQL string is not a numeric input argument. No dashboard consumer processes MCP output; its hook and SQLConsole consume GraphQL string cells. The affected consumers are MCP clients, including agents that read only text content.

## Concrete fix and constraints

Use a **query-only explicit-result path**: keep mcputil's typed input and error/panic wrapper, advertise the same QueryResult schema (derive with the pinned jsonschema.For[QueryResult]), return an explicit CallToolResult containing the once-marshaled QueryResult as json.RawMessage plus identical JSON text, and return nil for an `any` Out. SDK server.go:364 skips the output transformation when outval is nil; it preserves the explicit result. Merely setting StructuredContent while still returning typed QueryResult will be overwritten and does not fix this.

The application must continue owning the output contract on that path: serialize the concrete QueryResult type, retain its required fields and validate its shape without an untyped float64 round trip. Do not silently lose schema advertisement, remove input validation, bypass mcputil panic/safe-error handling, stringify finite numbers, or alter the REST/GraphQL representations. An additive helper may make the explicit-result contract clear, but only this tool is to opt in. No global SDK fork/upgrade is required. A local SDK-only experiment below proves this path preserves both server-side result forms while still rejecting a non-string sql argument; it is a feasibility probe, not an implemented product fix or a substitute for tests.

## Blast radius, aliases and controls

- One query_render_postgres registration exists in production Go. Searches also find comments and the existing sensitive-operation scope table, not another alias. REST POST `/v1/postgres/{id}/query` and GraphQL executeDatabaseQuery are comparison surfaces and retain their existing handlers. Workspace-id middleware remains in force.
- There are **189 syntactic mcputil.AddTool registration call sites** across 24 packages (some register families/loops, so this is not a runtime tool-name count). Generic SDK output conversion can affect other exact-number outputs in principle, but they were not live-probed and no claim of 189 broken tools is made. The fix is allowlisted to one query registration; audit the shared helper and prove existing typed registrations are unchanged. Census by package follows.
- Resource census: Postgres SQL query affected; web/static/cron/worker/private/Key Value do not expose this SQL tool. Their other MCP tools stay on current registrations. Wider output/input precision auditing is unverified adjacent work, not an implicit migration in this milestone.
- Positive live controls on the same fixture: 42, 1.25, null, true, bytea `AAH/`, ordinary integer arrays and exact text preserve their values through MCP. Float64 represents the small numeric controls exactly; text never enters numeric decoding. REST and console preserve the large native numeric values. UUID byte arrays are a separate driver-conversion defect added to m157/t008, not a precision failure.
- Adjacent behavior: retain read-only transaction policy, SQL validation and safe errors, scoped not-found/forbidden, unauthenticated rejection, statement timeout, row and byte caps. No auth/error taxonomy change or pre-settle dashboard behavior is needed.
- Unverified: input numeric arguments on other tools, other result types, arbitrary clients' JSON decoders, writable RETURNING, extreme numeric exponent handling and authenticated Render wire precision. The wire guarantee cannot force a client to use a lossless JSON decoder. End-to-end tests must inspect raw bytes or UseNumber, not reintroduce rounding in their oracle.

## Dedupe, contract and cleanup

Whole-tree open/blocked/done searches for precision/rounding/900719925/UseNumber/applySchema and UUIDs found no owner for MCP exact-number serialization. m157 covers non-finite numeric, temporal and UUID core conversion; this is an independent finite-number adapter cause. Its finite-MCP baseline assumption is now explicitly corrected and its parity task depends on m158/t002. No existing precision milestone was declared regressed. DO_NOT_DO and open milestone titles across workstreams were reviewed. Latest 40 product commits and targeted collector, MCP registration, wrapper and dependency history show no landed fix waiting for deploy.

ADR006 documents the shared core and read-only agent tool; ADR009 governs managed Postgres. [Render MCP documentation](https://render.com/docs/mcp-server), checked 2026-10-03, advertises database queries but does not specify exact numeric encoding. No live authenticated Render comparison was performed. Preserve Bex's measured REST/GraphQL exact values; do not normalize them onto the wrong MCP answer.

Deleted owned fixture: API GET 404; owned Database/CNPG cluster/jobs/pods/services/PVC/secrets inventory zero. Session revoked. Local artifacts verified: `.playwright-mcp/qa-r40-api-captures.json`, `qa-r40-uuid.png` (inspected), `qa-r40-network.txt`, `qa-r40-console.txt`, `qa-r40-sdk-probe.go`, `qa-r40-sdk-probe.txt`, `qa-r40-inventory-final.json`, `qa-muse-r40-ledger.json`. Only expected post-delete GET 404 in console/network. Complete requests/responses are embedded below so ignored artifacts are not the handoff's sole evidence.

### Registration census

```json
{
  "agentsessions": 14,
  "apikeys": 3,
  "apps": 45,
  "billing": 3,
  "deploys": 7,
  "envgroups": 17,
  "environments": 5,
  "events": 3,
  "github": 3,
  "jobs": 4,
  "keyvalue": 7,
  "logs": 2,
  "members": 12,
  "metrics": 2,
  "notifications": 2,
  "postgres": 24,
  "projects": 5,
  "registrycreds": 5,
  "sandbox": 4,
  "secrets": 10,
  "sshkeys": 3,
  "usage": 1,
  "webhooks": 6,
  "workspaces": 2
}
```

## Complete live probes

Raw response strings preserve source bytes; read GraphQL string cells and both MCP text/structuredContent before parsing numbers.

```json
{
  "graphql": [
    {
      "at": "2026-10-03T04:31:26.107Z",
      "request": [
        {
          "operationName": "ExecuteDatabaseQuery",
          "variables": {
            "id": "dpg-db0891itm2ss7389ql90",
            "sql": "SELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;",
            "allowWrites": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation ExecuteDatabaseQuery($id: String!, $sql: String!, $allowWrites: Boolean) {\n  executeDatabaseQuery(id: $id, sql: $sql, allowWrites: $allowWrites) {\n    columns\n    rows {\n      values\n      __typename\n    }\n    rowCount\n    truncated\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"exact_int\",\"exact_decimal\",\"identifier\",\"text_control\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"9007199254740993\",\"12345678901234567890.123456789\",\"[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0]\",\"550e8400-e29b-41d4-a716-446655440000\"]}],\"truncated\":false}}}]\n"
    },
    {
      "at": "2026-10-03T04:32:20.592Z",
      "request": [
        {
          "operationName": "ExecuteDatabaseQuery",
          "variables": {
            "id": "dpg-db0891itm2ss7389ql90",
            "sql": "SELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;",
            "allowWrites": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation ExecuteDatabaseQuery($id: String!, $sql: String!, $allowWrites: Boolean) {\n  executeDatabaseQuery(id: $id, sql: $sql, allowWrites: $allowWrites) {\n    columns\n    rows {\n      values\n      __typename\n    }\n    rowCount\n    truncated\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"exact_int\",\"exact_decimal\",\"identifier\",\"text_control\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"9007199254740993\",\"12345678901234567890.123456789\",\"[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0]\",\"550e8400-e29b-41d4-a716-446655440000\"]}],\"truncated\":false}}}]\n"
    }
  ],
  "probes": [
    {
      "surface": "REST",
      "url": "https://api.bex.co/v1/postgres/dpg-db0891itm2ss7389ql90/query",
      "request": {
        "sql": "SELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;"
      },
      "status": 200,
      "responseRaw": "{\"columns\":[\"exact_int\",\"exact_decimal\",\"identifier\",\"text_control\"],\"rows\":[[9007199254740993,12345678901234567890.123456789,[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],\"550e8400-e29b-41d4-a716-446655440000\"]],\"rowCount\":1,\"truncated\":false}\n"
    },
    {
      "surface": "MCP",
      "url": "https://api.bex.co/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 40,
        "method": "tools/call",
        "params": {
          "name": "query_render_postgres",
          "arguments": {
            "postgresId": "dpg-db0891itm2ss7389ql90",
            "sql": "SELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;"
          }
        }
      },
      "status": 200,
      "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":40,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"exact_int\\\",\\\"exact_decimal\\\",\\\"identifier\\\",\\\"text_control\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[9007199254740992,12345678901234567000,[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],\\\"550e8400-e29b-41d4-a716-446655440000\\\"]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"exact_int\",\"exact_decimal\",\"identifier\",\"text_control\"],\"rowCount\":1,\"rows\":[[9007199254740992,12345678901234567000,[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],\"550e8400-e29b-41d4-a716-446655440000\"]],\"truncated\":false}}}\n\n"
    },
    {
      "surface": "MCP fresh reload",
      "url": "https://api.bex.co/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 401,
        "method": "tools/call",
        "params": {
          "name": "query_render_postgres",
          "arguments": {
            "postgresId": "dpg-db0891itm2ss7389ql90",
            "sql": "SELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;"
          }
        }
      },
      "status": 200,
      "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":401,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"exact_int\\\",\\\"exact_decimal\\\",\\\"identifier\\\",\\\"text_control\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[9007199254740992,12345678901234567000,[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],\\\"550e8400-e29b-41d4-a716-446655440000\\\"]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"exact_int\",\"exact_decimal\",\"identifier\",\"text_control\"],\"rowCount\":1,\"rows\":[[9007199254740992,12345678901234567000,[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],\"550e8400-e29b-41d4-a716-446655440000\"]],\"truncated\":false}}}\n\n"
    },
    {
      "surface": "MCP controls",
      "url": "https://api.bex.co/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 402,
        "method": "tools/call",
        "params": {
          "name": "query_render_postgres",
          "arguments": {
            "postgresId": "dpg-db0891itm2ss7389ql90",
            "sql": "SELECT ARRAY['550e8400-e29b-41d4-a716-446655440000'::uuid,NULL::uuid] AS identifiers, NULL::uuid AS absent, decode('0001ff','hex') AS binary_control, ARRAY[85,14,132]::int[] AS integers, 42::bigint AS small_int, 1.25::numeric AS simple_decimal, true AS flag, 9007199254740993::bigint::text AS exact_text;"
          }
        }
      },
      "status": 200,
      "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":402,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"identifiers\\\",\\\"absent\\\",\\\"binary_control\\\",\\\"integers\\\",\\\"small_int\\\",\\\"simple_decimal\\\",\\\"flag\\\",\\\"exact_text\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[[[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],null],null,\\\"AAH/\\\",[85,14,132],42,1.25,true,\\\"9007199254740993\\\"]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"identifiers\",\"absent\",\"binary_control\",\"integers\",\"small_int\",\"simple_decimal\",\"flag\",\"exact_text\"],\"rowCount\":1,\"rows\":[[[[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],null],null,\"AAH/\",[85,14,132],42,1.25,true,\"9007199254740993\"]],\"truncated\":false}}}\n\n"
    },
    {
      "surface": "REST numeric arrays",
      "url": "https://api.bex.co/v1/postgres/dpg-db0891itm2ss7389ql90/query",
      "request": {
        "sql": "SELECT ARRAY[9007199254740993::bigint,-9007199254740993::bigint] AS exact_array, 0.12345678901234567890123456789::numeric AS fraction, 42::bigint AS small_int, 1.25::numeric AS simple_decimal, NULL::numeric AS absent;"
      },
      "status": 200,
      "responseRaw": "{\"columns\":[\"exact_array\",\"fraction\",\"small_int\",\"simple_decimal\",\"absent\"],\"rows\":[[[9007199254740993,-9007199254740993],0.12345678901234567890123456789,42,1.25,null]],\"rowCount\":1,\"truncated\":false}\n"
    },
    {
      "surface": "MCP numeric arrays",
      "url": "https://api.bex.co/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 403,
        "method": "tools/call",
        "params": {
          "name": "query_render_postgres",
          "arguments": {
            "postgresId": "dpg-db0891itm2ss7389ql90",
            "sql": "SELECT ARRAY[9007199254740993::bigint,-9007199254740993::bigint] AS exact_array, 0.12345678901234567890123456789::numeric AS fraction, 42::bigint AS small_int, 1.25::numeric AS simple_decimal, NULL::numeric AS absent;"
          }
        }
      },
      "status": 200,
      "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":403,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"exact_array\\\",\\\"fraction\\\",\\\"small_int\\\",\\\"simple_decimal\\\",\\\"absent\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[[9007199254740992,-9007199254740992],0.12345678901234568,42,1.25,null]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"exact_array\",\"fraction\",\"small_int\",\"simple_decimal\",\"absent\"],\"rowCount\":1,\"rows\":[[[9007199254740992,-9007199254740992],0.12345678901234568,42,1.25,null]],\"truncated\":false}}}\n\n"
    },
    {
      "surface": "cleanup",
      "url": "https://api.bex.co/v1/postgres/dpg-db0891itm2ss7389ql90",
      "status": 404,
      "responseRaw": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
    }
  ],
  "freshConsole": "SQL console\nRun a single SQL statement against this database. Results are limited to 500 rows and queries time out after 10 seconds.\n\nPress Ctrl+Enter or \u2318+Enter to run\n\nRun query\n\n1 row returned\n\nexact_int\texact_decimal\tidentifier\ttext_control\n9007199254740993\t12345678901234567890.123456789\t[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0]\t550e8400-e29b-41d4-a716-446655440000\n\nQuery history\n\nClear\nSELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;"
}
```

## Local SDK feasibility probe

Executed with `go run /Volumes/nvme4tbfish/projects/bex4/.playwright-mcp/qa-r40-sdk-probe.go` from lego/backend, using the workspace's pinned dependencies. No network or production changes. Receiving middleware captures the server result before a client decoder can round it. First result: current typed output; second: explicit result with the same advertised schema; third: invalid input still rejected.

```go
package main
import("context";"encoding/json";"fmt";"github.com/google/jsonschema-go/jsonschema";"github.com/modelcontextprotocol/go-sdk/mcp")
type Input struct{SQL string `json:"sql"`}
type Output struct{Columns []string `json:"columns"`;Rows [][]any `json:"rows"`;RowCount int `json:"rowCount"`;Truncated bool `json:"truncated"`}
func main(){ctx:=context.Background();s:=mcp.NewServer(&mcp.Implementation{Name:"qa-local",Version:"1"},nil);value:=Output{[]string{"integer","decimal"},[][]any{{int64(9007199254740993),json.Number("12345678901234567890.123456789")}},1,false};schema,e:=jsonschema.For[Output](nil);if e!=nil{panic(e)}
mcp.AddTool(s,&mcp.Tool{Name:"typed"},func(context.Context,*mcp.CallToolRequest,Input)(*mcp.CallToolResult,Output,error){return nil,value,nil})
mcp.AddTool(s,&mcp.Tool{Name:"explicit",OutputSchema:schema},func(context.Context,*mcp.CallToolRequest,Input)(*mcp.CallToolResult,any,error){b,e:=json.Marshal(value);if e!=nil{return nil,nil,e};return &mcp.CallToolResult{StructuredContent:json.RawMessage(b),Content:[]mcp.Content{&mcp.TextContent{Text:string(b)}}},nil,nil})
s.AddReceivingMiddleware(func(next mcp.MethodHandler)mcp.MethodHandler{return func(ctx context.Context,method string,req mcp.Request)(mcp.Result,error){r,e:=next(ctx,method,req);if method=="tools/call"{b,_:=json.Marshal(r);fmt.Println(string(b))};return r,e}})
ct,st:=mcp.NewInMemoryTransports();ss,e:=s.Connect(ctx,st,nil);if e!=nil{panic(e)};defer ss.Close();c:=mcp.NewClient(&mcp.Implementation{Name:"qa-client",Version:"1"},nil);cs,e:=c.Connect(ctx,ct,nil);if e!=nil{panic(e)};defer cs.Close();for _,name:=range []string{"typed","explicit"}{_,e=cs.CallTool(ctx,&mcp.CallToolParams{Name:name,Arguments:Input{"SELECT"}});if e!=nil{panic(e)}};_,e=cs.CallTool(ctx,&mcp.CallToolParams{Name:"explicit",Arguments:map[string]any{"sql":12}});if e!=nil{panic(e)}
}
```

```jsonl
{"content":[{"type":"text","text":"{\"columns\":[\"integer\",\"decimal\"],\"rowCount\":1,\"rows\":[[9007199254740992,12345678901234567000]],\"truncated\":false}"}],"structuredContent":{"columns":["integer","decimal"],"rowCount":1,"rows":[[9007199254740992,12345678901234567000]],"truncated":false}}
{"content":[{"type":"text","text":"{\"columns\":[\"integer\",\"decimal\"],\"rows\":[[9007199254740993,12345678901234567890.123456789]],\"rowCount\":1,\"truncated\":false}"}],"structuredContent":{"columns":["integer","decimal"],"rows":[[9007199254740993,12345678901234567890.123456789]],"rowCount":1,"truncated":false}}
{"content":[{"type":"text","text":"validating \"arguments\": validating root: validating /properties/sql: type: 12 has type \"integer\", want \"string\""}],"isError":true}
```
