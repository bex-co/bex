# CLI sweep 1200 — census clean at the 1200 mark (live, 2026-10-20)

Read-only census. No new bug filed.

- No `qa1*` fixtures survive across services, Key Value, Postgres, env groups, projects, or webhooks in `bex-canary` — 700 sweeps of fixtures (501–1199) all deleted and verified.
- All three baselines read 200, unmutated. Production pin unchanged (`9e9c28e82d0f`).
