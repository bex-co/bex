# w3 · m11 — Outbound event webhooks (Render `/webhooks` parity)

**Worker:** worker3 **Goal:** A workspace can register a URL + event-type subscription and have bex push signed, thin-payload notifications on deploy/lifecycle/scaling events — closing bex's last inbound-only asymmetry on the webhooks row. **Status:** todo

## Tasks (in order)

| id   | title                                                                                                                                                            | est | depends_on   |
| ---- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------- | --- | ------------ |
| t001 | Data model: `webhook_endpoints` (+ `webhook_deliveries`) control-plane tables — url, secret, subscribed event types, enabled flag, workspace scope                | 45m | —            |
| t002 | Shared internal domain-event emitter wired into deploy lifecycle, suspend/resume/restart, manual scale, autoscaling changes, cron-run completion — reuse the same instrumentation points w3/m7's service-events feed introduces, don't duplicate | 1h  | t001         |
| t003 | Delivery worker: async dispatch, Standard-Webhooks HMAC-SHA256 signing, 15s timeout, bounded exponential-backoff retries, auto-disable + email via the existing SMTP courier (w4/m7) after N consecutive failures | 1h  | t002         |
| t004 | REST: CRUD for webhook endpoints (`/v1/webhooks/endpoints`) + delivery-history read                                                                              | 40m | t001         |
| t005 | GraphQL: `webhookEndpoints`/`webhookDeliveries` queries + create/update/delete mutations                                                                         | 30m | t004         |
| t006 | MCP: `list_/create_/delete_webhook_endpoint` tools (bex superset — Render's own MCP ships none)                                                                  | 30m | t004         |
| t007 | Dashboard: Settings → Integrations → Webhooks section (create form, event-type picker, one-time secret reveal, delivery-history list, enable/disable toggle)    | 1h  | t005, t006   |
| t008 | Live verification: local mock receiver, trigger a real deploy, confirm signed payload + signature validation, simulate failures through the auto-disable path   | 40m | t003, t007   |
| t009 | Render parity: check REST/GraphQL/MCP/UI shape consistency for the new webhooks surface against render.com's documented behavior, flag any drift as follow-up   | 30m | t008         |
| t010 | Simplify: run `/simplify` over the code this milestone changed                                                                                                    | 30m | t009         |
| t011 | Test coverage: meaningful tests for delivery signing, retry/backoff, auto-disable, and each surface's CRUD                                                        | 45m | t009         |
| t012 | Closeout: verify DoD, mark done, move to `w3/done/m11/`                                                                                                           | 15m | t010, t011   |

## Definition of done

A workspace admin registers a webhook URL + event subscription from the dashboard (or REST/GraphQL/MCP), triggers a deploy, and the endpoint receives a correctly HMAC-signed payload within seconds; a failing endpoint retries on schedule and auto-disables with an email notice after repeated failure — verified live, not just unit-tested.

## Source + Goal linkage

- **Source:** `/pm-brainstorm for more` 2026-07-12, following user direction to pursue the one genuinely unowned parity row (`docs/ADR018-render-parity.md` § Platform events & integrations, "Outbound event webhooks").
- **Goal linkage:** Render-parity core surface ([docs/ADR018-render-parity.md](../../../docs/ADR018-render-parity.md)); closes bex's only inbound-only asymmetry on the webhooks row.
- **Expected outcome:** third-party integrations (Slack bots, CI systems, custom dashboards) can subscribe to bex lifecycle events without polling — the parity ledger's webhooks row flips from `✖` to `✅` across REST/GraphQL/MCP/UI.
- **Why now:** the event sources it composes (deploy lifecycle `w2/m5`, SMTP courier `w4/m7`) are already shipped; sequencing after `w3/m7`'s in-flight service-events feed avoids two independent event-instrumentation passes over the same write paths. Render parity applies — the milestone lands a full REST/GraphQL/MCP/UI surface, so the standing Render-parity closing task is included, not omitted.
