# CLI sweep 700 — env-var verb dialects (live, 2026-10-19)

Same disposable fixture as sweep 699, deleted after this sweep and verified absent (REST 404).

- `POST /services/{id}/env-vars` (single-add guess) → 405 `method not allowed`: the write verb is PUT replace-all (sweep 647), as on Render.
- `PUT` two vars, then `GET …/env-vars?limit=1` → one entry with `cursor:"A1"` (keyset paging works on this subresource).
- `DELETE /services/{id}/env-vars/A1` → 204; remaining list shows only `A2` with its revision stamp.

No bug filed.
