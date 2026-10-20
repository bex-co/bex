# CLI sweep 707 — instance listing across lifecycle (live, 2026-10-19)

Disposable free web service `qa707-web` (`srv-dbbbupavfpec73dm0l90`). Deleted and verified absent (REST 404).

- `bex services instances <sid>` during `creating` already listed the (single) instance with its `createdAt`; the same instance id persisted unchanged once the service went live — no churn across the rollout.
- Addressing by service **name** (`instances qa707-web`) resolved to the same list, unlike `bex restart` (sweep 665, ID-only).
- Earlier same-day recon: `services instances` on a Postgres id and on a synthetic service id both fail with 404 (datastores have no instance list here).

No bug filed.
