# CLI sweep 1154 — audit log records this session's own actions (live, 2026-10-20)

Read-only sweep. No new bug filed. No credential values recorded here.

- `GET /owners/{id}/audit-logs?limit=5` → 200 in the `{auditLog, cursor}` envelope. The newest entries are this session's own work — including the audit-list call itself (`audit.List`, `status: success`) and the sweep 1153 service deletion (`DeleteServerEvent`) — each with timestamp, actor (type `rest_api`, the QA user's id), and workspace resource. The trail is current, not batched or delayed.
- The same route under a non-member owner id → 403, matching the owners boundary from sweep 1147.
