# CLI sweep 838 — webhook delivery-history read surface (live, 2026-10-20)

Two disposable disabled webhooks created, read, deleted (204s, then GETs 404). No new bug filed.

- Webhook single GET returns the endpoint without the signing secret (secret is creation-response-only; sweep 791).
- Delivery history lives at `GET /webhooks/{id}/events` (the pinned spelling — my `/deliveries` probe's 404 was a wrong-path harness error): a fresh disabled endpoint → 200 `[]`, matching the `[{webhookEvent, cursor}]` history contract in [webhooks-api.md](../../../docs/render-artifacts/webhooks-api.md).
