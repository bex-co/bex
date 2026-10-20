# CLI sweep 757 — deploy-hook read dialect (live, 2026-10-19)

Disposable free web service `qa757-web` (`srv-dbbcmpd4c21c738t03jg`). Deleted and verified absent (REST 404). The hook URL itself was never printed or recorded — only its shape.

- `GET /services/{id}/deploy-hook` → 200 `{"url": …}` for the authorized owner (the service object itself carries no hook field).
- `POST /services/{id}/deploy-hook` → 405 (no regeneration verb on this route).

No bug filed.
