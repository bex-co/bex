# w8 · m562 — A Key Value client outside the IP allow list reads Render's "AUTH failed: Client IP address is not in the allowlist.", not a bare TLS EOF

**Worker:** worker8 **Goal:** a client whose source the store's allow list refuses completes a TLS handshake under the store's own certificate and reads `-Client IP address is not in the allowlist.` for every command it sends, so `valkey-cli` and `bex kv-cli` print Render's documented `AUTH failed: Client IP address is not in the allowlist.` No byte from a refused source reaches Valkey, and an unknown hostname is still dropped before TLS. **Status:** done (2026-11-23; t005's live check passed on production pin `1a5a8e872398`)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `kv-internal-proxy`: an allow-list refusal listener that ends TLS with the store's certificate and answers every command with Render's refusal, never dialing Valkey — **DONE** | 45m | — |
| [t002](done/t002.md) | Operator: the proxy and the stable Service carry the refusal port; the held-template roll decision — **DONE** | 45m | t001 |
| [t003](done/t003.md) | `kv-sni-proxy`: a refused source on a known store's name goes to the refusal port, inside the handshake caps and counted as a rejection — **DONE** | 45m | t002 |
| [t004](done/t004.md) | Docs: ADR021 and the operator guide describe the refusal path — **DONE** | 20m | t003 |
| [t005](done/t005.md) | Live check on production in `bex-canary` — **DONE** | 30m | t004 |
| [t006](done/t006.md) | Render parity — **DONE** | 15m | t005 |
| [t007](done/t007.md) | Simplify — **DONE** | 15m | t006 |
| [t008](done/t008.md) | Test coverage — **DONE** | 30m | t006, t007 |
| [t009](done/t009.md) | Closeout — **DONE** | 10m | t008 |

Total: **9 tasks, about 4h15m**.

## Definition of done

1. **The refusal is Render's.** From a host outside a store's allow list, on that store's public hostname:
   - `bex kv-cli <id> -- PING` in a PTY prints `AUTH failed: Client IP address is not in the allowlist.`
   - `valkey-cli --tls --sni <id>.kv.bex.co -h <id>.kv.bex.co -p 6379 PING`, with no password, prints `(error) Client IP address is not in the allowlist.`
   - The wire reply is `-Client IP address is not in the allowlist.\r\n`, for every command the client sends, until the client closes or the refusal's bounded lifetime ends.
2. **Nothing reaches Valkey.** A refused connection never opens a stream to Valkey or to the proxy's TLS pass-through. A test proves the upstream saw no connection. The refusal never checks the password, so it reveals nothing beyond Render's text.
3. **The controls hold.**
   - An unknown hostname (`nope.kv.bex.co`) is still dropped after the ClientHello, before any handshake.
   - An admitted source still reads `NOAUTH` without the password and `PONG` with it.
   - A store with external access cleared (`--clear-ip-allow-list`) is unchanged: its connection info has an empty `externalConnectionString`.
4. **Bounded.** A refused connection holds its per-source and per-network handshake slots (w21/070) for its whole life, which ends at most `handshakeTimeout` (10 s) after accept. It is not counted toward the store's route capacity. `bex_kv_proxy_connections_rejected_total` counts it under its own reason. The refusal listener answers a bounded number of refusals at once and closes the rest.
5. **Degrades to today.** A store whose proxy runs a template without the refusal listener, or whose stable Service still selects Valkey directly, gets today's EOF: the dial to the refusal port fails, and nothing reaches Valkey.
6. **Tests and docs.** Operator tests cover the refused-source route, the refusal listener, the rendered port and the degraded path. ADR021 § IP allowlist and the operator guide describe the path.

## Source + Goal linkage

