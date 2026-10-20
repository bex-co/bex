# CLI sweep 1008 — GraphQL malformed-request handling (live, 2026-10-20)

Read-only GraphQL probes. No fixture created. No new bug filed.

- A truncated query → a positioned syntax error (`Expected Name, found EOF` with line/column and a source snippet); an empty query → `query is required`; an unknown field → `Cannot query field … on type "Query"`. All three answer in the standard GraphQL error envelope with `data: null` — no 500s, no stack traces.
