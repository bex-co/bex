# Native SQL arrays lose their dimensions

Severity: **major**. Valid successful SQL queries return a flattened value that cannot distinguish a matrix from a list. This extends the open typed-value milestone with t009, not a separate duplicate milestone.

## Live reproduction

Sweep 44, user-requested continuous QA, 2026-10-02 America/Los_Angeles / 2026-10-03 UTC. Owned Free Postgres 18 `qa-20261002-sql-shapes-r44`, ID `dpg-db098eitm2ss7389qm2g`, private access, workspace `tea-d98210cbbpdc73dcrkvg`, Database UID `d6178900-9be2-4507-9d4b-bf3415b749bd`. Provisioning completed normally in approximately two minutes. No production data writes; every query was SELECT.

Open SQL console and run:

```sql
SELECT ARRAY[[1,2],[3,4]] AS native_matrix,
       to_json(ARRAY[[1,2],[3,4]]) AS json_matrix,
       ARRAY[1,2,3,4] AS flat_control,
       array_dims(ARRAY[[1,2],[3,4]]) AS dimensions,
       ARRAY[[1,2],[3,4]]::text AS text_control;
```

Actual dashboard cells: `[1,2,3,4]`, `[[1,2],[3,4]]`, `[1,2,3,4]`, `[1:2][1:2]`, `{{1,2},{3,4}}`. Reload the page, reopen SQL console, refill and run: same result. REST and MCP return the same flattened native cell with HTTP 200; both MCP text and structuredContent agree. PostgreSQL JSON/text/dimension controls independently establish that the source value is 2D. These small exact integers avoid m158's numeric precision issue.

A second SELECT confirmed 3D native integer arrays flatten to eight elements and 2D text arrays flatten while retaining scalar null versus the literal string NULL. PostgreSQL to_json retains both structures. Empty array → `[]`, NULL array → null, ordinary nullable flat array → `[1,null,3]`, bytea → `AAH/` all passed in UI/GraphQL, REST and MCP. Full SQL and responses are embedded below.

**Expected:** native matrix `[[1,2],[3,4]]`, cube `[[[1,2],[3,4]],[[5,6],[7,8]]]`, and text matrix `[["a",null],["NULL","b"]]` as JSON values on REST/MCP, with nested JSON cell text on GraphQL/dashboard. Preserve current 1D, empty, SQL NULL and bytea shapes. PostgreSQL custom array lower bounds are not represented by JSON indexing; no new bounds metadata object is proposed or claimed verified. The claim here is dimension lengths/nesting and ordinal element order.

## Independently traced mechanism

- `lego/backend/internal/postgres/query.go:258-297` bounds raw values, then **279** calls rows.Values and appends those already decoded values. JSON budgeting at **292** faithfully encodes the now-flat slice. A recursive normalizer after Values, as planned for the other m157 types, cannot recover discarded dimensions.
- Actual pinned **pgx v5.10.0**, `rows.go:292-333`: Values chooses each registered codec using field DataTypeOID and Format, then invokes DecodeValue on raw bytes. `pgtype/array_codec.go:398-406` explicitly scans into `[]any`. The decisive scanner for this exact any slice is **not** the optimized FlatArray[int32] path: `pgtype/pgtype.go:954-986` selects `wrapPtrSliceReflectScanPlan` / `anySliceArrayReflect`; `pgtype/builtin_wrappers.go:767-778` computes cardinality and allocates a single slice, discarding dimension boundaries. `array_codec.go:263-281` had the binary dimensions before calling that setter. Merely blaming JSON serialization or adding recursion to flat output is insufficient.
- Strong control trace: `pgtype/json.go:235-243` invokes the codec's Unmarshal into any; `pgtype_default.go:68` wires encoding/json.Unmarshal. Already structured PostgreSQL JSON therefore retains nested JSON slices. It does not traverse ArrayCodec at all. Small int/text/null JSON controls passed; this is not a claim that arbitrary large JSON numbers preserve precision.
- Supported driver mechanism: `pgtype/array.go:386-429` provides `Array[T]` with Elements, Dims and Valid, retaining dimensions through SetDimensions. `pgtype.Map.Scan` at `pgtype.go:1158` can scan raw field bytes to `pgtype.Array[any]`; the executor can pass its actual connection TypeMap (`conn.go:465`, rows also expose Conn at rows.go:339). Read existing raw budgets first. The local proof below ran against the repository's pinned dependency in both binary and text formats and retained the 2×2 dimensions, while DecodeValue flattened them.

