# CLI sweep 935 — autoscaling read of an unset config (live, 2026-10-20)

Disposable free web service created, read, deleted (204, then GET 404). No new bug filed.

- `GET /services/{id}/autoscaling` with nothing configured → 200 with the explicit zero object (`enabled:false, min:0, max:0`, both criteria disabled) — an honest "not configured" shape rather than a 404 a client could mistake for a missing service; the synthetic-service control → 404, so existence still gates first (the write side's paid gate is sweep 795).
