# CLI sweep 699 — single-deploy read scoping (live, 2026-10-19)

Disposable free web service `qa699-web` (`srv-dbbbqvdfsg2s73ako980`, deploy `dep-dbbbqvdfsg2s73ako98g`). Deleted and verified absent (REST 404).

- `GET /services/{owner}/deploys/{dep}` → 200.
- The same deploy id under a different service (the baseline) → 404 `deploy not found` — reads are scoped to the owning service, indistinguishable from a synthetic deploy id (also 404 `deploy not found`).

No bug filed.
