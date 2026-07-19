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

`service_event_facts` is deliberately closed: a checked nine-type discriminator, checked public reason codes, and dedicated deploy/image/instance/count/branch/commit columns. It has no details JSON, generic message, or value column. The operator remains database-free; it exposes only typed autoscaling status on the App CR. bex-api's control-plane reconciler records observed phase/readiness/autoscaling edges and advances its phase/readiness checkpoint in the same transaction.

Facts retain two timestamps for two different contracts. `at` is the original occurrence time used by REST, GraphQL, MCP, dashboard, Metrics, and webhook payloads. `recorded_at` is insertion time used only by the outbound worker's durable watermark. This prevents a delayed operator observation with an older `at` from falling behind an already-advanced webhook cursor.

Every surface derives the same `evt-…` id from the same source key:

- REST: Render-compatible bare `[{event,cursor}]` with default one-hour API window.
- GraphQL: the same ids/types/timestamps plus flattened typed detail fields.
- MCP `list_service_events`: the REST envelope inside `{events:[…]}`.
- Events and Metrics: explicit-range consumers of the GraphQL feed.
- Outbound webhooks: the same fact source and derived id; thin signed payloads remain value-free.

## Verification

- Real Postgres migration and composition: `TestPGStore` covers idempotent migration, transactional image-pull journaling, three-source keyset paging/filtering, tenant scope, and late-fact webhook cursors.
- Cross-surface contract: `TestEventSurfaceParity` projects an autoscaling fact through REST, GraphQL, and MCP and checks Render's REST `fromInstances`/`toInstances` spelling.
- Transition discrimination: store reconciler and operator tests cover baseline/no-op, failure/recovery, suspend/resume, and autoscaling start/end without manual-scale confusion.
- Git decisions: signed-push tests cover skip/build-filter facts, unrelated/auto-deploy-off negatives, and delivery retry idempotency without copying commit messages.
- Dashboard: explicit-window, cursor accumulation/deduplication, preceding-window, grouped filter, and Metrics range tests.
- Live harness: `scripts/events-verify.sh` passed against the non-production CAPD cluster on 2026-07-18. Its 24-event feed covered real intent/observed events, image-pull failure and recovery, source decisions, autoscaling start/end, exact cursor reconstruction, REST/GraphQL agreement, retry no-ops, a readable env-value sentinel absent from every event, store-less 503 behavior, and product-path cleanup.

The official API specification is explicitly unversioned, so this comparison records the capture date and should be refreshed when Render changes its enum.
