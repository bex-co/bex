# w6 · m456 — Key Value connections stop at the plan's limit, as on Render (free 50 … 50g 50,000)

**Worker:** worker6 **Goal:** every Key Value instance accepts exactly the connections its plan allows on Render, no fewer (a paid store is capped at 1,024 by its internal proxy today) and no more (a free store takes 1,024 where Render's takes 50), and the client past the limit receives Valkey's own `ERR max number of clients reached`. **Status:** blocked: t001–t009 done. t006's production checks passed and found two defects in the rollout, fixed in the same ship. t011 checks the fixed operator on production once the deploy carrying the fix lands (see BLOCKED — 2026-11-17, second).

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [Render's connection limit on every Valkey plan, pinned in tiers](done/t001.md) — **DONE** | 45m | — |
| t002 | [Production read: which stores already exceed their new limit](done/t002.md) — **DONE** | 30m | t001 |
| t003 | [`valkey-server` starts with `--maxclients` from its plan](done/t003.md) — **DONE** | 1h | t001 |
| t004 | [The limit reaches running servers and plan upgrades with `CONFIG SET`](done/t004.md) — **DONE** | 45m | t003 |
| t005 | [kv-internal-proxy admits the plan's limit and lets Valkey answer the refusal](done/t005.md) — **DONE** | 1h | t003 |
| t006 | [Ship, verify on production, and record the limits in ADR021 and ADR018](done/t006.md) — **DONE** | 45m | t002, t004, t005 |
| t011 | [Production check of the fixed rollout: the limit's record lands, and a store waiting on another deferred change does not roll](t011.md) | 30m | t006 |
| t007 | [Render parity](done/t007.md) — **DONE** | 30m | t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009, t011 |

## Definition of done

- On production, a free Key Value accepts 50 concurrent client connections and the 51st receives `ERR max number of clients reached`; a `standard` store accepts 1,000; upgrading the free store to `starter` raises its limit to 250. _(Amended 2026-11-17, t003–t005: the upgrade restarts `valkey-server` as Render's does, "Upgrading requires a restart" ([Key Value](https://render.com/docs/key-value), read 2026-11-17), because it changes the pod's CPU and memory; the restarted server starts with the new limit, and the limit adds no restart of its own. The original "without restarting" could not hold on bex or on Render.)_
- A `pro` store's internal proxy admits its 5,000 (a test proves it no longer refuses at 1,024), and at every store's limit the refusal is Valkey's error on both the plain and the TLS listener, never a closed connection.
- Every existing store adopted its limit without a Valkey pod restart. _(Amended 2026-11-17, t006: the first rollout did not meet this. Two of the three production stores restarted once at 12:10Z because the limit's hold did not compose with the priority-class and environment-label holds, and nothing can undo those restarts. t006 fixed the defect, and the bullet now requires the fixed operator to adopt the limit with no restart on production: t011's drill of a store waiting on another deferred change, and the third store's record landing with no restart.)_
- A guard test pins every Valkey plan id and alias bex accepts to Render's figure; ADR021 and ADR018 record the limits; t002's stores over their new limit are reported.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity for w6` 2026-11-17, item 1, filed by the user's `/pm them to w6`. Render: [compute plans § Render Key Value plans](https://render.com/docs/compute-plans#render-key-value-plans), read 2026-11-17 (free 50, `256mb` 250, `1g` 1,000, `3g` 3,000, `5g` 5,000, `10g` 10,000, `20g` 20,000, `40g` 40,000, `50g` 50,000), and [Key Value](https://render.com/docs/key-value): "Specify a compute plan to determine the instance's available RAM and connection limit." Render's dashboard shows the limit on each plan card ([key-value.md](../../../../docs/render-artifacts/key-value.md)).
- **bex today:** `valkeyArgs` (`lego/operator/internal/controller/keyvalue_controller.go`) sets no `--maxclients`, so Valkey's default 10,000 applies, and every tenant connection passes `kv-internal-proxy`, whose `MaxConnections` defaults to 1,024 per listener (`lego/operator/internal/kvinternalauth/proxy.go:50`) and sheds the excess by closing the connection. ADR018 has no row for connection limits.
- **Goal linkage:** ADR008 § Render parity first: every customer-facing behavior matches render.com in every workspace, and this divergence has no sign-off in ADR018.
- **Expected outcome:** a paid store serves the connections its plan sells (5,000 on `pro`, where it stops at 1,024 today), a free store stops at Render's 50, and a client at the limit reads why.
- **Why now:** w5/m266 (2026-11-05) put `3g` through `pro_max` on sale, and each is held to 1,024 connections, a fifth of what Render sells as `pro`. Free stores take 20 times Render's 50, a cost exposure that grows with every free signup. w6/m457 and w6/m459 read the table t001 adds.
- **Render parity task:** included. The limit is tenant-visible on every client connection, and w6/m459 shows it in the dashboard; REST, GraphQL and MCP fields do not change, and t007 confirms they say nothing that contradicts it.

## Connection paths (t001, 2026-11-17)

Every way a connection reaches a store's `valkey-server`, and what bounds it after m456:

| Path | Who | Bound |
| --- | --- | --- |
| `<id>:6379` → `<id>-internal` plain listener (AUTH injected) → `<id>-backend:6379` | tenant services, internal URL, store without `internalAuth` | the proxy's budget, the plan's limit, shared with the next row |
| kv-sni-proxy → `<id>:6380` → `<id>-internal` TLS passthrough listener → `<id>-backend:6380` (TLS ends in Valkey) | tenant, external URL | the same budget; kv-sni-proxy's own 64 per resource first (w6/m457) |
| `<id>:6379` → Valkey directly; external TLS → `<id>:6380` → Valkey | tenant of a store with `spec.internalAuth` (proxy retired) | `--maxclients` only, the limit plus the reserve (follow-up w6/m463) |
| `<id>-backend` or the pod IP | anything that bypasses the proxy | `--maxclients` |

Platform clients, each holding a connection only while it runs, all inside the reserve of 10:

| Client | Connects as | At most at once |
| --- | --- | --- |
| Valkey readiness probe (`valkey-cli PING`, every 5 s, 2 s timeout) | `default`, localhost | 1 |
| redis_exporter sidecar (`kvExporterEnv`), one connection per scrape by the single `prometheus-server` | `bex`, localhost | 1 |
| `kv-internal-proxy --check` readiness (dials `<id>-backend` directly, outside the budget) | `default` | 2 (old and new pod during a rollout, `maxSurge: 1`) |
| Operator runtime passes (`withPlatformValkey`: TLS listener, maxmemory, maxclients, persistence handoff), sequential per store | `bex`, pod IP | 1 (2 across a leader handoff) |
| Backup snapshot (`valkey-cli --rdb`, CronJob `Forbid`), now dialing `<id>-backend` | `bex` | 1 (2 if a manual one overlaps) |
| Persistence-transition helper | own server on a Unix socket, not the serving one | 0 |

Sum 8, reserve 10. kv-sni-proxy holds no connection of its own: it dials per tenant client.

## Production read (t002, 2026-11-17)

`max by (pod) (max_over_time(redis_connected_clients[30d]))` on production `prometheus-server` at 2026-11-17T09:22Z. Retention reaches back to 2026-11-13T06:00Z (about 4 days), so this is the peak since then. `redis_connected_clients` counts the exporter's own scrape connection.

- Live stores: 3, all `starter` (limit 250) and behind the proxy, none with `internalAuth`. Peaks 37, 18 and 2.
- 40 stores deleted within the window peaked at 1–3.
- **None** is over its new limit; no peak reaches even free's 50.
- All three live Valkey processes run `open files 10032/10032`, so `CONFIG SET` can lower them from 10,000 but not raise them past it. A `starter` store goes to 260 with no restart.

## Progress — 2026-11-17 (t001–t005, t008, t009)

Shipped design, with where it deviates from the task text:

- **The proxy holds the tenant's limit; the server keeps a platform reserve.** t003/t005 planned the server to refuse first, with the proxy's budget above it. The platform's clients hold connections only while they run (see Connection paths), so a server-only bound of limit + reserve would admit up to the limit + 10 tenant connections and then lock out the probes, exporter and operator. Instead `kv-internal-proxy` holds exactly the plan's limit across both listeners and answers the next client with Valkey's exact `-ERR max number of clients reached\r\n`. `valkey-server` runs `--maxclients` limit + 10 and refuses only what bypasses the proxy, and the proxy forwards that refusal on the plain listener.
- **The TLS refusal is written by the proxy.** Valkey 8 writes nothing to a TLS client past `maxclients`: it drops it before the handshake (reproduced on the pinned image, `SSLEOFError`). The proxy mounts the store's certificate (optional, as the server does) and answers a refused TLS client inside TLS. A store with no certificate yet closes the client, which is Valkey's own behavior.
- **The backup snapshot dials `<id>-backend`**, so it uses the reserve instead of a tenant connection.
- **The proxy's memory limit scales with the plan** (32 Mi + 48 KiB per connection, at least 128 Mi). 5,000 idle connections measured 195.7 MB in its cgroup, so the old fixed 128 Mi would OOM a `pro` store's proxy at about 3,300 connections.
- **A server started without the flag can be lowered, not raised.** Valkey sets its open-files limit to `maxclients` + 32 at start (10,032 on every production store). The template keeps its flagless args only when the plan's value is at most 10,000; a pre-m456 `pro_plus` or larger store rolls once. Production has none.
- **Stores with `internalAuth` bypass the proxy**, so they are bounded at limit + 10. Filed as [w6/135](../../done/135.md), promoted to [w6/m463](../../m463/README.md); no production store has it.

Verified locally:
- `go test` for tiers, `internal/controller` (envtest and `BEX_TEST_VALKEY=1`, Valkey 7 and 8), `internal/kvinternalauth` (`-race`) and `cmd/kv-internal-proxy`.
- Every new test failed against a reverted fix: the hold, the settled-store skip, the refusal bytes, the shared budget and the upstream-refusal forwarding.
- Real Valkey: free starts at 60 and refuses the 61st with Valkey's error. A runtime raise to 260 admits exactly 200 more on the same `run_id`, and lowering below the connected count keeps every client.

ADR021 § 9 and ADR018 (the "Datastore connection limits by plan" row and a gap-backlog row) record it. That is t006 step 3; t006's production checks and t007 wait for the deploy.

## BLOCKED — 2026-11-17: the production deploy carrying `0eed45bcb`

- **Done.** t001–t005, t008 and t009 (see Progress) and t006 step 3 (ADR021 § 9, ADR018). Shipped in `0eed45bcb`.
- **Gate (time, clears by itself).** A push or scheduled `deploy.yml` run deploys `main` once the production pin is 6 h old, and a dispatched run deploys at once. Pins landed at 06:48Z, 07:47Z, 09:41Z, 10:08Z and 10:34Z on 2026-11-17, so the deploy carrying `0eed45bcb` should land within hours.
- **How to check it cleared.**
  - `git log origin/main --grep "pin platform images"` shows a pin whose producer contains `0eed45bcb`.
  - On production, every `<id>-internal` Deployment carries `BEX_KV_MAX_CONNECTIONS`: `kubectl get deploy -A -l app.bex.co/keyvalue-internal-proxy -o jsonpath='{..env[?(@.name=="BEX_KV_MAX_CONNECTIONS")].value}'`.
- **Then.**
  - t006 steps 1, 2 and 4, with probes in `bex-canary`:
    - free: 50 connections and the 51st reads the error, on the internal URL and on the external TLS URL from an allowlisted client;
    - `standard`: 1,000 connections;
    - upgrade free to `starter`: the restarted server runs `maxclients` 260 and accepts 250;
    - the three existing `starter` stores run `maxclients` 260 with their Valkey pods' restart counts unchanged and `app.bex.co/valkey-maxclients` recorded;
    - delete the probes.
  - t007 reads REST, GraphQL, MCP and the dashboard on production.
  - t010 closes.

## UNBLOCKED — 2026-11-17: the production deploy carrying `0eed45bcb` landed

- **Affected:** w6/m456 t006, t007, t010. **Source:** verified evidence, read by the `/loopx w6` worker at 16:55Z.
- **The pin.** `fc4fbf28a` ("pin platform images to db48cde2bae9", 12:08Z) is on `origin/main`, and `git merge-base --is-ancestor 0eed45bcb db48cde2bae9` holds. Production's `bex-system` Deployments run `bex-operator@sha256:ced89914…`, the digest that pin wrote to `deploy/gitops/base/bex.yaml`.
- **The env.** All three production `<id>-internal` Deployments (`red-d9p49kdrtmes73c34ovg`, `red-dbnukk1ngmjs739tgulg`, `red-dbpf5q5me2kc73f2l0og`, all `starter`) carry `BEX_KV_MAX_CONNECTIONS=250` and are Available at their latest generation. The BLOCKED section's check command selects nothing: the Deployments carry no `app.bex.co/keyvalue-internal-proxy` label. `kubectl get deploy -A | grep -- -internal` finds them.
- **Remaining gates:** none. **Next:** t006 steps 1, 2 and 4 in `bex-canary`, t007, t010, as listed under BLOCKED — Then.

## Production checks — 2026-11-17 (t006, t007)

- **Limits hold on production** (t006, `bex-canary` probes, all deleted with `GET` 404 afterwards):
  - free: 50 admitted, and the 51st reads `-ERR max number of clients reached` on the internal URL and inside TLS on the external URL;
  - `standard`: 1,000 admitted, the 1,001st refused;
  - free upgraded to `starter`: the restarted server runs `maxclients` 260, 250 are admitted and the 251st is refused.
- **The existing stores (t002's three):** none sees refusals; their peaks since the deploy are 17, 33 and 1 against 250. The checks found two defects in the rollout:
  - **A refused record.** `red-dbpf5q5me2kc73f2l0og` adopted 260 by `CONFIG SET` with no restart, but production's `bex-operator-pod-labels` admission policy refused the pod annotation recording it on every pass.
  - **Two restarts.** `red-d9p49kdrtmes73c34ovg` and `red-dbnukk1ngmjs739tgulg` restarted once at 12:10Z. The limit's hold did not compose with the deferred priority-class and environment-label holds those stores were waiting on.
  - Both are fixed in the commit `git log origin/main --grep 'w6/m456 t006'` names, with regression tests that fail against the old code. The DoD's existing-store bullet is amended accordingly. Details: [t006](done/t006.md).
- **Render parity** (t007): the figure and the refusal match Render's. REST, MCP, production GraphQL and the deployed dashboard state nothing that contradicts the limit. The dashboard display is w6/m459's, which is filed and waiting for its own deploy. Details: [t007](done/t007.md).

## BLOCKED — 2026-11-17 (second): the production deploy carrying t006's fix

- **Done.** t001–t009. The limits hold on production, and t006's fix is shipped (`git log origin/main --grep 'w6/m456 t006'`).
- **Gate (time, clears by itself).** A production pin whose producer contains the fix: `git log origin/main --grep "pin platform images"`, then `git merge-base --is-ancestor <fix> <producer>`. The pin at 12:08Z makes a push or scheduled `deploy.yml` run deploy `main` from 18:08Z.
- **Then.** t011 (the fixture's record lands with no restart; a free probe store edited to a pre-m456, pre-priority-class template adopts 60 without a second roll; the probe is deleted), then t010.
