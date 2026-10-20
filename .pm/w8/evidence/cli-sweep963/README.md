# CLI sweep 963 — PATCH service body discipline (live, 2026-10-20)

One disposable free web service created, exercised, deleted (204; GET 404). No new bug filed.

- Rename → 200. An unknown body field → coded 400 naming the field (`request body contains unknown field "qaBogusField963"`); a wrong-typed `name` → 400; an over-long name (300 chars) → 400.
- An empty PATCH body → 200 no-op (the service is returned unchanged), matching idempotent-update expectations.
- A display name containing a slash is accepted on rename. Rename is display-label only (the slug stays immutable, established in earlier sweeps), and no pinned contract restricts display-label characters, so this is recorded, not filed.
