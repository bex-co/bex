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
- Deploy-history rows show the available status, commit/message, trigger, deployed time, duration, and pre-deploy outcome without placeholders. The Rollback link to the deploy's Confirm Rollback page (w1/m646, [rollback-comparison.md](rollback-comparison.md)) is reachable from successful historical (`deactivated`) rows, but not from the current live row or a failed row; row navigation and the sibling action remain separate keyboard targets.
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
| Deliberate scope — **superseded (w9/m58, 2026-07-26)** | Render's header shows an internal address; at capture time the bex dashboard/API contract did not supply one. | Closed by ADR041 D4: bex-api now supplies `internalAddress` (`<slug>:<port>`, w9/m57 made it resolvable) and the service-detail header — the same chrome the deploy-detail page renders under — shows it in the Connect → Internal menu (a private service also shows it as its Service Address). |
| Deliberate compatibility | An omitted REST/GraphQL limit returns full history for older bex clients, while Render defaults to 20; bex MCP exposes additional status/time filters. | Preserve the documented compatibility behavior and AI-native filter superset. |
| Hard non-goal | Render navigation includes browser Shell, persistent disks, one-off jobs, PR previews, and external log-drain upsells. | Keep excluded by `.pm/DO_NOT_DO.md`; no parity follow-up was created. |

## Notices

Recorded 2026-11-26 for `w1/m659` from Render's public dashboard bundle (entry `index-D5V4ZgbE.js`, read the same day): the `DeployContainer` chunk's deploy page, `useDeployDetails` (the facts), `CancelReasonText`, `FailureReasonFragment` (the health-check sentence) and `artifact-image-ref` (the trigger kinds). No frontend flag gates any of it.

Render's deploy page puts a **Notices** section between the deploy header and the log. It is a panel: a small-caps `Notices` heading over a list whose items are separated by a rule. The heading hides itself when the list has no item (`[&:not(:has(+_ul_>_li))]:hidden`), so a plain successful deploy shows nothing. The entries, in Render's order:

| # | Render's words | Render's condition | Link | bex fact | bex shows |
| --- | --- | --- | --- | --- | --- |
| 1 | The failure reason, then "Read our docs for common ways to troubleshoot your deploy." | `deploy.reason` is set; the docs sentence is left out when the failure step is `INTERNAL` | "docs" → `/troubleshooting-deploys` | `failureReason` (+ `failureReasonTemplate`) | bex's own failure sentence, unchanged, which already names the step; the docs sentence links bex's [How deploys work § Troubleshoot in order](https://bex.co/docs/platform/how-deploys-work#troubleshoot-in-order) and is left out for `platform_error`, bex's internal failure |
| 2 | "Build cache cleared" | `triggerDetails.clearCache` | — | `deploys.clear_cache` (w4/413), served as GraphQL `Deploy.clearCache` | ✓ |
| 3 | "Transferred from {workspace}" | `triggerDetails.transferredFrom` | — | n/a: bex has no service transfer between workspaces | — |
| 4 | "Updated to a free compute plan" | `triggerDetails.isTransitionToFreeTier` | — | a plan change opens a `settings_change` deploy; the row now records a move from a paid plan to the free one, GraphQL `Deploy.freePlanTransition` | ✓ |
| 5 | "Render periodically runs zero-downtime maintenance to ensure platform reliability." | trigger kind `system` | — | trigger `deployed_by_render`, bex's platform rebuild | "bex periodically runs zero-downtime maintenance to ensure platform reliability." |
| 6 | "Rolled back to {deployId}" | trigger `rollback` or `api` with a `targetDeployId` | the target deploy's page | `rollbackOf` | ✓ |
| 7 | "Rollback is not available because this build no longer exists." | status `DELETED` (Render's past deploy) and `rollbackSupportStatus` `ROLLBACK_UNSUPPORTED_BUILD_NO_LONGER_EXISTS` | — | status `deactivated` and `buildAvailable: false` (w8/m156) | ✓ |
| 8 | "Synced from {Blueprint id}" | trigger `iac` with its sync's source and execution | the Blueprint's sync page, `/blueprint/{id}/sync/{syncId}` | trigger `blueprint`; the row now records the Blueprint and the sync that opened it, GraphQL `Deploy.blueprintId` and `Deploy.blueprintSyncId` (public `exs-` and `exe-` ids) | ✓, linking the Blueprint's page, whose sync history lists the sync: bex has no page for one sync |
| 9 | "Canceled by you", "Canceled by {email}", "… via API", "Canceled when the service was suspended" | `cancelReason` kind `user`, `api` or `suspended` | — | `cancelReason` (+ template) | bex's own cancel sentence, unchanged, for every cancel that is not a supersede; a cancel by a person records no reason in bex (w6/m52), so it shows none |
| 10 | "Another deploy started", then " . To avoid canceling active deploys when new ones are triggered, adjust your overlapping deploy policy." | the deploy's `DeployEnded` event names a `newDeploy`; the policy sentence only under `CANCEL_AND_DEPLOY_LATEST` and not on a preview | "Another deploy" → the new deploy; "overlapping deploy policy." → Workspace Settings `#deploy-policy` | a supersede `cancelReason` (`superseded_by`, `superseded_by_newer_release`), the workspace's `overlappingDeployPolicy` | bex's supersede sentence, unchanged ("Superseded by dep-…"), then the policy sentence under Override, on a line of its own, linking Workspace Settings `#overlapping-deploy-policy`. Render's stray space before the period is not copied: a line of its own needs no joining period in any language |
| 11 | "Another deploy is in progress", then " . To avoid queuing deploys, adjust your overlapping deploy policy." | status `QUEUED` and a deploy of the service's list is in progress; the policy sentence only under `QUEUE_AND_UPDATE` and not on a preview | "Another deploy" → that deploy; the policy link as in 10 | status `queued`; the in-progress deploy is found in the service's `deploys` list on the client, as Render does, so no new field; `Workspace.overlappingDeployPolicy` | ✓, the policy sentence under Wait, on a line of its own as in 10 |
| 12 | "Waiting for internal health check to return a successful response code at: {host}:{port}{path}" while in progress; "Timed out after waiting for internal health check …" when the deploy timed out | the service has a Health Check Path | "health check" → `/deploys#zero-downtime-deploys` | the service's `healthCheckPath` and `port`; a timeout is a failed deploy with a rollout budget it ran out of (the deploy row's `end_timed_out_seconds`, w8/m146), served as GraphQL `Deploy.timedOutSeconds`, Render's `reason.timedOutSeconds` | ✓ while the deploy is `update_in_progress` (Render's `IN_PROGRESS`) or failed with a timeout, at `0.0.0.0:{port}{path}`, "health check" linking bex's [Health checks](https://bex.co/docs/platform/health-checks) |

