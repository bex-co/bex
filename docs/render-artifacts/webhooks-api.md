# Render outbound webhooks API contract

Captured 2026-08-16 from Render's [webhook guide](https://render.com/docs/webhooks), [public OpenAPI document](https://api-docs.render.com/openapi/render-public-api-1.json), and the create/list/retrieve/update/delete/history references linked from the OpenAPI operations. The single-event retrieval contract was rechecked against the live official OpenAPI on 2026-08-17 for `w8/m24`: the route, identifier pattern, required response fields, and error set were unchanged, while the event enum had added `artifact_fetch_failed` and `artifact_source_changed` to the repository's prior 65-value snapshot. Attempt-history behavior was rechecked on 2026-08-17 against the official guide and Render's official [Recent deliveries walkthrough](https://render.com/blog/light-up-your-builds-with-render-webhooks): the dashboard shows every send attempt, expands the JSON request and endpoint response, and offers Resend. An authenticated dashboard walk on 2026-08-17 separately captured 64 picker values; it must not be conflated with either the 67-value OpenAPI enum or the 62-value prose guide. `w4/m83` rechecked the live official sources plus the history operation on 2026-08-17: Render still publishes no queue-depth limit, overflow status, or cross-workspace scheduling order. This is the dated compatibility fixture for `w2/m70`, `w8/m24`, `w8/m25`, `w8/m26`, and `w4/m83`.

## Wire contract

All management routes are under `/v1/webhooks`.

| Operation | Render request | Render success |
| --- | --- | --- |
| Create `POST /webhooks` | Required `ownerId`, unique non-empty `name`, public HTTPS `url`, `enabled`, and `eventFilter`. An empty `eventFilter` means every supported event. | `201` webhook object. |
| List `GET /webhooks` | Optional repeated `ownerId`, opaque `cursor`, and `limit` (default 20, range 1–100). | `[{webhook, cursor}]`. |
| Get `GET /webhooks/{id}` | Path id. | Webhook object. |
| Patch `PATCH /webhooks/{id}` | Any subset of `name`, `url`, `enabled`, and `eventFilter`; an explicitly empty filter still means all events. | Updated webhook object. |
| Delete `DELETE /webhooks/{id}` | Path id. | `204` with no body. |
| History `GET /webhooks/{id}/events` | Optional ISO-8601 `sentBefore`/`sentAfter`, opaque `cursor`, and `limit` (default 20, range 1–100). | `[{webhookEvent, cursor}]`. |

The Render webhook object requires `id`, `name`, `url`, `secret`, `enabled`, and `eventFilter`. A history item requires `id`, `eventId`, `eventType`, and `sentAt`, and can include `statusCode`, `responseBody`, or `error`. The list cursors are keyset tokens carried beside each object. Render's API schema returns `secret` on create, list, get, and patch because those operations share the same response schema.

bex deliberately does **not** return the stored signing secret after create. Its `whsec_…` value is a mint-once credential, matching the existing API-key security precedent. List/get/patch therefore omit `secret`; this is an explicit security divergence, not strict Render response parity. GraphQL and MCP keep their native `eventTypes` spelling and mint-once response while calling the same core verbs.

Render's prose requires a publicly reachable HTTPS destination. A notification is a Standard Webhooks signed HTTPS `POST` with `webhook-id`, `webhook-timestamp`, and `webhook-signature`. Its thin JSON body is `{type, timestamp, data: {id, serviceId, serviceName, status?}}`; `data.status` is documented for `build_ended`, `deploy_ended`, `cron_job_run_ended`, and `job_run_ended`, with `succeeded`, `failed`, or `canceled` values.

### Event hydration

The notification is deliberately thin. Its `webhook-id` and `data.id` are the same `evt-…` identity, which an authenticated receiver passes to `GET /v1/events/{eventId}` to retrieve the full event. Render's pinned path parameter matches `^evt-[0-9a-z]{20}$`. A successful response is the bare event object—not a cursor wrapper—and requires `id`, `timestamp`, `serviceId`, `type`, and `details`. The OpenAPI operation also declares 400, 401, 403, 404, 406, 410, 429, 500, and 503 responses. Bex additionally accepts optional `ownerId` on this read so a dashboard link can name a non-default selected workspace; the membership check and owner-safe 404 remain authoritative.

