# w3 · m27 — Outbound webhooks: Render's public `/webhooks` wire contract (verify-first)

**Worker:** worker3 **Goal:** A Render-SDK-shaped client can manage bex webhooks unmodified: bex serves Render's `/v1/webhooks` route family (create/list/read/full-update/delete + `/events`) with Render's field names, and the update verb exists on all three surfaces. **Status:** done — DONE 2026-07-15

## Tasks (in order)

| id   | title                                                                                             | est | depends_on             | status |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ---------------------- | ------ |
| t001 | Pin the fresh spec's webhook contracts into a render-artifacts doc; decide alias-vs-move          | 40m | —                      | done — DONE |
| t002 | Serve Render's routes: `/v1/webhooks` CRUD with full-body PATCH (bex paths stay as aliases)       | 60m | t001                   | done — DONE |
| t003 | `GET /v1/webhooks/{id}/events` per the pinned schema; keep `/deliveries` as the bex superset      | 45m | t001                   | done — DONE |
| t004 | GraphQL `updateWebhookEndpoint` + MCP `update_webhook_endpoint` (full-update parity with REST)    | 40m | t002                   | done — DONE |
| t005 | Update ADR018:149/:169 + ADR006 §Outbound event webhooks; check official CLI/MCP webhook commands | 30m | t002, t003, t004       | done — DONE |
| t006 | Render parity                                                                                     | 30m | t005                   | done — DONE |
| t007 | Simplify                                                                                          | 20m | t006                   | done — DONE |
| t008 | Test coverage                                                                                     | 40m | t006                   | done — DONE |
| t009 | Closeout                                                                                          | 15m | t008                   | done — DONE |

## Definition of done

A wire-level test drives create → list → get → full-update (`name`/`url`/`enabled`/`eventFilter`) → disable → delete → `GET …/events` against Render's paths with Render's field names (per the pinned schemas) and passes; GraphQL and MCP expose the same full-update verb; the bex `/v1/webhooks/endpoints…` paths still work (aliases); ADR018's Outbound-event-webhooks row and § bex-ahead no longer claim Render is dashboard-only; the spec snapshot backing the contract is pinned in the repo.

## Implementation (w3/m27, 2026-07-15)

**Render's 6 routes added** in `lego/backend/internal/webhooks/rest.go`:
`GET/POST /v1/webhooks`, `GET/PATCH/DELETE /v1/webhooks/{id}`, `GET /v1/webhooks/{id}/events`.
bex-original aliases (`/v1/webhooks/endpoints...`) kept — except the single-endpoint
`GET /v1/webhooks/endpoints/{id}` alias, which would conflict at registration time with
`GET /v1/webhooks/{id}/events` (both are 4-segment GET patterns matching `/v1/webhooks/endpoints/events`);
`GET /v1/webhooks/{id}` is now the canonical single-endpoint-read path for both Render and bex clients.

**Full-body PATCH** at `PATCH /v1/webhooks/{id}`: new `updateEndpointRequest` struct
(name/url/enabled/eventFilter), new `Update` verb in `service.go` (authorize → fetch → merge → write),
new `UpdateWebhookEndpoint` store method (full-state SQL UPDATE with `disabled_reason = CASE WHEN enabled THEN '' ...`).

**`eventFilter` field alias**: both `eventTypes` (bex) and `eventFilter` (Render) emitted in responses;
requests accept either (`eventFilter` wins if both present via `resolveEventTypes`).

**GraphQL**: `updateWebhookEndpoint` mutation (id/ownerId/name/url/eventTypes/eventFilter/enabled args).

**MCP**: `update_webhook_endpoint` tool (id/ownerId/name/url/eventTypes/enabled args).

**`fakeEndpointStore` in service_test.go** updated to implement the new `UpdateWebhookEndpoint` interface method.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 14, 2026-07-15 — live re-fetch of `api-docs.render.com/openapi/render-public-api-1.json` (130 paths) shows a `/webhooks` family that postdates bex's assessment. bex drift verified in code.
- **Goal linkage:** Render API compatibility (docs/ADR006-bex-api.md "one core, thin adapters") — a whole public route family Render clients can now reach 404s on bex.
- **Expected outcome:** webhook management works from any Render SDK/client unmodified; the parity ledger's webhooks row is true again.
- **Why now:** the spec changed under us — every day the ledger's ✅ row overstates parity; w3 owns webhooks (m11) and its delivery machinery is untouched by this milestone.
- **Render parity:** included — REST/GraphQL/MCP surface change is the whole point.
