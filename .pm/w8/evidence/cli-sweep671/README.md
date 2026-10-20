# CLI sweep 671 — notification settings PATCH (live, 2026-10-19)

Disposable free web service `qa671-web` (`srv-dbbbi7tfsg2s73ako8d0`). Deleted and verified absent (REST 404).

- `PATCH {"notifyOnFail":"bogus"}` and `"yes"` → 400 `notifyOnFail must be one of default|notify|ignore`. `"notify"` → 200, read back as `"notifyOnFail":"notify"`.
- `PATCH {"notificationsToSend":…}` (any value, top-level or nested) → 400 `request body contains unknown field "notificationsToSend"`: the field is a read-only projection — after setting `notifyOnFail=notify` it read back as `"failure"` (from `"default"` before), derived server-side.

No bug filed.
