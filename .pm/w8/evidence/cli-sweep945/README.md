# CLI sweep 945 — webhook PATCH discipline (live, 2026-10-20)

Disposable webhook created, patched, deleted (204, then GET 404). No new bug filed.

- PATCH `url` and `enabled` → 200 each, GET reflects both; the signing secret never re-appears on reads (creation-only, sweeps 791/838).
- A non-URL `url` → coded 400 `WEBHOOK_URL_INVALID` ("url must be an absolute HTTPS URL", `params.field`) — endpoint URLs are validated as absolute HTTPS at write, so a plaintext or malformed receiver cannot be configured silently.
