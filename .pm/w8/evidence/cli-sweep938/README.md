# CLI sweep 938 — rollback to the live deploy is a named conflict (live, 2026-10-20)

Disposable free web service created, deployed, probed, deleted (204, then GET 404). No new bug filed.

- `POST …/rollback {"deployId": <the only, live deploy>}` → coded 409 `deploy "…" is already live — nothing to roll back to` (the complementary case to sweep 800's synthetic-id 404: the target exists but is current, and the answer explains rather than queuing a no-op redeploy).
