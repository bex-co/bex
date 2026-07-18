# w1 · m49 — Webhook pages: `/webhooks/new` + `/webhook/$id` for Render UX parity

**Worker:** worker1 **Goal:** replace the modal-based webhook create flow with Render's page-based UX — a dedicated `/webhooks/new` create page and a per-webhook `/webhook/$id` detail page (Activity + Settings) — closing the no-edit gap the dashboard has despite the backend's fully shipped update verb, and killing a live blocking bug in the current modal. **Status:** done — **Done 2026-07-17**: whole DoD walked live on dev-1 (create via a real click at the old bug's 800px viewport, secret step, detail header, a real `deploy_started` delivery with working filters, Settings edit round-trip over `updateWebhookEndpoint`, type-to-confirm delete); 1400 dashboard tests + typecheck + lint green; mint-once secret contract kept by user decision (t006); evidence in `docs/render-artifacts/webhooks-ui.md`, ADR018 row updated; simplify residuals filed as `w1/033`.

## Tasks (in order)

| id   | title                                                                             | est | depends_on       |
| ---- | --------------------------------------------------------------------------------- | --- | ---------------- |
| t001 | Capture the Render webhook-UX artifact (`docs/render-artifacts/webhooks-ui.md`) — **DONE** | 30m | —                |
| t002 | Grouped/searchable event-picker component (labels, groups, tri-state, counter) — **DONE**  | 45m | t001             |
| t003 | `/webhooks/new` create page; retire `CreateWebhookDialog` — **DONE**                       | 45m | t002             |
| t004 | `/webhook/$id` detail page: header parity + Activity (deliveries table) — **DONE**         | 60m | t003             |
| t005 | Settings tab: edit via `updateWebhookEndpoint` + type-to-confirm delete — **DONE**         | 45m | t004             |
| t006 | Signing-secret contract decision: mint-once kept (user decision) — **DONE**                | 20m | t005             |
| t007 | ScrollArea containment sweep (the `max-h-*`-on-root misuse) — **DONE**                     | 30m | t003             |
| t008 | Render parity — cross-surface consistency + live dev-1 walk + ADR018 update — **DONE**     | 30m | t005, t006, t007 |
| t009 | Simplify — `/simplify` over the changed code (residuals → `w1/033`) — **DONE**             | 30m | t008             |
| t010 | Test coverage — picker logic, route flows, edit wiring — **DONE**                          | 45m | t008             |
| t011 | Closeout — verify DoD, sync status, move to done — **DONE**                                | 15m | t009, t010       |

## Definition of done

On dev-1 (`.pm/w1/dev-1/`), walked live in a browser:

- `https://<dashboard>/webhooks/new` renders a first-class create page (no dialog); the `/webhooks` page's "Add webhook" navigates there; the old `CreateWebhookDialog` is deleted.
- The event picker shows human-readable labels in collapsible groups with a search box, a tri-state "All events" master checkbox, group-checkbox cascade, and a live "N events selected" counter — driven by the backend `webhookEventTypes` vocabulary (unknown keys degrade to their raw key, never crash).
- Creating a webhook shows the signing secret per the t006-decided contract, then lands on `/webhook/<id>`.
- `/webhook/<id>` shows header parity (name, Enabled state, id + copy, URL + copy, event chips with show-more) and an Activity view of deliveries with All/Successful/Failed filtering; the delivery-history modal is gone.
- Settings edits name/URL/events/enabled through the **already-shipped** `updateWebhookEndpoint` mutation and saves round-trip; Delete requires typing `delete webhook <name>`.
- The Create-button hit-test bug cannot recur: no dashboard `ScrollArea` usage relies on `max-h-*` on the root for containment (sweep + regression coverage).
- `yarn typecheck && yarn lint && yarn test` green; `docs/ADR018-render-parity.md`'s webhook UI cell updated with evidence.

## Source + Goal linkage

- **Source:** user request 2026-07-17 — "learn from https://dashboard.render.com/webhooks/new, use https://dashboard.bex.co/webhooks/new instead of modal for feature parity. try the entire flow" — followed by a same-day live walk of both dashboards (screenshots in `.playwright-mcp/`: `render-webhooks-new.png`, `render-webhook-settings.png`, `bex-webhook-modal.png`, `bex-webhook-modal-overlap.png`, `bex-webhook-secret.png`) and a backend surface audit.
- **Goal linkage:** Render parity on the dashboard surface (`docs/ADR018-render-parity.md`); completes the UI half of w3/m11's outbound-webhooks feature the way w1/m45 completed route parity.
- **Expected outcome:** webhook management reaches Render's UX shape — page-based create with a humane event picker, a per-webhook page, and **edit** (today's dashboard cannot edit at all, while `PATCH /v1/webhooks/{id}`, GraphQL `updateWebhookEndpoint`, and MCP `update_webhook_endpoint` have been live since w3/m11 — the gap is purely unwired UI). The live blocking bug — the create modal's events list overflows the dialog and intercepts the Create button's clicks (`create-webhook-dialog.tsx:149`: `max-h-56` on the ScrollArea root never constrains the Radix `size-full` viewport) — dies with the modal.
- **Why now:** user-directed; the modal bug means webhook creation is broken-by-click at common viewport heights **today** (the flow was only completable via JS dispatch during the walk); the backend edit verb shipping unused is standing parity debt.
- **Render parity task included** because this is feature work on a tenant-facing UI surface; the backend surfaces (REST/GraphQL/MCP) are already aligned per the 2026-07-17 audit, so t008 focuses on the UI cell + confirming no drift was introduced.

## Non-goals

- Widening the 17-key event vocabulary toward Render's ~60 (each event needs an emitting mechanism — separate milestones own those features).
- Render's sidebar Observability / Private Links / Dedicated IPs entries (w1/m45 + ADR018 non-goals).
- Plan-tier webhook-endpoint caps (no billing system — w6 non-goal).
- A backend `createdBy`/creator field on EndpointView (display it only if the API already exposes it; otherwise record the divergence in the artifact rather than growing the API here).
