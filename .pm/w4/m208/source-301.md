# Key Value plan loading states skip the responsive tier grid

Why: the Instance type card grows 70px on desktop and 234px on mobile because its tier grid is unreserved while either resource data or catalog data loads.

**Severity:** minor. **Estimate:** 60m. **Source:** user-requested infinite functional `$qa-find-bugs` loop, `muse.env` through the login helper, requested w4, passes a454/a465, 2026-10-14. **Goal linkage:** [ADR008](../../../docs/ADR008-vision.md) pillar 1, predictable hosting dashboard journeys; [ADR021](../../../docs/ADR021-keyvalue-management.md) managed Key Value. Filing only.

## Reproduction and observed result

Create an owned private Free Valkey 8 instance, open its Overview, then cold-navigate from Environment Groups while holding only the original HTTP response containing its KeyValue read for seven seconds. Fulfill that successful HTTP 200 response unchanged. Repeat at 1440×1000 and 390×844. This pass used `qa-20261014-a454-plan`, `red-db7n966t41sc7388o9ag`; Free remained selected and Cancel/Save remained disabled. No plan selection, plan mutation or networking change was made.

| Plan card | Pending | Ready | Expansion |
| --------- | ------: | ----: | --------: |
| Desktop   |   167px | 237px |      70px |
| Mobile    |   207px | 441px |     234px |

The pending header/body are 24/85px on desktop and 24/125px on mobile. Ready header/body are 40/139px and 80/303px respectively. Pending has one title bar and one body block, with no description, tier grid or footer. Ready has translated title/description, three tier cards in one desktop row or three mobile rows, and a bordered footer with Cancel/Save action slots. Document width remains 390px on mobile. An independent catalog query returns Free, Starter and Standard with the same specs/prices shown in the ready card; the resource read returns `plan: free`, `public: false`, `status: available`.

A previous independent fixture a441 captured the same 167→237 and 207→441 pairs while investigating w4/298. This fresh a454 fixture reproduces the bounded plan-region failure; the earlier pass is corroboration, not a second plan-focused delayed-catalog replay.

Complete unchanged-read request/response, native creation/deletion, DOM bounds, accessibility trees, catalog control and cleanup are committed in [the evidence receipt](../evidence/301-keyvalue-plan-skeleton.json). Viewed screenshots: `.playwright-mcp/qa-a454-plan-pending-desktop.png`, `qa-a454-plan-ready-desktop.png`, `qa-a454-plan-pending-mobile.png`, and `qa-a454-plan-ready-mobile-card.png`. The last image scrolls the actual content container 80px upward to show the complete ready card. Pending/ready viewport origins differ; compare card structure and height, not an alleged scroll or parent-header defect.

The read receipts retain the complete selected owned KeyValue operation/response entry. Apollo can batch reads: the handler delays the whole matching original HTTP response, while sibling operations/batch cardinality were not retained. No isolated catalog/networking timing or whole-wire-body claim is made; the finding compares within-card geometry.

## Catalog-arrival state after the resource is ready (a465)

A second producer state is now independently observed and belongs to this same issue. On two owned private Free instances, `qa-20261014-a465-catalog` / `red-db7p3qet41sc7388obv0` and `qa-20261014-a465-card` / `red-db7p8u9llfps73emud2g`, fresh-load Environment Groups to clear the page's Apollo cache, then navigate to the resource. Hold the unchanged response selected by KeyValueInstanceTypes for seven seconds. The entity heading is already present and the route-skeleton count is zero: this is the mounted plan card waiting for its catalog.

At both widths on both fixtures, the radio group has zero children and the card has zero skeleton elements. Header and Cancel/Save footer are already present. The content is only 69px high; catalog arrival adds three 70px tier cards, changing it to 139px desktop / 303px mobile. Outer bounds again change 167→237 desktop / 207→441 mobile. Free is checked after arrival, both actions remain disabled, and mobile width stays 390px. No plan was selected or saved.

