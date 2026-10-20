# CLI sweep 841 — disk/snapshot synthetic-id discipline (live, 2026-10-20)

Read-only/refused probes; nothing created. No new bug filed.

- `GET /disks/{synthetic}` and `GET /disks/{synthetic}/snapshots` → coded 404 `disk not found` (named, not bare).
- `POST /disks/{synthetic}/snapshots/restore {}` → 400 `invalid request body at /snapshotKey` — the required field is validated by name before the disk lookup.
- `GET /disks?ownerId=…` → 200 `[]` (no disks in canary; disk creation is paid-gated, sweep 804).
