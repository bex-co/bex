# w6 · m512 — A Key Value proxy roll that changes the proxy's code lets open commands finish

**Worker:** worker6 **Goal:** when a deploy does change `/kv-internal-proxy`, the old proxy pod stops accepting, keeps relaying its open connections until each client closes or a bounded grace period ends, and only then closes the rest, so a command in flight at the roll gets its reply. A roll that must cut access, such as enabling internal authentication, still closes the old pod's connections within seconds. **Status:** blocked (t001–t006, t008, t009 done and shipped; t007 waits for the first production pin whose producer contains the ship commit, then t010)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [`kvinternalauth.Serve` drains](done/t001.md) — **DONE** | 1h | — |
| t002 | [`kv-internal-proxy` drains on `SIGTERM`](done/t002.md) — **DONE** | 30m | t001 |
| t003 | [The proxy Deployment gives a draining pod its bound, and tenants cannot hold the reserve](done/t003.md) — **DONE** | 1h | t002 |
| t004 | [A roll that must cut access still closes the old pod's connections within seconds](done/t004.md) — **DONE** | 1h30m | t003 |
| t005 | [Prove it on the local cluster](done/t005.md) — **DONE** | 1h | t004 |
| t006 | [Record it in ADR021 and file the SNI-proxy drain](done/t006.md) — **DONE** | 30m | t004 |
| t007 | [Ship and verify on production](t007.md) | 1h | t005, t006 |
| t008 | [Simplify](done/t008.md) — **DONE** | 15m | t007 |
| t009 | [Test coverage](done/t009.md) — **DONE** | 30m | t007 |
| t010 | [Closeout](t010.md) | 10m | t008, t009 |

## Blocked — 2026-11-19

**Gate (time, clears by itself):** the first production pin whose producer contains this milestone's ship commit. Check it with `git log origin/main --grep "pin platform images"`, then `git merge-base --is-ancestor <commit> <producer>`. Production deploys `main` once its pin is 6 h old. Then run t007's steps: the three stores' proxies rolled once onto the drain env and the 35 s grace period, and the w6/m463 t005 internal-auth check on a `bex-canary` free probe. After that comes t010. Everything else is done: the code, the local proof (t005), ADR021 § 10 with w6/144 and w6/145 filed (t006), Simplify (t008) and Test coverage (t009).

## Definition of done

- A test cancels `Serve`'s drain while a client is mid-command and the client still reads the upstream's reply. A connection attempted after the drain began is not accepted by the old listener. An idle client stays open until the deadline and is closed then. Every budget slot is released.
- `kv-internal-proxy` drains on `SIGTERM` for `BEX_KV_DRAIN_SECONDS` and exits as soon as no stream is open. With the variable unset it closes at once, as today. The proxy Deployment's `terminationGracePeriodSeconds` covers the bound.
- On the local cluster, a proxy roll caused by a binary change lets a client mid-command get its reply, and an idle client is closed no earlier than the grace period.
- Enabling internal authentication still closes a held unauthenticated client within seconds, and new unauthenticated clients read `-NOAUTH`. This holds on the local cluster (the w6/m463 t003 check) and on production on a `bex-canary` free probe (the w6/m463 t005 check).
- While a store at its plan's limit rolls, its tenants cannot take Valkey's platform reserve of 10 (ADR021 § 9). The new proxy's `--check`, the exporter and the operator still connect, and a test covers it.
- ADR021 § 10 states the rule, Render's basis and the gap that remains. The SNI-proxy drain is filed as its own item.

## Source + Goal linkage

- **Source:** promoted from inbox note [w6/142](../../done/142.md) on 2026-11-19 by `/loopx w6`. w6/m469 t004 filed it as w6/140's "drain on `SIGTERM`" direction.
- **Goal linkage:** ADR008 § Render parity first.
  - Render breaks a store's connections when the customer changes the store. Enabling internal authentication: "Render restarts your Key Value instance (it will be unavailable for a few seconds)". A plan change: "Upgrading requires a restart" ([Key Value](https://render.com/docs/key-value), read 2026-11-19).
  - For its own infrastructure, Render breaks them only in service-affecting maintenance, which "usually occur[s] no more than once every three months". Render gives advance notice by email, and a paid service can usually reschedule the window ([platform maintenance](https://render.com/docs/platform-maintenance), read 2026-11-19).
  - bex rolls every store's proxy, without notice, whenever a deploy changes `/kv-internal-proxy`: 2 of the 11 deploys on 2026-11-17 (ADR021 § 10). No ADR018 row signs this off.
  - Draining narrows the gap for short-lived traffic. It does not close it for held clients. t006 records the gap and files the stricter variant.
- **Expected outcome:** commands in flight when a proxy-code deploy lands get their replies instead of a reset. Clients that use a connection per command, or finish within the grace period, see no error. Held clients (pub/sub, blocking commands, pools) still reconnect once, at the end of the grace period at the latest.
- **Why now:** after w6/m469 the proxy rolls only on deploys that change its code, and each of those still fails every in-flight command of every store at once. The same drain pattern just shipped for the activator (w6/m485) and the ssh-gateway (w6/m482). The sequencing constraint below orders this after m469's production check.
- **Render parity task omitted:** the change is an operator-internal mechanism. No REST, GraphQL, MCP or dashboard field or message changes. t005 and t007 check the tenant-visible effect.

## Findings at promotion (2026-11-19, code at `001351697`)

- **The premise holds.** `Serve` (`lego/operator/internal/kvinternalauth/proxy.go`) closes the listener and every active connection in its `context.AfterFunc` as soon as its context is cancelled. `cmd/kv-internal-proxy/main.go` builds that context with `signal.NotifyContext(…, SIGINT, SIGTERM)`. The Deployment (`reconcileKeyValueInternalProxy`, `lego/operator/internal/controller/keyvalue_internal_proxy.go`) runs 1 replica with `maxSurge: 1` and `maxUnavailable: 0`, and sets no `terminationGracePeriodSeconds`, so it gets the default 30 s.
- **What rolls the proxy:**
  - the image: a binary change (w6/m469)
  - `BEX_KV_MAX_CONNECTIONS` and the memory limit: a plan change, which restarts Valkey as well
  - `BEX_KV_INTERNAL_AUTH=client`: enabling internal authentication
  - `credentialRevisionAnnotation`: a credential rotation
  - the TLS env and volume: making the store public or private
  - `replicas: 0`: suspension
- **The old pod cannot tell why it got `SIGTERM`.** Its env and grace period are fixed when it is created, so the new template cannot shorten its drain. A roll that must cut access needs a signal from outside the pod. The operator has no `delete` on pods (`lego/operator/config/rbac/role.yaml`: `get`, `list`, `patch`, `watch`). It does already dial each store's Valkey as `kvPlatformUser`, whose ACL is `+@all` (`valkeyPlatformUserRule`), so `CLIENT LIST` and `CLIENT KILL` are available to it.
- **The reserve.** Valkey runs with `--maxclients` set to the plan's limit plus 10. Each proxy pod holds its own budget of the plan's limit. While an old pod drains, the old and new pods together can therefore admit up to twice the limit, and tenants can take the 10 connections reserved for the readiness probe, the exporter, the proxy's `--check` and the operator (ADR021 § 9). A new proxy pod whose `--check` is refused turns NotReady and leaves the store with no endpoint.
- **SNI proxies.** `bex-kv-sni-proxy` and `bex-pg-sni-proxy` also close every stream on `SIGTERM`. They are `hostPort` DaemonSets that cannot surge (w6/m469 § Findings), so draining them needs a different design, and it is out of scope here. t006 files it.

## Design constraints

- **Do not ship before w6/m469 t005 has its held-client deploy.**
  - This milestone changes `/kv-internal-proxy`, so its shipping deploy rolls every store's proxy once. That roll still closes at once, because the old pods run the old code.
  - m469's DoD bullet 2 needs a production deploy whose `/kv-internal-proxy` is byte-identical while probe `m469-t005-client` holds its connection.
  - Implement and verify locally meanwhile. Ship once m469 t005 has recorded that deploy, or once m469 is in `done/`.
  - This milestone does not change the SNI binaries, so w6/m470 is unaffected.
- **The drain is opt-in by env.** With `BEX_KV_DRAIN_SECONDS` unset the proxy closes at once, as today, so `make run` and the existing tests are unchanged.
- **One roll.** The drain env and `terminationGracePeriodSeconds` land in the same template write as the new image, so no store rolls twice.
- **A roll that must cut access closes at once.** Enabling internal authentication always does. t004 decides, with evidence, for a credential rotation and for making a store private.
- **The proxy pod gains no authority:** no service account token and no new Secret.
