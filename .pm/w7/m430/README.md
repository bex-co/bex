# w7 · m430 — A database's Metrics page charts Render's database activity: network, transactions, lock-delayed queries, disk activity and disk operations

**Worker:** worker7 **Goal:** a Postgres's metrics show the five graphs Render's database Metrics page has and bex lacks (Network Activity, Transaction Volume, Lock-Delayed Queries, Disk Activity, Disk Operations), a Key Value shows Disk Activity, and a service with a disk shows Disk Activity and Disk Operations, with the same series on REST, GraphQL, MCP and the dashboard. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Render's database Metrics page as a contract, with a source for each graph | 45m | — |
| t002 | Scrape the series: transactions, network and data-volume I/O for datastore and disk-bearing pods | 90m | t001 |
| t003 | A source for lock-delayed queries | 75m | t001 |
| t004 | The core verb and REST, GraphQL and MCP serve the new metrics | 90m | t002, t003 |
| t005 | Dashboard: the graphs on the Postgres, Key Value and service Disk pages | 90m | t004 |
| t006 | Live check on production: load and a one-second lock wait on a canary Postgres | 45m | t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 45m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

On production, in `bex-canary`, with an owned Postgres (free where the workspace's free slot allows) deleted afterwards:

1. The Postgres's metrics show Network Activity, Transaction Volume, Lock-Delayed Queries, Disk Activity and Disk Operations with non-empty series after a short load. One deliberate lock wait of at least one second shows as one lock-delayed query after it completes.
2. A Key Value shows Disk Activity beside its existing graphs, and a service with a persistent disk shows Disk Activity and Disk Operations on its Disk page.
3. REST, GraphQL `datastoreMetrics` and MCP `get_datastore_metrics` return the same series for the same query, and a metric that does not apply to the resource's kind is refused by name.
4. The series added to the metrics store are limited to datastore and disk-bearing pods, and the growth in head series is recorded.
5. `docs/render-artifacts/metrics-page.md`, ADR010 and ADR018's Extended metrics row describe the graphs and their sources.
6. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, `bash scripts/gitops-validate.sh`, and dashboard `yarn lint` + `yarn test` pass.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for render parity`, 2026-11-16, item 5; filed by `/pmx them all to w7` the same day.
- **Render reference:** render.com/docs/service-metrics, read 2026-11-16. § Disk storage: Disk Usage, Disk Activity ("The amount of data your service has read from and written to disk") and Disk Operations ("The number of read and write operations your service has performed on its disk"), for Postgres, Key Value (Disk Activity only) and services with a persistent disk. § Database activity: Active Connections, Network Activity ("The amount of data your database has read from and written to the network"), Transaction Volume ("The number of transactions executed by your database"), Replication Lag, and Lock-Delayed Queries ("The number of recently completed database queries that were delayed by another operation holding a lock for one second or longer. Queries appear on this graph after they've completed.").
- **bex today:** `lego/backend/internal/metrics/datastore.go:61-77` serves `disk`, `disk_capacity`, `db_connections`, `replication_lag`, `kv_memory` and `kv_connections`, plus CPU and memory (w1/m180). The `cnpg-tenant-db` scrape keeps only connection, replication and archive series (`deploy/gitops/base/prometheus.yaml:889`), and the cAdvisor scrape drops the rows with an empty `container` label (`:486-490`). None of the five graphs appears anywhere in `docs/` or `.pm/`.
- **Goal linkage:** ADR008 Render parity first; ADR010 (observability); ADR009 and ADR021 (managed Postgres and Key Value are paid products whose Metrics page is where an owner diagnoses load).
- **Expected outcome:** an owner moving a database from Render finds the same graphs with the same meaning.
- **Why now:** `w7/m427` sizes the metrics store for 30 days. New series should be known before that sizing is final, so this milestone follows it and t002 reports the growth back.
- **Decision state:** Render publishes no REST path for these five metrics (the pinned OpenAPI's `/metrics/*` paths have none of them), so the REST and MCP names are bex extensions kept consistent with GraphQL. That is this proposal's recommendation, not a user decision. Showing Disk Activity for a Free Key Value follows from bex's Free tier having a disk (ADR018 Key Value row); Render hides it there because its Free tier has none.
- **Render parity:** included: new metrics on REST, GraphQL, MCP and the dashboard.