The complete delayed HTTP JSON batches in [the catalog-loading receipt](../evidence/301-keyvalue-catalog-loading.json) contain exactly **KeyValueInstanceTypes, KeyValueIpAllowList, KeyValueMaxmemoryPolicy, KeyValuePersistenceMode**, all fulfilled unchanged with HTTP 200. This is not a catalog-only network delay. The card consumes the catalog, resource plan and capability state, not the held networking/policy fields. All request/response JSON bodies and the batch cardinality are retained; headers are excluded and the generic scrub recorded zero redactions. Compare within-card anatomy/height, not sibling/page movement.

Viewed full-card pairs from the second fixture: `.playwright-mcp/qa-a465-card-pending-desktop.png`, `qa-a465-card-ready-desktop.png`, `qa-a465-card-pending-mobile.png`, `qa-a465-card-ready-mobile.png`. The first fixture's desktop pending viewport image clipped the card after scrolling, so it is excluded from the cited complete pairs. Its DOM/accessibility receipt remains corroboration.

**Additional root cause:** `features/keyvalue/hooks/use-key-value-instance-types.ts:22–40` normalizes absent catalog data to `[]` and returns `loading`/error. `key-value-plan-section.tsx:34` takes only instanceTypes, then `:67–72` passes that empty array to PlanCardGrid. Its actual `instanceTypes.map` consumer creates no card children. Apollo does not supply an automatic visual placeholder. The route's generic ReservedCard and this mounted empty grid are two separate producers of the same unreserved plan content.

**Extended target:** use one inert plan-section preview for both the resource-unresolved route branch and the mounted card's `loading && instanceTypes.length === 0` branch. Preserve an already loaded/cached grid during later reads, keep all hooks unconditional, and preserve existing selected-plan/permission/error/empty-catalog behavior. Put the reusable presentation in a small shared module that both callers can import without a cycle; do not import the monolithic route-skeletons file back into the ready feature component. The ready PlanCardGrid, prices, selection and API remain unchanged. An outer-only correction leaves the catalog-stage failure intact.

This supplement updates existing w4/301 rather than filing a duplicate. The two relevant pending integrations now need verification; Postgres has its separate producer and is not live-tested by this supplement. The original a454 packet limitations stay as recorded. This new batch capture fills the catalog-arrival coverage gap at both widths but does not certify catalog errors/empty/changed-size, locale/permission variants or the implementation suites.

Both own fixtures were deleted (**14:36:44.876Z**, **14:42:39.259Z**). Both REST ids were 404 and all seven baseline inventory ID sets matched **14:42:46.769Z**. Own session revoked: owners 401, zero cookies and one blank page **14:43:15.921Z**; exact handles removed.

## Root cause, callers and concrete target

`dashboard/src/common/components/route-skeletons.tsx:2220–2222` chooses `ReservedCardSkeleton className="h-[207px] sm:h-[167px]"` for the plan region. Its implementation at `:449–466` has only a 20px title placeholder and a flexible body block. Increasing those fixed heights alone would still omit the ready anatomy.

The ready producer is `features/keyvalue/components/key-value-plan-section.tsx:54–93`: CardTitle/CardDescription, CardContent with `space-y-4`, PlanCardGrid, then `flex justify-end gap-2 border-t pt-4` and two buttons. The actual native DOM/CSS consumer is `common/components/plan-card-grid.tsx:8–9,45–86`: one-column grid below the `sm` breakpoint, three columns above, 12px gaps, and bordered/padded cards containing a name/price line and specs line. `ui/card.tsx:6–68` supplies matching Card padding, 16px section gap, 4px header gap and description text metrics. The recorded DOM confirms these structures and heights; no automatic library layout behavior is assumed.

**Target:** use one inert title/description/responsive-grid/footer preview in the Key Value Overview plan region and the mounted card's empty-while-loading catalog branch. Match the exercised three-tier catalog's 237px desktop / 441px mobile ready geometry and natural description wrapping. Preserve outer frame, responsive navigation, sibling regions, cached loaded grids and all ready plan behavior. Keep the ready catalog API-driven; do not hardcode plan names, specs, prices, eligibility or selected state into business logic. Render placeholders rather than a fabricated Key Value entity with mutation hooks. Do not enable interaction before data, clamp the ready description or add compensating margins below the card.

