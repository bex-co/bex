# CLI sweep 798 — restart works; cache purge is a documented non-goal (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- `POST /services/{id}/restart` → 200 with the service object; restart on a synthetic id → 404 coded `NOT_FOUND`.
- `POST /services/{id}/cache/purge` → 404 `not found`. Documented non-goal, not a bug: [ADR018](../../../docs/ADR018-render-parity.md) row "Cache purge | — | — | — | — | Render `POST …/cache/purge` (static-site CDN). bex has no build CDN cache — non-goal." The route is not served, consistent with the ledger.
