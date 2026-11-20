# w9 · m521 — A new Key Value's connection strings take Render's shapes: a credential-free internal URL and the external URL from the first read, a `REDISCLI_AUTH=… valkey-cli` command, and the store id as the user

**Worker:** worker9 **Goal:** a client that reads a new Key Value's connection info, straight after the create as Render's Terraform provider does, gets the three strings Render answers, in Render's shapes, and each one connects once the store is available. **Status:** todo (t001–t005, t007–t009 done)

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| [t001](done/t001.md) | A new store's internal URL is credential-free from its first reconcile — **DONE** | 1h30m | — |
| [t002](done/t002.md) | The external URL is answered from the create, and `externalCertificate` says whether it is open yet (amends w8/237) — **DONE** | 1h | — |
| [t003](done/t003.md) | The CLI command reads ` REDISCLI_AUTH=<password> valkey-cli --user … -h … -p … --tls`, and `bex kv-cli` still connects — **DONE** | 45m | t002 |
| [t004](done/t004.md) | The credentialed strings name the store id as the user, with no running store restarted — **DONE** | 2h | t003 |
| [t005](done/t005.md) | Recheck `keyvalue` and `redis` with the Terraform harness on dev-9 and grade class 10 — **DONE** | 1h | t001, t004 |
| [t006](t006.md) | Production check on a pin that carries the milestone | 45m | t005 |
| [t007](done/t007.md) | Render parity — **DONE** | 30m | t006 |
| [t008](done/t008.md) | Simplify — **DONE** | 20m | t007 |
| [t009](done/t009.md) | Test coverage — **DONE** | 45m | t007, t008 |
| [t010](t010.md) | Closeout | 15m | t009 |

Total: **10 tasks, about 8h45m**.

## Definition of done

1. A Key Value created public answers its first connection-info read, while it is still `creating`, on REST `GET /v1/key-value/{id}/connection-info`, `GET /v1/redis/{id}/connection-info` (`redisCLICommand`) and GraphQL `keyValueConnectionInfo`, with:
   - `externalConnectionString` `rediss://<id>:<password>@<externalHost>:6379`;
   - `internalConnectionString` `redis://<id>:6379`, with no credentials;
   - `cliCommand` ` REDISCLI_AUTH=<password> valkey-cli --user <id> -h <externalHost> … -p 6379 --tls`, with Render's leading space and the `--sni <externalHost>` the shared SNI edge needs, placed where t003 records.
2. Each string connects once the store is available, on dev-9 and on production. The internal URL connects from a service in the same workspace and from no other workspace. The external URL and the CLI command, run verbatim, connect from an allowed source once the TLS listener opens. `bex kv-cli` (the unmodified Render CLI) connects too.
3. No running store restarted for the route or the username change. A Persistence Off fixture keeps its keys, its `run_id` and its restart count.
4. `TestAccKeyvalueDataSource` and `TestAccRedisDataSource` pass check 12 (internal). The [checklist](../../../docs/terraform-compatibility-checklist.md) records that checks 11 and 13 fail only on `oregon-keyvalue\.`, Render's regional shared hostname, and that every other part of both regexes matches.
5. [ADR021](../../../docs/ADR021-keyvalue-management.md) (the Internal URL row, the w8/237 bullet and the `redis-cli --sni` paragraph) and the Key Value row of [ADR018](../../../docs/ADR018-render-parity.md) describe the shipped strings. Any remaining difference is recorded there.

## Source + Goal linkage

