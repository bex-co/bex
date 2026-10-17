# a821 — Two Environment Groups list layout defects

**Severity:** minor for each finding. **Observed:** 2026-10-17 01:38–01:42 UTC. **Research main:** `5448f8d9c3f30db207bc896b83985f4fdefbac42`. Filing only. [Complete durable evidence](../../evidence/m220-a821-env-groups-layout.json).

## 1. Mobile count-column labels stack vertically and inflate the header

Fresh-load `https://dashboard.bex.co/env-groups` at390×844, and repeat at360×844. No response interception is needed for these fresh controls. Horizontally scroll the existing table to the count columns. The body remains at390/360px while the inner table scrolls, but **Env Vars** occupies24.765625px and **Secret Files**24.953125px. Their visible text occupies seven and ten distinct character y positions. All five th cells become200.5px tall. The table is677.09px wide; the scroll containers are356/326px. At1440px, the ordinary successful ready control has36px headers with readable labels. Counts and metadata are correct; no API-data defect is alleged.

**Expected:** keep these short column labels on one line in the horizontally scrolling table. Preserve normal36px header height and viewport-bounded document width. Horizontal scrolling itself is intended and is not the bug.

**Cause:** `dashboard/src/routes/env-groups.tsx:208–220` renders five native TableHead instances without nowrap/minimum label width. Its Name and Environment cells have min-w-48/min-w-40 at:226/:241; Updated has whitespace-nowrap at:250, while the two count columns have only numeric content. `common/components/ui/table.tsx:5–17` correctly provides overflow-x-auto; `:66–78` renders native th with normal wrapping. `common/root-route/shell-component.tsx:37` applies inherited overflow-wrap:anywhere. Computed styles confirm normal white-space, anywhere overflow-wrap and20px line height. These opportunities let count columns shrink to near one-character content widths.

