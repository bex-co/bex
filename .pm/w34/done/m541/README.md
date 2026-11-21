# w34 · m541 — A Pro-workspace Postgres with at least 10 GB publishes to an external subscriber once logical replication is enabled, as Render enables it

**Worker:** worker34 **Goal:** a bex Postgres on a Pro (or higher) workspace with at least 10 GB of storage can be enabled for logical replication the way Render's support team enables it, after which an external PostgreSQL subscribes to it through its external URL and receives every change. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Reproduce today's refusal on dev-N; write ADR009 § Logical replication and the ADR018 row — **DONE** | 45m | — |
| [t002](done/t002.md) | `Database.spec.logicalReplication` in `lego/types`, with CRD codegen — **DONE** | 30m | t001 |
| [t003](done/t003.md) | Operator: the named role has CNPG `replication: true` only while enabled — **DONE** | 1h | t002 |
| [t004](done/t004.md) | Operator: `allTables` is a CNPG `Publication` (`FOR ALL TABLES`); RBAC; status — **DONE** | 1h | t002 |
| [t005](done/t005.md) | Retained WAL is bounded: `max_slot_wal_keep_size` follows the disk while enabled — **DONE** | 45m | t003 |
| [t006](done/t006.md) | Ops-only enable/disable verb with Render's prerequisites, schema grants, audit and runbook — **DONE** | 1h15m | t003, t004 |
| [t007](done/t007.md) | The database page and the CLI skill pack say logical replication is enabled on request — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Live check on dev-N: an external PostgreSQL 18 subscriber through `pg-sni-proxy` — **DONE** | 1h | t005, t006 |
| [t009](done/t009.md) | Render parity — **DONE** | 30m | t007, t008 |
| [t010](done/t010.md) | Simplify — **DONE** | 15m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009 |
| [t012](done/t012.md) | Closeout (live replay on production) — **DONE** | 45m | t010, t011 |

## Progress (2026-11-21, loopx w34)

