# CLI sweep 1180 — census clean at the 1180 mark (live, 2026-10-20)

Read-only census. No new bug filed.

- No `qa11*` fixtures survive across services, Key Value, or Postgres in `bex-canary`; every fixture from sweeps 1100–1179 was deleted and verified absent in its own sweep (and the one KV created in sweep 1172 is gone).
- Baseline service and Postgres read 200, unmutated.
