# CLI sweep 741 — per-service notification override (live, 2026-10-19)

On sweep 740's fixture before deleting it (verified absent, REST 404). The override was restored to `default` before deletion.

- `GET /notification-settings/overrides/services/{id}` → `{"notificationsToSend":"default","previewNotificationsEnabled":"default"}`.
- `PATCH {"notificationsToSend":"bogus"}` → 400 `invalid request body at /notificationsToSend`; `"none"` → 200, and the service object itself read back `"notificationsToSend":"none"` (the override projects onto the service, ADR018 w3/m15); restored to `"default"` → 200.

No bug filed.
