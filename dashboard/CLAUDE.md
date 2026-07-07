# dashboard/CLAUDE.md

The bex dashboard — see [README.md](README.md) for what it is and why it exists, and its [Authentication](README.md#authentication) section before touching anything under `src/features/auth/` or `src/common/lib/ory/`.

## Directory structure

```
src/
├── routes/                # File-based routes (TanStack Router)
│   ├── index.tsx           # sample "Services" page (bex-api shaped, not wired yet)
│   ├── auth.{login,sign-up,forgot-password,reset-password,logout}.tsx
│   └── settings.tsx        # account settings (Kratos settings flow)
├── features/auth/          # login/registration/recovery/settings pages + shared page shell
├── features/metrics/        # App metrics page (Render-style) — first real bex-api GraphQL client
├── common/
│   ├── apollo/             # Apollo Client, pointed at bex-api's /graphql (VITE_API_URL)
│   ├── lib/ory/            # Kratos config + FrontendApi factory (@ory/client-fetch)
│   ├── lib/auth/            # requireAuth beforeLoad guard
│   ├── hooks/use-ory-flow.ts # client-side Kratos flow fetch/redirect
│   ├── components/
│   │   ├── ui/              # shadcn/Radix component kit
│   │   └── dashboard-layout/ # sidebar + header chrome wrapping authenticated pages
│   ├── providers/           # RootProvider: theme + toaster + viewport-height only
│   ├── root-route/          # Root shell, 404, and error page components
│   ├── hooks/, lib/, types/  # generic utilities kept from the scaffold
├── config/                  # config.ts — VITE_API_URL / VITE_SSR_API_URL
└── test/                    # vitest setup + shared mocks
```

Add new non-auth feature code under `src/features/<name>/`, following a self-contained-module pattern (components/hooks/types per feature) rather than piling everything into `common/`.

## Code standards

- React 19+ patterns: no `FC` type, use plain function components.
- No i18n — this scaffold ships hardcoded English strings. Reintroduce an i18n layer only when there's an actual multi-language requirement, not preemptively.
- Tests live in `__tests__` directories adjacent to the code they test, e.g. `src/common/hooks/__tests__/use-mobile.test.ts`.
- Don't hand-roll auth forms — `@ory/elements-react`'s flow components already track whatever methods/fields Kratos's config actually enables; hardcoding form shapes would drift out of sync with it.

## GraphQL (bex-api, not auth)

`codegen.ts` generates typed queries/mutations from `src/**/*.graphql` into `src/graphql/definitions.ts`, introspecting bex-api's `/graphql` (`VITE_API_URL`). Every bex-api route requires a real credential (`docs/auth.md`) — introspection too — so `yarn codegen` needs `CODEGEN_SESSION_TOKEN` (an Ory session token) set in the environment; without it, codegen falls back to an unauthenticated request that bex-api will reject. This is unrelated to Kratos as a runtime dependency — bex-api and Kratos are separate services (`docs/architecture.md`, `docs/auth.md`) — the token is just how codegen authenticates _to_ bex-api.

## Visual layout pattern (w5/m2)

Reused from `beancount-dashboard`'s reports/overview page and ledger-layout chrome — reference files (read-only, not in this repo): `.../src/features/reports/overview/{index.tsx,components/overview-stat-card.tsx,components/overview-metrics-panel.tsx}`, `.../src/common/components/ledger-layout/{index.tsx,layout-header.tsx}`. Reuse our own `Card`/shadcn primitives — don't import beancount's components or CSS.

- **Section rhythm:** `space-y-6` between major page sections (the reference's own pages vary between `space-y-2 md:space-y-4` and `space-y-6 md:space-y-8`; standardize on `space-y-6` — matches this repo's existing `routes/index.tsx`).
- **Card grids:** `gap-4` — `grid grid-cols-1 gap-4 lg:grid-cols-2` for paired panels, `grid-cols-4 gap-4` for a stat-card row.
- **Stat card shape** (`overview-stat-card.tsx`, `variant="card"`): a bare `Card` with only a `CardHeader` — `CardDescription` as the label, `CardTitle` (`text-base font-semibold tabular-nums`, scaling up at container breakpoints) as the value. No `CardContent`. Use this shape for any future label+value summary tile (e.g. a services-count/status tile above `routes/index.tsx`'s sample table).
- **Header chrome:** `h-16` header (`h-12` when compact), `px-4`, `flex items-center justify-between`, `border-b bg-background/95 backdrop-blur supports-backdrop-filter:bg-background/60` — this repo's `dashboard-layout/index.tsx` `DashboardHeader` already matches this exactly; keep it as the baseline other pages should read as consistent with.
- **Content padding:** reference wraps routed content in `p-4 sm:p-6` (`p-2` on mobile) inside the layout's `<main>`, at `max-w-full`. This repo's `routes/index.tsx` currently uses a heavier `px-6 py-10 sm:px-10` + `mx-auto max-w-4xl` landing-page treatment — tighten toward the reference's denser `p-4 sm:p-6` when polishing (t005).
- **Typography:** card titles/headings use shadcn's default `CardTitle` sizing (no oversized hero headings inside dashboard content); reserve larger type for the auth pages' hero copy (`auth-page-shell`), not for in-app content.

Pages/components this pattern applies to (w5/m2 scope): `features/auth/pages/{login,register,forgot-password,settings,logout}-page/index.tsx`, `routes/index.tsx`, `common/components/dashboard-layout/{index.tsx,dashboard-sidebar.tsx}`.

## SSR gotcha

`vite.config.ts` sets `ssr.noExternal: ["@ory/elements-react"]` — the package ships extensionless relative imports (e.g. `"./session-provider"`) that only resolve under bundler resolution, not Node's strict ESM loader. Without this, `yarn dev`/`yarn build` SSR-render any page importing `@ory/elements-react` with "Cannot find module" and silently falls back to full client rendering. Don't remove it.

## Development commands

```bash
yarn dev                # Start Vite dev server
yarn build              # Build for production
yarn typecheck          # generate-routes + tsc -b
yarn lint               # typecheck + eslint
yarn format             # Prettier formatting
yarn test               # Vitest
yarn test:coverage      # Vitest with coverage
yarn kill               # Kill process on port 5173
```
