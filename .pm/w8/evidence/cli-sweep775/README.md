# CLI sweep 775 — conditional-request headers (live, 2026-10-19)

Read-only against the baseline `hello-go`.

`GET /services/{id}` returns no `ETag`; `If-None-Match: *` and a bogus `If-Match` both still answer 200 with the full object — no conditional-request support on this surface (the pinned Render API has none either).

No bug filed.
