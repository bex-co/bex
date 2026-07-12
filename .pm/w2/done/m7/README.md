# w2 · m7 — Key Value API surface (REST/GraphQL/MCP, Render-shaped)

**Worker:** worker2 **Goal:** Expose the shipped w1/m14 KeyValue mechanism as a Render-compatible product surface: `/v1/key-value` REST (full CRUD + `connection-info` + `suspend`/`resume`), the dashboard-consistent GraphQL nouns, and Render's official MCP tool names (`create_key_value` / `list_key_value_instances` / `get_key_value`) — one core service, three thin adapters, same pattern as `lego/backend/internal/postgres/`. **Status:** implementation complete + green (build/test/lint); live end-to-end acceptance + closeout pending the next bex-api deploy (needs `/ship`).

## Tasks (in order)

| id   | title                                                                                 | est | depends_on             | status           |
| ---- | ------------------------------------------------------------------------------------- | --- | ---------------------- | ---------------- |
| t001 | Capture Render's Key Value surface shapes (OpenAPI, MCP schemas, dashboard GraphQL)    | 30m | —                      | **DONE** (folded) |
| t002 | Core `keyvalue` feature package: CRUD + connection-info over KeyValue CRs (name-as-id) | 60m | t001                   | **DONE**         |
| t003 | REST adapter — Render `/v1/key-value` endpoints + connection-info                      | 45m | t002                   | **DONE**         |
| t004 | Suspend/resume — CR field + operator scale-to-zero + surface verbs                     | 45m | t003                   | **DONE**         |
| t005 | GraphQL adapter (`keyValue*` nouns)                                                    | 40m | t002                   | **DONE**         |
| t006 | MCP tools — `create_key_value` / `list_key_value_instances` / `get_key_value`          | 30m | t002                   | **DONE**         |
| t007 | Render parity — REST/GraphQL/MCP consistency vs Render's contract; ledger + docs       | 30m | t003, t004, t005, t006 | **DONE**         |
| t008 | Simplify — `/simplify` over the m7 diff                                                | 20m | t007                   | **DONE**         |
| t009 | Test coverage — core verbs, adapters, connection-info, suspend/resume                  | 45m | t007                   | **DONE**         |
| t010 | Closeout — live end-to-end acceptance against a deployed bex-api, then move to `done/` | 10m | t009                   | pending deploy   |

## Implementation notes (2026-07-09)

- **Feature package** `lego/backend/internal/keyvalue/` (`service.go`, `tiers.go`, `rest.go`, `graphql.go`, `mcp.go`) — one core, three thin adapters, wired into `internal/api/server.go` (fields + `NewServer` + `features()`).
- **Id decision:** name-as-id, matching the managed-Postgres sibling (no `red-` mint). `docs/ADR020-identifiers.md` § Known deviations extended to name key-value stores alongside databases — keeps the two datastore surfaces uniform. See t002.
- **Suspend/resume:** added `spec.suspended` to the `KeyValue` CRD (regenerated CRD/deepcopy); the reconciler scales the StatefulSet to zero when suspended (PVC + Secret kept), settling Ready with a `Suspended` reason. REST `POST …/suspend|/resume` (202) + GraphQL `suspendKeyValue`/`resumeKeyValue`. MCP deliberately has no suspend/delete tools (Render's MCP server doesn't either).
- **Tests green:** `internal/keyvalue/keyvalue_test.go` (REST CRUD + create-validation + connection-info public/private + suspend/resume + GraphQL + MCP + catalog), operator `keyvalue_test.go` suspend/resume envtest. `go test ./...`, `make lint-backend` (0 issues), operator controller envtest all pass.
- **Docs:** `docs/ADR018-render-parity.md` Key Value row flipped REST/GraphQL/MCP ✖→✅ (UI stays ✖ → w5/m12); `docs/ADR006-bex-api.md` § Managed Key Value + MCP table; `docs/ADR021-keyvalue-management.md` status; `docs/ADR020-identifiers.md` deviation.
- **Live-confirmed on the prod app-cluster (2026-07-09, no deploy needed)** via the gated `TestKeyValueLiveIntegration` (`BEX_TEST_KUBECONFIG=…`, skipped in CI): the **actual bex-api Go verbs** ran against the live cluster + operator — `CreateKeyValue` (public) → operator reconcile creating→available (~34s) → `KeyValueConnectionInfo` returned both `redis://…svc:6379` (internal) and `rediss://…@m7-live.kv.bex.co:6379` (external) assembled by the real code → `DeleteKeyValue` → gone. So Create/List/Get/ConnectionInfo(internal+external)/Delete are proven end-to-end live, on top of w1/m14's create→Ready→PING/SET/GET→delete-cascade.
- **Remaining (t010) — one genuinely deploy-gated item:** **suspend/resume scale-to-zero** needs the new operator + regenerated CRD in prod (`spec.suspended`; prod runs `e6372a1` whose CRD prunes the field). The mechanism is proven by the operator envtest against a real apiserver, but the live scale-to-zero can only be confirmed after `/ship` → CI → Argo deploys the new operator. (Note: the _HTTP transport + auth gate_ layer couldn't be smoke-tested locally because the app-cluster node's **kubelet TLS is currently broken** — `kubectl port-forward`/`exec` fail with `tls: internal error`, so Hydra is unreachable from the laptop — but that layer is unit-tested, and the live integration above bypasses it by driving the verbs directly through the API server, which works.)

## Definition of done

Against a cluster with the w1/m14 operator: `POST /v1/key-value` creates a KeyValue CR that reaches Ready; `GET /v1/key-value` lists it Render-shaped; `GET /v1/key-value/{id}/connection-info` returns the 3-field contract (`internalConnectionString` `redis://…`, `externalConnectionString` when public, `cliCommand`) with the real minted password; suspend scales the Valkey to zero and resume brings it back; the same verbs work over GraphQL and MCP with identical semantics; deletion cascades. `cd lego/backend && go test ./...` and `make lint-backend` green.

## Source + Goal linkage

- **Source:** promoted inbox note `w2/003` (parked "blocked on w1/007 mechanism" — the mechanism shipped 2026-07-09 as w1/m14, verified live in prod: create → Ready → PING/SET/GET → delete cascade) + user parity report 2026-07-09 ("I don't see it on dashboard.bex.co — are you sure about parity with dashboard.render.com/new/redis?"). Surface contract already captured in the ledger's Key Value row (docs/ADR018-render-parity.md): 8 REST endpoints, 3-field connection-info, plans `free`/`starter`/`standard`.
- **Goal linkage:** pillar 1 (Render parity) — Key Value is a first-class Render resource type on all four of its surfaces; bex currently has ✖/✖/✖/✖ with only the mechanism underneath. Pillar 3 (agent-native) — Render's own MCP server ships the three KV tools; agents can't manage bex KV stores until these exist.
- **Expected outcome:** an agent or API client can create, inspect, connect to, suspend/resume, and delete a managed Valkey over REST/GraphQL/MCP exactly as they would on Render; the ledger row flips ✖→✅ for REST/GraphQL/MCP.
- **Why now:** the mechanism just shipped and is proven in prod, so surfaces are no longer fake (the exact condition `w2/003` was parked on); the user has explicitly flagged the parity gap. The dashboard milestone (w5/m12) consumes this GraphQL surface — this milestone sequences first.
- **Render parity task included:** t007 — the whole milestone is Render-facing surface work.