bex keeps that receiver workflow across all 53 event types it advertises, including datastore events that have no service-scoped activity-list home — both the audit-derived ones and, since w3/m82, the observed `datastore_event_facts` rows (availability, backup, restore, major upgrade). The event index writes every managed-datastore fact under the same `fact:<source_key>` identity an App fact uses, and the by-id lookup joins both fact tables, so a `dpg-…`/`red-…` notification hydrates by the id it delivered instead of resolving to a not-found. The global lookup is indexed and owner-scoped: it authorizes the caller's effective workspace before fetching its event, returns the same safe not-found response for missing and foreign IDs, and returns 503 when the control-plane store is unwired. REST uses Render's route and object; GraphQL `serviceEvent(id:)` and MCP `get_service_event` are bex extensions over the same lookup. The signing secret authenticates the webhook body only—it is not a credential for the retrieval request.

The receiver must return 2xx within 15 seconds. Render makes **at most eight attempts total** per notification, emails after the third failure, uses exponential backoff with the last attempt about 33 hours after the first, and disables the webhook only after all attempts fail. “Eight attempts” is not an initial request plus eight retries.

### Immutable attempts and Resend

Render's history item is a **send attempt**, not a mutable notification summary. One source event fan-out creates one logical endpoint notification; its initial send and every automatic retry appear as separate rows with their own history `id` and `sentAt`. They retain the notification's `eventId` and byte-identical request body. `webhook-id` therefore stays equal to that `eventId`, while each send mints its own `webhook-timestamp` and signature. A failed HTTP exchange or transport error belongs under **Failed immediately**, even while the logical notification has another retry scheduled.

The public REST history envelope remains `[{webhookEvent,cursor}]`, with each immutable attempt projected onto Render's documented `id`, `eventId`, `eventType`, `sentAt`, `statusCode`, `responseBody`, and `error` fields. bex additionally accepts `status=delivered|failed` on that list as a documented query extension used by its dashboard; it does not change the item or cursor envelope.

Render documents Resend as a dashboard action but publishes no public REST, GraphQL, or MCP replay operation. bex labels all three replay adapters as extensions over one admin-authorized core verb:

- REST `POST /v1/webhooks/{endpointId}/events/{attemptId}/resend` requires `Idempotency-Key` and returns `202` with the queued attempt reservation.
- GraphQL `resendWebhookDelivery(endpointId:, attemptId:, ownerId:, idempotencyKey:)` returns the same `WebhookDelivery` reservation.
- MCP `resend_webhook_delivery` takes the same endpoint, attempt, and idempotency identities.

The richer GraphQL/MCP `WebhookDelivery` view carries `attemptNumber`, attempt `status`, `statusCode`/`transportError`, bounded `requestBody` and `responseBody`, `sentAt`, plus the logical parent's `parentStatus` and `nextAttemptAt`. It never carries the signing secret. Resend is endpoint- and workspace-scoped, refuses deleted/foreign/disabled targets safely, and deduplicates concurrent submission of the same idempotency key. It reuses the original source `eventId` and stored payload bytes but is signed at the actual new send time. A successful manual attempt closes the logical notification so its superseded automatic retry is not sent later.

The durable model mirrors that distinction: `webhook_deliveries` owns notification uniqueness, payload, retry schedule, and terminal state; append-only attempt evidence owns each exchange outcome. The migration preserves every legacy row as one logical notification and backfills its latest recorded exchange as one attempt. Endpoint deletion cascades through both levels. Retention deletes whole notification subtrees, so attempts and payloads cannot be orphaned.

### Bounded admission and fair scheduling (bex extension)

Render's public guide promises destination plan gates (one URL on Pro and up to 100 on Scale/Enterprise), the 15-second/eight-attempt retry contract, and the delivery-history behavior above. Its public API describes history as events that have actually been sent. Neither source exposes a notification-backlog quota, an overflow object/status, or a cross-workspace claim order. The exact implementation behind those boundaries is therefore not a capturable Render contract.

