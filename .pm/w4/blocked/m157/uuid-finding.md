# UUID query cells expose driver byte arrays

- **Severity:** minor — identifiers remain recoverable but the console does not display a copyable UUID; no stored data changes.
- **Repro:** 2026-10-03 UTC (2026-10-02 local), QA sweep 40. Create Free Postgres 18, public access off, open SQL console and run the SELECTs below. Reload, open SQL console and repeat. Owned fixture `qa-20261002-parity-r40`, `dpg-db0891itm2ss7389ql90`, namespace `tea-d98210cbbpdc73dcrkvg`, CR UID `8d8279e0-52d4-4142-8201-38efb3e08d24`.
- **Expected:** native UUID `550e8400-e29b-41d4-a716-446655440000` is a canonical lowercase hyphenated string, also inside UUID arrays; NULL remains NULL. **Actual:** native UUID is `[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0]` on REST/MCP and that array text in the console. The same value cast to text is correct. Fresh reload reproduces it.
- **Root:** pgx v5.10.0 `pgtype/uuid.go:280-290` UUIDCodec.DecodeValue returns `[16]byte`, not pgtype.UUID. Although UUID.MarshalJSON (`:109`) and UUID.String (`:101`) are correct, neither runs on that returned array. `lego/backend/internal/postgres/query.go:279-284` appends native values unchanged; Go JSON encodes fixed byte arrays as numeric arrays. `graphql.go:373-389` falls through to JSON marshaling, and `dashboard/src/features/databases/hooks/use-execute-database-query.ts:35-42` copies these string cells directly into `components/sql-console.tsx:172-180`.
- **Fix:** extend m157's shared pre-budget normalizer with the actual decoded UUID shape, using `pgtype.UUID{Bytes: value, Valid: true}.String()` or equivalent canonical formatting. Recurse decoded UUID arrays. Do not stringify generic integer arrays or change `[]byte` bytea base64. Validate the actual driver types, not only hand-built pgtype.UUID fixtures. GraphQL already accepts the resulting string and REST/MCP QueryResult.Rows is `[][]any`, so no schema widening is needed.
- **Blast/aliases:** the same two entry methods Query (MCP read-only) and ExecuteQuery (REST/GraphQL read/write), one executeAuthorizedQuery/runSQLQuery/collectQueryRows path and three adapters enumerated in finding.md. REST POST `/v1/postgres/{id}/query`, GraphQL executeDatabaseQuery, MCP query_render_postgres. runReadOnlyQuery is a test helper, not another public alias. Resource census: Postgres affected; web/static/cron/worker/private/Key Value do not use this SQL collector. New shared normalization must preserve finite numeric/time and m157's special-token branches.
- **Controls:** same-instance raw REST, GraphQL console and MCP preserve NULL, bytea `AAH/`, integer array `[85,14,132]`, bigint 42, numeric 1.25, true, and text-cast UUID. Exact large numeric controls are correct in REST/GraphQL but fail independently in MCP; see m158, not an effect of UUID conversion.
- **Adjacent classes:** auth, missing/forbidden, SQL syntax, read-only enforcement, confirmed writes, timeout, row/raw/encoded budgets and safe errors remain unchanged. During a query the existing loading/result behavior remains; there is no new cache redirect or pre-settle state.
- **Unverified:** multi-dimensional UUID arrays, domain/composite/extension codecs and write RETURNING need local coverage; no production table writes or authenticated Render UUID experiment were performed.
- **Render/contract:** [PostgreSQL 18 UUID documentation](https://www.postgresql.org/docs/18/datatype-uuid.html) specifies canonical textual output. [Render MCP docs](https://render.com/docs/mcp-server) support database queries but do not define UUID JSON encoding. This is a Bex usability/serialization correction under ADR009 and ADR006, not a claimed measured Render wire mismatch.
- **Dedupe:** whole-tree open/blocked/done UUID and byte-array search found sandbox ID and member identity work, not SQL-result formatting. Existing m157 owns this collector and is extended by t008. DO_NOT_DO permits this; latest 40 product commits and collector/adapter history show no landed UUID fix. This is not a regression claim.
- **Evidence:** `.playwright-mcp/qa-r40-uuid.png` inspected; `.playwright-mcp/qa-r40-api-captures.json` and full probes below are durable here. Network/console capture shows only the expected post-delete GET 404. Fixture API 404, owned Database/CNPG cluster/jobs/pods/services/PVC/secrets inventory zero, session revoked. Ledger `.playwright-mcp/qa-muse-r40-ledger.json`.
- **Estimate:** 25m incremental implementation task plus the existing milestone's parity/test closing tasks.

## Exact live requests and complete responses

The GraphQL mutation records come from actual UI clicks, including the fresh-page repeat. Raw response strings avoid accidental JavaScript numeric rounding in the evidence.

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
    },
    {
      "at": "2026-10-03T04:33:01.253Z",
      "request": [
        {
          "operationName": "ExecuteDatabaseQuery",
          "variables": {
            "id": "dpg-db0891itm2ss7389ql90",
            "sql": "SELECT ARRAY['550e8400-e29b-41d4-a716-446655440000'::uuid,NULL::uuid] AS identifiers, NULL::uuid AS absent, decode('0001ff','hex') AS binary_control, ARRAY[85,14,132]::int[] AS integers, 42::bigint AS small_int, 1.25::numeric AS simple_decimal, true AS flag, 9007199254740993::bigint::text AS exact_text;",
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
      "responseRaw": "[{\"data\":{\"executeDatabaseQuery\":{\"__typename\":\"DatabaseQueryResult\",\"columns\":[\"identifiers\",\"absent\",\"binary_control\",\"integers\",\"small_int\",\"simple_decimal\",\"flag\",\"exact_text\"],\"rowCount\":1,\"rows\":[{\"__typename\":\"DatabaseQueryRow\",\"values\":[\"[[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],null]\",null,\"AAH/\",\"[85,14,132]\",\"42\",\"1.25\",\"true\",\"9007199254740993\"]}],\"truncated\":false}}}]\n"
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
      "surface": "REST controls",
      "url": "https://api.bex.co/v1/postgres/dpg-db0891itm2ss7389ql90/query",
      "request": {
        "sql": "SELECT ARRAY['550e8400-e29b-41d4-a716-446655440000'::uuid,NULL::uuid] AS identifiers, NULL::uuid AS absent, decode('0001ff','hex') AS binary_control, ARRAY[85,14,132]::int[] AS integers, 42::bigint AS small_int, 1.25::numeric AS simple_decimal, true AS flag, 9007199254740993::bigint::text AS exact_text;"
      },
      "status": 200,
      "responseRaw": "{\"columns\":[\"identifiers\",\"absent\",\"binary_control\",\"integers\",\"small_int\",\"simple_decimal\",\"flag\",\"exact_text\"],\"rows\":[[[[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],null],null,\"AAH/\",[85,14,132],42,1.25,true,\"9007199254740993\"]],\"rowCount\":1,\"truncated\":false}\n"
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
      "surface": "cleanup",
      "url": "https://api.bex.co/v1/postgres/dpg-db0891itm2ss7389ql90",
      "status": 404,
      "responseRaw": "{\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\"}\n"
    }
  ],
  "console": "SQL console\nRun a single SQL statement against this database. Results are limited to 500 rows and queries time out after 10 seconds.\n\nPress Ctrl+Enter or \u2318+Enter to run\n\nRun query\n\n1 row returned\n\nexact_int\texact_decimal\tidentifier\ttext_control\n9007199254740993\t12345678901234567890.123456789\t[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0]\t550e8400-e29b-41d4-a716-446655440000\n\nQuery history\n\nClear\nSELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;",
  "freshConsole": "SQL console\nRun a single SQL statement against this database. Results are limited to 500 rows and queries time out after 10 seconds.\n\nPress Ctrl+Enter or \u2318+Enter to run\n\nRun query\n\n1 row returned\n\nexact_int\texact_decimal\tidentifier\ttext_control\n9007199254740993\t12345678901234567890.123456789\t[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0]\t550e8400-e29b-41d4-a716-446655440000\n\nQuery history\n\nClear\nSELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;",
  "controlConsole": "SQL console\nRun a single SQL statement against this database. Results are limited to 500 rows and queries time out after 10 seconds.\n\nPress Ctrl+Enter or \u2318+Enter to run\n\nRun query\n\n1 row returned\n\nidentifiers\tabsent\tbinary_control\tintegers\tsmall_int\tsimple_decimal\tflag\texact_text\n[[85,14,132,0,226,155,65,212,167,22,68,102,85,68,0,0],null]\tNULL\tAAH/\t[85,14,132]\t42\t1.25\ttrue\t9007199254740993\n\nQuery history\n\nClear\nSELECT ARRAY['550e8400-e29b-41d4-a716-446655440000'::uuid,NULL::uuid] AS identifiers, NULL::uuid AS absent, decode('0001ff','hex') AS binary_control, ARRAY[85,14,132]::int[] AS integers, 42::bigint AS small_int, 1.25::numeric AS simple_decimal, true AS flag, 9007199254740993::bigint::text AS exact_text;\nSELECT 9007199254740993::bigint AS exact_int, 12345678901234567890.123456789::numeric AS exact_decimal, '550e8400-e29b-41d4-a716-446655440000'::uuid AS identifier, '550e8400-e29b-41d4-a716-446655440000'::uuid::text AS text_control;"
}
```
