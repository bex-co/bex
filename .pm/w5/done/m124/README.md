# w5 · m124 — Service settings: one section list drives the page and its skeleton, and image services reuse the Deploy card

**Worker:** worker5 **Goal:** The settings page and its pending skeleton can't drift (the AGENTS.md skeleton rule), and image services get the same Deploy card (Docker Command, Pre-Deploy Command, Deploy Hook) as repo services. **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Drive the settings page from one section list — **DONE** | 45m | — |
| t002 | Make the skeleton map the same section list — **DONE** | 1h | t001 |
| t003 | Reuse the Deploy card for image services — **DONE** | 1h | t001 |
| t004 | Render parity — **DONE** | 20m | t002, t003 |
| t005 | Simplify — **DONE** | 15m | t004 |
| t006 | Test coverage — **DONE** | 30m | t004, t005 |
| t007 | Closeout — **DONE** | 10m | t006 |

## Definition of done

- For each service type × source, a test asserts the skeleton's regions equal the page's sections, and pending vs ready is verified side by side at desktop and narrow-mobile widths.
- Image services' Deploy card shows Docker Command, Pre-Deploy Command and Deploy Hook. The duplicated row, confirm step and gate in `image-command-section.tsx` are gone, and so is the standalone `!service?.repo` deploy-hook branch.
- Pre-Deploy for an image service works end to end on dev-5.

## Source + Goal linkage

- **Source:** Last-24h code review, 2026-10-05, of `739c4b217` (w4/m166) and `174fca341` (w4/m165): m166 added a skeleton region by hand, and the skeleton already disagrees with the page for image services.
- **Goal linkage:** The root AGENTS.md skeleton rule; ADR018 dashboard parity for image services.
- **Expected outcome:** No layout jump on settings navigation, and image services are configurable like Render's.
- **Why now:** Every new settings section repeats the drift, and m166 just added one.
- **Render parity included:** image-service settings UI changes.

## Evidence — 2026-10-06

**One section list (t001).** `dashboard/src/features/services/lib/settings-sections.ts`:

- `serviceSettingsSections(service)` returns the sections a service's Settings page renders, in order. A service not loaded yet gets General, Notifications and Health Checks.
- The page renders `sections.map`, one `<section id>` per entry with its content from an exhaustive switch. The navigation links the same list (all but Outbound IPs) and labels Suspend as Resume for a suspended service, with each href built from the id.
- The list now includes the Outbound IPs card. The page always rendered it, but the navigation and the skeleton never did.
- The page tests passed unchanged after the refactor (33 of 33).

**The skeleton draws the same list (t002).** `ServiceSettingsSkeleton` draws one region per section from the parent route's service, or from a representative repo web service or static site while the parent is loading.

- **Heights.** Each card reserves its ready card's exact height, measured on dev-5 at 390px and 1440px. They are keyed by what the card shows: type, plan (a paid web service's idle row is shorter), region, suspended state, Docker or native build, and image or repo source.
- **The old skeleton was wrong in both directions.** It reserved 2018px for a repo Build section that measures 1640px, and 387px for Custom Domains, which measures 611px. It drew Custom Domains, Networking, Health Checks and Maintenance for an image worker that has none of them. It also had no Port, Outbound IPs or cron sections.
- **Tests.** A type × source table renders the page and the skeleton for each of 9 shapes. It checks the page's sections against a literal expected list, the skeleton's regions against the page's sections (each region drawing its card), and the navigation's links and slots. The not-loaded list is tested too.
- **Side by side on dev-5.** The route was held pending by a temporary 6-second loader (not committed). The skeleton and the ready page were compared section by section at 390px and 1440px for 9 services: an image web, private, worker and cron service; a paid web service; a static site; repo Docker and native web services; and a git cron job, with the web, private and cron services suspended.
  - All 18 comparisons show the same sections in the same order.
  - Every height matches except one: the image worker's Source card is 20px off at 390px, because the image name wraps.
- **Screenshots**, kept locally in the gitignored `.playwright-mcp/`: `m124-image-web-390-pending.png`, `m124-image-web-390-ready.png`, `m124-image-web-1440-pending.png` and `m124-image-web-1440-ready.png`.

**One Deploy card (t003).** `components/deploy-card.tsx` is the Deploy card extracted from `BuildDeploySection`: Pre-Deploy Command, the Start, Docker or image command, Auto-Deploy when given, and the Deploy Hook.

- Image web, private and worker services use it on its own, with the image's command copy and no Auto-Deploy, since there is no git push to follow.
- `image-command-section.tsx` is deleted (its copy of the command row, confirm step and can_create gate), along with its locale key. The standalone Deploy Hook card now renders only for cron jobs, whose Deploy section is the schedule.
- **End to end on dev-5.** In the image web service's new card, saving the Pre-Deploy Command `echo m124-migrate` stamped release 2. The operator ran `predeploy-…-m124-web-gen-2`, which completed in 3s and printed `m124-migrate`. `status.preDeploy` read Succeeded for generation 2, and the release went live as Running on `rev-2`.

**Render parity (t004).** REST (`PATCH` `preDeployCommand`), MCP (`update_service`) and GraphQL (`setPreDeployCommand`) write through one service, whose `preDeployCommandApplies` refuses only cron jobs and static sites. Image services were already accepted on every surface; only the dashboard lacked the field. Render's image-backed services carry the same Deploy section (Pre-Deploy Command, Docker Command, Deploy Hook) and no Auto-Deploy. Match; nothing filed.

**`/simplify` (t005)**, three reviews. Applied:

- Both switches (the page's sections, the skeleton's regions) are exhaustive at compile time. The section and region wrappers come from the map, so an anchor cannot drift from its id.
- The parity table pins each shape's expected list, so it fails when the list itself regresses, not only when the two views disagree.
- `isDockerBuild`, `isCronType` and `isStaticSiteType` live in `service-type.ts` and replace inline checks, including `BuildDeploySection`'s two copies. The two `BuildDeploySection` calls are one call.
- The pending components pass `toServiceView(…)`, the page's own normalization. Navigation hrefs come from the section id. The plain skeleton cards are one table. Only the Source card skeleton translates.
- Gating comments moved into `serviceSettingsSections`; stale Deploy Hook comments fixed.

Skipped:

- Splitting `BuildDeploySection` so the page composes the Build card and `DeployCard` itself: it would change that component's API and its own tests for no behavior change.
- The entry chunk grows by about 0.9 KB gzip, because `route-skeletons.tsx` already sits in it; not worth moving the verification skeleton out of it.

**Tests (t006).** The parity table and the not-loaded test above; an image service's one Deploy card (Pre-Deploy, Docker Command, Deploy Hook, no Auto-Deploy, no standalone hook); and Resume for a suspended service. Each fails when its behavior is reverted (4 mutations):

- an image card without Pre-Deploy;
- the navigation ignoring `suspended`;
- the standalone hook returning for image services, which fails the three image shapes and the card test;
- a skeleton region drawing nothing, which fails the four shapes with a port.

**Gates.** Dashboard `yarn lint` (typecheck, ESLint, unused files) and `yarn test` (477 files, 4301 tests) green.
