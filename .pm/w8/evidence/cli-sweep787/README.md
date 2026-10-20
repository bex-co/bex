# CLI sweep 787 — `services` output formats: JSON table parity pins (live, 2026-10-19)

Disposable free web service created, inspected, deleted (204). No new bug filed.

- `services -o yaml` emits the same service objects (wrapped `- service:`, `createdAt` first key) with the fixture visible and every documented field — YAML parity with the JSON shape from sweep 576 holds across the whole mixed-type list.
- `services` (text, no export) prints the human table with NAME/PROJECT/ENVIRONMENT/TYPE/ID and mixed types correctly (Cron Job rows, the baseline Postgres row).
