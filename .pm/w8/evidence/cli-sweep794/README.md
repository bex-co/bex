# CLI sweep 794 — maintenance mode gating on a free service (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `PATCH {"serviceDetails":{"maintenanceMode":{"enabled":true,"uri":…}}}` on a free plan → named 400 `maintenanceMode requires a paid web service plan` — fail-closed before any write (no paid transition attempted or made).
- The top-level `{"maintenanceMode":…}` spelling → 400 `unknown field "maintenanceMode"`; the field only lives under `serviceDetails`.
- GET confirms the fixture stayed `{"enabled":false,"uri":""}` throughout.
