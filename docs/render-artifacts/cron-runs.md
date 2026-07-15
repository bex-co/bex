# Render cron-job runs contract

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
