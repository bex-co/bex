# CLI sweep 795 — autoscaling gating on a free service (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `PUT /services/{id}/autoscaling` with `enabled:true` on a free plan → named 400 `autoscaling requires a paid instance type` — fail-closed before any write. The plan gate fires before the min/max validation (an inverted `min:5,max:2` body gets the same plan refusal), which is acceptable ordering: both are named 400s and nothing is stored.
- `DELETE /services/{id}/autoscaling` with no config present → 204 (idempotent revert path).
