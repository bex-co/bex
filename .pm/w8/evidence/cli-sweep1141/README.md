# CLI sweep 1141 — PATCH rejects unknown and read-only fields by name (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404). No new bug filed.

- An unknown top-level field → 400 with `request body contains unknown field "nopeField"` (strict decoding, not silent ignore).
- Attempts to PATCH `id` or `createdAt` → 400, and read-back confirms both are unchanged.

Writers cannot smuggle undeclared fields or rewrite server-owned identity/timestamps.
