# SQL JSON values lose digits and scalar form

Why: a successful SQL read must preserve the database's identifiers and decimal values.

- **Severity:** major — returned JSON numbers change before the dashboard receives them. Stored values are not changed.
- **Source:** user-requested continuous functional hosting QA, pass 51, 2026-10-10, using the designated muse.env login helper and filing to w4.
- **Scheduling:** extend w4/m157 with `/pm add-task`, as a sixth typed-value class; do not create a parallel decoder milestone. New implementation and closing checks are t010–t013. Existing completed tasks remain historical evidence for their original classes.
- **Estimate:** 125m additional: consumer audit 30m, bounded decoding 45m, Simplify 15m, meaningful coverage 35m. Existing t003 owns adapter/live verification.
- **Repro:** create an owned private Free Postgres, open `/databases/<id>`, scroll to SQL console, and run the SELECTs below with writes disabled. Reload and repeat. The fixture used was `qa-20261010-loop-a51-sql`, `dpg-db5228cb8ulc73efu4c0`, PostgreSQL 18.
- **Expected:** JSON object/array numbers retain PostgreSQL's integer and fraction digits. GraphQL cells contain JSON text, including lowercase `null` and a quoted JSON string; SQL NULL remains a nullable cell rendered `NULL`. REST/MCP keep native JSON types. Whitespace and object key order are not acceptance requirements.
- **Actual:** two fresh-page HTTP 200 results change JSONB `9007199254740993` to `9007199254740992` and `12345678901234567890.123456789` to `12345678901234567000`. JSON null becomes GraphQL null and a JSON string loses its quotes. The identical values cast to text are exact. A separate SELECT reproduces rounding for `json` and a JSONB array element.
- **Strong controls:** the same sibling SELECT preserves native bigint `9007199254740993`, native numeric `12345678901234567890.123456789`, float `1.5`, false and SQL NULL. Duplicate column names, subsequent result-shape replacement, empty text, Unicode, and the Control+Enter execution shortcut also worked. These controls do not establish REST/MCP or unselected type behavior.
- **Evidence:** viewed result-table screenshot `.playwright-mcp/qa-pg-json-a51-results.png`; full pass ledger `.playwright-mcp/qa-pg-a51-ledger.json`; `.playwright-mcp/qa-pg-a51-console.txt` has zero warnings/errors before logout. The earlier `qa-pg-json-a51-values.png` shows the editor above the result and is not result evidence. The exact probes and complete responses below are durable; ignored screenshots are supplementary.

## Root cause and consumer path

1. `lego/backend/internal/postgres/query.go:297` calls `rows.Values()` after raw budgets. Pinned pgx v5.10.0 `rows.go:294–321` looks up each field OID and calls its codec's `DecodeValue`.
2. Pinned `pgtype/pgtype_default.go:68–69` installs encoding/json.Unmarshal for JSON and JSONB. `pgtype/json.go:235–243` and `jsonb.go:105–134` unmarshal into `any`: object numbers become float64, JSON null becomes nil, and JSON strings become ordinary Go strings. The precision is already lost here; formatting the float later cannot recover it.
3. `query_values.go:119–161` preserves finite decoded floats and defaults to the decoded value. Its exact Numeric branch at :129–138 explains why native numeric is an effective control. The collector's `json.Marshal(out.Rows)` at `query.go:321` faithfully serializes already altered values; the budget is not the cause.
4. `graphql.go:356–395` projects each value through `queryCellString`: nil becomes a nullable cell, ordinary string is returned unquoted, and objects are JSON-marshaled. `DatabaseQueryRow.values` uses `gqlutil.StrsField` (:342), which declares [String] (`gqlutil.go:121–122`), so exact JSON text and actual SQL null are both expressible without a schema change. graphql-go v0.8.1 `scalars.go:323` uses its String coercer. The dashboard hook `use-execute-database-query.ts:33–40` copies those strings/nulls; `sql-console.tsx:198–213` displays them directly and substitutes `NULL` for null/undefined. No client numeric parser caused the captured change.
5. JSONB[] is separately traced: `query.go:301–309` re-decodes array bytes via `query_values.go:62–72`, scanning into pgtype.Array[any]. The pinned default array registrations at `pgtype_default.go:152–153` retain element Type pointers. The array scanner reaches the JSON codec's `PlanScan` (`json.go:139–169`) and `scanPlanJSONToJSONUnmarshal.Scan` (:199–222), which invokes the same Unmarshal into an any element. Preserving array dimensions does not preserve JSON number digits.
6. JSONB binary format requires its version byte to be removed/validated; pinned `jsonb.go:105–128` and its PlanScan wrapper (:44–83) do this. Do not copy a binary JSONB field blindly into a JSON payload.