The [CSS Text specification](https://www.w3.org/TR/css-text-3/#overflow-wrap-property), checked2026-10-17, defines inherited overflow wrapping and includes anywhere breaks in min-content sizing. This matches the measured native browser behavior; no React or table-library sizing algorithm is inferred.

**Diagnostic control:** a temporary, browser-only `main table thead th { white-space: nowrap; }` at360px changes all header heights200.5→36px and count widths to72.78/93.25px. The table scroll width becomes793px, its container stays326px and document width360px. The style is removed in finally. This proves a local layout remedy can work; it is not a deployed fix or a reason to change every table.

**Fix:** make this list's five header labels non-wrapping with sufficient intrinsic column width, keeping the existing scroll container and global long-text behavior. Maintain the Name/Environment minimums, right-aligned numeric values, timestamp behavior and matching five-column order. Do not remove global wrapping or alter shared TableHead for unobserved tables.

## 2. Pending header/action and table geometry differ from the ready list

Fresh-load the New Blueprint page as a neutral client state, then use the normal global Search navigation to Environment Groups. Delay native HTTP batches containing EnvGroups only after their real responses fetch, as in the exact factory. Each valid replay retains the matched operation and its complete corresponding HTTP200 response, then fulfills the original response unchanged. The whole matching batch is delayed; sibling payloads/names are not retained, so no isolated-query claim is made. Each capture also records a later ordinary read after release. No API errors are injected.

| Width | Pending header height | Ready header height | Search y pending→ready | Pending action | Ready action |
| --- | --- | --- | --- | --- | --- |
| 1440 | 79 | 81 | 151→153 | 96×36 | 202.17×32 |
| 390 | 127 | 145 | 191→209 | 96×36 | 202.17×32 |
| 360 | 127 | 145 | 191→209 | 96×36 | 202.17×32 |

At both mobile widths the description reserves one16px bar but ready copy wraps to40px; the title reserves24px versus28px. The mobile preview compresses five equal columns into the visible width, while the ready list uses a horizontally scrolling table with different column constraints. At desktop the equal-width columns also differ from the Name/Environment/count/date layout. Both states actually have **five** columns. An early six-column count was wrong and is excluded from the findings. Representative three-row placeholders versus one loaded row are allowed by the old contract and are not filed as a row-count defect.

**Cause:** `common/components/route-skeletons.tsx:667–678` uses generic PageHeaderSkeleton; `:144–174` hard-codes h6/w48 title, h4/w72 description and h9/w24 actions. The ready `routes/env-groups.tsx:116–145` lays out the actual translated title/description and NewEnvGroupDialog trigger; that trigger is size=sm with Plus at `features/env-groups/components/new-env-group-dialog.tsx:101–106`. `EnvGroupsContentSkeleton` at:681–691 uses TableRowsSkeleton's repeat(5,minmax(0,1fr)) grid (:176–216), not the ready table's constraints.

**Fix:** make the Environment Groups pending header reserve its own translated copy, wrapping and exact button/icon footprint; existing ReservedText/ReservedButton primitives are available in route-skeletons. Establish one local five-column layout for the repaired ready table and both content-skeleton mounts, with the same max-width and inner horizontal overflow. Do not imitate the broken tall header or globally resize generic skeleton helpers.

## Blast radius, state handling and scope

Production JSX/reference search: one canonical list route; one route-pending reference to EnvGroupsListPageSkeleton at env-groups.tsx:43; two EnvGroupsContentSkeleton mounts (env-groups.tsx:158 and route-skeletons.tsx:675); one ready list Table with five TableHead instances. Private PageHeaderSkeleton has three direct JSX sites (Blueprints list593, Environment Groups673, Webhooks729). Private TableRowsSkeleton has five direct JSX sites (252,505,688,822,2075), including wrapper use; these are syntactic counts, not runtime instance counts. Keep generic helpers and other consumers unchanged. A bounded route-source search finds detail links back to /env-groups, not a separate list alias.

Preserve populated data, search/Reset, empty/no-match distinctions, read-error handling, initial content loading and client-side navigation. Initial content loading is source-traced through the second skeleton mount but was not separately forced in this pass; it is audit/test work. Preserve m219/t009's independent startup readiness behavior when implementations meet in this route. No API field, mutation, capability policy, scope resolution or global CSS contract changes.

Unverified: non-English layouts, other browsers, empty/error/no-match geometry, CSR cache combinations beyond captured global Search navigation, other TableHead consumers and every other m79 route. These are bounded regression/audit tasks, not additional observed bugs.

## Dedupe, history and previous guarantee

Re-read DO_NOT_DO; reviewed82 actual open/blocked milestone titles (excluding evidence/done); searched active/blocked/done for Environment Groups skeletons, table header wrapping and vertical columns; reviewed the latest40 product commits and targeted table/shell/list/skeleton history. Main remains current after rebase pull. No landed local header or list-pending geometry fix exists. w4/304 is scope lookup copy,305 cron run-detail geometry,298 datastore card previews; m219 is pre-handler controlled-input loss, not this route-pending or native table layout. No existing open item owns these two fixes.

Completed **w5/m79/t006** promised Environment Groups pending/ready parity. The current pending mismatch is a gap/regression against that completed contract; this pass does not identify a later introducing commit. Full old DoD review:

| Prior m79 guarantee | This pass |
| --- | --- |
| 82-route fail-closed manifest and dispositions | Not rerun; preserve existing manifest. |
| Each pending route swaps without material shift/duplicate chrome/hydration errors | Environment Groups fails header/column shape; no console errors recorded. Other routes unverified. |
| Same bounds, padding, responsive columns, heading/action slots and major regions at desktop/mobile; representative row counts allowed | This list measured at1440/390/360; action/copy/column mismatch confirmed. Outer document widths remain correct. |
| Reuse existing baseline, change only absent/mismatched shapes | Scoped list fix reuses existing primitives; generic helpers and other owners preserved. |
| Redirect/server routes remain skeleton-free and canonical | No redirect/server behavior changed or retested; bounded list-alias search only. |
| All-route sweeps, reduced motion and dashboard gates pass | No implementation in this filing. Changed-route sweeps/checks belong to t007/t008; full historical82-route matrix remains unverified. |

[Render's environment-variable documentation](https://render.com/docs/configure-environment-variables#environment-groups), checked2026-10-17, describes group/list management but does not specify these mobile widths or pending geometry. No authenticated Render visual comparator is claimed. The target follows bex's own desktop control, browser-local CSS control and explicit repository skeleton rule.

## Evidence and cleanup

Eight complete receipts, exact probe scripts and successful matched requests/responses are durable. Supplementary inspected images under .playwright-mcp: qa-a821-envgroups-{pending,ready}-{1440,390}.png and qa-a821-envgroups-mobile-right.png. Additional360px screenshots exist. The first short-wait probe is excluded; desktop V3 corrects V2's search-region-versus-bar measurement. There are no fixture mutations. Final seven-family inventory equals baseline, no console errors or HTTP failures, and own-session revocation confirms401, zero cookies and one about:blank page.

## Follow-up a822

[The Blueprint list](finding-a822.md) independently reproduces the same short-header wrapping defect. The final ready-header allowlist is two route-owned tables/ten header sites; the pending-layout finding above remains scoped to Environment Groups. See the separate dated receipts for those new observations.
