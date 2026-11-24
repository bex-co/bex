# w2 · m614 — Dashboard Build Source detail workspace: builds, live logs, scoped trigger, linked services, env and settings

**Worker:** worker2 **Goal:** Ship `/build-sources/$buildSourceId` and `/build-sources/$buildSourceId/builds/$buildId` in the Dashboard with Builds history, live-streaming Build Detail logs (`bds-…` / `bld-…`), Render's round-9 scoped Trigger Build dialog (`autodeploy_only`/`none`/`custom` + `clearCache` + `deploys`/`skippedServices`), Linked Services management, save-only Build-time Environment, Settings, and safe Delete. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Create `/build-sources/$buildSourceId` layout route with header (source badge, region, repo/image subtitle, primary Trigger Build CTA), tab nav (`Builds`, `Services`, `Environment` [git only], `Settings`), not-found redirect, and desktop/390px breakpoint-matched skeletons | 1h | `w2/m611`, `w2/m613` |
| t002 | Build Trigger Build dialog exposing Render's round-9 `deploy.serviceScope` (`autodeploy_only`, `none`, `custom` with linked-service checkboxes) + `clearCache`, surfacing returned `deploys` and `skippedServices` | 1h | t001 |
| t003 | Build Builds tab (`/build-sources/$buildSourceId` / `builds`) listing `builds(buildSourceId)` with status badges, commit/image digest, timestamps, pagination, and live polling while active | 55m | t001 |
| t004 | Build Build Detail & Live Build Logs route (`/build-sources/$buildSourceId/builds/$buildId`) streaming `w2/m611`'s `logs(resource: bds-…, build: [bld-…])` + SSE tail, and link linked-service Deploy Detail (`deploy-detail-page.tsx`) directly here | 1h15m | t003 |
| t005 | Build Linked Services tab (`serviceLinks` + quick link/unlink actions) and save-only Build-time Environment tab (git sources only: env vars, secret files, workspace env groups via `secrets`/`envgroups` GraphQL without triggering deploys) | 1h5m | t001 |
| t006 | Build Settings tab (`updateBuildSource`: name, branch, runtime, `buildCommand`, `rootDir`, `buildFilter`, `dockerfilePath`, `dockerContext`, `registryCredential`, `imageUrl`) and safe Delete danger zone | 50m | t001 |
| t007 | Complete all 8 dashboard locales (`en`, `zh`, `de`, `es`, `ja`, `ko`, `pt`, `ru`), verify role gates (viewer read-only), and run Vitest + Playwright checks at 1440px and 390px including pending skeletons | 45m | t002, t004, t005, t006 |
| t008 | Render parity | 20m | t007 |
| t009 | Simplify | 15m | t008 |
| t010 | Test coverage | 30m | t008 |
| t011 | Closeout | 10m | t009, t010 |

## Definition of done

- Opening `/build-sources/$buildSourceId` renders a header with source metadata and primary **Trigger Build** button (git sources only, `can_create`-gated) plus tabs (`Builds`, `Services`, `Environment` [git only], `Settings`) and cleanly redirects unknown IDs back to `/build-sources`.
- **Trigger Build** exposes Render's round-9 `deploy.serviceScope` (`autodeploy_only`, `none`, `custom` with `serviceIds`) and `clearCache` (`do_not_clear` / `clear`), and clearly presents the resulting `deploys` (clickable links to each service deploy) and `skippedServices` (suspended notice).
- **Builds** lists build history with live polling while builds run, and clicking any build opens `/build-sources/$buildSourceId/builds/$buildId` with historical + live-streaming SSE build logs (`resource=bds-…`, `build=bld-…`) and fan-out deploy links; linked-service Deploy Detail (`deploy-detail-page.tsx`) links directly here.
- **Linked Services** lists all linked services with links, type/region/auto-deploy info, and quick Link/Unlink actions; **Build-time Environment** (git sources only, omitted for image sources) edits env vars, secret files, and workspace env groups save-only without triggering a deploy.
- **Settings** persists updates via `updateBuildSource` (without rebuilding), and **Delete Build Source** clearly refuses while services remain linked (showing linked services) and deletes cleanly once unlinked.
- All pending skeletons match ready states at `1440px` and `390px` (registered in route-skeleton manifest); all 8 locales have zero missing keys; `cd dashboard && yarn lint && yarn test` passes; `docs/ADR018/services-lifecycle.md` and `docs/ADR018/logs.md` reflect end-to-end Build Sources parity.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w2` (2026-11-23), materialized via `/pmx w2`. Verified at `0f9166bc7f`: backend GraphQL already ships `builds`, `build`, `triggerBuild` (round-9 `w4/m596` `deploy`/`deploys`/`skippedServices`), `updateBuildSource`, `deleteBuildSource`, `linkBuildSource`, staged `secrets` (`envScope` → `BuildSourceScope` + `WithStagedWrite`), and `envgroups` (`LinkService`/`UnlinkService` with `WithBuildSources`), with zero Dashboard detail UI yet.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) and [ADR097](../../../docs/ADR097-build-sources.md) — completes end-to-end Build Sources parity across webhooks, logs, REST, GraphQL, MCP, and Dashboard.
- **Expected outcome:** Teams can run the complete Build Reuse lifecycle — inspect builds, tail live build logs, trigger scoped builds, manage linked services, edit build-time env, and update/delete Build Sources — directly in the Dashboard.
- **Why now:** Immediately follows `w2/m611` (Build Source logs) and `w2/m613` (Build Sources list/create) right after `w4/m596` landed the round-9 scoped Trigger Build backend contract.
- **Render parity:** Included — delivers the full Dashboard operational UI for Build Sources, build logs, and round-9 scoped Trigger Build.
- **Depends on:** `w2/m611`, `w2/m613`.
- **Brainstorm recommendations, not user decisions:** Tab IA (`Builds`, `Services`, `Environment`, `Settings`) and route structure (`/build-sources/$buildSourceId` + `/build-sources/$buildSourceId/builds/$buildId`) follow bex's service/cron/sandbox-group detail patterns under ADR008 while matching Render's documented API semantics 1:1.
