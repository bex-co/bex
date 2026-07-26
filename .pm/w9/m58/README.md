# w9 · m58 — Connect-style internal-address surface: REST/GraphQL/MCP + dashboard (ADR041 D4)

**Worker:** worker9 **Goal:** every web + private service exposes its internal address (`<slug>:<port>`) the way Render does — a field on all three API surfaces matching Render's captured shape, and a dashboard Connect affordance (Internal tab; a private service's "Service Address"; the deploy-detail header) — closing the internal-address gap `docs/render-artifacts/deploy-detail-page.md` recorded as deliberate scope. **Status:** todo

## Tasks (in order)

| id   | title                                                             | est | depends_on |
| ---- | ----------------------------------------------------------------- | --- | ---------- |
| t001 | Field-shape decision from the m57 capture                         | 30m | —          |
| t002 | Internal-address field on REST + GraphQL + MCP                    | 60m | t001       |
| t003 | Dashboard Connect UX: Internal tab, Service Address, deploy header | 90m | t002       |
| t004 | Render parity: three-surface + UI consistency sweep               | 30m | t003       |
| t005 | Simplify: `/simplify` over the changed code                       | 20m | t004       |
| t006 | Test coverage: adapters + dashboard                               | 45m | t004       |
| t007 | Closeout                                                          | 15m | t005, t006 |

## Definition of done

For a live store-managed web service and private service: REST `GET /v1/services/{id}`, the GraphQL service object, and the MCP `get_service` tool all return the internal address in the field shape `docs/render-artifacts/service-addresses.md` (w9/m57 t001) captured from Render — identical values across the three surfaces; the dashboard shows a Connect-style internal address (private service labels it Service Address; the deploy-detail header shows it, superseding the deliberate-scope note in `docs/render-artifacts/deploy-detail-page.md`); `docs/ADR018-render-parity.md` carries the address rows with evidence; the value printed is the same `<slug>:<port>` that m57 proved resolvable.

## Source + Goal linkage

- **Source:** [docs/ADR041-service-addresses.md](../../../docs/ADR041-service-addresses.md) D4, decomposed on user request "achieve address parity with render.com" (2026-07-25); the discovery gap is ADR041 gap 3 (Render Connect → Internal / Service Address / deploy-header vs bex none).
- **Goal linkage:** Render parity across all surfaces (ADR018) + the deploy-experience anchor w9 owns (the deploy-detail header address was scoped out of the original page milestone, recorded in `deploy-detail-page.md`).
- **Expected outcome:** a tenant (or agent over MCP) can discover how to reach a service from a sibling without reading bex docs — same affordance Render users expect.
- **Why now:** strictly gated on w9/m57 (the address must exist and resolve before it is surfaced; the field name must copy the t001 capture). Sequencing it immediately after keeps the ADR041 context warm and avoids shipping a Render-shaped address nobody can see.
- **Render parity closing task included:** the whole milestone is user-facing surface work (REST/GraphQL/MCP/UI).

## Out of scope

- Datastore connect surfaces (Postgres/Key Value already have connection-info reveals — ADR009/ADR021).
- SSH/Shell connect tabs (w2/m39, w2/m55 own those).
- Any mechanism change (m57 owns the Services + literals).
