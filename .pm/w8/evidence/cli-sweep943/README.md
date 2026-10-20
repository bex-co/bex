# CLI sweep 943 — webhook delivery pipeline fires and records (live, 2026-10-20)

Disposable free web service + enabled webhook (synthetic `example.com` URL) created, deploy triggered, both deleted (204s). No new bug filed.

- Within ~20 s of the triggered deploy, `GET /webhooks/{id}/events` holds a real delivery: `whd-…` id, the source `eventId`, `eventType: "deploy_ended"`, `serviceName`, `sentAt`, the receiver's actual `statusCode` (405 from example.com), and a captured `responseBody` excerpt — the full at-least-once delivery + history contract from [webhooks-api.md](../../../docs/render-artifacts/webhooks-api.md) working end-to-end, not just CRUD (sweeps 791/838).
