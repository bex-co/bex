# Finite PostgreSQL years outside JSON time bounds fail valid queries

Why: a single supported date in an otherwise valid SQL result currently prevents the customer from reading the entire result through the console or query API.

**Severity:** major, bounded to result sets containing finite temporal years outside 0–9999. No database outage or data loss was observed. **Source:** functional QA a208, 2026-10-11 UTC, dashboard.bex.co, credentials handled privately by the login helper from muse.env. **Research pin:** `66bf81a61e4cd3173478863fcadb884626bb3289`. Add `t014` to existing w4/m157; filing only.

## Repro and controls

Owned Free private Postgres fixture `qa-20261011-a208-dates` (`dpg-db5s4pchj6ks739l0n8g`), server `18.4 (Debian 18.4-1.pgdg13+1)`, created 17:07:49 UTC.

1. Run the complete control SELECT below in SQL console. It returns year-10000 and BC literals as text, plus native year 9999, a 2D integer matrix, date infinities, UUID, finite float and SQL NULL correctly.
2. Run `SELECT DATE '10000-01-01' AS future_date;`. Desktop at 17:10:05 returns GraphQL HTTP 200 with null executeDatabaseQuery and internal-error response; the console shows **internal error**.
3. Reload at 390×844, navigate with the SQL console section link, and repeat at 17:13:39. Same operation response/error; document width is 390. The initial locator timeout before activating this deferred section was a harness error, not a product finding.
4. The five separate read-only REST SELECTs below each return HTTP 500: native future DATE, BC DATE, future TIMESTAMP, future TIMESTAMPTZ and a native DATE array. The paired text/ordinary-date controls return HTTP 200.
5. A subsequent ordinary console SELECT at 17:15:13 returns its recovery marker, year 9999, matrix and null successfully. SQL execution and the database remain available.

