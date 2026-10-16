# pgvector is packaged but unusable in a fresh application database — sweep353

[Results](results.json) · [complete native create/get/readiness/SQL/catalog/error/cleanup receipts](native-SQL-catalog-readiness-cleanup.json) · [locked CNPG feature reference](locked-CNPG-feature-reference.json) · [actual cluster projections](cluster-projection-observation.go.txt) / [events](cluster-projection-events.jsonl) · [maintained neighboring tests](maintained-neighbor-events.jsonl) · [fresh main](fresh-main-comparison.json) · [final inventory](final-inventory.json) · [dedupe](dedupe.json).

Research sourcebd0ffac525f2ff1dfb4d7c7add9f81000fed8036, serving SHA unknown. Fresh origin779c825e2 only adds an environment-group QA note; product code is identical and no pgvector repair/owner is found. Published Bex0.3.2/a6350da0 and unchanged Render2.27.0/a764810a7682 target api.bex.co with the continuing isolated human QA identity in bex-canary. The external psql executable is the same local dependency for both clients; no executable is installed over the customer's binary.

## Native create and normal SQL journeys

Native Bex creates explicit Free PostgreSQL18 in Frankfurt with connectionPool none, custom application database/user and no replicas: dpg-db8s6hq9cl7s73ae4b1g, qa-20261016-1a5fcc-pgext. Native0 and exact returned workspace/name/plan/version prove ownership; pre-create PostgreSQL inventory and quota are empty, original services6. [Exact create](initial-native-create.sh.txt). Normal creating→available readiness observations are retained. A native SQL connection control returns the exact database/user, major18, UTF8 and SELECT1. No admin connection or role change is used.

A native catalog query shows vector0.8.6 available and not installed, alongside hstore1.8, pg_trgm1.6, pgcrypto1.4 and uuid-ossp1.1. Thus this is not a missing package or an extension-name typo. Each client starts a fresh native process and runs an ordinary migration plus a tiny distance check, within an explicit rollback-only transaction and10s statement/3s lock bounds:

```sql
BEGIN;
SET LOCAL statement_timeout='10s';
SET LOCAL lock_timeout='3s';
CREATE EXTENSION IF NOT EXISTS vector;
SELECT '[1,2,3]'::vector <-> '[1,2,4]'::vector;
ROLLBACK;
```

Actual commands wrap the SELECT as one marked JSON row; full argv and stdout/stderr are retained. Both native parents exit1 without timeout. PostgreSQL reports permission denied to create extension vector and the superuser requirement; neither distance SELECT runs. No failed request is retried blindly or repaired by using a different database identity.

Each client's independent hstore transaction executes CREATE EXTENSION IF NOT EXISTS hstore, returns its exact marker and k=>v lookup value v, and rolls back: native0. The second native catalog query through Render exactly matches the first and confirms both extensions remain uninstalled, proving no persistent extension mutation from these experiments. Seven native SQL commands cover1 connection control,2 catalog reads,2 vector failures and2 hstore successes. Render receives the own API-returned CA in a per-call0600 file, removed in finally; the known CA setup requirement is not confused with this SQL error. Successful child stderr limitations remain the existing079 issue, not this failure.

