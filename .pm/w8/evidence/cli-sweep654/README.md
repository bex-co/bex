# CLI sweep 654 — `deploys list`/`cancel` via pinned CLI (live, 2026-10-19)

Disposable free web service `qa654-web` (`srv-dbbbd32vfpec73dm0jgg`). Deleted and verified absent (REST 404).

- Queued a bad-image deploy (`dep-dbbbd3dfsg2s73ako7k0`) to hold a cancel window; `bex deploys list <sid> -o json` showed it as `queued`.
- `bex deploys cancel <sid> <dep> --confirm` → `Deploy … successfully canceled`; REST list confirmed status `canceled`.
- Canceling the same (terminal) deploy again → 409 `conflict: deploy "…" is already canceled`.
- Canceling a synthetic deploy id → 404 `NOT_FOUND: deploy not found`.

Cancel semantics, terminal-state conflict, and not-found handling are all named and correct. No bug filed.
