# w34 · m541 — A Pro-workspace Postgres with at least 10 GB publishes to an external subscriber once logical replication is enabled, as Render enables it

**Worker:** worker34 **Goal:** a bex Postgres on a Pro (or higher) workspace with at least 10 GB of storage can be enabled for logical replication the way Render's support team enables it, after which an external PostgreSQL subscribes to it through its external URL and receives every change. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Reproduce today's refusal on dev-N; write ADR009 § Logical replication and the ADR018 row | 45m | — |
| [t002](t002.md) | `Database.spec.logicalReplication` in `lego/types`, with CRD codegen | 30m | t001 |
| [t003](t003.md) | Operator: the named role has CNPG `replication: true` only while enabled | 1h | t002 |
| [t004](t004.md) | Operator: `allTables` is a CNPG `Publication` (`FOR ALL TABLES`); RBAC; status | 1h | t002 |
| [t005](t005.md) | Retained WAL is bounded: `max_slot_wal_keep_size` follows the disk while enabled | 45m | t003 |
| [t006](t006.md) | Ops-only enable/disable verb with Render's prerequisites, schema grants, audit and runbook | 1h15m | t003, t004 |
| [t007](t007.md) | The database page and the CLI skill pack say logical replication is enabled on request | 30m | t006 |
| [t008](t008.md) | Live check on dev-N: an external PostgreSQL 18 subscriber through `pg-sni-proxy` | 1h | t005, t006 |
| [t009](t009.md) | Render parity | 30m | t007, t008 |
| [t010](t010.md) | Simplify | 15m | t009 |
| [t011](t011.md) | Test coverage | 45m | t009 |
| [t012](t012.md) | Closeout (live replay on production) | 45m | t010, t011 |

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
