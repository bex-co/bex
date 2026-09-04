# dashboard/CLAUDE.md

The bex dashboard — see [README.md](README.md) for what it is and why it exists, and its [Authentication](README.md#authentication) section before touching anything under `src/features/auth/` or `src/common/lib/ory/`.

## Directory structure

```
src/
├── routes/                # File-based routes (TanStack Router)
│   ├── index.tsx           # Services list — live bex-api `services` query + lifecycle actions (w5/m4)
│   ├── auth.{login,sign-up,forgot-password,reset-password,logout}.tsx
│   ├── setup.payment.tsx   # sign-up payment wall (ADR075 D7 rev. 2026-08-29) — bare (no chrome), verification lands here
│   ├── services.$serviceId.tsx            # service-detail shell + tab nav, with child tabs:
│   │     services.$serviceId.{index,env,logs,metrics,plan,settings}.tsx
│   ├── settings.tsx        # account settings (Kratos settings flow)
│   └── $.tsx               # catch-all 404
├── features/auth/          # login/registration/recovery/settings pages + shared page shell
├── features/onboarding/     # payment wall page + `PaymentSetupGate` (root backstop) + the pure `paymentSetupState` decision
├── features/services/       # service list + detail tabs (overview/env/plan/settings), lifecycle actions
├── features/logs/           # live logs page (query + follow) over bex-api
├── features/metrics/        # App metrics page (Render-style) — first real bex-api GraphQL client
├── features/i18n/           # language-switcher component
├── i18n/                    # i18next core: config, init, detect-language, resource aggregation
├── common/
│   ├── apollo/             # Apollo Client, pointed at bex-api's /graphql (VITE_API_URL)
│   ├── lib/ory/            # Kratos config + FrontendApi factory (@ory/client-fetch)
│   ├── lib/auth/            # requireAuth beforeLoad guard
│   ├── hooks/use-ory-flow.ts # client-side Kratos flow fetch/redirect
│   ├── hooks/use-translations.ts # i18next wrapper (see Internationalization below)
│   ├── locales/              # common.* translation keys
│   ├── components/
│   │   ├── ui/              # shadcn/Radix component kit
│   │   └── dashboard-layout/ # sidebar + header chrome wrapping authenticated pages
│   ├── providers/           # RootProvider: theme + toaster + i18next + viewport-height
│   ├── root-route/          # Root shell, 404, and error page components
│   ├── hooks/, lib/, types/  # generic utilities kept from the scaffold
├── config/                  # config.ts — VITE_API_URL / VITE_SSR_API_URL
└── test/                    # vitest setup + shared mocks
```

Add new non-auth feature code under `src/features/<name>/`, following a self-contained-module pattern (components/hooks/types per feature) rather than piling everything into `common/`.

## Code standards

- React 19+ patterns: no `FC` type, use plain function components.
- Every user-visible string goes through `useTranslations()`'s `t()`, not a hardcoded literal — see Internationalization below.
- Tests live in `__tests__` directories adjacent to the code they test, e.g. `src/common/hooks/__tests__/use-mobile.test.ts`.
- Don't hand-roll auth forms — `@ory/elements-react`'s flow components already track whatever methods/fields Kratos's config actually enables; hardcoding form shapes would drift out of sync with it.

## GraphQL (bex-api, not auth)

`codegen.ts` generates typed queries/mutations from `src/**/*.graphql` into `src/graphql/definitions.ts`, introspecting bex-api's `/graphql` (`VITE_API_URL`). Every bex-api route requires a real credential (`docs/ADR012-auth.md`) — introspection too — so `yarn codegen` needs `CODEGEN_SESSION_TOKEN` (an Ory session token) set in the environment; without it, codegen falls back to an unauthenticated request that bex-api will reject. This is unrelated to Kratos as a runtime dependency — bex-api and Kratos are separate services (`docs/ADR002-architecture.md`, `docs/ADR012-auth.md`) — the token is just how codegen authenticates _to_ bex-api.

### Offline codegen (no live bex-api / no session token)

When you have no running bex-api to introspect (the common case for a feature splice — see the hand-splice note below), regenerate from the backend's own schema dump instead of a live endpoint — two commands, no session:

```bash
cd lego/backend && SCHEMA_DUMP_PATH=/tmp/schema.json go test ./internal/api/ -run '^TestDumpGraphQLSchema$' -count=1
cd dashboard && SCHEMA_JSON=/tmp/schema.json yarn codegen   # codegen.ts prefers SCHEMA_JSON over the live endpoint
```

`TestDumpGraphQLSchema` (`internal/api/schema_dump_test.go`) writes the server's introspection JSON; `codegen.ts` reads `SCHEMA_JSON` when set. Use this to **reconcile a hand-splice**: run it, then `git diff src/graphql/definitions.ts`. An empty diff means the hand-spliced types match what codegen produces (correct); any hunk is the drift to reconcile — **splice per-feature, never wholesale-accept the regenerated file** (`definitions.ts` is partly hand-maintained; a full-codegen overwrite can lose hand-maintained parts, so keep only the reconciled feature hunk and restore the rest). Verified end to end reconciling w5/m60's cron/registry-credential splices (`w5/032`): byte-identical, zero drift.

## Visual layout pattern (w5/m2)

Reused from `beancount-dashboard`'s reports/overview page and ledger-layout chrome — reference files (read-only, not in this repo): `.../src/features/reports/overview/{index.tsx,components/overview-stat-card.tsx,components/overview-metrics-panel.tsx}`, `.../src/common/components/ledger-layout/{index.tsx,layout-header.tsx}`. Reuse our own `Card`/shadcn primitives — don't import beancount's components or CSS.

- **Section rhythm:** `space-y-6` between major page sections (the reference's own pages vary between `space-y-2 md:space-y-4` and `space-y-6 md:space-y-8`; standardize on `space-y-6` — matches this repo's existing `routes/index.tsx`).
- **Card grids:** `gap-4` — `grid grid-cols-1 gap-4 lg:grid-cols-2` for paired panels, `grid-cols-4 gap-4` for a stat-card row.
- **Stat card shape** (`overview-stat-card.tsx`, `variant="card"`): a bare `Card` with only a `CardHeader` — `CardDescription` as the label, `CardTitle` (`text-base font-semibold tabular-nums`, scaling up at container breakpoints) as the value. No `CardContent`. Use this shape for any future label+value summary tile (e.g. a services-count/status tile above `routes/index.tsx`'s sample table).
- **Header chrome:** `h-12` compact header, `px-4`, `flex items-center justify-between`, `border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60` — this repo's `dashboard-layout/index.tsx` `DashboardHeader` matches this exactly (the sidebar's header area — `p-2` + the default `h-8` workspace-switcher button — adds up to the same 48px, so the two top edges align); keep it as the baseline other pages should read as consistent with.
- **Content padding:** reference wraps routed content in `p-4 sm:p-6` (`p-2` on mobile) inside the layout's `<main>`, at `max-w-full`. This repo's `routes/index.tsx` currently uses a heavier `px-6 py-10 sm:px-10` + `mx-auto max-w-4xl` landing-page treatment — tighten toward the reference's denser `p-4 sm:p-6` when polishing (t005).
- **Typography:** card titles/headings use shadcn's default `CardTitle` sizing (no oversized hero headings inside dashboard content); reserve larger type for the auth pages' hero copy (`auth-page-shell`), not for in-app content.

Pages/components this pattern applies to (w5/m2 scope): `features/auth/pages/{login,register,forgot-password,settings,logout}-page/index.tsx`, `routes/index.tsx`, `common/components/dashboard-layout/{index.tsx,dashboard-sidebar.tsx}`.

## One rail (w5/m64)

The dashboard has exactly **one** left sidebar. A route module must never render an `<aside>` or its own rail inside `DashboardLayout` — that yields two side-by-side sidebars, which is what `/agents` shipped before w5/m64. Contextual navigation is a branch **inside** `DashboardSidebar`, in one of two flavors:

- **Replace** the nav (`ProjectSidebar`, `ServiceSidebar`) — deep hierarchical context, entered via a back link.
- **Augment** it (`AgentSessionsNavSection`) — a section-scoped working-set list beneath the global nav, Devin's shape. Scope it to its own routes; it must not follow the user elsewhere.

`DashboardLayout` takes no `sidebar` override prop, and `routes/__tests__/one-rail-invariant.test.ts` fails the build if a route module grows an `<aside>` or imports a `*-sidebar`. A genuinely needed second panel goes on the **right** — the agent-session evidence panel was the reference implementation until w5/m65 removed it, so there is currently no live example; the rule still holds for the next one. Rationale: `docs/ADR047-cloud-coding-agent-sessions.md` § D9a.

## SSR gotcha

`vite.config.ts` sets `ssr.noExternal: ["@ory/elements-react"]` — the package ships extensionless relative imports (e.g. `"./session-provider"`) that only resolve under bundler resolution, not Node's strict ESM loader. Without this, `yarn dev`/`yarn build` SSR-render any page importing `@ory/elements-react` with "Cannot find module" and silently falls back to full client rendering. Don't remove it.

## Bundle & initial-load hygiene (w9/m60)

Route code-splitting (w5/m67) only reaches per-route page bodies; the always-mounted provider tree pins shared weight into the **client entry chunk** that every first load pays. w9/m60 cut it from **1,148 → 475 kB gzip** (initial JS: entry + eager vendor chunks). Keep these invariants:

- **Icons, never barrels.** Import named lucide icons (or add to `EmptyState`'s static `ICONS` map, `common/components/empty-state.tsx`) — never `import * as Icons from "lucide-react"` + dynamic index; the namespace import defeats tree-shaking and ships all ~1,900 icons. A new `iconName` value adds a named import to that map (guarded by `empty-state.test.tsx`).
- **Locale `description` fields are dev-only.** They document strings for translators; `extractMessages` drops them at runtime and the `stripLocaleDescriptions()` Vite plugin removes them from the production bundle. Keep authoring them, but don't rely on them existing at runtime.
- **Only the default locale is eager.** `src/i18n/index.ts` bundles `DEFAULT_LANGUAGE`; other locales live in `resources-<lang>.ts` and are lazy-loaded via `loadLanguageResources` + `ensureLanguage` (`i18n/init.ts`). **Always `await ensureLanguage(lang)` before `i18n.changeLanguage(lang)`** (root `beforeLoad`, the switcher, the hydration sync) or a non-default session renders raw keys / mismatches on SSR. Tests preload zh in `src/test/setup.ts` for synchronous availability.
- **`react-intl` is lazy.** It exists only for `OryToaster` (`common/providers/ory-toaster.tsx`), mounted via `React.lazy` from `root-provider.tsx` so formatjs stays out of the entry. Ory flow toasts fire after user interaction (well after the chunk loads); don't move it back inline.
- **Vendor chunks + no public sourcemaps.** `vite.config.ts` splits React/Radix/Apollo into cache-stable `vendor-*` chunks via `manualChunks` (React and its runtime deps must stay in one chunk — a split-init cycle otherwise), and sets `build.sourcemap: false` so no readable `.map` files ship publicly. Debug prod stack traces against the committed source or a local sourcemapped build. Run `ANALYZE=1 yarn build` for a gitignored `.output/bundle-analysis.html` treemap before adding a heavy dependency.

## Navigation pending states (the white-flash fix)

**The chrome is a persistent root-level shell.** `DashboardLayout` (sidebar + header + `SidebarProvider`) is mounted exactly **once**, in `RootComponent` (`common/root-route/root-component.tsx`), **above** the router `<Outlet/>` — so navigating between sidebar items swaps only the routed content while the sidebar and header stay mounted (no remount, no re-fetch, no full-viewport blank). It is gated on a per-route `staticData: { chrome: true }` flag (typed via the `StaticDataRouteOption` augmentation in `src/router.tsx`): `RootComponent` reads `useMatches` and wraps the `<Outlet/>` in `DashboardLayout` when any active match sets it, else renders a bare `<Outlet/>` (auth, health, redirect shims, 404). Tag every **top-level** authenticated route and each detail **parent** (`services.$serviceId`, `static.$serviceId`, `project.$projectId`, `webhook.$webhookId`, …); detail children inherit through the match tree. **Do not** move `DashboardLayout` back inside a page — that reintroduces the remount + the white flash (the pending fallback would paint at the root outlet with no chrome).

A page component may still render `<DashboardLayout>{content}</DashboardLayout>` as its own wrapper: `DashboardLayout` is **nesting-aware** via `InShellContext` — when a shell is already mounted above it (the app), the inner instance is a pass-through that renders just its children; when rendered standalone (a page unit test), it renders the full shell. So the inner wrapper is a no-op in the app (single persistent shell) and gives each page test its chrome in isolation — keep it, don't strip it.

The router (`src/router.tsx`) holds the outgoing page for `defaultPendingMs: 150` and then shows `RoutePending` (`common/root-route/route-pending.tsx`), a **visible** spinner + loading title — now painted **inside** the persistent shell's content region, not the whole viewport. Never swap the default pending back to a DOM-less component and never set `defaultPendingMs`/`defaultPendingMinMs` to 0 globally: any navigation slow enough to show pending would unmount the page into a blank white document, and every fast navigation would double-mount the chrome.

**Shape parity is mandatory.** Every route-specific pending skeleton must be a structural preview of that route's post-loading content at the same viewport—not merely a generic indicator from the same broad family. Match the ready state's outer container, padding, max-width, responsive columns, heading and action slots, tabs/navigation, and every always-present major region such as cards, tables, form groups, charts, timelines, log panels, or terminals. Reserve the same stable heights so the swap does not materially shift the page. Data-dependent row counts may use a representative bounded count, but a skeleton must not invent a region the ready page lacks, omit an always-present region, expose resolved or sensitive copy, or enable an action before its deciding data exists. Reuse a shared skeleton only when those structures actually match. For every skeleton change, force the route to remain pending and compare pending versus ready at desktop and narrow-mobile widths; tests or evidence must fail if the structural mapping regresses.

- Resource-detail routes (`services.$serviceId`, `static.$serviceId`, `databases.$databaseId`, `keyvalue.$keyValueId`, `blueprints.$blueprintId`, `webhook.$webhookId`, `env-groups_.$groupId`) reuse their **`component` as `pendingComponent`** with a route-level `pendingMs: 0` — the page frame (chrome + header + skeleton stack) doubles as its own pending state during the blocking title loader. This works because those components tolerate an absent `Route.useLoaderData()` (only `useLoaderErrorRetry` reads it, and it no-ops on `undefined`). A component that dereferences its loader data or renders `<Outlet/>` unconditionally (see `project.$projectId`) needs a dedicated pending component instead.
- The component-as-pending pattern requires `component` + `pendingComponent` to share one code-split chunk: `codeSplittingOptions.defaultBehavior` groups them in **both** `vite.config.ts` (`tanstackStart({ router: ... })`) and `vitest.config.ts` (`tanstackRouter(...)`). If the groupings drift apart or are removed, the splitter strips the shared identifier out of the reference file and the route module throws `ReferenceError` at import.
- **Top-level LIST/CREATE routes use a route-shaped `pendingComponent`** from `common/components/route-skeletons.tsx`, rather than falling back to the bare `RoutePending` spinner. The older `ListPageSkeleton` / `FormPageSkeleton` helpers in `detail-skeletons.tsx` remain valid only for a destination whose ready geometry actually matches them; a route-family label is not enough. These pending components mount only when navigation exceeds `defaultPendingMs` (a prefetched/cached navigation skips them). Redirect-only routes (`static.index`, `static.new`) have no component or pending state, so they get none.
- The detail-route `pendingMs: 0` + component-as-pending config was **audited against the persistent shell (w9/m69) and retained**: rendering the detail frame's skeleton at 0ms is still the right behavior (the shell persists, but the frame's header/tabs skeleton should appear immediately, not after `defaultPendingMs`). Don't remove it, and don't cargo-cult `pendingMs: 0` onto a list route — a list route wants its default-delay, destination-shaped skeleton, not an instant one.
- Title loaders pass `cause` to `titleLoaderFetchPolicy` (`common/lib/document-head`): `network-only` on entry/preload, `cache-first` on retained-match re-runs (`stay` — tab switches, search-param changes, the loader-error retry) so tab clicks don't refire the title query.
- Links to a service must target its canonical base (`serviceBaseForType`: `static_site` → `/static/<id>`, else `/services/<id>` — see `ResourceLink`, global search). Both detail **parents** (`services.$serviceId`, `static.$serviceId`) call `loadServiceDetail`, which canonicalizes the base (with subpath via `redirectPreservingSuffix`) before render — so a static site hit at `/services/<id>/<subpath>` still lands, under `/static/...`. Prefer the canonical link up front to skip that bounce (loader RTT + chunk).

## Sign-up payment wall (ADR075 D7, revised 2026-08-29)

Hosted bex requires a bound payment method before any resource use (`BEX_REQUIRE_PAYMENT_METHOD=all`). The dashboard collects it as the last onboarding step, not by intercepting the first create:

- **`/setup/payment`** (`routes/setup.payment.tsx` → `features/onboarding/pages/payment-setup-page`) is a **bare** route in the auth-page shell — verification success navigates there with the guarded `next`, same-tab Stripe Checkout returns there (`useBillingOnboarding({ returnPath })`), and it forwards to `next` the moment readiness says the gate is open. Exits: "Self-host bex instead" (GitHub) and sign out.
- **`PaymentSetupGate`** (`features/onboarding/components/`) wraps the `<Outlet/>` inside the persistent shell in `RootComponent` and redirects a still-refused workspace's billing manager to the wall from **any chrome route**. It is fail-open: only the server's definitive `paymentMethodOnboardingRequired: true` moves anyone; loading/errored/`canManageBilling=false` reads render the page (the API's 402 + `PaymentRequiredProvider` dialog remain the backstop).
- **Never re-derive the gate client-side.** `paymentMethodOnboardingRequired` is computed in bex-api from the same `PaymentGate` call a create runs (true only in `all` mode and only for a workspace that is not bound/excluded/comped). Don't wall on `paymentMethodReady` or lifecycle — an excluded first-party workspace is unbound yet allowed.

## Internationalization (i18n)

i18next + react-i18next (w5/m3), modeled on `beancount-dashboard`'s `docs/i18n.md` but adapted for TanStack Start SSR (no custom `entry-server.tsx` here — detection happens in the root route's `beforeLoad`).

- **Feature-based locales:** every feature that owns user-visible strings has `locales/{en,zh}.ts` (`src/common/locales/`, `src/features/{auth,metrics,services}/locales/`) exporting `Record<string, TranslationEntry>` where `TranslationEntry = { message: string; description: string }`. `src/i18n/index.ts` imports every namespace, flattens each via `extractMessages`, and merges them into per-language `{ key: message }` resources (`en`, `zh`) fed to i18next.
- **Namespaced keys:** every key is prefixed by feature (`common.*`, `auth.*`, `metrics.*`, `services.*`). `useTranslations()` (`src/common/hooks/use-translations.ts`) wraps `useTranslation` and in dev logs a console error on an unprefixed key and a warning on a key missing from the `en` resources — this is how a typo'd key gets caught instead of silently rendering itself.
- **Interpolation** uses single braces (`t("metrics.responseTimes", { quantile })` → `"Response Times ({quantile})"` in the locale file) — `src/i18n/init.ts` sets `interpolation: { prefix: "{", suffix: "}" }` to match; don't switch a locale file to i18next's default `{{...}}` without also changing that config.
- **Pluralization** uses i18next's native `_one`/`_other` suffixed keys with `t("ns.key", { count })` and a **numeric** `count` — no hand-rolled One/Many key pairs, ternaries, or `"(s)"`. Author `"ns.key_one"` + `"ns.key_other"` in `en.ts`; `zh.ts` gets **only** `"ns.key_other"` (Chinese has a single cardinal plural category — `locale-parity.test.ts` enforces both shapes, and `src/i18n/__tests__/plurals.test.ts` is the reference to copy). A sentence with two counts is two pluralized keys composed at the call site (see `service-environment-editor.tsx`'s unsaved-changes bar); a count needing thousands separators passes a second display param (see `metrics.requestsCount`'s `{formatted}`).
- **SSR language detection + persistence** (`src/i18n/detect-language/{server,client}.ts`, isomorphic via `detectLanguage` in `index.ts`): priority is URL (`?lang=`/`?locale=`) → cookie (`i18nextLng`) → `Accept-Language` header (server) / SSR-stamped `<html lang>` (client) → `DEFAULT_LANGUAGE`. The client's `<html lang>` fallback is what keeps the hydration pass in agreement when the server chose a language via `Accept-Language` that the client cannot read — without it a first-time `Accept-Language: zh` visitor hydrates an English render over a Chinese document (whole-tree React #418). The root route's `beforeLoad` (`src/routes/__root.tsx`) calls `detectLanguage()` and `await i18n.changeLanguage(...)` **before** the route component renders, on both the server and the client's initial hydration pass, so the first render always agrees — no `LanguageDetector` plugin (it reads localStorage before hydration and would cause a React hydration mismatch). `persistLanguage()` (`src/i18n/utils.ts`) writes both a 1-year cookie and localStorage; `useLanguageHydrationSync()` (mounted once in `RootComponent`) applies a post-hydration override from `?lang=` or a stale localStorage preference, never during the initial render.
- **Switcher:** `src/features/i18n/language-switcher.tsx` — self-contained (no dashboard-chrome dependency), calls `i18n.changeLanguage(lang)` + `persistLanguage(lang)`. Mounted in `AuthPageShell` (unauthenticated chrome); in authenticated chrome, language selection lives in `UserNav`'s avatar dropdown alongside the theme submenu.
- **Ory Elements** doesn't inherit react-i18next — `@ory/elements-react`'s flow components take their own `intl.locale`. `useOryConfig()` (`src/common/lib/ory/config.ts`) returns `oryConfig` with `intl.locale` set to the current `i18n.language`; every auth page (`login`, `register`, `forgot-password`, `settings`) calls this instead of importing the static `oryConfig` directly, so the Ory-rendered form follows the same language as the rest of the app, including on the SSR render.
- **Adding a language:** add the code to `SUPPORTED_LANGUAGES`/`LANGUAGE_NAMES` (`src/i18n/config.ts`), add a sibling `<lang>.ts` next to every `en.ts` locale file, then register its imports in `src/i18n/index.ts`'s aggregation (`en`/`zh` exports + `resources`). Ory Elements already ships translations for most locale codes (`OryLocales` in `@ory/elements-react`) — no extra wiring needed there beyond the locale code matching.
- **Adding a feature's own strings:** create `src/features/<name>/locales/{en,zh}.ts`, prefix every key with the feature name, and import+merge it into both `en` and `zh` in `src/i18n/index.ts`.

## Development commands

```bash
yarn dev                # Start Vite dev server (expects VITE_API_URL to point at a real bex-api)
yarn local-bex          # Local bex-api + Kratos dev stub on :8099 (scripts/local-bex.mjs)
yarn dev:local          # Vite dev wired to yarn local-bex (no cluster/Ory/prod-CORS)
yarn build              # Build for production
yarn typecheck          # generate-routes + tsc -b
yarn lint               # typecheck + ESLint + unused file/dependency analysis
yarn format             # Prettier formatting
yarn test               # Vitest — CI-enforced on every push/PR touching dashboard/** (.github/workflows/dashboard-test.yml)
yarn test:coverage      # Vitest with coverage
yarn kill               # Kill process on port 5173
```

`yarn typecheck && yarn lint && yarn test` must all pass before `deploy.yml` builds or pushes the dashboard image (`build-and-deploy` `needs:` the dashboard test job).

### Local development without a cluster (`local-bex`)

Pointing a localhost dashboard at **prod** bex-api doesn't work: `api.bex.co`'s CORS
allowlist rejects most localhost origins, and even where it doesn't, the Kratos
session cookie is host-scoped to `*.bex.co` and isn't sent from a `localhost`
origin — so every bex-api call 401s. Full-fidelity local bex needs the mock cluster

- Ory stack (`scripts/mock-cluster.sh`), which is heavy for frontend work.

`scripts/local-bex.mjs` (run via `yarn local-bex`, wired by `yarn dev:local`) is a
tiny **dev stub** — no deps, no auth, wide-open CORS — that speaks just enough of
bex-api's wire protocol to run the app offline: the GraphQL reads (`services`,
`server`, `logs`, safe empties for the rest), the SSE live-log tail
(`GET /v1/logs/subscribe`), and Kratos `GET /sessions/whoami` (so the auth guard
passes). It streams synthetic app logs so the Logs viewer's history + live tail are
exercised end-to-end. It is a DEV TOOL only — never a real backend.