bex adds an operator-owned safety ceiling of 10,000 open logical notifications per workspace (`BEX_MAX_WEBHOOK_DELIVERIES_PER_WORKSPACE`; `0` disables) and round-robin workspace ranking for due attempts. This is a deliberate internal multi-tenant extension, not a customer-visible Render field or quota API. A capped projection advances the source-event watermark and leaves the source deploy/resource mutation's ordinary success response unchanged. Because no network send was reserved or attempted, no parent or attempt is exposed by REST, GraphQL, MCP, or the dashboard. This agrees with Render's sent-attempt history semantics rather than manufacturing a Failed row for an exchange that never happened. Admitted work retains the exact public attempt/history, signature, retry, and Resend behavior above; fair selection changes only which workspace receives the next worker slot.

## 2026-08-17 live 67-value API enum

The following list is generated from the 2026-08-17 live OpenAPI schema at `GET /events/{eventId}`. It is the fixture vocabulary, in the schema's order:

1. `artifact_fetch_failed`
2. `artifact_source_changed`
3. `autoscaling_config_changed`
4. `autoscaling_ended`
5. `autoscaling_started`
6. `branch_deleted`
7. `build_ended`
8. `build_started`
9. `commit_ignored`
10. `cron_job_run_ended`
11. `cron_job_run_started`
12. `deploy_ended`
13. `deploy_started`
14. `disk_created`
15. `disk_updated`
16. `disk_deleted`
17. `image_pull_failed`
18. `instance_count_changed`
19. `job_run_ended`
20. `maintenance_mode_enabled`
21. `maintenance_mode_uri_updated`
22. `maintenance_ended`
23. `maintenance_started`
24. `pipeline_minutes_exhausted`
25. `plan_changed`
26. `pre_deploy_ended`
27. `pre_deploy_started`
28. `server_available`
29. `server_failed`
30. `server_hardware_failure`
31. `server_restarted`
32. `service_resumed`
33. `service_suspended`
34. `zero_downtime_redeploy_ended`
35. `zero_downtime_redeploy_started`
36. `edge_cache_enabled`
37. `edge_cache_disabled`
38. `edge_cache_purged`
39. `auto_deploy_disabled`
40. `auto_deploy_enabled`
41. `postgres_available`
42. `postgres_backup_completed`
43. `postgres_backup_failed`
44. `postgres_backup_started`
45. `postgres_cluster_leader_changed`
46. `postgres_connection_pool_changed`
47. `postgres_connection_pool_enabled_changed`
48. `postgres_created`
49. `postgres_disk_size_changed`
50. `postgres_disk_autoscaling_enabled_changed`
51. `postgres_ha_status_changed`
52. `postgres_restarted`
53. `postgres_unavailable`
54. `postgres_upgrade_failed`
55. `postgres_upgrade_started`
56. `postgres_upgrade_succeeded`
57. `postgres_restore_failed`
58. `postgres_restore_succeeded`
59. `postgres_read_replicas_changed`
60. `postgres_pitr_checkpoint_started`
61. `postgres_pitr_checkpoint_failed`
62. `postgres_pitr_checkpoint_completed`
63. `postgres_read_replica_stale`
64. `postgres_wal_archive_failed`
65. `key_value_available`
66. `key_value_config_restart`
67. `key_value_unhealthy`

### Source drift inside Render's own contract

Render's prose guide currently lists 62 event types. It includes `postgres_credentials_created` and `postgres_credentials_deleted`, which are absent from the 67-value OpenAPI enum. Conversely, the API enum includes `artifact_fetch_failed`, `artifact_source_changed`, all three edge-cache values, and both auto-deploy values, which the prose event catalog omits. bex keeps the two Postgres credential events because it has truthful audited producers and Render still documents them; API-enum fixtures classify them as documented extensions.

