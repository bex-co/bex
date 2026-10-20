# CLI sweep 806 — notification override omits required `serviceId` → filed w8/138 (live, 2026-10-20)

Disposable free web service created, override cycled and reset to `default`, service deleted (204, then GET 404). One bug filed: [w8/138](../../138.md).

- `GET`/`PATCH /notification-settings/overrides/services/{id}` → 200 with only `notificationsToSend` + `previewNotificationsEnabled`; the pinned Render schema requires `serviceId` as well. The workspace list on the same surface includes `serviceId` per row — the single-object struct (`lego/backend/internal/apps/rest.go:1448`) was never given the field.
- Values themselves behave: PATCH `failure` applies, bogus enum → 400 `invalid request body at /notificationsToSend`, reset to `default` round-trips.