[Render's supported extension contract](https://render.com/docs/postgresql-extensions) includes pgvector for PostgreSQL13+ and identifies vector as its SQL extension name. No authenticated Render database is used. The functional expectation here is that a new supported database can run an ordinary vector migration and use embedding vectors, not that every extension has identical installation rules.

## Source and implementable provisioning path

The exact-pin cmd/psql.go dispatches noninteractive SQL to pkg/tui/views/psql.go:45–94, obtains the database connection information and executes the external psql with the unchanged command/extra flags. Bex's pgtrust code only supplies the own CA; unchanged Render reaches the same error with equivalent CA preparation. Backend postgres/service.go:806 assembles that application connection, and rest.go:249 exposes its route. Healthy SELECT/catalog/hstore controls prove this path works.

On the operator, database_controller.go:815 invokes reconcileCluster; at932–935 it uses cnpgClusterSpec:450. The projection selects the reviewed image at480–487 and an application database/owner at520–527. The only provisioned extension is pg_stat_statements, with its separate managed preload path. There is no vector application-database provisioning, no postInitApplicationSQL vector step and no CNPG Database resource producer. The current GVK census and RBAC at706 include Cluster/Backup/ScheduledBackup/Pooler, not the upstream Database extension-management resource.

Four unchanged real cnpgClusterSpec observations cover13/18 × default/custom application names. All exact identity/reviewed-image controls pass and exports contain no vector provisioning (5 named observation events, zero skips/failures). This is serializer/source scope, not four hosted databases or passing vector regressions. Three maintained identity/bootstrap/recovery tests pass separately. [Source observation](cluster-projection-observation.go.txt). No repository Go file is added.

The chart archive is downloaded through its official Helm index, checked against the repository's exact668e065f… archive hash, and matches locked chart0.29.0/appVersion1.30.0. The matching tagged upstream Database CRD directly verifies extensions fields name/ensure/schema/version and default reclaim policy retain. This proves the capability in the source's locked dependency, not the running production controller version. The earlier guessed chart URL404 is a local reference lookup failure, corrected using the authoritative index; no deployment is performed.

[CNPG's declarative database documentation](https://cloudnative-pg.io/docs/1.29/declarative_database_management/#managing-extensions-in-a-database) describes ensuring named extensions in an application database. A concrete repair is an owned postgresql.cnpg.io/v1 Database for the existing application database/owner/cluster, ensuring only vector present with retain semantics, reconciled for fresh and existing parents. Add the matching operator resource wiring/RBAC and observable readiness/error handling. Keep existing database content, names, pg_stat_statements management and customer connections unchanged. Document platform-preenabled vector versus Render's user-install description; the original IF NOT EXISTS migration and subsequent vector use must work as the ordinary application user.

## Cleanup and qualifications

Native deletion exits0. Detail404/listempty, PostgreSQL used0/terminating0, original services6 and Blueprint1, groups/projects empty. A privately cached exact-owned endpoint executes SELECT1 before deletion, then a fresh ordinary psql attempt fails with exit2 afterward. Those two data-plane diagnostics are API-only, not extra native coverage. Own private connection/CA files, per-call Render config and CA files are removed. No paid resource, PostgreSQL upgrade, baseline mutation or user table is involved; physical pod/volume teardown is unobserved.

[Excluded setup](excluded-setup.json): the first overlong fixture name receives the documented RESOURCE_NAME_INVALID400; native list and quota prove no allocation, before a new nonce/shorter name is created. An early ready helper references the rejected no-ID intent and does no SQL; a private inline helper resolves its path incorrectly and makes no production call. These setup errors are not findings and no ambiguous create is replayed.

Local image pulls do not finish: the first stalls in the local credential helper and only its exact stdout-verified own process tree is stopped; a separate empty-config anonymous attempt has a120s bound. Its config is removed and no own container remains. [Limit record](local-image-inspection-limit.json). No local image filesystem/server execution is claimed; the hosted catalog directly establishes package presence.

Live scope is one Free PostgreSQL18 custom-database/custom-user fixture, two native clients and tiny rollback-only SQL. Default names, other majors/plans, restored/existing-data backfill, real CNPG extension reconciliation, shared query UI/MCP use, vector table/index persistence, browser UI and exact deployed revisions are implementation acceptance work. Helpers: [native journey](finite-native-journey.py.txt), [isolated clients](isolated-native-client-helpers.py.txt), [private cleanup](private-endpoint-cleanup-helper.py.txt), [chart verification](locked-chart-reference-driver.py.txt). Isolated auth remains for the continuing loop; uncommitted.
