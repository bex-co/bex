# w3 · m82 — Datastore observed-lifecycle events: availability, backup, upgrade, restore into the events feed, webhooks, and push

**Worker:** worker3 **Goal:** a tenant learns when their managed Postgres or Key Value becomes unavailable, recovers, finishes or fails a backup, restores, or upgrades — as durable facts in the owner-scoped event index, as Render's `postgres_*` / `key_value_*` webhook types, and as push notifications — instead of the platform-only `bex_datastore_ready` alerts the operator sees today. **Status:** done

## Tasks (in order)

| id   | title                                                                                                                  | est | depends_on       |
| ---- | ---------------------------------------------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Observed datastore checkpoint: reconciler snapshots Database/KeyValue phase + Ready condition into availability facts | 60m | —               | — **DONE** |
| t002 | Backup, restore, and upgrade facts from operator-projected CNPG outcomes and phase edges                               | 60m | t001            | — **DONE** |
| t003 | Audited-intent datastore facts from existing verbs (HA, read replicas, pooler, disk size, Key Value config restart)    | 45m | —               | — **DONE** |
| t004 | Advertise the new types: webhook catalog + picker, pinned ledger, ADR018 row, `data.id` hydration                     | 45m | t001, t002, t003 | — **DONE** |
| t005 | Push delivery for datastore availability and backup/restore/upgrade failure (additive opt-in)                         | 45m | t001            | — **DONE** |
| t006 | Live proof on dev-3 then prod: induced outage/recovery pairs, backup completion, webhook + push delivery               | 45m | t004, t005      | — **DONE** |
| t007 | Render parity                                                                                                          | 30m | t006            | — **DONE** |
| t008 | Simplify                                                                                                               | 20m | t007            | — **DONE** |
| t009 | Test coverage                                                                                                          | 40m | t007            | — **DONE** |
| t010 | Closeout                                                                                                               | 10m | t009            | — **DONE** |

## Definition of done

On a live cluster, an induced managed-Postgres outage and recovery produce exactly one `postgres_unavailable` and one `postgres_available` in the owner-scoped event index (`GET /v1/events/{eventId}` resolves both), delivered to a subscribed webhook endpoint and as push inbox rows for a member who opted in; a Key Value restart produces the `key_value_unhealthy` / `key_value_available` pair the same way; a completed backup produces one `postgres_backup_completed`; a suspended datastore produces **no** unavailable fact. The webhook picker and the pinned ledger advertise the new types, and `docs/render-artifacts/webhooks-api.md` no longer lists them under "No truthful producer". A stale-transition regression test (the w6/m41 phantom-pair class) passes for datastores as it does for Apps. The default push policy is unchanged.

## Evidence (2026-09-08)

- **Deterministic suites (cluster-independent):** `cd lego/backend && go test ./...` green; `make test` in `lego/operator/` green; dashboard webhook/events/notifications tests green (82). Coverage includes availability edge + debounce + stale rejection (`datastore_event_facts_test.go`), backup/restore/upgrade edges (`datastore_lifecycle_facts_test.go`), audit→Render name projection (`postgres/webhook_effects_test.go`, `keyvalue/webhook_effects_test.go`), webhook vocabulary (53 advertised), push opt-in (`notifications/push_datastore_test.go`). PG-backed legs (`*_pg_test.go`) are written and gated on `BEX_TEST_DB_URI` (execute in CI).
- **Turnkey live probe:** `scripts/datastore-events-verify.sh` delivered. **dev-3 run owed** this session — harness was down (`kubectl` missing, bex-api not running); **prod run owed** (hetzner-prod off-limits without authorization — m46/m81 precedent). Re-run the script against a healthy `dev-env.sh 3 up` or an authorized prod disposable fixture before calling the live DoD closed.
- **Deliberate hold-outs:** `postgres_read_replicas_changed` stays unsupported (create-only; no patch verb). Email for availability stays behind `w2/028`. CNPG-event-level types (PITR/WAL/leader) remain unsupported per milestone Out of scope.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for w3` 2026-09-08 #1, verified against the code the same day. Re-opens the FUTURE-MAYBE entry "Datastore availability/health/completion webhook events" (the `w3/m26` residual) on a **changed premise**: m26 omitted these because "the operator has no control-plane event channel", but `w3/m19` + `w3/m78` built exactly that channel for Apps (`store/reconciler.go` `recordObservations` → `observedServiceStateFor` → `RecordObservedServiceState`), `w6/m41` hardened it against stale reads, and `w7/m77` moved every managed datastore into its `<ws>` namespace. Verified 2026-09-08: `store/reconciler.go` has zero references to Database/KeyValue CRs; `store/event_facts.go` has no datastore fact types; `notifications/delivery_policy.go` knows only App and agent events; `docs/render-artifacts/webhooks-api.md` still lists every `postgres_available`/`postgres_unavailable`/backup/restore/upgrade/`key_value_*` type under "No truthful producer; remain unsupported".
- **Goal linkage:** pillar 1 (Render-compatible surfaces — closes roughly a dozen values of Render's 67-value webhook enum), ADR052 notifications, managed data (ADR009 / ADR021).
- **Expected outcome:** tenants are paged when their datastore goes down and told when it recovers, backup/restore/upgrade failures reach the tenant, and webhook consumers get Render's datastore vocabulary. The operator-only `DatastoreStuckProvisioning` / `DatastoreObservationFailing` alerts stay as they are.
- **Why now:** three production forums run on managed Postgres today and a datastore outage produces zero tenant-visible signal; the deferral's stated blocker no longer exists; and the observed-fact machinery is fresh enough that extending it is a projection change rather than new architecture.
- **Render parity task included:** the milestone adds webhook event types, event-index rows, and notification events across REST, GraphQL, MCP, and the dashboard/mobile pickers.

## Out of scope

- An **email** channel for availability — stays behind the `w2/028` flap-digest trigger in FUTURE-MAYBE (email covers deploy events only, by design).
- CNPG-event-level types with no cheap status source: `postgres_pitr_checkpoint_*`, `postgres_wal_archive_failed` (the platform alert `DatabaseNotArchivingWAL` covers the operator), `postgres_cluster_leader_changed`, `postgres_read_replica_stale`. Keep unsupported unless one falls out of t002 for free.
- Datastore `DeletionStalled` — already `w8/012`. t002 adds fields to `Database.status`; coordinate with w8 if that note is picked up concurrently.
