# CLI sweep 685 — Postgres backups/recovery dialects (live, 2026-10-19)

Read-only against the baseline Postgres (free plan).

- `GET /postgres/{id}/backups` → 404: no such route (Render has no backups-list endpoint; recovery state is the surface).
- Canonical `GET /postgres/{id}/recovery` → 200 `{"recoveryStatus":"NOT_AVAILABLE"}` — the honest no-backup state for a free database, per ADR018 (never a synthesized window). Synthetic id → 404 `NOT_FOUND`.
- The bex-native alias `GET /postgres/{id}/recovery-info` → plain-text `404 page not found`: it is on the validator's retired-alias list (`retiredPublicRESTAlias`), so it bypasses the JSON error shape by design.
- `GET /postgres/{id}/export` → 200 `[]` (no exports yet).

No bug filed.
