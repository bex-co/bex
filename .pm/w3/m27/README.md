# w3 · m27 — Outbound webhooks: Render's public `/webhooks` wire contract (verify-first)

**Worker:** worker3 **Goal:** A Render-SDK-shaped client can manage bex webhooks unmodified: bex serves Render's `/v1/webhooks` route family (create/list/read/full-update/delete + `/events`) with Render's field names, and the update verb exists on all three surfaces. **Status:** todo

## Tasks (in order)

| id   | title                                                                                             | est | depends_on             |
| ---- | ------------------------------------------------------------------------------------------------- | --- | ---------------------- |
| t001 | Pin the fresh spec's webhook contracts into a render-artifacts doc; decide alias-vs-move          | 40m | —                      |
| t002 | Serve Render's routes: `/v1/webhooks` CRUD with full-body PATCH (bex paths stay as aliases)       | 60m | t001                   |
| t003 | `GET /v1/webhooks/{id}/events` per the pinned schema; keep `/deliveries` as the bex superset      | 45m | t001                   |
| t004 | GraphQL `updateWebhookEndpoint` + MCP `update_webhook_endpoint` (full-update parity with REST)    | 40m | t002                   |
| t005 | Update ADR018:149/:169 + ADR006 §Outbound event webhooks; check official CLI/MCP webhook commands | 30m | t002, t003, t004       |
| t006 | Render parity                                                                                     | 30m | t005                   |
| t007 | Simplify                                                                                          | 20m | t006                   |
| t008 | Test coverage                                                                                     | 40m | t006                   |
| t009 | Closeout                                                                                          | 15m | t008                   |

## Definition of done

A wire-level test drives create → list → get → full-update (`name`/`url`/`enabled`/`eventFilter`) → disable → delete → `GET …/events` against Render's paths with Render's field names (per the pinned schemas) and passes; GraphQL and MCP expose the same full-update verb; the bex `/v1/webhooks/endpoints…` paths still work (aliases); ADR018's Outbound-event-webhooks row and § bex-ahead no longer claim Render is dashboard-only; the spec snapshot backing the contract is pinned in the repo.

## Source + Goal linkage

- **Source:** `/pm-brainstorm` round 14, 2026-07-15 — live re-fetch of `api-docs.render.com/openapi/render-public-api-1.json` (130 paths) shows a `/webhooks` family (`POST/GET /webhooks`, `GET/PATCH/DELETE /webhooks/{webhookId}`, `GET /webhooks/{webhookId}/events`; PATCH body `name`/`url`/`enabled`/`eventFilter`; POST requires `eventFilter`) that postdates bex's assessment — `docs/ADR018-render-parity.md:149,169` records "Render manages webhooks from the dashboard only" (verified 2026-07-12, now stale). bex drift verified in code: paths are `/v1/webhooks/endpoints…` (`lego/backend/internal/webhooks/rest.go:121-183`), REST PATCH is enabled-only (`rest.go:161`, ADR006:218 "PATCH(enabled)"), GraphQL/MCP have no update verb (`graphql.go:132-158`, `mcp.go:64-89` — the accepted v1 omission in `.pm/w3/done/m11/done/t002.md`-era t005), bex says `eventTypes` where Render says `eventFilter`, and bex serves `/deliveries` where Render reads `/events`.
- **Goal linkage:** Render API compatibility (docs/ADR006-bex-api.md "one core, thin adapters") — a whole public route family Render clients can now reach 404s on bex.
- **Expected outcome:** webhook management works from any Render SDK/client unmodified; the parity ledger's webhooks row is true again.
- **Why now:** the spec changed under us — every day the ledger's ✅ row overstates parity; w3 owns webhooks (m11) and its delivery machinery is untouched by this milestone.
- **Render parity:** included — REST/GraphQL/MCP surface change is the whole point.
