# w6 · m458 — Managed PgBouncer runs with Render's settings, and read replicas are pooled too

**Worker:** worker6 **Goal:** a database with connection pooling accepts up to 30,000 clients on its pooled URL, keeps at most `max_connections` − 10 server connections, closes a client idle for a day and retries a failed server login after 2 s, as Render's managed PgBouncer does, and its read replicas are pooled as Render pools each replica. **Status:** blocked: t001–t004, t006–t008 and t010 done. t005's 1,000-client check failed on production on PgBouncer's open-file limit; t010 fixes it, and t005's re-run and closeout wait for the production deploy carrying that fix (see BLOCKED — 2026-11-17, second).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Record what the pooler runs today](done/t001.md) — **DONE** | 30m | — |
| t002 | [`poolerSpec` sets Render's PgBouncer settings](done/t002.md) — **DONE** | 1h | t001, w6/134 |
| t003 | [The pooler pod can hold 30,000 clients](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Read replicas are pooled, as Render pools each replica](done/t004.md) — **DONE** | 1h30m | t002 |
| t005 | [Verify 1,000 pooled clients on production; ADR009 and ADR018](t005.md) | 45m | t003, t004 |
| t006 | [Render parity](done/t006.md) — **DONE** | 30m | t005 |
| t010 | [PgBouncer starts with the container's hard open-file limit](done/t010.md) — **DONE** | 30m | t003 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008, t010 |

## Definition of done

- On production, a `basic-1gb` probe database with pooling accepts 1,000 concurrent clients on its pooled URL while `SHOW POOLS` holds at most 90 server connections, and PgBouncer's `SHOW CONFIG` reads `max_client_conn` 30000, `default_pool_size` and `max_db_connections` 90, `client_idle_timeout` 86400 and `server_login_retry` 2.
- A plan change rewrites the pool size without dropping connected clients.
- A database with a read replica has pooled replica connections in the form t004 records from Render's contract, consistently on REST, GraphQL, MCP and the dashboard.
- ADR009 and ADR018 record the settings.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-17, item 4, filed by the user's `/pm them to w6`. Render: [Postgres connection pooling](https://render.com/docs/postgresql-connection-pooling), read 2026-11-17: transaction pooling; `max_client_conn` 30000; `max_db_connections` / `default_pool_size` "`max_connections` minus 10" ("Render Postgres reserves 10 connections for direct database connections and internal operations"); `client_idle_timeout` 86400; `server_login_retry` 2; port 6432; pooling is enabled for every read replica, each with its own pool.
- **bex today:** `poolerSpec` (`lego/operator/internal/controller/database_controller.go:694`) sets one PgBouncer instance with `type: rw` and `poolMode: transaction`, and no other setting, so the pooler runs CNPG's or PgBouncer's defaults (PgBouncer's are `max_client_conn` 100 and `default_pool_size` 20; t001 reads what actually runs). Replica endpoints have no pooler.
- **Goal linkage:** ADR008 § Render parity first.
- **Expected outcome:** pooling does what tenants turn it on for, many clients on few server connections, with Render's numbers.
- **Why now:** if t001 confirms the defaults, a pooled database refuses its 101st client where Render accepts 30,000, which is the case pooling exists for. w6/134 supplies the pool size's input. Free databases already cannot pool (w8/m50).
- **Render parity task:** included. t004 may add pooled replica strings to connection info on every surface.

## t001 — what the pooler ran (2026-11-17)

On dev-6, a `basic-1gb` probe with `pooler: true` (CNPG 1.30.1, `ghcr.io/cloudnative-pg/pgbouncer:1.25.2`, the versions production runs). The generated `pgbouncer.ini` set `pool_mode` and CNPG's own auth and TLS settings, nothing else. `SHOW CONFIG` on the admin socket:

| setting | Render | running | on CNPG 1.30.1's allowlist |
| --- | --- | --- | --- |
| `max_client_conn` | 30000 | 100 | yes |
| `default_pool_size` | `max_connections` − 10 (90) | 20 | yes |
| `max_db_connections` | `max_connections` − 10 (90) | 0 (unlimited) | yes |
| `client_idle_timeout` | 86400 | 0 | yes |
| `server_login_retry` | 2 | 15 | yes |

All five are reloadable (`SHOW CONFIG` `changeable` yes). The allowlist is `AllowedPgbouncerGenericConfigurationParameters` in CNPG v1.30.1's `internal/webhook/v1/pooler_webhook.go`. Production had no Pooler at all on 2026-11-17: 4 databases, none pooled. w6/134's input, the plan's `maxConnections`, was already in `tiers` (w6/m457 t002). The server runs PostgreSQL's 100, which equals it on every current plan, so t002 did not wait for 134's server-side half.

## t004 — Render's contract for a replica's pooled connection

- **Pinned OpenAPI** (`render-public-api-1.json`): `postgresConnectionInfo` has `internalConnectionPoolString` and `externalConnectionPoolString`; `readReplica` is `{id, name}`, with no connection fields of its own.
- **Docs** (read 2026-11-17): "Render automatically enables it for all of that database's read replicas"; "Each read replica has its own independent connection pool"; pooling cannot be toggled per replica; a replica is connected to "just like you do your primary instance, using its internal or external connection URL"; pooled URLs are the direct ones on port 6432. Render does not document a replica's pooled hostname.
- **bex's form.** bex's named replicas share one reader Service (`<id>-ro`, ADR018's HA row), so one CNPG Pooler `type: ro`, `<id>-pooler-ro`, serves them all with the primary's settings and one replica's pool. Each replica's status, `connection-info` `readReplicaConnectionStrings[]` and view `connectionInfo` carry its pooled internal (`<id>-pooler-ro.<ns>.svc:6432`) and external (`<id>-pool-ro.<domain>:5432`, the primary's external pooled port) connection, on REST, GraphQL and MCP; the dashboard's pooled reveal lists them per replica. Per-replica pools, which need per-replica endpoints, are filed as [w6/137](../../done/137.md), promoted to [m467](../../m467/README.md).

## Progress — 2026-11-17 (t001–t004, t007, t008)

- **t002.** `pgbouncerParameters` puts Render's five settings on every Pooler, with the pool from `tiers.PostgresTier.PoolerPoolSize` (`maxConnections` − 10). On dev-6, 1,000 `pgbench` clients through a pooler with these settings held exactly 90 server connections and 910 waiting clients. Resizing the pool mid-run (90 → 50) issued `RELOAD` without rolling the Pod, and the run ended with 0 failed transactions.
- **t003.** The container's file limit is 524,288 on production tenant nodes (read in a tenant Postgres Pod; containerd 2.3.3) and 1,073,741,816 on kind. PgBouncer logs `max expected fd use: 30012`. Memory with TLS clients: 23 MiB idle and 143 MiB with 3,000 connected, so about 1.2 GiB at 30,000. `poolerTemplate` now sets 100m / 256 Mi requests and 2 / 2 Gi limits; CNPG 1.30.1 merged them into its container on dev-6. Pooler Pods carry `bex-datastore`, which `tenant-compute-quota` excludes. Recorded in ADR009 § Managed PgBouncer settings.
- **t004.** On dev-6, a replica added to the probe and the operator's `ro` Pooler spec applied: through `<id>-pooler-ro:6432`, `pg_is_in_recovery()` read `t` from the replica's Pod IP, a row written through the primary's pooler was read back, and an `INSERT` was refused as read-only. The pg-sni-proxy routes `<id>-pool-ro`, the server certificate names it, and both poolers go away with pooling or with the last replica.
- **t007.** `/simplify`: the dashboard's four pooled fields share one helper; the pooler names stay literal in pg-sni-proxy, as `-pool` already was.
- **t008.** `TestEveryPlansPoolerRunsRendersPgBouncerSettings`, `TestReadReplicasArePooledWithTheirDatabase` (a plan change rewrites the pool without touching the pod template), the proxy's resolve/index/capacity tests, `TestPooledDatabasesReplicasHavePooledConnections` (REST, GraphQL, MCP; pooling off) and the dashboard hook and panel tests. Each failed with the code it covers reverted.
- **Shared-cluster limit.** dev-6 shares the cluster's bex operator, so the operator code itself was not deployed there. The dev-6 checks ran its generated Pooler specs on CNPG; envtest and the unit tests cover the reconcile.
- Probe resources on dev-6 were deleted.

## BLOCKED — 2026-11-17: the production deploy carrying this milestone's commit

- **Done.** t001–t004, t007 and t008 (above), plus ADR009 § Managed PgBouncer settings and ADR018's rows marked implemented, production pending.
- **Gate (time, clears by itself).** A push or scheduled `deploy.yml` run deploys `main` once the production pin is 6 h old, and a dispatched run deploys at once.
- **How to check it cleared.** `git log origin/main --grep "pin platform images"` shows a pin whose producer contains this milestone's commit. On production, `kubectl get crd databases.app.bex.co -o yaml | grep -c poolerExternalHost` reads 2 (the status field on the database and on each replica).
- **Then.**
  - t005 with probes in `bex-canary`: a `basic-1gb` database with pooling; from a probe service, 1,000 concurrent clients on the internal pooled URL; `SHOW POOLS` holds at most 90 server connections and `SHOW CONFIG` reads Render's five settings; add a replica and read replica data through its pooled URL; delete the probes. ADR018's rows get the evidence.
  - t006 reads connection info for the primary and its replica on REST, GraphQL, MCP and the dashboard on production.
  - t009 closes.

## UNBLOCKED — 2026-11-17: the production deploy carrying `ba5357084` landed

- **Affected:** w6/m458 t005, t006, t009. **Source:** verified evidence, read by the `/loopx w6` worker at 21:50Z.
- **The pin.** `e36296ae8` ("pin platform images to e0a5ba20aac8", 18:33Z) is on `origin/main`, and `git merge-base --is-ancestor ba5357084 e0a5ba20aac8` holds. Production's `bex-controller-manager`, `bex-api` and `bex-pg-sni-proxy` run `bex-operator@sha256:f7b06e9c…`, the digest that pin wrote to `deploy/gitops/base/bex.yaml`, and the dashboard runs the pinned `bex-dashboard@sha256:76d3a40b…`.
- **The CRD.** On production, `kubectl get crd databases.app.bex.co -o yaml | grep -c poolerExternalHost` reads 2.
- **Remaining gates:** none. **Next:** t005's production check in `bex-canary`, t006 on every surface on production, t009, as listed under BLOCKED — Then.

## Production check — 2026-11-17 (t005 in part, t006, t010)

- **Settings and replicas hold on production.** A `basic_1gb` probe (`dpg-dbucp4g8vt4s73a1ougg`, pooled, replica `r1`) in `bex-canary`: both PgBouncers' `SHOW CONFIG` read Render's five settings; a client read the replica's data through `<id>-pooler-ro:6432` and was refused an `INSERT`; both external pooled names connected. Details: [t005](t005.md).
- **1,000 clients did not.** All 1,000 connected, but PgBouncer ran at the container's open-file soft limit of 1,024 (`kernel file descriptor limit: 1024 (hard: 524288)`), logged `accept() failed: Too many open files` and held 10 server connections of its 90 at 63 transactions per second. t003 had read the hard limit, and kind's soft limit (1,073,741,816) hid it on dev-6.
- **Fixed** ([t010](done/t010.md)). `poolerTemplate` starts PgBouncer through `pgbouncerRunCommand`, which raises the soft limit to the hard one and `exec`s CNPG's manager. In production Poolers with that command PgBouncer ran at 524,288, and the same 1,000 clients held 87 server connections (the database's other 10 tenant slots were two poolers' `auth_query` sessions) with 913 waiting, at 412 transactions per second, 0 failed. ADR009 § Managed PgBouncer settings and ADR018 carry the correction and the evidence.
- **Connection info on every surface** ([t006](done/t006.md)). REST, GraphQL, MCP and the dashboard show the same pooled strings and pool hosts for the primary and `r1`; no new drift.
- **Cleanup.** The probe was deleted by ID (`DELETE` 204 at 22:14Z, then `GET` 404); the load pod `m458-pgbench` and the test Poolers `m458-nofile-test` and `m458-nofile-test2` were deleted and read `NotFound`. Production runs no Pooler.

## BLOCKED — 2026-11-17 (second): the production deploy carrying t010's fix

- **Done.** t001–t004, t006–t008 and t010; t005's settings, replica and record steps.
- **Gate (time, clears by itself).** A push or scheduled `deploy.yml` run deploys `main` once the production pin is 6 h old (pin `e36296ae8` from 18:33Z), and a dispatched run deploys at once.
- **How to check it cleared.** `git log origin/main --grep "pin platform images"` shows a pin whose producer contains the commit that adds `pgbouncerRunCommand` (`git log origin/main -S pgbouncerRunCommand`). On production, the controller's image is that pin's digest.
- **Then.**
  - t005's re-run in `bex-canary`: a new `basic-1gb` pooled probe; its PgBouncer logs `kernel file descriptor limit: 524288`; 1,000 concurrent clients on the internal pooled URL, with `SHOW POOLS` at most 90 server connections and no failed transaction; delete the probe by ID and confirm `GET` answers 404. ADR009's production-check bullet and ADR018's row drop "pending".
  - t009 closes.

