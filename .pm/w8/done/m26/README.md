# w8 · m26 — Webhook management UX + drift-proof dashboard parity

**Worker:** worker8 **Goal:** close the live dashboard's remaining validation, list, loading, and authorization gaps while pinning Render's dashboard/API vocabulary split as an explicit compatibility fixture **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Add accessible inline create and Settings validation — **DONE** | 45m | — |
| t002 | Add webhook search, latest outcome, and compact event summaries — **DONE** | 60m | — |
| t003 | Make picker loading and failures actionable — **DONE** | 45m | t001 |
| t004 | Gate mutation controls by role and present coded refusals — **DONE** | 45m | t001, t002 |
| t005 | Pin dashboard/OpenAPI event drift and automate fixture checks — **DONE** | 45m | — |
| t006 | Run authenticated admin and non-admin parity walkthroughs — **DONE** | 45m | t003, t004, t005 |
| t007 | Render parity — **DONE** | 30m | t006 |
| t008 | Simplify — **DONE** | 30m | t007 |
| t009 | Test coverage — **DONE** | 45m | t007 |
| t010 | Closeout — **DONE** | 15m | t008, t009 |

## Definition of done

Create and Settings show field-local, accessible errors for empty/duplicate names, missing events, malformed/non-HTTPS destinations, and server-coded failures; submit controls explain why they are unavailable. The webhook list has endpoint-specific search, compact event chips with overflow disclosure, and the latest immutable attempt outcome/time from m25. Event-picker loading, query failure, empty vocabulary, and retry states never silently collapse into an unusable form. Admins can create/update/toggle/delete/resend; non-admins see read-only surfaces and consistent authorization feedback without optimistic mutation affordances. A dated fixture distinguishes Render's authenticated 64-value picker from its 67-value OpenAPI enum and fails loudly on drift, while Bex continues to advertise only truthful producers and names deliberate secret/cap/product differences. Automated component/route/accessibility tests and authenticated local walkthroughs leave both workspaces free of probe webhooks.

## Source + Goal linkage

- **Source:** Authenticated Bex↔Render webhook audit on 2026-08-17. Render showed inline validation and endpoint search; Bex silently disabled incomplete create, emitted invalid-URL feedback only as a toast, exposed no webhook-specific search/latest outcome, and had code-level unconditional mutation affordances. The same walk measured 64 selectable Render dashboard event types versus 67 in current OpenAPI and 32 truthful Bex types.
- **Goal linkage:** ADR008 pillars 1 and 3 plus the human dashboard counterpart: a Render-compatible feature must be understandable and safe for both people and agents, with no dashboard-only ambiguity.
- **Expected outcome:** An operator can discover, validate, inspect, and manage webhooks with Render-familiar feedback; a read-only member cannot be tricked into doomed mutations; future Render vocabulary drift becomes a named fixture failure instead of stale parity prose.
- **Why now:** m24 and m25 correct the core contract and evidence model. Finishing UI behavior afterward avoids polishing the obsolete mutable-row API and provides the final acceptance gate for the parity program. Render parity is included as t007 because this milestone changes dashboard behavior and its GraphQL-backed permission/error consumption.
- **Anti-goal boundary:** “Parity” does not authorize persistent disks, edge-cache purge, provider maintenance/hardware lifecycle, zero-downtime redeploy controls, workflows, preview environments, retrievable secrets, or fabricated event producers. The fixed Bex endpoint cap and mint-once secret remain documented deliberate divergences.
