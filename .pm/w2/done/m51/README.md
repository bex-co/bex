# m51 — `matchesTimeWindow` passes legacy env groups with empty timestamps

**Status:** DONE 2026-07-16

## Problem

`matchesTimeWindow` in `lego/backend/internal/envgroups/service.go` returned
`false` on any `time.Parse` error. Legacy env groups created before w6/m24
added `createdAt`/`updatedAt` stamping have empty strings for those fields.
When a caller applied any time filter (`createdBefore`, `createdAfter`,
`updatedBefore`, `updatedAfter`), `time.Parse("")` failed and the legacy group
was silently excluded from the result — as if it had never existed.

This violated the codebase convention that omitted data is never treated as an
exclusion signal.

## What shipped

- **`matchesTimeWindow`** (`envgroups/service.go`): added an `if raw == ""`
  short-circuit that returns `true` before the parse. Empty timestamp → passes
  any window. An unparseable-but-non-empty raw string still returns `false`
  (genuine data corruption, not a legacy omission).
- **`TestREST_EnvGroupLegacyEmptyTimestampPassesTimeFilter`**
  (`envgroups/rest_test.go`): seeds a group with no timestamp keys in the store,
  then asserts it appears in both a `createdBefore` and an `updatedBefore`
  filtered list that would exclude any known-created group.
- **ADR018** env-groups row updated with w2/m51 note; gap-backlog row added.

## Commit

`fix(envgroups): matchesTimeWindow passes legacy groups with empty timestamps (w2/m51)`
