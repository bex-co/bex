# CLI sweep 964 — webhook endpoint CRUD and show-once secret (live, 2026-10-20)

One disposable service (deleted 204) and one webhook endpoint (deleted 204; GET 404; no `qa964` leftovers in service or webhook lists). No new bug filed.

- Two initial 400s were harness errors, corrected against the handler source: the create body is the flat Render shape `{ownerId, name, url, eventFilter: […], enabled}` — not nested filter objects and not `eventTypes`.
- With the correct shape: create → 201, and the signing secret is present **only** in that create response. A follow-up GET returns keys `enabled, eventFilter, id, name, url` — the secret is never re-echoed (show-once discipline verified live).
- PATCH disable → 200; DELETE → 204; GET after → 404.
- `GET /webhooks/event-types` returns the full event vocabulary catalog read-only.
