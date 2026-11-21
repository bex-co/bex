# Runbook: enable a Postgres for logical replication (publisher or subscriber)

**What it is:** Render enables logical replication only through its support team, on request, for publishers and subscribers ([render.com/docs/postgresql-logical-replication](https://render.com/docs/postgresql-logical-replication)). bex's operators do the same with one command inside the bex-api pod. It reaches bex-api's cluster-internal verb (`POST /internal/postgres-logical-replication` on `:8091`, behind `BEX_CP_TOKEN`); no tenant surface has it. Design: [ADR009 § Logical replication](../ADR009-postgresql-management.md#logical-replication-w34m541), [§ Subscribers](../ADR009-postgresql-management.md#subscribers-w34m542) and [§ Lifecycle](../ADR009-postgresql-management.md#lifecycle-w34m543). Board: `w34/m541`, `w34/m542`, `w34/m543`.

One database may be a publisher, a subscriber or both. Each `enable` sets the parts it names and keeps the others.

## The request (publisher)

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
| `LOGICAL_REPLICATION_SUBSCRIBER_FORM` | The subscriber form does not fit the database's PostgreSQL major: `--subscriber-role` on 13–15, `--subscription` on 16 and later. Use the other one. |
| `LOGICAL_REPLICATION_SUBSCRIPTIONS_EXIST` | The subscriber role still owns the subscriptions the message names. Ask the user to `DROP SUBSCRIPTION` them first. |

## Enable a publisher

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
- **A database with a standby** (high availability): the answer's `warnings` says which of these applies, by the database's PostgreSQL major.
  - **17 and later:** a subscription keeps its place through a failover or switchover only when it is created `WITH (failover = true)` on a PostgreSQL 17 or later subscriber (`ALTER SUBSCRIPTION … SET (failover = true)` for an existing one). A CDC tool or an older subscriber creates its slot as the role with `SELECT pg_create_logical_replication_slot('<name>', 'pgoutput', false, false, true)` and subscribes with `create_slot = false`. While the standby is down, such a subscription receives nothing new: changes reach it once the standby has them.
  - **13–16:** a failover or switchover loses every logical slot, because bex's image does not have `pg_failover_slots` yet (w8/m92). Each subscriber must then be re-created, which copies its tables again. Upgrading to 17 or later keeps the slots.
- **A major-version upgrade** is refused (`POSTGRES_UPGRADE_LOGICAL_SLOTS_BEHIND`, naming the slots) while a slot is invalidated or has not consumed all WAL, because `pg_upgrade` would fail on it: the subscriber connects and catches up, or the slot is dropped (`SELECT pg_drop_replication_slot('<name>')` as the role). From 13–16 the upgrade is accepted but drops every slot, and the answer's `upgradeNotice` names them: re-create their subscriptions afterwards.

## Enable a subscriber

A subscriber copies another database, on Render, on bex or anywhere else, and follows it until cutover. Ask the workspace for the database's `dpg-` id and which PostgreSQL major it runs (`version` on the database), then use the form for that major. The prerequisites and their refusals are the publisher's.

**PostgreSQL 16 and later: a subscriber role.** The role creates its own subscriptions, as on Render.

```bash
kubectl -n bex-system exec deploy/bex-api -- /api postgres-logical-replication enable \
  --database dpg-… --subscriber-role <credential>
```

Exit 0 means `roleCreatesSubscriptions` is true: the role is a member of `pg_create_subscription`, and the verb granted it `CREATE` on the database if it lacked it. Recommend the database's default user. It owns the tables it created, and PostgreSQL applies each change as the table's owner, so another role's subscription fails to apply unless it is created `WITH (run_as_owner = true)` and the role can write the tables.

Tell the user:

1. Create the tables on the subscriber first (`pg_dump --schema-only` from the publisher, then restore it): logical replication does not copy DDL.
2. Run, as that role, `CREATE SUBSCRIPTION <name> CONNECTION 'postgresql://<user>:<password>@<publisher host>:5432/<db>?sslmode=require' PUBLICATION <publication>;`. The connection string must carry a password. The publisher's role needs its `REPLICATION` attribute: a bex publisher gets it from the publisher form above, a Render one from Render's support.
3. Follow `SELECT * FROM pg_stat_subscription;` until `received_lsn` keeps up with the publisher's `pg_current_wal_lsn()`. At cutover, stop writes on the publisher, wait for the lag to reach 0, copy each sequence with `setval` (sequences are not replicated), then point the application at the subscriber and `DROP SUBSCRIPTION`.

**PostgreSQL 13–15: a subscription bex creates.** Only a superuser can create or own a subscription on these majors, so bex creates it for the user. Ask for the subscription's name, the publisher's URL with its password, and the publication. Pass the URL on stdin, so the password is on no command line:

```bash
printf '%s' 'postgresql://<user>:<password>@<host>:5432/<db>?sslmode=require' |
  kubectl -n bex-system exec -i deploy/bex-api -- /api postgres-logical-replication enable \
    --database dpg-… --subscription <name> --publisher-url - --publication <publication>
```

The user creates the tables first: CloudNativePG's `CREATE SUBSCRIPTION` fails while a published table is missing, and the answer's `message` then carries the operator's `LogicalReplication` condition with PostgreSQL's error. Exit 0 means `liveSubscriptions` lists it. `sslmode` may be `disable`, `allow`, `prefer` or `require` (the default); `verify-ca` and `verify-full` are refused because the instance has no root certificate for an arbitrary publisher. A database takes at most four. Running the same command with a new password replaces the stored one. The user cannot alter or drop this subscription themselves (`must be owner of subscription`): remove it with `disable --subscription <name>`.

**A subscriber with a standby** (high availability). The answer's `warnings` says so on every major: the standby replicates asynchronously, as Render's does, so an automatic failover can lose the changes the subscriptions applied in its last seconds, and their publishers do not send those again. Nothing is applied twice, and a failover requested on a healthy primary (`POST /v1/postgres/{id}/failover`, a switchover) loses nothing. Tell the user that after an automatic failover (a `postgres_cluster_leader_changed` event they did not request), a subscription whose tables must match the publisher exactly is re-created over emptied tables, which copies them again:

- **16 and later:** as the subscriber role, `DROP SUBSCRIPTION <name>` (which also drops its slot on the publisher), `TRUNCATE` its tables, then `CREATE SUBSCRIPTION` again as above.
- **13–15:** run `disable --subscription <name>`, which drops it and its slot, have the user `TRUNCATE` its tables, then run the enable again with the same subscription and the publisher URL the user gives you again.

**Network.** Every database's instance can already reach another workspace's external URL, a host outside bex, and a same-workspace database's internal host on port 5432; nothing is opened per subscriber. A publisher with inbound IP rules must admit bex's tenant node addresses, from which subscribers connect (there is no fixed egress IP).

## A recovered copy

A database recovered to a point in time is a new database: it is not enabled, whatever its source was. Its source's publisher and subscriber credentials are retired on it (the owner stays its login), and it runs with `max_logical_replication_workers = 0` until it is enabled as a subscriber, so the subscriptions its backup restored do not start and compete with the source for its slots. Before the copy's first subscriber enable, the verb refuses with `LOGICAL_REPLICATION_SUBSCRIPTIONS_EXIST` while any of those is left, naming each `<database>.<subscription>`. Ask the user to detach and drop each one they own:

```sql
ALTER SUBSCRIPTION <name> DISABLE;
ALTER SUBSCRIPTION <name> SET (slot_name = NONE);
DROP SUBSCRIPTION <name>;
```

Detaching first keeps the drop from dropping the source's slot on the publisher. A subscription owned by the superuser (one bex created on 13–15) is dropped the same way by an operator on the primary, as in **A subscription bex created that will not drop** under Disable, without dropping the slot on the publisher. Then run the enable again; CloudNativePG restarts the instance once, to lift the worker limit.

## Check

```bash
kubectl -n <workspace namespace> get databases.app.bex.co dpg-… -o jsonpath='{.spec.logicalReplication}{"\n"}{.status.conditions[?(@.type=="LogicalReplication")]}{"\n"}'
kubectl -n <workspace namespace> get publications.postgresql.cnpg.io dpg-…-all-tables
kubectl -n <workspace namespace> get subscriptions.postgresql.cnpg.io   # the 13–15 subscriptions bex created
```

## Disable

```bash
kubectl -n bex-system exec deploy/bex-api -- /api postgres-logical-replication disable --database dpg-… [--publisher | --subscriber | --subscription <name>]
```

Without a flag both parts go. `--publisher` or `--subscriber` removes one part and keeps the other; `--subscription <name>` removes one subscription bex created.

**Publisher.** The verb drops the database's logical replication slots (ending the walsenders that hold them), clears the spec, waits for the role to lose `REPLICATION` and the all-tables publication to go, and ends any walsender left, so subscribers' next connections are refused. `max_slot_wal_keep_size` returns to PostgreSQL's default. The answer lists `droppedSlots`. A slot in `remainingSlots` still holds back WAL with no bound: enable the database again and disable it once more, which retries the drop. The schema grants stay; they are ordinary privileges the user may revoke. One audit row, `postgres.DisableLogicalReplication`, is recorded.

Tell the user to drop their subscriptions. Their publications stay.

**Subscriber.** A subscriber role that still owns subscriptions is refused with `LOGICAL_REPLICATION_SUBSCRIPTIONS_EXIST`, naming each as `<database>.<subscription>`: PostgreSQL would keep applying them after the role lost `pg_create_subscription`. The user drops them (`DROP SUBSCRIPTION`), then run the disable again. The role leaves `pg_create_subscription`; its `CREATE` on the database stays. Subscriptions bex created are dropped by CloudNativePG, which also drops each one's slot on the publisher, and their password Secrets are deleted.

**A subscription bex created that will not drop.** If the instance restarted between the removal and CloudNativePG's drop, the drop can no longer authenticate to the publisher, and the `Subscription` stays with `could not connect to publisher when attempting to drop replication slot` in its status. Detach it from its slot and drop it as the superuser on the primary, then drop the slot on the publisher (`SELECT pg_drop_replication_slot('<name>')`, or ask its provider):

```bash
kubectl -n <workspace namespace> exec <dpg-…>-1 -c postgres -- psql -d <database> \
  -c 'ALTER SUBSCRIPTION <name> DISABLE' -c 'ALTER SUBSCRIPTION <name> SET (slot_name = NONE)' -c 'DROP SUBSCRIPTION <name>'
```

CloudNativePG's next pass finds nothing to drop and releases the object.
