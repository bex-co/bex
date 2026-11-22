# w8 · m562 — A Key Value client outside the IP allow list reads Render's "AUTH failed: Client IP address is not in the allowlist.", not a bare TLS EOF

**Worker:** worker8 **Goal:** a client whose source the store's allow list refuses completes a TLS handshake under the store's own certificate and reads `-Client IP address is not in the allowlist.` for every command it sends, so `valkey-cli` and `bex kv-cli` print Render's documented `AUTH failed: Client IP address is not in the allowlist.` No byte from a refused source reaches Valkey, and an unknown hostname is still dropped before TLS. **Status:** todo (t001 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | `kv-internal-proxy`: an allow-list refusal listener that ends TLS with the store's certificate and answers every command with Render's refusal, never dialing Valkey — **DONE** | 45m | — |
| [t002](t002.md) | Operator: the proxy and the stable Service carry the refusal port; the held-template roll decision | 45m | t001 |
| [t003](t003.md) | `kv-sni-proxy`: a refused source on a known store's name goes to the refusal port, inside the handshake caps and counted as a rejection | 45m | t002 |
| [t004](t004.md) | Docs: ADR021 and the operator guide describe the refusal path | 20m | t003 |
| [t005](t005.md) | Live check on production in `bex-canary` | 30m | t004 |
| [t006](t006.md) | Render parity | 15m | t005 |
| [t007](t007.md) | Simplify | 15m | t006 |
| [t008](t008.md) | Test coverage | 30m | t006, t007 |
| [t009](t009.md) | Closeout | 10m | t008 |

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

- **Source:** inbox note [w8/408](../done/408.md), filed by routine `cli-qa` sweep r32 on 2026-11-22 and promoted by `/loopx w8` the same day.
- **Triage evidence (2026-11-22, `b339bb1828`):**
  - **The premise holds.** `kvRouter.resolve` returns `false` when `!route.Allow.Admits(source)` (`lego/operator/cmd/kv-sni-proxy/main.go:163`). `handleConn` then logs `no allowed route` and closes after the ClientHello (`main.go:351`), exactly as for an unknown name.
  - **The store's certificate is already in the right place.** Each store's `kv-internal-proxy` mounts its `<id>-kv-tls` Secret, optional, and already ends TLS with it to answer a client past the budget with Valkey's maxclients refusal (`kvinternalauth.refuse`, `lego/operator/internal/kvinternalauth/proxy.go:447`; rendering at `internal/controller/keyvalue_internal_proxy.go:119`). A refusal listener beside it keeps tenant private keys in their own namespace. `kv-sni-proxy` cannot end TLS itself until w4/m357 t002's `*.kv.bex.co` wildcard lands, which is gated.
  - **The network already admits a new port.** `kv-sni-proxy` dials the stable Service `<id>.<ns>.svc.cluster.local:6380` (`main.go:118`), which selects the proxy (`keyvalue_controller.go:806`). Tenant namespaces admit `bex-system` on every port (`allow-datastore-control-ingress`, peer-scoped, `lego/backend/internal/store/namespaces.go:1136`). No egress policy names 6380.
  - **The wire text.** Render documents the client's display, `AUTH failed: Client IP address is not in the allowlist.` (render.com/docs/key-value § Enabling external connections, `key-value.md` lines 340–344, read 2026-11-22). `valkey-cli` prints `AUTH failed: %s` with the server's error string itself (`valkey-cli.c:1549` at Valkey 8.0.2, `cliAuth`). So the server's string is `Client IP address is not in the allowlist.`. A server reply of `-AUTH failed: …` would print `AUTH failed: AUTH failed: …`.
  - **Every command, not just the first.** In one-shot mode `valkey-cli` keeps the connection after a failed `AUTH` and sends the command on it (`cliConnect(CC_QUIET)` then `noninteractive`, `valkey-cli.c:9709-9710`). A refusal that closed after its first reply would print `AUTH failed: …` and then `Server closed the connection`. Answering each command prints Render's line, then `(error) Client IP address is not in the allowlist.`.
  - **Postgres has no documented wording to match.** `pg-sni-proxy` drops a refused source the same way (`cmd/pg-sni-proxy/main.go:295`), but Render's Postgres docs document no message for a disallowed IP (render.com/docs/postgresql-creating-connecting § Restricting external access, read 2026-11-22). `bex psql` refuses first on the client with the pinned client's own `IP address (<ip>) not in allow list for <name>` (`pkg/tui/views/psql.go:66`), as Render's CLI does. So there is no Postgres parity target, and this milestone leaves `pg-sni-proxy` alone.
  - **More than one sitting.** It touches a package and a binary, the operator's rendering and its held-template rule, the shared proxy, two docs and a live check.
- **Goal linkage:** ADR008 § Render parity first, for managed Key Value ([ADR021](../../../docs/ADR021-keyvalue-management.md)). bex's own Key Value skill pack already promises Render's line (`lego/cli/internal/skills/pack/skills/bex-keyvalue/references/troubleshooting.md:3`).
- **Expected outcome:** a customer blocked by their own allow list reads that it blocked them, instead of an EOF that looks like an outage, a wrong host or a TLS fault.
- **Why now:** every external Key Value user who sets an allow list meets it on a changed IP, and the skill pack sends them to the allow list for an error they never see.
- **Render parity included:** the refusal is a tenant-facing wire behavior. REST, GraphQL, MCP and the dashboard do not change, so t006 checks the wire against Render's docs and the pinned client.
