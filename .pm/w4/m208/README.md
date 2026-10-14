# w4 · m208 — Datastore plan previews match their ready cards

**Worker:** worker4 **Goal:** Postgres and Key Value plan cards preserve their ready title/description, responsive tier grid and footer geometry through resource, catalog and lazy loading. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Define one inert datastore plan preview from ready geometry | 30m | — |
| t002 | Use the shared preview in both Key Value loading branches | 20m | t001 |
| t003 | Align the complete Postgres plan loading chain | 35m | t001 |
| t004 | Render parity | 15m | t002, t003 |
| t005 | Simplify | 10m | t004 |
| t006 | Test coverage | 40m | t004 |
| t007 | Closeout | 10m | t005, t006 |

## Definition of done

- On owned private Free Postgres and Key Value fixtures at 1440×1000 and 390×844, delay the unchanged resource read. The plan preview preserves its title/description, three tier slots and bordered Cancel/Save footer. For the exercised catalogs it has the same 237px desktop / 441px mobile bounds as ready.
- After each resource is visible, delay its catalog response with complete batch receipts. The plan content remains structurally reserved; no empty Key Value radio group creates a 70/234px expansion and no Postgres 80px/gap8 placeholder creates a 10/22px shrink. Tier slots match the 70px ready cards and 12px gap; header/footer/text wrapping stay consistent.
- Independently verify the Postgres plan's cold lazy-module fallback and offscreen-to-mounted reservation at both widths. They use matching plan geometry without changing unrelated DeferredMount behavior.
- During later reads keep an already loaded/cached grid visible. Preserve Free selection, disabled unchanged actions, actual API prices/specs, permission reasons, Postgres compatibility messages and catalog error/empty behavior. No fake resource, plan eligibility, actionable placeholder radio or API change.
- Preserve the ten unrelated ReservedCard call sites, the two PlanCardGrid create consumers, their three create-preview callers, other detail regions and existing alias redirects. Complete locale, permission, unknown-plan, changed-catalog and reduced-motion dispositions and relevant dashboard checks.
- Reproduce the fixes live, delete exact owned fixtures, restore baseline inventory ids and revoke the own session. Close only when the source-only/unprobed states and all earlier criteria are independently verified.

## Source + Goal linkage

- **Source:** user-requested infinite functional `$qa-find-bugs w4`, muse.env only through the login helper; a454/a465/a466, 2026-10-14. Promoted from w4/301: [historical source](source-301.md), [expanded finding](finding.md), [Key Value resource](../evidence/301-keyvalue-plan-skeleton.json), [Key Value catalog](../evidence/301-keyvalue-catalog-loading.json), [Postgres](../evidence/m208-postgres-plan-loading.json). Severity minor; filing only.
- **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 1, usable hosting configuration; supplied skeleton-shape rule and w5/m79 t009's faithful datastore previews; [ADR009](../../../docs/ADR009-postgresql-management.md), [ADR021](../../../docs/ADR021-keyvalue-management.md).
- **Expected outcome:** data arrival replaces placeholders with the same plan-card anatomy rather than expanding/shrinking its grid and footer.
- **Why now:** fresh live evidence extends the original 60m Key Value note to two datastore families, four live loading branches and two remaining Postgres source-traced stages. This is one shared pending-chain fix with multiple implementation tasks, so it is promoted instead of duplicating the issue or keeping an artificially sub-hour note.
- **Scope:** dashboard preview/reservation only. [m207](../m207/README.md) owns radio naming/keyboard interaction; [298](../298.md), [299](../299.md) and [303](../303.md) own different detail regions. Source arrays, prices, eligibility, plan updates, authorization and readiness semantics are preserved.
- **Standing closing tasks:** parity preserves existing UI/API-driven behavior and conscious Render differences; simplify, meaningful tests and live closeout are included. No signed-in Render loading-layout comparison is claimed.

**Total estimate:** 2h40.