## Fix contract and bounded scope

Preserve the server's JSON representation before generic float decoding. A query-connection-local JSON/JSONB codec that returns an owned json.RawMessage for a non-SQL-NULL any target can retain exact numeric tokens and scalar identity. Keep SQL NULL as nil. Register or otherwise update both JSON/JSONB array element codecs too: re-registering only the scalar OID leaves the default array codec's existing ElementType pointer unchanged. Handle text and binary JSONB via the codec, and copy bytes because the row buffer is reused.

This is a proposed implementation, not a shipped fix. An ignored local prototype with the actual pinned driver (`.playwright-mcp/qa-a51-codec-proof.go` / `qa-a51-codec-proof.txt`) successfully decoded the exact object, JSON null, quoted Unicode string and a mixed JSONB array without floats. Text and binary JSONB both worked. On the selected Go 1.27 toolchain json.RawMessage prints as its jsontext.Value alias; it remains a distinct JSON-marshaled value, not a bare bytea []byte.

The existing GraphQL helper's JSON-marshaling branch can render RawMessage as JSON text while its string branch still leaves SQL text unchanged. REST and the exact-JSON MCP registration marshal RawMessage as native JSON, retaining numbers rather than stringifying the result. The encoded result cap must count this lossless output; keep raw cell/row/result/column caps and row truncation. Do not broaden this into global driver/pool changes, change the schema or SQL input policy, convert all numbers into strings, or alter timeout/transaction behavior.

## Shared consumers and aliases

Searches excluded \*\_test.go definitions when counting production callers:

- `collectQueryRows`: one production caller, `runSQLQuery` at `query.go:395`.
- `runSQLQuery`: two production entry paths — the selected default executor in `executeAuthorizedQuery` (:218) and `runReadOnlyQuery` (:259).
- `executeAuthorizedQuery`: two public core callers, Query and ExecuteQuery. Query is MCP `query_render_postgres` (`mcp.go:157–160`); ExecuteQuery serves REST `POST /v1/postgres/{id}/query` (`rest.go:251–276`) and GraphQL `executeDatabaseQuery` (`graphql.go:691–695`), including confirmed writable RETURNING locally. No separate legacy SQL-query alias was found.
- `runReadOnlyQuery`: one production caller, `runAuthorizedInsight` (`insights.go:419`). That helper has four call sites: runInsight, TopQueries, ParameterOverrides, and ParameterSpec observation. runInsight has four call sites: Processes, Sizes' database-size and table-size reads, and TableScans. Thus seven built-in SQL executions across six public insight methods share the executor. Their existing text/integer/float consumers (`insights.go:424–467`) must remain unchanged; the new conversion is allowlisted to JSON/JSONB, including their native array elements.
- SQLConsole: one production mount, `routes/databases.$databaseId.tsx:272`, using the canonical database detail route. The guessed /insights subroute was a harness mistake; it is not a finding.
- `queryArrayColumns`, `decodeQueryArray` and `normalizeQueryValue` each have one external production caller in the collector; normalizeQueryValue also recurses into []any.
- Existing MCP `mcputil.AddExactJSONTool` (`mcputil.go:84–105`) marshals QueryResult once and sends the same raw bytes in both response forms. This m158 fix cannot reconstruct values already rounded by pgx.

There are no service-family SQL-console siblings for web, static, cron, worker or private services; Key Value has a different command surface. Do not apply the JSON codec change to those families.

## Adjacent behavior and unverified work

- JSON-null and SQL-NULL elements of a native JSON array both naturally encode as JSON null on REST/MCP; this filing does not add type metadata to distinguish them there. A top-level GraphQL JSON-null cell must instead be the non-null string `null`, distinguishable from an actual nullable SQL-NULL cell.
- Preserve regular SQL text, bytea/base64, booleans, finite floats, bigint/Numeric digits, UUIDs, temporal tokens and reconstructed native array dimensions. Preserve current error outcomes; this is a successful-result conversion change, not error classification work.
- **Live observed:** GraphQL/dashboard JSONB twice from fresh loads, json and JSONB[] once, native numeric controls, fixture deletion and own session cleanup.
- **Not live exercised in this pass:** REST and MCP JSON queries, JSON[] and multi-dimensional JSON arrays, writable RETURNING, bytea/time/UUID/special-number cases, row/byte cap boundaries, built-in insight projections. t003/t010/t013 must verify these rather than presenting them as observed.
- **Render:** the repository's ADR009/ADR018 records Render's read-only MCP query tool and the bex console as a deliberate extension. Vendor JSON numeric/scalar representations were not measured. Native PostgreSQL's text casts are the correctness oracle for this report.

