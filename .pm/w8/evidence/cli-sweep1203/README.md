# CLI sweep 1203 — CLI datastore commands mirror REST (live, 2026-10-20)

Read-only CLI sweep (baseline Postgres read, never mutated; no credential values recorded). No new bug filed.

- `bex postgres list -o json` → exit 0 with the baseline database (its `databaseName`/`databaseUser` identifier fields present, no password material); `bex keyvalues list -o json` → `{data: []}` matching the REST list.
- `bex postgres get <baseline>` → exit 0; `bex postgres get dpg-000…` → exit 1 with a workspace-scoped not-found message. A bare `bex postgres -o json` (no subcommand) prints help and exits 0 — the pinned CLI's noun/verb structure.