The authenticated dashboard picker is a third, distinct 64-value set. Relative to OpenAPI, its six API-only values are exactly `artifact_fetch_failed`, `artifact_source_changed`, `edge_cache_disabled`, `edge_cache_enabled`, `edge_cache_purged`, and `plan_changed`; its three picker-only values are exactly `instance_type_changed`, `postgres_credentials_created`, and `postgres_credentials_deleted`. `instance_type_changed` is the dashboard spelling for the product slot OpenAPI calls `plan_changed`; bex serves the truthful API spelling and labels it “Plan Changed.” The complete normalized arrays and every Bex supported/extension/alias/anti-goal/source-bound disposition are machine-readable in [render-webhook-vocabulary-2026-08-17.json](fixtures/render-webhook-vocabulary-2026-08-17.json).

`scripts/render-schema-drift.sh` now checks the live official OpenAPI enum byte-for-value/order against that 67-value array in the existing weekly schema-drift workflow. The authenticated picker is deliberately not screen-scraped in CI; its dated 64-value fixture is checked for exact counts/differences and the Go vocabulary test proves that every served or omitted Bex value has one disposition.

## 2026-08-16 bex diff, implementation, and disposition

The baseline was computed from `internal/webhooks.EventTypes`, not from prior ADR prose. Before m70, bex advertises 24 types: 21 overlap the API enum, 46 API values are missing, and three are absent from the API enum. The three are `branch_changed` (a bex extension) and the two prose-documented Postgres credential events above.

| Disposition | Exact types / behavior | Reason |
| --- | --- | --- |
| Implement now from existing durable facts (`t002`) | `branch_deleted`, `build_started`, `build_ended`, `pre_deploy_started`, `pre_deploy_ended`, `job_run_ended` | `service_event_facts` already stores these transitions and checked terminal outcomes; the outbound projector currently drops them. |
| Implement now from an existing discriminated audit row (`t002`) | `auto_deploy_enabled`, `auto_deploy_disabled` | `apps.SetAutoDeploy` already records the resulting boolean. One intent verb must project the correct outcome instead of inventing two verbs. |
| Correct the producer (`t003`) | `cron_job_run_started`, `cron_job_run_ended` | Already advertised, but currently derived from trigger/cancel intent. Reconcile observed `App.status.runs` so scheduled and manual runs both emit, cancellation emits only after terminal observation, and the ended payload has status. |
| Preserve truthful current coverage | `autoscaling_config_changed`, `autoscaling_started`, `autoscaling_ended`, `commit_ignored`, `deploy_started`, `deploy_ended`, `disk_created`, `disk_deleted`, `disk_updated`, `image_pull_failed`, `instance_count_changed`, `maintenance_mode_enabled`, `maintenance_mode_uri_updated`, `plan_changed`, `postgres_backup_started`, `postgres_created`, `postgres_restarted`, `server_available`, `server_failed`, `server_restarted`, `service_resumed`, `service_suspended` | Existing deploy, audit, or typed-fact sources. Persistent-disk create/update/delete (ADR082) project from `apps.AddDisk` / `apps.UpdateDisk` / `apps.DeleteDisk` under Render's spellings (w8/m34). |
| Preserve explicit bex extension | `branch_changed` | bex has a durable Git branch-change fact; Render's public API has only branch deletion. |
| Preserve Render-prose/documented extensions | `postgres_credentials_created`, `postgres_credentials_deleted` | Truthful audited producers and still named in Render's webhook guide, despite omission from its OpenAPI enum. |
| Repository anti-goal; document, do not fabricate | `edge_cache_enabled`, `edge_cache_disabled`, `edge_cache_purged`; `maintenance_started`, `maintenance_ended`; `server_hardware_failure`; `zero_downtime_redeploy_started`, `zero_downtime_redeploy_ended` | Explicit cache purge/CDN control and provider maintenance/hardware lifecycle remain rejected product surfaces in `.pm/DO_NOT_DO.md` or the m70 anti-goal boundary. (Persistent disks were reopened by ADR082; their Render event types moved to "Preserve truthful current coverage" in w8/m34.) |
| Implement from typed datastore audit effects (`w3/m82 t003`) | `postgres_ha_status_changed`, `postgres_connection_pool_enabled_changed`, `postgres_disk_size_changed`, `key_value_config_restart` | Each is produced by a successful datastore PATCH that changed exactly that setting; the audit row carries the value it was set to, which `GET /v1/events/{id}` returns in `details`. `key_value_config_restart` was verified against the mechanism first: the operator folds `maxmemoryPolicy`/`persistenceMode` into the Valkey StatefulSet's container args, so changing either really does roll the pod. |
| Implement from observed datastore facts (`w3/m82 t001`–`t004`) | `postgres_unavailable`, `postgres_available`, `key_value_unhealthy`, `key_value_available`; `postgres_backup_completed`, `postgres_backup_failed`; `postgres_restore_succeeded`, `postgres_restore_failed`; `postgres_upgrade_started`, `postgres_upgrade_succeeded`, `postgres_upgrade_failed` | The durable source these were waiting on now exists: `datastore_event_facts` (migration 0107) is the Database/KeyValue twin of `service_event_facts`, written by the control-plane reconciler from typed CR status against a per-datastore checkpoint, so a level-triggered resync records one edge rather than one per tick. Availability arms only after the first observed healthy state (provisioning is not an outage) and a suspended datastore is never reported unavailable. |
| No truthful producer; remain unsupported | `artifact_fetch_failed`, `artifact_source_changed`, `pipeline_minutes_exhausted`; `postgres_cluster_leader_changed`, `postgres_connection_pool_changed`, `postgres_disk_autoscaling_enabled_changed`, `postgres_read_replicas_changed`, `postgres_pitr_checkpoint_started`, `postgres_pitr_checkpoint_failed`, `postgres_pitr_checkpoint_completed`, `postgres_read_replica_stale`, `postgres_wal_archive_failed` | Some underlying products exist, but the outbound feed has no durable, exact transition row carrying the required identity/time/outcome. Advertising these before such a source exists would make the contract untruthful. `postgres_read_replicas_changed` is the deliberate w3/m82 hold-out: read replicas are a **create-time-only** array in bex (`PostgresPatch` has no replica field, and the Blueprint apply path writes `spec.readReplicas` without recording a datastore audit effect), so there is no post-create mutation to source the event from. Reopen when a replica add/remove verb exists. |

