# CLI sweep 1190 — census clean at the 1190 mark (live, 2026-10-20)

Read-only census. No new bug filed.

- No `qa11*` fixtures survive across services, Key Value, Postgres, env groups, or webhooks in `bex-canary`.
- All three baselines (service `hello-go`, Postgres, blueprint `bex`) read 200, unmutated.
