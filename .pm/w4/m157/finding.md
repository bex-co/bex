# Postgres special numeric results fail or silently change to zero

Why: database users and MCP agents must receive the value PostgreSQL returned, rather than an internal error or a plausible but false numeric result.

**Source:** live qa-find-bugs sweep 37, 2026-10-03 UTC, requested muse.env identity, workspace `tea-d98210cbbpdc73dcrkvg`, HEAD `0b1ec76db`. Owned Free PostgreSQL 18 fixture `qa-20261002-types-r37`, id `dpg-db07mqatm2ss7389ql1g`. No table writes were needed: every probe was a constant SELECT.

## Two independently traced defects

### A. Floating-point NaN and infinities abort a valid query

**Severity:** major for the affected SQL-result journey (query cannot return a result); no database data loss or platform outage observed.

Dashboard SQL console, fresh reload reproduction, REST and MCP all fail on valid special floating values:

- `SELECT 'NaN'::double precision AS special_value;` → REST 500 internal_error.
- `SELECT 'Infinity'::real AS special_value;` → GraphQL HTTP 200 with executeDatabaseQuery null and internal error; console alert internal error after fresh load.
- `SELECT '-Infinity'::double precision AS special_value;` → MCP HTTP 200, isError true, internal error.
- `SELECT ARRAY['Infinity'::double precision, 1.25::double precision] AS values;` → REST 500. One special element breaks the complete result, not just that cell.

**Root:** `lego/backend/internal/postgres/query.go:258-295` collects `pgx.Rows.Values()` unchanged then calls `json.Marshal(out.Rows)` to enforce its encoded-size budget. The error returns errQueryFailed, an unclassified `core.Err` at `:81`, which is correctly sanitized rather than exposed. This is a serialization failure after successful SQL execution, not a SQLSTATE failure or unavailable database. The fix belongs before the budget encoding, not in global error sanitization.

Actual pinned driver **pgx v5.10.0**: `pgtype/float4.go:312-325` returns float32, `float8.go:358` returns float64. Actual selected backend toolchain **Go 1.27.0**: `encoding/json/encode.go:572-574` rejects IsInf/IsNaN with UnsupportedValueError. `pgtype/array_codec.go:399-412` decodes arrays into `[]any`, explaining the nested reproduction. This was read in the installed module/toolchain sources, not inferred from UI prose.

### B. Numeric infinity becomes an ordinary zero

**Severity:** major (silent result corruption; persisted database values are unchanged).

`SELECT 'Infinity'::numeric AS pos, '-Infinity'::numeric AS neg, ARRAY['Infinity'::numeric,'-Infinity'::numeric,12.5::numeric] AS nested;` returns REST **200** with `rows:[[0,0,[0,0,12.5]]]`. MCP independently returns `[0,0]`. Dashboard initially displays `0 0`, and a fresh-load second query displays numeric `0` beside its own `::text` control `-Infinity`.

**Separate root:** the same unchanged Values forwarding reaches `pgtype.Numeric`, not a Go float. Pinned `pgtype/numeric.go:832-843` returns the Numeric struct; its `MarshalJSON:240-251` checks Valid and NaN but never checks InfinityModifier. `numberTextBytes:266` returns `0` when Int is nil. Binary/text scans explicitly produce Numeric{InfinityModifier: ±Infinity, Valid:true} (`:627-629`, `:797-799`), with nil Int. JSON encoding succeeds, so no error is raised and both signs are silently lost. This explains why A errors while B succeeds incorrectly; do not collapse them into one unspecified JSON bug.

GraphQL `postgres/graphql.go:353-389` then uses queryCellString, which JSON-marshals non-string values again. This also renders numeric NaN as the text **"NaN" with literal quotation marks**, while its numeric Infinity peers render `0`. A shared normalization before result budgeting can supply ordinary string tokens so this existing string branch preserves the intended spelling.

## Expected behavior and fix direction

Normalize non-finite SQL numeric values at the shared result boundary, before its encoded JSON budget check and adapter projection. Return exact string tokens **NaN**, **Infinity**, **-Infinity** for float4, float8, and pgtype.Numeric special values, including elements of pgx-decoded arrays. REST/MCP use JSON strings for those values; GraphQL scalar cells show the bare token text (no extra quote layer), with an array cell retaining its existing JSON-array string presentation. SQL NULL remains JSON/null, never a special-value replacement.

Preserve finite numbers as their existing native JSON values, exact finite Numeric encoding, integers, booleans, text, bytea/base64 and timestamps. Do not stringify the entire result, parse finite Numeric through float64, change SQL, or require users to cast everything to text. Keep raw cell/row/total budgets, encoded-result budget, row truncation, statement deadline, read-only transaction and write-confirmation fences. Measure the normalized representation for the encoded budget so normalization cannot bypass the cap. Explicitly handle the driver's actual decoded array container rather than assuming only top-level scalar cells.