- **Source:** inbox note [w9/122](../done/122.md), promoted 2026-11-19 by `/loopx w9`. It came from w9/108's Terraform recheck on dev-9 (2026-11-18, [checklist](../../../docs/terraform-compatibility-checklist.md) class 10).
- **Goal linkage:** Render parity for managed Key Value ([ADR008 § Render parity first](../../../docs/ADR008-vision.md#render-parity-first), [ADR018](../../../docs/ADR018-render-parity.md), [ADR021](../../../docs/ADR021-keyvalue-management.md)). Render's Terraform provider is a client bex must serve ([checklist](../../../docs/terraform-compatibility-checklist.md)), and so is every client that copies Render's strings.
- **Expected outcome:** a `render_keyvalue` or `render_redis` data source reads the same three strings Render's does. A customer moving from Render pastes the same internal URL and the same CLI command and connects.
- **Why now:** class 10 is the last bex-side failure in both Key Value data source tests (checks 1–10 pass since w9/108).
- **Render parity:** included (t007). The strings are served on REST, the `/v1/redis` alias and GraphQL. They are shown in the dashboard's Connect panel and the CLI's `keyvalues` output, and run by `kv-cli`. Render's MCP server has no credential read.

## Findings (triage, 2026-11-19, main `ca468bd9d`)

- **Render's answers.** In the provider's cassettes at pin `06af456`, every store answers its first `connection-info` read while it is `unknown`/`creating`. There are nine stores across the keyvalue and redis packages. Three of them were created with `ipAllowList: []`. Each answer has three strings:
  - `externalConnectionString` `rediss://red-…:<32 chars>@oregon-keyvalue.localhost.render.com:6377`;
  - `internalConnectionString` `redis://red-…:6379`;
  - `cliCommand` ` REDISCLI_AUTH=<32 chars> valkey-cli --user red-… -h oregon-keyvalue.localhost.render.com -p 6377 --tls`.

  So the note's first open question is settled: a new store's internal URL carries no credentials, as Render documents (ADR021 § Internal authentication, checked 2026-10-28).
- **The checks** (`internal/provider/keyvalue/datasource/datasource_test.go` and `redis/datasource/datasource_test.go`, lines 41/49/57):
  - check 11: `^rediss://red-.*:.{32}@oregon-keyvalue\..*:637[7,9]$`;
  - check 12: `^redis://red-.*:637[7,9]$`;
  - check 13: `^ REDISCLI_AUTH=.{32} valkey-cli --user red-.* -h oregon-keyvalue\..* -p 637[7,9] --tls$`.

  The resource tests' looser forms (`resource_test.go:68/76/84`) want a host ending in `com`. `oregon-keyvalue.` is Render's regional shared TLS endpoint, which routes by username, and port `6377` is Render's external port (the regexes admit `6379`). Those are Render's own infrastructure. bex routes each store by SNI at `<id>.<BEX_KV_DOMAIN>:6379` (ADR021). bex passwords are 32 URL-safe characters (`lego/types/credpw`), so `.{32}` matches.
- **bex today:**
  - **Internal.** `KeyValueConnectionInfo` (`lego/backend/internal/keyvalue/service.go:1407`) drops the credentials only once `status.internalAuthOptional` holds (`service.go:1458`). The operator sets that flag only after the store's proxy is Ready and its stable Service has switched (`lego/operator/internal/controller/keyvalue_internal_proxy.go:318-331`). A read at create therefore answers `redis://default:<password>@<id>.<ns>.svc:6379`.
  - **External.** The external string is blanked while `status.tlsListener` is false (w8/237, `service.go:1449-1456`).
  - **CLI.** The CLI command is `redis-cli -u …` or `redis-cli --sni <host> -u …` (`service.go:1465-1480`).
  - **User.** Every credentialed string and the compatibility Secret name the `default` user (`keyvalue_controller.go:301-315`). Valkey's tenant ACL is server args (`valkeyDefaultUserRule`, `keyvalue_controller.go:262`), so adding a user to the template restarts a store whose template is re-rendered.
- **Internal authentication today.** Every store has an auth-injecting proxy: the manager always sets `InternalProxyImage` (`cmd/manager/main.go:701-713`), and readiness already waits for it (`updateKeyValueReadiness`). The proxy authenticates each upstream connection as the restricted `default` user. Valkey keeps its password and ACL. The NetworkPolicies admit the workspace (and environment) peers to the stable Service, and only the store's own proxy to its backend. Dropping the password from the string therefore turns no auth off and widens no reach. t001 records the security reasoning.
- **valkey-cli 8.1.1** sends a TLS server name only with `--sni` (`src/cli_common.c:111`, flag `src/valkey-cli.c:2799`). It reads `REDISCLI_AUTH` (`src/valkey-cli.c:87`). bex's shared edge needs the server name, so `--sni` stays in bex's command.
- **The Render CLI's `kv-cli`** (`render-oss/cli` `pkg/tui/views/rediscli.go`, `loadDataRedisCLI`) splits `cliCommand` on spaces. It runs everything after `redis-cli`/`valkey-cli` as arguments and sets the process environment to exactly the words before it, so it relies on the `REDISCLI_AUTH=…` prefix. It falls back to `redis-cli` when `valkey-cli` is not installed. t003 rechecks it.
- **Out of scope here, for t007 to weigh.** Render answers an external string even for a store with `ipAllowList: []`: the endpoint exists and the list admits nothing. A private bex store (`spec.public` false, w4/m116, w4/m315) has no external endpoint and no external string.
