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
├── common/
│   ├── apollo/             # Apollo Client setup — kept, not yet pointed at bex-api (see codegen.ts TODO)
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

`codegen.ts` generates typed queries/mutations from `src/**/*.graphql` into `src/graphql/definitions.ts` — no `.graphql` files exist yet. Point `schema` at bex-api's `/graphql` (`docs/bex-api.md`) once wiring up real queries. This is unrelated to Kratos — bex-api and Kratos are separate services (`docs/architecture.md`, `docs/auth.md`).

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
