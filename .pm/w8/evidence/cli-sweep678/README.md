# CLI sweep 678 — webhook CRUD validation and lifecycle (live, 2026-10-19)

Fixture webhook `qa678-wh` (`whk-…`) deleted (204) and list-verified empty. Its one-time signing secret was returned at create only and is not recorded here.

`POST /v1/webhooks` refusal walk:

| Body | Result |
| --- | --- |
| `{}` | 400 `invalid request body at /name` |
| `eventFilter` as Render-object guess `{"types":[…]}` | 400 `invalid request body at /eventFilter` (bex takes a string array, matching the pinned schema's array form) |
| `eventFilter:["deploy_failed"]` | 400 `invalid request body at /eventFilter/0` — items validate against the pinned Render event-type enum (`deploy_ended`, `deploy_started`, …) |
| valid filter, no `enabled` | 400 `WEBHOOK_ENABLED_REQUIRED` |
| valid filter + `enabled`, no `ownerId` | 400 `WEBHOOK_OWNER_REQUIRED` (the validator relaxes Render's required `ownerId`; the handler re-imposes it, coded) |
| `url: http://…` | 400 `WEBHOOK_URL_INVALID: url must be an absolute HTTPS URL` |

Full valid body → 201 with `secret` issued once. `PATCH {"name":…}` → 200 partial rename (no secret re-issued). `DELETE` → 204. Contrast with sweep 676: here the Render schema and handler agree, and each layer's refusal is coded and named.

No bug filed.
