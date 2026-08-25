# w5 · m79 — Route-by-route dashboard preloader skeleton completion

**Worker:** worker5 **Goal:** every dashboard route that can paint pending UI reserves the final page shape with a truthful skeleton, while redirects and server-only endpoints stay instant and skeleton-free **Status:** done

## Tasks (in order)

| id | title | est | depends_on |
| --- | --- | --- | --- |
| t001 | Freeze the 82-route manifest and skeleton contract | 45m | — — **DONE** |
| t002 | Overview `/` | 30m | t001 — **DONE** |
| t003 | Agents list + detail | 45m | t002 — **DONE** |
| t004 | Billing + Notifications | 45m | t003 — **DONE** |
| t005 | Blueprints list + create + detail | 1h | t004 — **DONE** |
| t006 | Environment Groups list + detail | 45m | t005 — **DONE** |
| t007 | Webhooks list + create + detail + settings | 1h | t006 — **DONE** |
| t008 | Project overview + settings | 45m | t007 — **DONE** |
| t009 | Postgres + Key Value detail/create | 1h | t008 — **DONE** |
| t010 | Workspace create + settings | 45m | t009 — **DONE** |
| t011 | Account settings | 30m | t010 — **DONE** |
| t012 | Primary Kratos auth routes | 1h | t011 — **DONE** |
| t013 | Logout + OAuth consent/device + invite | 1h | t012 — **DONE** |
| t014 | Service create + shell/root + deploy list | 1h | t013 — **DONE** |
| t015 | Service deploy detail + Events | 45m | t014 — **DONE** |
| t016 | Service Logs + Metrics | 45m | t015 — **DONE** |
| t017 | Service Environment + Settings | 45m | t016 — **DONE** |
| t018 | Service Scaling + Plan + Disk | 1h | t017 — **DONE** |
| t019 | Service Shell + Headers + Redirects | 1h | t018 — **DONE** |
| t020 | Static-site shell + deploy routes | 1h | t019 — **DONE** |
| t021 | Static-site Events + Environment + Metrics | 1h | t020 — **DONE** |
| t022 | Static-site Headers + Redirects + Settings | 1h | t021 — **DONE** |
| t023 | Redirects, workspace alias, 404, and server-only exclusions | 45m | t022 — **DONE** |
| t024 | Live slow-navigation sweep across every rendering route | 1h | t023 — **DONE** |
| t025 | Render parity | 30m | t024 — **DONE** |
| t026 | Simplify | 30m | t025 — **DONE** |
| t027 | Test coverage | 1h | t025 — **DONE** |
| t028 | Closeout | 10m | t026, t027 — **DONE** |

## Route inventory (generated full paths)

The authoritative inventory is `dashboard/src/routeTree.gen.ts`'s `FileRoutesByFullPath`: **82 patterns** on 2026-08-24. A route is handled exactly once below. Layout/index pairs that share a browser URL remain separate because either layer can own the pending state.

- **Core/top-level (14):** `/`, `/$`, `/agents`, `/billing`, `/blueprints`, `/env-groups`, `/healthz`, `/invite`, `/login`, `/notifications`, `/register`, `/settings`, `/usage`, `/webhooks`.
- **Auth/API and first-level detail/create (23):** `/agents/$agentSessionId`, `/api/connected-agents`, `/api/sessions`, `/auth/consent`, `/auth/device`, `/auth/forgot-password`, `/auth/login`, `/auth/logout`, `/auth/reset-password`, `/auth/sign-up`, `/auth/verification`, `/blueprints/$blueprintId`, `/blueprints/new`, `/cron/$`, `/d/$`, `/databases/$databaseId`, `/env-groups/$groupId`, `/keyvalue/$keyValueId`, `/keyvalue/new`, `/new/database`, `/new/project`, `/new/redis`, `/new/workspace`.
- **Resource shells and aliases (15):** `/project/$projectId`, `/pserv/$`, `/r/$`, `/services/$serviceId`, `/services/new`, `/static/$serviceId`, `/static/new`, `/u/$`, `/w/$`, `/web/$`, `/webhook/$webhookId`, `/webhooks/new`, `/worker/$`, `/workspace/settings`, `/static/`.
- **Nested screens (30):** `/auth/device/success`, `/billing/$first/$`, `/project/$projectId/settings`, `/services/$serviceId/disk`, `/services/$serviceId/env`, `/services/$serviceId/events`, `/services/$serviceId/headers`, `/services/$serviceId/logs`, `/services/$serviceId/metrics`, `/services/$serviceId/plan`, `/services/$serviceId/redirects`, `/services/$serviceId/scaling`, `/services/$serviceId/settings`, `/services/$serviceId/shell`, `/static/$serviceId/env`, `/static/$serviceId/events`, `/static/$serviceId/headers`, `/static/$serviceId/metrics`, `/static/$serviceId/redirects`, `/static/$serviceId/settings`, `/webhook/$webhookId/settings`, `/auth/device/`, `/project/$projectId/`, `/services/$serviceId/`, `/static/$serviceId/`, `/webhook/$webhookId/`, `/services/$serviceId/deploys/$deployId`, `/static/$serviceId/deploys/$deployId`, `/services/$serviceId/deploys/`, `/static/$serviceId/deploys/`.

