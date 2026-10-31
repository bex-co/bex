# Webhook list loading preview clips controls and shifts content

Why: the list's generic pending card previews different header and control geometry, so the ready search and table move after a successful read.

**Severity:** minor. Functional QA a832, 2026-10-17 UTC. Extend this milestone's loading-layout scope through t010; filing only. Probe HEAD was 7a41843a92d97a0612bf1c93f556086dae06f411; source/dedupe rechecked at 66b041df33b19cf2a6a99b0b6c082e599f6abc10.

## Reproduction and measured target

Use an owned disabled webhook with one event subscription. At 390×844 or 1440×844, start on /blueprints/new, use global Search → Webhooks, hold the real successful WebhookEndpoints HTTP response, measure the actual webhooks-list pending frame, then fulfill that response unchanged and measure the ready list. The [durable artifact](../evidence/m220-a832-webhooks-loading.json) contains the exact V2 scripts, selected complete native request/response entries, three valid replays, source census, controls and cleanup.

The handler delays the whole matching HTTP batch. Only the WebhookEndpoints request and corresponding complete response entry are retained; this is not an isolated-query delay. The V2 selector scopes measurements to the pending frame, avoiding the hidden previous-route card. The earlier unscoped zero-card sample is excluded.

| Region | 390px pending → ready | 1440px pending → ready |
| --- | --- | --- |
| Card outer x / width | 16 / 358 → same | 512 / 672 → same |
| Header height | 104 → 160px | 96 → 80px |
| Content top | 205 → 261px | 205 → 189px |
| Table top | 205 → 335px | 205 → 263px |
| Action footprint | 96×32 → 133.95×32px | 96×32 → 133.95×32px |
| Description placeholder / real copy | 16 → 140px high | 16 → 60px high |

On mobile the pending action and search placeholders reach x401, beyond the card's x374 right edge and the x390 viewport. The ready action and search end at x353. The document remains 390px wide; the screenshot shows clipped placeholders, not document-width growth. The pending header also supplies two control placeholders (one collapses to zero height on mobile), while ready CardContent contains one search field plus its help text.

A second fresh 390px V2 replay repeats these exact bounds. Both viewport screenshot pairs were inspected: .playwright-mcp/qa-a832-webhooks-{pending,ready}-{390,1440}.png. Screenshots remain ignored supplements; the committed scripts and measured receipts are the basis.

**Target:** preserve the existing ready card layout. The pending title/translated description/action use the same grid and footprints; reserve the search plus help in the corresponding CardContent position. The table preview uses the ready list's five columns and inner overflow behavior. Static header/search/table-top bounds match through the swap at each tested width. Representative data-dependent row counts remain allowed; the fixture's one row does not require a one-row placeholder or equal whole-card height.

## Located cause, consumers and scope

- dashboard/src/routes/webhooks.tsx:22 selects WebhooksListPageSkeleton; :45 mounts WebhooksPanel. Each has one production wiring site. The canonical observed route is /webhooks; detail/create/Settings are distinct components.
- common/components/route-skeletons.tsx:710–719 wraps TableCardSkeleton with controls and description. Its generic implementation at :221–257 places a fixed-width description/action row and two controls inside CardHeader, followed immediately by the table.
- features/webhooks/components/webhooks-panel.tsx:83–102 instead uses CardTitle, CardDescription and CardAction directly. Its ready search/help live in CardContent at :117–143, before the table at :146. The generic pending tree therefore cannot reserve this ready geometry.
- common/components/ui/card.tsx:20–71 gives the real CardAction a second grid column spanning both title/description rows. The pending generic div/action placeholder does not use that slot structure. The translated strings already exist and route-skeletons.tsx already imports useTranslations, so this layer can reserve real wrapped copy without changing data or APIs.
- TableCardSkeleton has **10 direct JSX sites**, enumerated in the artifact: BillingPageSkeleton; BlueprintsTableSkeleton; two BlueprintDetailContentSkeleton sites; two EnvGroupDetailContentSkeleton sites; WebhooksListPageSkeleton; WorkspaceSettingsPageSkeleton; DeploysListSkeleton; WebhookActivitySkeleton. This is a syntactic-site count, not a runtime-instance count. Replace the webhook-list use locally; preserve the other nine sites and the generic helper. No global Card/Table/wrapping change is proposed.
- WebhooksPanel's initialLoading PanelTableSkeleton is a distinct branch. Audit it and ready empty/search/error/capability states under t004; this pass proves route-pending geometry only. Endpoint metadata is workspace-scoped; no web/static/cron/worker/private/Postgres/Key Value runtime change is involved.

Ready table wrapping is an explicit excluded control: 390px header height40.5px, with Latest delivery and Enabled on two lines, versus36px desktop. It does not reproduce the pathological vertical labels from a821/a822 and adds no third ready-table target.

## Dedupe, prior contract and boundaries

Open/blocked/done distinctive searches, all84 actual open/blocked milestone titles, DO_NOT_DO, recent40 product commits and targeted TableCardSkeleton history were reviewed. There is no landed local fix for this tree. m220 already owns list-loading geometry; extend it rather than create another milestone. m219's hydration fix, 295's Webhook Settings field ordering, and298's datastore metadata/connection cards are different components and remain separate.

Completed w5/m79, specifically t007, owns four webhook URLs and its shared shape-parity gate. Its six DoD bullets were checked against this limited replay: (1) manifest infrastructure was not rerun; (2) /webhooks has visible pending UI and one shell, but geometry fails; (3) same-width desktop/mobile bounds expose the header/control mismatch; (4) the generic family placeholder needs this local replacement; (5) redirect/server-only routes were not exercised; (6) no all-route sweep, reduced-motion check or full dashboard gates were rerun. The other three webhook URLs and the full82-route contract are not re-certified. Prior route-sweep evidence labels /webhooks Pass/Pass with a local-asset navigation harness; the current production replays demonstrate the uncovered shape gap, without claiming a later regression.

[Render's webhook documentation](https://render.com/docs/webhooks) establishes workspace webhook setup, but does not specify authenticated loading CSS/timing. No signed-in Render visual comparison was made. The target comes from the repository's explicit skeleton contract. Preserve request/response fields, permissions, failures, missing-endpoint handling, delivery behavior and mint-once credentials.

**Unverified:** 360px, Chinese, other roles, reduced motion, initial SSR, empty/error content loading and the nine unchanged generic-helper consumers. They belong in scoped implementation/audit tests rather than observed DoD assertions.

## Cleanup

Own Free receiver srv-db9e4scfcf2c73c2mp40 and disabled endpoint whk-db9e52cfcf2c73c2mp60 were deleted natively. Service, its deploy, endpoint and delivery-history reads return404; all seven baseline ID sets match. The App across namespaces, serving pod and build children are gone; public URL404. Exact-session logout succeeded, whoami401, cookies0, one blank page and own handles removed. All delay handlers were removed in finally. The create receipt explicitly omits the one-time secret; no created-secret page was captured.
