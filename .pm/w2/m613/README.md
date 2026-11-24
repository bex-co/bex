# w2 · m613 — Dashboard Build Sources list, navigation, search, project visibility and create wizard

**Worker:** worker2 **Goal:** Ship `/build-sources` and `/build-sources/new` in the Dashboard along with sidebar navigation, `+ New > Build Source`, `⌘K` global search, Project Overview visibility, role-aware gates, breakpoint-matched pending skeletons (1440px and 390px), and all 8 locales. **Status:** todo

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Scaffold `dashboard/src/features/build-sources/` GraphQL documents (`buildSources`, `buildSource`, `createBuildSource`, `updateBuildSource`, `deleteBuildSource`, `builds`, `triggerBuild`) and Apollo hooks + run GraphQL codegen | 45m | — |
| t002 | Add Build Sources entry points across dashboard chrome: sidebar nav item, topbar `+ New > Build Source` (role-gated on `can_create`), `⌘K` global search results, and Project Overview Build Sources section | 55m | t001 |
| t003 | Build `/build-sources` list route with search/filter, source-kind badge (Git vs Docker image), region, linked-service count, project chip, empty state, and desktop (1440px) + narrow-mobile (390px) breakpoint-matched pending skeleton | 1h5m | t001, t002 |
| t004 | Build `/build-sources/new` create wizard supporting Git and Docker Image sources, build-time `envVars`/`secretFiles`/`envGroupIds`, optional `projectId`, initial `serviceIds` linking, and breakpoint-matched skeleton | 1h30m | t001, t002 |
| t005 | Wire `use-build-sources.ts` cache refetch/eviction, viewer/read-only role gates, and partial-link `409` (`BUILD_SOURCE_PARTIALLY_LINKED`) error UX | 35m | t003, t004 |
| t006 | Add all 8 locale dictionaries (`en`, `zh`, `de`, `es`, `ja`, `ko`, `pt`, `ru`) and Vitest + Playwright visual/skeleton checks at 1440px and 390px | 40m | t003, t004, t005 |
| t007 | Render parity | 20m | t006 |
| t008 | Simplify | 15m | t007 |
| t009 | Test coverage | 30m | t007 |
| t010 | Closeout | 10m | t008, t009 |

## Definition of done

- Signed-in workspace members can navigate to `/build-sources` from the sidebar, find Build Sources via `⌘K` global search and inside a scoped Project Overview, and view all Build Sources with source kind (Git repo/branch/runtime or Docker image), region, linked-service count, and project scope.
- Developers and Admins can create both Git-backed (including `buildCommand`, `rootDir`, `dockerfilePath`/`dockerContext`, `buildFilter`, `registryCredentialId`, build-time `envVars`/`secretFiles`/`envGroupIds`) and Image-backed (`imageUrl`, `registryCredentialId`) Build Sources via `+ New > Build Source` (`/build-sources/new`), optionally scoped to a project and linked to existing services.
- Viewers see Build Sources read-only with create buttons gated/hidden consistently with services and datastores.
- Preloading skeletons for `/build-sources` and `/build-sources/new` match their ready-state geometry side by side at desktop (`1440px`) and narrow-mobile (`390px`) widths per `dashboard/AGENTS.md` and pass the route-skeleton manifest checks.
- All 8 dashboard locales have zero missing keys; `cd dashboard && yarn lint && yarn test` passes cleanly.

## Source + Goal linkage

- **Source:** `/pm-brainstorm one aspect of render parity to w2` (2026-11-23), materialized via `/pmx w2`. Verified at `0f9166bc7f`: backend GraphQL (`lego/backend/internal/apps/build_sources_graphql.go` and `deploys/build_sources.go`) already exposes full Build Sources CRUD + builds queries, while `dashboard/src/routes/` has zero `/build-sources*` routes.
- **Goal linkage:** Render parity first ([ADR008](../../../docs/ADR008-vision.md#render-parity-first)) and [ADR097](../../../docs/ADR097-build-sources.md) — eliminates the API-only dark-feature gap for Build Sources in the Dashboard.
- **Expected outcome:** Users can discover, browse, search, and create Build Sources entirely inside the Dashboard without dropping to REST/CLI/Blueprints.
- **Why now:** All backend GraphQL CRUD queries/mutations are already live on `main`; this directly unblocks `w2/m614`'s Build Source detail workspace.
- **Render parity:** Included — adds user-facing Dashboard list, search, navigation, and create UI for Build Sources.
- **Brainstorm recommendations, not user decisions:** Because Render's Build Reuse UI is in early/private access without a public route-by-route screenshot capture, sidebar placement, list columns, and `/build-sources` + `/build-sources/new` IA follow bex's established Render-parity dashboard conventions (matching `/sandbox-groups` in `w2/m533` and `/env-groups`), while matching Render's documented `POST /v1/build-sources` field contract 1:1.
