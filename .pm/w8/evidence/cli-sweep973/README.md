# CLI sweep 973 — audit logs under the owner path (live, 2026-10-20)

Read-only probes. No fixture created. No new bug filed.

- Audit logs live at `GET /owners/{ownerId}/audit-logs` (the flat `/audit-logs` path correctly 404s). The list returns timestamped entries with actor, event, status, and resource — including the audit-read event itself.
- An unknown owner id → coded 403 `FORBIDDEN` (same non-serving boundary as the owner detail in sweep 958); a non-integer `limit` → coded 400 naming the query parameter.
- Entry metadata carries scope/relation context but no credential material.