- **Shipped:** the spec field, the operator's projection (the publisher role's `REPLICATION`, the `bex_all_tables` CloudNativePG `Publication`, the `max_slot_wal_keep_size` bound of a fifth of the disk), the operators' verb `/api postgres-logical-replication` with Render's prerequisites, schema grants, slot cleanup on disable and audit rows, the runbook, the dashboard note and the agent-skill section. Design and evidence: [ADR009 § Logical replication](../../../../docs/ADR009-postgresql-management.md#logical-replication-w34m541); the ADR018 row "Logical replication (publisher)" is ◐ with its owners.
- **t001's checks** ran on a local PostgreSQL 18 publisher shaped like a bex tenant, because no cluster can run here (t008): `wal_level` `logical`; the default user's `FOR TABLE` publication succeeds and its `FOR ALL TABLES` is superuser-only; an external subscriber as the default user is refused with `permission denied to start WAL sender`. DoD item 3 quotes PostgreSQL 15's wording of that refusal; PostgreSQL 16 and later, bex's default 18 included, say `permission denied to start WAL sender` (SQLSTATE 42501 in both), and the replay should expect that.
- **Verified off-cluster:** the verb end to end on real PostgreSQL 17 (CI's) and 18 with a real subscription (copy, insert, update, delete), the default user's `FOR TABLE` publication decoding for the publisher, the refusals writing nothing, and disable dropping both slots while the subscription kept reconnecting; a local PostgreSQL 18 pair over TLS replicated both publications to an external subscriber. That local run found that a single terminate-then-drop loses the slot to the subscriber's reconnect within a second, so disable drops each slot in a loop.
- **Left (until the replay below):** t008 (DoD 1–5 on a real cluster) had to run on production, since no dev stack is available to w34; it and t012 waited for the production pin to carry this milestone's commits.

## Unblock record (2026-11-21, loopx w34)

- **Affected:** w34/m541 (t008, t012).
- **Source:** verified evidence, not a user decision. The newest pin on `origin/main`, `b32b3ff43`, pins `cce6c749f6ff`, which carries `a2c7040f4` (`git merge-base --is-ancestor a2c7040f4 cce6c749f6ff` exits 0). Both bex-api pods, the controller manager and the activator run `bex-operator@sha256:e14f8ee4…`, the pin's `controller` digest, and `/api postgres-logical-replication` answers its usage inside the bex-api pod.
- **Behavior and scope:** unchanged. The definition of done stands as written; t008 runs on production as the first half of t012, in `bex-canary` switched to Pro under the user's standing permission (2026-10-31, [w4/done/295](../../../w4/done/295.md)) and back to Hobby afterwards, with the fixtures deleted in the same run.
- **Remaining gates:** none.
- **Next:** walk t008 (DoD 1–5) with the runbook's verb and an external PostgreSQL 18 subscriber, delete the fixtures (by-id `GET` 404), switch `bex-canary` back to Hobby, then close with t012.

## Production replay (2026-11-21, t008 + t012) — DONE

Pin `b32b3ff43` pins `cce6c749f6ff`, which carries `a2c7040f4`; bex-api, the controller manager and the activator ran its `bex-operator@sha256:e14f8ee4…`. The walk ran as the QA user (`scripts/qa-login.sh`, REST and GraphQL on `api.bex.co` with its session) in `bex-canary` (`tea-daif693dqjvc73e7as3g`), with the verb inside the bex-api pod as the runbook says, from 09:40Z to 09:50Z.

- **Plan.** `bex-canary` was already on **Scale** (GraphQL `workspaces { plan }`), above Pro, as every walk since 2026-10-31 has found it, and other sessions were working in it during the walk. So no plan switch was made and none was undone: the workspace reads Scale before and after. The plan steps written above assumed Hobby.
- **Fixtures.** `qa-20261121-m541-pub` (`dpg-dc0me40n82tc73dukfq0`, `basic_256mb`, 10 GB, PostgreSQL 18.6) and `qa-20261121-m541-small` (`dpg-dc0me4d2n01c73bc157g`, 5 GB), both public; a dedicated credential `cdc_reader` from `POST /v1/postgres/{id}/users`; the default user created `items` (100 rows) and `orders` (50 rows) and `CREATE PUBLICATION p2 FOR TABLE orders`. The subscriber was a `postgres:18-alpine` container on this host, with databases `sub1` (both tables) and `sub2` (`orders`), connecting to `dpg-dc0me40n82tc73dukfq0.db.bex.co:5432` with `sslmode=require` (TLSv1.3).

| DoD | Observed on production |
| --- | --- |
| 3 (before enable) | As the default user and as `cdc_reader`, a `replication=database` connection through the external URL and the subscriber's `CREATE SUBSCRIPTION … PUBLICATION p2` were both refused with `FATAL: permission denied to start WAL sender` / `Only roles with the REPLICATION attribute may start a WAL sender process` (PostgreSQL 16+ wording, SQLSTATE 42501). The default user's `CREATE PUBLICATION … FOR ALL TABLES` was refused with `must be superuser to create FOR ALL TABLES publication`; its `FOR TABLE` publication succeeded. |
| 2 | Against the 5 GB database, `enable --role … --schema public --all-tables` exited 1 with `LOGICAL_REPLICATION_STORAGE_TOO_SMALL` (`storageGB` 5, `minStorageGB` 10). Against a Hobby workspace's database (the existing QA fixture `dpg-dbu98feggh5s73emep10` in `tian-personal`, `plan` `hobby`; the plan is checked first, so no fixture of this run's was needed) it exited 1 with `LOGICAL_REPLICATION_PLAN_REQUIRED` ("this workspace is on Hobby"). Each Database CR kept its `resourceVersion`, no `spec.logicalReplication` or `LogicalReplication` condition appeared, no CloudNativePG `Publication` was created, and the workspace audit log has no row for either. |
| 1 | `enable --database dpg-dc0me40n82tc73dukfq0 --role cdc_reader --schema public --all-tables` exited 0 in 25 s: `converged`, `roleReplication` and `publicationLive` true. On the CRs: the spec, the condition `PublicationApplied` ("publication bex_all_tables is applied"), the `Publication` `dpg-dc0me40n82tc73dukfq0-all-tables` applied, the managed role `replication: true`. On the instance: `cdc_reader` `rolreplication` t (the default user f), `bex_all_tables` `puballtables` t owned by `postgres`, and `cdc_reader`'s `IDENTIFY_SYSTEM` through the external URL answered. Subscriptions `s_all` (`bex_all_tables`, `sub1`) and `s_p2` (`p2`, `sub2`) created their slots on the publisher and copied every row. After 50 inserts, 21 updates and 15 deletes on `items` and 30 inserts, 16 updates and 20 deletes on `orders`, both subscriptions reached the publisher's `pg_current_wal_lsn()` and the row counts and an `md5` over every row matched on the publisher, `sub1` and `sub2` (`items` 135, `orders` 60). One audit row, `postgres.EnableLogicalReplication` (actor `system`/`bex-operations`); re-running the same enable answered the same and left the CR's `resourceVersion` unchanged. |
| 5 | Never enabled, both databases read `SHOW max_slot_wal_keep_size` `-1` (the 5 GB one throughout, read on its instance). Enabled, the Cluster carried `max_slot_wal_keep_size: 2048MB` and the instance read `2GB`, with both slots `reserved`. After the disable, the parameter left the Cluster and the instance read `-1` again. No other Database on production has `spec.logicalReplication` set. |
| 4 | `disable --database dpg-dc0me40n82tc73dukfq0` exited 0 in 26 s with `droppedSlots` `["s_all","s_p2"]`. The spec and condition were gone, the `Publication` was deleted and `bex_all_tables` with it (the user's `p2` stays, as the runbook says), the managed role lost `replication`, `cdc_reader` read `rolreplication` f, and `pg_replication_slots` and `pg_stat_replication` were empty. Every reconnect of both apply workers, from 09:48:09Z (while the verb was still waiting) on, failed with `permission denied to start WAL sender`, and a row inserted after the disable did not reach `sub1`. One audit row, `postgres.DisableLogicalReplication`. |
| 6 | CI on `a2c7040f4`: `test (operator)`, `test (backend)` and `test (dashboard)` passed. |

**Cleanup.** The subscriber container was removed; both databases were deleted (`DELETE` 204, the CRs and CloudNativePG clusters gone, by-id `GET` 404 for each at 09:50Z); `bex-canary` still reads Scale; the QA session was logged out (`whoami` 401) and its local state files deleted. The `tian-personal` fixture `dpg-dbu98feggh5s73emep10` is not this run's and was left as it was.

## Definition of done

On production, in `bex-canary` on a Pro (or higher) workspace plan, with owned fixtures deleted afterwards (by-id `GET` 404):

1. A Postgres with at least 10 GB of storage, enabled through the ops verb with a named credential as its publisher role, streams changes to an external PostgreSQL 18 that subscribes through `<id>.db.bex.co:5432` over TLS. Rows inserted, updated and deleted on the publisher appear on the subscriber, both for a `CREATE PUBLICATION … FOR TABLE …` the database's default user ran and for the `FOR ALL TABLES` publication the verb created.
2. The verb refuses, by name and before any write, a database on a Hobby workspace and a database with less than 10 GB of storage.
3. A database that is not enabled still refuses the walsender (PostgreSQL's `must be superuser or replication role to start walsender`), as today.
4. Disable removes the role's `REPLICATION` attribute and the all-tables publication, and the subscriber's connection is refused afterwards.
5. Every enabled database runs with `max_slot_wal_keep_size` set, and a disabled or never-enabled one runs as today.
6. `make test` (from `lego/operator/`), `cd lego/backend && go test ./...` and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for one aspect of render parity (Postgres logical replication), 2026-11-20, item 1; filed by `/pmx them all to w34`. The page was found by listing Render's 224 docs pages (render.com/docs/llms.txt) against every `render.com/docs/<slug>` cited in `docs/`, `.pm/`, `lego/` and `dashboard/src`: `postgresql-logical-replication` is cited nowhere, and no board item, ADR or code covers it.
- **Render reference:** render.com/docs/postgresql-logical-replication, read 2026-11-20. "Render Postgres databases do not enable logical replication by default." Prerequisites: "Your workspace must be on a **Pro** plan or higher" and "Your database must have at least 10 GB of storage." Enabling is not self-service: "To enable logical replication, please reach out to our support team in the Render Dashboard." The request names the `dpg-` id of each database (publishers and subscribers), the role subscribers will use (Render recommends "a dedicated read-only user for this purpose"), the schemas subscribers read, and whether to publish all tables. "The command `CREATE PUBLICATION [name] FOR ALL TABLES` requires superuser privileges and must be executed by our support team", while "Commands of the form `CREATE PUBLICATION [name] FOR TABLE…` can be executed by you." Subscribers "do not need to be Render-managed." Render's pinned OpenAPI (`lego/backend/internal/api/openapi/render-public-api-1.json`) has no logical-replication field.
- **bex today:**
  - CNPG 1.30.1 (`deploy/gitops/base/cnpg-operator.yaml`) sets `wal_level = 'logical'` by default and sets no `max_slot_wal_keep_size` (CNPG v1.30.1 `docs/src/postgresql_conf.md`, `docs/src/replication.md`). bex keeps `wal_level` and `max_replication_slots` operator-managed (`lego/backend/internal/postgres/service.go:1654`).
  - `pg-sni-proxy` never terminates TLS ([ADR009 § External URL](../../../../docs/ADR009-postgresql-management.md)), so a replication connection reaches the instance unchanged.
  - No bex role can start a walsender: the managed-roles projection writes no `replication` key (`lego/operator/internal/controller/database_controller.go:358`, `:666`), and the default owner is not a superuser.
  - The operator has no RBAC for CNPG `publications` (`database_controller.go:876`, `lego/operator/config/rbac/role.yaml:200`).
  - Workspace-plan feature gates exist (`lego/backend/internal/core/plan_gate.go:82`, e.g. `FeatureNetworkIsolation: {minimum: planPro}`).
- **Goal linkage:** ADR008 § Render parity first; managed data ([ADR009](../../../../docs/ADR009-postgresql-management.md)); a new ADR018 Managed Postgres row (t001).
- **Expected outcome:** CDC tools (Debezium, Fivetran, Airbyte) and external replicas can follow a bex Postgres the way they follow a Render one.
- **Why now:** a documented Render capability with zero coverage. m542 (bex as subscriber) and m543 (lifecycle) build on this milestone's spec field and verb.
- **Decision state:** making bex's operators play Render's support team (an ops-only verb, no public field) is this proposal's recommendation, resting on ADR008 Render parity first. It is not a recorded user decision. bex has no support channel today (the dashboard's "contact support" strings point nowhere), so how a workspace asks is [w34/002](../../002.md), which this milestone does not wait on. The `max_slot_wal_keep_size` value is bex's choice until [w34/001](../../001.md) captures Render's.
- **Render parity:** included. Logical replication is a tenant-facing SQL capability with a dashboard note.
