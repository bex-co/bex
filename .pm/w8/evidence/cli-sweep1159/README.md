# CLI sweep 1159 — notification overrides live under `/notification-settings` (live, 2026-10-20)

Two disposable free web services created and deleted (204, verified 404). No new bug filed.

- The guessed nested route (`/services/{id}/notification-override`) → 404. The declared surface is `/v1/notification-settings/overrides/services/{id}`.
- A fresh service's override reads 200 with `serviceId` equal to the path id plus `notificationsToSend: default` and `previewNotificationsEnabled: default` — the w8/138 fix (missing `serviceId`) still holds at the current pin.
- PATCHing `previewNotificationsEnabled` with a boolean → 400 at that JSON path (the field speaks the enum vocabulary, not booleans).
- After service DELETE, the override read → 404: overrides do not outlive their service.
