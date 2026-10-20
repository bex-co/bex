# CLI sweep 750 — Postgres recovery guards (live, 2026-10-19)

Baseline Postgres only; nothing created (list still holds only the baseline).

- `POST /postgres/{synthetic}/export` and `/recovery` → 404 `NOT_FOUND`.
- `POST /postgres/{baseline}/recovery {"restoreName":…,"restoreTime":…}` on the free database → 400 `"dpg-…" has no backups to recover from` — the no-backup-store gate (sweep 685's `NOT_AVAILABLE` read side) refuses by name before any restore instance is stamped.
- Missing the new-instance name → 400 `name (the new instance) is required`; a future `restoreTime` hits the same backups gate first.

No bug filed.
