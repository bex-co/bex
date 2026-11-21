# w34 · m541 — A Pro-workspace Postgres with at least 10 GB publishes to an external subscriber once logical replication is enabled, as Render enables it

**Worker:** worker34 **Goal:** a bex Postgres on a Pro (or higher) workspace with at least 10 GB of storage can be enabled for logical replication the way Render's support team enables it, after which an external PostgreSQL subscribes to it through its external URL and receives every change. **Status:** blocked (t001–t007, t009–t011 done; t008 and t012 wait for the production pin)

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
| [t008](t008.md) | Live check on dev-N: an external PostgreSQL 18 subscriber through `pg-sni-proxy` | 1h | t005, t006 |
| [t009](done/t009.md) | Render parity — **DONE** | 30m | t007, t008 |
| [t010](done/t010.md) | Simplify — **DONE** | 15m | t009 |
| [t011](done/t011.md) | Test coverage — **DONE** | 45m | t009 |
| [t012](t012.md) | Closeout (live replay on production) | 45m | t010, t011 |

## Progress (2026-11-21, loopx w34)

- **Shipped:** the spec field, the operator's projection (the publisher role's `REPLICATION`, the `bex_all_tables` CloudNativePG `Publication`, the `max_slot_wal_keep_size` bound of a fifth of the disk), the operators' verb `/api postgres-logical-replication` with Render's prerequisites, schema grants, slot cleanup on disable and audit rows, the runbook, the dashboard note and the agent-skill section. Design and evidence: [ADR009 § Logical replication](../../../docs/ADR009-postgresql-management.md#logical-replication-w34m541); the ADR018 row "Logical replication (publisher)" is ◐ with its owners.
- **t001's checks** ran on a local PostgreSQL 18 publisher shaped like a bex tenant, because no cluster can run here (t008): `wal_level` `logical`; the default user's `FOR TABLE` publication succeeds and its `FOR ALL TABLES` is superuser-only; an external subscriber as the default user is refused with `permission denied to start WAL sender`. DoD item 3 quotes PostgreSQL 15's wording of that refusal; PostgreSQL 16 and later, bex's default 18 included, say `permission denied to start WAL sender` (SQLSTATE 42501 in both), and the replay should expect that.
- **Verified off-cluster:** the verb end to end on real PostgreSQL 17 (CI's) and 18 with a real subscription (copy, insert, update, delete), the default user's `FOR TABLE` publication decoding for the publisher, the refusals writing nothing, and disable dropping both slots while the subscription kept reconnecting; a local PostgreSQL 18 pair over TLS replicated both publications to an external subscriber. That local run found that a single terminate-then-drop loses the slot to the subscriber's reconnect within a second, so disable drops each slot in a loop.
- **Left:** t008 (DoD 1–5 on a real cluster) has to run on production, since no dev stack is available to w34; it and t012 follow once the production pin carries this milestone's commits, with `bex-canary` on Pro for the walk under the user's 2026-10-31 permission and back to Hobby afterwards.

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
  - `pg-sni-proxy` never terminates TLS ([ADR009 § External URL](../../../docs/ADR009-postgresql-management.md)), so a replication connection reaches the instance unchanged.
  - No bex role can start a walsender: the managed-roles projection writes no `replication` key (`lego/operator/internal/controller/database_controller.go:358`, `:666`), and the default owner is not a superuser.
  - The operator has no RBAC for CNPG `publications` (`database_controller.go:876`, `lego/operator/config/rbac/role.yaml:200`).
  - Workspace-plan feature gates exist (`lego/backend/internal/core/plan_gate.go:82`, e.g. `FeatureNetworkIsolation: {minimum: planPro}`).
- **Goal linkage:** ADR008 § Render parity first; managed data ([ADR009](../../../docs/ADR009-postgresql-management.md)); a new ADR018 Managed Postgres row (t001).
- **Expected outcome:** CDC tools (Debezium, Fivetran, Airbyte) and external replicas can follow a bex Postgres the way they follow a Render one.
- **Why now:** a documented Render capability with zero coverage. m542 (bex as subscriber) and m543 (lifecycle) build on this milestone's spec field and verb.
- **Decision state:** making bex's operators play Render's support team (an ops-only verb, no public field) is this proposal's recommendation, resting on ADR008 Render parity first. It is not a recorded user decision. bex has no support channel today (the dashboard's "contact support" strings point nowhere), so how a workspace asks is [w34/002](../002.md), which this milestone does not wait on. The `max_slot_wal_keep_size` value is bex's choice until [w34/001](../001.md) captures Render's.
- **Render parity:** included. Logical replication is a tenant-facing SQL capability with a dashboard note.
