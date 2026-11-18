# w6 · m467 — Each read replica has its own endpoint and connection pool, as on Render

**Worker:** worker6 **Goal:** each named read replica is one reader of its own. Its internal and external URLs reach only its own instance, never the primary or the HA standby, and it stays the same instance across reconciles and failovers. With pooling on, each replica has its own PgBouncer with a pool of `max_connections` − 10, so two replicas hold 90 server connections each at once, as Render's do. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Choose how a replica gets its own reader and pool on CNPG 1.30.1, prove it on dev-6, record it in ADR009](t001.md) | 1h30m | — |
| t002 | [Each named replica has its own reader instance and internal URL](t002.md) | 2h | t001 |
| t003 | [Each pooled replica has its own PgBouncer, sized `max_connections` − 10](t003.md) | 1h30m | t002 |
| t004 | [External hostnames reach each replica's own reader and pooler](t004.md) | 1h | t002, t003 |
| t005 | [dev-6: two replicas hold 90 server connections each; ADR009 and ADR018](t005.md) | 1h | t004 |
| t006 | [Production: the same on a probe in `bex-canary`](t006.md) | 45m | t005 |
| t007 | [Render parity](t007.md) | 30m | t005 |
| t008 | [Simplify](t008.md) | 15m | t007 |
| t009 | [Test coverage](t009.md) | 45m | t007 |
| t010 | [Closeout](t010.md) | 10m | t006, t008, t009 |

## Definition of done

- On dev-6 and on a production probe, a database with HA, two named replicas `a` and `b` and pooling holds 90 server connections on each replica at once through the two pooled URLs (`SHOW POOLS` on each replica's PgBouncer; `pg_stat_activity` on each replica's instance), with no failed transaction.
- `<id>-ro-a`'s direct and pooled URLs, internal and external, read only replica `a`'s instance (`inet_server_addr()` / `pg_is_in_recovery()` is `t`), and `b`'s only `b`'s. The HA standby and the primary serve no replica URL.
- A switchover or a primary failure leaves every replica URL on a non-primary instance that is not the HA standby, and a planned failover (`POST /v1/postgres/{id}/failover`) promotes the HA standby, never a replica.
- Adding or removing a replica, turning pooling on or off and changing the plan roll no primary Pod. Removing one replica leaves the other's connections up. Each change that restarts or replaces anything is listed in ADR009.
- REST, GraphQL, MCP and the dashboard show each replica's own internal, external and pooled hosts.
- ADR009 (§ Named read replicas and § Managed PgBouncer settings) and ADR018's HA row describe per-replica readers and pools, and no longer say "names remain aliases to a shared reader pool". If t001 finds Render's behavior cannot be reached, ADR018 records the shared pool with the user's sign-off instead, and the milestone is parked for it.

## Source + Goal linkage

- **Source:** promoted from [w6/137](../done/137.md) on 2026-11-17. w6/m458 t004 filed it: m458 pooled the replicas with one CNPG `Pooler` of `type: ro` (`ba5357084`), and its open-file fix is `89d8ed7fd`. w6/134 (`d0e19225e`) sets each plan's `max_connections`. This milestone builds on both.
- **Render** ([connection pooling](https://render.com/docs/postgresql-connection-pooling) and [read replicas](https://render.com/docs/postgresql-read-replicas), read 2026-11-17): "Each read replica has its own independent connection pool"; a replica is connected to "just like you do your primary instance, using its internal or external connection URL"; pooled URLs are the direct ones on port 6432; Render's `readReplica` schema has its own `id`. Render's HA standby is a separate instance from its read replicas.
- **bex today (checked at `75053c6e6`):**
  - Replicas are extra instances of the database's one CNPG Cluster (`databaseInstanceCount`, `lego/operator/internal/controller/database_instances.go:28`). Every replica's `internalHost` is the shared `<id>-ro.<ns>.svc` (`updateExternalAddressStatus`, `database_controller.go:2078`), and pg-sni-proxy routes every `<id>-ro-<name>` label to `<id>-ro` (`lego/operator/cmd/pg-sni-proxy/main.go:213`). CNPG's `-ro` Service selects every standby, the HA standby included.
  - With pooling, `reconcilePooler` (`database_controller.go:1416`) runs one CNPG `Pooler` `type: ro`, `<id>-pooler-ro`, sized as one replica's pool (90 on every current plan) and shared by every name, with one public label `<id>-pool-ro` (`replicaPoolerLabel`, `:1090`).
  - `triggerCNPGSwitchover` (`:2051`) promotes any non-primary instance, so a planned failover can promote a read replica's instance; CNPG's automatic failover can too.
  - The CNPG `Pooler` CRD on production (CNPG 1.30.1) takes `type` `rw`, `ro` or `r` only, with no instance target. Its `Cluster` CRD has `spec.replica` (`enabled`, `source`, `primary`, `self`, `promotionToken`, `minApplyDelay`), which replica clusters use.
  - The replica id `<id>-ro-<name>` is capped at one DNS label (`readReplicaNameMaxLength`, `lego/backend/internal/postgres/admission.go:138`), so a per-replica name cannot just append `-pool` (ADR009 § Named read replicas).
  - The backend builds connection strings from the replica statuses (`lego/backend/internal/postgres/service.go:1200-1222`, `:1797`), so new hosts reach every surface through status.
- **Restarts.** On production, none: its 5 databases have no replica, no HA and no pooling, and no `Pooler` exists (read at 2026-11-18 00:05Z). The change must still roll no primary Pod anywhere. The shared `<id>-pooler-ro` is replaced, so its clients reconnect; production runs none. t001 records what each candidate does to an existing database with replicas (option B recreates its readers as new clusters).
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)). ADR018's HA row records "names remain aliases to a shared reader pool" without a user sign-off, so the gap stands open.
- **Expected outcome:** a tenant with two replicas gets twice one replica's read capacity and pool, as on Render, and a replica URL always reads the replica it names.
- **Why now:** m458 and 134 shipped the pool sizing and the plan's `max_connections` on 2026-11-17. With replica pooling live, a second replica adds no pool, and a replica URL can read the HA standby. No production database has replicas yet, so the topology can change before any tenant depends on it.
- **No user decision gates the code.** Under Render parity first, per-replica readers and pools are the parity choice; t001 picks the mechanism.
- **Render parity included:** connection info on REST, GraphQL, MCP and the dashboard changes per replica.
- **Out of scope:** replica plans, counts and eligibility (unchanged). The edge load balancer's connection ceiling (w6/136).
