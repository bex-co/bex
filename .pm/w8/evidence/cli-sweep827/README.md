# CLI sweep 827 — free Postgres quota gate (live, 2026-10-20)

No fixture created (create refused; nothing to clean). No new bug filed.

- `bex postgres create --plan free` in canary → 400 coded `WORKSPACE_RESOURCE_LIMIT`: "workspace is limited to 1 Postgres database; delete an existing Postgres database to create another". The one free slot is held by the baseline `qa-20261017-64552e-pg14`, which this loop never deletes — so the CLI Postgres create/delete cycle remains covered by REST evidence from earlier rotations, and the quota refusal itself is the live-observed behavior here: named, coded, and actionable.
