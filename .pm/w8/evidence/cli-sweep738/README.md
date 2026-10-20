# CLI sweep 738 — method handling on env-vars (live, 2026-10-19)

Disposable free web service `qa738-web` (`srv-dbbci3l4c21c738t02ug`). Deleted and verified absent (REST 404).

- `PATCH /services/{id}/env-vars` → 405 `method not allowed` (PUT replace-all and single-key DELETE are the write verbs, sweep 700).
- `HEAD` → 200; `OPTIONS` → 204; the stored set was untouched by all three probes.

No bug filed.