## SSR environment variables

Server-only (not `VITE_`) variables read by the dashboard's SSR runtime, verbatim from the platform env-var inventory (the Go components' tables live in `lego/operator/CLAUDE.md` and `lego/backend/CLAUDE.md`):

| Component       | Variable                                                                                 | Meaning                                                                                                                                                                                                                                                                                                                                                  |
| --------------- | ---------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| dashboard (SSR) | `HYDRA_ADMIN_URL`, `HYDRA_PUBLIC_URL`, `OAUTH_TRUSTED_CLIENTS`, `OAUTH_PLATFORM_CLIENTS` | OAuth2 consent + official Render CLI device verification at `/auth/consent` and `/auth/device` (docs/ADR012-auth.md §7/§8a): Hydra's admin API, its browser-reachable public issuer, the auto-consent allowlist, and the operator-owned platform client-ID registry. Server-only (not `VITE_`); missing URLs make their corresponding routes answer 503. |
| dashboard (SSR) | `OAUTH_API_SCOPE`                                                                        | **ignored as a second matrix** (w8/m27). Consent uses the closed vocabulary `bex.read` / `bex.write` / `bex.sensitive` (plus identity scopes; `bex.api` is stripped from third-party grants). The env name is retained for compatibility. Server-only (not `VITE_`).                                                                                     |
