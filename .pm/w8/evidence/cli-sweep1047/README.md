# CLI sweep 1047 — Postgres subresource reads on the baseline (live, 2026-10-20)

Read-only probes on the baseline Postgres; nothing mutated. No new bug filed.

- `GET /postgres/{id}/recovery` → 200 `{recoveryStatus: "NOT_AVAILABLE"}` (an honest state answer for a free database); `GET …/export` → 200 `[]` (no exports taken).
- Guessed `/backups` and `/events` subresources → 404 (backups are not a listable subresource here; database events ride the workspace surfaces probed earlier).
