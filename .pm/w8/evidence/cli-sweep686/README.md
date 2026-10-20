# CLI sweep 686 — Postgres export gate (live, 2026-10-19)

Baseline Postgres only; nothing created (export list stayed `[]`).

- `POST /postgres/{id}/export` with a `{}` body → 400 `request body is not allowed for this operation` (no-body operation, validator-level).
- With no body on the free baseline → 400 `"dpg-…" has no backup store; exports are unavailable` — the plan-durability gate names the database and the reason, matching ADR009 (free is ephemeral, no backup store).

No bug filed.
