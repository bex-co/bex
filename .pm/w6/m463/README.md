# w6 · m463 — Key Value stores with internal authentication stop at exactly their plan's connection limit

**Worker:** worker6 **Goal:** a store with `spec.internalAuth` (w5/m268) holds its tenants to exactly the plan's connection limit, as every other store has since w6/m456, so a tenant at the limit reads Valkey's `ERR max number of clients reached` and the platform's reserve of 10 stays free for the readiness probe, exporter, operator and backups. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | [kv-internal-proxy counts without authenticating: a plain passthrough listener](t001.md) | 1h | — |
| t002 | [The operator keeps the proxy in front of a store with internal authentication](t002.md) | 1h30m | t001 |
| t003 | [Prove the limit and the reserve on a real store with internal authentication](t003.md) | 45m | t002 |
| t004 | [Record it in ADR021 and ADR018](t004.md) | 20m | t002 |
| t005 | [Ship and verify on production](t005.md) | 45m | t003, t004 |
| t006 | [Render parity](t006.md) | 30m | t005 |
| t007 | [Simplify](t007.md) | 15m | t006 |
| t008 | [Test coverage](t008.md) | 30m | t006 |
| t009 | [Closeout](t009.md) | 10m | t007, t008 |

## Definition of done

- On production, a free Key Value with internal authentication accepts 50 authenticated client connections on its internal URL and on its external TLS URL (from an allowlisted client), and the 51st reads `ERR max number of clients reached`, on TLS inside the handshake.
- While a tenant holds all 50, the Valkey readiness probe, the redis_exporter scrape, the operator's runtime pass and a backup snapshot all still connect (the store stays Ready, metrics keep flowing).
- Enabling internal authentication on a running store restarts no Valkey pod (restart count and `run_id` unchanged), and drops the connections that used the unauthenticated URL, as on Render.
- A test fails if an internal-auth store's `<id>` Service selects the Valkey pods, or if the store's proxy injects `AUTH`.
- ADR021 § 9 and ADR018's "Datastore connection limits by plan" row no longer list stores with internal authentication as open.

## Source + Goal linkage

- **Source:** promoted from inbox note `w6/135` on 2026-11-17, filed by w6/m456 t005 ([m456 Progress](../done/m456/README.md)) and recorded in [ADR021 § 9](../../../docs/ADR021-keyvalue-management.md#9-connection-limits-w6m456-2026-11-17) ("Not yet exact: stores with internal authentication") and ADR018's gap backlog. Render: [compute plans § Render Key Value plans](https://render.com/docs/compute-plans#render-key-value-plans) (free 50 … `50g` 50,000) and [Key Value](https://render.com/docs/key-value) ("Any existing connections that use your unauthenticated URL will break").
- **bex today:** `reconcileKeyValueInternalRoute` (`lego/operator/internal/controller/keyvalue_internal_proxy.go`) retires `kv-internal-proxy` when `spec.internalAuth` is set, and `reconcileKeyValueService` (`keyvalue_controller.go`) points `<id>` straight at the Valkey pods. Tenants then meet only `--maxclients` (the limit + the reserve of 10) on both ports, and can hold the reserve, locking the probe, exporter and operator out. At its own limit Valkey drops a TLS client before the handshake with no error (`acceptCommonHandler`, reproduced on the pinned Valkey 8 image in m456). The backup snapshot dials `<id>` for these stores (`keyValueBackupHost`, `keyvalue_backup.go`), so it would take a tenant connection once `<id>` is counted.
- **Builds on m456 (`0eed45bcb`).** Reuse its shared `kvinternalauth.Budget`, the TLS refusal written inside the store's own certificate, the scaled proxy memory limit, `--maxclients` limit + 10 and the backend Service; do not redo them. m456 itself is parked on its production deploy, and this milestone's production check (t005) needs a deploy carrying both.
- **Goal linkage:** ADR008 § Render parity first. Render holds every Key Value instance to its plan's limit, internal authentication or not; this divergence has no ADR018 sign-off.
- **Expected outcome:** enabling internal authentication no longer raises a store's tenant limit by 10 or lets a tenant starve the platform's clients; the 51st client of a free store reads Valkey's refusal on both URLs.
- **Why now:** internal authentication is a one-click control on every surface (dashboard Connections button, GraphQL, MCP, REST extension), and no production store has it yet (3 stores, all `starter`, all proxied, m456 t002). Fixing it before the first one enables it means no tenant ever sees the looser limit or a store whose probe is locked out.
- **Render parity task:** included. The limit is tenant-visible on every connection, and the enable-internal-auth confirmation on the dashboard, GraphQL, MCP and REST describes what breaks; t006 checks that nothing contradicts the shipped behavior.