## Dedupe and prior definition-of-done walk

All open/blocked/done PM files were searched for JSONB, JSON precision/rounding, JSONCodec and JSON scalar. Fifteen matching paths were inspected for their matching scope. Relevant ownership is m157 and m158; other matches describe datastore schema, webhook/transcript storage, native-array controls, deploy APIs or unrelated JSON reserialization. The nine open milestones across workstreams, full DO_NOT_DO, recent 40 product commits and targeted JSONCodec/normalizeQueryValue/query_values.go history were reviewed. No anti-goal or newer lossless JSON query fix applies. `9987710ad` implemented m157's prior five classes, with no JSON-preserving codec.

Extend m157 because it owns the shared typed-value boundary. Its array finding explicitly limits its JSON control to small int/text/null values and disclaims arbitrary large JSON precision; completed t009 excludes JSON-number work outside existing owners. This follow-up broadens its bounded contract as new work; it does not falsely claim the original five-class guarantee regressed. Preserve each original DoD item:

| Original m157 DoD | Pass 51 evidence / follow-up |
| --- | --- |
| 2D/3D integer and 2D text/null shape; flat/empty/SQL-NULL arrays; bytea | Not re-run. JSONB[] rounding has its own scanner trace; retain all original shape and bytea checks in t003/t013. |
| Canonical UUID scalar/array; NULL/bytea/int-array controls | Not re-run; leave original acceptance intact. |
| Temporal infinity markers and finite-time/integer controls | Not re-run; leave original acceptance intact. |
| real/double NaN/infinities and signed numeric infinities | Not re-run; JSON number precision is an additional class, not a replacement for this check. |
| Bare scalar special-token text and mixed arrays | Not re-run; JSON quoted strings use the JSON path and must not add quotes to existing special-number strings. |
| Finite float, exact numeric/bigint, null/text and ordinary arrays | Native float/bigint/numeric/false/SQL NULL and text-cast JSON controls pass in the complete sibling response below. Other adapters/types remain to verify. |
| Normalization before encoded budget, raw caps, truncation, transaction/timeout/errors, local write RETURNING | Source traced; boundary and local-write behavior not exercised here, retained for coverage. |
| Meaningful decoder/adapter tests plus live probes and cleanup | Prototype demonstrates expressibility, not product regression coverage. Owned fixture and session cleanup completed; original adapter/live acceptance stays open. |

m158's four DoD items remain separate: native bigint/numeric exact bytes (console controls pass here; MCP/REST not re-run), signed array/fraction controls (not re-run), stable tool shape and execution behavior (source inspected; no change requested), meaningful composed tests/live fixture cleanup (not claimed completed for m158). Its post-core SDK mechanism is distinct from this earlier pgx JSON loss, so no duplicate MCP serializer task is filed.

## Cleanup

Only the owned private Free database was created, without tables or data writes. It was deleted at 11:39:49.773Z; GET returned 404 at 11:40:04.701Z. Complete service/database/Key Value/project/environment-group/Blueprint ID sets match the initial baseline (5/4/2/3/1/1). Pre-existing resources, including other workers' QA fixtures, were left unchanged. The login helper returned `ok logged-out`; the old browser cookie then returned 401 from whoami at 11:40:21.564Z, followed by zero cookies, about:blank and the standard viewport. Only this pass's session files were removed.

## Durable requests and complete responses

The exact GraphQL request bodies include the dashboard's Apollo extensions. POST target is https://api.bex.co/graphql, content type application/json, using the normal signed-in browser session. Replace the deleted fixture ID with a new owned Free database; never reuse a pre-existing resource for mutable work.

### Owned Free fixture

