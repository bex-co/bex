# w8 · m24 — Webhook event hydration: retrieve a payload's `data.id`

**Worker:** worker8 **Goal:** make every Bex webhook's thin `data.id` usable with Render's single-event retrieval contract, across REST and Bex's GraphQL/MCP extensions **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Pin the retrieve-event contract and source lookup matrix — **DONE** | 45m | — |
| t002 | Materialize an owner-scoped event-ID lookup index — **DONE** | 75m | t001 |
| t003 | Serve Render's `GET /v1/events/{eventId}` contract — **DONE** | 60m | t002 |
| t004 | Add GraphQL and MCP single-event adapters — **DONE** | 45m | t003 |
| t005 | Verify webhook `data.id` hydration and update contract docs — **DONE** | 45m | t003, t004 |
| t006 | Render parity — **DONE** | 30m | t005 |
| t007 | Simplify — **DONE** | 30m | t006 |
| t008 | Test coverage — **DONE** | 45m | t006 |
| t009 | Closeout — **DONE** | 15m | t007, t008 |

## Definition of done

For every one of Bex's 32 advertised outbound-webhook types, the `evt-…` value delivered as `webhook-id` and `data.id` resolves through an indexed, owner-scoped lookup and `GET /v1/events/{eventId}` to the exact authorized event, Render-shaped type/timestamp/service/details fields, and stable not-found behavior. A caller cannot retrieve another workspace's event. GraphQL `serviceEvent` and MCP `get_service_event` use the same core lookup and coded errors. `scripts/webhooks-verify.sh` proves create → truthful event → signed payload → retrieve-by-id against a caller-supplied public HTTPS receiver. Migration/backfill tests cover all three durable sources, missing/foreign IDs, details variants, and cross-surface parity; docs no longer instruct a Bex receiver to call a route Bex lacks.

## Source + Goal linkage

- **Source:** Authenticated Bex↔Render webhook audit on 2026-08-17, plus `docs/render-artifacts/webhooks-api.md`. Render explicitly tells receivers to hydrate the webhook's `data.id`; Bex currently mounts only `GET /v1/services/{id}/events`. This is follow-up work to `w2/done/m70`, not a replacement for its completed CRUD/history alignment.
- **Goal linkage:** ADR008 pillars 1 and 3: Render-trained webhook consumers must work without an adapter, while GraphQL and MCP expose the same core capability to agents.
- **Expected outcome:** A receiver can take any Bex webhook payload, fetch its full event by ID, and receive the same authorized facts regardless of REST, GraphQL, or MCP.
- **Why now:** The thin webhook body deliberately omits details. Without the documented hydration route, the current payload is a dead-end for automation and one of the highest-impact gaps found in the live parity walk. Render parity is included as t006 because this milestone changes REST, GraphQL, MCP, verification docs, and receiver-visible behavior.
- **Anti-goal boundary:** This milestone does not add event types, fabricate provider facts, expose secret data, or reopen any persistent-disk, edge-cache, maintenance-run, hardware, workflow, or preview-environment anti-goal.
