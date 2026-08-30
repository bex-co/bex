# w2 · m57 — Official `kv-cli`: automated PTY + live Valkey acceptance

**Worker:** worker2 **Goal:** Prove the unmodified official Render CLI's `kv-cli [id|name]` command works end to end against bex by automating its interactive TUI under a pseudo-terminal and completing real authenticated Valkey operations over bex's public TLS/SNI edge. **Status:** done (official CLI production acceptance passed; datastore wildcard A/AAAA records reconciled; all tasks and validation complete)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add a redacting PTY probe for the unmodified official CLI — **DONE** | 45m | — |
| t002 | Build the disposable public-Key-Value live verifier — **DONE** | 45m | t001 |
| t003 | Wire the opt-in verifier into the CLI compatibility workflow — **DONE** | 30m | t002 |
| t008 | Reconcile datastore wildcard DNS directly to the raw TCP edge — **DONE** | 45m | t003 |
| t010 | Preserve datastore client IP through the Hetzner TCP edge — **DONE** | 45m | t008 |
| t011 | Emit an SNI-capable public Valkey CLI command — **DONE** | 30m | t010 |
| t004 | Run live id/name acceptance and close the checklist row — **DONE** | 30m | t011 |
| t009 | Audit Render parity after the datastore DNS repair — **DONE** | 15m | t004 |
| t005 | Simplify the PTY and verifier implementation — **DONE** | 20m | t009 |
| t006 | Add meaningful timeout, redaction, failure, and cleanup coverage — **DONE** | 30m | t009 |
| t007 | Closeout — DoD met → move milestone to `done/` — **DONE** | 15m | t005, t006, t008 |

## Definition of done

With the official `render-oss/cli` v2.21.0 binary at commit `c398207` left byte-for-byte unmodified, one documented command provisions a disposable public bex Key Value, runs `render kv-cli --output interactive <red-id> -- ...` and the same command by display name under an automated pseudo-terminal, and proves `PING`, unique-key `SET`, exact-value `GET`, and cleanup through the real `rediss://` endpoint. The run uses bex-api authentication and the CLI's own `/v1/key-value` resolution plus `connection-info` path; it does not execute a copied connection string directly. It requires no human keystrokes or attached terminal, has bounded timeouts, cleans the fixture after success or failure, and never prints bearer tokens, passwords, credential-bearing URIs, or kubeconfig contents. Sanitized dated evidence from a full external-edge environment updates `docs/cli-compatibility-checklist.md`'s `kv-cli [id|name]` row to `[x]`, with a reproducible rerun entrypoint.

## Research findings

- The checklist's original blocker was harness-only: the CLI auto-selects text output when all streams are not TTYs, and `ParseCommandInteractiveOnly` rejects the command before it constructs a client. An explicit `--output interactive` overrides that auto-selection; the TUI can then be driven under a pseudo-terminal.
- Arguments after `--` are appended to the spawned `redis-cli`/`valkey-cli` process. One-shot commands such as `PING`, `SET`, and `GET` therefore terminate without a person exiting an interactive REPL; a successful child exit makes the CLI's one-item TUI stack exit too.
- The ID path first calls `GET /v1/key-value/{id}/connection-info`. The name path lists with `?name=...`, requires exactly one match, then calls connection-info with the resolved `red-` id. Both paths must be exercised; proving only one does not close the row.
- bex chooses the external `rediss://` URI when the Key Value is public. After t011, public connection-info returns `redis-cli --sni <externalHost> -u <uri>` because `redis-cli` does not infer a TLS server name from the URI; private connection-info retains `redis-cli -u <uri>`.
- A local pseudo-terminal probe with v2.21.0 reached bex's Key Value REST path instead of firing the interactive-only guard, disproving "not verifiable headlessly." The later production run completed the real public-endpoint proof.
- dev-9 cannot supply that proof: its documented omissions include `BEX_KV_DOMAIN`, cert-manager, and external datastore connectivity. Production supplied the complete `:6379` edge, DNS, certificate, SNI proxy, and explicitly allowed source CIDR.
- The official CLI's Render `KeyValuePOSTInput` has no bex-only `public` field. A nonempty Render `ipAllowList` is now the REST adapter's public intent, so the verifier creates its disposable fixture through the unmodified CLI's documented `keyvalues create --ip-allow-list` path and still uses that same CLI for both connection attempts.
- The first production verifier run on 2026-07-18 reached connection-info and launched `redis-cli`, but bounded every command at 45 seconds. A follow-up TCP preflight proved the available fixture's public `:6379` endpoint unreachable. Hetzner reported the load balancer's 6379 service healthy, while `*.kv.bex.co` resolved through the same Cloudflare-proxied address set as `api.bex.co` instead of the load balancer. ADR021 already requires DNS-only records because ordinary Cloudflare proxying cannot carry raw Valkey TCP; t008 repairs and gates that deployed contract before t004 reruns acceptance.
- After the DNS repair, a second production run reached the Valkey SNI proxy but every exact-source request was rejected before backend dial. The Terraform TCP listener had PROXY protocol disabled, so the proxy saw the Hetzner load balancer address instead of the client address despite `externalTrafficPolicy: Local`; t010 restores the original-source trust chain for both datastore allowlists before acceptance reruns.
- After the source-IP repair, a third production run passed both allowlist layers but the SNI proxy rejected the official CLI's TLS ClientHello because it had no server name. `redis-cli` 8.0.3 does not infer SNI from `rediss://`; t011 adds its supported explicit `--sni <externalHost>` argument to the public connection-info command while leaving private commands unchanged.
- The final 2026-07-18 production run passed opaque-id `PING`/`SET`, display-name `GET`/`DEL`, and every cleanup assertion with the official v2.21.0 CLI unmodified. Sanitized evidence is in [`evidence/2026-07-18-kv-cli-acceptance.md`](evidence/2026-07-18-kv-cli-acceptance.md).

## Source + Goal linkage

- **Source:** user request 2026-07-18 to research and arrange w2 work for [`docs/cli-compatibility-checklist.md`](../../../docs/cli-compatibility-checklist.md)'s open `kv-cli [id|name]` row; source inspection of `render-oss/cli` v2.21.0 (`c398207`) `cmd/kvcli.go`, `pkg/command/outputresolver.go`, and `pkg/tui/views/rediscli.go`; existing bex contracts in [`docs/ADR021-keyvalue-management.md`](../../../docs/ADR021-keyvalue-management.md), `lego/backend/internal/keyvalue/service.go`, and `scripts/cli-compat.sh`.
- **Goal linkage:** ADR008 pillar 1 and the AI-native thesis's Render-tooling compatibility: Render's official CLI is bex's fifth compatibility surface, and this closes the last unverified Key Value session command without building a first-party CLI.
- **Expected outcome:** `kv-cli` is a reproducibly verified `[x]` by opaque id and display name, including real authentication, TLS/SNI routing, password use, and Valkey data operations; future regressions fail a maintained opt-in verifier rather than returning the row to manual guesswork.
- **Why now:** w9's 2026-07-18 sweep left this row open solely because dev-9 had no TTY or public Key Value edge. The upstream code exposes a bounded automation seam, bex's production `:6379` edge already exists, and w2 owns the shipped Key Value API plus interactive-access verification patterns.
- **Render parity closing task:** t009 was added after the production run exposed the datastore raw-TCP DNS defect. The repair remains infrastructure-only, so the audit must confirm REST/GraphQL/MCP connection-info semantics and dashboard behavior remain unchanged while the official Render CLI gains the already-promised public reachability.
