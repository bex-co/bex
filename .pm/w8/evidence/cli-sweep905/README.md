# CLI sweep 905 — w8/138 spot-check at pin `3ede47d3c5a9` (live, 2026-10-20)

Disposable free web service created for the probe and deleted (204, then GET 404). No new bug filed.

- `GET /notification-settings/overrides/services/{id}` still returns only `notificationsToSend` + `previewNotificationsEnabled` — no `serviceId`. w8/138's fix (`d98da2870`) is not an ancestor of the current pin, so the finding stays open; this is the expected pre-fix answer, recorded so the eventual closure can diff against it.
