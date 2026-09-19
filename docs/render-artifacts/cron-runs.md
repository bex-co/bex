# Render cron-job runs contract

> bex's cron-job design (the `cron_job` type, CronJob mechanism, run history, and this contract) is consolidated in [ADR038-cron-jobs.md](../ADR038-cron-jobs.md). This file is the pinned Render-side capture it references.

Verified against Render's live public OpenAPI on 2026-07-14:

- OpenAPI: <https://api-docs.render.com/openapi/render-public-api-1.json>
- Trigger reference: <https://api-docs.render.com/reference/run-cron-job>
- Cancel reference: <https://api-docs.render.com/reference/cancel-cron-job-run>
- Cron execution behavior: <https://github.com/render-oss/skills/blob/main/skills/render-cron-jobs/SKILL.md>
- Official MCP server: <https://github.com/render-oss/render-mcp-server>

## Current Render REST contract

Render currently exposes one path:

| method | path | behavior | response |
| --- | --- | --- | --- |
| `POST` | `/v1/cron-jobs/{cronJobId}/runs` | cancel an active run, then trigger a replacement | `200` + `cronJobRun` |
| `DELETE` | `/v1/cron-jobs/{cronJobId}/runs` | cancel the currently running execution | `204`, empty body |

The current spec has no list-runs route, no get-run-by-id route, and no per-run cancel route. Those three routes described in `w2/m36` are therefore bex extensions, not current Render endpoints:

- `GET /v1/cron-jobs/{id}/runs`
- `GET /v1/cron-jobs/{id}/runs/{runId}`
- `POST /v1/cron-jobs/{id}/runs/{runId}/cancel`

bex also implements Render's current `DELETE .../runs` route. Its explicit per-run cancellation returns `409` for an already-terminal run; Render's current cancel-current OpenAPI only documents `204` plus infrastructure/auth errors and does not specify a terminal-run response. A conflict is preferable to a silent successful no-op.

Render documents a single concurrent cron execution. A manual trigger while one is active cancels the active execution before starting the replacement. bex applies `ForbidConcurrent` to scheduled Kubernetes CronJobs and carries cancel-old plus trigger-new intent in the same App spec update for manual runs.

**A canceled run stays canceled (w4/m114 t001).** `spec.cancelRun` is a single slot the API reuses for every cancellation, and `spec.runAt` is never cleared when its manual run ends — so the operator's no-recreate guard, which used to consult that slot alone, was disarmed by the next unrelated cancel and recreated the canceled Job (observed in production 2026-09-17: a canceled manual run returned to `pending` and executed two fresh attempts). The guard now reads terminal HISTORY (`status.runs`, which deliberately retains terminal entries across Job GC and across the deletion a cancel performs) and keeps the cancel slot only as a second term. A canceled, failed or succeeded manual run is never recreated while its `runAt` stands; a fresh trigger mints a new `runAt`, hence a new Job name, and runs normally.

## Run object

`cronJobRun` has:

| field | type | required | bex source |
| --- | --- | :-: | --- |
| `id` | string | yes | deterministic `crr-…` derived from the backing Kubernetes Job name via `internal/id` |
| `status` | enum | yes | `pending`, `successful`, `unsuccessful`, or `canceled` |
| `startedAt` | RFC3339 date-time | no | Job `status.startTime` |
| `finishedAt` | RFC3339 date-time | no | completion/failure transition or accepted cancellation time |
| `triggeredBy` | string | no | omitted: Kubernetes Jobs do not retain the API caller identity |
| `canceledBy` | string | no | omitted for the same reason |

The CR keeps mechanism-facing `Running`/`Succeeded`/`Failed`/`Canceled` values; bex-api maps them to Render's wire enum. Actor fields are omitted rather than fabricated.

## Paging and IDs

The bex list extension uses Render's standard array item envelope:

```json
[{ "cronJobRun": { "id": "crr-…", "status": "successful" }, "cursor": "crr-…" }]
```

`cursor` is the last run's stable derived ID, and `limit` follows the shared 1–100/default-20 REST rule. Unknown cursors return an empty tail. GraphQL and MCP return the same run fields; their clients page by echoing the final run ID.

The derived ID deliberately hides the Kubernetes Job name while remaining stable across reads and after Job garbage collection. `App.status.runs` retains terminal history, capped at ten entries.

## Service field

Render's cron service details include `lastSuccessfulRunAt` as an RFC3339 date-time. bex derives it from the newest successful `status.runs` entry and exposes it as `serviceDetails.lastSuccessfulRunAt` on REST and `Service.lastSuccessfulRunAt` on GraphQL.

## MCP

Render's official MCP server currently exposes cron creation but no run trigger/list/get/cancel tools. bex's `run_cron_job`, `list_cron_job_runs`, `get_cron_job_run`, and `cancel_cron_job_run` are documented extensions over the same service-layer verbs used by REST and GraphQL.

## Dashboard (w5/m60)

The cron-job runs panel now reaches the two previously dashboard-unconsumed verbs, matching Render's cron page interactions:

| Closure | Verdict | Notes |
| --- | --- | --- |
| Trigger Run (`runCronJob`) | ✅ match | A confirmed **Trigger Run** button in the runs-panel header fires `runCronJob(id)` and refetches the history so the new run appears with live status. Any backend error is surfaced **inline**, not swallowed in a toast. **Trigger-during-active is preemption, on every surface (corrected w4/m114 t003).** `TriggerCronRun` cancels the active run and returns the new pending one in a single spec update (`apps/service.go`'s `pendingCronRun` → `spec.cancelRun` + `spec.runAt`), which is Render's own cancel-then-replace — so the earlier claim on this row that bex "rejects while one is active" was never the shipped behavior, and the dashboard's disabled button was the only surface enforcing it. Found live 2026-09-17: the button was disabled with "A run is already in progress" while the same instant's direct `runCronJob` returned a pending run and flipped the active one to `canceled` — and the disabled button blocked the one in-product way to preempt a wedged run. The button now stays enabled during an active run and its confirm dialog names the consequence ("A run is already in progress. Triggering now cancels it and starts a new run immediately, outside the schedule."). |
| Run detail (`cronJobRun`) | ✅ match | A history row expands to a detail read via `cronJobRun(serviceId, runId)` — status, absolute start/finish timestamps, computed duration, and the run id (the row shows only relative start + duration). A stale/unknown run id renders an explicit error, never a blank panel. |

Cross-surface: the UI's semantics equal the REST/MCP verbs — `runCronJob` = MCP `run_cron_job` = `POST .../runs`; `cronJobRun` = MCP `get_cron_job_run` = `GET .../runs/{runId}`. No new drift filed. Verified by the dashboard suite (`use-cron-runs`/`cron-runs-section` trigger, active-run rejection, detail-expand, and detail-error tests); the live browser walk was infra-blocked in-session (dev-5 unraisable) and is folded into the shared w5/m60 deferral note.

## Deferred m60 walkthrough closeout (2026-09-06)

The remaining notification/registry checks from `w5/029` passed on production with disposable fixtures, now deleted; see `w5/done/029.md` for exact outcomes and artifacts. The dated 2026-08-08 cron Trigger Run and run-detail proof is retained there. No fresh terminal cron-run capture is claimed by this follow-up.
