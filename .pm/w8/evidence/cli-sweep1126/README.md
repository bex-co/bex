# CLI sweep 1126 — Accept ignored; lifecycle verbs require empty bodies (live, 2026-10-20)

Two disposable free web services created and deleted (204, verified 404). No new bug filed.

- `Accept: application/xml`, `text/csv`, `application/json; q=0`, and `*/*; q=0` on a service read all → 200: content negotiation is not performed; the API always speaks JSON.
- `POST /suspend` with a `{}` JSON body → 400, service stays `not_suspended`. The same verb with an empty/absent body → 202 with `suspended: suspended` and `suspenders: ["user"]` on read-back.
- Repeating the empty-body suspend on an already-suspended service → 202 again (idempotent), and DELETE on the suspended service → 204.

Verb endpoints accept no payload at all — mirroring the deploy-cancel body discipline found earlier — rather than ignoring an unexpected one.
