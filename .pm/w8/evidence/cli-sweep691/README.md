# CLI sweep 691 — blueprint reads (live, 2026-10-19)

Read-only; the pre-existing `bex` blueprint was only listed, never touched.

- `GET /blueprints?ownerId=…` → 200 with the one pre-existing blueprint (`blp-…`, status `created`).
- Synthetic id with a malformed suffix (22 chars) → 400 `invalid path parameter "blueprintId"` at the validator; a well-formed 20-char synthetic → 404 `blueprint not found`. The widened `blp-` pattern (render_openapi.go compat map) behaves as documented.

No bug filed.
