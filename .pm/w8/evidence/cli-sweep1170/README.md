# CLI sweep 1170 — suspended services accept config writes, refuse deploys (live, 2026-10-20)

One disposable free web service created, suspended, and deleted (204, verified 404). No new bug filed.

- While suspended: env-var PUT → 200 and rename PATCH → 200 (configuration stays editable so the service can be fixed before resuming), but deploy trigger → 409 (consistent with restart-while-suspended, sweep 1127).
- DELETE while suspended → 204.

Suspension freezes execution, not configuration.
