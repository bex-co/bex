# m52 — `GET /v1/services` list filter parity: `?type=`, `?suspended=`, time windows

**Status:** DONE 2026-07-16

## Problem

`GET /v1/services` only honored `?name=`, `?ownerId=`, and `?environmentId=`.
Render's OpenAPI also documents `?type=` (serviceType enum), `?suspended=`, and
four time-window params (`?createdBefore/After`, `?updatedBefore/After`).

The Render CLI uses `?type=background_worker` when the caller runs
`render services --type background_worker`, so missing this filter meant bex
returned all service types instead of the requested subset.

All the data was already in `AppView` (fields `Type`, `Suspended`, `CreatedAt`,
`UpdatedAt`); the filter block in `apps/rest.go` just had no code for them.

## What shipped

- **`?type=`** — OR-filter over `effectiveType(a.Type)` following the existing
  `?name=`/`?environmentId=` pattern; repeated and comma-separated values both
  work (`renderListParam`).
- **`?suspended=`** — boolean string `true`/`false`; unknown value → named 400
  (`suspended must be true or false`); absent means unfiltered.
- **`?createdBefore`/`?createdAfter`/`?updatedBefore`/`?updatedAfter`** —
  RFC3339, named 400 on malformed; empty `AppView.CreatedAt`/`UpdatedAt` (legacy
  Apps without stored timestamps) passes any window — same rule as
  `matchesTimeWindow` in envgroups (w2/m51).
- **`time` import** added to `apps/rest.go`.
- **Tests** (`apps_test.go`):
  - `TestRESTListTypeFilter` — single, comma-separated, and repeated `?type=`
    values; verifies OR semantics and correct mapping through `effectiveType`.
  - `TestRESTListSuspendedFilter` — `?suspended=true`, `?suspended=false`,
    `?suspended=maybe` → 400.
  - `TestRESTListTimeFilters` — malformed timestamp → 400; empty `AppView`
    timestamps pass the window (legacy rule).
- **ADR018** "List services" row updated with filter parity note; gap-backlog
  row added.

## Commit

`feat(apps): add ?type=, ?suspended=, and time-window filters to GET /v1/services (w2/m52)`