After m70, the advertised vocabulary was 32 types; **w8/m34** added Render's three persistent-disk types (`disk_created`/`disk_updated`/`disk_deleted`) now that ADR082 reopened disks, for 35. **w3/m82** adds the fifteen managed-datastore types in the two rows above, for **53** types: 47 overlap the pinned API enum, plus four bex extensions (`branch_changed`, `service_hibernated`, `service_woken`, `service_moved`) and the two Render-prose extensions (`postgres_credentials_created`, `postgres_credentials_deleted`). Cron start/end come from observed durable run state, including scheduled runs; terminal payloads use the checked `succeeded`, `failed`, or `canceled` status. Unsupported families stay visible in this ledger instead of appearing in the picker.

## Final surface result

| Gap found on 2026-08-16/17 | Implemented result |
| --- | --- |
| Create omits required `enabled`, permits an empty/defaulted name, accepts HTTP, and rejects an empty all-events filter. PATCH cannot distinguish omitted from explicitly empty `eventFilter`. | REST create now requires `ownerId`, a unique non-empty name, explicit enabled state and filter, accepts HTTPS only, and stores empty as a future-inclusive all-events subscription. Sparse PATCH distinguishes omission from explicit empty on REST, GraphQL, and MCP. Stable coded refusals include `WEBHOOK_OWNER_REQUIRED`, `WEBHOOK_NAME_INVALID`, `WEBHOOK_NAME_CONFLICT`, and `WEBHOOK_URL_INVALID`. |
| List is unwrapped/unpaged, accepts one owner id, and get/patch expose bex-only fields while omitting Render's post-create secret. | REST returns the exact supported non-secret webhook fields in `[{webhook,cursor}]`, handles repeated `ownerId`, and pages on immutable `(created_at,id)`. The post-create secret omission is deliberately retained. |
| History uses `{delivery,cursor}` and bex field names, lacks time filters and response evidence. | REST returns `[{webhookEvent,cursor}]`, with one row per immutable send attempt, and exposes status/body/error evidence. Time and cursor predicates page stable `(sentAt,id)` values; the bex `status` extension filters attempt outcomes rather than terminal parent state. Request and response evidence is valid UTF-8 and bounded; GraphQL, MCP, and the dashboard retain the richer parent/retry diagnostics. |
| A receiver gets a thin `data.id`, but bex has no global retrieve-by-ID route; finding the full event requires a bounded service-list search and does not work for datastore events. | `GET /v1/events/{eventId}` resolves the unchanged ID through the owner-scoped event index and returns the same event projection as the list surface. GraphQL `serviceEvent(id:)` and MCP `get_service_event` share the lookup; missing and foreign IDs are indistinguishable. |
| The worker has eight backoff delays after the initial delivery, producing nine attempts. | The default has seven delays and exactly eight total attempts. The final attempt is 32h40m30s after the first; the third-failure and terminal-disable notices remain distinct; no ninth POST is possible. |
| Dashboard labels/counts and activity evidence consume the old contract; raw event keys leak in detail/list views. | Create and Settings preserve empty-filter all-events, creation can start disabled, known events use translated human labels, the destination and Settings status are links, creator subjects resolve through the authorized member read, and Activity exposes `sentAt`, status/response/error evidence plus server-side time/status predicates with matching cursor pages. |
| Create/Settings silently disable invalid forms, picker failures collapse into an unusable state, endpoint rows lack search/latest outcome, and mutation controls do not reflect capability. | Accessible field-local validation and coded refusals cover name/URL/event failures; the picker distinguishes loading/error/retry/empty; rows search human labels and show a batched latest immutable outcome with bounded chips; definitive non-managers get read-only list/create/Activity/Settings surfaces while server authorization remains authoritative. |
| “Render event vocabulary” is treated as one moving count despite the authenticated picker and public API disagreeing. | The dated fixture pins the exact 64 picker and 67 OpenAPI arrays plus the exact six API-only/three picker-only differences. The weekly official-schema check diffs the live 67-value enum, while a Go classification test proves every truthful Bex value and every omitted family has exactly one disposition. |
| ADR018 says Render manages webhooks only in the dashboard, marks full parity, calls the worker single-replica, and says endpoint count is unbounded. | ADR018 now records partial parity, current REST management/history, multi-replica-safe dispatch/claims, the fixed 25-endpoint bex cap, and the remaining source/product boundaries. |

