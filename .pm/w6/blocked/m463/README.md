# w6 · m463 — Key Value stores with internal authentication stop at exactly their plan's connection limit

**Worker:** worker6 **Goal:** a store with `spec.internalAuth` (w5/m268) holds its tenants to exactly the plan's connection limit, as every other store has since w6/m456, so a tenant at the limit reads Valkey's `ERR max number of clients reached` and the platform's reserve of 10 stays free for the readiness probe, exporter, operator and backups. **Status:** blocked (2026-11-17): t001–t004, t007 and t008 done; t005, t006 and t009 wait for the production deploy carrying this milestone's commit

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [kv-internal-proxy counts without authenticating: a plain passthrough listener](done/t001.md) — **DONE** | 1h | — |
| t002 | [The operator keeps the proxy in front of a store with internal authentication](done/t002.md) — **DONE** | 1h30m | t001 |
| t003 | [Prove the limit and the reserve on a real store with internal authentication](done/t003.md) — **DONE** | 45m | t002 |
| t004 | [Record it in ADR021 and ADR018](done/t004.md) — **DONE** | 20m | t002 |
| t005 | [Ship and verify on production](t005.md) | 45m | t003, t004 |
| t006 | [Render parity](t006.md) | 30m | t005 |
| t007 | [Simplify](done/t007.md) — **DONE** | 15m | t006 |
| t008 | [Test coverage](done/t008.md) — **DONE** | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

## Definition of done

- On production, a free Key Value with internal authentication accepts 50 authenticated client connections on its internal URL and on its external TLS URL (from an allowlisted client), and the 51st reads `ERR max number of clients reached`, on TLS inside the handshake.
- While a tenant holds all 50, the Valkey readiness probe, the redis_exporter scrape, the operator's runtime pass and a backup snapshot all still connect (the store stays Ready, metrics keep flowing).
- Enabling internal authentication on a running store restarts no Valkey pod (restart count and `run_id` unchanged), and drops the connections that used the unauthenticated URL, as on Render.
- A test fails if an internal-auth store's `<id>` Service selects the Valkey pods, or if the store's proxy injects `AUTH`.
- ADR021 § 9 and ADR018's "Datastore connection limits by plan" row no longer list stores with internal authentication as open.

## Source + Goal linkage

