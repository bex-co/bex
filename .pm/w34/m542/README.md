# w34 · m542 — A bex Postgres subscribes to another bex database or one outside bex once it is enabled as a subscriber, as on Render

**Worker:** worker34 **Goal:** a bex Postgres enabled as a subscriber lets its named role run `CREATE SUBSCRIPTION` against another enabled bex database or a PostgreSQL outside bex, so a live database can be copied onto bex and kept in sync until cutover. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](t001.md) | Contract in ADR009: subscriber enablement by major version (16+ `pg_create_subscription`; 13–15) | 45m | m541/t001 |
| [t002](t002.md) | `spec.logicalReplication.subscriberRole`; the operator grants `pg_create_subscription` only while enabled | 1h | t001, m541/t002 |
| [t003](t003.md) | Egress: an enabled subscriber's CNPG pods reach its publisher, and nothing more | 1h15m | t001 |
| [t004](t004.md) | The ops verb enables a subscriber with Render's prerequisites | 30m | t002, m541/t006 |
| [t005](t005.md) | Subscriber lifecycle: the apply worker resumes after restarts; disable with live subscriptions is refused by name | 45m | t002 |
| [t006](t006.md) | Live check on dev-N: bex→bex across two workspaces, and an outside PostgreSQL→bex | 1h15m | t003, t004, t005 |
| [t007](t007.md) | Render parity | 30m | t006 |
| [t008](t008.md) | Simplify | 15m | t007 |
| [t009](t009.md) | Test coverage | 45m | t007 |
| [t010](t010.md) | Closeout (live replay on production) | 45m | t008, t009 |

## Definition of done

On production, in `bex-canary` on a Pro (or higher) workspace plan, with owned fixtures deleted afterwards (by-id `GET` 404):

1. A Postgres with at least 10 GB, enabled through the ops verb as a subscriber with a named role, runs `CREATE SUBSCRIPTION` as that role against (a) another workspace's Postgres enabled as a publisher by m541, through its external URL, and (b) a PostgreSQL outside bex. In both, the initial copy completes and later inserts, updates and deletes arrive.
2. A database that is not enabled as a subscriber gets PostgreSQL's own `permission denied to create subscription`, as today.
3. A subscriber on PostgreSQL 13–15 behaves as t001 decided.
4. No new egress opens for a database that is not enabled as a subscriber.
5. `make test` (from `lego/operator/`) and `cd lego/backend && go test ./...` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` for one aspect of render parity (Postgres logical replication), 2026-11-20, item 2; filed by `/pmx them all to w34`.
- **Render reference:** render.com/docs/postgresql-logical-replication, read 2026-11-20. The request names "each Render Postgres database you want to enable logical replication for (both publishers _and_ subscribers)", and after enablement the user creates "a subscription on each subscriber" themselves.
- **bex today:**
  - No role can create a subscription: on PostgreSQL 16 and later that needs membership in `pg_create_subscription` (with `CREATE` on the database), and on 13–15 a superuser. The managed-roles projection grants neither (`lego/operator/internal/controller/database_controller.go:358`, `:430` `inRoles`).
  - A managed database lives in its workspace's default-deny `<ws>` namespace. Its CNPG pods have the egress ADR043 D8.3 lists ([ADR043](../../../docs/ADR043-tenant-namespace-isolation.md)); whether they can open TCP 5432 to another workspace's external URL or to the internet is unverified (t003).
  - CNPG v1.30.1 `docs/src/logical_replication.md`: a declarative `Subscription` connects through an `externalClusters` entry and "can connect to any PostgreSQL database accessible over the network"; DDL, sequence data and large objects are not replicated, and the subscriber's tables must exist first.
- **Goal linkage:** ADR008 § Render parity first, and ADR008's "Render alternative": a live database can move onto bex without a `pg_dump` outage.
- **Expected outcome:** a team leaving Render (or any PostgreSQL host) copies its database onto bex while it keeps serving, waits for lag 0, and cuts over with a brief pause.
- **Why now:** follows m541 directly (same spec field, verb and ADR section). Without it, the only way onto bex is a dump with downtime.
- **Decision state:** the 13–15 path is t001's choice; this proposal recommends an ops-run CNPG `Subscription` (Render's support team can run `CREATE SUBSCRIPTION` as a superuser on any version), resting on ADR008 Render parity first. Refusing 13–15 by name would be a divergence that needs the user's sign-off in ADR018.
- **Render parity:** included. Logical replication is a tenant-facing SQL capability.
