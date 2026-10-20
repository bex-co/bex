# CLI sweep 893 — `notificationsToSend` writes live on the overrides subresource (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `PATCH /services/{id} {"notificationsToSend":"failure"}` → 400 `unknown field "notificationsToSend"` — by design: the read object exposes the dual fields (`notificationsToSend` + legacy `notifyOnFail`, both `default` here), but the single write path is `/notification-settings/overrides/services/{id}` (sweep 806, `rest.go:1446`), so the two spellings can never be written through competing endpoints. `notifyOnFail` remains PATCH-writable on the service itself (sweep 867).
