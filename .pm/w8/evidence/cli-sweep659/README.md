# CLI sweep 659 — datastore not-found dialects (live, 2026-10-19)

Read-only/synthetic probes only; the baseline Postgres (`dpg-db9r654flioc739mcpbg`, available, free, v14) was only read, never mutated.

- CLI `postgres get|delete|resume dpg-0000000000000000000000` and `keyvalues get|delete red-0000000000000000000000`: the pinned CLI resolves the name in-workspace first and fails with `No Postgres database named '…' in workspace tea-…` (resp. Key Value) plus a hint to pass the ID — helpful, workspace-scoped, no cross-workspace leak.
- REST `GET /postgres/{synthetic}` and `GET /key-value/{synthetic}` → 404 `NOT_FOUND`.

Not-found handling is consistent across CLI and REST. No bug filed.
