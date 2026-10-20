# CLI sweep 791 — webhook endpoint CRUD cycle (live, 2026-10-20)

Disposable webhook (disabled, synthetic `example.com` URL) created, patched, deleted (204, then GET 404). No new bug filed.

- Create without `eventFilter` → named 400 `WEBHOOK_EVENT_FILTER_REQUIRED` with `params.field` — fail-closed by name, matching the Render-schema allowlist (`create-webhook: ownerId, enabled, eventFilter`).
- Create with `eventFilter:["deploy_started"]` → 201; the signing secret is returned once at creation (value not recorded) and is absent from the PATCH response.
- PATCH rename → 200 with the filter preserved; PATCH with a bogus event type → 400 `invalid request body at /eventFilter/0` (validator names the element).
- DELETE → 204; GET after → 404.
