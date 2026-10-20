# CLI sweep 690 — validator edges on service reads (live, 2026-10-19)

Read-only against the baseline `hello-go` (verified intact afterwards, HTTP 200 object read).

- `GET /services/{id}?bogusParam=1` → 400 `unsupported query parameter "bogusParam" for retrieve-service`.
- Trailing-slash `GET /services/{id}/` → 404; uppercase id → 404 `NOT_FOUND` (ids are case-sensitive, no silent normalization).
- `DELETE /services/{id}` carrying a body → 400 `request body is not allowed for this operation` at the validator, before any handler ran.

No bug filed.
