# Render deploy-detail comparison

Recorded 2026-07-14 for `w5/m29`, rechecked 2026-07-15 for `w2/m38`, and re-walked in a live browser on 2026-07-16 for `w5/m40`, using the live deploy-page capture already described by `w9/m1`, Render's current public documentation, generated REST client, and official MCP tool source.

## Render reference

- A deploy is opened from the service Events timeline, and its detail page is the place to view that individual deploy's logs. The explorer supports text search and renders timestamp, instance, level, and message facts. See [Logs in the Render Dashboard](https://render.com/docs/logging#logs-for-an-individual-deploy-or-job).
- An in-progress detail page carries **Cancel deploy**. See [Deploying on Render — Canceling a deploy](https://render.com/docs/deploys#canceling-a-deploy).
- Successful and failed deploys settle visibly to **Live** and **Failed**, and the log feed is the primary debugging surface. See [Your First Render Deploy](https://render.com/docs/your-first-deploy#5-monitor-your-deploy).
- Render's public deploy object carries `commit { id, message, createdAt }` for Git-backed deploys, image facts for image-backed deploys, `createdAt`/`updatedAt`/`startedAt`/`finishedAt`, trigger, and eleven statuses (`created`, `queued`, build/pre-deploy/update progress and failure states, `live`, `canceled`, `deactivated`). Its REST list accepts status plus exclusive created/updated/finished before/after bounds. Sources: the official [`render-oss/render-mcp-server` generated client](https://github.com/render-oss/render-mcp-server/blob/main/pkg/client/types_gen.go) and [Render Public API OpenAPI](https://api-docs.render.com/openapi/render-public-api-1.json).
- Render documents the order build → optional pre-deploy command → deploy and exposes three overlapping-deploy policies. bex preserves its existing newest-wins behavior (`cancel_running_deploys`) rather than adding a policy setting in this milestone. Source: [Deploying on Render](https://render.com/docs/deploys).
- Render's official `list_deploys` MCP tool currently exposes `serviceId`, `limit`, and `cursor`; bex additionally exposes the REST status/time filters through MCP so all three of its own adapters remain equivalent. Source: [`render-oss/render-mcp-server/pkg/deploy/tools.go`](https://github.com/render-oss/render-mcp-server/blob/main/pkg/deploy/tools.go).

## bex result

- The standalone Deploys history supports case-insensitive local search across the loaded deploy id, full commit SHA, and commit message while retaining the server-side status filter and keyset pagination. Its count says **N deploys loaded** while another page exists and uses **N deploys** only after pagination is exhausted.
- Deploy-history rows show the available status, commit/message, trigger, deployed time, duration, and pre-deploy outcome without placeholders. The shared rollback confirmation is reachable from successful historical (`deactivated`) rows, but not from the current live row or a failed row; row navigation and the sibling action remain separate keyboard targets.
- `/services/$serviceId/deploys/$deployId` shows the stored status, trigger, deploy id, resolved commit id/message/author timestamp when available, image, pre-deploy result, all four deploy timestamps, and a duration derived through the same helper as the list.
- Deploy rows persist the truthful observed subset of all eleven statuses. The status timeline combines the deploy row with service events whose `details.deployId` exactly matches the deploy, timestamps the current state with `updatedAt`, and renders an earlier `live` step plus the later replacement instant for `deactivated`. It never treats an `evt-…` id as a `dep-…` id or invents an unrecorded intermediate phase.
- The deploy-window log viewer interleaves `type=build`, `type=predeploy`, and `type=app` chronologically. Loading, query error, successful empty results, unavailable durable history, and live-tail disconnection are separate states. A missing durable store produces an explicit historical-build-log unavailable state while non-build log legs remain usable; it is never relabeled as an empty successful query.
- Events-list and detail-page actions share one Cancel/Rollback component. Rollback navigates to the newly created deploy, not the restored historical row.
- The dashboard labels and filters all eleven statuses, polls every open state, and stops for every terminal state. REST, GraphQL, and MCP return the same status/timestamps and accept the same status plus created/updated/finished time filters.
- The Web Service header names the operator-derived **Service** phase and the control-plane **Latest deploy** result separately, links the latest result to its deploy, and shows the API-supplied runtime when present. This prevents a stale service phase from being presented as though it were the deploy result without overwriting either source of truth.

## 2026-07-16 live dashboard re-walk

The corrected Render reference was `https://dashboard.render.com/web/srv-d2rnr3jipnbc73deuvgg`, not the static-site service used in the first comparison. The authenticated Render walk captured its deploy history and deploy `dep-d92lnrmq1p3s738j2bk0`; it did not mutate the reference service.

The dev-5 acceptance used two real services and three persisted deploy rows:

- `m40-live-web`: current live `dep-d9cplqi9086lu3qou6eg` and historical deactivated `dep-d9cplna9086lu3qou6dg`;
- `node-hello`: terminal build failure `dep-d9cp31a9086lu3qou6a0`, with runtime `node` and no durable log store.

Playwright verified search plus status-filter composition, honest complete-count wording, absence of current-live rollback, historical rollback confirmation without proceeding, explicit service/latest-deploy facts, the failed and live detail states, and keyboard focus after resizing the same page to `390x844`. The gitignored captures are `m40-live-list-final.png`, `m40-live-list-narrow.png`, `m40-live-rollback-dialog.png`, `m40-live-detail-final.png`, `m40-failed-detail-final.png`, `m40-render-list-final.png`, and `m40-render-detail-final.png` under `.playwright-mcp/`.

The browser and code audit classify the remaining visual differences as follows:

| Classification | Remaining difference | Disposition |
| --- | --- | --- |
| Environment/data | The image-backed dev-5 live fixture has no Git commit metadata; the storeless stack has no retained build history. | Omit unavailable commit fields and identify the unavailable durable log store. Do not synthesize either fact. |
| Existing owner | Render exposes running-instance shell access. | Running-instance SSH remains owned by `w2/m39`; this UI pass does not duplicate it. |
| Deliberate scope | Render's header shows an internal address, while the current bex dashboard/API contract does not supply one. | Preserve the real public URL and revision; do not manufacture an internal hostname in a presentation-only milestone. |
| Deliberate compatibility | An omitted REST/GraphQL limit returns full history for older bex clients, while Render defaults to 20; bex MCP exposes additional status/time filters. | Preserve the documented compatibility behavior and AI-native filter superset. |
| Hard non-goal | Render navigation includes browser Shell, persistent disks, one-off jobs, PR previews, and external log-drain upsells. | Keep excluded by `.pm/DO_NOT_DO.md`; no parity follow-up was created. |

## Recorded drift

The lifecycle/timestamp gap recorded by `w5/m29` is closed by `w2/m38`. The commit-author timestamp drift is closed by `w2/m42`: bex captures the resolved Git author timestamp once at deploy-open and exposes it on REST, GraphQL, MCP, and the dashboard, while still omitting it when resolution is unavailable. Remaining drift is explicit:

- Omitting REST/GraphQL `limit` still returns the full history for pre-pagination bex clients, while Render REST defaults to 20. This is a deliberate compatibility contract, not an unowned implementation gap.
- bex exposes status/time filters on MCP in addition to Render's official tool's current limit/cursor inputs. This is a deliberate AI-native surface-parity superset; it does not alter the returned deploy semantics.
