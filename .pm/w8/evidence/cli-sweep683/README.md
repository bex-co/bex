# CLI sweep 683 — session-command guards and psql command mode (live, 2026-10-19)

Nothing mutated; the only query was a read-only `SELECT 1` on the baseline Postgres.

- `bex ssh <synthetic>` and `bex kv-cli <synthetic>` in non-interactive mode → `` `bex ssh` can only be used in interactive mode `` (resp. kv-cli), before any resolution or connection attempt.
- `bex psql <id>` without `--command` → `--command flag is required in non-interactive mode` with usage. With `--command "SELECT 1"` on a synthetic id → `No Postgres instance found with name or ID '…'`.
- `bex psql <baseline> --command "SELECT 1 AS qa683" -o json` → the tabular output in an `{"output": …}` envelope. The non-interactive data path works.

No bug filed.