## Verification fixture

`TestRenderRESTCreateReadPatchDeleteAndMintOnceFixture`, `TestRenderRESTWebhookListEnvelopeMultiOwnerAndCursor`, and the attempt-history surface fixtures pin the supported Render REST fields, envelopes, filters, pagination, and deliberate create-only secret. Event lookup tests pin the three durable source families, unchanged derived IDs, owner-safe not-found behavior, and REST/GraphQL/MCP parity. Worker tests pin the full attempt clock, immutable retry evidence, manual reservation, and Standard Webhooks reference vector. Real-Postgres migration and two-worker tests verify deterministic backfill, disjoint claims, idempotent Resend, retention cascades, and one history row per signed exchange. `scripts/webhooks-verify.sh` is the executable live fixture: through a caller-supplied public HTTPS tunnel it captures a signed delivery, checks `webhook-id == data.id == retrieved.id`, compares the global result with the service-list event, proves auto-disable separately, then repairs a failing receiver and verifies an idempotent manual Resend with stable event/body identity and fresh signing metadata. It removes every endpoint and service through product APIs. The script still requires an actual run with a public receiver to constitute live evidence; production correctly rejects HTTP destinations and private/loopback SSRF targets.

The remaining product divergences are intentional or source-bound: bex never re-returns a signing secret, caps every workspace at 25 endpoints rather than Render's plan-specific 1/100 gates, exposes GraphQL/MCP management and retry diagnostics that Render does not, and does not advertise the unsupported families listed above.