```json
{
  "time": "2026-10-10T11:27:29.544Z",
  "request": [
    {
      "operationName": "CreateDatabase",
      "variables": {
        "name": "qa-20261010-loop-a51-sql",
        "databaseName": "qa_a51_data",
        "databaseUser": "qa_a51_owner",
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
  "response": [
    {
      "data": {
        "createDatabase": {
          "__typename": "Database",
          "databaseName": "qa_a51_data",
          "databaseUser": "qa_a51_owner",
          "environmentId": null,
          "id": "dpg-db5228cb8ulc73efu4c0",
          "name": "qa-20261010-loop-a51-sql",
          "plan": "free",
          "projectId": null,
          "status": "creating"
        }
      }
    }
  ]
}
```

### First fresh-page reproduction

```json
{
  "time": "2026-10-10T11:31:52.952Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db5228cb8ulc73efu4c0",
        "sql": "SELECT '{\"n\":9007199254740993,\"amount\":12345678901234567890.123456789}'::jsonb AS typed_object, '{\"n\":9007199254740993,\"amount\":12345678901234567890.123456789}'::jsonb::text AS text_object, 'null'::jsonb AS typed_null, 'null'::jsonb::text AS text_null, '\"qa-a51 雪\"'::jsonb AS typed_string, '\"qa-a51 雪\"'::jsonb::text AS text_string, NULL::text AS sql_null",
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
  "response": [
    {
      "data": {
        "executeDatabaseQuery": {
          "__typename": "DatabaseQueryResult",
          "columns": [
            "typed_object",
            "text_object",
            "typed_null",
            "text_null",
            "typed_string",
            "text_string",
            "sql_null"
          ],
          "rowCount": 1,
          "rows": [
            {
              "__typename": "DatabaseQueryRow",
              "values": [
                "{\"amount\":12345678901234567000,\"n\":9007199254740992}",
                "{\"n\": 9007199254740993, \"amount\": 12345678901234567890.123456789}",
                null,
                "null",
                "qa-a51 雪",
                "\"qa-a51 雪\"",
                null
              ]
            }
          ],
          "truncated": false
        }
      }
    }
  ]
}
```

### Second fresh-page reproduction

```json
{
  "time": "2026-10-10T11:32:18.558Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db5228cb8ulc73efu4c0",
        "sql": "SELECT '{\"n\":9007199254740993,\"amount\":12345678901234567890.123456789}'::jsonb AS typed_object, '{\"n\":9007199254740993,\"amount\":12345678901234567890.123456789}'::jsonb::text AS text_object, 'null'::jsonb AS typed_null, 'null'::jsonb::text AS text_null, '\"qa-a51 雪\"'::jsonb AS typed_string, '\"qa-a51 雪\"'::jsonb::text AS text_string, NULL::text AS sql_null",
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
  "response": [
    {
      "data": {
        "executeDatabaseQuery": {
          "__typename": "DatabaseQueryResult",
          "columns": [
            "typed_object",
            "text_object",
            "typed_null",
            "text_null",
            "typed_string",
            "text_string",
            "sql_null"
          ],
          "rowCount": 1,
          "rows": [
            {
              "__typename": "DatabaseQueryRow",
              "values": [
                "{\"amount\":12345678901234567000,\"n\":9007199254740992}",
                "{\"n\": 9007199254740993, \"amount\": 12345678901234567890.123456789}",
                null,
                "null",
                "qa-a51 雪",
                "\"qa-a51 雪\"",
                null
              ]
            }
          ],
          "truncated": false
        }
      }
    }
  ]
}
```

### JSON sibling failure and native numeric controls

```json
{
  "time": "2026-10-10T11:38:54.681Z",
  "request": [
    {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db5228cb8ulc73efu4c0",
        "sql": "SELECT '{\"n\":9007199254740993}'::json AS typed_json, '{\"n\":9007199254740993}'::json::text AS text_json, ARRAY['{\"n\":9007199254740993}'::jsonb,'null'::jsonb,'\"qa-a51 雪\"'::jsonb,NULL::jsonb] AS jsonb_array, ARRAY['{\"n\":9007199254740993}'::jsonb,'null'::jsonb,'\"qa-a51 雪\"'::jsonb,NULL::jsonb]::text AS text_array, 9007199254740993::bigint AS native_bigint, 12345678901234567890.123456789::numeric AS native_numeric, 1.5::float8 AS native_float, false AS flag, NULL::text AS sql_null",
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
  "response": [
    {
      "data": {
        "executeDatabaseQuery": {
          "__typename": "DatabaseQueryResult",
          "columns": [
            "typed_json",
            "text_json",
            "jsonb_array",
            "text_array",
            "native_bigint",
            "native_numeric",
            "native_float",
            "flag",
            "sql_null"
          ],
          "rowCount": 1,
          "rows": [
            {
              "__typename": "DatabaseQueryRow",
              "values": [
                "{\"n\":9007199254740992}",
                "{\"n\":9007199254740993}",
                "[{\"n\":9007199254740992},null,\"qa-a51 雪\",null]",
                "{\"{\\\"n\\\": 9007199254740993}\",\"null\",\"\\\"qa-a51 雪\\\"\",NULL}",
                "9007199254740993",
                "12345678901234567890.123456789",
                "1.5",
                "false",
                null
              ]
            }
          ],
          "truncated": false
        }
      }
    }
  ]
}
```

