# CLI sweep 573 — Postgres CLI reads and create refusals (baseline untouched)

**Verdict: no bug filed.** Same isolated QA session, `bex-canary`, HEAD-built CLI at `77ec448ae`, production pin `d98d07f62`, 2026-10-19T19:44–19:48Z. Read-only against the pre-existing baseline database (`dpg-db9r654flioc739mcpbg`) plus create-side refusals — no baseline update/suspend/delete was attempted, and nothing was created.

- **Create refusals, each at the right layer:** bogus plan and `--version 99` → boundary `invalid request body at /plan|/version`; region, disk size (`3` and explicit `0` — "must be 1 or a multiple of 5"), connection-pool, and `--ip-allow-list` grammar → pinned-client validation before any call; malformed database name/user → coded `POSTGRES_IDENTIFIER_INVALID` spelling the full identifier grammar; any Datadog input → coded `POSTGRES_DATADOG_UNSUPPORTED` naming the two fields to remove (the documented non-feature, refused rather than stored). A valid second free create → `WORKSPACE_RESOURCE_LIMIT` naming the 1-database workspace cap and its remedy; list unchanged.
- **Read parity:** `postgres get` resolves by id and by exact name; CLI JSON carries 22 keys, a faithful subset of the 27-key REST read (CLI omits only bex extras — `backupsEnabled`, `externalHost`, `ownerId`, `poolerEnabled`, `public`). `role: primary` and `suspenders: []` agree on both surfaces (the w8/129 fix, re-observed). YAML and text modes render the same record; the `pg` alias resolves identically. No connection strings were printed (key names and non-secret fields only).
- **Selectors:** well-formed nonexistent `dpg-000…` → the client's named "No Postgres database with ID …"; service/Key-Value ids and malformed strings take the name-resolution path and fail with the workspace-scoped remedy message.

## Unrun

- Suspend/resume/update/delete state machine on Postgres (needs an owned instance; the free slot is held by the baseline, which this loop never mutates — the Key Value twin was flown in sweep 570).
- `psql` session and connection-info contents (secret-bearing surface; intentionally not printed or exercised here).
