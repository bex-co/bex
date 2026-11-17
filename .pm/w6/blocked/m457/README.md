# w6 · m457 — External Postgres and Key Value connections are admitted up to the datastore's own limit, not 64

**Worker:** worker6 **Goal:** an allowlisted client outside the cluster can open as many connections to a database or Key Value as its plan allows, as on Render, and the one past the limit gets the server's own refusal instead of a dropped connection. **Status:** blocked: t001–t004, t008 and t009 done; t006's ADR and env-inventory records written. The production checks (t005) wait for the batched production deploy carrying this milestone's commit and w6/m456's `0eed45bcb` (see BLOCKED — 2026-11-17).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Decide the external admission rule and record it](done/t001.md) — **DONE** | 45m | — |
| t002 | [Each external route carries its datastore's connection limit](done/t002.md) — **DONE** | 1h | t001, w6/m456/t001, w6/134 |
| t003 | [pg-sni-proxy admits up to the route's limit](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [kv-sni-proxy admits up to the store's limit](done/t004.md) — **DONE** | 45m | t002 |
| t005 | [Production check from an allowlisted client](t005.md) | 45m | t003, t004, w6/m456/t006 |
| t006 | [ADR018 row and the proxies' env inventory](t006.md) | 20m | t005 |
| t007 | [Render parity](t007.md) | 30m | t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Definition of done

- On production, from an allowlisted client outside the cluster: 97 concurrent connections to a `basic-256mb` probe database succeed (Render's 100 less PostgreSQL's 3 `superuser_reserved_connections`) and the next receives `FATAL: sorry, too many clients already`, not an EOF or SSL error; a `starter` probe Key Value accepts 250 external TLS connections and the 251st receives `ERR max number of clients reached`.
- No proxy refuses a route below its datastore's limit, and the per-workspace cap is no lower than the sum of its public datastores' limits, or off.
- ADR009, ADR021 and ADR018 record the rule, and `lego/operator/AGENTS.md` documents the proxies' env vars as they now behave.
- _(Amended 2026-11-17, t003: PostgreSQL answers a non-superuser past `max_connections` − `superuser_reserved_connections` with `FATAL: remaining connection slots are reserved for roles with the SUPERUSER attribute` (SQLSTATE 53300, PostgreSQL 16+). `sorry, too many clients already` comes only once every slot is held. Render runs the same server, so either 53300 message is the parity refusal; an EOF or SSL error is not. CNPG's own superuser sessions use ordinary slots, so on production the tenant sees a few fewer than 97. The production check records the count and the message it reads.)_

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-17, item 3, filed by the user's `/pm them to w6`. Render: external Key Value connections "always require authentication" and are reachable only from the instance's IP allowlist ([Key Value](https://render.com/docs/key-value)); Render documents each plan's connection limit and no separate cap for external connections ([compute plans](https://render.com/docs/compute-plans)).
- **bex today:** `pg-sni-proxy` and `kv-sni-proxy` default to 64 connections per resource and 256 per workspace (`lego/operator/cmd/pg-sni-proxy/main.go:80`, `lego/operator/cmd/kv-sni-proxy/main.go:66`; `deploy/gitops` overrides neither). So every external Postgres client stops at 64 where Render's stops at 100, and every paid Key Value at 64 where Render's stops at 250 or more. The shed closes the connection (`sniproxy.Limiter.AcquireRoute`), which a client reports as a network error. The caps came with `91a24194f` (2026-08-16, "remediate repository security review") and are recorded in no ADR.
- **Goal linkage:** ADR008 § Render parity first, which amends an earlier hardening choice that conflicts with parity instead of keeping it.
- **Expected outcome:** serverless and edge apps, BI tools and migrations that reach a datastore from outside get Render's connection count, and a client at the limit reads why.
- **Why now:** it affects every external Postgres client on every plan, and the failure looks like a network fault. w6/m456 and w6/134 supply the per-plan figures it needs.
- **Render parity task:** included. External connection behavior is tenant-facing; connection info on REST, GraphQL, MCP and the dashboard does not change.

## Decision — t001, 2026-11-17

Recorded in [ADR009 § External connection admission](../../../../docs/ADR009-postgresql-management.md#external-connection-admission-w6m457-2026-11-17), with ADR021 § 9 for Key Value. The proxy relays TLS end to end and cannot write a server's error, so every cap is either the server's limit with room above it, or process protection no tenant within its plan reaches. They replace `91a24194f`'s 1,024 / 128 / 32 / 256 / 64.

| cap | now | why |
| --- | --- | --- |
| per route, one public hostname | twice its server's limit: plan `maxConnections` for primary and each replica, PgBouncer's 30,000 for `-pool`, plan `connectionLimit` for a store | up to twice the limit, the client past it still reaches the server and reads its refusal, even when as many arrive at once as the plan holds |
| per workspace | removed | with every route bounded, it either repeats the sum of its routes' caps or refuses a tenant within its plans |
| per source | `0` (off), env var kept as an incident lever | one host is entitled to its datastores' whole limits, and the largest exceed a process |
| handshake per /24 or /64 | 128 | counts only unfinished handshakes; keeps R25-12's 32 networks to fill a process |
| global per replica | 4,096, memory limit 512 Mi, `GOMEMLIMIT` 400 MiB | the edge `lb11` admits 10,000 over three replicas (at most about 3,334 each); 4,096 idle routed connections measured 412 MB in the proxy's cgroup |

Production on 2026-11-17: 3 platform replicas per proxy, at most 41 MB used by any of them in the last 4 days, and no rejection of any reason in Prometheus' retention. One public database (`basic-1gb`) and one public store (`starter`); `max_connections` 100 and `superuser_reserved_connections` 3 on the database.

**Not in scope, filed:** the edge load balancer admits 10,000 connections across all five listeners, fewer than a `pro_plus` or `pro_max` store or the pooled URL holds on Render ([w6/136](../../136.md), a cost decision).

## Progress — 2026-11-17 (t001–t004, t008, t009)

- **t002.** `tiers` carries Render's `maxConnections` on every Postgres tier (100 on all six) and `PostgresPoolerMaxClientConn` (30,000). This is w6/134's step 1: its catalog half landed here because each route needs the figure. `TestPostgresMaxConnectionsMatchRender` checks every tier under its Render spec id against Render's whole table, so a later `2c-8g` must carry 200, and `parse` refuses a tier without the figure. w6/134 still owns setting `max_connections` on the server and its dev-6 restart check. `PostgresCatalog.Resolve` and `ValkeyCatalog.Resolve` are now the one plan resolution that the operator and both proxies use. Each route's capacity is computed on every watch event, so a plan change applies to the next client.
- **t003/t004.** `Limiter.AcquireRoute(route, capacity)` replaced the shared 64 and the per-workspace bucket. Each public hostname is its own bucket, keyed by namespace, database and label. A shed is still metered as `reason="resource"`; the alert and Grafana descriptions name the new meaning.
- **Verified locally.** The mock cluster cannot run the proxies: PodSecurity `baseline` refuses their `hostPort` in `bex-system`. Instead:
  - PostgreSQL 18.6 with TLS behind `handleConn` and the shipped caps: 110 concurrent non-superuser `psql` sessions to a `basic-256mb` route gave 97 sessions and 13 × `FATAL: remaining connection slots are reserved for roles with the SUPERUSER attribute`. The same run with the old 64 gave 64 sessions and 6 × `SSL error: unexpected eof while reading`.
  - `TestAStoreAtItsLimitAnswersTheNextExternalClientWithValkeysRefusal` drives a `starter` store through kv-sni-proxy and kv-internal-proxy: 250 TLS clients are relayed, and the 251st reads `-ERR max number of clients reached`.
  - `TestARouteAdmitsTwiceItsPlansLimitAndShedsOnlyBeyond`: 200 clients are dialed on `basic-256mb`, the 201st is shed without a dial and metered, the pooled hostname and another tenant still admit, and a client that leaves frees its slot.
  - Each test failed with the old 64 per resource. The two end-to-end tests also failed with the old per-source 128.
- **t006 in part.** ADR009, ADR021 § 9, ADR018 (implemented, production pending), the ADR110 R25-12 amendment, `lego/operator/AGENTS.md` and `.env.example` describe the shipped caps. t006 keeps the ✅ on ADR018's row until t005.

## BLOCKED — 2026-11-17: the production deploy carrying this milestone's commit

- **Done.** t001–t004, t008 and t009 (see Decision and Progress), plus t006's records except ADR018's ✅.
- **Gate (time, clears by itself).** A push or scheduled `deploy.yml` run deploys `main` once the production pin is 6 h old, and a dispatched run deploys at once. t005 also needs w6/m456's `0eed45bcb` deployed: its Key Value check reads the refusal kv-internal-proxy writes.
- **How to check it cleared.**
  - `git log origin/main --grep "pin platform images"` shows a pin whose producer contains this milestone's commit, and therefore `0eed45bcb`.
  - On production, both proxies run the new caps: `kubectl -n bex-system get ds bex-pg-sni-proxy bex-kv-sni-proxy -o jsonpath='{..resources.limits.memory}'` reads `512Mi 512Mi`.
- **Then.**
  - t005 with probes in `bex-canary`, from a host on each probe's allowlist:
    - a public `basic-256mb` database: hold concurrent non-superuser sessions until PostgreSQL refuses; record the count (97 less CNPG's own sessions) and the 53300 message;
    - a public `starter` store: 250 external TLS clients, and the 251st reads `ERR max number of clients reached`;
    - `bex_pg_proxy_connections_rejected_total` and `bex_kv_proxy_connections_rejected_total` stay unchanged;
    - delete the probes.
  - t006: ADR018's row gets the ✅ with t005's evidence.
  - t007 reads connection info on REST, GraphQL, MCP and the dashboard on production; none of them changed.
  - t010 closes.