No global driver fork or broad dependency upgrade is necessary to define this local API contract. If an upstream version is chosen instead, it must demonstrably handle both mechanisms and retain the contract; an upgrade alone is not acceptance.

## Controls, consumer checks, aliases and blast radius

- Strong positive controls were queried on the same live instance and captured verbatim: finite real/double values and arrays return numbers; finite `12345678901234567890.123456789::numeric` retains its exact bytes on REST and its exact string in the dashboard; `9007199254740993::bigint` displays exactly; NULL, empty text, comma/multiline text, true and int arrays render correctly. MCP finite-double plus text-cast Infinity succeeds. `::text` casts preserve the same special values while native numeric casts do not, identifying conversion rather than SQL evaluation.
- Service entry points: **2**, Query (`query.go:112`, MCP read-only) and ExecuteQuery (`:168`, REST/GraphQL, read-only by default or explicitly confirmed write mode), both call executeAuthorizedQuery (`:179`). That helper chooses runSQLQuery; `collectQueryRows` has **1 production call** at `:363`. runReadOnlyQuery at `:245` is a retained focused test helper, not another public route.
- Public aliases: REST `POST /v1/postgres/{id}/query` (`rest.go:230-254`); GraphQL `executeDatabaseQuery(id,sql,allowWrites,confirm)` (`graphql.go:686-690`); MCP `query_render_postgres(postgresId,sql)` (`mcp.go:154-160`); dashboard SQL console uses that GraphQL mutation. Query-insights processes/top-queries/sizes/table-scans have distinct implementations; do not edit them or call them affected without separate evidence.
- QueryResult uses `[][]any` so mixed native numbers and special strings are representable. GraphQL DatabaseQueryRow values is a list of nullable strings (`graphql.go:335-339`, gqlutil.StrsField). queryCellString's string branch returns a string directly; normalized tokens therefore do not acquire the current NaN extra quotes. No GraphQL schema migration is required.
- Consumer `dashboard/src/features/databases/hooks/use-execute-database-query.ts:39-45` preserves the returned string cells. `components/sql-console.tsx:170-185` renders the value directly, mapping only null/undefined to NULL. Existing per-query loading/error handling is retained before settlement; do not optimistically fabricate rows or label an error an empty result.
- Resource census: Postgres only owns this SQL executor. Web/static/cron/worker/private are App services and Key Value uses Valkey, with no entry into this Postgres result collector. No other resource family needs a special-number patch.
- Security/error neighbors stay unchanged: missing/foreign IDs and auth checks, protected-environment confirmation for writable execution, statement timeout, bad SQL and SQLSTATE-safe errors, oversized result rejection, readonly write refusal. Do not expose internal driver details, row values, or connection secrets via logs/errors. Suspended-database connection failure remains separately tracked by **w4/177**.
- Unverified live: writable INSERT/UPDATE RETURNING special values (all probes were SELECT read-only), multidimensional array presentation, composite/extension types, special dates/timestamps, bytea/time controls, and authenticated Render output for these values. Those are not claimed observed defects. Cover the relevant write-result and nested-array behavior in local tests; unrelated date/composite API redesign is out of scope.

## Dedupe and contract

All open/blocked/done `.pm` files searched for whole-word NaN, Infinity, non-finite, large integer and numeric precision; earlier broad matches were manually narrowed to avoid matching words such as tenant. Matches in w7/done/048 concern a CSS slider calculation, w7/done/052 timestamp formatting, and w7/blocked/m156 replication WAL metrics; none owns Postgres query values. w4/177 has the same sanitized error text but a different suspended-connection cause. Open milestone titles across workstreams and DO_NOT_DO were reviewed; no anti-goal applies. Latest 40 product commits and targeted collectQueryRows/queryCellString history show no newer fix. The SQL-console feature commit `004d489e6` and collector refactor `9058b699e` explain existing paths; this filing does not claim a previously promised special-number fix regressed.

