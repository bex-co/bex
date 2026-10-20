# CLI sweep 817 — service preview endpoint is a rejected non-goal (live, 2026-10-20)

Disposable free image-backed web service created, probed, deleted (204, then GET 404; name-filter list confirms absence). No new bug filed.

- `POST /services/{id}/preview {}` (and with a body) → bare 404 `not found`. Expected: PR/service previews are a rejected anti-goal ([`.pm/DO_NOT_DO.md`](../../../DO_NOT_DO.md) — "Do not build PR preview environments", user decision 2026-07-12, `w2/m11` rejected), and [ADR029](../../../docs/ADR029-static-sites.md) repeats "PR Previews remains a rejected non-goal". The route is not served, consistent with the anti-goal.
