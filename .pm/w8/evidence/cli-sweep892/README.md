# CLI sweep 892 — deploy-hook reveal/rotate cycle (live, 2026-10-20)

Disposable free web services created, hook cycled, deleted (204s; GETs 404 — including one fixture left by a crashed harness run, reused for the cycle and then removed). No new bug filed. The hook URL itself was never printed; rotation was verified by comparing SHA-256 hashes of the URL.

- `GET /services/{id}/deploy-hook` → 200 `{"url": …}` for the authenticated owner, with `Cache-Control: no-store` (the code's stated cache-poisoning guard, `lego/backend/internal/deploys/rest.go:198`).
- `POST /services/{id}/deploy-hook/regenerate` → 200, also `no-store`; the next GET's URL hash differs — rotation is real, and the old token is replaced, not appended. Verbs directly on `/deploy-hook` (POST/PUT/PATCH without `/regenerate`) → 405.
- Sweep 820's note is refined: the service GET carries no hook field, but the dedicated authenticated GET above is the reveal path.
