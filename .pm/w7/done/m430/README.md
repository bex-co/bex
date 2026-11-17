# w7 · m430 — A database's Metrics page charts Render's database activity: network, transactions, lock-delayed queries, disk activity and disk operations

**Worker:** worker7 **Goal:** a Postgres's metrics show the five graphs Render's database Metrics page has and bex lacks (Network Activity, Transaction Volume, Lock-Delayed Queries, Disk Activity, Disk Operations), a Key Value shows Disk Activity, and a service with a disk shows Disk Activity and Disk Operations, with the same series on REST, GraphQL, MCP and the dashboard. **Status:** done (all 10 tasks done; walked on production 2026-11-17 at pin `db48cde2bae9`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Render's database Metrics page as a contract, with a source for each graph — **DONE** | 45m | — |
| t002 | Scrape the series: transactions, network and data-volume I/O for datastore and disk-bearing pods — **DONE** | 90m | t001 |
| t003 | A source for lock-delayed queries — **DONE** | 75m | t001 |
| t004 | The core verb and REST, GraphQL and MCP serve the new metrics — **DONE** | 90m | t002, t003 |
| t005 | Dashboard: the graphs on the Postgres, Key Value and service Disk pages — **DONE** | 90m | t004 |
| t006 | Live check on production: load and a one-second lock wait on a canary Postgres — **DONE** | 45m | t005 |
| t007 | Render parity — **DONE** | 20m | t006 |
| t008 | Simplify — **DONE** | 15m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007 |
| t010 | Closeout — **DONE** | 10m | t008, t009 |

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

**2026-11-17, unblocked (verified evidence, no user decision).** The release gate cleared by itself: the newest pin on origin/main, `fc4fbf28a chore(deploy): pin platform images to db48cde2bae9`, carries `4bb3c5993` (`git merge-base --is-ancestor 4bb3c5993 db48cde2bae9` succeeds). No gate remains. Next: t006, the production walk in `bex-canary` (a three-second lock wait and a sub-second one on an owned Postgres, then every new metric on REST, GraphQL, MCP and the dashboard), then t010.

**2026-11-17, t006: the production walk, 17:21–17:45 UTC, as the QA user in `bex-canary` (`tea-daif693dqjvc73e7as3g`).** Production ran the pin's digests: bex-api, the operator and the activator on `bex-operator@sha256:ced899147db9…`, the dashboard on `bex-dashboard@sha256:16bd3deae0b4…`, each rolled out. The kubeconfig came through a known-hosts pin built from the fingerprint of `Verify production SSH trust` run 44119356513, which all three control planes matched. Every read below used the window 17:20–17:38 at a 60 s step unless it says otherwise.

- _The operator's lock-wait setting, live._ All four tenant Postgres instances that predate the roll (`tea-d98210cbbpdc73dcrkvg`) read `log_lock_waits = on`, `deadlock_timeout = 1s`, with postmaster start times from 2026-11-12 and 2026-11-13, before the operator's roll at 12:12Z. t001 had read `off` on the same databases at 01:55Z, so they adopted the setting by reload, with no restart. The walk's new database read `on` too.
- _Fixtures._ A free Postgres `m430-walk-pg` (`dpg-dbu8qdeggh5s73emeopg`, PostgreSQL 18; the workspace had no Postgres, so its free slot was open), available at 17:25:57Z. A Starter background worker `m430-walk-disk` (`srv-dbu8qheggh5s73emeor0`, `busybox:1.37`) with a 1 GB disk at `/data` (`dsk-dbu8qheggh5s73emeos0`), whose command wrote and read back 16 MiB with direct I/O every ten seconds; `bex-canary` had no service with a disk. The existing Starter Key Value `qa-20261110-81b5-kv` (`red-dbpf5q5me2kc73f2l0og`, append-only file on) took three rounds of 3,000 short-lived `m430-walk:*` keys over its external URL at 17:31–17:32, all deleted afterwards (its one earlier key untouched).
- _Load._ 17:26:20–17:33:17: twelve rounds of a 2,000-row insert, a count and a filtered read, 240 single-row update transactions, then a 200,000-row bulk write (the database grew to 133 MB).
- _DoD 1 holds._ Over REST, all five Postgres graphs had data: Network Activity read 4,373,662 and write 3,426,339 bytes, Disk Activity read 68,183,647 and write 320,711,582 bytes, Disk Operations read 1,000 and write 1,529, all from the 17:26 bucket, and Transaction Volume 913 transactions from 17:27, about four minutes after the instance pod started (about 17:23), as the 240 s rate window predicts. The lock wait: at 17:33:24 one session ran `begin; update t set v = v + 1 where id = 1; select pg_sleep(3); commit;`, and a second, which waited in a loop until the first was sleeping, ran `update t set v = v + 10 where id = 1;` and took 2.84 s. The instance logged `process 689 still waiting for ShareLock on transaction 1034 after 1000.088 ms` at 17:33:26.78 and `process 689 acquired ShareLock on transaction 1034 after 2661.725 ms` at 17:33:28.45. At 17:35:10 the same pair with `pg_sleep(0.4)` waited 0.25 s, round trip included, and logged nothing. Lock-Delayed Queries then answered one point, value 1, in the bucket ending 17:34:00 (the minute the `acquired` line was written), and nothing for 17:35–17:38.
- _DoD 2 holds._ Key Value Disk Activity: read 63,110 and write 2,346,912 bytes, from 17:32. The service's disk: Disk Activity read 1,356,350,909 and write 1,368,975,906 bytes, Disk Operations read 2,399 and write 1,305, from 17:24. On the dashboard (headless Chromium with the QA session's storage state, the panels' default 12-hour range), the Postgres Metrics section drew Disk, Disk Activity, Disk Operations, CPU, Memory, Active Connections, Network Activity, Transaction Volume and Lock-Delayed Queries, each a chart with data (two lines with Read and Write legends where the graph has two series, a single bar of 1 for Lock-Delayed Queries), and Replication Lag its N/A state, as on a non-HA instance. The Key Value's Metrics tab drew Disk Activity (read and write) beside CPU, Memory and Connections, and the service's Disk page drew Disk Activity and Disk Operations, each with Read and Write.
- _DoD 3 holds._ For each of the five Postgres graphs, the Key Value's Disk Activity and the service's two disk graphs, REST `/v1/metrics/<graph>`, GraphQL `datastoreMetrics` and MCP `get_datastore_metrics` returned the same series: the same labels (`resource`, `instance`, and `direction` where there are two), units and points, one SHA-256 per graph across the three surfaces (MCP also labels each series with its `metric`, because one call can ask for several). Disk Operations and Network Activity on the Key Value, and Transaction Volume and Lock-Delayed Queries on the service, were refused by name on all three: REST 400 `bad_request`, a GraphQL error and an MCP `isError` result, with the same message (for example `metric "disk_operations" is not valid for a key-value resource; its disk graph is disk_activity`).
- _DoD 4 holds._ Production Prometheus has pod-level `container_network_*` rows for exactly five pods, the five tenant Postgres instances (the four existing ones and the walk's), and none for `hello-go`, the disk-bearing worker or the Key Value. `cnpg_pg_stat_database_xact_*` exists for those five tenant clusters only, one tenant database each. The walk's Postgres added four of them (two network, two transaction), as t002's count per instance predicts. Head series read 113,046 at 17:26Z and 114,639 at 17:39Z, below the 121,049 before the scrape change and the 124,243 just after it (t002), so the growth t002 recorded in w7/m427 stands.
- _Cleanup._ `DELETE /v1/postgres/dpg-dbu8qdeggh5s73emeopg` and `DELETE /v1/services/srv-dbu8qheggh5s73emeor0` answered 204 at 17:44:10. By 17:44:18 the by-id `GET`s of both answered 404, and the disk's `GET /v1/disks/dsk-dbu8qheggh5s73emeos0` answered 404. Their pods, claims and Hetzner volumes were gone by 17:45:06. The QA session was logged out, and the API answered 401 to its cookie afterwards. No API key was minted.
- t006 changed no code, so t007–t009 need no further pass. ADR018's Extended metrics row now records the production check.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for render parity`, 2026-11-16, item 5; filed by `/pmx them all to w7` the same day.
- **Render reference:** render.com/docs/service-metrics, read 2026-11-16. § Disk storage: Disk Usage, Disk Activity ("The amount of data your service has read from and written to disk") and Disk Operations ("The number of read and write operations your service has performed on its disk"), for Postgres, Key Value (Disk Activity only) and services with a persistent disk. § Database activity: Active Connections, Network Activity ("The amount of data your database has read from and written to the network"), Transaction Volume ("The number of transactions executed by your database"), Replication Lag, and Lock-Delayed Queries ("The number of recently completed database queries that were delayed by another operation holding a lock for one second or longer. Queries appear on this graph after they've completed.").
- **bex today:** `lego/backend/internal/metrics/datastore.go:61-77` serves `disk`, `disk_capacity`, `db_connections`, `replication_lag`, `kv_memory` and `kv_connections`, plus CPU and memory (w1/m180). The `cnpg-tenant-db` scrape keeps only connection, replication and archive series (`deploy/gitops/base/prometheus.yaml:889`), and the cAdvisor scrape drops the rows with an empty `container` label (`:486-490`). None of the five graphs appears anywhere in `docs/` or `.pm/`.
- **Goal linkage:** ADR008 Render parity first; ADR010 (observability); ADR009 and ADR021 (managed Postgres and Key Value are paid products whose Metrics page is where an owner diagnoses load).
- **Expected outcome:** an owner moving a database from Render finds the same graphs with the same meaning.
- **Why now:** `w7/m427` sizes the metrics store for 30 days. New series should be known before that sizing is final, so this milestone follows it and t002 reports the growth back.
- **Decision state:** Render publishes no REST path for these five metrics (the pinned OpenAPI's `/metrics/*` paths have none of them), so the REST and MCP names are bex extensions kept consistent with GraphQL. That is this proposal's recommendation, not a user decision. Showing Disk Activity for a Free Key Value follows from bex's Free tier having a disk (ADR018 Key Value row); Render hides it there because its Free tier has none.
- **Render parity:** included: new metrics on REST, GraphQL, MCP and the dashboard.
