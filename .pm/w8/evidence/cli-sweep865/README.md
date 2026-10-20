# CLI sweep 865 — Postgres create refusal vocabulary (live, 2026-10-20)

Refusal-only probes against create; the name list afterwards confirms nothing was created. The baseline Postgres was never patched (an early draft probe aimed at it was discarded unexecuted). No new bug filed.

- Bogus `version: "99"` and `plan: "bogus865"` → 400 `invalid request body at /<field>` (schema-named).
- Bogus `connectionPool` → coded 400 `POSTGRES_CONNECTION_POOL_UNKNOWN` with the allowed pair and the offending value echoed in `params` (the w2/024 alias contract working as designed).
- `diskSizeGB: -3` → coded 400 `POSTGRES_STORAGE_INVALID` with `params.field`.
