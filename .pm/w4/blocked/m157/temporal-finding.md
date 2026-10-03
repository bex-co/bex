# Temporal infinity markers are serialized as integers

Why: an unbounded date or timestamp is a meaningful SQL value; reporting it as 1 or -1 silently changes query results for users and agents.

**Source:** continuous QA sweep 39, 2026-10-03 UTC, muse.env identity, same QA workspace, HEAD `506ed7305`. Free PostgreSQL 18 fixture `qa-20261002-dates-r39`, id `dpg-db082natm2ss7389ql70`. **Severity:** major (incorrect successful query result, no stored-data change). This is a third independently traced result-conversion mechanism, added to open w4/m157 rather than creating competing work at the same shared boundary.

## Reproduction

Run the first SELECT below through the dashboard console. Infinite date/timestamp cells show 1/-1, while their PostgreSQL ::text controls show infinity/-infinity. Reload the page and run the second SELECT: same result. REST also returns `rows:[[1,-1,[1,-1],1,-1]]`, including a date array. MCP independently returns `[1,-1,1]`. Finite date/timestamp/timestamptz, NULL date and text-cast infinity controls return the correct existing representations. Full probes follow.

## Root, target, and consumer verification

Same Bex forwarding/budget boundary as finding.md (`postgres/query.go:258-295`) and GraphQL cell projection (`graphql.go:353-389`), **different driver value**: pgx v5.10.0 DateCodec.DecodeValue (`pgtype/date.go:396-414`), TimestampCodec.DecodeValue (`timestamp.go:352-369`), and TimestamptzCodec.DecodeValue (`timestamptz.go:354-371`) return **pgtype.InfinityModifier** for a non-finite value, and time.Time for an ordinary one. `pgtype.go:132-151` defines this as int8, with Infinity=1 and NegativeInfinity=-1; its String method provides infinity/-infinity but it has no JSON marshaler. Actual Go 1.27 encoding/json chooses integer encoding for an int8 (`encode.go:441,552`), so JSON encoding succeeds as 1/-1. This is neither the rejected float NaN path nor Numeric.MarshalJSON's zero fallback in the original two findings.

Extend the shared pre-budget normalization with an explicit **pgtype.InfinityModifier** branch: recognized positive/negative markers become strings **infinity** / **-infinity**, including decoded array elements. Keep PostgreSQL's lowercase temporal spelling (numeric/float spellings remain NaN/Infinity/-Infinity per finding.md). Never map ordinary int8/integer ±1 to these tokens, and do not stringify all integers. The decoder returns finite dates as time.Time, so preserve their existing RFC3339 projection. Do not treat an unexpected marker as a genuine numeric value; handle only defined marker states deliberately and cover that branch locally.

QueryResult's any-valued rows can express the strings; queryCellString's existing string branch passes them to GraphQL without extra quotes, and the dashboard renders the returned string directly. The two entry points, one result collector, REST/GraphQL/MCP aliases, Postgres-only resource census, raw/encoded caps, auth/write confirmation and timeout/error taxonomy are unchanged from finding.md. This task extends the shared conversion policy rather than adding an adapter-specific shim. Preserve pre-settle loading/error behavior and all finite controls. No changes to timestamp timezone policy or ordinary date formatting are requested.