- **Source:** inbox note [w8/408](../../done/408.md), filed by routine `cli-qa` sweep r32 on 2026-11-22 and promoted by `/loopx w8` the same day.
- **Triage evidence (2026-11-22, `b339bb1828`):**
  - **The premise holds.** `kvRouter.resolve` returns `false` when `!route.Allow.Admits(source)` (`lego/operator/cmd/kv-sni-proxy/main.go:163`). `handleConn` then logs `no allowed route` and closes after the ClientHello (`main.go:351`), exactly as for an unknown name.
  - **The store's certificate is already in the right place.** Each store's `kv-internal-proxy` mounts its `<id>-kv-tls` Secret, optional, and already ends TLS with it to answer a client past the budget with Valkey's maxclients refusal (`kvinternalauth.refuse`, `lego/operator/internal/kvinternalauth/proxy.go:447`; rendering at `internal/controller/keyvalue_internal_proxy.go:119`). A refusal listener beside it keeps tenant private keys in their own namespace. `kv-sni-proxy` cannot end TLS itself until w4/m357 t002's `*.kv.bex.co` wildcard lands, which is gated.
  - **The network already admits a new port.** `kv-sni-proxy` dials the stable Service `<id>.<ns>.svc.cluster.local:6380` (`main.go:118`), which selects the proxy (`keyvalue_controller.go:806`). Tenant namespaces admit `bex-system` on every port (`allow-datastore-control-ingress`, peer-scoped, `lego/backend/internal/store/namespaces.go:1136`). No egress policy names 6380.
  - **The wire text.** Render documents the client's display, `AUTH failed: Client IP address is not in the allowlist.` (render.com/docs/key-value § Enabling external connections, `key-value.md` lines 340–344, read 2026-11-22). `valkey-cli` prints `AUTH failed: %s` with the server's error string itself (`valkey-cli.c:1549` at Valkey 8.0.2, `cliAuth`). So the server's string is `Client IP address is not in the allowlist.`. A server reply of `-AUTH failed: …` would print `AUTH failed: AUTH failed: …`.
  - **Every command, not just the first.** In one-shot mode `valkey-cli` keeps the connection after a failed `AUTH` and sends the command on it (`cliConnect(CC_QUIET)` then `noninteractive`, `valkey-cli.c:9709-9710`). A refusal that closed after its first reply would print `AUTH failed: …` and then `Server closed the connection`. Answering each command prints Render's line, then `(error) Client IP address is not in the allowlist.`.
  - **Postgres has no documented wording to match.** `pg-sni-proxy` drops a refused source the same way (`cmd/pg-sni-proxy/main.go:295`), but Render's Postgres docs document no message for a disallowed IP (render.com/docs/postgresql-creating-connecting § Restricting external access, read 2026-11-22). `bex psql` refuses first on the client with the pinned client's own `IP address (<ip>) not in allow list for <name>` (`pkg/tui/views/psql.go:66`), as Render's CLI does. So there is no Postgres parity target, and this milestone leaves `pg-sni-proxy` alone.
  - **More than one sitting.** It touches a package and a binary, the operator's rendering and its held-template rule, the shared proxy, two docs and a live check.
- **Goal linkage:** ADR008 § Render parity first, for managed Key Value ([ADR021](../../../../docs/ADR021-keyvalue-management.md)). bex's own Key Value skill pack already promises Render's line (`lego/cli/internal/skills/pack/skills/bex-keyvalue/references/troubleshooting.md:3`).
- **Expected outcome:** a customer blocked by their own allow list reads that it blocked them, instead of an EOF that looks like an outage, a wrong host or a TLS fault.
- **Why now:** every external Key Value user who sets an allow list meets it on a changed IP, and the skill pack sends them to the allow list for an error they never see.
- **Render parity included:** the refusal is a tenant-facing wire behavior. REST, GraphQL, MCP and the dashboard do not change, so t006 checks the wire against Render's docs and the pinned client.

## Progress

- **2026-11-22: t001–t004 done** in `3de6522126` (the refusal listener), `07a8c672c2` (the rendered port), `0eb545da0a` (the routing and the alert exclusion) and `51d6cbf096` (the docs). Each task file records its design and tests.
- **Local check of DoD 1, not production.** `valkey-cli` 8.1 (`valkey/valkey:8.1`) in a TTY, against a locally built `/kv-internal-proxy` refusal listener with a self-signed certificate:
  - with no password, `PING` prints `(error) Client IP address is not in the allowlist.`;
  - with `REDISCLI_AUTH` and `--user`, the shape of `kv-cli`'s command, it prints `AUTH failed: Client IP address is not in the allowlist.` and then `(error) Client IP address is not in the allowlist.`.
  - Without a TTY it prints the same lines without `(error) `.
