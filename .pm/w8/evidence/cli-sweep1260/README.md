# CLI sweep 1260 — Postgres rename, name resolution, suspend/resume through the CLI (live, 2026-10-20)

One free Postgres created with `bex pg create --plan free` in `tian-personal` (canary's slot is held by a baseline), available in 103 s, deleted with `bex pg delete --confirm` (GET 404). No new item; one addendum to w9/m164.

- `bex psql <id>` and `bex psql <name>` both return rows. After `bex pg update <id> --name <new>`, the new name resolves and the old one fails at resolution (`No Postgres instance found with name or ID '<old>'`, exit 1).
- `bex pg suspend` → `suspended`, `suspenders: ["user"]`. `bex psql` while suspended fails with a bare `SSL error: unexpected eof while reading`, the same opaque front-door close as w9/m164, so it was appended there rather than filed separately. `bex pg resume` → `available` in 22 s, and the query works again.
