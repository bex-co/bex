# w4 · m379 — A datastore event retrieved by ID answers `details: {}`: Postgres created/restarted name no user, `key_value_config_restart` drops Render's required `reason`/`message`, and the pool, disk, HA, replica and upgrade events drop the values Render requires

**Worker:** worker4 **Goal:** `GET /v1/events/{eventId}` for every Postgres and Key Value event bex emits returns the `details` object Render's schema names for that type, filled from what bex records. A webhook consumer that fetches the event learns who acted and what changed, as on Render. **Status:** todo (t001–t005 done; t006, the live replay, waits on deploy)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Render the datastore details bex already records: `user` on the user-triggered Postgres events, `enabled` on the pool toggle, `reason`/`message` on `key_value_config_restart`, `fromVersion`/`toVersion` on the upgrade events — **DONE** | 45m | — |
| [t002](done/t002.md) | Record and render the from-values Render requires: `fromDiskSize`/`toDiskSize`, `fromStatus`/`toStatus` and `fromReplicas`/`toReplicas` — **DONE** | 45m | t001 |
| [t003](done/t003.md) | Render parity — **DONE** | 20m | t002 |
| [t004](done/t004.md) | Simplify — **DONE** | 15m | t003 |
| [t005](done/t005.md) | Test coverage — **DONE** | 30m | t003 |
| [t006](t006.md) | Closeout (live replay on production) | 20m | t004, t005 |

Total: **6 tasks, about 2h55m**.

## Definition of done

All on production, workspace `bex-canary`. Create a webhook with `eventFilter: []`, so it receives every event, and read each delivery's `eventId` from `GET /v1/webhooks/{id}/events`.

1. **Created:** after `POST /v1/postgres` (Free), `GET /v1/events/<postgres_created id>` answers `details: {"user": {"id": "usr-…", "email": "<caller's email>"}}`.
2. **Restarted:** after `POST /v1/postgres/{id}/restart`, the `postgres_restarted` event's `details.user` names the caller the same way.
3. **Key Value config restart:** after `PATCH /v1/key-value/{id} {"persistenceMode":"off"}` on a Free Key Value, `key_value_config_restart` answers `details` with a non-empty `reason` and a `message` that names the setting that changed.
4. **Unchanged:**
   - `postgres_available` keeps `details: {}`, Render's empty variant;
   - `postgres_unavailable` keeps its `reasonCode`, a recorded bex extra;
   - `postgres_cluster_leader_changed` keeps `leaderId`;
   - every service event's details are byte-for-byte unchanged.
5. **Paid-plan types** (pool, disk size, HA, read replicas, upgrade): covered by t001/t002 unit tests that drive each verb and fetch the event. They are not a live DoD item: they need a paid database, and this hunt never bought one.
6. **Fixtures:** the webhook, the Postgres and the Key Value are deleted, and each GET answers 404.

## Source + Goal linkage

- **Source:** live `/qa-find-bugs w4` loop, round 300, 2026-11-13 14:52–15:05 UTC (`muse.env`, workspace `bex-canary`; pin `5de3d5bcce8e`). The probes are under § Finding.
- **Governing docs:**
  - [ADR018](../../../docs/ADR018-render-parity.md) and [docs/render-artifacts/webhooks-api.md](../../../docs/render-artifacts/webhooks-api.md), the advertised webhook vocabulary;
  - w3/m82 (done), which projected datastore events. Its t003 step 3 required "the event index's `details` carry the resulting value (Render's payload shape per the pinned artifact)". The events service does record those values. The REST mapper never renders them.
- **Goal linkage:** Render parity: Render's webhook payload is thin, and its docs send consumers to Retrieve event for `details`. Managed data ([ADR009](../../../docs/ADR009-postgresql-management.md), [ADR021](../../../docs/ADR021-keyvalue-management.md)).
- **Expected outcome:** a consumer of a Postgres or Key Value event, for example a Slack relay that says who restarted the database, reads Render's fields on bex.
- **Why now:** the cause is one missing set of `case` arms plus three from-values. Nothing guards it, because Render's `details` is a `oneOf` in which several variants (for example "Postgres Available") have no required properties. `{}` therefore validates against the schema for every type.
- **Render parity task included (t003):** the REST event surface changes.

## Finding (round 300)

- **Fixtures** (all three deleted afterwards: `DELETE` 204, then `GET` 404):
  - webhook `qa-20261113-r300-hook` (`whk-dbri8o0k58is73auobfg`), URL `https://example.com/qa-r300`, which answers 405, and `eventFilter: []`;
  - Free Postgres 17 `qa-20261113-r300-pg` (`dpg-dbri8p1ens8c73fghq40`);
  - Free Key Value `qa-20261113-r300-kv` (`red-dbriah9ens8c73fghq70`).
