# Production official `kv-cli` acceptance — 2026-07-18

## Scope

- Target: `https://api.bex.co` plus the production public Valkey TLS/SNI edge.
- Client: the unmodified official `render-oss/cli` v2.21.0 binary built from commit `c398207`.
- Entrypoint: `scripts/cli-compat.sh kv-cli-verify` with production API/Hydra origins, an ephemeral workspace-scoped API key, and the runner's exact public `/32` allowlist.
- Automation: the verifier supplied `--output interactive` under a pseudo-terminal and appended one-shot Redis arguments after `--`; no human terminal or keystrokes were attached.

## Result

| Assertion | Result |
| --- | --- |
| Disposable Hobby workspace and workspace-scoped API key | PASS |
| Source-restricted public Key Value creation | PASS |
| Public DNS and TCP/6379 readiness | PASS |
| Opaque `red-` id: `PING` | PASS |
| Opaque `red-` id: unique-key `SET` | PASS |
| Display name resolution: exact-value `GET` | PASS |
| Display name resolution: key `DEL` | PASS |
| CLI-owned list/item/connection-info path | PASS |
| Key Value cleanup after the CLI commands | PASS |
| API-key revocation and workspace deletion | PASS |
| Independent post-run workspace/temp-secret audit | PASS — zero leftovers |

The verifier never executed a copied connection string. The official CLI resolved the opaque id directly, resolved the display name through its list filter, fetched bex's sensitive connection-info, and spawned `redis-cli` from the returned command. Public connection-info supplied the explicit SNI hostname required by the shared edge.

## Defects exposed and repaired by the live run

1. The datastore wildcard was incorrectly Cloudflare-proxied instead of DNS-only raw TCP.
2. Hetzner replaced the client address while PROXY protocol was disabled, so an exact source allowlist rejected the load balancer address.
3. `redis-cli` enabled TLS from `rediss://` but sent no server name; public connection-info now emits `--sni <externalHost>`.

No resource id, API key, token, password, connection URI, raw terminal transcript, source address, or kubeconfig content is retained in this evidence.
