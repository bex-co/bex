# CLI sweep 982 — cron job schedule validation and paid gate (live, 2026-10-20)

Refusal-side probes only; nothing was created. No new bug filed.

- A malformed schedule → coded 400 `SCHEDULE_INVALID` with an example expression and `params.field=schedule`.
- A valid schedule on the free plan → coded 400 `a cron_job requires a paid plan; plan must be one of starter|standard|…` — the paid gate refuses, names the allowed set, and creates nothing. No paid plan was submitted.
