# CLI sweep 800 — rollback/cancel/get on synthetic deploy ids (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `POST /services/{id}/rollback {"deployId":"dep-000…"}` → 404 coded `deploy not found` (named, not a bare 500).
- Rollback with an empty body → 400 `invalid request body at /deployId` (required field named).
- Cancel and GET on the synthetic deploy id → the same named 404.
