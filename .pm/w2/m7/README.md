# w2 · m7 — Key Value API surface (REST/GraphQL/MCP, Render-shaped)

**Worker:** worker2 **Goal:** Expose the shipped w1/m14 KeyValue mechanism as a Render-compatible product surface: `/v1/key-value` REST (full CRUD + `connection-info` + `suspend`/`resume`), the dashboard-consistent GraphQL nouns, and Render's official MCP tool names (`create_key_value` / `list_key_value_instances` / `get_key_value`) — one core service, three thin adapters, same pattern as `lego/backend/internal/postgres/`. **Status:** todo

## Tasks (in order)

| id   | title                                                                                  | est | depends_on |
| ---- | -------------------------------------------------------------------------------------- | --- | ---------- |
| t001 | Capture Render's Key Value surface shapes (OpenAPI, MCP schemas, dashboard GraphQL)    | 30m | —          |
| t002 | Core `keyvalue` feature package: CRUD + connection-info over KeyValue CRs + `red-` ids | 60m | t001       |
| t003 | REST adapter — Render `/v1/key-value` endpoints + connection-info                      | 45m | t002       |
| t004 | Suspend/resume — CR field + operator scale-to-zero + surface verbs                     | 45m | t003       |
| t005 | GraphQL adapter (nouns per t001 capture)                                               | 40m | t002       |
| t006 | MCP tools — `create_key_value` / `list_key_value_instances` / `get_key_value`          | 30m | t002       |
| t007 | Render parity — REST/GraphQL/MCP consistency vs captured Render behavior               | 30m | t003, t004, t005, t006 |
| t008 | Simplify — `/simplify` over the m7 diff                                                | 20m | t007       |
| t009 | Test coverage — core verbs, adapters, connection-info, suspend/resume                  | 45m | t007       |
| t010 | Closeout                                                                               | 10m | t009       |

## Definition of done

Against a cluster with the w1/m14 operator: `POST /v1/key-value` creates a KeyValue CR that reaches Ready; `GET /v1/key-value` lists it Render-shaped; `GET /v1/key-value/{id}/connection-info` returns the 3-field contract (`internalConnectionString` `redis://…`, `externalConnectionString` when public, `cliCommand`) with the real minted password; suspend scales the Valkey to zero and resume brings it back; the same verbs work over GraphQL and MCP with identical semantics; deletion cascades. `cd lego/backend && go test ./...` and `make lint-backend` green.

## Source + Goal linkage

- **Source:** promoted inbox note `w2/003` (parked "blocked on w1/007 mechanism" — the mechanism shipped 2026-07-09 as w1/m14, verified live in prod: create → Ready → PING/SET/GET → delete cascade) + user parity report 2026-07-09 ("I don't see it on dashboard.bex.co — are you sure about parity with dashboard.render.com/new/redis?"). Surface contract already captured in the ledger's Key Value row (docs/render-parity.md): 8 REST endpoints, 3-field connection-info, plans `free`/`starter`/`standard`.
- **Goal linkage:** pillar 1 (Render parity) — Key Value is a first-class Render resource type on all four of its surfaces; bex currently has ✖/✖/✖/✖ with only the mechanism underneath. Pillar 3 (agent-native) — Render's own MCP server ships the three KV tools; agents can't manage bex KV stores until these exist.
- **Expected outcome:** an agent or API client can create, inspect, connect to, suspend/resume, and delete a managed Valkey over REST/GraphQL/MCP exactly as they would on Render; the ledger row flips ✖→✅ for REST/GraphQL/MCP.
- **Why now:** the mechanism just shipped and is proven in prod, so surfaces are no longer fake (the exact condition `w2/003` was parked on); the user has explicitly flagged the parity gap. The dashboard milestone (w5/m12) consumes this GraphQL surface — this milestone sequences first.
- **Render parity task included:** t007 — the whole milestone is Render-facing surface work.
