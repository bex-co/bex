# w34 · m542 — A bex Postgres subscribes to another bex database or one outside bex once it is enabled as a subscriber, as on Render

**Worker:** worker34 **Goal:** a bex Postgres enabled as a subscriber lets its named role run `CREATE SUBSCRIPTION` against another enabled bex database or a PostgreSQL outside bex, so a live database can be copied onto bex and kept in sync until cutover. **Status:** blocked (t001–t005, t007–t009 done; t006 and t010 wait until the production pin carries `b4d4003fc`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | Contract in ADR009: subscriber enablement by major version (16+ `pg_create_subscription`; 13–15) — **DONE** | 45m | m541/t001 |
| [t002](done/t002.md) | `spec.logicalReplication.subscriberRole`; the operator grants `pg_create_subscription` only while enabled — **DONE** | 1h | t001, m541/t002 |
| [t003](done/t003.md) | Egress: an enabled subscriber's CNPG pods reach its publisher, and nothing more — **DONE** | 1h15m | t001 |
| [t004](done/t004.md) | The ops verb enables a subscriber with Render's prerequisites — **DONE** | 30m | t002, m541/t006 |
| [t005](done/t005.md) | Subscriber lifecycle: the apply worker resumes after restarts; disable with live subscriptions is refused by name — **DONE** | 45m | t002 |
| [t006](t006.md) | Live check on dev-N: bex→bex across two workspaces, and an outside PostgreSQL→bex | 1h15m | t003, t004, t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 15m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007 |
| [t010](t010.md) | Closeout (live replay on production) | 45m | t008, t009 |

## Progress (2026-11-21, loopx w34)

- **Shipped:** `spec.logicalReplication.subscriberRole` and `subscriptions[]`; the operator's projection (the subscriber role's `pg_create_subscription` membership on a cluster running 16 or later; on 13–15 an `externalClusters` entry and a CloudNativePG `Subscription` per requested subscription, with the verdicts on the `LogicalReplication` condition); the verb's subscriber forms with m541's prerequisites, the `CREATE` grant, a password Secret per 13–15 subscription, per-half disable and the `LOGICAL_REPLICATION_SUBSCRIPTIONS_EXIST` and `LOGICAL_REPLICATION_SUBSCRIBER_FORM` refusals; the runbook, the dashboard note and the agent skill. Design and evidence: [ADR009 § Subscribers](../../../../docs/ADR009-postgresql-management.md#subscribers-w34m542); the ADR018 row "Logical replication (subscriber)" is ◐ with its owners.
- **Egress (t003):** none needed. From a production CNPG pod in `bex-canary`, a TLS session through another database's `<id>.db.bex.co` reached PostgreSQL and one to a PostgreSQL outside bex connected, from the node's ExternalIP. Both ride `allow-internet-egress`; nothing opens for any database (DoD 4).
- **Verified off-cluster:** the verb end to end on real PostgreSQL 17 (CI's) and locally on 15 and 18: before enabling, the refusal is PostgreSQL's (DoD 2); on 16 and later the enabled default user subscribes and receives the copy, inserts, updates and deletes (DoD 1 at the SQL level), disable is refused naming the subscription until the user drops it, and a dedicated credential gets `CREATE` on the database; on 15 the role form is refused and a subscription bex creates is written with its password in a Secret and removed with it (DoD 3). Locally, a CNPG-shaped superuser subscription with a passfile on PostgreSQL 15 copied a PostgreSQL 18 publisher, and a PostgreSQL 17 subscriber stopped mid-stream resumed with every row exactly once.
- **Corrected premise (t005):** a subscription outlives its owner's `pg_create_subscription` and the owner can still alter and drop it; the disable refusal stands for another reason, recorded in ADR009.
- **Left:** t006 (DoD 1–4 on a real cluster) runs on production, since no dev stack is available to w34; it and t010 follow once the production pin carries this milestone's commit and m541's, with `bex-canary` on Pro for the walk under the user's 2026-10-31 permission and back to Hobby afterwards.

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
  - A managed database lives in its workspace's default-deny `<ws>` namespace. Its CNPG pods have the egress ADR043 D8.3 lists ([ADR043](../../../../docs/ADR043-tenant-namespace-isolation.md)); whether they can open TCP 5432 to another workspace's external URL or to the internet is unverified (t003).
  - CNPG v1.30.1 `docs/src/logical_replication.md`: a declarative `Subscription` connects through an `externalClusters` entry and "can connect to any PostgreSQL database accessible over the network"; DDL, sequence data and large objects are not replicated, and the subscriber's tables must exist first.
- **Goal linkage:** ADR008 § Render parity first, and ADR008's "Render alternative": a live database can move onto bex without a `pg_dump` outage.
- **Expected outcome:** a team leaving Render (or any PostgreSQL host) copies its database onto bex while it keeps serving, waits for lag 0, and cuts over with a brief pause.
- **Why now:** follows m541 directly (same spec field, verb and ADR section). Without it, the only way onto bex is a dump with downtime.
- **Decision state:** the 13–15 path is t001's choice; this proposal recommends an ops-run CNPG `Subscription` (Render's support team can run `CREATE SUBSCRIPTION` as a superuser on any version), resting on ADR008 Render parity first. Refusing 13–15 by name would be a divergence that needs the user's sign-off in ADR018.
- **Render parity:** included. Logical replication is a tenant-facing SQL capability.
