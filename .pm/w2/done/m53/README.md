# m53 — `GET /v1/postgres` and `GET /v1/key-value` list filter parity

**Status:** DONE 2026-07-16

## Problem

`GET /v1/postgres` and `GET /v1/key-value` only honored `?name=` and `?ownerId=`.
Render's OpenAPI also documents `?suspended=`, `?environmentId=`, and four
time-window params (`?createdBefore/After`, `?updatedBefore/After`) for both
datastore list endpoints.

Additionally, Key Value's `?name=` filter only matched the display name
(`kv.Name`) and not the opaque `red-` id — while the Postgres handler already
had `|| slices.Contains(names, p.ID)` as a fallback.

## What shipped

### `postgres/rest.go`

- **`?environmentId=`** — OR-filter over `p.EnvironmentID` (which comes from
  `Labels[core.LabelEnvironment]`), repeated values are OR'd.
- **`?suspended=`** — string enum `"suspended"`/`"not_suspended"` using
  `core.RenderSuspended`/`core.RenderNotSuspended`; unknown value → named 400.
- **`?createdBefore`/`?createdAfter`/`?updatedBefore`/`?updatedAfter`** —
  RFC3339, named 400 on malformed; empty `PostgresView.CreatedAt`/`UpdatedAt`
  passes any window (legacy-CR rule).

### `keyvalue/rest.go`

- **`?name=` id fallback** — `slices.Contains(names, kv.ID)` added alongside
  the existing name check.
- **`?environmentId=`** — same pattern as Postgres.
- **`?suspended=`** — same string enum as Postgres.
- **`?createdBefore`/`?createdAfter`/`?updatedBefore`/`?updatedAfter`** —
  same time-window loop as Postgres.
- **`"time"` import** added to `keyvalue/rest.go`.

### Tests

`postgres/filter_rest_test.go` (new file):

- `TestRESTListPostgresSuspendedFilter` — `?suspended=suspended`,
  `?suspended=not_suspended`, `?suspended=true` → 400 (invalid enum value).
- `TestRESTListPostgresEnvironmentIDFilter` — single and multi-value
  `?environmentId=` OR semantics.
- `TestRESTListPostgresTimeFilters` — malformed timestamp → 400; empty
  `CreatedAt` on fake-client CRs passes any time window.

`keyvalue/filter_rest_test.go` (new file):

- `TestRESTListKeyValueSuspendedFilter` — same as Postgres suspended test.
- `TestRESTListKeyValueEnvironmentIDFilter` — same as Postgres env-id test.
- `TestRESTListKeyValueNameByIDFilter` — `?name=<id>` resolves the opaque
  `red-` id (the fix); `?name=<display-name>` continues to work.
- `TestRESTListKeyValueTimeFilters` — same as Postgres time-filter test.

### ADR018

- Postgres log history row updated with `w2/m53` filter-parity note.
- Key Value row updated with `w2/m53` filter-parity note.
- Gap-backlog row added for m53.
