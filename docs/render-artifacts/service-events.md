# Service events: Render comparison and bex contract

**Captured:** 2026-07-18 **Render dashboard:** authenticated Events tab for `srv-caijkqarrk01jalurn90` **bex report:** `dashboard.bex.co/services/srv-d9bj8s3eg85c7390eb9g/events` returned a successful but empty view

## Evidence and root cause

The Render dashboard capture showed a paged, weeks-long timeline and a searchable grouped event filter. Render's current [List events API](https://api-docs.render.com/reference/list-events) still defaults `startTime` to one hour ago and returns cursor-bearing pages. Its 39-value public enum does not exactly match every dashboard label: for example, the dashboard calls `plan_changed` “Instance Type Changed,” and the supplied filter exposes “Branch Changed” while the public enum exposes only `branch_deleted`. Render's [webhook event documentation](https://render.com/docs/webhooks) confirms the availability, autoscaling, image-pull, ignored-commit, and maintenance-mode wire names and the `fromInstances`/`toInstances` autoscaling details.

bex's page was empty because the shared dashboard operation sent neither `startTime` nor `endTime`. `events.Service.List` correctly applied Render's one-hour API default, so a quiet service received `200 []` even though older rows existed. The route also requested only 20 rows and discarded every cursor. Metrics reused the same hook, so chart ranges wider than one hour also had incomplete event markers.

The fix leaves the REST default compatible and makes dashboard intent explicit:

- Events starts with a bounded 30-day window, cursor-loads all pages in that window, then steps backward through preceding bounded windows to service creation.
- Metrics sends the exact selected chart range and accumulates all cursor pages.
- Both clients deduplicate by stable event id; refetch resets accumulated paging state.
- The Events filter is a grouped, searchable multi-select over every event type bex can actually emit. Unsupported mechanisms never appear as dead choices.

## Supplied dashboard-label matrix

The result supports **19 of the 31 supplied labels** truthfully: 10 existing sources plus 9 new durable facts.

| Group | Supplied label | bex wire type | Disposition and source |
| --- | --- | --- | --- |
| Deploy | Deploy Started | `deploy_started` | Existing: deploy row `created_at` |
| Deploy | Deploy Ended | `deploy_ended` | Existing: deploy row terminal `finished_at` |
| Deploy | Image Pull Failed | `image_pull_failed` | Added: current-release `ImagePullBackOff` observation with bounded reason, deploy id, and image; emitted immediately while deploy retry/timeout policy remains unchanged |
| Deploy | Initial Deploy Hook Started | — | Omitted: Render preview-initialization mechanism; preview environments are a rejected product anti-goal |
| Deploy | Initial Deploy Hook Ended | — | Omitted: same preview-only source boundary |
| Deploy | Pipeline Minutes Exhausted | — | Omitted: bex has no pipeline-minute billing or enforcement source |
| Service Status | Resumed | `suspender_removed` | Existing: accepted user resume intent with actor |
| Service Status | Suspended | `suspender_added` | Existing: accepted user suspend intent with actor |
| Service Status | Instance Failed | `server_failed` | Added: previously healthy service gets a concrete readiness/container failure, including a failed new rollout instance; bounded reason only |
| Service Status | Server Restarted | `server_restarted` | Existing: accepted restart with triggering user |
| Service Status | Service Resumed | `service_resumed` | Added: operator-observed convergence from Hibernated to Running |
| Service Status | Service Suspended | `service_suspended` | Added: operator-observed convergence to Hibernated |
| Service Status | Service Recovered | `server_available` | Added: unhealthy-to-healthy observed edge |
| Scaling | Autoscaling Started | `autoscaling_started` | Added: typed App status transition, REST `fromInstances`/`toInstances` |
| Scaling | Autoscaling Ended | `autoscaling_ended` | Added: target replica count becomes ready |
| Scaling | Autoscaling Config Changed | `autoscaling_config_changed` | Existing: typed previous/current min/max |
| Scaling | Instance Count Changed | `instance_count_changed` | Existing: manual typed from/to count |
| Scaling | Branch Changed | `branch_changed` | Added bex extension: source edit with typed from/to branch; Render public enum has no equivalent `branch_changed` wire type |
| Scaling | Commit Ignored | `commit_ignored` | Added: signed matching push rejected by skip phrase, root directory, or build filter |
| Scaling | Instance Type Changed | `plan_changed` | Existing: Render's documented dashboard label for `plan_changed` |
| Scaling | Workflow Deploy Started | — | Omitted: workflows/tasks are a product anti-goal |
| Scaling | Workflow Deploy Ended | — | Omitted: workflows/tasks are a product anti-goal |
| Maintenance | Maintenance Started | — | Omitted: provider platform-maintenance orchestration is a product anti-goal |
| Maintenance | Maintenance Ended | — | Omitted: same provider-maintenance source boundary |
| Maintenance | Maintenance Deploy Started | — | Omitted: same provider-maintenance source boundary |
| Maintenance | Maintenance Deploy Finished | — | Omitted: same provider-maintenance source boundary |
| Maintenance Mode | Config Changed | `maintenance_mode_enabled` | Existing: typed accepted maintenance-mode toggle |
| Maintenance Mode | URI Updated | `maintenance_mode_uri_updated` | Existing: accepted URI update; URI/value is not copied into the event |
| Edge Cache | Enabled | — | Omitted: bex has no mutable edge-cache mode |
| Edge Cache | Disabled | — | Omitted: bex has no mutable edge-cache mode |
| Edge Cache | Purged | — | Omitted: static revisions are immutable; cache purge is a rejected anti-goal |

## One durable fact path

The service feed remains one projection, not nine bespoke emitters:

| Source | Owns | Idempotency |
| --- | --- | --- |
| `deploys` | deploy start/end | one stable deploy id plus phase |
| `audit_events` | accepted API intent/configuration | immutable audit id |
| `service_event_facts` | observed status and signed-Git decisions | producer-supplied stable `source_key` primary key |

`service_event_facts` is deliberately closed: a checked fifteen-type discriminator (w7/m66 added the deploy-lifecycle kinds below), checked public reason codes, a checked `status` column (`succeeded|failed|canceled`), and dedicated deploy/image/instance/count/branch/commit columns. It has no details JSON, generic message, or value column. The operator remains database-free; it exposes only typed autoscaling status on the App CR. bex-api's control-plane reconciler records observed phase/readiness/autoscaling edges and advances its phase/readiness checkpoint in the same transaction.

Facts retain two timestamps for two different contracts. `at` is the original occurrence time used by REST, GraphQL, MCP, dashboard, Metrics, and webhook payloads. `recorded_at` is insertion time used only by the outbound worker's durable watermark. This prevents a delayed operator observation with an older `at` from falling behind an already-advanced webhook cursor.

Every surface derives the same `evt-…` id from the same source key:

- REST: Render-compatible bare `[{event,cursor}]` with default one-hour API window.
- GraphQL: the same ids/types/timestamps plus flattened typed detail fields.
- MCP `list_service_events`: the REST envelope inside `{events:[…]}`.
- Events and Metrics: explicit-range consumers of the GraphQL feed.
- Outbound webhooks: the same fact source and derived id; thin signed payloads remain value-free.

## Deploy-lifecycle events (w7/m66)

The 2026-07-18 capture above covered the 31 labels of one supplied dashboard filter. Render's full event taxonomy is larger, and its timeline shows the build and pre-deploy beats as **distinct entries** rather than folding them into the deploy pair. w7/m66 closes that gap for the beats bex has a durable source for — the same `service_event_facts` path, extended with a checked `status` column:

| Render timeline entry | bex wire type | Source | Detail |
| --- | --- | --- | --- |
| Build started | `build_started` | control-plane reconciler observing a repo/Dockerfile-backed deploy's BuildKit build phase (ADR034) | deploy id, image |
| Build ended | `build_ended` | same, once the deploy leaves the build phase | `details.status` = `succeeded`\|`failed`\|`canceled` |
| Pre-deploy started | `pre_deploy_started` | reconciler observing `App.status.preDeploy` (w1/m33), start stamp | deploy id |
| Pre-deploy ended | `pre_deploy_ended` | same, terminal state | `details.status` = `succeeded`\|`failed` (in addition to `preDeployStatus` still on `deploy_ended`) |
| Job run ended | `job_run_ended` | `internal/jobs` syncing a one-off job to a finished state | `details.status` = `succeeded`\|`failed` (alongside the existing `job_started`/`job_canceled`) |
| Branch deleted | `branch_deleted` | the GitHub webhook's branch-delete signal (`push` with `deleted:true`, or the `delete` event) — auto-deploy is disabled | deleted branch in `branchFrom` |

Image-backed services (no build phase) emit no `build_*`; services with no pre-deploy command emit no `pre_deploy_*`; a canceled job keeps `job_canceled` and records no `job_run_ended`. Each fact is idempotent by `source_key` (`deploy:<id>:build_started`, `job:<id>:run_ended`, `git:<delivery>:<app>:branch_deleted`), so re-observing across resyncs never double-records. To catch branch deletions made through the GitHub UI (not `git push --delete`), the self-hosted GitHub App must also subscribe to the `delete` event ([docs/ADR026-github-integration.md](../ADR026-github-integration.md)).

### Still non-goals (honest, not faked)

No durable bex source exists for these, so they are omitted rather than invented: `artifact_*`, `server_hardware_failure`, provider `maintenance_started`/`maintenance_ended`, `pipeline_minutes_exhausted` (no pipeline-minute billing), `zero_downtime_redeploy_*`, `initial_deploy_hook_started`/`_ended` (preview-only), and every preview-environment/workflow event (rejected product anti-goals). The tenant-facing `maintenance_mode_*` toggle remains the one maintenance concept bex does record.

## Verification

- Real Postgres migration and composition: `TestPGStore` covers idempotent migration, transactional image-pull journaling, three-source keyset paging/filtering, tenant scope, and late-fact webhook cursors.
- Cross-surface contract: `TestEventSurfaceParity` projects an autoscaling fact through REST, GraphQL, and MCP and checks Render's REST `fromInstances`/`toInstances` spelling.
- Transition discrimination: store reconciler and operator tests cover baseline/no-op, failure/recovery, suspend/resume, and autoscaling start/end without manual-scale confusion.
- Git decisions: signed-push tests cover skip/build-filter facts, unrelated/auto-deploy-off negatives, and delivery retry idempotency without copying commit messages.
- Dashboard: explicit-window, cursor accumulation/deduplication, preceding-window, grouped filter, and Metrics range tests.
- Live harness: `scripts/events-verify.sh` passed against the non-production CAPD cluster on 2026-07-18. Its 24-event feed covered real intent/observed events, image-pull failure and recovery, source decisions, autoscaling start/end, exact cursor reconstruction, REST/GraphQL agreement, retry no-ops, a readable env-value sentinel absent from every event, store-less 503 behavior, and product-path cleanup.

## Disk events, and the dashboard catalog (2026-08-27, w6/m122)

Two corrections to the record above.

**`disk_*` is no longer a non-goal.** The 2026-07-18 capture listed it as omitted for want of persistent disks; bex has had them since [ADR082](../ADR082-persistent-disks.md) and emits four types from the `apps.AddDisk` / `apps.UpdateDisk` / `apps.DeleteDisk` / `apps.RestoreDiskSnapshot` audit verbs. That line has been removed. bex's spelling diverges from Render's for two of the four (`disk_attached`/`disk_detached` against Render's `disk_created`/`disk_deleted`), which is filed as its own open decision in `.pm/w6/068.md` — it is not settled here.

**The API was never the defect; the dashboard was.** w6/m122 found the Events tab rendering `events ∩ dashboard catalog`, so five emitted types (`custom_domain_verified` and the four `disk_*`) were invisible and — the filter's option list coming from that same catalog — unselectable. The three API surfaces were correct throughout, and `TestServiceEventSurfaceCarriesDriftedTypes` now pins that down by probe rather than assertion: REST, GraphQL and MCP all carry `custom_domain_verified` and `disk_attached` for the same fixture. **No ADR018 row is warranted for the tab fix** — making it fail-open restores Render's behaviour (Render's Events page shows the types its API emits) rather than introducing a divergence.

One standing property is worth stating because the probe made it concrete: `?type=` is validated against Render's pinned 39-value enum before the handler runs, so **every bex-named type is refused 400 on that parameter** — `env_vars_changed`, `custom_domain_added`, `custom_domain_verified` alike. That is the pinned contract working as designed, not a regression, and it is why the dashboard filters client-side over the unfiltered feed rather than pushing its type selection into the query.

The official API specification is explicitly unversioned, so this comparison records the capture date and should be refreshed when Render changes its enum.
