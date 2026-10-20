# CLI sweep 908 — concurrent PATCHes merge field-wise (live, 2026-10-20)

Disposable free web service created, patched concurrently, deleted (204, then GET 404). No new bug filed.

- Four simultaneous PATCHes (two racing `healthCheckPath` values, two racing `port` values) all → 200. The final read is one coherent value per field (`/b908`, `8081`) with **both** fields present — per-field application means neither writer's untouched field was clobbered by the other's whole-object write. Last-writer-wins per field, no torn or lost updates.
