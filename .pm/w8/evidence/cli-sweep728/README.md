# CLI sweep 728 — secret-file read scoping (live, 2026-10-19)

Disposable free web service `qa728-web` (`srv-dbbccadfsg2s73akoaog`) carrying one secret file. Deleted and verified absent (REST 404).

- `GET /services/{owner}/secret-files/qa728.txt` → 200 with the stored content round-tripping.
- The same file name under a different service (the baseline) → 404 `secret file not found` — indistinguishable from a nonexistent name on the owning service (also 404 `secret file not found`). Reads are scoped per service with no cross-service existence leak.
- `DELETE …/secret-files/qa728.txt` → 204; list back to `[]`.

No bug filed.
