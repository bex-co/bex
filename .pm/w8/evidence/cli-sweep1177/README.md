# CLI sweep 1177 — ids do not cross resource families (live, 2026-10-20)

One disposable free web service created and deleted (204, verified 404); the baseline Postgres was only read by id under a wrong route. No new bug filed.

- Postgres (`dpg-`), Key Value (`red-`), and project (`prj-`) ids under `/services/{id}` → 404, and the service's `srv-` id under `/postgres/{id}` and `/key-value/{id}` → 404. Each family resolves only its own prefixed ids; a real resource never leaks into a sibling family's shape.