- **DoD 3's last control moved.** Since w9/134 (`4eda96b34f`), a private store's connection info answers a `rediss://` URL over its reserved host, where it answered an empty `externalConnectionString`. The control becomes: a store cleared with `--clear-ip-allow-list` answers what w9/134 answers, and a client of its host is still dropped after the ClientHello, since the proxy routes no private store and a private store has no certificate.
- **t006 partly done (2026-11-22).** No API or dashboard surface changed, and the skill pack's page matches what `valkey-cli` prints. A private store's reserved host still reads a bare EOF, filed as [w8/409](../../done/409.md). Comparing with t005's live output is left (t006's record).

## Test map (t008)

| DoD | Tests |
| --- | --- |
| 1. The refusal is Render's | `TestAllowListRefusalAnswersEveryCommandInsideTLS` (AUTH, the command after it, a pipeline, inline commands; empty commands unanswered), `TestRunAnswersARefusedExternalClientWithRendersText`, `TestARefusedSourceReadsRendersRefusalUnderTheStoresCertificate` (end to end under the store's certificate); the local `valkey-cli` 8.1 check above; t005 live |
| 2. Nothing reaches Valkey | `TestARefusedSourceReadsRendersRefusalUnderTheStoresCertificate` (the backend sees no connection), `TestRunAnswersARefusedExternalClientWithRendersText` (the upstream reads nothing), `TestAllowListRefusalNamesNoUpstreamOrPassword` (no upstream or password can be configured; a counting upstream sees nothing); an AUTH with a password reads the same reply as PING |
| 3. The controls hold | `TestAnUnknownNameIsStillDroppedBeforeTheHandshake`, the admitted control in `TestARefusedSourceReadsRendersRefusalUnderTheStoresCertificate`, `TestPublicProxyPreservesEndToEndTLSAndMetersOnlyBackendWrites` (pass-through and cleared access), `TestRouterRoutesPublicKeyValueAndPreservesAllowlist` (refused, unrouted, cleared) |
| 4. Bounded | `TestARefusedConnectionHoldsItsHandshakeSlotAndNoRouteSlot`, `TestTheRefusalRelayEndsAtItsDeadline`, `TestAllowListRefusalClosesAnIdleClientAtItsDeadline`, `TestAllowListRefusalsPastTheBoundAreClosed`, `TestAllowListRefusalClosesAfterItsCommandBound`, `TestAllowListRefusalClosesAMalformedOrOversizedCommand`; `reason="allowlist"` in the first test; `datastore_proxy_shedding_alerts_test.yml` |
| 5. Degrades to today | `TestARefusalPortThatRefusesTheDialClosesTheClient`, the envtest spec "keeps a public store's template without the allow-list refusal, while its Service gains the port", `TestKeyValueInternalProxyAnswersAllowListRefusalsWithTheTLSPort` (a port Valkey never listens on) |

## Parked (2026-11-22): t005 waits for production's pin

- **Gate:** pin must carry `0eb545da0a`, t003's routing, which carries t001 `3de6522126` and t002 `07a8c672c2`. Production runs `eaaa61506165`, pinned at 15:04 UTC. The deploy running at 21:38 UTC builds `19fdb46f17`, which carries none of them. The next roll is the first push or hourly cron at least 6 h after that run's pin.
- **Next:**
  - Run t005 as written: a free store in `bex-canary` with `cidr=10.0.0.0/8`, the `bex kv-cli` and `valkey-cli` checks, the controls, and a delete by ID with its 404s. Check DoD 3's last control in its w9/134 form (above).
  - Then finish t006 step 1 and close out with t009.
  - `kv-sni-proxy`'s own pin moves only when its binary changes. `0eb545da0a` changes it, so check that the `kv-sni-proxy=` digest in `deploy/gitops/base/bex.yaml` moved with the operator's.

## Unblocked and closed (2026-11-23)

