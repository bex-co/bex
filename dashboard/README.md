# bex dashboard

A Render-style dashboard for bex — the human-facing client of [`bex-api`](../docs/bex-api.md)'s GraphQL adapter (`docs/bex-api.md` calls it "Render dashboard compatible"). Scaffolded from a mature TanStack Start + Apollo + shadcn/Radix app, stripped down to an empty shell with one sample route.

Per [`docs/vision.md`](../docs/vision.md) pillar 1 ("API-first — no dashboard-only features, ever"), this app is a pure client of actions already exposed via bex-api's REST/GraphQL/MCP surfaces — it never grows capabilities the API doesn't already have.

## Stack

- [TanStack Start](https://tanstack.com/start) + [TanStack Router](https://tanstack.com/router) (file-based routing, SSR)
- [Apollo Client](https://www.apollographql.com/docs/react) + [GraphQL Codegen](https://the-guild.dev/graphql/codegen) — wired but not yet pointed at a live schema (see `codegen.ts`)
- [shadcn/ui](https://ui.shadcn.com/) on Radix primitives, Tailwind CSS v4
- [Ory Elements](https://www.ory.com/docs/elements) (`@ory/elements-react`) + [`@ory/client-fetch`](https://www.npmjs.com/package/@ory/client-fetch) — renders Ory Kratos's login/registration/recovery/settings flows (see [Authentication](#authentication) below)
- Vitest + Testing Library

## Authentication

Auth is [Ory Kratos](../docs/auth.md) (`auth.bex.co` in prod) — a self-service, session-cookie identity API, not a custom backend. This dashboard doesn't hand-roll login/registration forms: `@ory/elements-react`'s `<Login>`/`<Registration>`/`<Recovery>`/`<Settings>` components render whatever fields/methods Kratos's flow actually returns, inside this app's own two-column page shell (`src/features/auth/components/auth-page-shell`) so it still looks like part of the dashboard rather than a bolted-on auth vendor's UI.

- `src/common/lib/ory/` — Kratos config (`config.ts`) and the `@ory/client-fetch` `FrontendApi` factory (`frontend.ts`), SSR-cookie-aware (see `common/server-fn/session.ts`).
- `src/common/hooks/use-ory-flow.ts` — client-side flow fetch; kicks off a fresh browser flow at Kratos directly if the URL has no `?flow=` yet.
- `src/routes/auth.{login,sign-up,forgot-password,reset-password,logout}.tsx`, `src/routes/settings.tsx` — the pages themselves.
- Root route's `beforeLoad` calls Kratos's `sessions/whoami` once per navigation and passes the session down via router context (`common/lib/auth/auth.ts`'s `requireAuth` guards routes on it).

**Local dev** (against the local mock cluster's Kratos, `docs/auth.md` §5): Traefik/`auth.bex.local` isn't reachable from a laptop-run dev server, so reach Kratos directly via a port-forward instead:

```sh
kubectl -n auth port-forward service/kratos-public 4433:80
VITE_KRATOS_PUBLIC_URL=http://localhost:4433 yarn dev
```

This has been verified end-to-end (registration → session → dashboard, logout → login) by driving a real browser against the local mock cluster.

## Development

```sh
yarn install
yarn dev            # http://localhost:5173
yarn typecheck       # generate routes + tsc -b
yarn lint            # typecheck + eslint
yarn build
yarn test            # vitest run
```

## Status

Scaffold only — the sample route on `/` renders hardcoded data shaped like bex-api's `GET /v1/services`. No live GraphQL wiring yet; `codegen.ts` has a `TODO` pointing at bex-api's `/graphql` endpoint for when that lands.

## Environment variables

See `.env.example`. `VITE_API_URL` is bex-api's GraphQL endpoint (default: a local bex-api dev instance at `http://localhost:8090/graphql`). `VITE_KRATOS_PUBLIC_URL` is Ory Kratos's public API (default: `https://auth.bex.co`).
