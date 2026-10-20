# CLI sweep 1027 — Idempotency-Key is per-endpoint, not global (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- Repeating a service create with the same `Idempotency-Key` header does **not** replay the first response: the second call runs into the name-conflict 409. Idempotency-Key handling is scoped to the endpoints that specify it (webhook resend, sweep 1026); the service-create contract in the pinned schema does not promise replay, so this is recorded, not filed.