Census: ReservedCardSkeleton has 12 production JSX calls (seven Postgres, five Key Value). This fix changes one of them; preserve the other 11, coordinating separately filed siblings w4/298 and w4/299. KeyValueOverviewSkeleton has one production JSX consumer, `routes/keyvalue.$keyValueId.tsx:277`, selected when Overview has no resource. The ready plan section has one route mount at `:243–248`; the same pending and ready frames at `:2205–2210` / route `:217` already agree on max-width, columns and gap. Canonical URL is `/keyvalue/$keyValueId`; `routes/r.$.tsx` redirects its Render-style alias. Postgres uses DatabasePlanSection and a separate plan placeholder, and is not a demonstrated failure from this pass.

The existing CreatePlanGridSkeleton at `route-skeletons.tsx:308–317` reuses the grid class but reserves 68px cards, while these ready cards are 70px. Its three production callers are Postgres create, Key Value create and service create. Reuse its pattern only if geometry actually matches; do not globally change those three unrelated create previews to fix this detail card.

## Bounded implementation and verification

1. **Caller/variant audit, 10m:** confirm the resource-pending and catalog-pending integrations and unchanged sibling/create producers; preserve catalog loading/error/empty, permission reason, selected/unknown plan and locale behavior. Resource-pending and resource-ready/catalog-arrival states are now live-tested; the latter HTTP batch also holds three documented sibling reads.
2. **Shared local preview, 25m:** add the inert title/description/grid/footer shape using ready spacing and grid class, share it between these two branches without a module cycle, and use the hook's loading state only when no catalog is loaded. Avoid changing loaded-card or shared plan behavior.
3. **Parity and simplify, 5m:** preserve current API-driven tier choices/prices, native footer affordances and known bex/Render differences. The shape target comes from the supplied skeleton rule; no signed-in Render loading-layout comparison is claimed.
4. **Meaningful checks, 15m:** run relevant dashboard checks and pending-versus-ready DOM/visual verification at both widths, including description wrapping, cached and cold catalog arrival, English/Chinese and actual permission states. Do not write a test that merely asserts a copied class string.
5. **Live closeout, 5m:** verify matching major regions/height and unchanged Free selection/inactive Save without a purchase, remove exact owned fixtures, then move the completed note to done and sync its README row.

## Deduplication, regression disposition and cleanup

Reviewed 65 unarchived milestone READMEs; searched open, blocked and done notes/tasks against the producer/consumer and Key Value plan-loading terms; inspected target history and the 35 newest dashboard product commits. Relevant completed dry-run work (w5/m116, w5/m126, w8/046, w2/m29) concerns validation before writes, not this rendering. w9/m6 concerns stable ids/rename. Open w4/181 is exporter CPU. None owns this plan preview.

Completed [w4/m196](../done/m196/README.md) explicitly excludes Postgres/Key Value from its service-plan helper. Open [w4/m207](../m207/README.md) owns radio naming and keyboard selection, not layout. [298](../298.md) owns datastore metadata/Connections and [299](../299.md) owns Maxmemory/Persistence, explicitly excluding this plan region. The wrong fixed-height plan placeholder dates to `4581336941`; current main still contains it, so this is not an already-fixed change awaiting deployment.

[w5/m79 t009](../../w5/done/m79/done/t009.md) promised faithful datastore detail/create skeletons and stable major-region heights at both widths. Full DoD disposition: the 82-route manifest is unchanged and not re-audited; canonical Key Value detail was delayed at both widths without observed duplicate chrome or console errors; frame/navigation remain the correct family, while this inner plan shape/height fails; existing primitives can express the fix; aliases remain redirects. Postgres detail, Key Value create, resource-id transitions, reduced-motion behavior, isolated catalog-only timing and implementation suites are not re-certified. Keep m79 archived and track this correction separately.

Native deletion succeeded **12:27:02.989Z**; independent REST returned **404** and all seven baseline inventory ID sets matched **12:27:39.075Z**. Own session revoked: owners **401**, cookies **0**, one page at `about:blank`, **12:28:00.081Z**; exact session handles removed. No foreign resource changed.