### Non-rendering dispositions

- **Server-only, never skeleton:** `/healthz`, `/api/sessions`, `/api/connected-agents`.
- **Immediate redirects, never skeleton:** `/login`, `/register`, `/usage`, `/billing/$first/$`, `/cron/$`, `/d/$`, `/new/database`, `/new/project`, `/new/redis`, `/pserv/$`, `/r/$`, `/static/`, `/static/new`, `/u/$`, `/web/$`, `/worker/$`. Their destination owns loading UI.
- **Data-dependent redirects/layout pairs:** `/w/$`, `/services/$serviceId`, `/services/$serviceId/`, `/static/$serviceId`, `/static/$serviceId/`, `/project/$projectId`, `/project/$projectId/`, `/webhook/$webhookId`, `/webhook/$webhookId/`, `/auth/device`, `/auth/device/`. These are explicitly covered because their loader or parent shell can visibly wait.
- **Catch-all:** `/$` gets a stable not-found render but no fake loading delay.

## Definition of done

- All 82 generated route patterns are present in a fail-closed manifest and mapped to a rendering task or an explicit no-skeleton disposition; adding a route without a disposition fails a focused test.
- Every route capable of visible pending/loading UI is exercised one route at a time under deterministic delay. Its skeleton matches the final frame, preserves dashboard/auth chrome, has no misleading resolved copy, and swaps without material layout shift, duplicate chrome, or hydration errors.
- Shape parity is evaluated against each route's actual post-loading content at the same viewport. The skeleton must match outer bounds, padding, max-width, responsive columns, heading/action slots, tabs, and all always-present major regions; it may use representative data-dependent row counts but cannot replace a different destination geometry with a generic family placeholder. Desktop and narrow-mobile pending-versus-ready evidence is required for every rendering route.
- Existing work is reused rather than duplicated: `w9/done/m69`'s list/form skeletons, `w9/done/m63`'s log/metrics/detail-tab skeletons, and route-specific skeletons already present become the baseline; change only routes whose loading state is absent, generic, misleading, or shape-mismatched.
- Redirect-only and server-only routes do not gain artificial pending UI; each lands on its canonical destination or response under the route-manifest test.
- Desktop and narrow-mobile live sweeps pass, reduced-motion behavior is respected, and dashboard typecheck, lint, and tests are green.

## Evidence

- [82-route desktop/mobile sweep and Render comparison](evidence/route-sweep.md)
- Dashboard gates: `yarn typecheck`, `yarn lint`, `yarn test` (367 files / 2,642 tests), and `yarn build` all passed on 2026-08-25.

## Source + Goal linkage

- **Source:** user request on 2026-08-24: list every route on `https://dashboard.bex.co/`, then use `/pm` to fix the preloader skeleton one route after another. Inventory cross-checked against the authenticated live sidebar/service navigation and the generated TanStack route tree.
- **Shape-contract clarification:** user requirement on 2026-08-25: every page's preloading skeleton must always match that page's post-loading content; this requirement is repeated as a blocking gate in every task rather than inferred from a shared skeleton-family name.
- **Goal linkage:** ADR008's AI-native Render-alternative vision depends on a dashboard that feels stable and trustworthy during network work; loading states are part of the human control-plane contract.
- **Expected outcome:** every navigable screen has an intentional, content-shaped loading experience, with a machine-checked route ledger preventing newly added screens from silently falling back to a spinner, blank region, or stale content.
- **Why now:** earlier milestones fixed selected lists and high-traffic detail tabs, but there is no complete route ledger and several create/auth/config routes still rely on local ad hoc loading branches or the generic fallback. The generated tree now exposes the finite boundary, so the remaining work can be completed systematically without redoing shipped coverage.
- **Render parity task:** included because this changes user-visible dashboard behavior. REST, GraphQL, and MCP shapes are untouched; comparison is UI-only and any intentional divergence is recorded rather than silently copied.
