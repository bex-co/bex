# CLI sweep 1173 — Postgres create validation refuses before provisioning (live, 2026-10-20)

Refusal-side only; nothing was created (fixture list verified empty). No new bug filed.

- Bogus `version` and bogus `plan` enum on a Postgres create → 400 each, validated before any database is provisioned.
- The workspace list contains only the baseline Postgres (read 200, never mutated); an unknown `dpg-` id → 404.
