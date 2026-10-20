# CLI sweep 1054 — content negotiation is JSON-always (live, 2026-10-20)

One disposable free web service created and deleted (204). No new bug filed.

- `Accept: text/html`, `application/xml`, and `*/*` all receive `200 application/json`. The API is JSON-only and does not 406 — the standard posture for this API family; recorded, not filed.
