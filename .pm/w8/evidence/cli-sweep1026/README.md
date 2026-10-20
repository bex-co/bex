# CLI sweep 1026 — webhook deliveries and resend idempotency gate (live, 2026-10-20)

One disposable webhook endpoint created and deleted (204). No service fixture. No new bug filed.

- `GET /webhooks/{id}/events` on a fresh endpoint → `200 []` (also without the owner query param — the endpoint id itself carries the scope for the owner-session caller).
- Resending an unknown attempt without an `Idempotency-Key` header → coded 400 `WEBHOOK_RESEND_IDEMPOTENCY_KEY_INVALID` spelling out the accepted key format (8–128 letters/digits/dots/underscores/colons/hyphens, `params.field=idempotencyKey`). The resend surface is idempotency-gated before the attempt lookup runs.