- **Deliveries** (`GET /v1/webhooks/whk-dbri8o0k58is73auobfg/events`): `postgres_created`, then after the restart `postgres_restarted`, `postgres_unavailable` and `postgres_available`, then after the PATCH `key_value_config_restart`. Each was retried once 30 s later after the 405, so delivery works.
- **Retrieve event** (`GET /v1/events/{eventId}`, complete bodies):

  ```
  {"id":"evt-g7k3evo2jkn498kortea","timestamp":"2026-11-13T14:53:24Z","serviceId":"dpg-dbri8p1ens8c73fghq40","type":"postgres_created","details":{}}
  {"id":"evt-611bt8o5al1cko16e0g3","timestamp":"2026-11-13T14:57:03Z","serviceId":"dpg-dbri8p1ens8c73fghq40","type":"postgres_restarted","details":{}}
  {"id":"evt-bt5mrd6pihlrnki84dbs","timestamp":"2026-11-13T14:57:04Z","serviceId":"dpg-dbri8p1ens8c73fghq40","type":"postgres_unavailable","details":{"reasonCode":"readiness_failed"}}
  {"id":"evt-t2goup501mljeaao48mj","timestamp":"2026-11-13T14:58:10Z","serviceId":"dpg-dbri8p1ens8c73fghq40","type":"postgres_available","details":{}}
  {"id":"evt-l987qotm4gdepg5rhtkv","timestamp":"2026-11-13T14:58:36Z","serviceId":"red-dbriah9ens8c73fghq70","type":"key_value_config_restart","details":{}}
  ```

- **Render** (pinned `lego/backend/internal/api/openapi/render-public-api-1.json`, `GET /events/{eventId}` → `details` → "Postgres Event Details" and "Key Value Event Details"):

  | type | Render's `details` | bex today |
  | --- | --- | --- |
  | `postgres_created`, `postgres_restarted` | `user {id, email}` | `{}` (probed) |
  | `key_value_config_restart` | `reason`, `message`, both **required** | `{}` (probed) |
  | `postgres_connection_pool_enabled_changed` | `enabled` **required**, `user` | `{}` (code) |
  | `postgres_disk_size_changed` | `fromDiskSize`, `toDiskSize` **required**, `user` | `{}` (code) |
  | `postgres_ha_status_changed` | `fromStatus`, `toStatus` **required** | `{}` (code) |
  | `postgres_read_replicas_changed` | `fromReplicas`, `toReplicas` **required** | `{}` (code) |
  | `postgres_upgrade_started` / `_succeeded` / `_failed` | `fromVersion`, `toVersion` **required**; `user` on started | `{}` (code) |
  | `postgres_available`, `postgres_unavailable`, backup and PITR types | no properties | `{}`, plus `reasonCode` on unavailable |
  | `postgres_cluster_leader_changed` | `leaderId` | `leaderId` (w2/m171) |

- **Root cause:**
  - **The values are recorded.** `events.Service` maps the audit rows into `Details`:
    - HA → `HighAvailabilityEnabled`, pool → `ConnectionPoolEnabled`, disk size → `DiskSizeGB`, Key Value → `MaxmemoryPolicy`/`PersistenceMode` (`lego/backend/internal/events/service.go:1372-1377`, `:1399-1401`);
    - the caller is on every audit row (`r.Caller`). It feeds `TriggeredByUser` only for `server_restarted` and `disk_snapshot_restored` (`:1345-1346`).
  - **The REST mapper drops them.** `toRenderEvent` (`lego/backend/internal/events/rest.go:316-457`) has no arm for any of these types. Its one datastore arm is `postgres_cluster_leader_changed` (`:419-420`). `renderDetails` already has `Enabled`, `Reason`, `Message` and `LeaderID` (`:135`, `:154-156`, `:176`).
  - **Some from-values are never recorded.** The audit recorders store only the resulting value:
    - `RecordDatabaseHighAvailabilityChanged`, `RecordDatabaseConnectionPoolChanged`, `RecordDatabaseDiskSizeChanged` (`lego/backend/internal/core/audit.go:1058-1096`);
    - the read-replica and disk-autoscaling rows carry no value. The comment at `lego/backend/internal/eventvocab/datastore.go:84-88` says "Render's envelope for them is the thin one". That holds for disk autoscaling, which has no Render detail variant, but Render requires `fromReplicas`/`toReplicas`.
  - **Upgrades** come from datastore facts. The fact's key is `<id>:upgrade:<current>-<spec>:<edge>` (`lego/backend/internal/store/datastore_event_facts.go:461`), so both versions are known. The fact branch (`events/service.go:1419-1461`) does not surface them.
- **Callers of the mapper:** `toRenderEvent` serves `GET /v1/events/{eventId}` (`rest.go:497`) and the service-events list (`toEventList`, `:466-469`). The list serves services only (`GET /v1/services/<dpg-…>/events` answers 404), so the new arms reach datastore events only through Retrieve event. Service-event output must not move (DoD 4).
- **Unverified:**
  - Render's exact `reason`/`message` values for `key_value_config_restart`, and its `fromStatus`/`toStatus` vocabulary for HA. t001 and t002 pick values and record them.
  - Whether Render emits `postgres_available` after a new database first becomes available. bex does not, by design (`nextDatastoreAvailability`, `datastore_event_facts.go:215-232`). It was not filed.
  - The paid-plan types were read from code, not probed.
