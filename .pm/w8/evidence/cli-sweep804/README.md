# CLI sweep 804 — disk create gating on a free service (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `POST /disks {"sizeGB":1,"mountPath":"/data804",…}` on a free plan → named 400 `disks require a paid instance type` — fail-closed before any write, same paid-gate family as maintenance mode (sweep 794) and autoscaling (sweep 795).
- `GET /disks?serviceId=srv-000…` → coded 404 `NOT_FOUND` (the filter resolves the service first rather than returning an empty page for a nonexistent owner resource).
