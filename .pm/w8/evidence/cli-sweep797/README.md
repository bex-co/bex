# CLI sweep 797 — suspend/resume idempotency (live, 2026-10-20)

Disposable free web service created, cycled, deleted (204, then GET 404). No new bug filed.

- `POST /services/{id}/suspend` → 202, GET shows `suspended`; suspend again → 202 again (idempotent, stays `suspended`).
- `POST /services/{id}/resume` → 202, GET shows `not_suspended`; resume again → 202 (idempotent).
- Final state matches the last command; no stuck or contradictory `suspended` value at any step.
