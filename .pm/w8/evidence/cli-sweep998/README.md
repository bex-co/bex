# CLI sweep 998 — CLI restart and deploy list commands (live, 2026-10-20)

One disposable free web service created via REST and deleted (204; GET 404). No new bug filed.

- `bex restart {id}` → `restarted successfully`; the restart lands as a new queued deploy with `trigger: api`, visible in `bex deploys list {id} -o json`. CLI and REST agree on the same deploy record.
