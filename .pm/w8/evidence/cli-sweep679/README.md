# CLI sweep 679 — workspace audit logs (live, 2026-10-19)

Read-only.

- `GET /owners/{canary}/audit-logs?limit=3` → 200 with recent entries (`aud-…` ids, actor `rest_api`, event/status/metadata) — the log is live and includes the QA session's own reads.
- `limit=0` → 400 `limit must be an integer between 1 and 1000` (this endpoint's bound is 1000, not the usual 100 — named either way).
- Synthetic owner id → 403 `FORBIDDEN` (existence not leaked).
- `startTime=banana` → 400 `invalid query parameter "startTime"`.

No bug filed.
