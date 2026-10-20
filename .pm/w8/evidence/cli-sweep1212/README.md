# CLI sweep 1212 — workflows empty state is the pinned CLI's `null` (live, 2026-10-20)

Read-only CLI sweep. No new bug filed.

- `bex workflows list -o json` on the empty workspace prints `null` and exits 0 — the pinned Render CLI's own empty-list rendering (same as `projects -o json`), already recorded as an upstream quirk of the oracle CLI rather than a bex defect, so it stays unfiled.
- `bex jobs` exposes create/list/cancel subcommands against the API surface that refuses one-off jobs with the coded 410 (sweep 1145).