[PostgreSQL 18 numeric type documentation](https://www.postgresql.org/docs/18/datatype-numeric.html), checked 2026-10-03, defines these values for numeric and floating-point types; they are supported SQL values, not malformed queries. [Render MCP documentation](https://render.com/docs/mcp-server) establishes the SQL-query tool comparison, but does not specify special-number JSON encoding. Bex's canonical token spelling is an explicit JSON-safe representation, not an unverified claim about Render wire output. ADR009 SQL-console error mapping and ADR006 shared adapters remain the local contract; REST/GraphQL console execution is a Bex extension.

## Cleanup and evidence

Fixture deleted through dashboard, API detail 404, Database/CNPG/pod/PVC/service/secret inventory empty, QA session revoked. Screenshots `.playwright-mcp/qa-r37-special-float.png` and `qa-r37-numeric-infinity.png` were inspected; network `.playwright-mcp/qa-r37-network.txt` contains the deliberate REST 500 probes. Full safe request/response records below survive the handoff. Headers/cookies/passwords are excluded. No product implementation changed in this QA run.

## Complete probes

GraphQL is POST https://api.bex.co/graphql with application/json and authenticated browser cookies; batched bodies/responses are preserved. REST POST and MCP URLs/bodies are explicit below. MCP Accept is application/json,text/event-stream; HTTP 200 with isError is a tool failure, not a successful query. Raw response strings preserve exact numeric bytes without JavaScript re-parsing.

### Capture 1

```json
{
  "at": "2026-10-03T03:50:01.639Z",
  "request": [
    {
      "operationName": "CreateDatabase",
      "variables": {
        "name": "qa-20261002-types-r37",
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
  "responseRaw": "[{\"data\":{\"createDatabase\":{\"__typename\":\"Database\",\"databaseName\":\"dpg_db07mqatm2ss7389ql1g\",\"databaseUser\":\"dpg_db07mqatm2ss7389ql1g_user\",\"environmentId\":null,\"id\":\"dpg-db07mqatm2ss7389ql1g\",\"name\":\"qa-20261002-types-r37\",\"plan\":\"free\",\"projectId\":null,\"status\":\"creating\"}}}]\n"
}
```

### Capture 2

```json
{
  "at": "2026-10-03T03:53:04.283Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db07mqatm2ss7389ql1g",
        "sql": "SELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, NULL::text AS nullable, ''::text AS empty_text, 'a,b'::text AS comma_text, E'line1\\nline2'::text AS multiline, true AS flag, ARRAY[1,2,3]::int[] AS numbers;",
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
  "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"exact_int\",\"exact_decimal\",\"nullable\",\"empty_text\",\"comma_text\",\"multiline\",\"flag\",\"numbers\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"9007199254740993\",\"12345678901234567890.123456789\",null,\"\",\"a,b\",\"line1\\nline2\",\"true\",\"[1,2,3]\"]}],\"truncated\":false}}}]\n"
}
```

### Capture 3

```json
{
  "at": "2026-10-03T03:53:22.106Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db07mqatm2ss7389ql1g",
        "sql": "SELECT 'NaN'::double precision AS nan_float, 'Infinity'::double precision AS positive_infinity, '-Infinity'::double precision AS negative_infinity;",
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
  "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":null},\"errors\":[{\"message\":\"internal error\",\"locations\":[{\"line\":2,\"column\":3}],\"path\":[\"executeDatabaseQuery\"]}]}]\n"
}
```

### Capture 4

```json
{
  "at": "2026-10-03T03:54:10.206Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db07mqatm2ss7389ql1g",
        "sql": "SELECT 'Infinity'::real AS special_value;",
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
  "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":null},\"errors\":[{\"message\":\"internal error\",\"locations\":[{\"line\":2,\"column\":3}],\"path\":[\"executeDatabaseQuery\"]}]}]\n"
}
```

### Capture 5

```json
{
  "at": "2026-10-03T03:55:05.491Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db07mqatm2ss7389ql1g",
        "sql": "SELECT 'Infinity'::numeric AS positive_inf, '-Infinity'::numeric AS negative_inf, 'NaN'::numeric AS not_a_number, 'Infinity'::numeric::text AS text_control;",
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
  "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"positive_inf\",\"negative_inf\",\"not_a_number\",\"text_control\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"0\",\"0\",\"\\\"NaN\\\"\",\"Infinity\"]}],\"truncated\":false}}}]\n"
}
```

### Capture 6

```json
{
  "at": "2026-10-03T03:56:07.569Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db07mqatm2ss7389ql1g",
        "sql": "SELECT '-Infinity'::numeric AS exact_numeric, '-Infinity'::numeric::text AS text_control;",
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
  "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"exact_numeric\",\"text_control\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"0\",\"-Infinity\"]}],\"truncated\":false}}}]\n"
}
```

### Capture 7

```json
{
  "at": "2026-10-03T03:56:39.564Z",
  "request": [
    {
      "operationName": "DeleteDatabase",
      "variables": {
        "id": "dpg-db07mqatm2ss7389ql1g"
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
```

### Capture 8

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db07mqatm2ss7389ql1g/query",
  "body": {
    "sql": "SELECT 'NaN'::double precision AS special_value;",
    "allowWrites": false
  },
  "status": 500,
  "responseRaw": "{\"error\":\"internal error\",\"id\":\"internal_error\",\"message\":\"internal error\"}\n"
}
```

### Capture 9

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db07mqatm2ss7389ql1g/query",
  "body": {
    "sql": "SELECT ARRAY['Infinity'::double precision, 1.25::double precision] AS values;",
    "allowWrites": false
  },
  "status": 500,
  "responseRaw": "{\"error\":\"internal error\",\"id\":\"internal_error\",\"message\":\"internal error\"}\n"
}
```

