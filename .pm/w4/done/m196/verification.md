# m196 verification — 2026-10-11

Code: `599fb16c6` `fix(dashboard): keep plan-picker loading geometry faithful (w4/m196)`. deploy.yml run 38122951438 green before the live check.

## t001 — known-type picker loading groups (live, production)

Owned Free Web fixture `qa-20261011-m196-plan` (`srv-db5l6fqek99c73ahuq60`, busybox:1.36.1, port 3000, plan free). Fresh-load `/services/<id>/plan` with the unchanged batched response containing InstanceTypes held 4 s (batch: BillingReadiness, Workspaces, InstanceTypes, Deploys; Server was not in the held batch, so this is the picker's catalog-loading branch with a known `web_service` type).

| Width | Phase | Card h | Content h | Groups (cards × card size) | Radios | Save |
| --- | --- | --- | --- | --- | --- | --- |
| 1440 | pending | 551 | 473 | Free 1 × 108×279, Paid 6 × 108×279 | 0 | disabled |
| 1440 | ready | 551 | 473 | Free 1 × 108×279, Paid 6 × 108×279 | 7, Free checked | disabled |
| 390 | pending | 1015 | 937 | Free 1 × 108×316, Paid 6 × 108×316 | 0 | disabled |
| 390 | ready | 1015 | 937 | Free 1 × 108×316, Paid 6 × 108×316 | 7, Free checked | disabled |

The recorded 355→551 and 771→1015 expansions are gone (0 px within-card change). Card top at 390 moved 333→336 as the header's deploy badge settled, the same bootstrap-header movement the finding excluded. Document width stayed 390. No plan mutation was submitted. Screenshots: `.playwright-mcp/m196-live-{1440,390}-{pending,ready}.png`.

## t002 — shared callers and types

| Caller | Type source | Verified |
| --- | --- | --- |
| InstanceTypePicker catalog loading | `serviceType` prop (resolved service) | live Web (above); local cron/worker at 1440+390 |
| Plan route `pendingComponent` (`ServicePlanPending`) | parent loader data (`getRouteApi("/services/$serviceId").useLoaderData()`), else null | local: cron and worker client navigations painted the route frame at the same card height as the picker pending (cron 551/1015, worker 403/867) |
| Plan page server gate (`!service && loading`) | `useCachedServiceType` (cached `server(id)`, else the workspace `services` list), else null | local fresh-load with Server batched with InstanceTypes: undecided type → Paid-only shape; unit tests for the hook |
| `ServiceRouteContentSkeleton` plan case (ServiceDetailLayout parent pending) | `useCachedServiceType`, mounted only while `!service` | route-skeletons tests for web/cron/worker/private/null |

- Known type: web_service/cron_job reserve Free (1) + Paid (6); background_worker/private_service reserve Paid only, matching their ready grouping. The Free decision reuses `offersFreeTier` (the same `PAID_ONLY_SERVICE_TYPES` policy as `offeredInstanceTypes`).
- Undecided type (null): reserves only the Paid ladder and never shows a Free group. **Known limitation:** for a Free-eligible service whose type is not yet known in any cache (cold deep link before the Server read), the card grows by one Free group (label + one card row) when the type resolves. On production the Server read resolved before the held catalog in this replay, so the measured path was the known-type branch.
- Aliases (local): `/web/…/plan`, `/cron/…/plan` → canonical with Free offered; `/worker/…/plan`, `/pserv/…/plan` → canonical without Free; current plan pre-checked. `/services/<static>/plan` canonicalizes to `/static/<id>/plan` (not found, no picker), unchanged. Postgres/Key Value plan surfaces untouched.
- Live coverage is the canonical Free Web catalog branch only. Cron, worker and private, the route/parent pending branches and the aliases were verified locally (`yarn dev` + `scripts/local-bex.mjs`, 3-tier mock catalog) and in unit tests, not live. A live paid-only fixture was not created because it would incur paid compute.

## t003 — Render parity

The ready grouping follows w5/done/m7's capture (Free separated from the paid ladder, pre-selected current plan, Save disabled until the selection changes), and keeps its deliberate price and custom-instance omissions. No Render loading animation was captured, so no transient-parity claim is made. No REST, GraphQL or MCP input/output or paid-only policy changed. The diff is dashboard-only.

## t004 — Simplify

Applied: the route pending reads parent loader data instead of the cache. Cache reads are mounted only in pending branches. Types narrowed to `string | null`. `FREE_TIER_PLACEHOLDERS` is named. Caller-listing comments were trimmed. Skipped: folding the new test router helper into `renderServiceRootSkeleton` (minor).

## t005 — Tests and gates

- `instance-type-picker.test.tsx`: for web/cron/worker/private, the pending group structure (groups, heading and grid classes, card frame, text rows) equals the ready structure, with no radios and Save disabled. A separate test checks that an undecided type reserves no Free group. The pre-fix helper had no groups, so these tests fail on it.
- `route-skeletons.test.tsx`: parent-pending plan frame card counts per type, including null.
- `use-cached-service-type.test.tsx`: server read, list fallback, undecided.
- jsdom does no layout, so the pixel dimensions above come from the real browser (local and live).
- `yarn lint` passed and `yarn test` passed (491 files, 4520 tests). No Go was touched.

## Cleanup

DeleteService returned 200/true. `server(id)` then returned NOT_FOUND. `qa-login.sh --logout` printed `ok logged-out`, whoami returned 401, cookies 0, and the browser was left on about:blank. The session handle files were removed.
