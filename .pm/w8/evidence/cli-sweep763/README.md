# CLI sweep 763 — API response headers (live, 2026-10-19)

Read-only `GET /v1/users`, header names only.

- Present: `Content-Security-Policy`, `Strict-Transport-Security`, `X-Content-Type-Options`, `X-Frame-Options`, `Server-Timing`, plus Cloudflare transport headers.
- Absent: rate-limit budget headers (`RateLimit-*` / `X-RateLimit-*`) — nothing exposes the caller's remaining quota on this surface, matching the pinned API's behavior.

No bug filed.