**Fix:** t009 retains array field metadata/raw bytes and decodes recognized registered array codecs through that dimensional target, reconstructs bounded nested slices from dimension lengths, normalizes leaf values through the shared m157 scalar converter, and applies the encoded-size budget to the final shape. Preserve non-array/unknown codec behavior, don't infer SQL arrays from Go slice kind (bytea is a counterexample), and don't fork the driver or rewrite user SQL to `to_json` (that changes the query/type contract and can introduce number coercion). Enforce safe checked cardinality and array shape, including empty/null handling. Driver errors retain existing safe query failure behavior.

## Consumer and shared-surface audit

One production call to collectQueryRows (`query.go:363`) feeds runSQLQuery. Public Service.Query (MCP read-only; :107-112) and ExecuteQuery (REST/GraphQL; :125-168) converge through executeAuthorizedQuery (:179). runReadOnlyQuery (:245) is the focused internal/test helper, not another public route. Shared normalization must cover both read-only queries and locally tested confirmed write RETURNING without changing their respective permissions.

Aliases: REST POST `/v1/postgres/{id}/query` (`rest.go:234-253`), GraphQL mutation executeDatabaseQuery (`graphql.go:669-686`), MCP query_render_postgres (`mcp.go:153-162`). The dashboard calls GraphQL. `QueryResult.Rows` (`query.go:87-92`) is `[][]any`, so nested array values fit with no schema change. GraphQL's DatabaseQueryRow values are nullable string cells (`graphql.go:336-340`); `queryCellString:372-389` JSON-marshals nested values, and the existing JSON control proves the result passes the live schema and consumer. Dashboard `hooks/use-execute-database-query.ts:35-42` copies the cells; `components/sql-console.tsx:172-180` renders them. No client reshape is required. MCP's typed schema already accepts the nested JSON control; m158 remains the separate later numeric-precision prerequisite.

Resource-family census: **Postgres only** exposes this SQL executor. Web, private, worker, cron and static Apps plus Key Value do not call it. Those six families are unchanged, not independently live tested in this sweep. Array codecs cover supported registered array element types; SQL domains/composites/extensions were not exercised and must not be swept into an unverified representation redesign.

Adjacent classes: keep query authorization, protected-environment write confirmation, read-only transactions, single-statement enforcement, row/raw/encoded caps, truncation, timeouts and error redaction. Shape repair is after raw size checks and before encoded budgeting, not an excuse to materialize unbounded rows. No data during loading or failed query should be fabricated. The existing query pending and error behavior is unchanged.

## Dedupe and scope disposition

Searched all PM open/blocked/done text for multidimensional arrays, dimension loss, native array shape and driver types. Existing **w4/m157** is the owner: its numeric/temporal/UUID findings explicitly called multidimensional arrays unverified. This sweep supplies missing evidence and a distinct earlier decoding mechanism; it does not retrospectively turn those old caveats into successful checks. Added t009 before parity, updated t001/t003/t005 and the milestone count/DoD. Ordinary-array preservation now explicitly means the passing 1D/empty/NULL controls, not retaining this newly observed flattening.

Re-read DO_NOT_DO; no anti-goal applies. Recent 40 dashboard/lego commits and query.go symbol/file history show no dimension-preserving fix waiting on main. `git log -S rows.Values()` traces the path to original **469e76908**, the first read-only query tool; subsequent executor consolidation retains it. Not a claimed regression of a closed array-shape fix. Open milestone census from the immediately preceding sweep plus current board was reviewed; m158 is later MCP numeric coercion, not this loss, and remains a dependency for large-number controls. No other board item is duplicated or closed.

