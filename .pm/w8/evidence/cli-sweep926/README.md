# CLI sweep 926 — id path-shape discipline (live, 2026-10-20)

Disposable free web service created, probed, deleted (204, then GET 404). No new bug filed.

- Trailing-space (`%20`) and uppercase variants of a real id → coded 404 `NOT_FOUND`: ids match byte-exactly (no case folding or trimming that could alias one resource onto another), while the exact id resolves and lists exactly once.