[PostgreSQL 18's date/time contract](https://www.postgresql.org/docs/18/datatype-datetime.html) supports date years from 4713 BC to 5874897 AD and timestamp years through 294276 AD. These are valid native values. No authenticated Render query execution or vendor extended-year encoding was captured.

## Independent cause and bounded target

- Pinned **pgx v5.10.0** `pgtype/date.go:396–414`, `timestamp.go:352–369` and `timestamptz.go:354–371` return a finite decoded cell as **time.Time**; infinities instead use the modifier already fixed by t007. This case does not run that infinity branch.
- `lego/backend/internal/postgres/query_values.go:119–161` has no time.Time branch and returns the finite value unchanged; arrays recurse through the same policy at 151–159. `query.go:309` invokes normalization, then **321–324** marshals the complete rows and turns any encoding error into errQueryFailed.
- Installed Go 1.27 `src/time/format_rfc3339.go:65–73` checks that JSON/RFC3339 years fit exactly four digits. Its year-range error explains the whole-result failure. The production toolchain was not read. PostgreSQL has accepted the literal, as the exact text-cast controls independently prove; neither SQL syntax, connection failure nor an exceeded byte limit explains these captures.
- Apply a **query-local time.Time case before the encoded-result budget**, including existing array recursion. Keep year 0–9999 as their current values/encoding. For finite years outside that range, emit an explicit signed expanded Gregorian year string, preserving month/day, time, fractional precision and offset, instead of handing the value to time.Time.MarshalJSON. Target examples: `+010000-01-01T00:00:00Z`, `+010000-01-01T12:34:56.123456Z`, and `-000001-01-01T00:00:00Z` for PostgreSQL 0002 BC (astronomical year -1). Use a sign and at least six year digits, allowing more digits for PostgreSQL's date range without truncation. This is an explicit Bex query-cell representation, not a claim that it is four-digit RFC3339 or that all JavaScript Date ranges accept it. Preserve the normal date/time representations and timezone semantics.
- No global pgx codec registration, normal-year rewrite, SQL text rewriting, null substitution, clipping to year 9999, date-to-infinity coercion or timestamp-to-number conversion. Update the policy comment/docs to include this newly observed class without rewriting prior completed acceptance.
- `QueryResult.Rows` already holds any-valued cells. `postgres/graphql.go:375–389` returns a string cell unchanged, while normal time.Time uses RFC3339Nano. The console at `dashboard/src/features/databases/components/sql-console.tsx:195–204` renders that string directly and reserves NULL for actual null/undefined. Thus a correctly normalized string clears the existing consumers rather than merely moving the marshal failure into an adapter. REST keeps the native JSON string; MCP's existing exact-JSON tool handles it through the shared result.

## Shared scope and verification

- Two service entry points, **Query** and **ExecuteQuery**, use the shared authorized executor; its default executor and runReadOnlyQuery use runSQLQuery, whose one collector call invokes normalization. The normalizer has two production call sites: the collector and its own array recursion.
- User entry points: REST `POST /v1/postgres/{id}/query`, GraphQL mutation `executeDatabaseQuery`, dashboard SQL console, MCP `query_render_postgres` (Query). There is no separate adapter workaround to fix. Writable RETURNING uses the same collector but was not exercised on production.
- The built-in Insights helper also uses runReadOnlyQuery. Exhaustive runAuthorizedInsight search finds four production call sites: the general insight read, TopQueries, parameter read, and parameter-save observation. Audit these and the existing seven built-in insight execution cases already assigned to t013; preserve their normal-time/scalar extraction contracts.
- The conversion is global within this **Postgres query collector**, allowlisted to out-of-range finite time.Time values. Web/static/cron/worker/private services and Key Value do not use this SQL result path. Do not change platform event timestamps, datastore metadata timestamps or the shared GraphQL scalar library.
- Native driver/executor tests must cover each of DATE/TIMESTAMP/TIMESTAMPTZ and BC/future bounds, native array recursion/dimensions, text/binary codec paths, normal-year boundaries including year zero, microseconds/offset preservation, SQL NULL, and existing infinity/numeric/UUID/bytea/JSON contracts. Assert the new values before encoded-budget accounting, and retain genuine raw/encoded-size errors and truncation.
- t003 must replay the captured cases through REST/GraphQL/dashboard and MCP after the change, using fresh owned fixtures and cleanup. t012/t013 include this follow-up. Missing, denied, unauthenticated, suspended/unavailable, timeout, SQL syntax and size-limit outcomes keep their existing paths; this task changes serialization of successful cells only. No such policy matrix was exercised here.
- **Unverified:** live MCP, native BC timestamp/timestamptz, multidimensional temporal arrays, year-zero and PostgreSQL extreme-range boundaries, non-UTC offsets, writable RETURNING, the proposed formatter, and the entire historical m157 DoD. Those are explicit t014/t003/t013 verification work; this pass does not close m157.

## Dedupe, goal and cleanup

Whole open/blocked/done board search and all 72 active README scans found no implemented finite-year correction. m157 temporal-finding.md explicitly left BC/far-future dates unverified, and completed t007 explicitly excluded their formatting. Extend that milestone rather than create a competing one. Its original infinity/UUID/2D-array controls pass here; JSON precision follow-up remains independent and open. Refreshed last-40 product history and targeted normalization history retain this gap at HEAD, not a landed correction awaiting deployment. DO_NOT_DO has no conflicting anti-goal.

This advances ADR009 managed-database operability and ADR006 shared query contracts under ADR008 dependable hosting. The new t014 is **50m**, with existing parity/simplify/coverage closeout tasks extended and prior completed work preserved.

Complete local evidence: `.playwright-mcp/qa-sql-finite-years-a208-ledger.json`; viewed screenshots `.playwright-mcp/qa-sql-far-date-a208-desktop.png` and `.playwright-mcp/qa-sql-far-date-a208-mobile.png`. These ignored files supplement the complete durable probes below. Deleted the owned database at 17:15:37; REST 404 at 17:16:39; all six inventories exactly match baseline. Own session logout succeeded, owners returned 401 at 17:16:56, browser is one blank page with zero cookies, and exact own session files were removed. No production data writes, password/connection reveal or foreign resource mutations were performed.

## Complete durable probes

```json
{
  "desktop": {
    "at": "2026-10-11T17:10:05.792Z",
    "status": 200,
    "request": {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db5s4pchj6ks739l0n8g",
        "sql": "SELECT DATE '10000-01-01' AS future_date;",
        "allowWrites": false
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation ExecuteDatabaseQuery($id: String!, $sql: String!, $allowWrites: Boolean) {\n  executeDatabaseQuery(id: $id, sql: $sql, allowWrites: $allowWrites) {\n    columns\n    rows {\n      values\n      __typename\n    }\n    rowCount\n    truncated\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "executeDatabaseQuery": null
      },
      "errors": [
        {
          "message": "internal error",
          "locations": [
            {
              "line": 2,
              "column": 3
            }
          ],
          "path": ["executeDatabaseQuery"]
        }
      ]
    }
  },
  "freshMobile": {
    "at": "2026-10-11T17:13:39.349Z",
    "status": 200,
    "request": {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db5s4pchj6ks739l0n8g",
        "sql": "SELECT DATE '10000-01-01' AS future_date;",
        "allowWrites": false
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation ExecuteDatabaseQuery($id: String!, $sql: String!, $allowWrites: Boolean) {\n  executeDatabaseQuery(id: $id, sql: $sql, allowWrites: $allowWrites) {\n    columns\n    rows {\n      values\n      __typename\n    }\n    rowCount\n    truncated\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "executeDatabaseQuery": null
      },
      "errors": [
        {
          "message": "internal error",
          "locations": [
            {
              "line": 2,
              "column": 3
            }
          ],
          "path": ["executeDatabaseQuery"]
        }
      ]
    }
  },
  "rest": [
    {
      "name": "far_date",
      "at": "2026-10-11T17:14:00.947Z",
      "request": {
        "method": "POST",
        "url": "https://api.bex.co/v1/postgres/dpg-db5s4pchj6ks739l0n8g/query",
        "body": {
          "sql": "SELECT DATE '10000-01-01' AS future_date;",
          "allowWrites": false
        }
      },
      "status": 500,
      "response": {
        "error": "internal error",
        "id": "internal_error",
        "message": "internal error"
      }
    },
    {
      "name": "bc_date",
      "at": "2026-10-11T17:14:01.049Z",
      "request": {
        "method": "POST",
        "url": "https://api.bex.co/v1/postgres/dpg-db5s4pchj6ks739l0n8g/query",
        "body": {
          "sql": "SELECT DATE '0002-01-01 BC' AS past_date;",
          "allowWrites": false
        }
      },
      "status": 500,
      "response": {
        "error": "internal error",
        "id": "internal_error",
        "message": "internal error"
      }
    },
    {
      "name": "far_timestamp",
      "at": "2026-10-11T17:14:00.692Z",
      "request": {
        "method": "POST",
        "url": "https://api.bex.co/v1/postgres/dpg-db5s4pchj6ks739l0n8g/query",
        "body": {
          "sql": "SELECT TIMESTAMP '10000-01-01 12:34:56.123456' AS future_timestamp;",
          "allowWrites": false
        }
      },
      "status": 500,
      "response": {
        "error": "internal error",
        "id": "internal_error",
        "message": "internal error"
      }
    },
    {
      "name": "far_timestamptz",
      "at": "2026-10-11T17:14:00.886Z",
      "request": {
        "method": "POST",
        "url": "https://api.bex.co/v1/postgres/dpg-db5s4pchj6ks739l0n8g/query",
        "body": {
          "sql": "SELECT TIMESTAMPTZ '10000-01-01 12:34:56.123456+00' AS future_timestamptz;",
          "allowWrites": false
        }
      },
      "status": 500,
      "response": {
        "error": "internal error",
        "id": "internal_error",
        "message": "internal error"
      }
    },
    {
      "name": "date_array",
      "at": "2026-10-11T17:14:01.080Z",
      "request": {
        "method": "POST",
        "url": "https://api.bex.co/v1/postgres/dpg-db5s4pchj6ks739l0n8g/query",
        "body": {
          "sql": "SELECT ARRAY[DATE '9999-12-31',DATE '10000-01-01'] AS dates;",
          "allowWrites": false
        }
      },
      "status": 500,
      "response": {
        "error": "internal error",
        "id": "internal_error",
        "message": "internal error"
      }
    },
    {
      "name": "text_and_finite_controls",
      "at": "2026-10-11T17:14:01.147Z",
      "request": {
        "method": "POST",
        "url": "https://api.bex.co/v1/postgres/dpg-db5s4pchj6ks739l0n8g/query",
        "body": {
          "sql": "SELECT DATE '9999-12-31' AS finite, DATE '10000-01-01'::text AS far_text, DATE '0002-01-01 BC'::text AS bc_text, TIMESTAMP '10000-01-01 12:34:56.123456'::text AS timestamp_text, TIMESTAMPTZ '10000-01-01 12:34:56.123456+00'::text AS timestamptz_text, NULL::date AS null_control;",
          "allowWrites": false
        }
      },
      "status": 200,
      "response": {
        "columns": [
          "finite",
          "far_text",
          "bc_text",
          "timestamp_text",
          "timestamptz_text",
          "null_control"
        ],
        "rows": [
          [
            "9999-12-31T00:00:00Z",
            "10000-01-01",
            "0002-01-01 BC",
            "10000-01-01 12:34:56.123456",
            "10000-01-01 12:34:56.123456+00",
            null
          ]
        ],
        "rowCount": 1,
        "truncated": false
      }
    }
  ],
  "controls": {
    "at": "2026-10-11T17:09:50.662Z",
    "status": 200,
    "request": {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db5s4pchj6ks739l0n8g",
        "sql": "SELECT current_setting('server_version') AS version,\nDATE '9999-12-31' AS ordinary_limit,\nDATE '10000-01-01'::text AS far_date_text,\nDATE '0002-01-01 BC'::text AS bc_date_text,\nTIMESTAMP '10000-01-01 12:34:56.123456'::text AS far_timestamp_text,\nTIMESTAMPTZ '10000-01-01 12:34:56.123456+00'::text AS far_timestamptz_text,\nARRAY[[1,2],[3,4]] AS matrix,\nARRAY['infinity'::date,'-infinity'::date] AS infinite_dates,\n'123e4567-e89b-12d3-a456-426614174000'::uuid AS uuid_control,\n1.5::float8 AS float_control,\nNULL::date AS null_control;",
        "allowWrites": false
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation ExecuteDatabaseQuery($id: String!, $sql: String!, $allowWrites: Boolean) {\n  executeDatabaseQuery(id: $id, sql: $sql, allowWrites: $allowWrites) {\n    columns\n    rows {\n      values\n      __typename\n    }\n    rowCount\n    truncated\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "executeDatabaseQuery": {
          "__typename": "DatabaseQueryResult",
          "columns": [
            "version",
            "ordinary_limit",
            "far_date_text",
            "bc_date_text",
            "far_timestamp_text",
            "far_timestamptz_text",
            "matrix",
            "infinite_dates",
            "uuid_control",
            "float_control",
            "null_control"
          ],
          "rowCount": 1,
          "rows": [
            {
              "__typename": "DatabaseQueryRow",
              "values": [
                "18.4 (Debian 18.4-1.pgdg13+1)",
                "9999-12-31T00:00:00Z",
                "10000-01-01",
                "0002-01-01 BC",
                "10000-01-01 12:34:56.123456",
                "10000-01-01 12:34:56.123456+00",
                "[[1,2],[3,4]]",
                "[\"infinity\",\"-infinity\"]",
                "123e4567-e89b-12d3-a456-426614174000",
                "1.5",
                null
              ]
            }
          ],
          "truncated": false
        }
      }
    }
  },
  "recovery": {
    "at": "2026-10-11T17:15:13.776Z",
    "status": 200,
    "request": {
      "operationName": "ExecuteDatabaseQuery",
      "variables": {
        "id": "dpg-db5s4pchj6ks739l0n8g",
        "sql": "SELECT 'qa-a208-recovered' AS marker, DATE '9999-12-31' AS normal_date, ARRAY[[1,2],[3,4]] AS matrix, NULL::date AS sql_null;",
        "allowWrites": false
      },
      "extensions": {
        "clientLibrary": {
          "name": "@apollo/client",
          "version": "4.1.3"
        }
      },
      "query": "mutation ExecuteDatabaseQuery($id: String!, $sql: String!, $allowWrites: Boolean) {\n  executeDatabaseQuery(id: $id, sql: $sql, allowWrites: $allowWrites) {\n    columns\n    rows {\n      values\n      __typename\n    }\n    rowCount\n    truncated\n    __typename\n  }\n}"
    },
    "response": {
      "data": {
        "executeDatabaseQuery": {
          "__typename": "DatabaseQueryResult",
          "columns": ["marker", "normal_date", "matrix", "sql_null"],
          "rowCount": 1,
          "rows": [
            {
              "__typename": "DatabaseQueryRow",
              "values": [
                "qa-a208-recovered",
                "9999-12-31T00:00:00Z",
                "[[1,2],[3,4]]",
                null
              ]
            }
          ],
          "truncated": false
        }
      }
    }
  }
}
```