- **Source:** promoted from inbox note `w6/135` on 2026-11-17, filed by w6/m456 t005 ([m456 Progress](../../done/m456/README.md)) and recorded in [ADR021 § 9](../../../../docs/ADR021-keyvalue-management.md#9-connection-limits-w6m456-2026-11-17) ("Not yet exact: stores with internal authentication") and ADR018's gap backlog. Render: [compute plans § Render Key Value plans](https://render.com/docs/compute-plans#render-key-value-plans) (free 50 … `50g` 50,000) and [Key Value](https://render.com/docs/key-value) ("Any existing connections that use your unauthenticated URL will break").
- **bex today:** `reconcileKeyValueInternalRoute` (`lego/operator/internal/controller/keyvalue_internal_proxy.go`) retires `kv-internal-proxy` when `spec.internalAuth` is set, and `reconcileKeyValueService` (`keyvalue_controller.go`) points `<id>` straight at the Valkey pods. Tenants then meet only `--maxclients` (the limit + the reserve of 10) on both ports, and can hold the reserve, locking the probe, exporter and operator out. At its own limit Valkey drops a TLS client before the handshake with no error (`acceptCommonHandler`, reproduced on the pinned Valkey 8 image in m456). The backup snapshot dials `<id>` for these stores (`keyValueBackupHost`, `keyvalue_backup.go`), so it would take a tenant connection once `<id>` is counted.
- **Builds on m456 (`0eed45bcb`).** Reuse its shared `kvinternalauth.Budget`, the TLS refusal written inside the store's own certificate, the scaled proxy memory limit, `--maxclients` limit + 10 and the backend Service; do not redo them. m456 is closed and live on production since pin `e36296ae8` (producer `e0a5ba20aac8`): the proxy holds the tenant's exact limit, `valkey-server` keeps the reserve of 10, and `cba5b9b5a` fixed the rollout holds, so a limit change restarts no existing store. This milestone's production check (t005) waits only on a deploy carrying its own commit.
- **Goal linkage:** ADR008 § Render parity first. Render holds every Key Value instance to its plan's limit, internal authentication or not; this divergence has no ADR018 sign-off.
- **Expected outcome:** enabling internal authentication no longer raises a store's tenant limit by 10 or lets a tenant starve the platform's clients; the 51st client of a free store reads Valkey's refusal on both URLs.
- **Why now:** internal authentication is a one-click control on every surface (dashboard Connections button, GraphQL, MCP, REST extension), and no production store has it yet (3 stores, all `starter`, all proxied, m456 t002). Fixing it before the first one enables it means no tenant ever sees the looser limit or a store whose probe is locked out.
- **Render parity task:** included. The limit is tenant-visible on every connection, and the enable-internal-auth confirmation on the dashboard, GraphQL, MCP and REST describes what breaks; t006 checks that nothing contradicts the shipped behavior.

## Progress — 2026-11-17 (t001–t004, t007, t008)

Shipped in the commit `git log origin/main --grep 'w6/m463'` names (the `fix(datastores)` one).

- **The proxy stays in front of every store.** `kvinternalauth.Config.Mode` replaces `Passthrough`. `InjectAuth` is w8/204's front, `PlainPassthrough` is new and forwards a plain client's bytes unchanged, and `TLSPassthrough` is the external listener. `kv-internal-proxy` runs its plain listener in `PlainPassthrough` when `BEX_KV_INTERNAL_AUTH=client`, which the operator sets on the proxy of a store with `spec.internalAuth`. Both listeners share the plan's budget, and the client past it reads `-ERR max number of clients reached` in plain text on the plain listener and inside TLS on the TLS one.
- **The check keeps the password** (t001's decision): `--check` is an authenticated `PING` in both modes. Valkey answers an unauthenticated `PING` with `-NOAUTH` before it checks loading, so a password-free check would call a loading server ready.
- **Routing.** `<id>` routes through the proxy for every store once it is ready and stays there. A store that enabled internal auth before this change keeps `<id>` on Valkey until its new proxy is ready. Readiness waits on the proxy for every store. The backup snapshot dials `<id>-backend` for every proxied store.
- **Enabling internal auth rolls only the proxy.** On the local cluster (t003) the old proxy pod served until 2.8 s after the patch, then closed the unauthenticated client it held. The store read `Provisioning` for about a second, and Valkey's `run_id`, restart counts and StatefulSet revision did not change.
- **Verified locally** (t003): 50 authenticated clients admitted and the 51st refused on `<id>:6379`. While they were held, the store stayed Ready, the exporter scraped `redis_up 1`, a snapshot over `<id>-backend` transferred the RDB, and a maxmemory-policy change applied by `CONFIG SET` within a second. The TLS repeat needs a public store, which the local cluster cannot issue a certificate for, so it moves to t005.
- **Found:** a ready backup pod is an endpoint of `<id>-backend`, so a nightly upload resets about half of a store's new connections. Filed as [w6/139](../../139.md); this milestone does not cause it.
- **Simplify and Test coverage** (t007, t008) ran before t005/t006, as in m456: the code is final before the deploy, and t006 reads only surfaces this milestone does not change.

## BLOCKED — 2026-11-17: the production deploy carrying this milestone's commit

- **Done.** t001–t004, t007, t008 (see Progress).
- **Gate (time, clears by itself).** A production pin whose producer contains the commit: `git log origin/main --grep "pin platform images"`, then `git merge-base --is-ancestor <commit> <producer>`. Production deploys `main` once its pin is 6 h old, on a push or the schedule.
- **How to check it cleared.** `bex-system`'s `bex-controller-manager` runs the digest that pin wrote to `deploy/gitops/base/bex.yaml`.
- **Then.**
  - t005 in `bex-canary`:
    - a free probe store with internal auth enabled while a client holds an unauthenticated connection (it drops; Valkey's `run_id` and restart count hold);
    - 50 authenticated clients and the 51st refused, on the internal URL and on the external TLS URL from an allowlisted client;
    - the platform clients while the 50 are held;
    - the three existing `starter` stores' proxies, which must not have rolled for this deploy (none has internal auth, so their templates are unchanged);
    - delete the probes.
  - t006 reads REST, GraphQL, MCP and the dashboard confirmation on production.
  - t009 closes.
