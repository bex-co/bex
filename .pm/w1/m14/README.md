# w1 · m14 — Key Value (Valkey/Redis) managed store: CRD + reconciler

**Worker:** worker1 **Goal:** Close the first half of the Key Value parity gap — the platform mechanism: a `KeyValue` CR the operator reconciles into a running Valkey instance (single-instance per tier, internal service DNS, optional public TCP/SNI route, credentials in a Secret), exactly analogous to `Database`→CNPG. **Status:** done (2026-07-09)

## Tasks (in order)

| id   | title                                                                          | est | depends_on   |
| ---- | ------------------------------------------------------------------------------ | --- | ------------ |
| t001 | `KeyValue` CRD type + Valkey tier catalog in `lego/types` — **DONE** (`keyvalue_types.go`; `valkey` tier family in `tiers.yaml`/`tiers.go` with shared `validateDatastore`; codegen'd deepcopy + CRD) | 30m | —            |
| t002 | Operator reconciler: `KeyValue` → Valkey workload + Service + connection Secret — **DONE** (`keyvalue_controller.go`: tier-sized StatefulSet + headless Service + password Secret, all owner-ref'd) | 45m | t001         |
| t003 | Optional public route (Traefik TCP/SNI, mirroring `BEX_DB_DOMAIN`) — **DONE** (`BEX_KV_DOMAIN` + `IngressRouteTCP` SNI route; connection-info Secret with `uri`/`externalUri`; generalized shared `ingressRouteTCPSpec`) | 30m | t002         |
| t004 | Simplify — `/simplify` over the CRD + reconciler — **DONE** (4 agents → shared `deleteTraefikRoute` + `guaranteedResources` + `growOnlyStorage`, inlined the KV route wrapper; lint debt 19→15, all new code clean) | 20m | t003         |
| t005 | Test coverage — envtest for the `KeyValue` reconciler — **DONE** (`keyvalue_test.go`: tier→compute+storage, headless Service, stable-password Secret, owner-ref cascade, pure-fn route) | 30m | t003         |
| t006 | Closeout — **DONE** (`make test` green incl. reconciler envtest; parity ledger + `docs/keyvalue-management.md` + `BEX_KV_DOMAIN` documented; tasks → `done/`) | 10m | t005         |

## Definition of done

A `KeyValue` CR reconciles to a running Valkey instance reachable at an internal service DNS name, sized to its tier, with credentials in a Secret; optional public reach via a Traefik TCP/SNI route when the DB-domain-style config is set; `make test` green including the reconciler envtest. Surfaces — REST/GraphQL/MCP (`list_key_value`/`get_key_value`/`create_key_value`) + a dashboard page — are explicitly OUT of scope for this milestone (they are w2/w5 follow-ons, exactly as the source note scopes it).

## Source + Goal linkage

- **Source:** inbox note `w1/007` (from `/pm-brainstorm for w2` 2026-07-08), surfaced as a ✖ row in `docs/render-parity.md` (Key Value store, → w1/m14).
- **Goal linkage:** pillar 1 (Render parity) — Key Value is a first-class Render datastore product bex lacks entirely.
- **Expected outcome:** a tenant (via CR today, API later) can provision a managed Valkey the same way they provision managed Postgres.
- **Why now:** the parity audit (m13) ranked Key Value as the largest missing datastore; the mechanism must exist before any REST/GraphQL/MCP/dashboard surface can be built (the same CR-first ordering `Database` followed).
- **Render parity closing task OMITTED:** this milestone ships an operator-internal mechanism (CRD + reconciler) with no REST/GraphQL/MCP/UI surface — the Render-consistent adapters are deliberately deferred to w2/w5. Simplify + Test coverage therefore depend on the last implementation task (t003), and Closeout on Test coverage.