[PostgreSQL 18 array documentation](https://www.postgresql.org/docs/18/arrays.html) establishes multidimensional arrays and their nested representation; PostgreSQL's own live JSON/text controls are the expected-shape evidence. Render SQL/MCP array wire representation was not authenticated or probed. Existing m157 Render-parity task retains this limitation rather than inventing a vendor guarantee.

## Unverified and cleanup

Custom lower bounds, domain/composite/extension arrays, malformed wire values, write RETURNING, large dimension budgets and combined multidimensional UUID/nonfinite/temporal leaves were not live probed. t009/t005 require local regression coverage, including scalar normalization at nested leaves. Interval/time representation was inspected as an adjacent candidate but no live assertion is filed. No product code changed; local driver feasibility probe ran, not the product suites.

Screenshot `.playwright-mcp/qa-r44-array-shape.png` was visually inspected and shows native-vs-JSON/text/dimension cells. `.playwright-mcp/qa-r44-api-captures.json` holds exact requests and complete responses; console/network and final inventory are local artifacts. Complete durable replay data follows, so the finding does not depend on gitignored files.

UI Delete database succeeded. Postgres GET returned 404; exact Database UID/name, CNPG Cluster, Jobs, Pods, Services, PVCs and Secrets inventory was empty. Logout returned `ok logged-out`; session cookies/scratch jar were cleared. No tables were created or foreign resources mutated.

## Local pinned-driver feasibility proof

Ran `go run ../../.playwright-mcp/qa-r44-array-codec-probe.go` from `lego/backend` (exit 0). The source is research only, not a product implementation:

```go
package main
import("encoding/json";"fmt";"github.com/jackc/pgx/v5/pgtype")
func main(){m:=pgtype.NewMap();typ,_:=m.TypeForOID(pgtype.Int4ArrayOID);for _,format:=range []int16{pgtype.BinaryFormatCode,pgtype.TextFormatCode}{a:=pgtype.Array[int32]{Elements:[]int32{1,2,3,4},Dims:[]pgtype.ArrayDimension{{Length:2,LowerBound:1},{Length:2,LowerBound:1}},Valid:true};b,e:=m.Encode(pgtype.Int4ArrayOID,format,a,nil);if e!=nil{panic(e)};v,e:=typ.Codec.DecodeValue(m,pgtype.Int4ArrayOID,format,b);if e!=nil{panic(e)};var preserved pgtype.Array[any];if e=m.Scan(pgtype.Int4ArrayOID,format,b,&preserved);e!=nil{panic(e)};j,_:=json.Marshal(map[string]any{"format":format,"decodedValue":v,"dimensions":preserved.Dims,"elements":preserved.Elements});fmt.Println(string(j))}}

```

Complete stdout:

```json
{"decodedValue":[1,2,3,4],"dimensions":[{"Length":2,"LowerBound":1},{"Length":2,"LowerBound":1}],"elements":[1,2,3,4],"format":1}
{"decodedValue":[1,2,3,4],"dimensions":[{"Length":2,"LowerBound":1},{"Length":2,"LowerBound":1}],"elements":[1,2,3,4],"format":0}
```

## Durable live probes

GraphQL records include the actual UI create, both matrix SELECTs, the control SELECT and deletion. REST/MCP raw bodies preserve the JSON nesting without client reinterpretation.

```json
{
  "captures": [
    {
      "at": "2026-10-03T05:35:54.484Z",
      "request": [
        {
          "operationName": "CreateDatabase",
          "variables": {
            "name": "qa-20261002-sql-shapes-r44",
            "ownerId": "tea-d98210cbbpdc73dcrkvg",
            "plan": "free",
            "public": false
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation CreateDatabase($name: String!, $databaseName: String, $databaseUser: String, $ownerId: String, $environmentId: String, $plan: String, $version: String, $diskSizeGB: Int, $public: Boolean) {\n  createDatabase(\n    name: $name\n    databaseName: $databaseName\n    databaseUser: $databaseUser\n    ownerId: $ownerId\n    environmentId: $environmentId\n    plan: $plan\n    version: $version\n    diskSizeGB: $diskSizeGB\n    public: $public\n  ) {\n    id\n    name\n    databaseName\n    databaseUser\n    plan\n    status\n    projectId\n    environmentId\n    __typename\n  }\n}"
        }
      ],
      "status": 200,
      "responseRaw": "[{\"data\":{\"createDatabase\":{\"__typename\":\"Database\",\"databaseName\":\"dpg_db098eitm2ss7389qm2g\",\"databaseUser\":\"dpg_db098eitm2ss7389qm2g_user\",\"environmentId\":null,\"id\":\"dpg-db098eitm2ss7389qm2g\",\"name\":\"qa-20261002-sql-shapes-r44\",\"plan\":\"free\",\"projectId\":null,\"status\":\"creating\"}}}]\n"
    },
    {
      "at": "2026-10-03T05:38:33.779Z",
      "request": [
        {
          "operationName": "ExecuteDatabaseQuery",
          "variables": {
            "id": "dpg-db098eitm2ss7389qm2g",
            "sql": "SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;",
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
      "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"native_matrix\",\"json_matrix\",\"flat_control\",\"dimensions\",\"text_control\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"[1,2,3,4]\",\"[[1,2],[3,4]]\",\"[1,2,3,4]\",\"[1:2][1:2]\",\"{{1,2},{3,4}}\"]}],\"truncated\":false}}}]\n"
    },
    {
      "at": "2026-10-03T05:39:05.047Z",
      "request": [
        {
          "operationName": "ExecuteDatabaseQuery",
          "variables": {
            "id": "dpg-db098eitm2ss7389qm2g",
            "sql": "SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;",
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
      "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"native_matrix\",\"json_matrix\",\"flat_control\",\"dimensions\",\"text_control\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"[1,2,3,4]\",\"[[1,2],[3,4]]\",\"[1,2,3,4]\",\"[1:2][1:2]\",\"{{1,2},{3,4}}\"]}],\"truncated\":false}}}]\n"
    },
    {
      "at": "2026-10-03T05:39:29.582Z",
      "request": [
        {
          "operationName": "ExecuteDatabaseQuery",
          "variables": {
            "id": "dpg-db098eitm2ss7389qm2g",
            "sql": "SELECT ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]] AS cube, to_json(ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]]) AS cube_json, ARRAY[['a',NULL],['NULL','b']]::text[] AS text_matrix, to_json(ARRAY[['a',NULL],['NULL','b']]::text[]) AS text_json, ARRAY[]::integer[] AS empty_array, NULL::integer[] AS null_array, ARRAY[1,NULL,3] AS flat_nullable, decode('0001ff','hex') AS bytea_control;",
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
      "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"cube\",\"cube_json\",\"text_matrix\",\"text_json\",\"empty_array\",\"null_array\",\"flat_nullable\",\"bytea_control\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"[1,2,3,4,5,6,7,8]\",\"[[[1,2],[3,4]],[[5,6],[7,8]]]\",\"[\\\"a\\\",null,\\\"NULL\\\",\\\"b\\\"]\",\"[[\\\"a\\\",null],[\\\"NULL\\\",\\\"b\\\"]]\",\"[]\",null,\"[1,null,3]\",\"AAH/\"]}],\"truncated\":false}}}]\n"
    },
    {
      "at": "2026-10-03T05:40:40.528Z",
      "request": [
        {
          "operationName": "DeleteDatabase",
          "variables": {
            "id": "dpg-db098eitm2ss7389qm2g"
          },
          "extensions": {
            "clientLibrary": {
              "name": "@apollo/client",
              "version": "4.1.3"
            }
          },
          "query": "mutation DeleteDatabase($id: String!, $confirm: String) {\n  deleteDatabase(id: $id, confirm: $confirm)\n}"
        }
      ],
      "status": 200,
      "responseRaw": "[{\"data\":{\"deleteDatabase\":true}}]\n"
    }
  ],
  "probes": [
    {
      "path": "/v1/postgres/dpg-db098eitm2ss7389qm2g/query",
      "request": {
        "sql": "SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;"
      },
      "status": 200,
      "body": "{\"columns\":[\"native_matrix\",\"json_matrix\",\"flat_control\",\"dimensions\",\"text_control\"],\"rows\":[[[1,2,3,4],[[1,2],[3,4]],[1,2,3,4],\"[1:2][1:2]\",\"{{1,2},{3,4}}\"]],\"rowCount\":1,\"truncated\":false}\n"
    },
    {
      "path": "/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 44,
        "method": "tools/call",
        "params": {
          "name": "query_render_postgres",
          "arguments": {
            "postgresId": "dpg-db098eitm2ss7389qm2g",
            "sql": "SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;"
          }
        }
      },
      "status": 200,
      "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":44,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"native_matrix\\\",\\\"json_matrix\\\",\\\"flat_control\\\",\\\"dimensions\\\",\\\"text_control\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[[1,2,3,4],[[1,2],[3,4]],[1,2,3,4],\\\"[1:2][1:2]\\\",\\\"{{1,2},{3,4}}\\\"]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"native_matrix\",\"json_matrix\",\"flat_control\",\"dimensions\",\"text_control\"],\"rowCount\":1,\"rows\":[[[1,2,3,4],[[1,2],[3,4]],[1,2,3,4],\"[1:2][1:2]\",\"{{1,2},{3,4}}\"]],\"truncated\":false}}}\n\n"
    }
  ],
  "controlProbes": [
    {
      "path": "/v1/postgres/dpg-db098eitm2ss7389qm2g/query",
      "request": {
        "sql": "SELECT ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]] AS cube, to_json(ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]]) AS cube_json, ARRAY[['a',NULL],['NULL','b']]::text[] AS text_matrix, to_json(ARRAY[['a',NULL],['NULL','b']]::text[]) AS text_json, ARRAY[]::integer[] AS empty_array, NULL::integer[] AS null_array, ARRAY[1,NULL,3] AS flat_nullable, decode('0001ff','hex') AS bytea_control;"
      },
      "status": 200,
      "body": "{\"columns\":[\"cube\",\"cube_json\",\"text_matrix\",\"text_json\",\"empty_array\",\"null_array\",\"flat_nullable\",\"bytea_control\"],\"rows\":[[[1,2,3,4,5,6,7,8],[[[1,2],[3,4]],[[5,6],[7,8]]],[\"a\",null,\"NULL\",\"b\"],[[\"a\",null],[\"NULL\",\"b\"]],[],null,[1,null,3],\"AAH/\"]],\"rowCount\":1,\"truncated\":false}\n"
    },
    {
      "path": "/mcp",
      "request": {
        "jsonrpc": "2.0",
        "id": 45,
        "method": "tools/call",
        "params": {
          "name": "query_render_postgres",
          "arguments": {
            "postgresId": "dpg-db098eitm2ss7389qm2g",
            "sql": "SELECT ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]] AS cube, to_json(ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]]) AS cube_json, ARRAY[['a',NULL],['NULL','b']]::text[] AS text_matrix, to_json(ARRAY[['a',NULL],['NULL','b']]::text[]) AS text_json, ARRAY[]::integer[] AS empty_array, NULL::integer[] AS null_array, ARRAY[1,NULL,3] AS flat_nullable, decode('0001ff','hex') AS bytea_control;"
          }
        }
      },
      "status": 200,
      "body": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":45,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"cube\\\",\\\"cube_json\\\",\\\"text_matrix\\\",\\\"text_json\\\",\\\"empty_array\\\",\\\"null_array\\\",\\\"flat_nullable\\\",\\\"bytea_control\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[[1,2,3,4,5,6,7,8],[[[1,2],[3,4]],[[5,6],[7,8]]],[\\\"a\\\",null,\\\"NULL\\\",\\\"b\\\"],[[\\\"a\\\",null],[\\\"NULL\\\",\\\"b\\\"]],[],null,[1,null,3],\\\"AAH/\\\"]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"cube\",\"cube_json\",\"text_matrix\",\"text_json\",\"empty_array\",\"null_array\",\"flat_nullable\",\"bytea_control\"],\"rowCount\":1,\"rows\":[[[1,2,3,4,5,6,7,8],[[[1,2],[3,4]],[[5,6],[7,8]]],[\"a\",null,\"NULL\",\"b\"],[[\"a\",null],[\"NULL\",\"b\"]],[],null,[1,null,3],\"AAH/\"]],\"truncated\":false}}}\n\n"
    }
  ],
  "firstSnapshot": "- main:\n  - navigation \"Breadcrumbs\": Database qa-20261002-sql-shapes-r44\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - heading \"qa-20261002-sql-shapes-r44\" [level=1]\n  - text: Available\n  - button \"Open actions menu\"\n  - navigation \"Database details\":\n    - button \"Overview\"\n    - button \"Logs\"\n  - navigation \"Database sections\":\n    - link \"Details\":\n      - /url: \"#metadata\"\n    - link \"Connections\":\n      - /url: \"#connection\"\n    - link \"SQL console\":\n      - /url: \"#sql-console\"\n    - link \"High Availability\":\n      - /url: \"#high-availability\"\n    - link \"Metrics\":\n      - /url: \"#metrics\"\n    - link \"Instance type\":\n      - /url: \"#plan\"\n    - link \"Insights\":\n      - /url: \"#insights\"\n    - link \"Recovery\":\n      - /url: \"#recovery\"\n    - link \"Access control\":\n      - /url: \"#access-control\"\n    - link \"Danger Zone\":\n      - /url: \"#danger-zone\"\n  - text: Details Name Change the display name. The database ID and all connection details stay the same.\n  - textbox \"Name\" [disabled]: qa-20261002-sql-shapes-r44\n  - button \"Edit database name\"\n  - term: Status\n  - definition: Available\n  - term: Instance type\n  - definition: Free\n  - term: Version\n  - definition: PostgreSQL 18\n  - term: Database\n  - definition: dpg_db098eitm2ss7389qm2g\n  - term: User\n  - definition: dpg_db098eitm2ss7389qm2g_user\n  - term: Storage\n  - definition: 1 GB\n  - term: High availability\n  - definition: \"No\"\n  - term: Public access\n  - definition: \"No\"\n  - term: Region\n  - definition: fsn1\n  - term: Created\n  - definition:\n    - time: 2m\n  - text: Connections Connection strings and the database password. Revealed only when you ask — never shown automatically.\n  - button \"Reveal connection info\"\n  - text: SQL console Run a single SQL statement against this database. Results are limited to 500 rows and queries time out after 10 seconds.\n  - textbox \"SQL query\": SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;\n  - paragraph: Press Ctrl+Enter or ⌘+Enter to run\n  - button \"Run query\"\n  - paragraph: 1 row returned\n  - table:\n    - rowgroup:\n      - row \"native_matrix json_matrix flat_control dimensions text_control\":\n        - columnheader \"native_matrix\"\n        - columnheader \"json_matrix\"\n        - columnheader \"flat_control\"\n        - columnheader \"dimensions\"\n        - columnheader \"text_control\"\n    - rowgroup:\n      - 'row \"[1,2,3,4] [[1,2],[3,4]] [1,2,3,4] [1:2][1:2] {{1,2},{3,4}}\"':\n        - cell \"[1,2,3,4]\"\n        - cell \"[[1,2],[3,4]]\"\n        - cell \"[1,2,3,4]\"\n        - cell \"[1:2][1:2]\"\n        - 'cell \"{{1,2},{3,4}}\"'\n  - paragraph: Query history\n  - button \"Clear\"\n  - button \"SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;\"\n  - text: High Availability Replicated cluster with automatic failover. When enabled, a standby is always ready to take over. Status Disabled\n  - paragraph: High availability requires a plan with at least 1 CPU. This database's plan does not offer it.\n  - text: Metrics Live resource usage for this instance Disk Capacity 1 GiB Disk autoscaling 1 GB current · 1 GB max This plan does not support disk autoscaling. An existing setting can be turned off.\n  - switch \"Disk autoscaling\" [disabled]\n  - text: This plan does not support disk autoscaling. An existing setting can be turned off. No data in range Active Connections No data in range Replication Lag N/A — no replica (enable High Availability to see replication lag)\n  - button \"Delete Database\"\n  - button \"Restart Database\"\n  - button \"Suspend Database\"",
  "repeatSnapshot": "- main:\n  - navigation \"Breadcrumbs\": Database qa-20261002-sql-shapes-r44\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - heading \"qa-20261002-sql-shapes-r44\" [level=1]\n  - text: Available\n  - button \"Open actions menu\"\n  - navigation \"Database details\":\n    - button \"Overview\"\n    - button \"Logs\"\n  - navigation \"Database sections\":\n    - link \"Details\":\n      - /url: \"#metadata\"\n    - link \"Connections\":\n      - /url: \"#connection\"\n    - link \"SQL console\":\n      - /url: \"#sql-console\"\n    - link \"High Availability\":\n      - /url: \"#high-availability\"\n    - link \"Metrics\":\n      - /url: \"#metrics\"\n    - link \"Instance type\":\n      - /url: \"#plan\"\n    - link \"Insights\":\n      - /url: \"#insights\"\n    - link \"Recovery\":\n      - /url: \"#recovery\"\n    - link \"Access control\":\n      - /url: \"#access-control\"\n    - link \"Danger Zone\":\n      - /url: \"#danger-zone\"\n  - text: Details Name Change the display name. The database ID and all connection details stay the same.\n  - textbox \"Name\" [disabled]: qa-20261002-sql-shapes-r44\n  - button \"Edit database name\"\n  - term: Status\n  - definition: Available\n  - term: Instance type\n  - definition: Free\n  - term: Version\n  - definition: PostgreSQL 18\n  - term: Database\n  - definition: dpg_db098eitm2ss7389qm2g\n  - term: User\n  - definition: dpg_db098eitm2ss7389qm2g_user\n  - term: Storage\n  - definition: 1 GB\n  - term: High availability\n  - definition: \"No\"\n  - term: Public access\n  - definition: \"No\"\n  - term: Region\n  - definition: fsn1\n  - term: Created\n  - definition:\n    - time: 3m\n  - text: Connections Connection strings and the database password. Revealed only when you ask — never shown automatically.\n  - button \"Reveal connection info\"\n  - text: SQL console Run a single SQL statement against this database. Results are limited to 500 rows and queries time out after 10 seconds.\n  - textbox \"SQL query\": SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;\n  - paragraph: Press Ctrl+Enter or ⌘+Enter to run\n  - button \"Run query\"\n  - paragraph: 1 row returned\n  - table:\n    - rowgroup:\n      - row \"native_matrix json_matrix flat_control dimensions text_control\":\n        - columnheader \"native_matrix\"\n        - columnheader \"json_matrix\"\n        - columnheader \"flat_control\"\n        - columnheader \"dimensions\"\n        - columnheader \"text_control\"\n    - rowgroup:\n      - 'row \"[1,2,3,4] [[1,2],[3,4]] [1,2,3,4] [1:2][1:2] {{1,2},{3,4}}\"':\n        - cell \"[1,2,3,4]\"\n        - cell \"[[1,2],[3,4]]\"\n        - cell \"[1,2,3,4]\"\n        - cell \"[1:2][1:2]\"\n        - 'cell \"{{1,2},{3,4}}\"'\n  - paragraph: Query history\n  - button \"Clear\"\n  - button \"SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;\"\n  - text: High Availability Replicated cluster with automatic failover. When enabled, a standby is always ready to take over. Status Disabled\n  - paragraph: High availability requires a plan with at least 1 CPU. This database's plan does not offer it.\n  - text: Metrics Live resource usage for this instance Disk Capacity 1 GiB Disk autoscaling 1 GB current · 1 GB max This plan does not support disk autoscaling. An existing setting can be turned off.\n  - switch \"Disk autoscaling\" [disabled]\n  - text: This plan does not support disk autoscaling. An existing setting can be turned off.\n  - img \"Line chart with 1 data points\": 0 B 31.8 MiB 63.6 MiB\n  - text: Active Connections\n  - img \"Line chart with 2 data points\": 0 0.5 1\n  - text: Replication Lag N/A — no replica (enable High Availability to see replication lag)\n  - button \"Delete Database\"\n  - button \"Restart Database\"\n  - button \"Suspend Database\"",
  "controlSnapshot": "- main:\n  - navigation \"Breadcrumbs\": Database qa-20261002-sql-shapes-r44\n  - button \"Search\": Search ⌘ K\n  - button \"New\"\n  - button \"Help and resources\"\n  - button \"P\"\n  - heading \"qa-20261002-sql-shapes-r44\" [level=1]\n  - text: Available\n  - button \"Open actions menu\"\n  - navigation \"Database details\":\n    - button \"Overview\"\n    - button \"Logs\"\n  - navigation \"Database sections\":\n    - link \"Details\":\n      - /url: \"#metadata\"\n    - link \"Connections\":\n      - /url: \"#connection\"\n    - link \"SQL console\":\n      - /url: \"#sql-console\"\n    - link \"High Availability\":\n      - /url: \"#high-availability\"\n    - link \"Metrics\":\n      - /url: \"#metrics\"\n    - link \"Instance type\":\n      - /url: \"#plan\"\n    - link \"Insights\":\n      - /url: \"#insights\"\n    - link \"Recovery\":\n      - /url: \"#recovery\"\n    - link \"Access control\":\n      - /url: \"#access-control\"\n    - link \"Danger Zone\":\n      - /url: \"#danger-zone\"\n  - text: Details Name Change the display name. The database ID and all connection details stay the same.\n  - textbox \"Name\" [disabled]: qa-20261002-sql-shapes-r44\n  - button \"Edit database name\"\n  - term: Status\n  - definition: Available\n  - term: Instance type\n  - definition: Free\n  - term: Version\n  - definition: PostgreSQL 18\n  - term: Database\n  - definition: dpg_db098eitm2ss7389qm2g\n  - term: User\n  - definition: dpg_db098eitm2ss7389qm2g_user\n  - term: Storage\n  - definition: 1 GB\n  - term: High availability\n  - definition: \"No\"\n  - term: Public access\n  - definition: \"No\"\n  - term: Region\n  - definition: fsn1\n  - term: Created\n  - definition:\n    - time: 3m\n  - text: Connections Connection strings and the database password. Revealed only when you ask — never shown automatically.\n  - button \"Reveal connection info\"\n  - text: SQL console Run a single SQL statement against this database. Results are limited to 500 rows and queries time out after 10 seconds.\n  - textbox \"SQL query\": SELECT ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]] AS cube, to_json(ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]]) AS cube_json, ARRAY[['a',NULL],['NULL','b']]::text[] AS text_matrix, to_json(ARRAY[['a',NULL],['NULL','b']]::text[]) AS text_json, ARRAY[]::integer[] AS empty_array, NULL::integer[] AS null_array, ARRAY[1,NULL,3] AS flat_nullable, decode('0001ff','hex') AS bytea_control;\n  - paragraph: Press Ctrl+Enter or ⌘+Enter to run\n  - button \"Run query\"\n  - paragraph: 1 row returned\n  - table:\n    - rowgroup:\n      - row \"cube cube_json text_matrix text_json empty_array null_array flat_nullable bytea_control\":\n        - columnheader \"cube\"\n        - columnheader \"cube_json\"\n        - columnheader \"text_matrix\"\n        - columnheader \"text_json\"\n        - columnheader \"empty_array\"\n        - columnheader \"null_array\"\n        - columnheader \"flat_nullable\"\n        - columnheader \"bytea_control\"\n    - rowgroup:\n      - row \"[1,2,3,4,5,6,7,8] [[[1,2],[3,4]],[[5,6],[7,8]]] [\\\"a\\\",null,\\\"NULL\\\",\\\"b\\\"] [[\\\"a\\\",null],[\\\"NULL\\\",\\\"b\\\"]] [] NULL [1,null,3] AAH/\":\n        - cell \"[1,2,3,4,5,6,7,8]\"\n        - cell \"[[[1,2],[3,4]],[[5,6],[7,8]]]\"\n        - cell \"[\\\"a\\\",null,\\\"NULL\\\",\\\"b\\\"]\"\n        - cell \"[[\\\"a\\\",null],[\\\"NULL\\\",\\\"b\\\"]]\"\n        - cell \"[]\"\n        - cell \"NULL\"\n        - cell \"[1,null,3]\"\n        - cell \"AAH/\"\n  - paragraph: Query history\n  - button \"Clear\"\n  - button \"SELECT ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]] AS cube, to_json(ARRAY[[[1,2],[3,4]],[[5,6],[7,8]]]) AS cube_json, ARRAY[['a',NULL],['NULL','b']]::text[] AS text_matrix, to_json(ARRAY[['a',NULL],['NULL','b']]::text[]) AS text_json, ARRAY[]::integer[] AS empty_array, NULL::integer[] AS null_array, ARRAY[1,NULL,3] AS flat_nullable, decode('0001ff','hex') AS bytea_control;\"\n  - button \"SELECT ARRAY[[1,2],[3,4]] AS native_matrix, to_json(ARRAY[[1,2],[3,4]]) AS json_matrix, ARRAY[1,2,3,4] AS flat_control, array_dims(ARRAY[[1,2],[3,4]]) AS dimensions, ARRAY[[1,2],[3,4]]::text AS text_control;\"\n  - text: High Availability Replicated cluster with automatic failover. When enabled, a standby is always ready to take over. Status Disabled\n  - paragraph: High availability requires a plan with at least 1 CPU. This database's plan does not offer it.\n  - text: Metrics Live resource usage for this instance Disk Capacity 1 GiB Disk autoscaling 1 GB current · 1 GB max This plan does not support disk autoscaling. An existing setting can be turned off.\n  - switch \"Disk autoscaling\" [disabled]\n  - text: This plan does not support disk autoscaling. An existing setting can be turned off.\n  - img \"Line chart with 1 data points\": 0 B 31.8 MiB 63.6 MiB\n  - text: Active Connections\n  - img \"Line chart with 2 data points\": 0 0.5 1\n  - text: Replication Lag N/A — no replica (enable High Availability to see replication lag)\n  - button \"Delete Database\"\n  - button \"Restart Database\"\n  - button \"Suspend Database\"",
  "deleted": {
    "status": 404,
    "body": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
  }
}
```