- **The gate cleared.** The newest pin on `origin/main`, `e6c65f5c62` (04:25 UTC), pins `1a5a8e872398`. `git merge-base --is-ancestor` holds for `0eb545da0a` and for t001 `3de6522126` and t002 `07a8c672c2`. The same pin moved the `kv-sni-proxy=` digest in `deploy/gitops/base/bex.yaml` with the controller's (both `sha256:65ef4f0d…`), so the front door runs t003's binary. The roll was live by behavior: only the new binaries answer the refusal below.
- **t005, live on that pin.** Run 2026-11-23 05:27–05:36 UTC from this host, which is outside `10.0.0.0/8`, with `bex` v0.9.0, `valkey-cli` 8.1.12 (`valkey/valkey:8-alpine`) and OpenSSL.
  - **Fixture:** free Key Value `qa-20261123-m562-kv` (`red-dc1std8oodks73aud77g`, disposable) in `bex-canary`, created with `--ip-allow-list "cidr=10.0.0.0/8,description=private"`. It was available in 22 s, and its connection info read `externalCertificate: publiclyTrusted`.
  - **DoD 1 holds.**
    - `bex kv-cli <id> -- PING` in a PTY, with and without `--confirm`, ran the connection info's `cliCommand` and printed `AUTH failed: Client IP address is not in the allowlist.`, then `(error) Client IP address is not in the allowlist.`.
    - `valkey-cli --tls --sni <id>.kv.bex.co -h <id>.kv.bex.co -p 6379 PING`, with no password, printed `(error) Client IP address is not in the allowlist.` in a PTY, and `Client IP address is not in the allowlist.` without one.
    - On the wire, `openssl s_client -verify_return_error` verified the store's own certificate (`CN` and SAN `<id>.kv.bex.co`, issuer Let's Encrypt `YR2`). An inline `PING` and then a RESP `GET k` each read `-Client IP address is not in the allowlist.\r\n` (two replies, 90 bytes).
  - **DoD 3 holds.**
    - With `--sni nope.kv.bex.co` on the store's address, `valkey-cli` printed `SSL_connect failed: unexpected eof while reading`. `s_client` wrote its 323-byte ClientHello and read 0 bytes, with no peer certificate.
    - The admitted control (`NOAUTH`, then `PONG`) rests on `TestARefusedSourceReadsRendersRefusalUnderTheStoresCertificate`, as t005 step 4 says. The production list was never widened.
    - The cleared control, in its w9/134 form: `bex kv update <id> --clear-ip-allow-list` answered `ipAllowList: []`. Connection info then had no `externalCertificate`, and its `externalConnectionString` and `cliCommand` named the reserved host `<id>.kv.bex.co:6379` (`rediss://`, `--tls`). A client of that host read `unexpected eof`, with 0 bytes read, because the edge routes no private store ([w8/409](../../done/409.md) closed that as no divergence).
  - **DoD 4, bounded.** This ran on a second fixture, `qa-20261123-m562-kv2` (`red-dc1t0qfqbg1c739j1e80`, disposable), with the same list. The server closed an idle refused connection 6.0 s after connect. A connection that sent `PING` once a second read 6 refusals before the same close. Both fit inside the edge's 10 s deadline: the refusal listener's own handshake timeout defaults to 5 s (`internal/kvinternalauth/proxy.go:195`).
  - **Cleanup.** Both stores were deleted by ID. Afterwards `bex kv get` exits 1 by ID and by name (`No Key Value with ID …`), and `kv list` omits both. REST `GET /v1/key-value/<id>` and `…/connection-info` answer 404 `{"error":"not found","id":"not_found","message":"not found"}`, and GraphQL `keyValue(id)` answers `null` with `NOT_FOUND`. The isolated CLI was logged out, and each Kratos jar was logged out before it was deleted. No connection string or password was printed; each stayed in a 0600 scratch file, since deleted.
  - **Not checked live:** DoD 2 (nothing reaches Valkey) and the `reason="allowlist"` counter need cluster access, so they rest on t001's and t003's tests.
- **t006 is done.** Step 1: render.com/docs/key-value.md (read 2026-11-23, lines 340–344) says "If you attempt to connect from a disallowed IP address, your client will display an error like the following: `AUTH failed: Client IP address is not in the allowlist.`", the line `bex kv-cli` printed. Step 4: ioredis 5.4.1 (Node 22, dummy password) emitted `ReplyError: Client IP address is not in the allowlist.`. That is the same server text; the `AUTH failed: ` prefix is `valkey-cli`'s own. No drift to file.
- **t009:** every task is done and the Definition of done holds, so the milestone moves to `done/`.
