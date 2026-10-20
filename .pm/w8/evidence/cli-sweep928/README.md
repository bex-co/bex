# CLI sweep 928 — GraphQL protocol discipline (live, 2026-10-20)

Read-only/refused GraphQL probes. No new bug filed.

- Unknown mutation field → 200 with `errors[]` + locations (GraphQL convention, as sweep 891).
- Variables work but the schema has no `ID` scalar: `$id: ID!` → `Unknown type "ID"` / `cannot be non-input type` — ids are `String` in this schema; clients generated against a generic GraphQL ID assumption get a precise schema error, not a silent coercion.
- A malformed-JSON GraphQL body → plain 400 `bad request` in the REST envelope.
