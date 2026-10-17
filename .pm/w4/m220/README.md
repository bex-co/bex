# w4 · m220 — Keep Environment Groups readable and stable while loading

**Worker:** worker4 **Goal:** the Environment Groups list has readable mobile headers and a pending layout that reserves its ready header, action and table geometry. **Status:** todo

**Estimate:** 2h20 (8 tasks). Filing only; no product fix shipped.

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Keep list column labels on readable lines | 20m | — |
| t002 | Reserve the exact translated page header and action | 20m | — |
| t003 | Give the list and its previews one column layout | 25m | t001 |
| t004 | Audit both loading callers and adjacent list states | 15m | t002, t003 |
| t005 | Check Render parity and unchanged API contracts | 10m | t004 |
| t006 | Simplify the changed code | 10m | t005 |
| t007 | Test real browser geometry and ready controls | 25m | t005 |
| t008 | Replay the live evidence and close out | 15m | t006, t007 |

## Definition of done

- Fresh-load populated `/env-groups` at390×844 and360×844, then horizontally scroll its table. **Env Vars** and **Secret Files** remain readable labels on one line; all five column headers share the ordinary36px desktop header height, rather than the captured200.5px row. Columns may scroll inside the existing table container; the document stays at the viewport width. Preserve row names, scope, counts and dates.
- Preserve the observed1440px control: all five labels are readable, ordinary header height stays36px and the content remains within its1024px maximum width.
- Repeat the exact native pending probe in [durable evidence](../evidence/m220-a821-env-groups-layout.json) at1440,390 and360px, releasing successful fetched responses unchanged. Pending and ready page-header bounds, translated title/description wrapping and New Environment Group action footprints agree at each width. The mobile search keeps its y position through the swap; currently it moves191→209px. The action reserves its ready202.17×32px footprint, rather than a96×36px block.
- The five pending table columns use the repaired ready table's minimum widths and horizontal scroll container. Do not compress the preview into five equal narrow mobile cells or reproduce the broken200px header. Representative data-dependent row counts remain allowed; matching the one-row fixture's exact height is not required.
- No resource writes are needed for replay. Inventory remains unchanged, this run's session is revoked, and dated fixed-build receipts support closure. Browser-local CSS from the hunt is a diagnostic control, not live completion evidence.

## Source + Goal linkage

- **Source:** ordinary functional `$qa-find-bugs w4`, pass a821, 2026-10-17 UTC; [two researched findings and prior-contract audit](finding.md), [exact scripts, requests/responses, DOM geometry and cleanup](../evidence/m220-a821-env-groups-layout.json). Research main `5448f8d9c3f30db207bc896b83985f4fdefbac42`.
- **Goal linkage:** ADR008's dependable Render alternative, ADR018's Environment Groups dashboard surface, and the supplied AGENTS.md requirement that pending geometry match ready content at the same breakpoint.
- **Expected outcome:** customers can read group counts on mobile and the list does not move its search or replace its column structure when loading finishes.
- **Why now:** repeated fresh360/390px loads produce vertical labels and a200px header. Three successful native-response pending replays show a distinct header/action/column mismatch. A local nowrap diagnostic restores36px headers without document overflow.
- **Scope:** the canonical Environment Groups list, its one route-pending component and the two production EnvGroupsContentSkeleton mounts. Preserve the global wrapping rule, Table primitives, the other two direct generic-header consumers enumerated in the finding, and unrelated tables. Coordinate with m219's separate search-readiness gate without duplicating its state work.
- **Render parity:** included because the visible dashboard changes. Render's public docs establish list/group management, not this exact layout or pending timing. No authenticated Render visual comparison was run; preserve API fields, error handling and data.
- **Dedupe:** reviewed82 actual open/blocked milestone titles, open/done/blocked text searches, DO_NOT_DO, current source and recent/targeted history. The pending defect is a gap in completed w5/m79's geometry contract, audited item by item in the finding. It does not claim the whole82-route guarantee was reverified. Existing m219 input readiness,304 scope labels,305 cron layout and298 datastore cards have different owners/mechanisms.
