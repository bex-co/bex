# CLI sweep 805 — workspace audit-log read surface (live, 2026-10-20)

Read-only probes; nothing created. No new bug filed.

- `GET /owners/{tea}/audit-logs?limit=3` → 200 with real entries from this loop's own activity (`audit.List`, `DeleteServerEvent`, actor + relation metadata, cursor envelopes). No secret material in the metadata (scopes/relation only).
- Bogus owner id → coded 403 `FORBIDDEN` (existence not leaked as 404-vs-403 split for a foreign workspace).
- Future `startTime` window → 200 `[]`; malformed `startTime` → named 400 `invalid query parameter "startTime"`.