### Owned fixture deletion

```json
{
  "time": "2026-10-10T11:39:49.773Z",
  "request": [
    {
      "operationName": "DeleteDatabase",
      "variables": {
        "id": "dpg-db5228cb8ulc73efu4c0"
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
  "response": [
    {
      "data": {
        "deleteDatabase": true
      }
    }
  ]
}
```

### Deletion readback and complete baseline inventory

```json
{
  "time": "2026-10-10T11:40:04.701Z",
  "gone": {
    "url": "https://api.bex.co/v1/postgres/dpg-db5228cb8ulc73efu4c0",
    "status": 404,
    "body": "{\"code\":\"NOT_FOUND\",\"error\":\"not found\",\"id\":\"not_found\",\"message\":\"not found\",\"params\":null}\n"
  },
  "inventory": {
    "request": {
      "operationName": "QaA51Inventory",
      "query": "query QaA51Inventory($ownerId:String!){services(ownerId:$ownerId){id name type} databases(ownerId:$ownerId){id name} keyValues(ownerId:$ownerId){id name} projects(ownerId:$ownerId){id name} envGroups(ownerId:$ownerId){id name} blueprints(ownerId:$ownerId){id name}}",
      "variables": {
        "ownerId": "tea-d98210cbbpdc73dcrkvg"
      }
    },
    "status": 200,
    "response": {
      "data": {
        "blueprints": [
          {
            "id": "blp-d9nqg95cavls73fp8m10",
            "name": "discourse_docker"
          }
        ],
        "databases": [
          {
            "id": "dpg-d9nqg95cavls73fp8m20",
            "name": "beancount-forum-db"
          },
          {
            "id": "dpg-d9rrkoc4h4mc73edurp0",
            "name": "tianpan-forum-db"
          },
          {
            "id": "dpg-d9rs3ee0ccis738kc7c0",
            "name": "blockeden-forum-db"
          },
          {
            "id": "dpg-db34l9bor55s73cqpq9g",
            "name": "qa-20261007-r16-owner-f208ab"
          }
        ],
        "envGroups": [
          {
            "id": "evg-db0s94c48ccs739jikr0",
            "name": "qa-20261003-keys-r85"
          }
        ],
        "keyValues": [
          {
            "id": "red-d9p49kdrtmes73c34ovg",
            "name": "beancount-forum-redis"
          },
          {
            "id": "red-da4086iii7bs73drbqh0",
            "name": "blockeden-forum-redis"
          }
        ],
        "projects": [
          {
            "id": "prj-d9dgeo0bd9nc73a0vh1g",
            "name": "bex.co"
          },
          {
            "id": "prj-d9e5qct5qe4s73b1mjn0",
            "name": "beancount.io"
          },
          {
            "id": "prj-d9sgfnbjghus73cg6hg0",
            "name": "forums"
          }
        ],
        "services": [
          {
            "id": "srv-d9bkcspg9s7c73d0n8ug",
            "name": "agentmarketcap-1",
            "type": "web_service"
          },
          {
            "id": "srv-d9bj8s3eg85c7390eb9g",
            "name": "beancount-cms-v2",
            "type": "web_service"
          },
          {
            "id": "srv-d9nqg9dcavls73fp8m2g",
            "name": "beancount-forum",
            "type": "web_service"
          },
          {
            "id": "srv-d9ndt8hmcglc739fkp50",
            "name": "eden-dash-v3",
            "type": "web_service"
          },
          {
            "id": "srv-d9e40ei9086p3l1jri30",
            "name": "eden-cms-v2",
            "type": "web_service"
          }
        ]
      }
    }
  }
}
```

### Own session revocation and browser reset

```json
{
  "time": "2026-10-10T11:40:21.564Z",
  "url": "https://auth.bex.co/sessions/whoami",
  "status": 401,
  "cookiesRemaining": 0,
  "urlAfter": "about:blank"
}
```
