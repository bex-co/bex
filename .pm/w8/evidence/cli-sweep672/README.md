# CLI sweep 672 — static-only subresources on a web service (live, 2026-10-19)

Disposable free web service `qa672-web` (`srv-dbbbigtfsg2s73ako8g0`). Deleted and verified absent (REST 404).

- `GET|PUT /services/{id}/routes` and `/headers` on a `web_service` → 400 `service "…" is not a static_site` (named type gate, both verbs).
- `/redirects` and `/rewrites` subresources → 404: correct, Render folds both into `routes` entries by `type`.

No bug filed.