### Capture 10

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db07mqatm2ss7389ql1g/query",
  "body": {
    "sql": "SELECT 'NaN'::numeric AS nan_numeric, 'Infinity'::numeric AS inf_numeric, 'NaN'::text AS text_value, 1.25::double precision AS finite_value;",
    "allowWrites": false
  },
  "status": 200,
  "responseRaw": "{\"columns\":[\"nan_numeric\",\"inf_numeric\",\"text_value\",\"finite_value\"],\"rows\":[[\"NaN\",0,\"NaN\",1.25]],\"rowCount\":1,\"truncated\":false}\n"
}
```

### Capture 11

```json
{
  "url": "https://api.bex.co/mcp",
  "body": {
    "jsonrpc": "2.0",
    "id": 37,
    "method": "tools/call",
    "params": {
      "name": "query_render_postgres",
      "arguments": {
        "postgresId": "dpg-db07mqatm2ss7389ql1g",
        "sql": "SELECT '-Infinity'::double precision AS special_value;"
      }
    }
  },
  "status": 200,
  "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":37,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"internal error\"}],\"isError\":true}}\n\n"
}
```

### Capture 12

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db07mqatm2ss7389ql1g/query",
  "body": {
    "sql": "SELECT 'Infinity'::numeric AS pos, '-Infinity'::numeric AS neg, ARRAY['Infinity'::numeric,'-Infinity'::numeric,12.5::numeric] AS nested;",
    "allowWrites": false
  },
  "status": 200,
  "responseRaw": "{\"columns\":[\"pos\",\"neg\",\"nested\"],\"rows\":[[0,0,[0,0,12.5]]],\"rowCount\":1,\"truncated\":false}\n"
}
```

### Capture 13

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db07mqatm2ss7389ql1g/query",
  "body": {
    "sql": "SELECT 'NaN'::real AS n, 'NaN'::real::text AS text_control;",
    "allowWrites": false
  },
  "status": 500,
  "responseRaw": "{\"error\":\"internal error\",\"id\":\"internal_error\",\"message\":\"internal error\"}\n"
}
```

### Capture 14

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db07mqatm2ss7389ql1g/query",
  "body": {
    "sql": "SELECT 1.25::real AS finite_real, 2.5::double precision AS finite_double, ARRAY[1.25,2.5]::double precision[] AS finite_array, 12345678901234567890.123456789::numeric AS exact_decimal;",
    "allowWrites": false
  },
  "status": 200,
  "responseRaw": "{\"columns\":[\"finite_real\",\"finite_double\",\"finite_array\",\"exact_decimal\"],\"rows\":[[1.25,2.5,[1.25,2.5],12345678901234567890.123456789]],\"rowCount\":1,\"truncated\":false}\n"
}
```

### Capture 15

```json
{
  "url": "https://api.bex.co/mcp",
  "body": {
    "jsonrpc": "2.0",
    "id": 37,
    "method": "tools/call",
    "params": {
      "name": "query_render_postgres",
      "arguments": {
        "postgresId": "dpg-db07mqatm2ss7389ql1g",
        "sql": "SELECT 'Infinity'::numeric AS pos, '-Infinity'::numeric AS neg;"
      }
    }
  },
  "status": 200,
  "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":37,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"pos\\\",\\\"neg\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[0,0]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"pos\",\"neg\"],\"rowCount\":1,\"rows\":[[0,0]],\"truncated\":false}}}\n\n"
}
```

### Capture 16

```json
{
  "url": "https://api.bex.co/mcp",
  "body": {
    "jsonrpc": "2.0",
    "id": 37,
    "method": "tools/call",
    "params": {
      "name": "query_render_postgres",
      "arguments": {
        "postgresId": "dpg-db07mqatm2ss7389ql1g",
        "sql": "SELECT 1.25::double precision AS finite, 'Infinity'::numeric::text AS text_control;"
      }
    }
  },
  "status": 200,
  "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":37,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"finite\\\",\\\"text_control\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[1.25,\\\"Infinity\\\"]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"finite\",\"text_control\"],\"rowCount\":1,\"rows\":[[1.25,\"Infinity\"]],\"truncated\":false}}}\n\n"
}
```

### Capture 17

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db07mqatm2ss7389ql1g",
  "status": 404,
  "responseRaw": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
}
```
