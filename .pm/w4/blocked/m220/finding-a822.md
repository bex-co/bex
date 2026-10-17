# a822 — Blueprints list headers also stack vertically on mobile

**Severity:** minor. **Observed:** 2026-10-17 01:47–01:48 UTC. **Research main:** `57733a6dfa32a7b8c22ea1d2cdaa318ab2a9c735`. Extend m220's existing header-readability owner with a second observed route; its pending-layout work remains specific to Environment Groups. [Six complete durable receipts and exact scripts](../../evidence/m220-a822-blueprint-headers.json).

## Reproduction and controls

Fresh-load `https://dashboard.bex.co/blueprints` at390×844 and repeat at360×844. This reads the existing list only; no Blueprint detail/manifest, mutation or reveal is opened. All five headers share a180.5px row at390 and220.5px at360. The narrow **Name**, **Branch** and **Last Updated** cells split words vertically. Name occupies four distinct character lines; Branch five/six; Last Updated nine/eleven. At1440px all labels occupy one line in a36px header.

| Width | Document width | Inner container | Inner table | Header height |
| ----- | -------------- | --------------- | ----------- | ------------- |
| 390   | 390            | 356             | 356         | 180.5         |
| 360   | 360            | 326             | 347.14      | 220.5         |
| 1440  | 1440           | 894             | 894         | 36            |

At360px, a temporary browser-only `main table thead th { white-space: nowrap; }` restores all five headers to36px. Name/Branch/Last Updated widths become54.42/62.70/104.75px; table scroll width grows to485px inside the unchanged326px container, and document width remains360px. The style is removed in finally, before the ordinary desktop control. This is a diagnostic target control, not a shipped fix.

Expected: the five short table labels remain readable on one line, with normal36px header height and overflow confined to the existing table scroller. Preserve row data, links, repository truncation and status/date semantics. The screenshot also shows page-title and row-text wrapping; a local thead change does not promise to repair those separate footprints. They are outside this measured header target. No Blueprint pending-layout assertion is added.

## Cause and bounded change

`dashboard/src/routes/blueprints.tsx:127–135` renders one table and five TableHead instances without a non-wrapping constraint. The repository cell at:149 uses max-w-[200px] and truncate; other cells have no equivalent minimum label widths. At mobile widths the count-independent columns shrink around it. Actual computed th styles are white-space:normal and inherited overflow-wrap:anywhere, as in a821's separately traced native TableHead/body CSS path. Character-level Range geometry and the reversible nowrap control confirm the mechanism; no library layout behavior is inferred.

Apply a local header constraint to this route's table, alongside the separately allowlisted Environment Groups table. The bounded family is now **two ready list Table mounts and ten TableHead JSX instances**, five per route. Keep shared TableHead/Table primitives and the global body wrapping rule unchanged; other tables need their own reproduction. `/blueprints` is the canonical list route; detail/new routes are separate consumers and were not audited as ready-list aliases.

Preserve existing initial loading, empty/error presentation, Generate/New actions, navigation and m220's group-specific skeleton work. No REST/GraphQL/MCP fields or policies change. Non-English text, other browsers, empty/error cases and other tables remain implementation audit/regression work, not observed failures.

## Dedupe, comparison and cleanup

Reviewed83 actual open/blocked milestone titles, open/done searches for Blueprint table/vertical headers, current route source and targeted history. m220 already owns this local header-sizing mechanism; extend it instead of creating another issue. The current ready table still lacks a non-wrapping constraint; no landed repair is waiting for deployment. DO_NOT_DO was re-read for the parent finding in this same pass sequence and has no matching exclusion. The prior w5/m79 pending contract is not reopened for Blueprint loading on this ready-only evidence.

[Render's public Blueprint specification](https://render.com/docs/blueprint-spec), checked2026-10-17, describes configuration fields, not this authenticated list's responsive geometry. No authenticated Render visual comparator is claimed. The target follows bex's own desktop and browser-local controls.

Inspected supplementary screenshot `.playwright-mcp/qa-a822-blueprints-mobile-right.png`; matching360px and desktop screenshots exist. The durable record contains complete DOM/character geometry, exact probes and controls. Inventory equals baseline across all seven families; no console errors or HTTP failures; own session ends401, zero cookies and one about:blank page. No resources created or changed.

Add t009 (15m), enlarge the caller audit15→20m, parity10→15m and tests25→35m. Final milestone: **9tasks, 2h55**. No product fix is implemented by this filing.
