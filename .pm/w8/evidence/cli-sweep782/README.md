# CLI sweep 782 — blueprint mutation guards on synthetic ids (live, 2026-10-19)

Synthetic targets only; the pre-existing `bex` blueprint was not touched.

- `PATCH /blueprints/blp-00000000000000000000` → 404 `blueprint not found`.
- `DELETE` on it → 404 `NOT_FOUND`; `POST …/sync` → 404 `blueprint not found`.

No bug filed.