- **Noted, not filed:**
  - **Restart status.** `POST /v1/postgres/{id}/restart` answers `200` with the database object (`lego/backend/internal/postgres/rest.go:276`, whose comment says "restart 200 — same as services"). Render's pinned spec answers `202` with no body for Postgres restart, as for suspend, resume and failover, while services restart is `200`. It is a one-line status change, to make in t003.
  - **Extra types.** `postgres_credentials_created`/`_deleted` are recorded bex extensions (webhooks-api.md).
- **Dedupe:**
  - w3/m82 (done) built the projection and named this requirement. Its DoD covered availability and backup delivery, not `details`.
  - w2/m171 (done) is the leader event only.
  - w5/m138 (done) added the replica and autoscaling types "with no value".
  - `grep -rln "fromDiskSize\|fromReplicas\|fromStatus" .pm` finds nothing.
  - Not deploy lag: `git log 5de3d5bcce8e..origin/main -- lego/backend/internal/events lego/backend/internal/core/audit.go` lists one commit, `31b6d1682` (w10/m373). It only drops the image from a Git-backed deploy's details, and leaves `toRenderEvent`'s datastore arms as they were. Line numbers above are at `origin/main` `dae0ccfc7`.

## Render parity walk (t003)

Pinned spec, `GET /events/{eventId}` → `details`. The table lists every variant of "Postgres Event Details" and "Key Value Event Details", and what bex answers since w4/m379.

| type | Render's member | bex |
| --- | --- | --- |
| `postgres_available` | empty | `{}` |
| `postgres_unavailable` | empty | `{reasonCode}` (recorded bex extra, unchanged) |
| `postgres_backup_started` / `_completed` / `_failed` | empty | `{}` |
| `postgres_pitr_checkpoint_started` / `_completed` / `_failed` | empty | `{}` |
| `postgres_cluster_leader_changed` | `leaderId` | `{leaderId}` (unchanged) |
| `postgres_connection_pool_enabled_changed` | `enabled` (required), `user` | `{user, enabled}` |
| `postgres_created` | `user` | `{user}`; `{}` for a row with no caller |
| `postgres_restarted` | `user` | `{user}` |
| `postgres_disk_size_changed` | `fromDiskSize`, `toDiskSize` (required), `user` | `{user, fromDiskSize, toDiskSize}`. A row recorded before w4/m379 has no `fromDiskSize` |
| `postgres_ha_status_changed` | `fromStatus`, `toStatus` (required) | `{fromStatus, toStatus}`, `enabled`/`disabled` (bex's words; Render's are unverified) |
| `postgres_read_replicas_changed` | `fromReplicas`, `toReplicas` (required) | `{fromReplicas, toReplicas}`. A row recorded before w4/m379 is `{}` |
| `postgres_upgrade_started` | `fromVersion`, `toVersion` (required), `user` | `{fromVersion, toVersion}`. No `user`: the fact is observed and records no caller |
| `postgres_upgrade_succeeded` / `_failed` | `fromVersion`, `toVersion` (required) | `{fromVersion, toVersion}`. A success recorded before w4/m379 repeats the new version |
| `key_value_available`, `key_value_unhealthy` | empty | `{}` |
| `key_value_config_restart` | `reason`, `message` (required) | `{reason: "config_change", message}`, the message naming the persistence mode |

- **Not emitted by bex:**
  - `postgres_connection_pool_changed` (Render's `from`/`to`): bex's pooler has no mode to change.
  - `postgres_read_replica_stale`: no producer.
  - Both stay in webhooks-api.md's "no truthful producer" row.
- **No details member in Render's union:**
  - `postgres_credentials_created`/`_deleted` are bex extras, not in Render's type enum.
  - `postgres_disk_autoscaling_enabled_changed`, `postgres_restore_succeeded`/`_failed` and `postgres_wal_archive_failed` are in the enum but have no member.
  - All of these answer `{}`.
- **Postgres restart status:** `202`, as Render answers (t003).

## Test record (t001–t005, 2026-11-14)

- `cd lego/backend && GOWORK=off go test ./...` passes, rebased on `fa496cb64`. Before that rebase, `internal/sandbox`'s `TestFileGatewayRefusalBeforeTheFirstByteReachesTheClient` failed on this macOS host, on the unchanged base as well. `fa496cb64` skips it on macOS.
- Real Postgres + OpenFGA (`scripts/backend-test-deps.sh`): `GOWORK=off BEX_TEST_REQUIRE_DEPS=1 go test -count=1 -p 1` over `internal/store`, `internal/events`, `internal/postgres`, `internal/keyvalue`, `internal/core`, `internal/webhooks` and `internal/api` passes. That includes the new `TestPGDatastoreEventFromValuesHydrate` and migration 0260.
