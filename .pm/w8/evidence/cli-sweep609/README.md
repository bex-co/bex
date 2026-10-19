# CLI sweep 609 — Postgres CLI boundary dialects on synthetic ids (baseline untouched)

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, CLI at `1a49cb806`, 2026-10-19. Read-only and refused probes only; the baseline free Postgres (`qa-20261017-64552e-pg14`) was only read, never mutated, suspended, or connected to.

- **Lookup honesty across verbs:** `postgres get|suspend|resume|delete|update` on `dpg-00000000000000000000` all fail identically — `No Postgres database with ID 'dpg-000…'`, exit 1. Resolution precedes validation: even `--plan banana` on the synthetic id gets the not-found, not a plan error.
- **Name resolution:** `postgres get qa-20261017-64552e-pg14` (by name) returns the baseline row; its read shape carries `role: primary` and `suspenders: []` (the w8/129 fields, live at this pin).
- **Session verbs:** `pgcli` in a pipe → `` `bex pgcli` can only be used in interactive mode ``. `psql` without `--command` in non-interactive mode → `--command flag is required in non-interactive mode` with a usage line; with `--command 'SELECT 1'`, synthetic id and garbage name both fail at resolution (`No Postgres instance found with name or ID '…'`) before any connection. An empty `--command ''` is treated as missing. No query was ever run against the baseline.
- The usage line still reads `Usage: render psql …` — the pinned client's own string (documented upstream residual class), not filed.

## Unrun

- A real `psql --command` round trip (needs an owned Postgres; the free slot is held by the baseline).
- `postgres create` full flight for the same reason (covered in part by sweep 573's refusals).
