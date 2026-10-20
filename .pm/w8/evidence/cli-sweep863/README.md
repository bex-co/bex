# CLI sweep 863 — method/Accept discipline (live, 2026-10-20)

Disposable free web service created and deleted (204, then GET 404). No new bug filed.

- `GET` with `Accept: text/html` still answers `200 application/json` (the API does not content-negotiate its REST shapes).
- `PUT /services/{id}` (no such verb; PATCH is the update spelling) → 405 coded `method_not_allowed`.
- Trailing-slash collection `GET /services/` → 404 `not_found` rather than a silent redirect to the canonical path.
