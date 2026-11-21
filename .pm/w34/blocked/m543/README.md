# w34 · m543 — Logical replication survives failover, switchover, restarts, suspend and major-version upgrades

**Worker:** worker34 **Goal:** every lifecycle operation bex offers on a Postgres keeps an enabled database's logical replication correct: a subscriber loses and duplicates no row across a failover or switchover, restarts and suspension keep slots and publications, a major-version upgrade never fails or silently drops slots because of them, and a recovered copy starts not enabled. **Status:** blocked (t001–t005, t007–t009 done; t006 and t010 wait until the production pin carries `19f52132c`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | HA on PostgreSQL 17+: logical slots follow a failover (`synchronizeLogicalDecoding`) — **DONE** | 1h | m541/t003 |
| [t002](done/t002.md) | HA on PostgreSQL 13–16: record the `pg_failover_slots` gap against w8/m92; the verb says subscribers resync after a failover — **DONE** | 30m | t001 |
| [t003](done/t003.md) | Major-version upgrade: refuse by name while a slot lags; say a pre-17 source drops slots — **DONE** | 1h15m | m541/t006 |
| [t004](done/t004.md) | Restarts, suspend and resume, plan change and storage resize keep slots and publications — **DONE** | 45m | m541/t008 |
| [t005](done/t005.md) | A point-in-time recovery starts with logical replication off — **DONE** | 45m | m541/t002 |
| [t006](t006.md) | Drill on dev-N (PostgreSQL 18, HA): fail the primary mid-stream, switch over, upgrade 17→18 | 1h15m | t001, t003, t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007 |
| [t010](t010.md) | Closeout (live replay on production) | 45m | t008, t009 |

## Progress (2026-11-21, loopx w34)

- **Shipped:** an HA publisher on PostgreSQL 17+ gets CloudNativePG's logical slot synchronization (t001); the verb warns how a failover treats an HA publisher's slots, and the HA 13–16 loss is recorded against w8/m92 (t002); a publisher's major-version upgrade is refused by name while a slot lags (`POSTGRES_UPGRADE_LOGICAL_SLOTS_BEHIND`) and from 13–16 accepted with an `upgradeNotice` naming the slots it drops, identically on REST, GraphQL and MCP, with the dashboard's dialog and a warning toast (t003); restarts, suspend, plan changes and resizes need no change (t004); a recovered copy starts not enabled, its source's logical-replication credentials retired and its restored subscriptions held still until it is enabled as a subscriber (t005). Design and evidence: [ADR009 § Lifecycle](../../../../docs/ADR009-postgresql-management.md#lifecycle-w34m543); the ADR018 row "Logical replication (lifecycle)" is ◐ with its owners.
- **Verified off-cluster:** `pg_upgrade` locally on PostgreSQL 16.15, 17.11 and 18.7 binaries with a real subscriber: a 17 source with a lagging slot fails, a caught-up one keeps its slot and the subscriber resumed with every row exactly once, and a 16 source drops the slot silently. Every rule on real PostgreSQL 17 (CI's) in `logical_slots_pg_test.go`, and the operator's projection in `database_logical_lifecycle_test.go`.
- **Found on the way:** a recovered copy's restored subscriptions would compete with the source for its publisher's slot (fixed by t005's worker guard); `tenantParameters` dropped only the canonical spelling of a managed setting (fixed); a subscriber with an asynchronous standby can skip its last transactions at a failover (filed as [w34/003](../../003.md)).
- **Left:** t006 (the drill) and t010 need a real cluster, which w34 does not have, so both run on production in `bex-canary` once the pin carries this milestone's commit `19f52132c` and m542's `b4d4003fc` and m541's `a2c7040f4`.

## Definition of done

On dev-N for the failover drill, and on production in `bex-canary` for the rest, with owned fixtures deleted afterwards (by-id `GET` 404):

1. On an enabled HA PostgreSQL 18 database, a forced failover and a switchover in the middle of a write stream lose and duplicate no row on an external subscriber whose subscription was created with `failover = true`.
2. On HA 13–16, the ADR018 row and the verb's answer say a failover makes subscribers resync, naming w8/m92 as the owner of the fix.
3. Suspend and resume, a plan change and a storage resize keep the database's slots and publications, and the subscriber resumes from its position.
4. A major-version upgrade requested while a logical slot has not consumed all WAL is refused by name on REST `PATCH /v1/postgres/{id}`, GraphQL `updateDatabaseVersion` and MCP `update_postgres`. One from a 17+ source with caught-up slots keeps them. One from a 13–16 source tells the caller the slots will not survive.
5. A database recovered to a point in time reads not enabled for logical replication, whatever its source was.
6. `make test` (from `lego/operator/`) and `cd lego/backend && go test ./...` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for one aspect of render parity (Postgres logical replication), 2026-11-20, item 3; filed by `/pmx them all to w34`.
- **Render reference:** render.com/docs/postgresql-logical-replication (read 2026-11-20) is silent on failover, upgrades and recovery; [w34/001](../001.md) captures Render's behavior. Render documents HA failover, major-version upgrades and point-in-time recovery for every paid Postgres, and bex has each at ✅ in ADR018. This milestone keeps that true for a database that publishes or subscribes.
- **bex today and its components:**
  - CNPG v1.30.1 `docs/src/replication.md`: logical decoding slot synchronization is off by default. Without it, "your logical replication clients may lose data" after a failover. With `replicationSlots.highAvailability.synchronizeLogicalDecoding: true` the operator manages logical slots "during failover and switchover, preventing slot invalidation". PostgreSQL 17+ also needs `hot_standby_feedback: 'on'` and `sync_replication_slots: 'on'`, and the subscription `failover: 'true'`. PostgreSQL 16 and earlier use the `pg_failover_slots` extension, which waits on ADR092's operand image ([w8/m92](../../../w8/blocked/m92/README.md)).
  - Major-version upgrades are CNPG's offline in-place `pg_upgrade` ([ADR009 § Major-version upgrades](../../../../docs/ADR009-postgresql-management.md), `lego/backend/internal/postgres/service.go:1968` `validateVersionUpgrade`). PostgreSQL's `pg_upgrade` migrates logical slots only from a 17+ old cluster, and refuses when a slot has not consumed all WAL (PostgreSQL docs, pg_upgrade § Prepare for publisher upgrades; t003 confirms).
  - Recovery creates a new database (`lego/backend/internal/postgres/recovery.go:341` `Recover`).
- **Goal linkage:** ADR008 § Render parity first; managed-data durability ([ADR009](../../../../docs/ADR009-postgresql-management.md)).
- **Expected outcome:** logical replication on bex is safe to rely on through every operation a user or the platform performs on the database.
- **Why now:** follows m541. Without t001 an HA publisher's subscribers can lose data on the first failover. Without t003 a lagging slot makes an upgrade fail.
- **Decision state:** refusing an upgrade while a slot lags (rather than dropping the slot to let it proceed) is this proposal's recommendation, resting on ADR009's rule that an upgrade never loses data silently; Render's behavior is uncaptured ([w34/001](../001.md)).
- **Render parity:** included. The upgrade refusal changes REST, GraphQL and MCP answers, and the dashboard's version-upgrade control shows it.
