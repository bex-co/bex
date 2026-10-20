# CLI sweep 812 — blueprint read surface (live, 2026-10-20)

Read-only probes; the pre-existing `bex` blueprint was only read, never synced, patched, or disconnected. No new bug filed.

- `GET /blueprints?ownerId=…` → 200 with the one pre-existing blueprint in a `{blueprint, cursor}` envelope (`status: created`, `autoSync: true`).
- `GET /blueprints/{id}` → 200 with repo/branch/path plus the stored manifest (owner-scoped read; the manifest is the owner's own IaC, not a secret surface).
- `GET /blueprints/{id}/syncs` → 200 `[]` (no sync runs recorded for it).
- Synthetic id → coded 404 `blueprint not found`.
