# w7 · m430 — A database's Metrics page charts Render's database activity: network, transactions, lock-delayed queries, disk activity and disk operations

**Worker:** worker7 **Goal:** a Postgres's metrics show the five graphs Render's database Metrics page has and bex lacks (Network Activity, Transaction Volume, Lock-Delayed Queries, Disk Activity, Disk Operations), a Key Value shows Disk Activity, and a service with a disk shows Disk Activity and Disk Operations, with the same series on REST, GraphQL, MCP and the dashboard. **Status:** blocked (t001–t005 and t007–t009 done; t006 and t010 wait for the production pin to carry `4bb3c5993`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Render's database Metrics page as a contract, with a source for each graph — **DONE** | 45m | — |
| t002 | Scrape the series: transactions, network and data-volume I/O for datastore and disk-bearing pods — **DONE** | 90m | t001 |
| t003 | A source for lock-delayed queries — **DONE** | 75m | t001 |
| t004 | The core verb and REST, GraphQL and MCP serve the new metrics — **DONE** | 90m | t002, t003 |
| t005 | Dashboard: the graphs on the Postgres, Key Value and service Disk pages — **DONE** | 90m | t004 |
| t006 | Live check on production: load and a one-second lock wait on a canary Postgres | 45m | t005 |
| t007 | Render parity — **DONE** | 20m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

On production, in `bex-canary`, with an owned Postgres (free where the workspace's free slot allows) deleted afterwards:

1. The Postgres's metrics show Network Activity, Transaction Volume, Lock-Delayed Queries, Disk Activity and Disk Operations with non-empty series after a short load. One deliberate lock wait of at least one second shows as one lock-delayed query after it completes.
2. A Key Value shows Disk Activity beside its existing graphs, and a service with a persistent disk shows Disk Activity and Disk Operations on its Disk page.
3. REST, GraphQL `datastoreMetrics` and MCP `get_datastore_metrics` return the same series for the same query, and a metric that does not apply to the resource's kind is refused by name.
4. The series added to the metrics store are limited to datastore and disk-bearing pods, and the growth in head series is recorded.
5. `docs/render-artifacts/metrics-page.md`, ADR010 and ADR018's Extended metrics row describe the graphs and their sources.
6. `cd lego/backend && go test ./...`, `make test` and `make lint` in `lego/operator/`, `bash scripts/gitops-validate.sh`, and dashboard `yarn lint` + `yarn test` pass.

## Record

**2026-11-17, parked on the release gate.** `4bb3c5993` shipped the five metrics on REST, GraphQL, MCP and the dashboard, the operator's `log_lock_waits`, and the scrape. The scrape is live on production (it ships through GitOps, not the image): 20 new series, data for a Postgres, a Key Value and a disk-bearing service, nothing for other pods (t002's record). bex-api, the dashboard and the operator move only with the next `pin platform images to` commit.

- **The gate clears by itself** when the newest pin carries `4bb3c5993`. Nothing is needed from anyone. The check:

  ```bash
  git fetch origin main
  pin=$(git log origin/main --grep='pin platform images to' -1 --format=%s | sed -E 's/.*pin platform images to ([0-9a-f]+).*/\1/')
  git merge-base --is-ancestor 4bb3c5993 "$pin" && echo "the pin carries 4bb3c5993"
  ```

- **Then:** t006 (the production walk), t010 (closeout). t007–t009 were done before the walk because the review and the tests had to precede the ship; if t006 changes code, they get another pass.

**For t006.**

- Once the operator rolls, every tenant database should read `log_lock_waits = on` with an unchanged postmaster start time (`select pg_postmaster_start_time(), current_setting('log_lock_waits')` before and after); that is the production half of t003's reload check.
- Transaction Volume's rate window is 240 s, so its first points appear four minutes after a database's series start; the network and disk graphs need one minute.
- The lock wait: one session `begin; update t set v = v + 1 where id = 1; select pg_sleep(3); commit;`, a second `update t set v = v + 10 where id = 1;` started half a second later; then the same with `pg_sleep(0.4)`. The first logs `still waiting … after 1000 ms` and `acquired … after ~2500 ms`; only the second line counts, in the bucket where it was written. The short wait logs nothing.
- An ordinary service pod has no `container_network_*` series at all (t002), which is DoD 4's absence check.
- **Production access.** The local host-key pin is stale. Fetch the kubeconfig as t001 did: read the pinned fingerprint from the newest `Verify production SSH trust` run (`gh run view <id> --log`), keyscan the three control planes, and pass a file built from the matching key as `BEX_SSH_KNOWN_HOSTS_FILE`. Prometheus answers through `kubectl -n monitoring exec <prometheus-server pod> -c prometheus-server -- wget -qO- 'http://localhost:9090/api/v1/query?query=…'`; the apiserver's service proxy cannot reach it.

**Found on the way, filed:** `w7/072` (the datastore panels read a fixed hour with no range picker).

## Source + Goal linkage

- **Source:** `/pm-brainstorm for render parity`, 2026-11-16, item 5; filed by `/pmx them all to w7` the same day.
- **Render reference:** render.com/docs/service-metrics, read 2026-11-16. § Disk storage: Disk Usage, Disk Activity ("The amount of data your service has read from and written to disk") and Disk Operations ("The number of read and write operations your service has performed on its disk"), for Postgres, Key Value (Disk Activity only) and services with a persistent disk. § Database activity: Active Connections, Network Activity ("The amount of data your database has read from and written to the network"), Transaction Volume ("The number of transactions executed by your database"), Replication Lag, and Lock-Delayed Queries ("The number of recently completed database queries that were delayed by another operation holding a lock for one second or longer. Queries appear on this graph after they've completed.").
- **bex today:** `lego/backend/internal/metrics/datastore.go:61-77` serves `disk`, `disk_capacity`, `db_connections`, `replication_lag`, `kv_memory` and `kv_connections`, plus CPU and memory (w1/m180). The `cnpg-tenant-db` scrape keeps only connection, replication and archive series (`deploy/gitops/base/prometheus.yaml:889`), and the cAdvisor scrape drops the rows with an empty `container` label (`:486-490`). None of the five graphs appears anywhere in `docs/` or `.pm/`.
- **Goal linkage:** ADR008 Render parity first; ADR010 (observability); ADR009 and ADR021 (managed Postgres and Key Value are paid products whose Metrics page is where an owner diagnoses load).
- **Expected outcome:** an owner moving a database from Render finds the same graphs with the same meaning.
- **Why now:** `w7/m427` sizes the metrics store for 30 days. New series should be known before that sizing is final, so this milestone follows it and t002 reports the growth back.
- **Decision state:** Render publishes no REST path for these five metrics (the pinned OpenAPI's `/metrics/*` paths have none of them), so the REST and MCP names are bex extensions kept consistent with GraphQL. That is this proposal's recommendation, not a user decision. Showing Disk Activity for a Free Key Value follows from bex's Free tier having a disk (ADR018 Key Value row); Render hides it there because its Free tier has none.
- **Render parity:** included: new metrics on REST, GraphQL, MCP and the dashboard.
