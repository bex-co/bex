# CLI sweep 1010 — API response headers (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- A service GET answers `200 application/json`. No `X-Request-Id` or `Cache-Control` header is present on the response. Render's pinned contract does not promise a request-id header on this surface, so the absence is recorded, not filed.
