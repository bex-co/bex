# CLI sweep 1164 — census clean; baselines intact (live, 2026-10-20)

Read-only census. No new bug filed.

- No `qa11*` fixtures survive in `bex-canary` across services, env groups, projects, or webhooks — including the short-lived paid-plan service from sweep 1163, which was deleted and is absent here.
- Baseline `hello-go` service and baseline Postgres both read 200 (read-only check; never mutated).
