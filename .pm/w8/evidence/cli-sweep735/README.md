# CLI sweep 735 — repeated-delete dialects (live, 2026-10-19)

Disposable free web service `qa735-web` (`srv-dbbchci720pc73c0km00`), deleted by the first call.

- Second CLI delete by id → `Error: No service with ID '…'.` (client resolution, nonzero exit).
- CLI delete by the now-gone name → `No service named 'qa735-web' in workspace …` with the workspace hint.
- REST `DELETE /services/{id}` repeated → 404 `NOT_FOUND`.

Deletes are not idempotent-204 on this surface, but every repeat is a clean, named not-found rather than an error or false success. No bug filed.