[PostgreSQL 18 date/time documentation](https://www.postgresql.org/docs/18/datatype-datetime.html#DATATYPE-DATETIME-SPECIAL-VALUES), checked 2026-10-03, explicitly defines both temporal infinities. No authenticated Render special-date probe was made; retain the milestone's honest vendor-encoding limitation.

## Dedupe, boundaries, and cleanup

Whole-tree open/blocked/done searches for date/timestamp with infinity/non-finite found only m157's prior unverified-neighbor note. The current product history and target query collector show no landed correction; DO_NOT_DO and the cross-workstream milestone inventory reviewed in the same QA session have no competing owner or anti-goal. This is newly verified scope, not a claim that the numeric filing had already reproduced temporal values. Implement with /pm add-task before the standing closing tasks; keep its acceptance explicit.

Date arrays and scalar date/timestamp/timestamptz were tested. Multidimensional temporal arrays, interval infinity, BC/far-future dates, extension/composite types and writable RETURNING remain unverified; do not claim those defects or redesign their wire contract here. Both direct and text-cast controls ran as read-only SELECTs.

Deleted the fixture through the dashboard; API GET returned 404; owned Database/CNPG/pod/PVC/service/secret inventory empty; session revoked. Screenshot `.playwright-mcp/qa-r39-infinite-dates.png` inspected, network `.playwright-mcp/qa-r39-network.txt`, complete raw captures below. Authentication headers/cookies and credentials are excluded.

## Complete requests and responses

GraphQL POST https://api.bex.co/graphql uses application/json and the existing authenticated browser session. Direct REST/MCP URLs and bodies are included; MCP Accept is application/json,text/event-stream. Raw response strings preserve wire values.

### Capture 1

```json
{
  "at": "2026-10-03T04:15:25.516Z",
  "request": [
    {
      "operationName": "CreateDatabase",
      "variables": {
        "name": "qa-20261002-dates-r39",
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
  "responseRaw": "[{\"data\":{\"createDatabase\":{\"__typename\":\"Database\",\"databaseName\":\"dpg_db082natm2ss7389ql70\",\"databaseUser\":\"dpg_db082natm2ss7389ql70_user\",\"environmentId\":null,\"id\":\"dpg-db082natm2ss7389ql70\",\"name\":\"qa-20261002-dates-r39\",\"plan\":\"free\",\"projectId\":null,\"status\":\"creating\"}}}]\n"
}
```

### Capture 2

```json
{
  "at": "2026-10-03T04:17:15.923Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db082natm2ss7389ql70",
        "sql": "SELECT 'infinity'::date AS future_date, '-infinity'::date AS past_date, 'infinity'::timestamp AS future_timestamp, '-infinity'::timestamptz AS past_timestamptz, 'infinity'::date::text AS text_control, '2026-10-03'::date AS finite_date;",
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
  "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"future_date\",\"past_date\",\"future_timestamp\",\"past_timestamptz\",\"text_control\",\"finite_date\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"1\",\"-1\",\"1\",\"-1\",\"infinity\",\"2026-10-03T00:00:00Z\"]}],\"truncated\":false}}}]\n"
}
```

### Capture 3

```json
{
  "at": "2026-10-03T04:18:34.334Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db082natm2ss7389ql70",
        "sql": "SELECT '-infinity'::date AS date_value, 'infinity'::timestamp AS timestamp_value, 'infinity'::timestamptz AS timestamptz_value, '-infinity'::date::text AS text_control;",
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
  "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"date_value\",\"timestamp_value\",\"timestamptz_value\",\"text_control\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"-1\",\"1\",\"1\",\"-infinity\"]}],\"truncated\":false}}}]\n"
}
```

### Capture 4

```json
{
  "at": "2026-10-03T04:18:47.615Z",
  "request": [
    {
      "operationName": "DeleteDatabase",
      "variables": {
        "id": "dpg-db082natm2ss7389ql70"
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

### Capture 5

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db082natm2ss7389ql70/query",
  "body": {
    "sql": "SELECT 'infinity'::date AS pos_date, '-infinity'::date AS neg_date, ARRAY['infinity'::date,'-infinity'::date] AS date_array, 'infinity'::timestamp AS pos_ts, '-infinity'::timestamptz AS neg_tstz;",
    "allowWrites": false
  },
  "status": 200,
  "responseRaw": "{\"columns\":[\"pos_date\",\"neg_date\",\"date_array\",\"pos_ts\",\"neg_tstz\"],\"rows\":[[1,-1,[1,-1],1,-1]],\"rowCount\":1,\"truncated\":false}\n"
}
```

### Capture 6

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db082natm2ss7389ql70/query",
  "body": {
    "sql": "SELECT '2026-10-03'::date AS d, '2026-10-03 01:02:03'::timestamp AS ts, '2026-10-03 01:02:03+00'::timestamptz AS tstz, NULL::date AS null_date, 'infinity'::date::text AS text_control;",
    "allowWrites": false
  },
  "status": 200,
  "responseRaw": "{\"columns\":[\"d\",\"ts\",\"tstz\",\"null_date\",\"text_control\"],\"rows\":[[\"2026-10-03T00:00:00Z\",\"2026-10-03T01:02:03Z\",\"2026-10-03T01:02:03Z\",null,\"infinity\"]],\"rowCount\":1,\"truncated\":false}\n"
}
```

### Capture 7

```json
{
  "url": "https://api.bex.co/mcp",
  "body": {
    "jsonrpc": "2.0",
    "id": 39,
    "method": "tools/call",
    "params": {
      "name": "query_render_postgres",
      "arguments": {
        "postgresId": "dpg-db082natm2ss7389ql70",
        "sql": "SELECT 'infinity'::date AS d, '-infinity'::timestamp AS ts, 'infinity'::timestamptz AS tstz;"
      }
    }
  },
  "status": 200,
  "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":39,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"d\\\",\\\"ts\\\",\\\"tstz\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[1,-1,1]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"d\",\"ts\",\"tstz\"],\"rowCount\":1,\"rows\":[[1,-1,1]],\"truncated\":false}}}\n\n"
}
```

### Capture 8

```json
{
  "url": "https://api.bex.co/mcp",
  "body": {
    "jsonrpc": "2.0",
    "id": 39,
    "method": "tools/call",
    "params": {
      "name": "query_render_postgres",
      "arguments": {
        "postgresId": "dpg-db082natm2ss7389ql70",
        "sql": "SELECT '2026-10-03'::date AS finite, 'infinity'::date::text AS text_control;"
      }
    }
  },
  "status": 200,
  "responseRaw": "event: message\ndata: {\"jsonrpc\":\"2.0\",\"id\":39,\"result\":{\"content\":[{\"type\":\"text\",\"text\":\"{\\\"columns\\\":[\\\"finite\\\",\\\"text_control\\\"],\\\"rowCount\\\":1,\\\"rows\\\":[[\\\"2026-10-03T00:00:00Z\\\",\\\"infinity\\\"]],\\\"truncated\\\":false}\"}],\"structuredContent\":{\"columns\":[\"finite\",\"text_control\"],\"rowCount\":1,\"rows\":[[\"2026-10-03T00:00:00Z\",\"infinity\"]],\"truncated\":false}}}\n\n"
}
```

### Capture 9

```json
{
  "url": "https://api.bex.co/v1/postgres/dpg-db082natm2ss7389ql70",
  "status": 404,
  "responseRaw": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
}
```
