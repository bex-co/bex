# Datastore plan previews differ across resource and catalog loading

**Severity:** minor. **Source:** continuous functional qa-find-bugs w4, a454/a465/a466, 2026-10-14; credentials consumed by the login helper. Filing only. [Original Key Value source](source-301.md), [resource receipts](../../evidence/301-keyvalue-plan-skeleton.json), [complete Key Value catalog batches](../../evidence/301-keyvalue-catalog-loading.json), [complete Postgres batches](../../evidence/m208-postgres-plan-loading.json).

## Four independently observed producer states

| Family / loading producer | Desktop pending → ready | Mobile pending → ready | Inner defect |
| --- | --- | --- | --- |
| Key Value / resource unresolved (a454) | 167→237px | 207→441px | Generic title/body block omits description, grid and footer |
| Key Value / resource ready, catalog pending (a465, two fixtures) | 167→237px | 207→441px | Empty radio group; zero placeholders; header/footer already present |
| Postgres / resource unresolved (a466) | 200→237px | 200→441px | Generic title/body block omits description, grid and footer |
| Postgres / mounted card, catalog pending (a466) | 247→237px | 463→441px | Three 80px blocks with 8px gaps replace three 70px ready cards with 12px gaps |

Both ready catalogs contain three tiers. Their plan cards are 237px at 1440×1000 and 441px at 390×844 for the exercised private Free states. Ready header/body heights are 40/139px desktop and 80/303px mobile. Key Value's empty-catalog content is 69px; Postgres's catalog placeholder content is 149/325px. Free stayed selected; Cancel/Save stayed disabled throughout. No plan selection or purchase occurred.

a465 uses two private Free Valkey fixtures; a466 uses private Free Postgres18 `qa-20261014-a466-pgplan` / `dpg-db7pg8ut41sc7388oc2g`. All response delays held successful original replies for seven seconds and fulfilled them unchanged. Postgres resource and catalog replies each contain one operation, respectively Database and DatabaseInstanceTypes. Key Value's catalog reply has four documented reads: types, networking, maxmemory and persistence. These complete request/response bodies are retained without headers; the generic scrub recorded zero redactions. The older a454 selected-operation projection keeps its documented unknown-batch limitation. Do not infer isolated Key Value catalog timing.

## Source trace and full pending-chain census

- Key Value resource: `common/components/route-skeletons.tsx:2220–2222` selects ReservedCardSkeleton at 207/167px. Postgres resource: `:2147–2149` independently selects that same generic helper at 200px. Its `:449–466` body cannot express either ready card.
- Key Value mounted catalog: `features/keyvalue/hooks/use-key-value-instance-types.ts:22–40` returns an empty list plus loading/error; `key-value-plan-section.tsx:34,66–72` ignores loading and maps that list through PlanCardGrid.
- Postgres mounted catalog: `features/databases/components/database-plan-section.tsx:104–110` creates three `h-20` skeletons in `gap-2`. Its adjacent ready branch uses PlanCardGrid's `p-3` cards and `gap-3`.
- Ready native consumer: `common/components/plan-card-grid.tsx:8–9,45–86` supplies one mobile column / three desktop columns, 12px gaps, name/price and specs lines. Actual ready rectangles measure 70px. `ui/card.tsx:6–68` supplies the 20px padding, 16px section gap and header/description typography. Both ready feature cards use the same description and bordered footer anatomy. No automatic framework sizing is assumed.
- Postgres's remaining loading chain is independently traced at `routes/databases.$databaseId.tsx:306–315`: DeferredMount reserves 200px before mount and its Suspense fallback is generic CardSkeleton(rows=3). `common/components/deferred-mount.tsx:9–32` applies minHeight only before mount and already accepts number or string. Preserve its API and unrelated callers; a responsive CSS-variable height at this one call can avoid a global wrapper change. The initial a466 unmounted empty-section receipt measured 200px and is excluded from the catalog-card claim. The JS fallback and offscreen-transition sequences require implementation verification; they are not asserted as independently delayed live failures.

Six plan-loading producers need a consistent preview/reservation: two resource branches, two catalog branches, the Postgres plan's lazy fallback and its deferred reservation. Four are live-reproduced; two are source-traced. ReservedCardSkeleton has 12 production JSX calls at filing: replace exactly the two plan-region calls and preserve the other ten, coordinating sibling changes by region rather than fragile line numbers.

