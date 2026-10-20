# CLI sweep 710 — datastore get by name (live, 2026-10-19)

Read-only; the baseline Postgres was only read.

- `bex postgres get qa-20261017-64552e-pg14` (by name) resolved to the baseline id with status `available`.
- Unknown names fail workspace-scoped and helpfully: `No Postgres database named '…' in workspace tea-… … pass the Postgres database ID instead` (same shape for Key Value, sweep 659's dialect).

No bug filed.
