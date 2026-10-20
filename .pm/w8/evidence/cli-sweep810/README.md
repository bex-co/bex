# CLI sweep 810 — Key Value create/rename/delete cycle (live, 2026-10-20)

Disposable free Key Value instance created, renamed, deleted (204, then GET coded 404). No new bug filed.

- `POST /key-value {"plan":"free"}` → 201 with nested `owner`, `options.maxmemoryPolicy: allkeys_lru`, Valkey `version: 8`, status `creating`.
- `PATCH {"name":"qa810-kv-b"}` → 200; the `red-…` id is unchanged across the rename (the stable-id contract from w9/m6 holds live).
- DELETE → 204; GET after → coded 404.