DatabasePlanSection and KeyValuePlanSection each have one route mount, at database route309 and keyvalue route244. PlanCardGrid has four production consumers: those two details plus Key Value Create and Service Create. Preserve the two create consumers and their pricing/selection behavior. CreatePlanGridSkeleton has three unrelated create callers and 68px cells; copying it unchanged does not match these 70px detail cards. The Postgres catalog hook has six production consumers: create dialog, metadata card, plan section, access-control panel, disk-autoscaling control and HA panel. The Key Value hook has three: create route, metadata card and plan section. Preserve all nine consumers; no hook, tier, eligibility, pricing or transport rewrite is needed.

Canonical detail routes are `/databases/$databaseId` and `/keyvalue/$keyValueId`; legacy d/r routes redirect into these components. Metrics/Logs have their own loaders. Other sections, all service-plan loaders, permission reasons, compatibility errors and current selection semantics remain outside the layout correction.

## Concrete target

One small inert plan-preview presentation, shared without a module cycle, supplies the ready title/description, correctly sized responsive tier slots and bordered Cancel/Save footer. Use it in both resource branches and both empty-while-loading catalog branches; use matching geometry for the Postgres lazy fallback and deferred reservation. Keep hooks unconditional and keep an already loaded/cached grid visible during later reads. Preserve existing error/empty/permission and compatibility behavior. Unknown data must not become a fake resource, fake plan eligibility or actionable radio choices.

For the tested catalogs, preserve 237px desktop / 441px mobile bounds through each loading stage and the 70px / 12px grid geometry. Natural translated text wrapping stays intact. Do not compensate with outside margins or shrink ready content to a faulty placeholder. Real tier names/specs/prices remain API-driven. Coordinate with m207 for radio naming/keyboard changes without absorbing that separate interaction work.

## Viewed evidence and limitations

Key Value complete catalog card pairs are the four `.playwright-mcp/qa-a465-card-{pending,ready}-{desktop,mobile}.png` artifacts. Postgres catalog pairs are `qa-a466-pg-catalog-mounted-{pending,ready}-{desktop,mobile}.png`; Postgres resource pairs use `qa-a466-pg-resource-pending-desktop.png` / `ready-desktop.png` and `resource-pending-mobile.png` with the complete mounted-catalog ready-mobile image as the same fixture/state ready control. All eight Postgres and four Key Value crops were viewed.

The original Postgres resource ready-mobile screenshot has sticky navigation overlap, so it is excluded from the complete visual pair; its DOM/accessibility receipt remains valid. Compare within-card geometry rather than viewport position, which can change with sibling loading and element screenshot scrolling. Do not infer duplicate chrome from the two resource header/body skeleton markers. No password/connection reveal, SQL query, datastore client, write, public-access change or paid-plan variant was exercised.

## Deduplication and original acceptance disposition

The complete Key Value finding is promoted from w4/301; its historical source remains here with corrected relative links, and no active duplicate inbox note remains. Existing 298 owns metadata/Connections, 299 policies, 303 Networking and m207 card-radio interaction. Completed m196 explicitly excludes datastore plan helpers; m124's service-settings section map is separate. Target code and recent history still contain the reproduced producers on main, so this is not a fixed-but-undeployed issue. Preserve the existing native plan compatibility constraints and w4/156 prices.

w5/m79 t009's complete shape gate promised faithful datastore detail/create previews at desktop/mobile. Disposition: the 82-route manifest is unchanged and not re-audited; both canonical detail plan regions now fail by direct measurements at both widths; existing correct outer frame/navigation are preserved; primitives can implement the correction; aliases stay redirects. Other datastore sections are separately tracked; create routes, resource-id transitions, reduced motion, locale/permission/error/changed-catalog variants, lazy-JS delay, offscreen transitions and suites are not re-certified by these live measurements. Keep the original milestone archived.

## Cleanup

Postgres native deletion succeeded **15:27:41.830Z**, REST was 404 and all seven baseline inventory id sets matched **15:27:53.932Z**. Own session was revoked: owners401, cookies0, one blank page **15:33:32.912Z**; exact handles removed. Key Value cleanup is retained in source-301 and both evidence files. All fixtures are gone; no foreign resource changed.
