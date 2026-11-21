# Runbook: enable a Postgres for logical replication (publisher)

**What it is:** Render enables logical replication only through its support team, on request ([render.com/docs/postgresql-logical-replication](https://render.com/docs/postgresql-logical-replication)). bex's operators do the same with one command inside the bex-api pod. It reaches bex-api's cluster-internal verb (`POST /internal/postgres-logical-replication` on `:8091`, behind `BEX_CP_TOKEN`); no tenant surface has it. Design: [ADR009 § Logical replication](../ADR009-postgresql-management.md#logical-replication-w34m541). Board: `w34/m541`.

## The request

Render's request names four things. Collect them from the workspace before you start:

| Input | Flag | Notes |
| --- | --- | --- |
| The database's `dpg-` id | `--database` | One command per database. |
| The role subscribers connect as | `--role` | Any live credential of the database: the owner, a default credential, or a user added with `POST /v1/postgres/{id}/users`. Render recommends a dedicated read-only user. |
| The schemas subscribers read | `--schema` (repeat it) | The role is granted `USAGE` and `SELECT` on each, including tables created later. Name every schema that holds a published table: the subscriber's initial copy reads each table as the role. |
| Publish all tables? | `--all-tables` | Creates the `FOR ALL TABLES` publication `bex_all_tables` in the primary database, which only a superuser can. Without it the user publishes with `CREATE PUBLICATION … FOR TABLE …` themselves. |

The verb checks Render's prerequisites and refuses, writing nothing, when one fails:

| Code | Meaning |
| --- | --- |
| `LOGICAL_REPLICATION_PLAN_REQUIRED` | The workspace is below Pro (or its plan could not be read). |
| `LOGICAL_REPLICATION_FREE_PLAN` | The database is on the free plan. |
| `LOGICAL_REPLICATION_STORAGE_TOO_SMALL` | The database has less than 10 GB of storage. Increase its disk first. |
| `LOGICAL_REPLICATION_ROLE_UNKNOWN` | `--role` is not a live credential of the database (a deleted user counts as gone). |
| `LOGICAL_REPLICATION_SCHEMA_UNKNOWN` | A `--schema` does not exist. |
| `LOGICAL_REPLICATION_DATABASE_UNAVAILABLE` | The database is suspended, being deleted, or not available right now. |
| `LOGICAL_REPLICATION_PUBLICATION_NAME_IN_USE` | The user already has a publication named `bex_all_tables`. Ask them to rename it. |

## Enable

```bash
kubectl -n bex-system exec deploy/bex-api -- /api postgres-logical-replication enable \
  --database dpg-… --role cdc_reader --schema public [--schema sales] [--all-tables]
```

The command waits up to two minutes for PostgreSQL itself to report the change and prints the verb's JSON answer. Exit 0 means `converged`: `roleReplication` is true and, with `--all-tables`, `publicationLive` is true. Exit 3 means the change is written but the database has not reported it yet. `message` carries the operator's `LogicalReplication` condition. Run the same command again to check: a repeated request writes nothing and records nothing new. Exit 1 is a refusal (see the table above) or a failure.

What changes: `Database.spec.logicalReplication` is set; the operator gives the role CloudNativePG's `replication: true`, sets `max_slot_wal_keep_size` to a fifth of the database's storage, and with `--all-tables` declares the CloudNativePG `Publication` `<id>-all-tables`. One audit row, `postgres.EnableLogicalReplication`, is recorded on the database.

Tell the user:

- Subscribers connect to the database's **external** URL (`externalHost` in the answer, port 5432, TLS) as the role, never the pooled URL: PgBouncer cannot carry a replication connection. The database must be public, with the subscriber's address allowed if it has inbound IP rules.
- They create `FOR TABLE` publications themselves as the database's default user, and subscribe to those or to `bex_all_tables`.
- The subscriber's tables must exist before `CREATE SUBSCRIPTION`: logical replication does not copy DDL.
- A slot that falls more than a fifth of the disk behind is invalidated (`wal_status = 'lost'` in `pg_replication_slots`) and its subscription has to be re-created.

## Check

```bash
kubectl -n <workspace namespace> get databases.app.bex.co dpg-… -o jsonpath='{.spec.logicalReplication}{"\n"}{.status.conditions[?(@.type=="LogicalReplication")]}{"\n"}'
kubectl -n <workspace namespace> get publications.postgresql.cnpg.io dpg-…-all-tables
```

## Disable

```bash
kubectl -n bex-system exec deploy/bex-api -- /api postgres-logical-replication disable --database dpg-…
```

The verb drops the database's logical replication slots (ending the walsenders that hold them), clears the spec, waits for the role to lose `REPLICATION` and the all-tables publication to go, and ends any walsender left, so subscribers' next connections are refused. `max_slot_wal_keep_size` returns to PostgreSQL's default. The answer lists `droppedSlots`. A slot in `remainingSlots` still holds back WAL with no bound: enable the database again and disable it once more, which retries the drop. The schema grants stay; they are ordinary privileges the user may revoke. One audit row, `postgres.DisableLogicalReplication`, is recorded.

Tell the user to drop their subscriptions. Their publications stay.
