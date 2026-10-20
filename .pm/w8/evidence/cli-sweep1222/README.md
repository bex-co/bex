# CLI sweep 1222 — webhook deliveries live under `/events` (live, 2026-10-20)

Two disposable webhooks created and deleted (204, verified 404). No new bug filed. Secrets are not recorded here.

- `/attempts` and `/deliveries` subroutes → 404; the declared surface is `GET /webhooks/{id}/events` (200 `[]` for a webhook with no deliveries) plus `POST /webhooks/{id}/events/{attemptId}/resend`, which rejects a malformed attempt id with 400.
- `GET /webhooks/event-types` → 200 with the subscribable event vocabulary (`auto_deploy_*`, `build_*`, `cron_job_run_*`, …) — the list the `eventFilter` enum validates against (sweep 1146).