bex adds one entry Render has no counterpart for: the operator's live diagnosis of an open deploy (`stallReason`, w4/m112), last, as it was under the header before.

bex has no service previews (`.pm/DO_NOT_DO.md`), so Render's "not on a preview" condition always holds.

**The reserved-port warning.** Below the Notices and above the log, while the deploy is in progress or failed, on a free plan, Render shows an informative banner when the service's `openPorts` holds an HTTP listener on 18012, 18013 or 19099: "It looks like your service might be listening on port {port}, which is a reserved port for free compute plans. Please configure your service to listen on a port that is not part of our reserved ports: 18012, 18013, 19099." with "free compute plans." linking `/free#other-limitations`. bex's port scan used to drop a reserved listener, so nothing could say it. Now the operator probes a reserved listener of an unready web instance for HTTP when no other port answers, and says so in the stall reason's hint (`hint_reserved_port`) instead of the generic "no open port", and the failure reason keeps that hint. GraphQL `Deploy.reservedPort` reads the port from the deploy's own reasons, and the dashboard shows Render's banner from it, "free compute plans." linking [bex's pricing](https://bex.co/pricing), since no bex docs page lists the reserved ports. The port stays out of the routable set and out of `openPorts`. bex reserves the three ports on every plan, as Render's web-services docs do, so the hint shows on every plan, and the banner, as on Render, only on a free one.

**REST and MCP.** Render's public deploy object (`render-public-api-1.json`) carries none of these facts, so REST and MCP deploys are unchanged.

## Recorded drift

The lifecycle/timestamp gap recorded by `w5/m29` is closed by `w2/m38`. The commit-author timestamp drift is closed by `w2/m42`: bex captures the resolved Git author timestamp once at deploy-open and exposes it on REST, GraphQL, MCP, and the dashboard, while still omitting it when resolution is unavailable. Remaining drift is explicit:

- Omitting REST/GraphQL `limit` still returns the full history for pre-pagination bex clients, while Render REST defaults to 20. This is a deliberate compatibility contract, not an unowned implementation gap.
- bex exposes status/time filters on MCP in addition to Render's official tool's current limit/cursor inputs. This is a deliberate AI-native surface-parity superset; it does not alter the returned deploy semantics.
